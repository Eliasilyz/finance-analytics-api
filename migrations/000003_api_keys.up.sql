-- 000003_api_keys.up.sql
-- API keys for authenticating /api/v1 requests.
--
-- Design notes (see docs/DECISIONS.md Phase 7):
--   * key_hash stores a SHA-256 hex digest of the full token, never the token
--     itself. SHA-256 (not bcrypt) is sufficient because tokens are generated
--     with crypto/rand at 256-bit entropy: a leaked hash cannot be brute-forced,
--     and a slow KDF would only add 50-100ms of latency to every request.
--   * key_prefix stores the first 8 characters of the token so validation does
--     a fast indexed lookup followed by a SINGLE hash comparison, instead of
--     scanning every key in the table.
--   * revoked_at is a soft-revoke: NULL means active, a timestamp means the key
--     has been revoked and must be rejected.
--   * expires_at is optional key rotation: NULL means the key never expires.

CREATE TABLE api_keys (
    id          BIGSERIAL PRIMARY KEY,
    key_prefix  VARCHAR(16)  NOT NULL,
    key_hash    CHAR(64)     NOT NULL UNIQUE,
    label       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_prefix ON api_keys (key_prefix);