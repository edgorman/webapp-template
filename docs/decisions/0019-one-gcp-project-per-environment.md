# 19. One GCP project per environment, plus a root

Status: Accepted

## Context

Staging must not be able to reach production resources by accident.

## Decision

- `<prefix>-stag` and `<prefix>-prod` are separate projects (the prefix is `gcp_project_prefix` in `infrastructure/config/root/terraform.tfvars`), and resources carry environment suffixes (`backend-stag`, `backend-prod`).
- `infrastructure/root` provisions the root project's contents and what both environments share: the two environment projects, every Terraform state bucket (root's included), the GitHub Actions WIF pool and provider, shared domain configuration, and the baseline APIs on all three projects.
- Root's first apply is the only manual one, against local state, since nothing has credentials or state before it. State is then migrated to the bucket it created (`terraform init -migrate-state`). After that root is planned on PR and applied on merge like the others.

## Consequences

Blast radius is a project boundary. Per-PR environments are a possible later addition, not needed yet.
