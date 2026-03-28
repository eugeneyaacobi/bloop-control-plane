# Tasks: bloop control-plane read API v1

**Input**: Design documents from `/specs/001-read-api-control-plane/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Include handler, repository, and integration tests because secure read APIs, persistence, and response safety are explicit requirements.

**Organization**: Tasks are grouped by setup, foundational work, and then the four user stories: customer read API, admin read API, durable/auditable product state, and SMTP-backed signup verification.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize the repository structure and baseline tooling for the API service.

- [ ] T001 Create Go module and baseline project structure in `/root/.openclaw/workspace-bloop-control-plane/go.mod`, `cmd/`, `internal/`, `pkg/`, `deploy/`, and `tests/`
- [ ] T002 [P] Add `.gitignore` entries for binaries, env files, local configs, and build artifacts in `/root/.openclaw/workspace-bloop-control-plane/.gitignore`
- [ ] T003 [P] Add Makefile or task runner commands for build, test, lint, and local compose workflows in `/root/.openclaw/workspace-bloop-control-plane/Makefile`
- [ ] T004 [P] Update `/root/.openclaw/workspace-bloop-control-plane/README.md` with project purpose, architecture summary, and local development basics
- [ ] T005 [P] Create customer/admin API contract docs in `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/contracts/customer-api.md` and `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/contracts/admin-api.md`
- [ ] T006 [P] Create local quickstart doc in `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/quickstart.md`
- [ ] T007 [P] Create research notes in `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/research.md`
- [ ] T008 [P] Create data model doc in `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/data-model.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core API, config, DB, and security foundations that MUST exist before any user story endpoints are implemented.

**⚠️ CRITICAL**: No user story work should begin until this phase is complete.

- [ ] T009 Implement shared version package in `/root/.openclaw/workspace-bloop-control-plane/pkg/version/version.go`
- [ ] T010 [P] Implement environment-driven configuration loader, including SMTP settings and verification-token config, in `/root/.openclaw/workspace-bloop-control-plane/internal/config/config.go`
- [ ] T011 [P] Implement structured logging with secret-redaction helpers in `/root/.openclaw/workspace-bloop-control-plane/internal/logging/logging.go`
- [ ] T012 [P] Implement database connection/bootstrap package in `/root/.openclaw/workspace-bloop-control-plane/internal/db/connection.go`
- [ ] T013 [P] Add migration framework and initial migration entrypoint in `/root/.openclaw/workspace-bloop-control-plane/internal/db/migrations/`
- [ ] T014 Define core domain models in `/root/.openclaw/workspace-bloop-control-plane/internal/models/`
- [ ] T015 [P] Implement HTTP router/bootstrap and monitoring endpoints (liveness/readiness) in `/root/.openclaw/workspace-bloop-control-plane/internal/api/router.go`
- [ ] T016 [P] Implement middleware scaffolding for request logging, panic recovery, and future auth hooks in `/root/.openclaw/workspace-bloop-control-plane/internal/api/middleware/`
- [ ] T017 [P] Implement basic security utility package for identifier validation and safe error responses in `/root/.openclaw/workspace-bloop-control-plane/internal/security/`
- [ ] T018 [P] Add unit tests for config, logging redaction, validation helpers, and verification-token helpers in `/root/.openclaw/workspace-bloop-control-plane/tests/unit/`

**Checkpoint**: API foundation, DB connection, migrations, and security scaffolding are ready.

---

## Phase 3: User Story 1 - Customer frontend reads workspace and tunnels (Priority: P1) 🎯 MVP

**Goal**: Provide stable customer-facing read endpoints for workspace summary, tunnel list, and tunnel detail.

**Independent Test**: Seed Postgres with a customer/account/tunnel dataset and verify the customer endpoints return the expected response shape without secrets.

### Tests for User Story 1

