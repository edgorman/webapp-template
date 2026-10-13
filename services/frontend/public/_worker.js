// Cloudflare Pages advanced-mode worker: what crawlers and link previews need that a
// JavaScript-only SPA can't give them. It lives in public/ so Vite copies it into dist/, which makes
// it part of the commit-SHA bundle frontend-publish stores and prod promotes unchanged. Everything
// it doesn't handle goes to the static assets untouched, _headers and all.
//
// It reads the same config.json frontend-deploy writes next to the bundle that the SPA does - the
// backend URL, the site's name and its canonical (production) URL - and only ever calls the backend
// anonymously, so it sees exactly what a signed-out reader can: a private note is a 404 to it, and
// its title never reaches the page.
//
// excerpt() mirrors snippetFrom in src/lib/format.ts, which this file can't import because it ships
// as-is rather than through Vite. Change both together.

// ponytail: stops the sitemap at 50 pages of 100 notes; move to a dedicated backend route if the
// site ever outgrows that.
const SITEMAP_MAX_PAGES = 50
// How long the backend gets to answer all its pages before the sitemap falls back.
const SITEMAP_TIMEOUT_MS = 10_000
// How long a colo keeps the last good sitemap to fall back to: a week.
const SITEMAP_LAST_GOOD_SECONDS = 7 * 24 * 60 * 60
// How long a note page waits for the backend before serving the plain shell: the SPA fetches the
// note itself anyway, so a slow or cold backend shouldn't hold up the page.
const NOTE_TIMEOUT_MS = 1500
// The SPA's routes that aren't notes but are worth indexing.
const STATIC_PAGES = ['/']

export default {
  async fetch(request, env) {
    const url = new URL(request.url)
    const config = await readConfig(env, url)
    // siteUrl is production's origin on every deployment, so it is also how this one tells whether
    // it is production.
    const prod = !!config.siteUrl && url.hostname === new URL(config.siteUrl).hostname
    // A deployment with no config.json still answers with absolute URLs, its own.
    config.siteUrl ||= url.origin
    const response = await route(request, env, url, config, prod)
    if (prod) return response
    // Staging (or a *.pages.dev preview) serves the same bundle and the same _headers as prod, so
    // this is the one place that can tell it apart and keep it out of search results.
    const marked = new Response(response.body, response)
    marked.headers.set('X-Robots-Tag', 'noindex')
    return marked
  },
}

async function route(request, env, url, config, prod) {
  if (url.pathname === '/robots.txt') {
    const body = prod
      ? `User-agent: *\nAllow: /\nSitemap: ${config.siteUrl}/sitemap.xml\n`
      : 'User-agent: *\nDisallow: /\n'
    return new Response(body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } })
  }
  if (url.pathname === '/sitemap.xml') return sitemap(config, url)
  const id = /^\/note\/([^/]+)\/?$/.exec(url.pathname)?.[1]
  if (id && id !== 'new' && request.method === 'GET') return noteShell(request, env, config, id)
  return env.ASSETS.fetch(request)
}

/** The SPA shell with the note's title, description, Open Graph and canonical tags in its head. */
async function noteShell(request, env, config, encodedId) {
  const [shell, note] = await Promise.all([env.ASSETS.fetch(request), fetchNote(config, encodedId)])
  if (!note || shell.status !== 200) return shell
  const headers = new Headers(shell.headers)
  headers.delete('Content-Length')
  headers.delete('ETag')
  // A function, not a string: a string replacement would expand $' and $& inside the note's title.
  const html = (await shell.text()).replace(/<title>[^<]*<\/title>/, () => noteTags(config, note))
  return new Response(html, { status: 200, headers })
}

/** The note as a signed-out reader sees it, or null for a private, missing or unreachable one. */
async function fetchNote(config, encodedId) {
  if (!config.backendUrl) return null
  try {
    const id = decodeURIComponent(encodedId)
    const signal = AbortSignal.timeout(NOTE_TIMEOUT_MS)
    const response = await fetch(`${config.backendUrl}/notes/${encodeURIComponent(id)}`, { signal })
    if (!response.ok) return null
    const note = await response.json()
    return note.visibility === 'public' ? note : null
  } catch {
    return null
  }
}

/**
 * Always a 200 sitemap: Search Console reports any error, or anything that isn't XML, as "Sitemap
 * could not be read", so a backend that's down, cold or slow falls back to the last good sitemap
 * this colo cached, or failing that to the static pages alone, rather than a 502.
 */
