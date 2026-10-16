# Decision records

One file per decision: its context, the decision, and its consequences. The rules that follow from them are in the `AGENTS.md` files, each linking here. A record changes only when it is superseded: write a new one, and mark the old one `Superseded by N`.

These records came with the template. They hold for any repository made from it until you supersede them; number your own from the next free number.

| # | Decision | Area |
| --- | --- | --- |
| [1](0001-manifests-live-with-their-code.md) | Manifests live with their code | repo |
| [2](0002-generated-code-is-committed.md) | Generated code is committed, next to the code that uses it | protos |
| [3](0003-pin-every-codegen-tool.md) | Every codegen tool is pinned exactly | protos |
| [4](0004-ts-proto-options.md) | ts-proto options | protos |
| [5](0005-protos-model-the-wire.md) | Protos model the wire, not the domain or the datastore | protos |
| [6](0006-protojson-settings.md) | protojson settings | protos |
| [7](0007-visibility-is-a-string.md) | Visibility is a string, not an enum | protos |
| [8](0008-no-hand-written-wire-types.md) | No hand-written wire types, enforced in CI | protos |
| [9](0009-wire-changes-across-releases.md) | Wire changes across the release window | protos |
| [10](0010-one-access-policy-table.md) | One access policy table, closed by default | backend |
| [11](0011-moon-not-pants.md) | moon, not Pants | ci |
| [12](0012-hand-declared-moon-graph.md) | A hand-declared moon graph, affected by changed files only | ci |
| [13](0013-deploy-markers.md) | Deploy decisions come from marker tasks | ci |
| [14](0014-prototools-pins-the-toolchain.md) | `.prototools` is the single source of tool versions | ci |
| [15](0015-build-once-promote-by-sha.md) | Build once, promote by commit SHA | ci |
| [16](0016-pre-release-gates.md) | What a pre-release waits for | ci |
| [17](0017-manual-prod-infrastructure-apply.md) | A manual prod apply, for bootstrapping only | ci |
| [18](0018-versions-from-conventional-commits.md) | Versions come from Conventional Commit subjects | ci |
| [19](0019-one-gcp-project-per-environment.md) | One GCP project per environment, plus a root | infrastructure |
| [20](0020-workload-identity-federation.md) | CI authenticates with Workload Identity Federation | infrastructure |
| [21](0021-per-environment-artifact-stores.md) | Each environment has its own artifact stores | infrastructure |
| [22](0022-artifact-retention.md) | Artifact retention keeps recent merges and every release | infrastructure |
| [23](0023-monitoring-on-cloud-run-metrics.md) | Monitoring watches Cloud Run's own metrics | infrastructure |
| [24](0024-debug-endpoint-contract.md) | The debug endpoint contract | backend |
| [25](0025-apply-saved-plans.md) | Applies use the plan already shown | ci |
| [26](0026-build-only-what-changed.md) | Build only what changed, reuse the rest | ci |
