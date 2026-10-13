# 11. moon, not Pants

Status: Accepted

## Context

Pants was this repository's first build orchestrator, adopted to stop maintaining the dependency graph by hand. Most of its cost became fighting the tool:

- `go vet` ran outside Pants, because its vet backend never changed into the module directory and `services/backend/go.mod` isn't at the build root.
- The frontend bundle was never really built through Pants: its sandbox held only declared dependencies, and nothing imports a tsconfig, `index.html` or `public/` asset.
- oxlint had no Pants backend, and a Python interpreter was needed only to parse HCL.
- The sandbox dropped `PATH`, breaking `terraform init` until patched.
- Composite actions were invisible to the graph, so a blanket `.github/**` glob was ORed onto every infrastructure decision.
- Pants couldn't bootstrap in a Claude Code cloud session (its bundled Python rejected the proxy's CA).

## Decision

Use moon v2: a per-project graph of hand-declared edges plus per-task `inputs` ([12](0012-hand-declared-moon-graph.md)).

Deliberately given up:

- Per-file dependency inference. With one Go module, one npm package and no cross-project imports, there is nothing for it to find.
- A sandbox. moon runs in the real checkout, so undeclared inputs aren't caught; `inputs` must be explicit where it matters.
- The GitHub Actions remote cache. moon's remote cache speaks only Bazel Remote Execution v2. `moon ci` skipping unaffected work covers most of it; revisit (e.g. `bazel-remote` on GCS) only if CI time becomes a problem.

## Consequences

The frontend bundle builds as `services/frontend:build`, and the backend image as `services/backend:image`.
