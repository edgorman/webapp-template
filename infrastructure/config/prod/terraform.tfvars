# Replace every value here for a new repository (docs/new-repo.md). gcp_project_id is
# <gcp_project_prefix>-prod, as infrastructure/root creates it, and the origin is this
# environment's entry in root's frontend_hostnames.
gcp_project_id      = "myapp-prod"
gcp_region          = "europe-west1"
environment         = "prod"
backend_cors_origin = "https://example.com"

# Where the monitoring alerts in infrastructure/env/monitoring.tf are sent. This grants nothing -
# it is an address to notify, not an account to admit.
alert_notification_emails = ["alerts@example.com"]
