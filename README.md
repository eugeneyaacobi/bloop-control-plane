# bloop-control-plane

Control-plane API/service for the bloop platform.

## Responsibilities

- customer/admin API
- Postgres-backed product state
- onboarding state
- signup verification plumbing
- audit logging
- risk/review visibility
- bridge between bloop frontend and bloop tunnel runtime

## Auth model

Normal app access now uses a lightweight signed session token instead of trusting ad hoc identity headers.

Accepted session inputs:
- `Authorization: Bearer <token>`
- `bloop_session` cookie by default (`SESSION_COOKIE_NAME` can override it)

Token claims are HMAC-SHA256 signed and include:
- user id
- account id for non-admin sessions
- role
- expiry (`exp`)
- version/kind metadata

Prototype fallback is still available, but only when explicitly enabled:
- `PROTOTYPE_MODE=true` and/or `ALLOW_DEV_AUTH_FALLBACK=true`
- when enabled, unauthenticated requests fall back to the seeded prototype session
- when disabled, unauthenticated requests get `401 Unauthorized`

## Auth & Security

Full authentication system with password-based auth, WebAuthn 2FA, API token management, and session handling.

### Auth Endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/auth/register` | Create account (email + username + password) |
| `POST /api/auth/login` | Password login, returns session cookie |
| `POST /api/auth/refresh` | Refresh session token |
| `GET /api/session/me` | Get current session |
| `POST /api/session/logout` | Clear session |

### WebAuthn 2FA

| Endpoint | Purpose |
|---|---|
| `POST /api/webauthn/register-begin` | Start credential enrollment |
| `POST /api/webauthn/register-finish` | Complete credential enrollment |
| `POST /api/webauthn/login-begin` | Start WebAuthn login |
| `POST /api/webauthn/login-finish` | Complete WebAuthn login |
| `GET /api/webauthn/credentials` | List user's credentials |
| `DELETE /api/webauthn/credentials/{id}` | Remove a credential |

### API Tokens (for tunnel relay auth)

| Endpoint | Purpose |
|---|---|
| `POST /api/tokens` | Create token (shown once) |
| `GET /api/tokens` | List tokens |
| `DELETE /api/tokens/{id}` | Revoke token |
| `POST /api/tokens/{id}/refresh` | Rotate token |

### Infrastructure Endpoints

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Health check |
| `GET /readyz` | Readiness check |
| `GET /metricsz` | Request counters |

### Security Features

- Argon2id password hashing (OWISP params)
- Rate limiting (per-IP + per-account)
- Account lockout after 20 failed attempts
- HMAC-SHA256 signed session tokens
- API tokens stored as SHA-256 hashes (never retrievable after creation)
- HIBP breach check (configurable)
- Audit logging to database
- CORS middleware (configurable origins)
- Request logging (structured JSON)

## Required / relevant environment variables

- `DATABASE_URL` — Postgres connection string
- `LISTEN_ADDR` — HTTP listen address, default `:8081`
- `SESSION_SECRET` — required when `ALLOW_DEV_AUTH_FALLBACK=false`; used to verify signed session tokens
- `SESSION_COOKIE_NAME` — optional cookie name, default `bloop_session`
- `SESSION_COOKIE_SECURE` — cookie secure flag, defaults to `true` outside prototype mode
- `SESSION_COOKIE_DOMAIN` — optional cookie domain
- `SESSION_TTL_SECONDS` — signed session TTL, default 7 days
- `PROTOTYPE_MODE` — explicit prototype/dev mode switch
- `ALLOW_DEV_AUTH_FALLBACK` — allows seeded prototype session fallback for local/dev work
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`
- `VERIFICATION_TOKEN_TTL_SECONDS`

### Auth & Security

- `PASSWORD_MIN_LENGTH` — minimum password length, default 12
- `BREACH_CHECK_ENABLED` — enable HIBP breach check, default false
- `LOGIN_RATE_LIMIT_IP` — max login attempts per IP per minute, default 10
- `LOGIN_RATE_LIMIT_ACCOUNT` — max login attempts per account per 15 min, default 5
- `ACCOUNT_LOCKOUT_THRESHOLD` — failed attempts before lockout, default 20
- `ACCOUNT_LOCKOUT_DURATION` — lockout duration, default 1h
- `API_TOKEN_DEFAULT_EXPIRY` — default token lifetime, default 720h
- `CORS_ALLOWED_ORIGINS` — comma-separated allowed origins
- `WEBAUTHN_RP_ID` — WebAuthn relying party ID, default bloop.to
- `WEBAUTHN_RP_NAME` — WebAuthn relying party name, default Bloop
- `WEBAUTHN_ORIGINS` — comma-separated WebAuthn allowed origins

See `.env.example` for a local starting point.

## Local development

1. Start dependencies:
   - `docker compose -f deploy/compose/local.yml up -d`
2. Export env vars from `.env.example` or your shell.
3. Run the service:
   - `go run ./cmd/bloop-control-plane`
4. Hit health endpoints:
   - `curl http://localhost:8081/healthz`
   - `curl http://localhost:8081/readyz`

### Dev auth examples

Prototype mode enabled:
- `curl http://localhost:8081/api/session/me`

Signed bearer token:
- send `Authorization: Bearer <signed-token>`
- or set the configured session cookie

The repo now issues signed sessions after successful signup verification and exposes `POST /api/session/logout` to clear the session cookie. Full login/re-auth remains a future slice.

## Docker

A fuller local stack is available via:
- `docker compose -f deploy/compose/dev-full.yml up --build`

## Open-source / public release notes

This repo now includes:
- GitHub Actions CI at `.github/workflows/ci.yml`
- production stack posture in `deploy/compose/v1-stack.yml`
- release guidance in `/root/.openclaw/workspace/BLOOP_PRODUCTION_RELEASE_RUNBOOK.md`
- release checklist in `/root/.openclaw/workspace/BLOOP_RELEASE_CHECKLIST.md`
- automatic patch releases on successful `main` CI via `.github/workflows/auto-release.yml`

Versioning / release policy:
- tags use semver: `vMAJOR.MINOR.PATCH`
- automation starts at `v0.1.0`
- every successful CI run on `develop` creates the next prerelease tag (`-rc.N`)
- every successful CI run on `main` creates the next stable patch release
- recommended flow: feature branch -> PR to `develop` -> validate prerelease -> promote to `main`

For AI agents / automation:
- validate with `go test ./...`
- deploy with `docker compose -f deploy/compose/v1-stack.yml up -d --build`
- verify `/healthz` and `/readyz`
- ensure `PROTOTYPE_MODE=false` and `ALLOW_DEV_AUTH_FALLBACK=false`

## Runtime ingest dev integration

A reproducible local end-to-end proof now exists for:
- control-plane + postgres + mailpit
- relay + client + local echo target
- runtime ingest into control-plane
- customer workspace runtime summary reflecting ingested truth in prototype/dev mode

Run from this repo:
- `make dev-runtime-e2e`

Tear down:
- `make dev-runtime-e2e-down`

Notes:
- control-plane listens on host `localhost:38081`
- Postgres is exposed on host `localhost:35432`
- the relay/client stack is started from the bloop-tunnel repo via `deploy/compose/dev-relay-ingest.yml`
- in prototype/dev mode, runtime ingest normalizes unknown relay session account ids onto `acct_default` so the local customer workspace can reflect live ingested data

## Status

Auth system, token management, WebAuthn 2FA, and tunnel CRUD shipped. See `specs/004-auth-tokens-webauthn/spec.md` for details.
