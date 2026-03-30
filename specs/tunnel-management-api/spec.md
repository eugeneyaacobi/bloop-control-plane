# Feature Specification: Tunnel Management API

**Feature Branch**: `007-tunnel-management-api`
**Created**: 2026-03-30
**Status**: Draft
**Input**: bloop-tunnel TUI client needs full CRUD API for tunnel lifecycle management, enrollment, and config validation against bloop-control-plane.

## Background

The bloop-tunnel CLI/TUI client can currently discover Docker containers and configure tunnels locally. However, the control plane API is read-only for tunnels (`GET /api/customer/tunnels`, `GET /api/customer/tunnels/{id}`). The TUI needs to create, update, and delete tunnels through the control plane so state is durable and consistent across relay runtime ingestion.

### Current API Surface

| Endpoint | Method | Status |
|---|---|---|
| `GET /api/customer/workspace` | Read | ✅ Exists |
| `GET /api/customer/tunnels` | Read | ✅ Exists |
| `GET /api/customer/tunnels/{id}` | Read | ✅ Exists |
| `POST /api/runtime/enroll` | Write | ✅ Exists |
| `POST /api/runtime/ingest/snapshot` | Write | ✅ Exists |

### Missing API Surface (this spec)

| Endpoint | Method | Purpose |
|---|---|---|
| `POST /api/customer/tunnels` | Create | Register new tunnel config |
| `PUT /api/customer/tunnels/{id}` | Update | Modify tunnel config |
| `DELETE /api/customer/tunnels/{id}` | Delete | Remove tunnel config |
| `POST /api/customer/tunnels/validate` | Validate | Check hostname availability & config |
| `GET /api/customer/tunnels/{id}/status` | Read | Real-time tunnel health from runtime |
| `GET /api/customer/config/schema` | Read | Config defaults & validation rules |
| `POST /api/customer/enrollment/verify` | Write | Verify enrollment token & claim installation |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a Tunnel via TUI (Priority: P1)

User configures a new tunnel in the TUI form (name, hostname, local address, access control). On save, the TUI calls `POST /api/customer/tunnels` to register the tunnel in the control plane. The tunnel appears in the workspace immediately.

**Why this priority**: Without tunnel creation, the entire TUI form flow is dead. This is the primary write path.

**Independent Test**: POST a valid tunnel payload, verify 201 response, GET the tunnel to confirm persistence.

**Acceptance Scenarios**:

1. **Given** user has an authenticated session, **When** they POST a valid tunnel config (name="webapp", hostname="webapp.bloop.to", target="localhost:8080", access="public"), **Then** the API returns 201 with the created tunnel including server-assigned ID and status "pending".
2. **Given** user has an authenticated session, **When** they POST a tunnel with a hostname already claimed by another account, **Then** the API returns 409 Conflict with a descriptive error.
3. **Given** user has an authenticated session, **When** they POST a tunnel with missing required fields, **Then** the API returns 422 with field-level validation errors.

---

### User Story 2 - Update a Tunnel via TUI (Priority: P1)

User edits an existing tunnel (e.g., changes access control from "public" to "basic_auth"). The TUI calls `PUT /api/customer/tunnels/{id}` to persist the change.

**Why this priority**: Edit flow is essential — users will iterate on configs. Same priority as create since both are core CRUD.

**Independent Test**: PUT an updated tunnel payload, verify 200 response, GET to confirm changes persisted.

**Acceptance Scenarios**:

1. **Given** a tunnel exists with id "api", **When** user PUTs updated config changing access from "public" to "basic_auth" with credentials, **Then** the API returns 200 with the updated tunnel.
2. **Given** a tunnel exists, **When** user PUTs a change to a hostname that's already taken, **Then** the API returns 409 Conflict.
3. **Given** a tunnel does not exist, **When** user PUTs to `/api/customer/tunnels/nonexistent`, **Then** the API returns 404.

---

### User Story 3 - Delete a Tunnel via TUI (Priority: P1)

User selects a tunnel in the TUI list and confirms deletion. The TUI calls `DELETE /api/customer/tunnels/{id}`.

**Why this priority**: Completes the CRUD triad. Without delete, stale tunnels accumulate.

**Independent Test**: DELETE an existing tunnel, verify 204, GET returns 404.

**Acceptance Scenarios**:

1. **Given** a tunnel exists with id "api", **When** user sends DELETE, **Then** the API returns 204 No Content and subsequent GET returns 404.
2. **Given** a tunnel does not exist, **When** user sends DELETE, **Then** the API returns 404.
3. **Given** a tunnel is actively connected in runtime, **When** user sends DELETE, **Then** the API returns 204 but also emits a runtime event to disconnect the tunnel (fire-and-forget).

---

### User Story 4 - Validate Tunnel Config Before Save (Priority: P2)

As user fills in the tunnel form, the TUI calls `POST /api/customer/tunnels/validate` to check hostname availability and config validity before submission.

**Why this priority**: Better UX than getting rejected on submit, but not blocking for basic CRUD.

**Independent Test**: POST partial config, verify validation response with field-level feedback.

**Acceptance Scenarios**:

1. **Given** hostname "api.bloop.to" is available, **When** user validates config with that hostname, **Then** API returns 200 with `{valid: true}`.
2. **Given** hostname "api.bloop.to" is taken, **When** user validates config with that hostname, **Then** API returns 200 with `{valid: false, errors: [{field: "hostname", message: "already claimed"}]}`.
3. **Given** local port is 99999 (invalid), **When** user validates, **Then** API returns 200 with `{valid: false, errors: [{field: "localPort", message: "must be between 1-65535"}]}`.

