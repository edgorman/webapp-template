# Monitoring for the backend: an uptime check against the /health endpoint of the Debug Endpoint
# Contract, alert policies on that check and on Cloud Run's own request metrics, and a dashboard
# showing all of it. Shared by both environments, so staging and prod are watched the same way and a policy that turns out to
# be noisy is discovered in staging first.

resource "google_project_service" "monitoring" {
  project = var.gcp_project_id
  service = "monitoring.googleapis.com"
}

# Where every alert in this file is sent. One channel per address rather than one channel holding
# several: a channel is what a policy references, so keeping them separate is what lets a future
# policy notify a subset without a second channel being created for the same address.
resource "google_monitoring_notification_channel" "email" {
  depends_on = [google_project_service.monitoring]

  for_each = toset(var.alert_notification_emails)

  project      = var.gcp_project_id
  display_name = "Email ${each.value} (${var.environment})"
  type         = "email"

  labels = {
    email_address = each.value
  }
}

locals {
  notification_channels = [for c in google_monitoring_notification_channel.email : c.id]

  # The uptime check addresses the Cloud Run service by hostname, so it follows the service rather
  # than a URL written down here; monitored_resource wants the bare host, without the scheme.
  backend_host = trimprefix(google_cloud_run_v2_service.backend.uri, "https://")
}

# Polls /health from Google's checker regions. This is the first thing in the deployment that
# actually exercises the Debug Endpoint Contract - until now it was only ever read by hand from
# the frontend dashboard.
resource "google_monitoring_uptime_check_config" "backend_health" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} health"
  timeout      = "10s"
  # The longest period the check offers, deliberately: the service scales to zero, so every probe
  # is a request that may cold-start an instance, and a minute-by-minute check would keep one warm
  # around the clock for no diagnostic gain.
  period = "900s"

  monitored_resource {
    type = "uptime_url"

    labels = {
      project_id = var.gcp_project_id
      host       = local.backend_host
    }
  }

  http_check {
    path           = "/health"
    port           = 443
    use_ssl        = true
    validate_ssl   = true
    request_method = "GET"

    accepted_response_status_codes {
      status_class = "STATUS_CLASS_2XX"
    }
  }

  # A 200 alone only proves something is listening on the hostname; the body is what proves the
  # backend itself answered. Matched on the status field rather than the whole payload because the
  # timestamp and commit fields change on every request by design.
  content_matchers {
    content = "\"status\":\"ok\""
    matcher = "CONTAINS_STRING"
  }
}

