# Feature Specification: Authentication, Token Management & WebAuthn 2FA

**Feature Branch**: `004-auth-tokens-webauthn`
**Created**: 2026-03-30
**Status**: Spec
**Input**: bloop-control-plane needs password-based registration/login, WebAuthn 2FA, API token CRUD for relay auth, and token refresh. Frontend and TUI already expect these endpoints.

## Background

The control plane currently authenticates users via email signup + verification token, with a prototype/dev fallback for local development. There is no password-based authentication, no WebAuthn 2FA, no user-facing API token management for relay enrollment, and no token refresh mechanism.

The frontend (`bloop-frontend`) already has session infrastructure, signup forms, and customer/admin shells that call control plane endpoints. The tunnel client (`bloop-tunnel`) already calls `/api/runtime/enroll` but has no UI for managing enrollment tokens. The control plane must close these gaps with security as the top priority.

### Current API Surface (auth-relevant)

| Endpoint | Method | Status |
|---|---|---|
| `GET /api/session/me` | Read | ✅ Exists |
| `POST /api/session/logout` | Write | ✅ Exists |
| `POST /api/onboarding/signup/request` | Write | ✅ Exists |
| `POST /api/onboarding/signup/verify` | Write | ✅ Exists |
| `GET /api/onboarding/steps` | Read | ✅ Exists |
| `POST /api/runtime/enroll` | Write | ✅ Exists |

### Missing API Surface (this spec)

| Endpoint | Method | Purpose |
|---|---|---|
| `POST /api/auth/register` | Write | Username + password registration |
| `POST /api/auth/login` | Write | Password login, returns session |
| `POST /api/auth/refresh` | Write | Refresh session token |
| `POST /api/auth/webauthn/begin-registration` | Write | Start WebAuthn credential enrollment |
| `POST /api/auth/webauthn/finish-registration` | Write | Complete WebAuthn credential enrollment |
| `POST /api/auth/webauthn/begin-login` | Write | Start WebAuthn authentication |
| `POST /api/auth/webauthn/finish-login` | Write | Complete WebAuthn authentication |
| `GET /api/auth/webauthn/credentials` | Read | List user's WebAuthn credentials |
| `DELETE /api/auth/webauthn/credentials/{id}` | Write | Remove a WebAuthn credential |
| `GET /api/tokens` | Read | List user's API tokens |
| `POST /api/tokens` | Write | Create a new API token |
| `POST /api/tokens/{id}/revoke` | Write | Revoke an API token |
| `POST /api/tokens/{id}/refresh` | Write | Refresh an API token |

## Security Requirements

These are non-negotiable. Every implementation decision must satisfy them.

1. **Password hashing**: bcrypt with cost ≥ 12 (or argon2id). Never store plaintext passwords.
2. **Password requirements**: Minimum 12 characters. Check against Have I Been Pwned breach corpus (k-anonymity API) or a local breach list. No maximum length limit (hash handles any length).
3. **Timing-safe comparisons**: All token and credential comparisons must be constant-time.
4. **Rate limiting**: Login attempts rate-limited per IP and per account. Progressive backoff after 5 failed attempts. Account lockout after 20 failed attempts in 1 hour (with admin unlock).
5. **Session tokens**: Cryptographically random, ≥ 256 bits, HMAC-signed with server secret, versioned for revocation.
6. **API tokens**: Cryptographically random, ≥ 256 bits, stored as SHA-256 hash. Displayed once at creation.
7. **WebAuthn**: Use go-webauthn library. Store credential public keys, not private keys. AAGUID allowlisting for known authenticators. Require user verification (not just presence).
8. **CORS**: Restrict to known frontend origins. No wildcard in production.
9. **Input validation**: All inputs validated and sanitized. No SQL injection vectors. Parameterized queries only.
10. **Audit logging**: Log all auth events (login, logout, token creation, token revocation, 2FA enable/disable, failed attempts) with timestamp, IP, user agent.
11. **No secrets in logs**: Never log passwords, tokens, or session cookies.
12. **HTTPS only**: All auth endpoints reject plain HTTP in production.

## User Scenarios & Testing

### User Story 1 - Register with Username & Password (Priority: P1)

A new user visits the website, fills in username, email, and password. The control plane creates their account with a hashed password and issues a session.

**Why P1**: Without registration, no other auth flow works. This is the onboarding entry point.

**Independent Test**: POST valid registration, verify 201, confirm password is hashed in DB, confirm session cookie is set.

