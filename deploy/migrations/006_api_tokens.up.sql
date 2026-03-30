-- Migration 006: API tokens for relay authentication
-- Adds api_tokens table for managing tunnel authentication tokens

CREATE TABLE IF NOT EXISTS api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for active token lookups by user
CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id_active ON api_tokens(user_id, revoked_at) WHERE revoked_at IS NULL;

-- Index for token hash lookups during authentication
CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);

-- Index for account-level token lookups
CREATE INDEX IF NOT EXISTS idx_api_tokens_account_id ON api_tokens(account_id);

-- Index for cleanup of expired tokens
CREATE INDEX IF NOT EXISTS idx_api_tokens_expires_at ON api_tokens(expires_at) WHERE expires_at IS NOT NULL;
