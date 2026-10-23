# A Pages project per environment, named <gcp_project_prefix>-<env> to match the
# cloudflare_project_name inputs in push-commit.yaml / promote-release.yaml. Each is served at its
# pages.dev subdomain, plus a custom domain for every entry in frontend_hostnames.
data "cloudflare_zone" "site" {
  count = var.cloudflare_zone == "" ? 0 : 1

  filter = {
    name = var.cloudflare_zone
  }
}

resource "cloudflare_pages_project" "frontend" {
  for_each = toset(var.gcp_projects)

  account_id        = var.cloudflare_account_id
  name              = "${var.gcp_project_prefix}-${each.key}"
  production_branch = "main"
}

resource "cloudflare_pages_domain" "frontend" {
  for_each = var.frontend_hostnames

  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.frontend[each.key].name
  name         = each.value
}

# cloudflare_pages_domain validates the hostname but doesn't create the routing record. It must be
# proxied and point at the project's actual `subdomain`, or HTTPS fails with a cipher mismatch.
resource "cloudflare_dns_record" "frontend" {
  for_each = var.frontend_hostnames

  zone_id = data.cloudflare_zone.site[0].id
  # Relative to the zone; the zone apex stays the full name, which Cloudflare reads as "@".
  name    = trimsuffix(each.value, ".${var.cloudflare_zone}")
  type    = "CNAME"
  content = cloudflare_pages_project.frontend[each.key].subdomain
  proxied = true
  ttl     = 1
}