**Acceptance Scenarios**:
1. **Given** no existing account with the email, **When** user POSTs `/api/auth/register` with `{email, username, password}`, **Then** API returns 201 with session cookie, password is stored as bcrypt/argon2id hash, never plaintext.
2. **Given** an account already exists with that email, **When** user POSTs `/api/auth/register`, **Then** API returns 409 without revealing whether email or username is taken (generic "registration failed" to prevent enumeration).
3. **Given** a password under 12 characters, **When** user POSTs `/api/auth/register`, **Then** API returns 422 with validation error.
4. **Given** a password found in known breach databases, **When** user POSTs `/api/auth/register`, **Then** API returns 422 with warning about compromised password.
5. **Given** valid registration, **When** the response is returned, **Then** the password is not present in any log output or response body.

---

### User Story 2 - Login with Password (Priority: P1)

A returning user enters email and password. The control plane verifies credentials and issues a session.

**Why P1**: Core authentication. Required for all protected endpoints.

**Independent Test**: Register a user, then POST `/api/auth/login`, verify 200 with session cookie.

**Acceptance Scenarios**:
1. **Given** a registered user, **When** they POST `/api/auth/login` with correct credentials, **Then** API returns 200 with session cookie and user context.
2. **Given** a registered user, **When** they POST with wrong password, **Then** API returns 401 with generic "invalid credentials" (no hint about which field is wrong).
3. **Given** 5 failed login attempts for an account within 15 minutes, **When** the 6th attempt is made, **Then** API returns 429 with retry-after header.
4. **Given** 20 failed attempts in 1 hour, **When** any login attempt is made, **Then** account is locked and API returns 423.
5. **Given** a user with WebAuthn enrolled, **When** they login with password, **Then** session has `requires_webauthn: true` flag until 2FA is completed.

---

### User Story 3 - Enable WebAuthn 2FA (Priority: P2)

An authenticated user wants to add a security key (YubiKey, etc.) as a second factor.

**Why P2**: Important for security but users must first register and login with password.

**Independent Test**: Complete begin/finish registration flow, verify credential stored in DB, verify user's `webauthn_enabled` flag is set.

**Acceptance Scenarios**:
1. **Given** an authenticated user without WebAuthn, **When** they POST `/api/auth/webauthn/begin-registration`, **Then** API returns a PublicKeyCredentialCreationOptions challenge.
2. **Given** a valid challenge, **When** user POSTs `/api/auth/webauthn/finish-registration` with signed assertion, **Then** API stores credential public key, returns 200 with credential metadata.
3. **Given** a user with WebAuthn enabled, **When** they login with password, **Then** they must complete WebAuthn verification before getting a full session.
4. **Given** a user with multiple WebAuthn credentials, **When** they list credentials, **Then** API returns all credentials with names, dates, and last-used timestamps.
5. **Given** a user with WebAuthn credentials, **When** they delete the last credential, **Then** `webauthn_enabled` is set to false automatically.

---

### User Story 4 - Login with WebAuthn (Priority: P2)

A user with WebAuthn enrolled authenticates using their security key.

**Why P2**: Depends on P2 registration, but enables passwordless login.

**Independent Test**: Complete begin/finish login flow, verify session issued.

**Acceptance Scenarios**:
1. **Given** a user with WebAuthn enrolled, **When** they POST `/api/auth/webauthn/begin-login` with their email, **Then** API returns PublicKeyCredentialRequestOptions with appropriate credentials.
2. **Given** a valid challenge, **When** user POSTs `/api/auth/webauthn/finish-login` with signed assertion, **Then** API verifies signature, issues full session, returns 200.
3. **Given** an invalid assertion, **When** finish-login is called, **Then** API returns 401, increments failed attempt counter.
4. **Given** a successful WebAuthn login, **When** the session is issued, **Then** it has `auth_method: "webauthn"` and `2fa_verified: true`.

---

### User Story 5 - Manage API Tokens (Priority: P1)

A user creates, lists, and revokes API tokens used to authenticate tunnel clients with the relay.

**Why P1**: Without token management, tunnels cannot authenticate to the relay. This is the bridge between the control plane and the tunnel infrastructure.

**Independent Test**: Create a token, verify it appears in list, revoke it, verify it no longer appears as active.

