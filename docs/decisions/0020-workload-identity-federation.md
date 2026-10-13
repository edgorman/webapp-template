# 20. CI authenticates with Workload Identity Federation

Status: Accepted

## Context

JSON service-account keys are long-lived secrets.

## Decision

- Workflows authenticate over short-lived OIDC tokens. One GitHub Actions service account lives in root and is granted roles on each project individually, never at org level. The provider's attribute condition admits only this repository's tokens.
- The root apply writes the provider path, service account email, project ids, region and site URLs into GitHub Actions variables, so workflows never hardcode them. Until that first apply, the variables are empty and the deploy jobs skip.
- The one credential WIF can't replace, the GitHub PAT root's Terraform uses to write those variables, was supplied by hand once at bootstrap and is read back from the `github_provider_token` Secret Manager secret on each run, never stored as a GitHub secret.

## Consequences

No stored cloud credentials in GitHub. A backend that calls a Google API does the same: its runtime service account, not an API key.
