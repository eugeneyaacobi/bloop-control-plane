# Tasks: Authentication, Token Management & WebAuthn 2FA

**Feature**: `004-auth-tokens-webauthn`
**Created**: 2026-03-30
**Status**: Complete

## Phase 1: Foundation & Database

### T001 — Add Go dependencies
- [x] `cd bloop-control-plane && go get github.com/go-webauthn/webauthn/webauthn`
- [x] `go get golang.org/x/crypto` (for argon2id)
- [x] Verify `go mod tidy` passes
- **Files**: `go.mod`, `go.sum`

### T002 — Create database migration: user_credentials
- [x] Create `deploy/migrations/004_user_credentials.up.sql`
- [x] Table: `user_credentials` (id, user_id FK, password_hash, password_algorithm, created_at, updated_at)
- [x] Add columns to users: `username TEXT UNIQUE`, `password_set BOOLEAN DEFAULT false`
- **Files**: `deploy/migrations/004_user_credentials.up.sql`

### T003 — Create database migrations: WebAuthn tables
- [x] Create `deploy/migrations/005_webauthn_credentials.up.sql`
- [x] Table: `webauthn_credentials` (id, user_id FK, credential_id BYTEA UNIQUE, public_key BYTEA, attestation_type, aaguid BYTEA, sign_count, name, transports TEXT[], last_used_at, created_at)
- [x] Create `deploy/migrations/005_webauthn_challenges.up.sql` (merged): `webauthn_challenges` (id, user_id, challenge BYTEA, kind, expires_at, created_at)
- [x] Add `webauthn_enabled BOOLEAN DEFAULT false` to users
- **Files**: `deploy/migrations/005_webauthn.up.sql`

### T004 — Create database migration: API tokens
- [x] Create `deploy/migrations/006_api_tokens.up.sql`
- [x] Table: `api_tokens` (id, user_id FK, account_id, name, token_hash UNIQUE, token_prefix, expires_at, revoked_at, last_used_at, created_at)
- [x] Index on `(user_id, revoked_at)` for active token lookups
- [x] Index on `token_hash` for authentication lookups
- **Files**: `deploy/migrations/006_api_tokens.up.sql`

### T005 — Create database migrations: Audit & lockout
- [x] Create `deploy/migrations/007_auth_audit.up.sql`
- [x] Table: `auth_audit_log` (id, user_id, account_id, event, ip_address, user_agent, success, metadata JSONB, created_at)
- [x] Create `deploy/migrations/008_login_attempts_lockout.up.sql`
- [x] Table: `login_attempts` (id, identifier, ip_address, success, attempted_at)
- [x] Table: `account_lockouts` (id, user_id UNIQUE, locked_until, failed_count, last_failed_at, locked_by)
- **Files**: `deploy/migrations/007_auth_audit.up.sql`, `deploy/migrations/008_lockout.up.sql`

### T006 — Password hashing module
- [x] Create `internal/security/password.go`
- [x] Implement `HashPassword(password string) (string, error)` using argon2id (time=3, memory=64MB, threads=4, keyLen=32)
- [x] Implement `VerifyPassword(password, hash string) (bool, error)` using constant-time comparison
- [x] Implement `CheckPasswordBreach(password string) (bool, error)` using HIBP k-anonymity (configurable opt-in)
- [x] Write unit tests for hash/verify round-trip and timing safety
- **Files**: `internal/security/password.go`, `internal/security/password_test.go`

### T007 — Auth audit repository
- [x] Create `internal/repository/audit_repository.go`
- [x] Implement `LogAuthEvent(ctx, event)` — insert audit record
- [x] Implement `GetRecentEvents(ctx, userID, limit)` — for admin dash
- **Files**: `internal/repository/audit_repository.go`

### T008 — Lockout repository
- [x] Create `internal/repository/lockout_repository.go`
- [x] Implement `RecordLoginAttempt(ctx, identifier, ip, success)` — insert attempt
- [x] Implement `GetFailedAttemptCount(ctx, identifier, since)` — count recent failures
- [x] Implement `LockAccount(ctx, userID, duration)` — set lockout
- [x] Implement `IsAccountLocked(ctx, userID)` — check lockout status
- [x] Implement `UnlockAccount(ctx, userID)` — clear lockout
- **Files**: `internal/repository/lockout_repository.go`

## Phase 2: Registration & Login

### T009 — Auth repository
- [x] Create `internal/repository/auth_repository.go`
- [x] Implement `CreateUserWithCredentials(ctx, email, username, passwordHash) (User, error)` — transactional user+credential creation
- [x] Implement `GetUserByEmail(ctx, email) (User, error)`
- [x] Implement `GetUserByUsername(ctx, username) (User, error)`
- [x] Implement `GetCredentialsByUserID(ctx, userID) (Credential, error)`
- [x] Implement `UpdatePasswordHash(ctx, userID, newHash) error`
- [x] All queries parameterized, no string interpolation
- **Files**: `internal/repository/auth_repository.go`

