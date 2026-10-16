# 27. Artifact stores are created by root, not by each environment

Status: Accepted (supersedes where [21](0021-per-environment-artifact-stores.md) put the stores; amends [17](0017-manual-prod-infrastructure-apply.md))

## Context

Every merge publishes the backend image and the frontend bundle into every environment's stores ([15](0015-build-once-promote-by-sha.md), [21](0021-per-environment-artifact-stores.md)), but env Terraform only reaches prod on promotion. With the stores in env Terraform, a repository's first merge failed pushing to prod's registry, which didn't exist until the first promotion, and so did any merge after a new per-environment store was added. The only way round it was the manual prod apply ([17](0017-manual-prod-infrastructure-apply.md)), which a new repository had no reason to know to run.

## Decision

- `infrastructure/root/artifact_stores.tf` creates each environment project's Artifact Registry `backend` repository, its `<project id>-frontend` bucket, the CI service account's writer grants on both, and the Artifact Registry API, for every project in `gcp_projects`. They are still one store per environment, written once and read only within its own project ([21](0021-per-environment-artifact-stores.md)); only who creates them moved. The retention rules ([22](0022-artifact-retention.md)) and their variables moved with them.
- Root applies before staging on every merge it is affected by, and on a repository's first, so every environment's stores exist before anything publishes into them. `services-backend` and `services-frontend` both wait on root's apply having succeeded or skipped.
- The rule: anything a merge writes into every environment belongs in root. Env Terraform keeps what only its own environment's apply, deploy or promotion touches (Cloud Run, Firestore, monitoring).

## Consequences

A merge never depends on prod having been promoted, so the manual prod apply ([17](0017-manual-prod-infrastructure-apply.md)) is left for per-environment resources that aren't stores. Root's plan and apply now include artifact stores, so a change to retention shows in root's plan, not the environments'.