async function sitemap(config, url) {
  const cache = typeof caches === 'undefined' ? null : caches.default
  const lastGood = new Request(new URL('/sitemap.xml?last-good', url))
  try {
    const xml = sitemapXml(await sitemapEntries(config))
    const response = xmlResponse(xml, 3600)
    // Kept well past the hour it's served fresh, so there's something to fall back to.
    await cache?.put(lastGood, xmlResponse(xml, SITEMAP_LAST_GOOD_SECONDS)).catch(() => {})
    return response
  } catch (e) {
    console.error(`Sitemap falling back: ${e instanceof Error ? e.message : e}`)
    const cached = await cache?.match(lastGood).catch(() => undefined)
    if (cached) return xmlResponse(await cached.text(), 300)
    return xmlResponse(sitemapXml(STATIC_PAGES.map((path) => staticEntry(config, path))), 300)
  }
}

/** Every <url> line: the static pages, then each public note. Throws if the backend can't be read. */
async function sitemapEntries(config) {
  if (!config.backendUrl) throw new Error('no backend configured')
  const entries = STATIC_PAGES.map((path) => staticEntry(config, path))
  // One deadline for all the pages, so a Cloud Run cold start falls back before the crawler gives up.
  const signal = AbortSignal.timeout(SITEMAP_TIMEOUT_MS)
  let startAfter = ''
  for (let i = 0; i < SITEMAP_MAX_PAGES; i++) {
    const query = new URLSearchParams({ limit: '100' })
    if (startAfter) query.set('startAfter', startAfter)
    const response = await fetch(`${config.backendUrl}/notes?${query}`, { signal })
    if (!response.ok) throw new Error(`GET /notes answered ${response.status}`)
    const page = await response.json()
    const notes = page.notes ?? []
    for (const note of notes) {
      if (note.visibility !== 'public') continue
      const lastmod = note.updatedAt ? `<lastmod>${escapeHtml(note.updatedAt)}</lastmod>` : ''
      entries.push(`  <url><loc>${escapeHtml(noteUrl(config, note))}</loc>${lastmod}</url>`)
    }
    if (!page.hasMore || notes.length === 0) break
    startAfter = notes.at(-1).createdAt
  }
  return entries
}

function staticEntry(config, path) {
  return `  <url><loc>${escapeHtml(config.siteUrl + path)}</loc></url>`
}

function sitemapXml(entries) {
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries,
    '</urlset>',
    '',
  ].join('\n')
}

function xmlResponse(xml, maxAge) {
  return new Response(xml, {
    headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': `public, max-age=${maxAge}` },
  })
}

/** config.json's backendUrl, siteUrl and siteName, each '' when this deployment has none. */
async function readConfig(env, url) {
  let config = {}
  try {
    const response = await env.ASSETS.fetch(new URL('/config.json', url))
    if (response.ok) config = await response.json()
  } catch {
    // Missing: Pages answers the SPA shell instead, which isn't JSON.
  }
  const text = (value) => (typeof value === 'string' ? value.replace(/\/$/, '') : '')
  return { backendUrl: text(config.backendUrl), siteUrl: text(config.siteUrl), siteName: text(config.siteName) }
}

function noteUrl(config, note) {
  return `${config.siteUrl}/note/${encodeURIComponent(note.id)}`
}

function noteTags(config, note) {
  const title = note.title || '(untitled)'
  const description = excerpt(note.body ?? '')
  const href = noteUrl(config, note)
  const siteName = escapeHtml(config.siteName)
  const jsonLd = JSON.stringify({
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: title,
    description,
    url: href,
    ...(note.createdAt ? { datePublished: note.createdAt } : {}),
    ...(note.updatedAt ? { dateModified: note.updatedAt } : {}),
    ...(note.authorUsername ? { author: { '@type': 'Person', name: note.authorUsername } } : {}),
  }).replace(/</g, '\\u003c')
  return [
    `<title>${escapeHtml(title)}${siteName ? ` · ${siteName}` : ''}</title>`,
    `<meta name="description" content="${escapeHtml(description)}" />`,
    `<link rel="canonical" href="${escapeHtml(href)}" />`,
    `<meta property="og:type" content="article" />`,
    `<meta property="og:site_name" content="${siteName}" />`,
    `<meta property="og:title" content="${escapeHtml(title)}" />`,
    `<meta property="og:description" content="${escapeHtml(description)}" />`,
    `<meta property="og:url" content="${escapeHtml(href)}" />`,
    `<meta name="twitter:card" content="summary" />`,
    `<script type="application/ld+json">${jsonLd}</script>`,
  ].join('\n    ')
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function excerpt(body, max = 160) {
  const plain = body.replace(/\s+/g, ' ').trim()
  if (plain.length <= max) return plain
  return `${plain.slice(0, max - 1).trimEnd()}…`
}
