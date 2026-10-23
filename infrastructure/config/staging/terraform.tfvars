# Replace every value here for a new repository (docs/new-repo.md). gcp_project_id is
# <gcp_project_prefix>-stag, as infrastructure/root creates it, and the origin is this
# environment's URL: its entry in root's frontend_hostnames, else its pages.dev subdomain (root's
# frontend_subdomains output).
gcp_project_id      = "myapp-stag"
gcp_region          = "europe-west1"
environment         = "stag"
backend_cors_origin = "https://myapp-stag.pages.dev"

# Where the monitoring alerts in infrastructure/env/monitoring.tf are sent. This grants nothing -
# it is an address to notify, not an account to admit. Empty: the alerts still show in the Cloud
# Monitoring console, nobody is emailed.
alert_notification_emails = ["alerts@example.com"]
