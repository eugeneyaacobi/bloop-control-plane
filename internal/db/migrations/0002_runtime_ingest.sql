CREATE TABLE IF NOT EXISTS runtime_tunnel_snapshots (
    id TEXT PRIMARY KEY,
    tunnel_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    hostname TEXT NOT NULL,
    access_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    degraded BOOLEAN NOT NULL DEFAULT FALSE,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_snapshots_account_id ON runtime_tunnel_snapshots(account_id);
CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_snapshots_tunnel_id ON runtime_tunnel_snapshots(tunnel_id);
CREATE INDEX IF NOT EXISTS idx_runtime_tunnel_snapshots_observed_at ON runtime_tunnel_snapshots(observed_at DESC);

CREATE TABLE IF NOT EXISTS runtime_events (
    id TEXT PRIMARY KEY,
    account_id TEXT,
    tunnel_id TEXT,
    hostname TEXT,
    kind TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_events_account_id ON runtime_events(account_id);
CREATE INDEX IF NOT EXISTS idx_runtime_events_occurred_at ON runtime_events(occurred_at DESC);
