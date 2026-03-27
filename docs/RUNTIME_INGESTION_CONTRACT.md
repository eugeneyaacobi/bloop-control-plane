# bloop runtime ingestion contract (v1)

This document defines the first practical contract between `bloop-tunnel` and `bloop-control-plane` so the product UI can move from DB-derived approximations to runtime-informed truth.

## Goal
Provide enough runtime signal for `bloop-control-plane` to project:
- active route counts
- protected/public route counts
- degraded route counts
- recent customer/admin activity
- flagged exposures requiring admin attention

This is a **v1 contract**, not a forever protocol.

## Integration stance
For v1, `bloop-tunnel` remains the runtime source of truth for live tunnel/session state.
`bloop-control-plane` becomes the durable product/API surface by ingesting runtime snapshots or events.

## Recommended v1 shape
### Option A (preferred for first step)
`bloop-tunnel` periodically POSTs a **signed runtime snapshot** to a control-plane ingest endpoint.

Why:
- simpler than full streaming/event bus
- easy to replay/replace
- enough for dashboard truth and operational visibility

### Option B (later)
Move to event stream / append-only updates when runtime complexity justifies it.

## Proposed control-plane endpoint
`POST /api/runtime/ingest/snapshot`

Auth:
- shared secret or signed token dedicated to runtime ingestion
- separate from user session auth

Content-Type:
- `application/json`

## Snapshot payload
```json
{
  "source": "bloop-relay",
  "capturedAt": "2026-03-26T22:00:00Z",
  "relay": {
    "region": "iad-1",
    "instanceId": "relay-primary"
  },
  "sessions": [
    {
      "sessionId": "sess_123",
      "clientIdentity": "client-gene-laptop",
      "connectedAt": "2026-03-26T21:58:00Z",
      "lastSeenAt": "2026-03-26T22:00:00Z",
      "connectionState": "connected"
    }
  ],
  "tunnels": [
    {
      "tunnelId": "api",
      "accountId": "acct_default",
      "hostname": "api.bloop.to",
      "target": "app-server:8080",
      "accessMode": "token_protected",
      "status": "healthy",
      "degraded": false,
      "sessionId": "sess_123",
      "region": "iad-1",
      "observedAt": "2026-03-26T22:00:00Z"
    }
  ],
  "events": [
    {
      "id": "evt_1",
      "kind": "tunnel.degraded",
      "accountId": "acct_default",
      "tunnelId": "api",
      "hostname": "api.bloop.to",
      "level": "warn",
      "message": "Increased upstream timeout rate detected",
      "occurredAt": "2026-03-26T21:59:55Z"
    }
  ]
}
```

## Minimum required fields for v1
### Tunnels
- `tunnelId`
- `accountId`
- `hostname`
- `accessMode`
- `status`
- `degraded`
- `observedAt`

### Events
- `id`
- `kind`
- `accountId` (or global-only event)
- `level`
- `message`
- `occurredAt`

## Control-plane responsibilities
On ingest, control-plane should:
1. authenticate ingest caller
2. upsert runtime projection rows keyed by account/tunnel
3. update recent activity feed materialization
4. derive customer/admin runtime snapshots from ingested truth
5. retain enough timestamp data to detect stale runtime reports

## Suggested first schema additions
Not implemented yet, but recommended:
- `runtime_tunnel_snapshots`
- `runtime_events`
- `runtime_sources`

## Derived UI fields from ingest
### Customer dashboard
- active routes
- protected routes
- degraded routes
- recent route/runtime activity

### Admin dashboard
- flagged exposures
- recent global runtime events
- tunnels missing auth / unhealthy / disconnected

## Failure/operational rules
- ingest should be idempotent for identical snapshot/event ids
- stale snapshots should not overwrite newer data
- control-plane should expose whether runtime data is stale vs fresh
- missing runtime feed should degrade gracefully to current DB-derived projection behavior

## v1 non-goals
- full streaming event bus
- complex multi-relay reconciliation
- long-term metrics storage
- alerting pipeline

## Recommended next implementation step
1. add `POST /api/runtime/ingest/snapshot`
2. add secret-based runtime ingest auth
3. add minimal runtime snapshot tables
4. switch `PostgresRuntimeRepository` to prefer ingested runtime rows over derived fallback logic
