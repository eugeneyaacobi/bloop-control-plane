# bloop-control-plane Constitution

## Core Principles

### I. Security Is a Design Requirement, Not a Cleanup Phase
The control-plane service must be designed with explicit security boundaries from the beginning. Authentication, authorization, input validation, secret handling, redaction, and auditability are mandatory design concerns, even when the first feature slice is read-first.

### II. Product State Lives Here
This service owns product-facing state for the bloop platform: users, accounts, workspaces, tunnel metadata projections, onboarding state, review flags, and audit events. It must not collapse into a thin wrapper that merely mirrors runtime internals without durable product semantics.

### III. Runtime and Control Concerns Stay Separated
`bloop-tunnel` owns tunnel runtime behavior and traffic mechanics. `bloop-control-plane` owns customer/admin API concerns and durable product-facing state. Integrations between them must be explicit and documented.

### IV. Postgres-Backed by Default
Persistent control-plane state must live in a relational datastore with schema discipline and migrations. Postgres is the default. In-memory-only product state is not acceptable beyond trivial bootstrapping.

### V. Read-First, Not Reckless-First
Initial API development should prioritize secure, read-only endpoints that serve the frontend’s customer/admin visibility needs. Mutation endpoints should arrive only after the read model, auth boundaries, and audit expectations are well-defined.

### VI. Auditability and Least Privilege
Sensitive operations and policy-relevant actions must be traceable through audit events. Internal roles, service credentials, database access, and API scopes should follow least-privilege defaults.

### VII. Docker-Deployable and Operationally Boring
The service should be easy to run in Docker with Postgres, clear environment-based configuration, and a path to secure deployment behind a reverse proxy. Cleverness is allowed in product design, not in deployment confusion.

## Scope Constraints

- V1 focuses on read-only customer/admin/onboarding API endpoints.
- V1 includes durable persistence for users/accounts/tunnels/audit-related models.
- V1 must be designed for future frontend integration and future runtime synchronization with `bloop-tunnel`.
- V1 should avoid premature write APIs, billing complexity, or speculative enterprise role hierarchies.

## Development Workflow

- Establish constitution, then spec, then plan, then tasks.
- Make security and data-model expectations explicit in the spec, not implied.
- Keep backend contracts typed and traceable to frontend-facing requirements.
- Prefer small, testable slices over a giant control-plane blob.

## Governance
This constitution governs the control-plane repository. Any future feature that weakens auditability, blurs runtime/control boundaries, or treats security as optional requires explicit justification in the spec and plan.

**Version**: 1.0.0 | **Ratified**: 2026-03-26 | **Last Amended**: 2026-03-26
