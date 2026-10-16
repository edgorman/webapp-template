# Starting a new repository from the template

From **Use this template** to a deployed staging site. Until step 6, the new repository's workflows run only the PR checks; every job that needs GCP or Cloudflare skips itself while the `WORKLOAD_IDENTITY_PROVIDER` repository variable is unset.

You need a GCP billing account, a Cloudflare account with your domain on it as a zone, and the toolchain from [`start-here.md`](start-here.md) plus the gcloud CLI.

## 1. Create the repository

Click **Use this template** on GitHub. Then, in the new repository:

- `.github/settings.yml`: set `name` and `description`, and set `is_template` to `false` (or delete the line). The [Settings app](https://github.com/apps/settings) applies this file, and the branch protection in it, once installed on the repository.
- `.github/CODEOWNERS`: replace the owner with yours.
- `LICENSE`: your name, or your own licence.

## 2. Name your project

Every name lives in five files; nothing else in the repository needs editing. Pick a short `prefix` (lowercase, hyphens, at most 25 characters): the GCP projects will be `<prefix>-root`, `<prefix>-stag` and `<prefix>-prod`, and project ids are global, so check they're free.

| File | Set |
| --- | --- |
| `infrastructure/config/root/terraform.tfvars` | `gcp_provider_project_id` (`<prefix>-root`), region and zone, `gcp_project_prefix`, `github_repository_owner`/`_name`, `site_name`, `cloudflare_zone`, `frontend_hostnames` |
| `infrastructure/config/root/terraform.tfbackend` | `<prefix>-root-terraform-state` |
| `infrastructure/config/staging/terraform.tfvars` | `gcp_project_id` (`<prefix>-stag`), `gcp_region`, `backend_cors_origin` (`https://` + the `stag` hostname), `alert_notification_emails` |
| `infrastructure/config/staging/terraform.tfbackend` | `<prefix>-stag-terraform-state` |
| `infrastructure/config/prod/terraform.tfvars` and `.tfbackend` | the same for `prod` |

Optionally rename the Go module (`go mod edit -module` in `services/backend`, then rewrite the imports); nothing outside the repository imports it, so the template's path works as-is.

## 3. Create the root project and credentials

1. Create the GCP project `<prefix>-root` by hand and link it to your billing account. Everything else is created by Terraform.
2. In that project, create an OAuth 2.0 client ID for Google sign-in (see "Google Sign-In" in `services/frontend/README.md`) and set `google_client_id` in the root `terraform.tfvars`. Its authorized JavaScript origins are `http://localhost:5173` and both `frontend_hostnames`.
3. Create a GitHub fine-grained personal access token for the new repository with read and write access to **Variables**.
4. Create a Cloudflare API token with **Account: Cloudflare Pages: Edit**, **Zone: DNS: Edit** and **Zone: Zone: Read** on your zone, and note your Cloudflare account id.

## 4. Bootstrap the root

The only Terraform you ever run by hand. With `gcloud auth application-default login` done:

```sh
cd infrastructure/root
TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_account_id=<ID> TF_VAR_cloudflare_api_token=<TOKEN> make init
```

It creates the environment projects, the state buckets, Workload Identity Federation for this repository, the Cloudflare Pages projects and DNS records, stores the three credentials in Secret Manager, and writes the repository variables the workflows read (`WORKLOAD_IDENTITY_PROVIDER`, `SERVICE_ACCOUNT`, `GOOGLE_CLIENT_ID`, `GCP_ROOT_PROJECT`, `GCP_PROJECT_PREFIX`, `GCP_REGION`, `SITE_NAME`, `STAGING_URL`, `PRODUCTION_URL`). Then it moves its own state into the bucket it made.

## 5. Open the first pull request

Commit steps 1-3's edits on a branch and open a pull request. Now that the variables exist, it plans root, staging and prod alongside the usual checks. Merge it.

## 6. Staging deploys

The merge applies staging's infrastructure, builds and deploys the backend and frontend, runs the smoke test against `STAGING_URL`, and cuts a pre-release. The first prod deploy is promoting that pre-release to a release (see `.github/AGENTS.md`).

## 7. Make it yours

- Replace or delete the `notes` example (`services/backend/README.md`'s "Adding a resource" lists every place it lives), and the frontend pages built on it.
- Keep the decision records in [`decisions/`](decisions/README.md) that still hold, supersede the ones that don't, and number your own after the last one.
