# Contributing to bloop-control-plane

## Branch / release policy

- `main` is release-candidate quality at all times.
- Every successful CI run on `main` creates the next patch release automatically.
- Do **not** push directly to `main` unless the change is intended to ship.
- Use pull requests for normal work, including docs and refactors.

## When to use PRs

Use a PR for:
- all feature work
- bug fixes
- docs changes
- workflow/config changes
- dependency changes

Direct pushes to `main` should be reserved for:
- urgent release fixes
- operator-approved hotfixes

## Release semantics

- Current automation bumps patch only: `v0.1.0` -> `v0.1.1` -> `v0.1.2`
- Minor/major releases should be cut intentionally by maintainers.
- `v1.0.0` should be cut only after an explicit readiness review.

## Required protections

Configure GitHub branch protection on `main`:
- require pull requests before merging
- require CI status checks to pass
- block force pushes
- block deletions

## Recommended CODEOWNERS

Require maintainer review for:
- `.github/workflows/*`
- `deploy/*`
- auth/security-sensitive code
- release automation
