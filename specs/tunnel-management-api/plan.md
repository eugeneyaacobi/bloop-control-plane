# Implementation Plan: Tunnel Management API

**Branch**: `007-tunnel-management-api` | **Date**: 2026-03-30 | **Spec**: `specs/tunnel-management-api/spec.md`

## Summary

Add full CRUD API for tunnel management to bloop-control-plane so the bloop-tunnel TUI can create, read, update, delete, and validate tunnel configurations. Also adds enrollment verification and runtime status endpoints. Follows existing chi router, repository pattern, and session auth middleware conventions.

## Technical Context

**Language/Version**: Go 1.24 (per go.mod)
**Primary Dependencies**: chi/v5 (router), pgx/v5 (PostgreSQL), bubbletea (TUI client side)
**Storage**: PostgreSQL (existing pgx pool)
**Testing**: Go standard testing + existing handler_test.go patterns
**Target Platform**: Linux server (Docker compose deployment)
**Project Type**: web API (single service)
**Performance Goals**: Standard CRUD — no special latency requirements
**Constraints**: Must maintain backward compatibility with existing read-only endpoints
**Scale/Scope**: Single-account prototype expanding to multi-tenant

## Constitution Check

- ✅ Follows existing repository pattern (CustomerRepository interface)
- ✅ Uses existing auth middleware (session.Resolver)
- ✅ Follows existing handler/service/repo layering
- ✅ Tests follow existing handler_test.go patterns

## Project Structure

### Documentation (this feature)

```text
specs/tunnel-management-api/
├── spec.md
├── plan.md              # This file
└── tasks.md             # Next step
```

### Source Code (repository root)

```text
internal/
├── api/
│   ├── customer/
│   │   ├── routes.go          # UPDATE: add new routes
│   │   ├── tunnels.go         # EXISTS: GET handlers
│   │   ├── tunnel_create.go   # NEW: POST /tunnels
│   │   ├── tunnel_update.go   # NEW: PUT /tunnels/{id}
│   │   ├── tunnel_delete.go   # NEW: DELETE /tunnels/{id}
│   │   ├── tunnel_validate.go # NEW: POST /tunnels/validate
│   │   ├── tunnel_status.go   # NEW: GET /tunnels/{id}/status
│   │   ├── config_schema.go   # NEW: GET /config/schema
│   │   ├── enrollment.go      # NEW: POST /enrollment/verify
│   │   └── handler_test.go    # UPDATE: add tests
│   └── authz/
│       └── authz.go           # EXISTS: no changes needed
├── models/
│   └── models.go              # UPDATE: add request/response types
├── repository/
│   ├── customer_repository.go # UPDATE: add CRUD methods to interface
│   └── customer_postgres.go   # UPDATE: add SQL implementations
└── service/
    └── customer.go            # UPDATE: add CRUD service methods

migrations/                    # NEW: DB schema changes
└── 007_tunnel_crud.sql
```

**Structure Decision**: Follow existing layered architecture exactly. One handler file per endpoint, following the existing `tunnels.go` / `tunnel_detail.go` convention.

## Implementation Phases

### Phase 1: Data Layer (models + repository)

Extend the Tunnel model with timestamps, add CRUD methods to CustomerRepository interface, implement in customer_postgres.go. Add DB migration for new columns.

### Phase 2: Service Layer

Add CreateTunnel, UpdateTunnel, DeleteTunnel, ValidateTunnel, GetTunnelStatus methods to CustomerService. Handle hostname normalization, uniqueness checks, and validation logic here.

### Phase 3: API Layer (handlers + routes)

Add handler files for each new endpoint. Wire routes in customer/routes.go. Follow existing patterns for auth checks and error handling.

### Phase 4: Tests

Integration tests for each endpoint covering success and error cases. Follow existing handler_test.go patterns.
