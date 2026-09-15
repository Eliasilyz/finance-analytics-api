package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// fakeStore is an in-memory auth.Store.
type fakeStore struct {
	keys     []models.APIKey
	nextID   int64
	prefixEr error
}

func (f *fakeStore) CreateAPIKey(_ context.Context, k models.APIKey) (int64, error) {
	f.nextID++
	k.ID = f.nextID
	f.keys = append(f.keys, k)
	return k.ID, nil
}

func (f *fakeStore) GetAPIKeysByPrefix(_ context.Context, prefix string) ([]models.APIKey, error) {
	if f.prefixEr != nil {
		return nil, f.prefixEr
	}
	var out []models.APIKey
	for _, k := range f.keys {
		if k.KeyPrefix == prefix {
			out = append(out, k)
		}
	}
	return out, nil
}

func (f *fakeStore) ListAPIKeys(_ context.Context) ([]models.APIKey, error) {
	return f.keys, nil
}

func (f *fakeStore) RevokeAPIKey(_ context.Context, id int64) error {
	for i := range f.keys {
		if f.keys[i].ID == id {
			t := time.Now()
			f.keys[i].RevokedAt = &t
			return nil
		}
	}
	return errors.New("not found")
}

func testService(store Store) *Service {
	return NewService(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGenerateRawShape(t *testing.T) {
	raw, err := GenerateRaw()
	if err != nil {
		t.Fatalf("GenerateRaw: %v", err)
	}
	// fda_ + 64 hex chars, 256-bit entropy.
	if len(raw) != len(KeyPrefix)+64 {
		t.Fatalf("raw key length = %d, want %d", len(raw), len(KeyPrefix)+64)
	}
	if !rawKeyRe.MatchString(raw) {
		t.Fatalf("raw key does not match expected shape: %q", raw)
	}
	stale, err := GenerateRaw()
	if err != nil {
		t.Fatalf("GenerateRaw (second): %v", err)
	}
	if HashKey(raw) == HashKey(stale) {
		t.Fatal("hash must differ between distinct keys")
	}
}

func TestCreateAndValidate(t *testing.T) {
	ctx := context.Background()
	svc := testService(&fakeStore{})
	raw, k, err := svc.Create(ctx, "test-key", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := svc.Validate(ctx, raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.ID != k.ID || got.KeyPrefix != LookupPrefix(raw) {
		t.Fatalf("validated key mismatch: %+v", got)
	}
	if got.KeyHash == raw {
		t.Fatal("plaintext token must not be treated as its hash")
	}
}

func TestValidateRejections(t *testing.T) {
	svc := testService(&fakeStore{})
	raw, _, err := svc.Create(context.Background(), "t", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	cases := map[string]string{
		"empty":                     "",
		"wrong prefix":              "abc_" + raw[len(KeyPrefix):],
		"bad hex char":              "fda_z" + raw[len(KeyPrefix)+1:],
		"too short":                 "fda_00",
		"uppercase hex":             "fda_" + uppercaseHex(raw[len(KeyPrefix):]),
		"valid-looking but unknown": "fda_" + allF(),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Validate(context.Background(), key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("want ErrInvalidKey, got %v", err)
			}
		})
	}
}

func TestRevokedAndExpiredRejected(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := testService(store)

	raw, k, err := svc.Create(ctx, "revokable", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Revoke(ctx, k.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := svc.Validate(ctx, raw); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("revoked key: want ErrInvalidKey, got %v", err)
	}

	past := time.Now().Add(-time.Hour)
	raw2, _, err := svc.Create(ctx, "expiring", &past)
	if err != nil {
		t.Fatalf("Create expired: %v", err)
	}
	if _, err := svc.Validate(ctx, raw2); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expired key: want ErrInvalidKey, got %v", err)
	}
}

func TestPrefixCollisionValidatesOnlyMatchingHash(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := testService(store)

	raw, real, err := svc.Create(ctx, "real", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Decoy: a second key whose stored prefix collides with the real key's,
	// but whose hash is different. Validation must pick the matching hash.
	_, decoy, err := svc.Create(ctx, "decoy", nil)
	if err != nil {
		t.Fatalf("Create decoy: %v", err)
	}
	store.keys[1].KeyPrefix = LookupPrefix(raw)

	got, err := svc.Validate(ctx, raw)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.ID != real.ID {
		t.Fatalf("resolved to %d, want real key %d", got.ID, real.ID)
	}
	if decoy.ID == got.ID {
		t.Fatal("decoy must not satisfy validation")
	}
}

func uppercaseHex(s string) string {
	out := []rune(s)
	for i, c := range out {
		if c >= 'a' && c <= 'f' {
			out[i] = c - 'a' + 'A'
		}
	}
	return string(out)
}

func allF() string {
	return "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
}
