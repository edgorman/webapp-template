/**
 * Client for the backend API (see /services/backend). Callers pass the header map from
 * useGoogleAuth, so this module knows nothing about any auth provider.
 *
 * Wire types come from `src/gen` rather than being declared here - see packages/protos/AGENTS.md's
 * "Contract Layer". This file declares no `export interface` of its own, and CI fails a PR that
 * adds one (`root:wire-types`).
 */
import type { CurrentUser, UpdateCurrentUserRequest, User } from '../gen/app/v1/user'
import type { ListNotesParams, Note, NotePage, NoteRequest } from '../gen/app/v1/note'
import type { ErrorResponse } from '../gen/app/v1/error'

export type { User, CurrentUser } from '../gen/app/v1/user'
export type { Note, NotePage, ListNotesParams, NoteRequest } from '../gen/app/v1/note'

/**
 * "public" or "private" - what a note's `visibility` actually holds, kept as a union here because
 * the proto field is a plain string (see `packages/protos/AGENTS.md`'s "Contract Layer").
 */
export type Visibility = 'public' | 'private'

/** Thrown for any non-2xx response, carrying the status so callers can treat 404 as "absent". */
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** Message to show for a rejected request, falling back when it isn't an Error. */
export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

/** The path a note lives at in the app. The id is escaped: it arrives from the API. */
export function notePath(note: Pick<Note, 'id'>): string {
  return `/note/${encodeURIComponent(note.id)}`
}

/**
 * Whether the signed-in caller wrote something attributed to username. Usernames are unique and a
 * note resolves its author's on every read, so this is the "is it mine" check the wire leaves a
 * client - it is never sent a uid to compare. Only for what to show: the backend decides every
 * write itself.
 */
export function isAuthor(profile: CurrentUser | null, username: string): boolean {
  return !!profile && !!username && profile.username === username
}

export type AuthHeaders = Record<string, string>

/** What a request carrying a credential the backend rejected tells its caller. */
export const SESSION_EXPIRED = 'Your sign-in has expired. Sign in again to carry on.'

async function send<T>(
  baseUrl: string,
  authHeaders: AuthHeaders,
  method: string,
  path: string,
  body?: unknown,
  onUnauthorized?: () => void,
): Promise<T> {
  const response = await fetch(`${baseUrl.replace(/\/$/, '')}${path}`, {
    method,
    headers: {
      ...authHeaders,
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  // A 401 with a credential attached means the credential itself was refused - in practice, that
  // it expired - so the caller hears that rather than a raw status, and the session is ended.
  if (response.status === 401 && authHeaders.Authorization) {
    onUnauthorized?.()
    throw new ApiError(401, SESSION_EXPIRED)
  }

  if (!response.ok) {
    // Every non-2xx response is an ErrorResponse, but fall back to the status if parsing it fails.
    const message = await response
      .json()
      .then((body: ErrorResponse) => body.error)
      .catch(() => undefined)
    throw new ApiError(response.status, message ?? `Request failed with ${response.status}`)
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

/** The API path for a page of `GET /notes`, carrying only the params a caller actually set. */
function notesListPath(params: ListNotesParams = {}): string {
  const query = new URLSearchParams()
  if (params.limit !== undefined) query.set('limit', String(params.limit))
  if (params.startAfter) query.set('startAfter', params.startAfter)

  const search = query.toString()
  return search ? `/notes?${search}` : '/notes'
}

function noteApiPath(id: string): string {
  return `/notes/${encodeURIComponent(id)}`
}

export function createApi(baseUrl: string, authHeaders: AuthHeaders, onUnauthorized?: () => void) {
  // Every call below goes through this, so each one ends the session on a refused credential.
  function request<T>(method: string, path: string, body?: unknown) {
    return send<T>(baseUrl, authHeaders, method, path, body, onUnauthorized)
  }
  return {
    listNotes: (params?: ListNotesParams) => request<NotePage>('GET', notesListPath(params)),
    getNote: (id: string) => request<Note>('GET', noteApiPath(id)),
    createNote: (note: NoteRequest) => request<Note>('POST', '/notes', note),
    updateNote: (id: string, note: NoteRequest) => request<Note>('PUT', noteApiPath(id), note),
    deleteNote: (id: string) => request<void>('DELETE', noteApiPath(id)),

    // A profile is addressed by its username; the Google sub it is keyed by is never a URL.
    getUser: (username: string) => request<User>('GET', `/users/${encodeURIComponent(username)}`),
    // The caller's own profile needs no name: the backend takes the owner from the credential,
    // which is also how a client discovers the username it was given at sign-up.
    getCurrentUser: () => request<CurrentUser>('GET', '/users/me'),
    putUser: (user: Partial<UpdateCurrentUserRequest>) => request<CurrentUser>('PUT', '/users/me', user),
    deleteUser: () => request<void>('DELETE', '/users/me'),
  }
}

export type Api = ReturnType<typeof createApi>
