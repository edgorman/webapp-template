import { ApiError, SESSION_EXPIRED, createApi } from './api'

const authHeaders = { Authorization: 'Bearer test-token', 'Authorization-Provider': 'google' }

function mockFetch(response: Partial<Response>) {
  const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, ...response })
  globalThis.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('createApi', () => {
  it('sends the auth headers and trims a trailing slash from the base URL', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ notes: [], hasMore: false }) })

    await createApi('https://api.example.com/', authHeaders).listNotes()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/notes')
    expect(init.method).toBe('GET')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer test-token')
    expect((init.headers as Record<string, string>)['Authorization-Provider']).toBe('google')
  })

  // Only the params a caller actually sets end up on the query string, so a bare `listNotes()`
  // still reads as a plain `/notes` request above.
  it('carries listNotes params on the query string', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ notes: [], hasMore: false }) })

    await createApi('https://api.example.com', authHeaders).listNotes({
      limit: 10,
      startAfter: '2026-08-01T00:00:00Z',
    })

    const [url] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('https://api.example.com/notes?limit=10&startAfter=2026-08-01T00%3A00%3A00Z')
  })

  it('serialises a JSON body for writes', async () => {
    const fetchMock = mockFetch({ status: 201, json: () => Promise.resolve({ id: 'note1' }) })

    await createApi('https://api.example.com', authHeaders).createNote({
      title: 'Hello',
      body: '',
      visibility: 'public',
    })

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(JSON.parse(init.body as string)).toEqual({ title: 'Hello', body: '', visibility: 'public' })
  })

  // 204 has no body, so parsing it as JSON would throw.
  it('returns without parsing a body on 204', async () => {
    mockFetch({
      status: 204,
      json: () => Promise.reject(new Error('should not be called')),
    })

    await expect(
      createApi('https://api.example.com', authHeaders).deleteNote('note1'),
    ).resolves.toBeUndefined()
  })

  // The status is what lets a caller treat "no profile yet" differently from a real failure.
  it('throws an ApiError carrying the status and the API error message', async () => {
    mockFetch({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'user not found' }),
    })

    const error = await createApi('https://api.example.com', authHeaders)
      .getUser('missing')
      .catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(404)
    expect((error as ApiError).message).toBe('user not found')
  })

  it('addresses a single note by its id', async () => {
    const fetchMock = mockFetch({ json: () => Promise.resolve({ id: 'note1' }) })
    const api = createApi('https://api.example.com', authHeaders)

    await api.getNote('note1')
    await api.updateNote('note1', { title: 't', body: '', visibility: 'private' })

    const calls = fetchMock.mock.calls as [string, RequestInit][]
    expect(calls[0][0]).toBe('https://api.example.com/notes/note1')
    expect(calls[0][1].method).toBe('GET')
    expect(calls[1][0]).toBe('https://api.example.com/notes/note1')
    expect(calls[1][1].method).toBe('PUT')
  })

  it('falls back to the status when the error body is not JSON', async () => {
    mockFetch({ ok: false, status: 502, json: () => Promise.reject(new Error('not json')) })

    const error = await createApi('https://api.example.com', authHeaders)
      .listNotes()
      .catch((e: unknown) => e)

    expect((error as ApiError).message).toContain('502')
  })

  it('ends the session and says so when the backend refuses the credential', async () => {
    mockFetch({ ok: false, status: 401, json: () => Promise.resolve({ error: 'invalid token' }) })
    const onUnauthorized = vi.fn()

    const error = await createApi('https://api.example.com', authHeaders, onUnauthorized)
      .updateNote('note1', { title: '', body: '', visibility: 'public' })
      .catch((e: unknown) => e)

    expect(onUnauthorized).toHaveBeenCalled()
    expect(error).toEqual(new ApiError(401, SESSION_EXPIRED))
  })

  it('leaves a signed-out 401 to the caller', async () => {
    mockFetch({ ok: false, status: 401, json: () => Promise.resolve({ error: 'missing bearer token' }) })
    const onUnauthorized = vi.fn()

    const error = await createApi('https://api.example.com', {}, onUnauthorized)
      .createNote({ title: '', body: '', visibility: 'public' })
      .catch((e: unknown) => e)

    expect(onUnauthorized).not.toHaveBeenCalled()
    expect((error as ApiError).message).toBe('missing bearer token')
  })
})

