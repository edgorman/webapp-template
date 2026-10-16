# App-specific APIs; the baseline set shared by every project, and the APIs behind the artifact
# stores, are enabled by infrastructure/root.
resource "google_project_service" "run" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
}

