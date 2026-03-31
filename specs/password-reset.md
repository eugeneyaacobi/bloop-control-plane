# Password Reset — Feature Spec

## Overview

Add a secure password reset flow across bloop-control-plane (backend) and bloop-frontend (frontend). When a user tries to register with an existing email, they should be offered the option to reset their password. A dedicated "Forgot password?" link should also exist on the login page.

## Security Requirements (Non-Negotiable)

1. **Single-use, time-limited tokens** — Reset tokens expire after 15 minutes. Each token is invalidated immediately after use (or on any new reset request for the same email).
2. **No user enumeration** — The "request reset" endpoint always returns 200, whether the email exists or not. The response message is identical. The email is sent only if the account exists; otherwise, silently succeed.
3. **Rate limiting** — Max 5 reset requests per email per hour. Max 20 per IP per hour. Exceeding returns 429.
4. **Breach check on new password** — New passwords must pass the same validation as registration (≥12 chars, HIBP breach check if enabled).
5. **Audit logging** — Every reset request, token validation, and password change is logged to `auth_audit` with IP, user agent, and outcome.
6. **Token entropy** — Tokens are cryptographically random, 32 bytes, base64url-encoded (43 chars). Stored as SHA-256 hash in DB (like verification tokens).
7. **No session issued on reset** — Completing a password reset does NOT log the user in. They must proceed to the login page.
8. **Invalidate all sessions** — When a password is reset, all existing sessions for that user should be invalidated (or flagged for re-auth). For now: log the event; session invalidation can be phase 2.

## Backend (bloop-control-plane)

### New Migration: `010_password_reset.up.sql`

```sql
CREATE TABLE password_reset_tokens (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address TEXT,
    user_agent TEXT
);

CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash) WHERE used_at IS NULL;
```

### New API Endpoints

#### `POST /api/auth/forgot-password`
- **Input:** `{ "email": "string" }`
- **Behavior:**
  - Look up user by email. If not found, return 200 anyway (no enumeration).
  - If found: generate a 32-byte random token, hash it (SHA-256), store in `password_reset_tokens` with 15-min expiry.
  - Invalidate any previous unused reset tokens for this user.
  - Send email with reset link containing the raw token.
  - Rate limit: 5/email/hour, 20/IP/hour.
- **Response (always):** `{ "message": "If an account with that email exists, a reset link has been sent." }`
- **Status:** 200

#### `POST /api/auth/reset-password`
- **Input:** `{ "token": "string", "new_password": "string" }`
- **Behavior:**
  - Hash the provided token (SHA-256), look up in `password_reset_tokens` where `used_at IS NULL` and `expires_at > now()`.
  - If not found or expired: return 400 with generic "invalid or expired token".
  - Validate new password (same rules as registration: ≥12 chars, breach check).
  - Update user's password hash in `user_credentials`.
  - Mark token as `used_at = now()`.
  - Audit log the event.
- **Response:** `{ "message": "Password has been reset. Please log in." }`
- **Status:** 200

### New Files

- `internal/api/auth/forgot_password.go` — handler
- `internal/api/auth/reset_password.go` — handler
- `internal/service/password_reset_service.go` — business logic
- `internal/repository/password_reset_repository.go` — DB access + interface
- `internal/db/migrations/010_password_reset.up.sql` — migration
- `internal/service/email_service.go` — add `SendPasswordResetEmail` method

### Existing Files to Modify

- `internal/api/auth/routes.go` — add `r.Post("/forgot-password", ...)` and `r.Post("/reset-password", ...)`
- `internal/config/config.go` — add `PasswordResetTokenTTL` (default 15m), `PasswordResetRateLimitEmail`, `PasswordResetRateLimitIP`
- `internal/service/email_service.go` — add reset email method

## Frontend (bloop-frontend)

### New Pages

#### `/forgot-password` — `app/(auth)/forgot-password/page.tsx`
- Simple form: email input + "Send reset link" button
- On submit: POST to `/api/auth/forgot-password`
- Always shows success message (no enumeration)
- Link from login page ("Forgot password?")

#### `/reset-password` — `app/(auth)/reset-password/page.tsx`
- Reads `?token=...` from URL
- Form: new password + confirm password
- On submit: POST to `/api/auth/reset-password`
- On success: redirect to `/login` with flash message "Password reset. Please sign in."
- On invalid/expired token: show error with link to request a new one

### New API Routes

#### `/api/auth/forgot-password/route.ts`
- Proxies to `POST ${CP_BASE}/api/auth/forgot-password`

#### `/api/auth/reset-password/route.ts`
- Proxies to `POST ${CP_BASE}/api/auth/reset-password`

### Existing Files to Modify

- `app/(auth)/login/page.tsx` — add "Forgot password?" link
- `app/(auth)/register/page.tsx` — when registration fails with "registration failed" (email exists), show option: "Already have an account? Reset your password"
- Register page: detect the email-exists case and offer a reset link inline

## UX Flow

1. **Login page:** "Forgot password?" link → `/forgot-password`
2. **Register page:** If email exists → inline message: "An account with this email already exists. Would you like to reset your password?" with link to `/forgot-password`
3. **Forgot password page:** Enter email → "Check your email" confirmation
4. **Email:** Contains link to `/reset-password?token=<raw_token>`
5. **Reset password page:** Enter new password → success → redirect to login

## Testing

### Backend Tests
- Unit: `password_reset_service_test.go` — token generation, expiry, rate limiting, validation
- Integration: `auth_flow_test.go` — add reset flow to existing auth test suite
- Test cases: valid reset, expired token, reused token, rate-limited email, rate-limited IP, non-existent email (200), weak new password rejected

### Frontend Tests
- E2E or manual: full flow from forgot-password → email → reset → login

## Implementation Order

1. Backend migration
2. Backend repository + service
3. Backend handlers + routes
4. Backend tests
5. Frontend API proxy routes
6. Frontend pages
7. Frontend integration (login/register page updates)
8. End-to-end testing
