import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { AppContext, type AppContextValue } from './context/AppContext'
import type { Note } from './lib/api'

export function fakeAppContext(overrides: Partial<AppContextValue> = {}): AppContextValue {
  return {
    user: null,
    api: null,
    authError: null,
    authReady: true,
    renderSignInButton: () => {},
    signOut: () => {},
    theme: 'light',
    toggleTheme: () => {},
    profile: null,
    setProfile: () => {},
    config: { siteUrl: 'https://app.example.com', siteName: 'Example App' },
    ...overrides,
  }
}

interface Options {
  context?: Partial<AppContextValue>
  /** The URL to render at. Defaults to '/'. */
  route?: string
  /** The route pattern matched against `route`, for pages that read useParams(). Defaults to `route`. */
  path?: string
}

/** Renders a page or component inside a fake AppContext and a router, without real auth/fetch wiring. */
export function renderWithApp(ui: ReactElement, { context, route = '/', path = route }: Options = {}) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <AppContext.Provider value={fakeAppContext(context)}>
        <Routes>
          <Route path={path} element={ui} />
        </Routes>
      </AppContext.Provider>
    </MemoryRouter>,
  )
}

/** A public note with every field filled in, for a test to override what it cares about. */
export function note(overrides: Partial<Note> = {}): Note {
  return {
    id: 'note1',
    authorUsername: 'calm-smiling-kestrel',
    title: 'Untitled',
    body: 'hello',
    visibility: 'public',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
    ...overrides,
  }
}