### T010 — Auth service
- [x] Create `internal/service/auth_service.go`
- [x] Implement `Register(ctx, email, username, password) (User, Session, error)` — validates password, hashes, creates user, issues session
- [x] Implement `Login(ctx, email, password, ip, userAgent) (Session, requiresWebAuthn bool, error)` — rate limit check, verify credentials, check lockout, audit log
- [x] Implement `RefreshSession(ctx, oldSession) (Session, error)` — validate old session, issue new one
- [x] Ensure no password or token value is ever logged
- [x] Ensure error messages for login failures are generic (no enumeration)
- **Files**: `internal/service/auth_service.go`, `internal/service/auth_service_test.go`

### T011 — Auth API handlers
- [x] Create `internal/api/auth/` directory
- [x] Create `internal/api/auth/routes.go` — Mount function with chi router
- [x] Create `internal/api/auth/register.go` — `POST /api/auth/register` handler
- [x] Create `internal/api/auth/login.go` — `POST /api/auth/login` handler
- [x] Create `internal/api/auth/refresh.go` — `POST /api/auth/refresh` handler
- [x] All handlers extract IP from `X-Forwarded-For` or `RemoteAddr` for audit
- [x] All handlers set session cookie with `HttpOnly`, `Secure`, `SameSite=Strict`
- [x] Rate limiting applied to login endpoint via existing `internal/security/ratelimit.go`
- **Files**: `internal/api/auth/routes.go`, `register.go`, `login.go`, `refresh.go`

### T012 — Add model types for auth
- [x] Add to `internal/models/models.go`:
  - `RegistrationRequest` (email, username, password)
  - `LoginRequest` (email, password)
  - `LoginResponse` (session context + requires_webauthn flag)
  - `RefreshResponse` (new session cookie info)
  - `AuthError` (generic error response)
- **Files**: `internal/models/models.go`

### T013 — Auth handler tests
- [x] Create `internal/api/auth/handler_test.go`
- [x] Test registration success, duplicate email, weak password, missing fields
- [x] Test login success, wrong password, locked account, rate limit
- [x] Test refresh success, expired session, revoked session
- [x] Verify no password/token values in response bodies
- **Files**: `internal/api/auth/handler_test.go`

## Phase 3: API Token Management

### T014 — Token repository
- [x] Create `internal/repository/token_repository.go`
- [x] Implement `CreateToken(ctx, userID, accountID, name, tokenHash, tokenPrefix, expiresAt) (Token, error)`
- [x] Implement `ListTokensByUser(ctx, userID) ([]Token, error)` — excludes expired
- [x] Implement `GetTokenByID(ctx, id) (Token, error)` — ownership check
- [x] Implement `RevokeToken(ctx, id) error`
- [x] Implement `LookupByHash(ctx, hash) (Token, error)` — for relay auth validation
- [x] Implement `UpdateLastUsed(ctx, id) error`
- **Files**: `internal/repository/token_repository.go`

### T015 — Token service
- [x] Create `internal/service/token_service.go`
- [x] Implement `CreateToken(ctx, userID, accountID, name, expiresIn) (Token, plaintextValue, error)` — generates 256-bit random token, stores SHA-256 hash, returns plaintext once
- [x] Implement `ListTokens(ctx, userID) ([]Token, error)`
- [x] Implement `RevokeToken(ctx, userID, tokenID) error` — ownership check
- [x] Implement `RefreshToken(ctx, userID, tokenID) (Token, plaintextValue, error)` — revoke old, create new
- [x] Implement `ValidateToken(ctx, tokenValue) (Token, error)` — for relay/internal use, checks hash/expiry/revocation
- **Files**: `internal/service/token_service.go`, `internal/service/token_service_test.go`

### T016 — Token API handlers
- [x] Create `internal/api/tokens/` directory
- [x] Create `internal/api/tokens/routes.go` — Mount with chi, under customer auth middleware
- [x] Create `internal/api/tokens/create.go` — `POST /api/tokens`
- [x] Create `internal/api/tokens/list.go` — `GET /api/tokens`
- [x] Create `internal/api/tokens/revoke.go` — `POST /api/tokens/{id}/revoke`
- [x] Create `internal/api/tokens/refresh.go` — `POST /api/tokens/{id}/refresh`
- [x] Ensure plaintext token is only in create/refresh response body, never stored or logged
- **Files**: `internal/api/tokens/routes.go`, `create.go`, `list.go`, `revoke.go`, `refresh.go`

### T017 — Token handler tests
- [x] Create `internal/api/tokens/handler_test.go`
- [x] Test create, list, revoke, refresh flows
- [x] Test token value appears exactly once in create response
- [x] Test revoked tokens fail validation
- [x] Test ownership enforcement (user can't revoke another user's token)
- **Files**: `internal/api/tokens/handler_test.go`

## Phase 4: WebAuthn 2FA

### T018 — WebAuthn configuration
- [x] Create `internal/security/webauthn.go`
- [x] Initialize `webauthn.WebAuthn` with config (RP ID, RP Name, Origins)
- [x] Config fields: `WebAuthnRPID`, `WebAuthnRPName`, `WebAuthnOrigins` in `internal/config/`
- [x] User entity adapter: implement `webauthn.User` interface for our user model
- **Files**: `internal/security/webauthn.go`

