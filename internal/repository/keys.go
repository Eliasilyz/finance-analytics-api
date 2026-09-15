package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// CreateAPIKey persists a new API key record. key_hash is a SHA-256 digest of
// the raw token and key_prefix its first 8 characters; the raw token itself is
// never stored.
func (r *Repository) CreateAPIKey(ctx context.Context, k models.APIKey) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO api_keys (key_prefix, key_hash, label, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		k.KeyPrefix, k.KeyHash, k.Label, k.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create api key: %w", err)
	}
	return id, nil
}

// GetAPIKeysByPrefix returns every key record whose lookup prefix matches. A
// prefix is deliberately short (32 bits) for fast lookups, so multiple keys may
// share it; the caller verifies the full token hash against each candidate.
func (r *Repository) GetAPIKeysByPrefix(ctx context.Context, prefix string) ([]models.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key_prefix, key_hash, label, created_at, expires_at, revoked_at
		FROM api_keys
		WHERE key_prefix = $1
		ORDER BY id`,
		prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("get api keys by prefix: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.KeyPrefix, &k.KeyHash, &k.Label, &k.CreatedAt, &k.ExpiresAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

// ListAPIKeys lists all key metadata for audit. KeyHash is included so the raw
// token can never be recovered but integrity is verifiable.
func (r *Repository) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, key_prefix, key_hash, label, created_at, expires_at, revoked_at
		FROM api_keys
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.KeyPrefix, &k.KeyHash, &k.Label, &k.CreatedAt, &k.ExpiresAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

// RevokeAPIKey soft-revokes a key: it stays in the table for audit but is
// rejected by Validate afterwards. Returns sql.ErrNoRows if the key is unknown.
func (r *Repository) RevokeAPIKey(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`,
		id)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("revoke api key rows: %w", err)
	} else if n == 0 {
		return fmt.Errorf("%w: api key %d", sql.ErrNoRows, id)
	}
	return nil
}
