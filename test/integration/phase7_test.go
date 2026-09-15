//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/eliasilyz/finance-analytics-api/internal/auth"
	"github.com/eliasilyz/finance-analytics-api/internal/cache"
	"github.com/eliasilyz/finance-analytics-api/internal/handler"
	"github.com/eliasilyz/finance-analytics-api/internal/rate"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

// authStack wires auth + per-key limiter + auth-failure guard through the real
// HTTP stack. Returns the router, the raw valid key, and the auth service (for
// revoking/timing-out keys mid-test).
func authStack(t *testing.T, db *sql.DB, rdb *redis.Client, limiterLimit, guardLimit int64, guardErr error) (*gin.Engine, string, *auth.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := repository.NewRepository(db)
	qs := service.NewQueryService(repo, cache.NewRedis(rdb, 10*time.Minute))
	authSvc := auth.NewService(repo, logger)

	raw, _, err := authSvc.Create(ctx, "integration-test", nil)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	lim := rate.NewRedis(rdb, limiterLimit, time.Hour)
	guard := rate.NewRedis(rdb, guardLimit, time.Hour)

	h := handler.New(qs, logger, handler.Options{
		Auth:      authSvc,
		Limiter:   lim,
		AuthGuard: guard,
	})
	r := gin.New()
	h.Routes(r)
	return r, raw, authSvc
}

func authedGet(t *testing.T, r *gin.Engine, path, key string) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestAuthMiddlewareOnRealStack proves the full HTTP surface requires a key and
// rejects missing/invalid keys uniformly with 401.
func TestAuthMiddlewareOnRealStack(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)
	r, raw, _ := authStack(t, db, rdb, 1000, 1000, nil)

	code, body := authedGet(t, r, "/api/v1/companies", "")
	if code != http.StatusUnauthorized || !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
		t.Fatalf("no key: %d %s", code, body)
	}
	code, body = authedGet(t, r, "/api/v1/companies", "nonsense")
	if code != http.StatusUnauthorized || !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
		t.Fatalf("malformed key: %d %s", code, body)
	}
	code, body = authedGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("0", 64))
	if code != http.StatusUnauthorized {
		t.Fatalf("unknown well-formed key: %d %s", code, body)
	}
	code, _ = authedGet(t, r, "/api/v1/companies", raw)
	if code != http.StatusOK {
		t.Fatalf("valid key: %d, want 200", code)
	}
	code, _ = authedGet(t, r, "/health", "")
	if code != http.StatusOK {
		t.Fatalf("health must stay public: %d", code)
	}
}

// TestRevokedKeyRejected proves a revoked key stops working immediately.
func TestRevokedKeyRejected(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)
	r, raw, authSvc := authStack(t, db, rdb, 1000, 1000, nil)

	if code, _ := authedGet(t, r, "/api/v1/companies", raw); code != http.StatusOK {
		t.Fatalf("before revoke: not 200")
	}
	if err := authSvc.Revoke(context.Background(), 1); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	code, body := authedGet(t, r, "/api/v1/companies", raw)
	if code != http.StatusUnauthorized || !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
		t.Fatalf("revoked key: %d %s", code, body)
	}
}

// TestExpiredKeyRejected proves an expired key is rejected.
func TestExpiredKeyRejected(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx := context.Background()
	repo := repository.NewRepository(db)
	qs := service.NewQueryService(repo, cache.NewRedis(rdb, 10*time.Minute))
	authSvc := auth.NewService(repo, logger)

	past := time.Now().Add(-time.Hour)
	raw, _, err := authSvc.Create(ctx, "expired", &past)
	if err != nil {
		t.Fatalf("create expired key: %v", err)
	}
	h := handler.New(qs, logger, handler.Options{Auth: authSvc, Limiter: rate.NewRedis(rdb, 1000, time.Hour)})
	r := gin.New()
	h.Routes(r)

	code, body := authedGet(t, r, "/api/v1/companies", raw)
	if code != http.StatusUnauthorized || !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
		t.Fatalf("expired key: %d %s", code, body)
	}
}

