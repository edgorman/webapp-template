# 9. Wire changes across the release window

Status: Accepted

## Context

A production release deploys the backend before the frontend ([15](0015-build-once-promote-by-sha.md)), so for a short window a new backend serves the previous bundle. The window is short.

## Decision

- Wire changes should be additive. `DiscardUnknown` ([6](0006-protojson-settings.md)) makes an added field safe to deploy ahead of the client that sends it.
- `buf breaking` against `origin/main` runs as the required `protos-breaking` check (`packages/protos:breaking`, in `conventions.yaml`). A deliberate break is marked breaking (`!` in the PR title or a `BREAKING CHANGE:` footer), which skips the check and bumps the major version.

## Consequences

Anything larger than a field addition is checked against the release ordering before it ships.
