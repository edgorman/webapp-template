# .github

Workflows, composite actions, repository settings and the moon setup CI runs through. What to do when a PR's checks fail is the `steward` skill (`.claude/skills/steward/SKILL.md`). Reasoning for each rule is in the linked record under `docs/decisions/`.

## CI/CD, Branching, & Release Lifecycle

- Everything builds, tests and deploys through GitHub Actions; infrastructure applies before services update.
- Every job that needs GCP or Cloudflare is skipped while `vars.WORKLOAD_IDENTITY_PROVIDER` is empty (the `changed` jobs and `promote-release.yaml`'s first job carry the `if:`, and everything else needs them), so a repository made from the template runs only PR checks until its root bootstrap ([20](../docs/decisions/0020-workload-identity-federation.md)).
- Every pipeline has an environment-scoped concurrency group.
- Shared steps live in composite actions under `.github/actions/`.

### Building with moon

- Every folder under `services/` or `packages/` needs a `moon.yml`; `root:projects-covered` fails otherwise ([12](../docs/decisions/0012-hand-declared-moon-graph.md)).
- Declare `dependsOn` by hand. Affected detection is changed files only, so a task that must react to another project lists it in `inputs` (`project://...`) ([12](../docs/decisions/0012-hand-declared-moon-graph.md)).
- Keep `moon ci`'s targets split one-to-one with the required jobs (`lint-check`, `test`, `protos-drift`; `pr-title` and `protos-breaking` in `conventions.yaml`) ([12](../docs/decisions/0012-hand-declared-moon-graph.md)).
- `deploy-{root,staging,prod}:plan` are `noop` markers whose `inputs` decide which environments a diff reaches. List every workflow and action an environment uses in its marker, and never set `runInCI: false` on one ([13](../docs/decisions/0013-deploy-markers.md)).
- Pin tool versions only in `.prototools`; keep its `go` pin, `go.mod` and the Dockerfile image in step ([14](../docs/decisions/0014-prototools-pins-the-toolchain.md)).
- No remote cache ([11](../docs/decisions/0011-moon-not-pants.md)).

### Versioning

- Versions are computed from Conventional Commit subjects; tags are plain `major.minor.patch`, with pre-release as GitHub's flag ([18](../docs/decisions/0018-versions-from-conventional-commits.md)).

### Staging Deployments

- Every merge builds the backend image and the frontend `dist/` once, publishes them by commit SHA to both staging's and prod's stores, and deploys staging from them, whether or not the service changed ([15](../docs/decisions/0015-build-once-promote-by-sha.md)).
- The frontend bundle carries nothing environment-specific; `config.json` is written at deploy time ([15](../docs/decisions/0015-build-once-promote-by-sha.md)).

### Pre-Release Generation

- `pre-release` runs on every merge, checks each direct need for `success` (`services-backend`, a matrix over the images, `services-frontend`, `smoke`, `version`), and takes its tag from `version` ([16](../docs/decisions/0016-pre-release-gates.md)).
- The prod plan in a pre-release runs with `-lock=false` ([16](../docs/decisions/0016-pre-release-gates.md)).

### Production Releases

- Promotion applies prod Terraform, then deploys the backend and the frontend by commit SHA from prod's own stores. Never rebuild for production, and never copy between projects ([15](../docs/decisions/0015-build-once-promote-by-sha.md)).
- The `release-<version>` image tag is for retention only; deploy by SHA ([22](../docs/decisions/0022-artifact-retention.md)).
- Rollback is promoting an earlier release tag ([15](../docs/decisions/0015-build-once-promote-by-sha.md)).
- `apply-infrastructure-prod.yaml` is only for backfilling a new per-environment resource a merge depends on; say so in that PR ([17](../docs/decisions/0017-manual-prod-infrastructure-apply.md)).