// TestRateLimitIsPerKey proves two different keys get independent buckets.
func TestRateLimitIsPerKey(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewRepository(db)
	authSvc := auth.NewService(repo, logger)
	rawA, _, err := authSvc.Create(ctx, "keyA", nil)
	if err != nil {
		t.Fatalf("create keyA: %v", err)
	}
	rawB, _, err := authSvc.Create(ctx, "keyB", nil)
	if err != nil {
		t.Fatalf("create keyB: %v", err)
	}

	qs := service.NewQueryService(repo, cache.NewRedis(rdb, 10*time.Minute))
	// Limit 3 per key. Key A burns its 3 slots; key B must be untouched.
	lim := rate.NewRedis(rdb, 3, time.Hour)
	h := handler.New(qs, logger, handler.Options{Auth: authSvc, Limiter: lim})
	r := gin.New()
	h.Routes(r)

	for i := 1; i <= 4; i++ {
		code, body := authedGet(t, r, "/api/v1/companies", rawA)
		if i < 4 {
			if code != http.StatusOK {
				t.Fatalf("keyA request %d: %d %s", i, code, body)
			}
		} else if code != http.StatusTooManyRequests || !strings.Contains(body, `"code":"RATE_LIMITED"`) {
			t.Fatalf("keyA request 4 should 429: %d %s", code, body)
		}
	}
	code, body := authedGet(t, r, "/api/v1/companies", rawB)
	if code != http.StatusOK {
		t.Fatalf("keyB must have its own bucket: %d %s", code, body)
	}
}

// TestAuthFloodGuardBadKeys proves many failed auth attempts from one IP trip
// the loose per-IP guard (429), while a valid key from the same IP still
// passes (the guard only counts auth failures).
func TestAuthFloodGuardBadKeys(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)

	// Guard limit 3 per window: the 4th failed auth attempt must 429.
	r, raw, _ := authStack(t, db, rdb, 1000, 3, nil)
	for i := 1; i <= 4; i++ {
		code, body := authedGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat(string(rune('0'+i)), 64))
		if i < 4 {
			if code != http.StatusUnauthorized {
				t.Fatalf("bad key %d: want 401, got %d %s", i, code, body)
			}
		} else if code != http.StatusTooManyRequests || !strings.Contains(body, `"code":"RATE_LIMITED"`) {
			t.Fatalf("4th bad key should 429 via flood guard: %d %s", code, body)
		}
	}
	// A valid key from the same IP is unaffected by the tripped failure-guard.
	code, body := authedGet(t, r, "/api/v1/companies", raw)
	if code != http.StatusOK {
		t.Fatalf("valid key after guard tripped: %d %s", code, body)
	}
}

// TestAuthFloodGuardRedisDownFailsOpen proves a dead Redis cannot take the API
// down: valid keys flow, and failed auth still returns 401 (not 429/500), in
// line with the Phase 6 fail-open policy.
func TestAuthFloodGuardRedisDownFailsOpen(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	seedAAPL(t, db)
	dead := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  300 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
	})
	t.Cleanup(func() { _ = dead.Close() })

	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewRepository(db)
	authSvc := auth.NewService(repo, logger)
	raw, _, err := authSvc.Create(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	qs := service.NewQueryService(repo, cache.NewRedis(dead, 10*time.Minute))
	lim := rate.NewRedis(dead, 1000, time.Hour)
	guard := rate.NewRedis(dead, 1000, time.Hour)
	h := handler.New(qs, logger, handler.Options{Auth: authSvc, Limiter: lim, AuthGuard: guard})
	r := gin.New()
	h.Routes(r)

	code, body := authedGet(t, r, "/api/v1/companies", "fda_"+strings.Repeat("1", 64))
	if code != http.StatusUnauthorized || !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
		t.Fatalf("dead guard + bad key: want 401, got %d %s", code, body)
	}
	if code, _ := authedGet(t, r, "/api/v1/companies", raw); code != http.StatusOK {
		t.Fatalf("dead guard + valid key: want 200, got %d", code)
	}
}

