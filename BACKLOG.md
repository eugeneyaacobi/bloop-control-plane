# bloop-control-plane Backlog

## Deferred / Next Up

### Email verification & onboarding
- Add local mail sink for dev/test (MailHog or Mailpit) and wire Docker/local compose to it
- Verify full end-to-end signup email round-trip using captured dev email token
- Decide whether verification tokens should remain SHA-256 hashed only or move to a stronger/structured token strategy
- Add resend-verification flow with safe anti-enumeration behavior
- Add rate limiting around signup/request/verify endpoints

### API hardening
- Add repository/handler/integration tests for customer/admin/onboarding APIs
- Add response safety tests that assert secrets/tokens never leak in serialized payloads
- Add liveness/readiness tests
- Add Dockerfile and full local compose for the control-plane service itself

### Data/runtime integration
- Decide how `bloop-control-plane` will synchronize runtime-derived tunnel state from `bloop-tunnel`
- Add persistent audit/review workflows beyond the initial seed/demo data
- Replace in-memory fallback paths with cleaner explicit dev/test strategies

### Frontend integration follow-up
- Replace mocked selectors in `bloop-frontend` with real API adapters once control-plane endpoints stabilize
- Add auth/session model alignment between frontend roles and backend authorization boundaries