---

### User Story 5 - Check Tunnel Runtime Status (Priority: P2)

User views the tunnels list and sees live status (connected, degraded, disconnected) from runtime ingestion data.

**Why this priority**: Enhances the tunnels list significantly but list works without it (just shows config-level status).

**Independent Test**: After runtime ingest, GET tunnel status returns runtime-derived health.

**Acceptance Scenarios**:

1. **Given** runtime has ingested a snapshot showing tunnel "api" as healthy, **When** user GETs `/api/customer/tunnels/api/status`, **Then** API returns `{status: "healthy", observedAt: "...", degraded: false}`.
2. **Given** no runtime data exists for a tunnel, **When** user GETs status, **Then** API returns `{status: "unknown", stale: true}`.

---

### User Story 6 - Get Config Defaults & Schema (Priority: P3)

TUI fetches config schema to populate defaults (allowed access modes, default relay URL, hostname patterns).

**Why this priority**: Nice-to-have for TUI polish. Hardcoded defaults work for v1.

**Independent Test**: GET schema returns expected defaults.

**Acceptance Scenarios**:

1. **Given** the API is running, **When** user GETs `/api/customer/config/schema`, **Then** API returns access modes, hostname patterns, default ports, and relay URL.

---

### User Story 7 - Verify Enrollment Token (Priority: P2)

TUI sends enrollment token to control plane to verify it's valid and claim the installation. Returns installation ID and ingest token for runtime reporting.

**Why this priority**: Needed for the TUI verification screen. Without it, enrollment is fire-and-forget with no client-side confirmation.

**Independent Test**: POST valid enrollment token, verify installation is claimed.

**Acceptance Scenarios**:

1. **Given** a valid enrollment token, **When** user POSTs to `/api/customer/enrollment/verify`, **Then** API returns `{valid: true, installationId: "...", ingestToken: "..."}`.
2. **Given** an expired/invalid enrollment token, **When** user POSTs, **Then** API returns 401 with `{valid: false, error: "invalid_or_expired"}`.

---

### Edge Cases

- What happens when two users try to claim the same hostname simultaneously? → First write wins, second gets 409.
- What happens when a tunnel is deleted while runtime is actively ingesting snapshots for it? → Ingest still accepts (tunnel may be re-created), but snapshot references a soft-deleted tunnel.
- What happens when the DB is temporarily unavailable? → Return 503, client should retry with backoff.
- What happens when a user tries to create a tunnel with access="basic_auth" but doesn't provide credentials? → 422 validation error.
- What happens with hostname normalization (case sensitivity, trailing dots)? → Hostnames are lowercased and stripped of trailing dots before comparison.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose `POST /api/customer/tunnels` to create a new tunnel for the authenticated account.
- **FR-002**: System MUST expose `PUT /api/customer/tunnels/{id}` to update an existing tunnel owned by the authenticated account.
- **FR-003**: System MUST expose `DELETE /api/customer/tunnels/{id}` to delete a tunnel owned by the authenticated account.
- **FR-004**: System MUST enforce account-scoped ownership on all tunnel operations (users can only CRUD their own tunnels).
- **FR-005**: System MUST validate tunnel configs: hostname format (RFC 952/1123), port range (1-65535), access mode (public|basic_auth|token_protected).
- **FR-006**: System MUST enforce hostname uniqueness per account (and ideally globally for subdomain-based routing).
- **FR-007**: System MUST return structured validation errors (field + message) on invalid input.
- **FR-008**: System MUST expose `POST /api/customer/tunnels/validate` for pre-submission validation.
- **FR-009**: System MUST expose `GET /api/customer/tunnels/{id}/status` returning runtime-derived health from ingested snapshots.
- **FR-010**: System MUST expose `GET /api/customer/config/schema` returning config defaults and validation rules.
- **FR-011**: System MUST expose `POST /api/customer/enrollment/verify` to verify enrollment tokens and return installation credentials.
- **FR-012**: All tunnel mutations MUST be scoped to the authenticated session's account ID.
- **FR-013**: System MUST normalize hostnames to lowercase, strip trailing dots before storage/comparison.
- **FR-014**: Delete of an actively-running tunnel SHOULD emit a runtime disconnect event (best-effort, non-blocking).

### Key Entities

- **Tunnel**: Core entity. Attributes: id (server-assigned), hostname, target (local address), access mode, status, region, owner (account_id). Updated to include: created_at, updated_at timestamps.
- **TunnelCreateRequest**: Client payload for creating tunnels. Fields: name, hostname, target, access, basic_auth (conditional), token_env (conditional).
- **TunnelUpdateRequest**: Partial update payload. All fields optional (omit = no change).
- **TunnelValidationRequest**: Partial tunnel config for pre-flight validation.
- **TunnelStatusResponse**: Runtime-derived status. Fields: status, observed_at, degraded, stale.
- **ConfigSchemaResponse**: Default config values and validation rules for TUI initialization.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 3 CRUD endpoints (POST/PUT/DELETE) pass integration tests with valid and invalid payloads.
- **SC-002**: Hostname conflict detection works correctly (409 on duplicate, 201 on unique).
- **SC-003**: Validation endpoint returns field-level errors that the TUI can display directly.
- **SC-004**: Runtime status endpoint returns stale/unknown correctly when no runtime data exists.
- **SC-005**: All endpoints maintain existing auth/session middleware patterns (no auth regressions).
- **SC-006**: Existing read-only endpoints continue working unchanged (backward compatible).
