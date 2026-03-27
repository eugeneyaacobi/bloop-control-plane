CREATE TABLE IF NOT EXISTS runtime_installations (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    environment TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_runtime_installations_account_id ON runtime_installations(account_id);
CREATE INDEX IF NOT EXISTS idx_runtime_installations_status ON runtime_installations(status);

CREATE TABLE IF NOT EXISTS runtime_installation_tokens (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL REFERENCES runtime_installations(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_installation_tokens_token_hash ON runtime_installation_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_runtime_installation_tokens_installation_id ON runtime_installation_tokens(installation_id);

CREATE TABLE IF NOT EXISTS runtime_tunnel_bindings (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    installation_id TEXT NOT NULL REFERENCES runtime_installations(id) ON DELETE CASCADE,
    tunnel_id TEXT NOT NULL REFERENCES tunnels(id) ON DELETE CASCADE,
    runtime_tunnel_name TEXT NOT NULL,
    hostname TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_bindings_account_id ON runtime_tunnel_bindings(account_id);
CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_bindings_installation_id ON runtime_tunnel_bindings(installation_id);
CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_bindings_tunnel_id ON runtime_tunnel_bindings(tunnel_id);

ALTER TABLE runtime_tunnel_snapshots ADD COLUMN IF NOT EXISTS installation_id TEXT REFERENCES runtime_installations(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_snapshots_installation_id ON runtime_tunnel_snapshots(installation_id);

ALTER TABLE runtime_events ADD COLUMN IF NOT EXISTS installation_id TEXT REFERENCES runtime_installations(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_runtime_events_installation_id ON runtime_events(installation_id);