**Acceptance Scenarios**:
1. **Given** an authenticated user, **When** they POST `/api/tokens` with `{name: "my-tunnel", expires_in: "720h"}`, **Then** API returns 201 with the full token value (shown once only), and stores only the SHA-256 hash.
2. **Given** an authenticated user, **When** they GET `/api/tokens`, **Then** API returns list of tokens with id, name, created_at, last_used_at, expires_at, but never the token value.
3. **Given** an authenticated user, **When** they POST `/api/tokens/{id}/revoke`, **Then** API sets revoked_at timestamp, returns 200.
4. **Given** an authenticated user, **When** they POST `/api/tokens/{id}/refresh`, **Then** API creates a new token with the same name, revokes the old one, returns the new token value.
5. **Given** a revoked token, **When** it is used to authenticate, **Then** authentication fails with 401.
6. **Given** a token that was displayed at creation, **When** the user tries to retrieve it later, **Then** it is not available (only the hash is stored).

---

### User Story 6 - Refresh Session Token (Priority: P1)

A user's session is nearing expiry. The client sends a refresh request to get a new session token without requiring re-authentication.

**Why P1**: Required for any long-lived session. Without refresh, users are constantly logged out.

**Independent Test**: Login, wait (or mock time), POST `/api/auth/refresh` with session cookie, verify new session token issued.

**Acceptance Scenarios**:
1. **Given** a valid non-expired session, **When** user POSTs `/api/auth/refresh`, **Then** API returns new session cookie with extended expiry.
2. **Given** an expired session, **When** user POSTs `/api/auth/refresh`, **Then** API returns 401 (must re-authenticate).
3. **Given** a revoked session version, **When** user POSTs `/api/auth/refresh`, **Then** API returns 401.
4. **Given** a successful refresh, **When** the old session token is used again, **Then** it still works until natural expiry (refresh doesn't invalidate old token, just issues new one).

---

### User Story 7 - Tunnel Endpoint Management from TUI (Priority: P1)

A user manages their tunnel endpoints from the bloop-tunnel TUI, which calls the control plane API. This extends existing CRUD (already implemented) to include token assignment and relay configuration.

**Why P1**: The TUI is the primary interface. Token assignment to tunnels is the missing link.

**Independent Test**: Create a tunnel via API, assign an API token to it, verify the tunnel config includes the token reference.

**Acceptance Scenarios**:
1. **Given** a user with API tokens and tunnels, **When** they update a tunnel to use an API token, **Then** the tunnel config stores a reference to the token (not the token value itself).
2. **Given** a tunnel with an assigned API token that is revoked, **When** the tunnel status is checked, **Then** status shows "credential_revoked" warning.
3. **Given** the TUI lists available tokens, **When** the user selects one for a tunnel, **Then** only non-expired, non-revoked tokens are shown.

## Data Model Changes

### New Tables

```sql
-- User credentials (password auth)
CREATE TABLE user_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    password_algorithm TEXT NOT NULL DEFAULT 'argon2id',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- WebAuthn credentials
CREATE TABLE webauthn_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    attestation_type TEXT NOT NULL,
    aaguid BYTEA,
    sign_count BIGINT NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT 'Security Key',
    transports TEXT[],
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- WebAuthn challenges (ephemeral)
CREATE TABLE webauthn_challenges (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    challenge BYTEA NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('registration', 'authentication')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API tokens for relay auth
CREATE TABLE api_tokens (
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

-- Auth audit log
CREATE TABLE auth_audit_log (
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

-- Login rate limiting (account-level)
CREATE TABLE login_attempts (
    id TEXT PRIMARY KEY,
    identifier TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Account lockout state
CREATE TABLE account_lockouts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    locked_until TIMESTAMPTZ,
    failed_count INTEGER NOT NULL DEFAULT 0,
    last_failed_at TIMESTAMPTZ,
    locked_by TEXT
);
```

### Modified Tables

```sql
-- Add to existing users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS webauthn_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_set BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ;
```

## Dependencies

- `github.com/go-webauthn/webauthn` — WebAuthn server-side implementation
- `golang.org/x/crypto` — argon2id password hashing
- Existing `internal/session` package for token management
- Existing `internal/repository` for database access
- Existing `internal/security` for rate limiting

## Out of Scope

- OAuth2 / social login (future consideration)
- Password reset flow (separate spec, but architecture should accommodate it)
- Email-based 2FA / TOTP (WebAuthn is the preferred 2FA method)
- Admin-level user management CRUD (partially exists, extend separately)
- Token scopes/permissions (all tokens are full-scope for now; scope model later)
