// Package auth issues and validates API keys for /api/v1 requests.
//
// A raw key is "fda_" + 32 bytes of crypto/rand encoded as 64 hex chars
// (256-bit entropy). Only its SHA-256 digest and an 8-char lookup prefix are
// persisted. SHA-256 is deliberately chosen over bcrypt: bcrypt's slowness
// exists to raise the cost of guessing low-entropy passwords, while a 256-bit
// random token cannot be brute-forced from a leaked digest, so a slow KDF
// would only add ~50-100ms of latency to every authenticated request for no
// security gain. See docs/DECISIONS.md (Phase 7).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

const (
	// KeyPrefix is the visible, non-secret prefix of every issued key.
	KeyPrefix = "fda_"
	// KeyEntropyBytes is the random payload size: 32 bytes = 256-bit entropy.
	KeyEntropyBytes = 32
	// LookupPrefixLen is the number of raw-key hex chars stored for indexed
	// lookups (32 bits). Collisions are handled by comparing every candidate.
	LookupPrefixLen = 8
)

// ErrInvalidKey is returned for any rejection: malformed key, unknown key,
// wrong hash, expired, or revoked. The handler maps every cause to a single
// 401 so attackers cannot distinguish valid-looking keys from real ones.
var ErrInvalidKey = errors.New("auth: invalid api key")

var rawKeyRe = regexp.MustCompile(`^fda_[0-9a-f]{64}$`)

// Store is the persistence surface the auth service needs. The repository
// package implements it; tests use an in-memory fake.
type Store interface {
	CreateAPIKey(ctx context.Context, k models.APIKey) (int64, error)
	GetAPIKeysByPrefix(ctx context.Context, prefix string) ([]models.APIKey, error)
	ListAPIKeys(ctx context.Context) ([]models.APIKey, error)
	RevokeAPIKey(ctx context.Context, id int64) error
}

// Service issues and validates API keys.
type Service struct {
	store Store
	log   *slog.Logger
}

// NewService wires an auth service.
func NewService(store Store, log *slog.Logger) *Service {
	return &Service{store: store, log: log}
}

// GenerateRaw produces a new raw key with 256-bit entropy. The caller is
// responsible for showing it exactly once.
func GenerateRaw() (string, error) {
	b := make([]byte, KeyEntropyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate key: %w", err)
	}
	return KeyPrefix + hex.EncodeToString(b), nil
}

// HashKey returns the SHA-256 hex digest of a raw key.
func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// LookupPrefix returns the first 8 hex chars of the random portion of a raw
// key, the indexed component used to narrow candidate lookups.
func LookupPrefix(raw string) string {
	return raw[len(KeyPrefix) : len(KeyPrefix)+LookupPrefixLen]
}

// Create issues a new key for label and persists it. It returns the raw token,
// which is shown exactly once and never retrievable afterwards. expiresAt may
// be nil for a non-expiring key.
func (s *Service) Create(ctx context.Context, label string, expiresAt *time.Time) (string, *models.APIKey, error) {
	raw, err := GenerateRaw()
	if err != nil {
		return "", nil, err
	}
	k := models.APIKey{
		KeyPrefix: LookupPrefix(raw),
		KeyHash:   HashKey(raw),
		Label:     label,
		ExpiresAt: expiresAt,
	}
	id, err := s.store.CreateAPIKey(ctx, k)
	if err != nil {
		return "", nil, err
	}
	k.ID = id
	s.log.Info("api key created", "id", id, "label", label, "prefix", k.KeyPrefix)
	return raw, &k, nil
}

// Validate checks a raw token against the store. All rejection causes collapse
// into ErrInvalidKey so callers cannot distinguish which condition failed.
func (s *Service) Validate(ctx context.Context, raw string) (*models.APIKey, error) {
	if len(raw) != len(KeyPrefix)+64 || !rawKeyRe.MatchString(raw) {
		return nil, ErrInvalidKey
	}
	candidates, err := s.store.GetAPIKeysByPrefix(ctx, LookupPrefix(raw))
	if err != nil {
		return nil, fmt.Errorf("auth: prefix lookup: %w", err)
	}
	now := time.Now()
	hash := HashKey(raw)
	for i := range candidates {
		k := &candidates[i]
		if subtle.ConstantTimeCompare([]byte(hash), []byte(k.KeyHash)) != 1 {
			continue
		}
		if k.RevokedAt != nil || (k.ExpiresAt != nil && now.After(*k.ExpiresAt)) {
			return nil, ErrInvalidKey
		}
		return k, nil
	}
	return nil, ErrInvalidKey
}

// Revoke soft-revokes a key by id. The row is kept for audit.
func (s *Service) Revoke(ctx context.Context, id int64) error {
	return s.store.RevokeAPIKey(ctx, id)
}

// List returns all key metadata (hashes and prefixes only; raw tokens are never
// recoverable).
func (s *Service) List(ctx context.Context) ([]models.APIKey, error) {
	return s.store.ListAPIKeys(ctx)
}
