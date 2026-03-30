-- Migration 007: Authentication audit log
-- Adds auth_audit_log table for tracking all authentication events

CREATE TABLE IF NOT EXISTS auth_audit_log (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    account_id TEXT,
    event TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for audit log queries
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_user_id ON auth_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_account_id ON auth_audit_log(account_id);
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_event ON auth_audit_log(event);
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_created_at ON auth_audit_log(created_at DESC);

-- Composite index for recent failed login attempts
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_user_event_time ON auth_audit_log(user_id, event, created_at DESC);

-- Index for IP-based queries (rate limiting support)
CREATE INDEX IF NOT EXISTS idx_auth_audit_log_ip_address ON auth_audit_log(ip_address);
