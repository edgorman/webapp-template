import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useGoogleAuth } from '../hooks/useGoogleAuth'
import { ApiError, createApi, type CurrentUser } from '../lib/api'
import type { RuntimeConfig } from '../lib/config'
import { useTheme } from '../lib/theme'
import { AppContext, type AppContextValue } from './AppContext'

export function AppProvider({
  children,
  // Resolved once at bootstrap by main.tsx (see lib/config.ts) before this ever renders.
  config,
}: {
  children: ReactNode
  config: RuntimeConfig
}) {
  const { backendUrl } = config
  const { user, authHeaders, error, ready, renderButton, signOut, expire } = useGoogleAuth()
  const { theme, toggleTheme } = useTheme()

  const api = useMemo(
    () => (backendUrl ? createApi(backendUrl, authHeaders, expire) : null),
    [backendUrl, authHeaders, expire],
  )

  const [profile, setProfile] = useState<CurrentUser | null>(null)

  useEffect(() => {
    if (!api || !user) {
      setProfile(null)
      return
    }

    let cancelled = false
    api
      .getCurrentUser()
      // A 404 means they are signed in but hold no profile yet - a new account, or one whose
      // profile was deleted. Creating it here is what gives every signed-in user a username: the
      // body is empty, so the backend assigns the name, which they can change in AccountPanel. Anything else (offline, a 500) leaves profile null to be retried on reload.
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 404) return api.putUser({})
        throw e
      })
      .then(
        (found) => {
          if (!cancelled) setProfile(found)
        },
        () => {
          if (!cancelled) setProfile(null)
        },
      )
    return () => {
      cancelled = true
    }
  }, [api, user])

  const value: AppContextValue = {
    user,
    api,
    authError: error,
    authReady: ready,
    renderSignInButton: renderButton,
    signOut,
    theme,
    toggleTheme,
    profile,
    setProfile,
    config,
  }

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}
