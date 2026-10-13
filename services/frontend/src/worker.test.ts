/**
 * @vitest-environment node
 */
// public/_worker.js, the Cloudflare Pages worker that serves crawlers. Node rather than
// jsdom, for the fetch API's Request/Response the worker is written against.
// @ts-expect-error -- plain JS that ships to Cloudflare as-is, so it has no declarations.
import worker from '../public/_worker.js'
import { snippetFrom } from './lib/format'

const SHELL = '<!doctype html><html><head><title>Example App</title></head><body><div id="root"></div></body></html>'
const BACKEND = 'https://api.example.run.app'

const publicNote = {
  id: 'note1',
  title: 'Hello <world>',
  body: 'Hi\n\nSome body text.',
  visibility: 'public',
  authorUsername: 'kestrel',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-02T00:00:00Z',
}

// The static assets: config.json when the deployment has one, the SPA shell for everything else.
function env(config: object | null = { backendUrl: BACKEND, siteUrl: 'https://example.com', siteName: 'Example App' }) {
  return {
    ASSETS: {
      fetch: vi.fn(async (input: Request | URL) => {
        const path = new URL(input instanceof Request ? input.url : input).pathname
        if (path === '/config.json' && config) return Response.json(config)
        return new Response(SHELL, { headers: { 'Content-Type': 'text/html', 'Content-Security-Policy': "default-src 'self'" } })
      }),
    },
  }
}

// The backend as a signed-out reader sees it: one public note, everything else a 404.
function backend(pages: object[] = [{ notes: [publicNote], hasMore: false }]) {
  const calls: string[] = []
  global.fetch = vi.fn(async (input: string | URL | Request) => {
    const url = new URL(String(input))
    calls.push(url.pathname + url.search)
    if (url.pathname === `/notes/${publicNote.id}`) return Response.json(publicNote)
    if (url.pathname === '/notes') return Response.json(pages[calls.filter((c) => c.startsWith('/notes?')).length - 1])
    return Response.json({ error: 'note not found' }, { status: 404 })
  }) as typeof fetch
  return calls
}

const get = (url: string, e = env()) => worker.fetch(new Request(url), e)

// Cloudflare's caches.default, which Node doesn't have: a Map keyed by URL.
function cache() {
  const store = new Map<string, Response>()
  ;(global as unknown as { caches: unknown }).caches = {
    default: {
      put: async (req: Request, res: Response) => void store.set(req.url, res),
      match: async (req: Request) => store.get(req.url)?.clone(),
    },
  }
  return store
}

const STATIC_ONLY = [
  '<?xml version="1.0" encoding="UTF-8"?>',
  '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
  '  <url><loc>https://example.com/</loc></url>',
  '</urlset>',
  '',
].join('\n')

