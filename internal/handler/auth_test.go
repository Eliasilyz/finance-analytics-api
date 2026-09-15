package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/auth"
	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/gin-gonic/gin"
)

// fakeAuthStore is an in-memory auth.Store for router tests.
type fakeAuthStore struct {
	keys   []models.APIKey
	nextID int64
}

func (f *fakeAuthStore) CreateAPIKey(_ context.Context, k models.APIKey) (int64, error) {
	f.nextID++
	k.ID = f.nextID
	f.keys = append(f.keys, k)
	return k.ID, nil
}

func (f *fakeAuthStore) GetAPIKeysByPrefix(_ context.Context, prefix string) ([]models.APIKey, error) {
	var out []models.APIKey
	for _, k := range f.keys {
		if k.KeyPrefix == prefix {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeAuthStore) ListAPIKeys(_ context.Context) ([]models.APIKey, error) {
	return f.keys, nil
}

func (f *fakeAuthStore) RevokeAPIKey(_ context.Context, id int64) error {
	for i := range f.keys {
		if f.keys[i].ID == id {
			now := time.Now()
			f.keys[i].RevokedAt = &now
			return nil
		}
	}
	return errors.New("not found")
}

// seedAuthStore creates one valid key and returns its raw token.
func (f *fakeAuthStore) seedKey(ctx context.Context, t *testing.T) string {
	t.Helper()
	svc := auth.NewService(f, slog.New(slog.DiscardHandler))
	raw, _, err := svc.Create(ctx, "test-key", nil)
	if err != nil {
		t.Fatalf("seed key: %v", err)
	}
	return raw
}

// authRouter builds a router with a real auth.Service backed by fakeAuthStore,
// plus optional limiter and flood guard.
func authRouter(t *testing.T, limiter, guard *fakeLimiter) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	store := &fakeAuthStore{}
	raw := store.seedKey(ctx, t)
	authSvc := auth.NewService(store, slog.New(slog.DiscardHandler))
	opts := Options{Auth: authSvc}
	if limiter != nil {
		opts.Limiter = limiter
	}
	if guard != nil {
		opts.AuthGuard = guard
	}
	h := New(service.NewQueryService(&queryStoreStub{}), slog.New(slog.DiscardHandler), opts)
	e := gin.New()
	h.Routes(e)
	return e, raw
}

func authGet(t *testing.T, r *gin.Engine, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestAuthRequiresKey(t *testing.T) {
	r, raw := authRouter(t, nil, nil)

	if w := authGet(t, r, "/api/v1/companies", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status %d, want 401", w.Code)
	}
	if w := authGet(t, r, "/api/v1/companies", "fda_notavalidkey"); w.Code != http.StatusUnauthorized {
		t.Fatalf("malformed key: status %d, want 401", w.Code)
	}
	if w := authGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("a", 64)); w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown well-formed key: status %d, want 401", w.Code)
	}
	if w := authGet(t, r, "/api/v1/companies", raw); w.Code != http.StatusOK {
		t.Fatalf("valid key: status %d, want 200", w.Code)
	}
}

func TestAuthHealthIsPublic(t *testing.T) {
	r, _ := authRouter(t, nil, nil)
	if w := authGet(t, r, "/health", ""); w.Code != http.StatusOK {
		t.Fatalf("health must stay public, got %d", w.Code)
	}
}

func TestAuthEnvelope(t *testing.T) {
	r, _ := authRouter(t, nil, nil)
	w := authGet(t, r, "/api/v1/companies", "")
	if !strings.Contains(w.Body.String(), `"code":"UNAUTHORIZED"`) {
		t.Fatalf("expected UNAUTHORIZED envelope, got %s", w.Body.String())
	}
}

func TestAuthFailuresCollapseToOne401(t *testing.T) {
	r, _ := authRouter(t, nil, nil)
	// Malformed and well-formed-but-unknown must be indistinguishable.
	a := authGet(t, r, "/api/v1/companies", "garbage")
	b := authGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("b", 64))
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Fatalf("rejection responses must be identical, got %d %s vs %d %s", a.Code, a.Body.String(), b.Code, b.Body.String())
	}
}

func TestAuthFloodGuardBlocksBruteForce(t *testing.T) {
	r, _ := authRouter(t, nil, &fakeLimiter{allow: false, err: nil})

	// The guard is per-IP on the AUTH-FAILURE path only: failed auth while the
	// guard is tripped yields 429 instead of 401.
	w := authGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("c", 64))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked IP with bad key: status %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"RATE_LIMITED"`) {
		t.Fatalf("expected RATE_LIMITED envelope, got %s", w.Body.String())
	}
}

func TestAuthFloodGuardFailsOpen(t *testing.T) {
	r, _ := authRouter(t, nil, &fakeLimiter{allow: false, err: errFakeDown})
	// Guard down → fail open → auth failure still 401 (not 429, not 200).
	// A guard outage must never mask the 401 that auth failure already produces.
	w := authGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("d", 64))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("guard down with bad key: status %d, want 401", w.Code)
	}
}

func TestAuthfloodGuardAppliesOnlyToFailures(t *testing.T) {
	// A tripped guard must not reject requests that authenticate successfully;
	// the guard only counts auth-failure floods per IP.
	r, raw := authRouter(t, &fakeLimiter{allow: true, err: nil}, &fakeLimiter{allow: false, err: nil})
	if w := authGet(t, r, "/api/v1/companies", raw); w.Code != http.StatusOK {
		t.Fatalf("valid key with tripped guard: status %d, want 200", w.Code)
	}
}
