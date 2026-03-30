# Tasks: Tunnel Management API

**Input**: `specs/tunnel-management-api/spec.md`, `specs/tunnel-management-api/plan.md`
**Prerequisites**: plan.md (required), spec.md (required)

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Data Layer (Shared Infrastructure)

- [ ] T001 Add `created_at`, `updated_at` timestamp columns to `tunnels` table. Create migration file `migrations/007_tunnel_crud.sql` with ALTER TABLE + indexes.
- [ ] T002 Update `models.Tunnel` struct in `internal/models/models.go` to include `CreatedAt time.Time` and `UpdatedAt time.Time` fields.
- [ ] T003 Add request/response types to `internal/models/models.go`: `TunnelCreateRequest`, `TunnelUpdateRequest`, `TunnelValidationRequest`, `TunnelValidationResponse`, `TunnelStatusResponse`, `ConfigSchemaResponse`, `EnrollmentVerifyRequest`, `EnrollmentVerifyResponse`.
- [ ] T004 Extend `CustomerRepository` interface in `internal/repository/customer_repository.go` with: `CreateTunnel`, `UpdateTunnel`, `DeleteTunnel`, `GetTunnelByHostname`, `GetRuntimeStatusByTunnelID`.
- [ ] T005 Implement new repository methods in `internal/repository/customer_postgres.go` with SQL queries for CRUD + hostname lookup + runtime status from `runtime_tunnel_snapshots`.
- [ ] T006 [P] Update `InMemoryCustomerRepository` with stub implementations for new methods (dev/test support).

## Phase 2: Service Layer

- [ ] T007 Add `CreateTunnel(ctx, accountID, req) (*Tunnel, error)` to `CustomerService` in `internal/service/customer.go`. Handle hostname normalization (lowercase, strip trailing dot), uniqueness check, server-assigned ID, status="pending".
- [ ] T008 Add `UpdateTunnel(ctx, accountID, tunnelID, req) (*Tunnel, error)` to `CustomerService`. Partial update (omit = no change). Re-validate hostname uniqueness if changed.
- [ ] T009 Add `DeleteTunnel(ctx, accountID, tunnelID) error` to `CustomerService`. Verify ownership before delete.
- [ ] T010 Add `ValidateTunnel(ctx, accountID, req) (*TunnelValidationResponse, error)` to `CustomerService`. Check hostname format, availability, port range, access mode validity. Return field-level errors.
- [ ] T011 Add `GetTunnelStatus(ctx, accountID, tunnelID) (*TunnelStatusResponse, error)` to `CustomerService`. Query runtime snapshot table, return status/stale flag.
- [ ] T012 Add `GetConfigSchema(ctx) (*ConfigSchemaResponse, error)` to `CustomerService`. Return static defaults (access modes, hostname patterns, default relay URL).
- [ ] T013 Add `VerifyEnrollment(ctx, token string) (*EnrollmentVerifyResponse, error)` to `CustomerService`. Validate token against runtime_installation_tokens, return installation ID + ingest token.

## Phase 3: API Handlers (US1-US3: CRUD)

- [ ] T014 [P] Create `internal/api/customer/tunnel_create.go` — handler for `POST /tunnels`. Decode request, call service, return 201 or error.
- [ ] T015 [P] Create `internal/api/customer/tunnel_update.go` — handler for `PUT /tunnels/{id}`. Decode request, extract ID from URL, call service, return 200 or error.
- [ ] T016 [P] Create `internal/api/customer/tunnel_delete.go` — handler for `DELETE /tunnels/{id}`. Extract ID, call service, return 204 or 404.
- [ ] T017 Update `internal/api/customer/routes.go` — mount new CRUD routes:
  ```
  r.Post("/tunnels", h.CreateTunnel)
  r.Put("/tunnels/{id}", h.UpdateTunnel)
  r.Delete("/tunnels/{id}", h.DeleteTunnel)
  ```
- [ ] T018 Update `internal/api/customer/workspace.go` — extend `CustomerWorkspaceService` interface with new service methods.

## Phase 4: API Handlers (US4-US7: Validation, Status, Config, Enrollment)

- [ ] T019 [P] Create `internal/api/customer/tunnel_validate.go` — handler for `POST /tunnels/validate`. Decode, call service, return validation result.
- [ ] T020 [P] Create `internal/api/customer/tunnel_status.go` — handler for `GET /tunnels/{id}/status`. Extract ID, call service, return runtime status.
- [ ] T021 [P] Create `internal/api/customer/config_schema.go` — handler for `GET /config/schema`. Call service, return schema.
- [ ] T022 [P] Create `internal/api/customer/enrollment.go` — handler for `POST /enrollment/verify`. Decode token, call service, return enrollment result.
- [ ] T023 Update `internal/api/customer/routes.go` — mount remaining routes:
  ```
  r.Post("/tunnels/validate", h.ValidateTunnel)
  r.Get("/tunnels/{id}/status", h.TunnelStatus)
  r.Get("/config/schema", h.ConfigSchema)
  r.Post("/enrollment/verify", h.VerifyEnrollment)
  ```

## Phase 5: Tests

- [ ] T024 [P] Add handler tests for tunnel CRUD in `internal/api/customer/handler_test.go` — cover 201/200/204 success, 404, 409 conflict, 422 validation.
- [ ] T025 [P] Add handler tests for validation endpoint — valid config, duplicate hostname, invalid port, missing fields.
- [ ] T026 [P] Add handler tests for status endpoint — with runtime data, without runtime data (stale).
- [ ] T027 [P] Add handler tests for config schema and enrollment verify.

## Phase 6: Verification

- [ ] T028 Run `go build ./...` and `go test ./...` — all pass, no regressions.
- [ ] T029 Verify existing read-only endpoints still work (GET /tunnels, GET /tunnels/{id}, GET /workspace).
