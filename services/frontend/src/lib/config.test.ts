import { DEFAULT_SITE_NAME, resolveConfig } from './config'

// What the site falls back to with no config.json naming it.
const site = { siteUrl: window.location.origin, siteName: DEFAULT_SITE_NAME }

function mockFetch(impl: () => Promise<Partial<Response>>) {
  const fetchMock = vi.fn(impl)
  globalThis.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('resolveConfig', () => {
  it('uses every field from config.json when present', async () => {
    const config = {
      backendUrl: 'https://config.example.com',
      version: 'v1.2.3',
      environment: 'staging',
      siteUrl: 'https://staging.example.com',
      siteName: 'Example',
      githubRepo: 'owner/repo',
    }
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve(config) }))

    await expect(resolveConfig()).resolves.toEqual(config)
  })

  // vite.config.ts sets VITE_BACKEND_URL to http://api.test for every test in this suite.
  it('falls back to VITE_BACKEND_URL and the page origin, with no version or environment, when config.json is missing (404)', async () => {
    mockFetch(() => Promise.resolve({ ok: false, status: 404 }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test', ...site })
  })

  it('falls back to VITE_BACKEND_URL when config.json is not valid JSON', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.reject(new Error('bad json')) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test', ...site })
  })

  it('falls back to VITE_BACKEND_URL when config.json is empty', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({}) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test', ...site })
  })

  it('falls back to VITE_BACKEND_URL when backendUrl is not a non-empty string', async () => {
    mockFetch(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: '' }) }))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test', ...site })
  })

  it('falls back to VITE_BACKEND_URL when the fetch itself fails', async () => {
    mockFetch(() => Promise.reject(new Error('network error')))

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'http://api.test', ...site })
  })

  it('leaves version and environment undefined when they are not non-empty strings', async () => {
    mockFetch(() =>
      Promise.resolve({ ok: true, json: () => Promise.resolve({ backendUrl: 'https://x.test', version: '', environment: 42 }) }),
    )

    await expect(resolveConfig()).resolves.toEqual({ backendUrl: 'https://x.test', ...site })
  })
})
