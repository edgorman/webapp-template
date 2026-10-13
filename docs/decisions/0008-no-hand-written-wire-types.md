# 8. No hand-written wire types, enforced in CI

Status: Accepted

## Context

Every wire type moved into `packages/protos` once, and a hand-written one would quietly come back the first time someone needed a shape in a hurry. Even the `{"error": "..."}` envelope is a message (`appv1.ErrorResponse`, `error.proto`), shared by every resource; `entity.ValidationError` is collapsed to it by `writeValidationError`.

## Decision

The `root:wire-types` moon task fails the PR on any `export interface` in `services/frontend/src/lib/api.ts`, or any `json:"..."` struct tag in `internal/service`'s non-test files other than `debug.go`. It runs in `lint-check`, a check already required,, rather than a new required check that could merge red until someone added it to `settings.yml`.

What stays hand-written, by design:

- `entity/access.go`'s policy table: authorization, not a shape.
- A third-party API's request and response shapes, which this repository doesn't define or version.
- Every `Validate()`/`Set*` method: business logic a proto can't express.
- `debug.go`'s `debugResponse`: nothing else declares that shape to drift against ([24](0024-debug-endpoint-contract.md)).
- `services/frontend/src/lib/config.ts`'s `RuntimeConfig`: `config.json` is written by the deploy pipeline, not served by the backend ([15](0015-build-once-promote-by-sha.md)).

## Consequences

A new wire type starts as a message in a `.proto`.