describe('_worker.js', () => {
  afterEach(() => {
    delete (global as unknown as { caches?: unknown }).caches
    vi.restoreAllMocks()
  })

  it('injects the note title, description, Open Graph and canonical tags into the shell', async () => {
    backend()
    const response = await get('https://example.com/note/note1')
    const html = await response.text()

    expect(html).toContain('<title>Hello &lt;world&gt; · Example App</title>')
    expect(html).not.toContain('<title>Example App</title>')
    expect(html).toContain('<meta name="description" content="Hi Some body text." />')
    expect(html).toContain('<meta property="og:title" content="Hello &lt;world&gt;" />')
    expect(html).toContain('<meta property="og:type" content="article" />')
    expect(html).toContain('<meta property="og:url" content="https://example.com/note/note1" />')
    expect(html).toContain('<link rel="canonical" href="https://example.com/note/note1" />')
    expect(html).toContain('"@type":"Article"')
    // The shell's own headers (the CSP from _headers) survive the rewrite.
    expect(response.headers.get('Content-Security-Policy')).toBe("default-src 'self'")
    expect(response.headers.get('X-Robots-Tag')).toBeNull()
  })

  it('leaves the shell untouched for a private or missing note, so nothing about it leaks', async () => {
    backend()
    const html = await (await get('https://example.com/note/secret')).text()
    expect(html).toBe(SHELL)
  })

  it('leaves the shell untouched when the deployment has no backend', async () => {
    backend()
    const html = await (await get('https://example.com/note/note1', env(null))).text()
    expect(html).toBe(SHELL)
  })

  // A string replacement would expand $' into the rest of the shell and $& into the matched <title>.
  it("keeps $' and $& in a title literal", async () => {
    backend()
    global.fetch = vi.fn(async () => Response.json({ ...publicNote, title: "What $' and $& mean in sed" })) as typeof fetch
    const html = await (await get('https://example.com/note/note1')).text()
    expect(html).toContain("<title>What $&#39; and $&amp; mean in sed · Example App</title>")
    expect(html.match(/<body>/g)).toHaveLength(1)
  })

  it('serves the plain shell once the note lookup misses its deadline', async () => {
    const deadline = new AbortController()
    const timeout = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(deadline.signal)
    global.fetch = vi.fn(
      (_input: unknown, init?: RequestInit) =>
        new Promise((_resolve, reject) => init?.signal?.addEventListener('abort', () => reject(init.signal?.reason))),
    ) as typeof fetch
    const pending = get('https://example.com/note/note1')
    await new Promise((resolve) => setTimeout(resolve, 0))
    deadline.abort(new DOMException('timed out', 'TimeoutError'))
    expect(await (await pending).text()).toBe(SHELL)
    expect(timeout).toHaveBeenCalledWith(1500)
  })

  it('allows crawling on prod and names the sitemap', async () => {
    const response = await get('https://example.com/robots.txt')
    expect(await response.text()).toBe('User-agent: *\nAllow: /\nSitemap: https://example.com/sitemap.xml\n')
  })

  it('marks every staging response noindex and disallows everything in its robots.txt', async () => {
    backend()
    const robots = await get('https://staging.example.com/robots.txt')
    expect(await robots.text()).toBe('User-agent: *\nDisallow: /\n')
    expect(robots.headers.get('X-Robots-Tag')).toBe('noindex')

    const asset = await get('https://staging.example.com/assets/index.js')
    expect(asset.headers.get('X-Robots-Tag')).toBe('noindex')
    expect(asset.headers.get('Content-Security-Policy')).toBe("default-src 'self'")
  })

  it('lists every public note, across pages, with its lastmod', async () => {
    const second = { ...publicNote, id: 'older', updatedAt: '2026-07-01T00:00:00Z', createdAt: '2026-07-01T00:00:00Z' }
    const calls = backend([
      { notes: [publicNote], hasMore: true },
      { notes: [second, { ...second, id: 'mine', visibility: 'private' }], hasMore: false },
    ])
    const response = await get('https://example.com/sitemap.xml')
    const xml = await response.text()

    expect(response.headers.get('Content-Type')).toBe('application/xml; charset=utf-8')
    expect(calls).toEqual(['/notes?limit=100', `/notes?limit=100&startAfter=${encodeURIComponent(publicNote.createdAt)}`])
    expect(xml).toBe(
      [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
        '  <url><loc>https://example.com/</loc></url>',
        '  <url><loc>https://example.com/note/note1</loc><lastmod>2026-08-02T00:00:00Z</lastmod></url>',
        '  <url><loc>https://example.com/note/older</loc><lastmod>2026-07-01T00:00:00Z</lastmod></url>',
        '</urlset>',
        '',
      ].join('\n'),
    )
  })

  // Search Console reports any non-XML answer as "Sitemap could not be read".
  it('serves the last good sitemap when the backend fails', async () => {
    const store = cache()
    backend()
    const good = await (await get('https://example.com/sitemap.xml')).text()
    expect(good).toContain('/note/note1')
    expect(await store.get('https://example.com/sitemap.xml?last-good')?.clone().text()).toBe(good)
    expect(store.get('https://example.com/sitemap.xml?last-good')?.headers.get('Cache-Control')).toBe('public, max-age=604800')

    global.fetch = vi.fn(async () => new Response('boom', { status: 500 })) as typeof fetch
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const response = await get('https://example.com/sitemap.xml')
    expect(response.status).toBe(200)
    expect(response.headers.get('Content-Type')).toBe('application/xml; charset=utf-8')
    expect(response.headers.get('Cache-Control')).toBe('public, max-age=300')
    expect(await response.text()).toBe(good)
  })

  it('serves the static pages when the backend fails and nothing is cached', async () => {
    cache()
    global.fetch = vi.fn(async () => { throw new TypeError('fetch failed') }) as typeof fetch
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const response = await get('https://example.com/sitemap.xml')
    expect(response.status).toBe(200)
    expect(response.headers.get('Content-Type')).toBe('application/xml; charset=utf-8')
    expect(await response.text()).toBe(STATIC_ONLY)
  })

  it('serves the static pages without a backend or a Cache API', async () => {
    backend()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const response = await get('https://example.com/sitemap.xml', env(null))
    expect(response.status).toBe(200)
    expect(await response.text()).toBe(STATIC_ONLY)
  })

  it('falls back once the backend misses its deadline', async () => {
    cache()
    const deadline = new AbortController()
    const timeout = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(deadline.signal)
    // A cold backend: it never answers, and only the deadline ends the wait.
    global.fetch = vi.fn(
      (_input: unknown, init?: RequestInit) =>
        new Promise((_resolve, reject) => init?.signal?.addEventListener('abort', () => reject(init.signal?.reason))),
    ) as typeof fetch
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const pending = get('https://example.com/sitemap.xml')
    await new Promise((resolve) => setTimeout(resolve, 0))
    deadline.abort(new DOMException('timed out', 'TimeoutError'))
    const response = await pending
    expect(timeout).toHaveBeenCalledWith(10_000)
    expect(response.status).toBe(200)
    expect(await response.text()).toBe(STATIC_ONLY)
  })

  // The worker keeps its own copy of excerpt(); this keeps it describing a note the way the SPA does.
  it('describes a note the same way the SPA does', async () => {
    const body = `line one\n\n  line two ${'word '.repeat(60)}`
    backend()
    global.fetch = vi.fn(async () => Response.json({ ...publicNote, body })) as typeof fetch
    const html = await (await get('https://example.com/note/note1')).text()
    expect(html).toContain(`<meta name="description" content="${snippetFrom(body)}" />`)
    expect(snippetFrom(body).length).toBeLessThanOrEqual(160)
  })
})
