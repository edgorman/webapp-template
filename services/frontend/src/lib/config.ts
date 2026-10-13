/**
 * Runtime configuration written by the deploy pipeline (see the Staging Deployments section of
 * `.github/AGENTS.md`) into `config.json`, served alongside the static build, so one bundle serves
 * every environment. Every field is absent in local dev, which has no `config.json`.
 *
 * - `version`/`environment`: the release tag this commit becomes and 'staging'/'production'.
 * - `siteUrl`/`siteName`: this environment's public origin and the site's display name, from the
 *   STAGING_URL/PRODUCTION_URL and SITE_NAME repository variables.
 * - `githubRepo`: `owner/name`, for the footer and release links.
 */
export interface RuntimeConfig {
  backendUrl?: string
  version?: string
  environment?: string
  siteUrl: string
  siteName: string
  githubRepo?: string
}

/** What the site calls itself where no config.json names it - local dev and tests. */
export const DEFAULT_SITE_NAME = 'My App'

function nonEmpty(value: unknown): string | undefined {
  return typeof value === 'string' && value ? value : undefined
}

/**
 * Resolves this deployment's runtime configuration from `config.json`, in one fetch: `backendUrl`
 * falls back to the build-time `VITE_BACKEND_URL` and the site to this page's own origin and
 * DEFAULT_SITE_NAME when the file is missing, unreachable, invalid JSON, or leaves a field empty.
 * Called once at bootstrap (see main.tsx), before the first render, so every page sees settled
 * values.
 */
export async function resolveConfig(fetchImpl: typeof fetch = fetch): Promise<RuntimeConfig> {
  let config: Record<string, unknown> = {}
  try {
    const response = await fetchImpl('/config.json')
    if (response.ok) config = (await response.json()) as Record<string, unknown>
  } catch {
    // Missing, unreachable, or not valid JSON - config stays {} and every field falls back below.
  }
  return {
    backendUrl: nonEmpty(config.backendUrl) ?? import.meta.env.VITE_BACKEND_URL,
    version: nonEmpty(config.version),
    environment: nonEmpty(config.environment),
    siteUrl: nonEmpty(config.siteUrl) ?? window.location.origin,
    siteName: nonEmpty(config.siteName) ?? DEFAULT_SITE_NAME,
    githubRepo: nonEmpty(config.githubRepo),
  }
}
