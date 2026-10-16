# WIF identity GitHub Actions uses to authenticate to GCP without a long-lived JSON key.

resource "google_service_account" "github_actions" {
  project      = var.gcp_provider_project_id
  account_id   = "github-actions"
  display_name = "GitHub Actions Service Account"
}

# Granted per-project on root and every environment, never org- or folder-wide.
resource "google_project_iam_member" "github_actions_editor" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/editor"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# roles/editor excludes IAM management, which this root's own applies need.
resource "google_project_iam_member" "github_actions_iam_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/resourcemanager.projectIamAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# The public-invoker binding in env/cloud_run.tf needs run.services.setIamPolicy.
resource "google_project_iam_member" "github_actions_run_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firestore_database needs datastore.databases.create, which roles/editor omits.
resource "google_project_iam_member" "github_actions_datastore_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/datastore.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firebase_project needs firebase.projects.update, which roles/editor omits.
resource "google_project_iam_member" "github_actions_firebase_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/firebase.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# backend_runtime_actas in env/cloud_run.tf needs iam.serviceAccounts.setIamPolicy, which is
# distinct from actAs itself and covered by none of the roles above.
resource "google_project_iam_member" "github_actions_service_account_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/iam.serviceAccountAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Required by env/providers.tf's user_project_override, which redirects API quota off the root
# project this service account lives in.
resource "google_project_iam_member" "github_actions_service_usage_consumer" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# The backend repository's writer binding in artifact_stores.tf needs
# artifactregistry.repositories.setIamPolicy, which roles/editor excludes. repoAdmin isn't enough
# for this despite the name - it covers repository content administration (deleting images,
# managing tags, cleanup policies) but not IAM policy on the repository itself; only
# artifactregistry.admin reaches that, confirmed against staging still failing after repoAdmin
# was granted and propagation delay was ruled out.
resource "google_project_iam_member" "github_actions_artifact_registry_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/artifactregistry.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# The frontend bucket's writer binding in artifact_stores.tf needs storage.buckets.setIamPolicy,
# which roles/editor excludes the same way it excludes setIamPolicy on the resources above.
# storage.buckets.create itself needs no extra grant - that part is already covered by editor.
resource "google_project_iam_member" "github_actions_storage_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_iam_workload_identity_pool" "github_pool" {
  project                   = var.gcp_provider_project_id
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Pool"
}

resource "google_iam_workload_identity_pool_provider" "github_provider" {
  project                            = var.gcp_provider_project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub Provider"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.actor"      = "assertion.actor"
  }

  # Restricts to OIDC tokens asserting this exact repository.
  attribute_condition = "assertion.repository == '${var.github_repository_owner}/${var.github_repository_name}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github_actions_wic" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_repository_owner}/${var.github_repository_name}"
}

# Wired into Actions repository variables so workflows never hardcode them. Their presence is also
# what turns on every deploy job: until this root's first apply writes them, the workflows run only
# the PR checks (see the `changed` jobs).
locals {
  actions_variables = {
    WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github_provider.name
    SERVICE_ACCOUNT            = google_service_account.github_actions.email
    # One client ID for every environment: passed to infrastructure/env as TF_VAR_google_client_id
    # and baked into the frontend build as VITE_GOOGLE_CLIENT_ID.
    GOOGLE_CLIENT_ID   = var.google_client_id
    GCP_ROOT_PROJECT   = var.gcp_provider_project_id
    GCP_PROJECT_PREFIX = var.gcp_project_prefix
    GCP_REGION         = var.gcp_provider_region
    SITE_NAME          = var.site_name
    STAGING_URL        = "https://${var.frontend_hostnames["stag"]}"
    PRODUCTION_URL     = "https://${var.frontend_hostnames["prod"]}"
  }
}

resource "github_actions_variable" "variables" {
  for_each = local.actions_variables

  repository    = var.github_repository_name
  variable_name = each.key
  value         = each.value
}