### T019 — WebAuthn repository
- [x] Create `internal/repository/webauthn_repository.go`
- [x] Implement `StoreCredential(ctx, credential) error`
- [x] Implement `ListCredentialsByUser(ctx, userID) ([]Credential, error)`
- [x] Implement `GetCredentialByID(ctx, id) (Credential, error)`
- [x] Implement `DeleteCredential(ctx, id) error`
- [x] Implement `UpdateSignCount(ctx, id, newCount) error`
- [x] Implement `CreateChallenge(ctx, challenge) error`
- [x] Implement `GetChallenge(ctx, id) (Challenge, error)`
- [x] Implement `DeleteChallenge(ctx, id) error`
- **Files**: `internal/repository/webauthn_repository.go`

### T020 — WebAuthn service
- [x] Create `internal/service/webauthn_service.go`
- [x] Implement `BeginRegistration(ctx, userID) (creationOptions, error)` — generate challenge, store it
- [x] Implement `FinishRegistration(ctx, userID, response) (credential, error)` — verify attestation, store credential, set webauthn_enabled
- [x] Implement `BeginLogin(ctx, email) (requestOptions, error)` — find user, load credentials, generate challenge
- [x] Implement `FinishLogin(ctx, email, response) (session, error)` — verify assertion, check sign count, issue full session with 2fa_verified
- [x] Implement `ListCredentials(ctx, userID) ([]Credential, error)`
- [x] Implement `DeleteCredential(ctx, userID, credentialID) error` — auto-disable if last credential
- **Files**: `internal/service/webauthn_service.go`, `internal/service/webauthn_service_test.go`

### T021 — WebAuthn API handlers
- [x] Create `internal/api/auth/webauthn_register.go` — begin + finish registration
- [x] Create `internal/api/auth/webauthn_login.go` — begin + finish login
- [x] Create `internal/api/auth/webauthn_credentials.go` — list + delete credentials
- [x] All WebAuthn handlers under `/api/auth/webauthn/*` route group
- [x] Registration handlers require authenticated session
- [x] Login begin handler does NOT require session (public endpoint)
- [x] Login finish handler does NOT require session (completes auth)
- **Files**: `internal/api/auth/webauthn_register.go`, `webauthn_login.go`, `webauthn_credentials.go`

### T022 — WebAuthn handler tests
- [x] Create `internal/api/auth/webauthn_test.go`
- [x] Test registration ceremony (begin + finish) with mocked WebAuthn
- [x] Test login ceremony with mocked WebAuthn
- [x] Test credential listing and deletion
- [x] Test auto-disable when last credential removed
- **Files**: `internal/api/auth/webauthn_test.go`

## Phase 5: Router Wiring & Config

### T023 — Update router
- [x] Modify `internal/api/router.go` to wire:
  - `/api/auth/*` → auth handlers (public routes: register, login, refresh, webauthn/begin-login, webauthn/finish-login)
  - `/api/auth/webauthn/*` (authenticated: begin-registration, finish-registration, credentials)
  - `/api/tokens/*` → token handlers (customer auth middleware)
- [x] Add new deps to `RouterDeps`
- **Files**: `internal/api/router.go`

### T024 — Update config
- [x] Add auth-related config fields to `internal/config/config.go`:
  - WebAuthn RP ID, name, origins
  - Password min length, breach check toggle
  - Login rate limits (IP, account)
  - Account lockout threshold and duration
  - API token default expiry
  - Session token expiry, refresh window
- [x] Load from env vars with sensible defaults
- **Files**: `internal/config/config.go`

### T025 — Integration test
- [x] Create `tests/integration/auth_flow_test.go`
- [x] Test full flow: register → login → create API token → list tokens → revoke token → refresh session → logout
- [x] Test WebAuthn flow: login → begin registration → finish registration → logout → begin login → finish login
- [x] Test security: rate limiting, account lockout, revoked tokens rejected, expired sessions rejected
- **Files**: `tests/integration/auth_flow_test.go`

## Dependency Graph

```
T001 ─┬─→ T006 → T010 → T011 → T013
T002 ─┤
T003 ─┤         T009 ──→ T010
T004 ─┼─→ T014 → T015 → T016 → T017
T005 ─┤
T007 ─┼─→ T010, T015, T020
T008 ─┘
T012 ──→ T011, T016
T018 ──→ T020 → T021 → T022
T023 ──→ depends on T011, T016, T021
T024 ──→ T011, T016, T021
T025 ──→ depends on all prior tasks
```

## Parallel Execution

- **Wave 1**: T001, T002, T003, T004, T005, T006, T007, T008, T012, T018 (all independent)
- **Wave 2**: T009, T014, T019, T024 (depend on migrations and foundations)
- **Wave 3**: T010, T015, T020 (depend on repositories)
- **Wave 4**: T011, T016, T021 (depend on services)
- **Wave 5**: T013, T017, T022, T023 (depend on handlers)
- **Wave 6**: T025 (integration, depends on everything)
