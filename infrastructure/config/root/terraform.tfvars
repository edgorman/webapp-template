# Replace every value here for a new repository (docs/new-repo.md). Bucket names in the three
# terraform.tfbackend files are <project id>-terraform-state and must match.
gcp_provider_project_id = "myapp-root"
gcp_provider_region     = "europe-west1"
gcp_provider_zone       = "europe-west1-b"
gcp_project_prefix      = "myapp"
gcp_projects            = ["stag", "prod"]
github_repository_owner = "your-github-user"
github_repository_name  = "your-repo"

site_name = "My App"
# No custom domain: the frontends are served at https://myapp-stag.pages.dev and
# https://myapp-prod.pages.dev. To add one, set cloudflare_zone and frontend_hostnames, e.g.
# cloudflare_zone    = "example.com"
# frontend_hostnames = { stag = "staging.example.com", prod = "example.com" }

# Created by hand once (Google Cloud Console > Credentials > OAuth client ID, in the root
# project) and set here; see services/frontend/README.md. Not a secret - the ID-token flow this
# app uses has no client secret, and the ID itself ships in the frontend bundle and every
# browser's network tab regardless.
google_client_id = ""

# The following are sensitive/unknown initially and thus have no default
# github_provider_token
# cloudflare_account_id
# cloudflare_api_token
# — pass them at apply time, e.g.:
#   TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_account_id=<ID> TF_VAR_cloudflare_api_token=<TOK> make init
