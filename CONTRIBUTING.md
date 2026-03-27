# Contributing to bloop-control-plane

## Branch / release policy

- `develop` is the integration branch for ongoing work.
- Successful CI on `develop` creates an automatic prerelease (`-rc.N`).
- `main` is promotion-only and should represent stable release candidates.
- Successful CI on `main` creates the next stable patch release automatically.
- Normal work should land on feature branches and merge into `develop` via pull request.

## When to use PRs

Use a PR for:
- all feature work
- bug fixes
- docs changes
- workflow/config changes
- dependency changes

Direct pushes to protected branches should be avoided. Use PRs into `develop`, then promote `develop` -> `main` intentionally.

## Release semantics

- Stable tags use semver: `vMAJOR.MINOR.PATCH`
- Develop prereleases use semver prerelease tags: `vMAJOR.MINOR.PATCH-rc.N`
- Minor/major releases should be cut intentionally by maintainers.
- `v1.0.0` should be cut only after an explicit readiness review.

## Required protections

Configure GitHub branch protection on `develop` and `main`:
- require pull requests before merging
- require CI status checks to pass
- require review from code owners
- block force pushes
- block deletions

## Recommended CODEOWNERS

Require maintainer review for:
- `.github/workflows/*`
- `deploy/*`
- auth/security-sensitive code
- release automation