# Fires when the check fails from more than one region at once. More than one is the point: a
# single failing checker is far more likely to be that checker than the service, and alerting on
# it would teach the recipient to ignore this alert.
resource "google_monitoring_alert_policy" "backend_uptime" {
  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is failing its health check"
  combiner     = "OR"

  conditions {
    display_name = "Uptime check failing"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\"",
        "resource.type=\"uptime_url\"",
        "metric.label.check_id=\"${google_monitoring_uptime_check_config.backend_health.uptime_check_id}\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = 1
      duration        = "60s"

      aggregations {
        # ALIGN_NEXT_OLDER carries the last result forward across the gaps between probes, and
        # REDUCE_COUNT_FALSE then counts how many regions that result was a failure in.
        alignment_period     = "1200s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} health check failing"
    mime_type = "text/markdown"
    content   = <<-EOT
      The uptime check against `https://${local.backend_host}/health` is failing from more than one
      region. Either the Cloud Run service is down or it is answering with something other than the
      `{"status":"ok",...}` body of the Debug Endpoint Contract.

      Start with the Cloud Run revision logs for `backend-${var.environment}`: a failing deploy
      leaves the previous revision serving traffic, so a red check here usually means the running
      revision itself broke rather than that a deploy was rejected.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Cloud Run counts every response by class, so 5xx is measured here rather than derived from logs -
# there is no log-based metric to define and keep in step with the handlers.
resource "google_monitoring_alert_policy" "backend_error_rate" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is returning 5xx responses"
  combiner     = "OR"

  conditions {
    display_name = "5xx responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        "metric.label.response_code_class=\"5xx\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_error_count_threshold
      # The window itself is the duration; requiring the count to stay high for a further period
      # would only delay an alert that a single window has already established.
      duration = "0s"

      aggregations {
        # A count over the window rather than a rate: on a site this quiet a rate reads as a
        # fraction too small to hold in your head, where "more than N errors in five minutes" is
        # a threshold that can be argued about.
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} serving 5xx responses"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` returned more than ${var.alert_error_count_threshold} server
      errors in five minutes. The handlers only answer 5xx when a repository call fails, so the Cloud Run logs for the current revision name the cause directly.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Latency is measured at the 95th percentile so a single slow request cannot raise it, and the
# threshold is set well above a cold start: the service scales to zero, so the first request after
# an idle spell legitimately takes seconds and alerting under that would page for normal behaviour.
resource "google_monitoring_alert_policy" "backend_latency" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is responding slowly"
  combiner     = "OR"

  conditions {
    display_name = "95th percentile request latency"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_latencies\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_latency_threshold_ms
      # Held for two windows, so a single burst of cold starts clears itself rather than alerting.
      duration = "600s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_PERCENTILE_95"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} responding slowly"
    mime_type = "text/markdown"
    content   = <<-EOT
      The 95th percentile latency of `backend-${var.environment}` has been above
      ${var.alert_latency_threshold_ms}ms for ten minutes, which is longer than a cold start
      explains. Firestore queries are the likeliest thing here slow enough to account
      for it.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Cloud Run's own 4xx count, for the one class of client error that is not browsing: a flood of
# them is someone probing with bad tokens or hammering the API into the rate limiter. The threshold
# is high on purpose - a 404 from a mistyped or stale link is ordinary traffic and alerting on a
# handful of them would teach the recipient to ignore this alert.
resource "google_monitoring_alert_policy" "backend_client_errors" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is returning a spike of 4xx responses"
  combiner     = "OR"

  conditions {
    display_name = "4xx responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        "metric.label.response_code_class=\"4xx\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_client_error_count_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} serving a spike of 4xx responses"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` returned more than ${var.alert_client_error_count_threshold}
      client errors in five minutes. Break the request count down by `response_code` in Metrics
      Explorer first: a wall of `401`s is someone trying tokens, `429`s are the rate limiter turning
      a caller away (see the separate rate limiting alert), and `404`s on paths the API does not
      serve are a scanner.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# The narrower half of the one above: a 429 is only ever the backend's own rate limiter refusing a
# caller (internal/service/ratelimit.go), so any real number of them means someone is sending far
# more than a person at an editor or a reader on a page does.
resource "google_monitoring_alert_policy" "backend_rate_limited" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is rate limiting callers"
  combiner     = "OR"

  conditions {
    display_name = "429 responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        # response_code is an INT64 label, so it is compared as a number rather than a string.
        "metric.label.response_code=429",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_rate_limited_count_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} rate limiting callers"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` refused more than ${var.alert_rate_limited_count_threshold}
      requests with `429` in five minutes, which is its rate limiter at work: someone is calling
      the API far faster than using the site would. The limiter already bounds each caller on each
      instance, so this is a notice rather than an outage. The request logs for the window show
      which paths and which client addresses the refused requests came from.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# One place each alert above can be clicked through to, showing the same signals side by side.
resource "google_monitoring_dashboard" "backend" {
  depends_on = [google_project_service.monitoring]

  project = var.gcp_project_id
  dashboard_json = jsonencode({
    displayName = "backend-${var.environment}"
    mosaicLayout = {
      columns = 12
      tiles = [for i, widget in [
        {
          title = "Requests by response class"
          xyChart = {
            dataSets = [{
              plotType   = "STACKED_BAR"
              targetAxis = "Y1"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/request_count\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_DELTA"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.response_code_class"]
                  }
                }
              }
            }]
          }
        },
        {
          title = "95th percentile latency (ms)"
          xyChart = {
            dataSets = [{
              plotType   = "LINE"
              targetAxis = "Y1"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/request_latencies\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_PERCENTILE_95"
                    crossSeriesReducer = "REDUCE_MAX"
                    groupByFields      = ["resource.label.service_name"]
                  }
                }
              }
            }]
            thresholds = [{ value = var.alert_latency_threshold_ms }]
          }
        },
        {
          title = "Instances"
          xyChart = {
            dataSets = [{
              plotType   = "STACKED_AREA"
              targetAxis = "Y1"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/container/instance_count\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_MAX"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.state"]
                  }
                }
              }
            }]
          }
        },
        # Two tiles to a row, each half the width. The API drops zero positions and fills in
        # targetAxis = "Y1" on each dataSet, so both are written the way it stores them, or
        # every plan shows the dashboard changing when nothing has.
        ] : merge(
        { width = 6, height = 4, widget = widget },
        { for k, v in { xPos = (i % 2) * 6, yPos = floor(i / 2) * 4 } : k => v if v != 0 },
      )]
    }
  })
}