// TestInjectionResistantSQL proves the DoD "input jahat" suite: hostile input
// at the API boundary is rejected or handled safely, never crashing or
// altering the data.
func TestInjectionResistantSQL(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)
	r, raw, _ := authStack(t, db, rdb, 1000, 1000, nil)

	// Symbol-shaped injections must be rejected before user input reaches SQL.
	// The rejection code may be 400 (IsValidSymbol) or 404 (gin can't route a
	// decoded slash), never 2xx (handler ran) and never 5xx (crash).
	evilSymbols := []string{
		"AAPL'); DROP TABLE companies;--",
		"' OR '1'='1",
		"<script>alert(1)</script>",
		"..",
	}
	for _, sym := range evilSymbols {
		path := "/api/v1/stocks/" + url.PathEscape(sym) + "/analytics"
		code, body := authedGet(t, r, path, raw)
		if code < 400 || code >= 500 {
			t.Fatalf("symbol %q: want 4xx, got %d %s", sym, code, body)
		}
	}

	// Screener filter injection must be bound as a parameter, not concatenated.
	evilSectors := []string{
		"'; DROP TABLE companies;--",
		"' OR '1'='1",
		"Technology'; UPDATE companies SET name='pwned';--",
	}
	for _, sector := range evilSectors {
		path := "/api/v1/screener?sector=" + url.QueryEscape(sector)
		code, body := authedGet(t, r, path, raw)
		if code != http.StatusOK {
			t.Fatalf("sector %q: want 200, got %d %s", sector, code, body)
		}
	}

	// The companies table must survive every attack above.
	var n int
	if err := db.QueryRow("SELECT count(*) FROM companies").Scan(&n); err != nil {
		t.Fatalf("count companies: %v", err)
	}
	if n != 1 {
		t.Fatalf("companies count = %d, want 1 (table must be intact)", n)
	}
	var name string
	if err := db.QueryRow("SELECT name FROM companies WHERE symbol='AAPL'").Scan(&name); err != nil || name != "Apple Inc." {
		t.Fatalf("AAPL row corrupted: name=%q err=%v", name, err)
	}
}

// TestAPIKeyCRUDOnRealDB proves repository-level API key lifecycle through a
// real Postgres: create → lookup by prefix → revoke → not found.
func TestAPIKeyCRUDOnRealDB(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	ctx := context.Background()
	authSvc := auth.NewService(repository.NewRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	raw, key, err := authSvc.Create(ctx, "crud-test", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := authSvc.Validate(ctx, raw)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.ID != key.ID || got.KeyPrefix != auth.LookupPrefix(raw) {
		t.Fatalf("validated key mismatch: %+v", got)
	}
	if got.KeyHash == raw {
		t.Fatal("plaintext must not be stored as its hash")
	}

	// Hash must actually equal SHA-256 of the raw token.
	if got.KeyHash != auth.HashKey(raw) {
		t.Fatalf("key_hash mismatch: %s", got.KeyHash)
	}

	if err := authSvc.Revoke(ctx, key.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := authSvc.Validate(ctx, raw); !errors.Is(err, auth.ErrInvalidKey) {
		t.Fatalf("revoked key: want ErrInvalidKey, got %v", err)
	}

	// Lookup by full (fake) token with the right shape but unknown prefix.
	if _, err := authSvc.Validate(ctx, "fda_"+strings.Repeat("f", 64)); !errors.Is(err, auth.ErrInvalidKey) {
		t.Fatalf("unknown key: want ErrInvalidKey, got %v", err)
	}
}
