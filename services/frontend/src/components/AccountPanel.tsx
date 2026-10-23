import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useApp } from '../context/AppContext'
import { errorMessage } from '../lib/api'
import { releaseUrl } from '../lib/format'
import { GoogleSignInButton } from './GoogleSignInButton'

const CloseIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 6 6 18" />
    <path d="m6 6 12 12" />
  </svg>
)

/**
 * Renames the caller. Every account is given a generated username at sign-up (see AppProvider), so
 * this is the only place one is chosen.
 */
function UsernameForm() {
  const { api, profile, setProfile } = useApp()
  const [username, setUsername] = useState(profile?.username ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!api || !profile) return null
  const changed = username.trim() !== '' && username.trim() !== profile.username

  return (
    <form
      className="field"
      onSubmit={(e) => {
        e.preventDefault()
        setSaving(true)
        setError(null)
        api
          .putUser({ username: username.trim() })
          .then(setProfile)
          .catch((err: unknown) => setError(errorMessage(err, 'Failed to save')))
          .finally(() => setSaving(false))
      }}
    >
      <label htmlFor="app-username">Username</label>
      <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
        <input id="app-username" className="input" value={username} onChange={(e) => setUsername(e.target.value)} />
        <button type="submit" className="btn btn-secondary" disabled={!changed || saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>
      {error && <p role="alert">{error}</p>}
    </form>
  )
}

interface Props {
  onClose: () => void
}

/** The account overlay opened from NavBar's account button: sign in/out, the username, a New note
 * shortcut, and (pinned to the bottom, see .panel-deploy-info) this build's release tag and
 * staging badge. */
export function AccountPanel({ onClose }: Props) {
  const { user, profile, authError, authReady, renderSignInButton, signOut, config } = useApp()
  const { version, environment, githubRepo } = config

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div className="panel-backdrop" onClick={onClose}>
      <div className="panel-drawer" role="dialog" aria-label="Account" onClick={(e) => e.stopPropagation()}>
        <div className="panel-drawer-header">
          <span className="panel-drawer-title">Account</span>
          <button type="button" className="btn btn-icon btn-secondary" aria-label="Close" onClick={onClose}>
            <CloseIcon />
          </button>
        </div>

        {user ? (
          <>
            <div className="panel-identity">
              <div className="app-avatar" aria-hidden="true">
                {(profile?.username ?? '?').charAt(0).toUpperCase()}
              </div>
              <div>
                <div className="panel-identity-name">{profile?.username ?? 'Loading…'}</div>
                <div className="text-muted panel-identity-email">{user.email}</div>
              </div>
            </div>
            {/* Keyed by the username, so the field resets to whatever the profile now holds. */}
            <UsernameForm key={profile?.username} />
            <Link to="/note/new" className="btn btn-primary btn-block" onClick={onClose}>
              New note
            </Link>
            <button type="button" className="btn btn-ghost btn-block" onClick={() => { signOut(); onClose() }}>
              Sign out
            </button>
          </>
        ) : (
          <>
            <p className="text-muted">Sign in to write and manage your notes.</p>
            {authError ? <p role="alert">{authError}</p> : <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />}
          </>
        )}

        {(version || environment === 'staging') && (
          <div className="panel-deploy-info">
            {/* Only staging says so - a production reader has no reason to be told they're on
                production. */}
            {environment === 'staging' && <span className="tag tag-outline">staging</span>}
            {version && githubRepo && (
              <a
                className="panel-version text-muted"
                href={releaseUrl(githubRepo, version)}
                target="_blank"
                rel="noopener noreferrer"
              >
                {version}
              </a>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
