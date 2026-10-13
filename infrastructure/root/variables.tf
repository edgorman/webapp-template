variable "gcp_provider_project_id" {
  description = "The id of the root GCP project, created by hand before the first apply (docs/new-repo.md)"
  type        = string
}

variable "gcp_provider_region" {
  description = "The region of the root GCP project"
  type        = string
  default     = "europe-west1"
}

variable "gcp_provider_zone" {
  description = "The zone of the root GCP project"
  type        = string
  default     = "europe-west1-b"
}

variable "gcp_project_prefix" {
  description = "The prefix used for the environment GCP project ids"
  type        = string
}

variable "gcp_projects" {
  description = "The environment GCP projects to create, in addition to root"
  type        = list(string)
  default     = ["stag", "prod"]
}

variable "github_provider_token" {
  description = "GitHub PAT (repo scope) for the `github` provider. Passed by hand at bootstrap only; CI sources it from Secret Manager after (see gcp_secret.tf)."
  type        = string
  sensitive   = true
}

variable "github_repository_owner" {
  description = "The owner of the GitHub repository"
  type        = string
}

variable "github_repository_name" {
  description = "The name of the GitHub repository"
  type        = string
}

variable "cloudflare_account_id" {
  description = "The id of the Cloudflare account"
  type        = string
}

variable "cloudflare_api_token" {
  description = "API token granting CI/CD access to the Cloudflare account. Passed by hand at bootstrap only; CI sources it from Secret Manager after (see gcp_secret.tf)."
  type        = string
  sensitive   = true
}

variable "google_client_id" {
  description = "Google OAuth 2.0 client ID for Google Sign-In, propagated to both services via the GOOGLE_CLIENT_ID Actions variable. Not a secret - it identifies the app."
  type        = string
  default     = ""
}

variable "site_name" {
  description = "Display name of the site, shown in page titles and link previews (the SITE_NAME Actions variable)"
  type        = string
}

variable "cloudflare_zone" {
  description = "The Cloudflare zone (domain) the frontend hostnames live in, e.g. example.com"
  type        = string
}

variable "frontend_hostnames" {
  description = "Full hostname of each environment's frontend, inside cloudflare_zone, e.g. { stag = \"staging.example.com\", prod = \"example.com\" }"
  type        = map(string)
}
