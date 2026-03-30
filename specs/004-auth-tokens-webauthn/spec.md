# Feature Specification: Authentication, Token Management & WebAuthn 2FA

**Feature Branch**: `fix/registration-error-message`
**Created**: 2026-03-30
**Status**: ✅ Implemented
**Merged via**: PR #2 (auth system), PR #3 (fixes + migration embedding)

## Implementation Summary

All routes shipped and tested. 45+ tests, 7,600+ lines, 45 files.

## API Surface (as implemented)

| Endpoint | Method | Status |
|---|---|---|
| `POST /api/auth/register` | Write | ✅ Done |
| `POST /api/auth/login` | Write | ✅ Done |
| `POST /api/auth/refresh` | Write | ✅ Done |
| `POST /api/webauthn/register-begin` | Write | ✅ Done |
| `POST /api/webauthn/register-finish` | Write | ✅ Done |
| `POST /api/webauthn/login-begin` | Write | ✅ Done |
| `POST /api/webauthn/login-finish` | Write | ✅ Done |
| `GET /api/webauthn/credentials` | Read | ✅ Done |
| `DELETE /api/webauthn/credentials/{id}` | Write | ✅ Done |
| `GET /api/tokens` | Read | ✅ Done |
| `POST /api/tokens` | Write | ✅ Done |
| `DELETE /api/tokens/{id}` | Write | ✅ Done |
| `POST /api/tokens/{id}/refresh` | Write | ✅ Done |
| `GET /healthz` | Read | ✅ Done |
| `GET /readyz` | Read | ✅ Done |
| `GET /metricsz` | Read | ✅ Done |

**Note**: WebAuthn routes are under `/api/webauthn/` (not `/api/auth/webauthn/`) for cleaner routing. Token revocation uses `DELETE /api/tokens/{id}` (RESTful) instead of `POST /revoke`.

## Security Checklist

- [x] Argon2id password hashing (OWASP params: t=3, m=64MB, p=4, key=32)
- [x] Minimum 12 character passwords
- [x] HIBP breach check (configurable via `BREACH_CHECK_ENABLED`)
- [x] Constant-time comparisons (subtle.ConstantTimeCompare)
- [x] Rate limiting: per-IP (10/min) and per-account (5/15min)
- [x] Account lockout after 20 failed attempts (1h lockout)
- [x] HMAC-SHA256 signed session tokens with version-based revocation
- [x] API tokens: stored as SHA-256 hash, displayed once at creation
- [x] WebAuthn via go-webauthn library, user verification required
- [x] CORS middleware (configurable via `CORS_ALLOWED_ORIGINS`)
- [x] Input validation on all endpoints
- [x] Audit logging to `auth_audit_log` table
- [x] No secrets in logs (Redact helper)
- [x] HttpOnly, Secure, SameSite=Strict session cookies
- [x] Request logging middleware (method, path, status, duration)
- [x] Panic recovery middleware

## Infrastructure

- [x] Embedded migrations via `go:embed` (no filesystem dependency)
- [x] Idempotent migration tracking (`schema_migrations` table)
- [x] Structured JSON logging (slog)
- [x] Request metrics endpoint (`/metricsz`: total + active requests)
- [x] Health + readiness probes

## DB Migrations (004–008)

- `004_user_credentials.up.sql`
- `005_webauthn.up.sql`
- `006_api_tokens.up.sql`
- `007_auth_audit.up.sql`
- `008_lockout.up.sql`

## Environment Variables

- `SESSION_SECRET` — HMAC signing key (required in production)
- `PASSWORD_MIN_LENGTH` — default 12
- `BREACH_CHECK_ENABLED` — default false
- `LOGIN_RATE_LIMIT_IP` — default 10
- `LOGIN_RATE_LIMIT_ACCOUNT` — default 5
- `ACCOUNT_LOCKOUT_THRESHOLD` — default 20
- `ACCOUNT_LOCKOUT_DURATION` — default 1h
- `API_TOKEN_DEFAULT_EXPIRY` — default 720h
- `CORS_ALLOWED_ORIGINS` — comma-separated origins
- `WEBAUTHN_RP_ID` — default bloop.to
- `WEBAUTHN_RP_NAME` — default Bloop
- `WEBAUTHN_ORIGINS` — comma-separated

## Out of Scope (future)

- OAuth2 / social login
- Password reset flow
- Email-based 2FA / TOTP
- Token scopes/permissions
- Admin-level user management CRUD
