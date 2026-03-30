# Implementation Plan: Authentication, Token Management & WebAuthn 2FA

**Feature**: `004-auth-tokens-webauthn`
**Created**: 2026-03-30
**Status**: Plan

## Technical Context

- **Language**: Go 1.24
- **Framework**: chi/v5 router, pgx/v5 postgres driver
- **Existing patterns**: Handler → Service → Repository layers, chi.Mount for route groups
- **New dependencies**: `github.com/go-webauthn/webauthn/webauthn`, `golang.org/x/crypto/argon2id`
- **Database**: PostgreSQL via pgx, migrations as numbered SQL files
- **Session model**: HMAC-signed tokens via `internal/session`, cookie-based sessions
- **Frontend contract**: Defined in `bloop-frontend/lib/control-plane-api.ts` and `lib/models/control-plane.ts`

## Architecture

All new code follows the existing pattern:

```
internal/
  api/auth/          — HTTP handlers for register, login, refresh, WebAuthn
  api/tokens/        — HTTP handlers for API token CRUD
  service/auth.go    — Registration, login, credential verification logic
  service/token.go   — API token lifecycle (create, revoke, refresh, validate)
  service/webauthn.go — WebAuthn ceremony orchestration
  repository/auth_repository.go    — User credentials, WebAuthn creds, challenges
  repository/token_repository.go   — API tokens CRUD
  repository/audit_repository.go   — Auth audit log
  repository/lockout_repository.go  — Account lockout state
  security/password.go   — Argon2id hashing, breach check
  security/webauthn.go   — WebAuthn config & helpers
```

## Constitution Check

- **No plaintext secrets**: Passwords hashed with argon2id. API tokens stored as SHA-256 hashes. WebAuthn stores public keys only. ✅
- **Parameterized queries**: All SQL uses pgx parameterized queries. ✅
- **Error messages**: No information leakage in error responses. Generic "invalid credentials" for login failures. ✅
- **Rate limiting**: Login endpoints rate-limited per IP and per account. Progressive backoff. ✅
- **Audit trail**: All auth events logged to `auth_audit_log`. ✅

## Phase 0: Foundation

### 0.1 Database Migrations

Create migration files for all new tables. Migration files go in `deploy/migrations/` with numbered prefixes.

Files:
- `deploy/migrations/004_user_credentials.up.sql`
- `deploy/migrations/005_webauthn_credentials.up.sql`
- `deploy/migrations/006_api_tokens.up.sql`
- `deploy/migrations/007_auth_audit_log.up.sql`
- `deploy/migrations/008_login_attempts_lockout.up.sql`
- `deploy/migrations/009_users_alter_webauthn.up.sql`

### 0.2 Password Hashing Module

Create `internal/security/password.go` with:
- `HashPassword(password string) (hash string, err error)` — argon2id with sensible defaults
- `VerifyPassword(password, hash string) (match bool, err error)` — constant-time comparison
- `CheckBreachPassword(password string) (compromised bool, err error)` — HIBP k-anonymity check (optional, configurable)

### 0.3 Auth Audit Logger

Create `internal/repository/audit_repository.go` with:
- `LogAuthEvent(ctx, userID, event, ip, userAgent, success, metadata)` — inserts audit record
- `GetRecentAuthEvents(ctx, userID, limit)` — for admin review

## Phase 1: Registration & Login

### 1.1 Registration

- `POST /api/auth/register` — accepts `{email, username, password}`
- Validates password (length ≥ 12, breach check optional)
- Hashes with argon2id
- Creates user record + credential record in transaction
- Issues session cookie
- Logs audit event
- Returns 409 for duplicate email/username (generic message, no enumeration)

### 1.2 Login

- `POST /api/auth/login` — accepts `{email, password}`
- Rate-limited per IP (10/min) and per account (5/15min)
- Loads credential, verifies with argon2id
- Checks account lockout
- If WebAuthn enabled, returns `{requires_webauthn: true, challenge_id: "..."}`
- If no WebAuthn, issues full session
- Logs audit event (success or failure)
- Failed attempts increment counter, lock after 20 in 1 hour

### 1.3 Session Refresh

- `POST /api/auth/refresh` — accepts current session cookie
- Validates existing session (not expired, not revoked version)
- Issues new session token with extended expiry
- Old token remains valid until natural expiry

