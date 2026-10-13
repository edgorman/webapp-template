import { createContext, useContext } from 'react'
import type { GoogleUser } from '../hooks/useGoogleAuth'
import type { Api, CurrentUser } from '../lib/api'
import type { RuntimeConfig } from '../lib/config'
import type { Theme } from '../lib/theme'

export interface AppContextValue {
  user: GoogleUser | null
  api: Api | null
  authError: string | null
  authReady: boolean
  renderSignInButton: (element: HTMLElement) => void
  signOut: () => void
  theme: Theme
  toggleTheme: () => void
  /**
   * The signed-in caller's own profile (GET /users/me), or null when signed out or before it
   * loads. It is how the app learns its own username: the Google identity carries only a sub,
   * which is never addressable.
   *
   * Note authors do not come from here - a note carries its own username. Comparing this
   * profile's username with it is how the app tells its own notes (see api.ts's isAuthor).
   */
  profile: CurrentUser | null
  /** Replaces `profile` after an edit, such as a rename, that answered with the new one. */
  setProfile: (profile: CurrentUser) => void
  /** This deployment's runtime configuration (see lib/config.ts): the site's name and public URL,
   * this build's release tag, 'staging'/'production', and the GitHub repository. */
  config: RuntimeConfig
}

// Exported so tests can render `<AppContext.Provider value={...}>` with a fake value instead of
// the real Google auth / fetch wiring AppProvider sets up.
export const AppContext = createContext<AppContextValue | null>(null)

export function useApp(): AppContextValue {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useApp must be used within AppProvider')
  return ctx
}