- [ ] T019 [P] [US1] Add repository tests for customer workspace/tunnel reads in `/root/.openclaw/workspace-bloop-control-plane/tests/repository/customer_repository_test.go`
- [ ] T020 [P] [US1] Add handler tests for customer endpoints in `/root/.openclaw/workspace-bloop-control-plane/tests/handler/customer_handler_test.go`
- [ ] T021 [P] [US1] Add integration test for customer workspace + tunnels endpoints in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/customer_api_test.go`

### Implementation for User Story 1

- [ ] T022 [P] [US1] Implement customer repository queries in `/root/.openclaw/workspace-bloop-control-plane/internal/repository/customer_repository.go`
- [ ] T023 [P] [US1] Implement customer service layer in `/root/.openclaw/workspace-bloop-control-plane/internal/service/customer_service.go`
- [ ] T024 [US1] Implement `GET /api/customer/workspace` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/customer/workspace.go`
- [ ] T025 [US1] Implement `GET /api/customer/tunnels` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/customer/tunnels.go`
- [x] T026 [US1] Implement `GET /api/customer/tunnels/{id}` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/customer/tunnel_detail.go`
- [x] T026a [US1] Implement `POST /api/customer/tunnels` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/customer/create_tunnel.go`
- [ ] T027 [US1] Add response shaping that aligns with frontend customer models in `/root/.openclaw/workspace-bloop-control-plane/internal/service/customer_presenters.go`
- [ ] T028 [US1] Ensure no secret-bearing fields are serialized in customer responses

**Checkpoint**: Customer-facing API is independently usable by the frontend.

---

## Phase 4: User Story 2 - Admin frontend reads platform-wide oversight data (Priority: P2)

**Goal**: Provide stable admin-facing read endpoints for overview, users, tunnel inventory, and review queue data.

**Independent Test**: Seed multiple users/accounts/tunnels/review flags and verify admin endpoints return platform-wide oversight views without leaking secret material.

### Tests for User Story 2

- [ ] T029 [P] [US2] Add repository tests for admin overview/users/tunnels/review queue in `/root/.openclaw/workspace-bloop-control-plane/tests/repository/admin_repository_test.go`
- [ ] T030 [P] [US2] Add handler tests for admin endpoints in `/root/.openclaw/workspace-bloop-control-plane/tests/handler/admin_handler_test.go`
- [ ] T031 [P] [US2] Add integration test for admin read endpoints in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/admin_api_test.go`

### Implementation for User Story 2

- [ ] T032 [P] [US2] Implement admin repository queries in `/root/.openclaw/workspace-bloop-control-plane/internal/repository/admin_repository.go`
- [ ] T033 [P] [US2] Implement admin service layer in `/root/.openclaw/workspace-bloop-control-plane/internal/service/admin_service.go`
- [ ] T034 [US2] Implement `GET /api/admin/overview` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/admin/overview.go`
- [ ] T035 [US2] Implement `GET /api/admin/users` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/admin/users.go`
- [ ] T036 [US2] Implement `GET /api/admin/tunnels` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/admin/tunnels.go`
- [ ] T037 [US2] Implement `GET /api/admin/review-queue` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/admin/review_queue.go`
- [ ] T038 [US2] Add response shaping that aligns with frontend admin models in `/root/.openclaw/workspace-bloop-control-plane/internal/service/admin_presenters.go`

**Checkpoint**: Admin-facing API is independently usable by the frontend.

---

## Phase 5: User Story 3 - Control-plane state is durable and auditable (Priority: P3)

**Goal**: Persist product-facing state in Postgres with enough durability and audit support to power customer/admin read models.

**Independent Test**: Run the API with Postgres, seed data, restart the service, and verify persisted state remains readable through the endpoints.

### Tests for User Story 3

- [ ] T039 [P] [US3] Add migration/integration test for schema bootstrapping in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/migrations_test.go`
- [ ] T040 [P] [US3] Add persistence/restart integration test in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/persistence_test.go`
- [ ] T041 [P] [US3] Add response-safety test ensuring secret fields are absent in serialized payloads in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/response_safety_test.go`

### Implementation for User Story 3

- [ ] T042 [P] [US3] Create initial SQL migrations for users, accounts, memberships, tunnels, review_flags, audit_events, signup_requests, email_verifications, and onboarding-related tables in `/root/.openclaw/workspace-bloop-control-plane/internal/db/migrations/`
- [ ] T043 [P] [US3] Implement shared repository/database helpers in `/root/.openclaw/workspace-bloop-control-plane/internal/repository/common.go`
- [ ] T044 [US3] Implement onboarding data repository/service in `/root/.openclaw/workspace-bloop-control-plane/internal/repository/onboarding_repository.go` and `/root/.openclaw/workspace-bloop-control-plane/internal/service/onboarding_service.go`
- [ ] T045 [US3] Implement `GET /api/onboarding/steps` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/onboarding/steps.go`
- [ ] T046 [US3] Add audit event persistence scaffolding in `/root/.openclaw/workspace-bloop-control-plane/internal/audit/audit.go`
- [ ] T047 [US3] Add development seed data path for local verification in `/root/.openclaw/workspace-bloop-control-plane/internal/db/seed.go`

**Checkpoint**: Control-plane state is durable, readable, and structurally audit-ready.

---

## Phase 6: User Story 4 - New users verify signup through email with audit trail (Priority: P4)

**Goal**: Provide SMTP-backed signup verification with durable verification records and audit trail support.

**Independent Test**: Submit a signup request, record an email delivery attempt, and verify a valid token successfully while invalid/expired tokens fail safely.

### Tests for User Story 4