### 1.4 Route Wiring

New route group in `internal/api/router.go`:
```go
r.Route("/api/auth", func(sr chi.Router) {
    authapi.Mount(sr, authHandler)
})
```

## Phase 2: API Token Management

### 2.1 Token Creation

- `POST /api/tokens` — accepts `{name, expires_in}`
- Generates 256-bit random token, returns full value
- Stores SHA-256 hash in `api_tokens`
- Stores first 8 chars as `token_prefix` for identification
- Returns `{id, name, token, token_prefix, created_at, expires_at}`

### 2.2 Token Listing

- `GET /api/tokens` — lists all active tokens for the authenticated user
- Returns `{id, name, token_prefix, created_at, last_used_at, expires_at}`
- Filters out fully expired tokens (but shows revoked ones with revoked_at)

### 2.3 Token Revocation

- `POST /api/tokens/{id}/revoke` — sets `revoked_at` on the token
- Validates token belongs to the authenticated user
- Logs audit event

### 2.4 Token Refresh

- `POST /api/tokens/{id}/refresh` — creates new token, revokes old one
- Same name, new value
- Returns new token value (shown once)

### 2.5 Token Validation Helper

Internal function `ValidateAPIToken(ctx, tokenValue string) (token, error)` for use by the relay and enrollment endpoints. Checks hash, expiry, revocation.

## Phase 3: WebAuthn 2FA

### 3.1 WebAuthn Configuration

Create `internal/security/webauthn.go`:
- Initialize `webauthn.WebAuthn` with relying party config
- RP ID from config (e.g., `bloop.to`)
- RP name: "Bloop"
- Origins: configured allowed origins

### 3.2 Registration Ceremony

- `POST /api/auth/webauthn/begin-registration` — authenticated users only
  - Creates challenge, stores in `webauthn_challenges`
  - Returns `PublicKeyCredentialCreationOptions`
- `POST /api/auth/webauthn/finish-registration` — authenticated users only
  - Validates attestation response
  - Stores credential (id, public key, attestation type, aaguid)
  - Sets `webauthn_enabled = true` on user
  - Cleans up challenge

### 3.3 Authentication Ceremony

- `POST /api/auth/webauthn/begin-login` — accepts `{email}`
  - Looks up user, loads their credentials
  - Creates challenge
  - Returns `PublicKeyCredentialRequestOptions` with allowed credentials
- `POST /api/auth/webauthn/finish-login` — accepts signed assertion
  - Verifies signature against stored public key
  - Updates sign count (detect cloned authenticators)
  - Issues full session with `2fa_verified: true`
  - Logs audit event

### 3.4 Credential Management

- `GET /api/auth/webauthn/credentials` — list user's credentials with metadata
- `DELETE /api/auth/webauthn/credentials/{id}` — remove a credential
  - If last credential removed, set `webauthn_enabled = false`

## Phase 4: Integration & Wiring

### 4.1 Update Router

Wire new route groups into `internal/api/router.go`:
- `/api/auth/*` → auth handlers
- `/api/tokens/*` → token handlers (under customer auth middleware)
- `/api/auth/webauthn/*` → WebAuthn handlers

### 4.2 Update Models

Add to `internal/models/models.go`:
- Registration/login request/response types
- WebAuthn request/response types
- API token types

### 4.3 Config Extensions

Add to `internal/config/config.go`:
- `WebAuthnRPID`, `WebAuthnRPName`, `WebAuthnOrigins`
- `PasswordMinLength`, `BreachCheckEnabled`
- `LoginRateLimitIP`, `LoginRateLimitAccount`
- `AccountLockoutThreshold`, `AccountLockoutDuration`
- `APITokenDefaultExpiry`

### 4.4 Frontend Integration Points

The frontend already expects these via `lib/control-plane-api.ts`. New endpoints must match the existing session model (`SessionContext` with role, displayName, accountId, etc.).

## Dependencies (execution order)

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4
                                    ↑         │
                                    └─────────┘
                               (wiring depends on all phases)
```

Phase 1 and Phase 2 can proceed in parallel after Phase 0.
Phase 3 depends on Phase 1 (users must exist before WebAuthn enrollment).
Phase 4 is integration wiring after all other phases.