- [ ] T048 [P] [US4] Add unit tests for verification token generation/expiry in `/root/.openclaw/workspace-bloop-control-plane/tests/unit/verification_test.go`
- [ ] T049 [P] [US4] Add handler tests for signup request and verification endpoints in `/root/.openclaw/workspace-bloop-control-plane/tests/handler/signup_handler_test.go`
- [ ] T050 [P] [US4] Add integration test for signup request + verify flow with SMTP delivery attempt recording in `/root/.openclaw/workspace-bloop-control-plane/tests/integration/signup_verification_test.go`

### Implementation for User Story 4

- [ ] T051 [P] [US4] Implement SMTP email service in `/root/.openclaw/workspace-bloop-control-plane/internal/service/email_service.go`
- [ ] T052 [P] [US4] Implement signup and verification repositories in `/root/.openclaw/workspace-bloop-control-plane/internal/repository/signup_repository.go`
- [ ] T053 [P] [US4] Implement signup verification service and token lifecycle helpers in `/root/.openclaw/workspace-bloop-control-plane/internal/service/signup_service.go` and `/root/.openclaw/workspace-bloop-control-plane/internal/security/verification.go`
- [ ] T054 [US4] Implement `POST /api/signup/request` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/onboarding/signup_request.go`
- [ ] T055 [US4] Implement `POST /api/signup/verify` in `/root/.openclaw/workspace-bloop-control-plane/internal/api/onboarding/signup_verify.go`
- [ ] T056 [US4] Add audit event recording for signup request, email send attempt, and verification outcome in `/root/.openclaw/workspace-bloop-control-plane/internal/audit/audit.go`
- [ ] T057 [US4] Ensure safe anti-enumeration and no raw-token logging behavior across signup flows

**Checkpoint**: Signup verification plumbing is functional, auditable, and ready for future public onboarding.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Deployment polish, docs, and cross-cutting hardening across the service.

- [ ] T058 [P] Add main server entrypoint in `/root/.openclaw/workspace-bloop-control-plane/cmd/bloop-control-plane/main.go`
- [ ] T059 [P] Add Dockerfile in `/root/.openclaw/workspace-bloop-control-plane/deploy/docker/Dockerfile`
- [ ] T060 [P] Add local Docker Compose for API + Postgres in `/root/.openclaw/workspace-bloop-control-plane/deploy/compose/local.yml`
- [ ] T061 [P] Document environment variables, local run steps, SMTP/security notes, and anti-enumeration behavior in `/root/.openclaw/workspace-bloop-control-plane/README.md`
- [ ] T062 [P] Add quickstart validation steps in `/root/.openclaw/workspace-bloop-control-plane/specs/001-read-api-control-plane/quickstart.md`
- [ ] T063 Run full build/test cycle and document remaining gaps before moving to broader mutation APIs

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: starts immediately
- **Phase 2 (Foundational)**: depends on Setup completion and blocks all user stories
- **Phase 3 (US1)**: starts after Foundational
- **Phase 4 (US2)**: starts after Foundational and can proceed after shared repository/service patterns exist
- **Phase 5 (US3)**: starts after Foundational and should be completed before treating the API as durable
- **Phase 6 (US4)**: starts after Foundational and depends on persistence/security foundations
- **Phase 7 (Polish)**: depends on desired user stories being complete

### User Story Dependencies

- **US1 (P1)**: starts after Foundational and provides the first frontend-usable API slice
- **US2 (P2)**: starts after Foundational and depends on shared persistence/repository patterns
- **US3 (P3)**: starts after Foundational and underpins durable control-plane operation
- **US4 (P4)**: starts after Foundational and depends on durable persistence plus security/config scaffolding

### Within Each User Story

- Repository tests/handler tests before endpoint finalization
- Models and queries before services
- Services before HTTP handlers
- Response shaping before frontend integration
- Security/redaction expectations enforced before calling the story done

### Parallel Opportunities

- Setup tasks marked `[P]` can run in parallel
- Foundational config/logging/security tasks marked `[P]` can run in parallel
- Repository and handler tests within a story can run in parallel
- Customer/admin repository/service implementation can be split across files
- Docker/docs tasks in the final phase can run in parallel

---

## Implementation Strategy

### MVP First (Recommended)

1. Complete Setup
2. Complete Foundational phase
3. Complete User Story 1 (customer read API)
4. Validate customer endpoints against frontend needs
5. Add User Story 2 (admin read API)
6. Add User Story 3 (durability/audit/onboarding support)
7. Finish Docker/docs/test polish

### Incremental Delivery

1. Foundation + DB setup
2. Customer workspace/tunnels API
3. Admin overview/inventory API
4. Onboarding + durable control-plane persistence
5. Signup verification + SMTP plumbing
6. Docker/test/doc polish

---

## Notes

- Keep the first slice read-only.
- Prefer explicit response models over leaking DB row shapes directly.
- Keep secret-bearing fields and product-display fields separate from the start.
- Treat auditability and role separation as structural concerns, not future TODO comments.
