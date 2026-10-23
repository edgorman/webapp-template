import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { GoogleSignInButton } from '../components/GoogleSignInButton'
import { useApp } from '../context/AppContext'
import { ApiError, errorMessage, isAuthor, notePath, type NoteRequest, type Visibility } from '../lib/api'

type State =
  | { phase: 'loading' }
  | { phase: 'not-found' }
  | { phase: 'forbidden' }
  | { phase: 'error'; message: string }
  | { phase: 'ready' }

const EMPTY: NoteRequest = { title: '', body: '', visibility: 'public' }

/**
 * Writes a new note at /note/new, or edits the one at /note/:id/edit. The backend decides every
 * write; the ownership check here only keeps a form off the screen for someone who would be told no.
 */
export function NoteEditor() {
  const { id } = useParams<{ id: string }>()
  const { api, user, profile, authError, authReady, renderSignInButton } = useApp()
  const navigate = useNavigate()
  const [state, setState] = useState<State>(id ? { phase: 'loading' } : { phase: 'ready' })
  const [draft, setDraft] = useState<NoteRequest>(EMPTY)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!api || !id || !profile) return
    let current = true
    api
      .getNote(id)
      .then((note) => {
        if (!current) return
        if (!isAuthor(profile, note.authorUsername)) return setState({ phase: 'forbidden' })
        setDraft({ title: note.title, body: note.body, visibility: note.visibility })
        setState({ phase: 'ready' })
      })
      .catch((e: unknown) => {
        if (!current) return
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        setState({ phase: 'error', message: errorMessage(e, 'Failed to load note') })
      })
    return () => {
      current = false
    }
  }, [api, id, profile])

  if (!api) {
    return (
      <div className="page">
        <p className="text-muted center-note">No backend deployed yet - no backend URL is configured.</p>
      </div>
    )
  }
  if (!user) {
    return (
      <div className="page center-note">
        <p className="text-muted">Sign in to write a note.</p>
        {authError ? <p role="alert">{authError}</p> : <GoogleSignInButton ready={authReady} onRender={renderSignInButton} />}
      </div>
    )
  }
  // Ownership is told by username, which arrives with the caller's profile shortly after sign-in.
  if (state.phase === 'loading' || (id && !profile)) {
    return (
      <div className="page">
        <p className="text-muted center-note">Loading…</p>
      </div>
    )
  }
  if (state.phase !== 'ready') {
    const message =
      state.phase === 'not-found' ? 'Note not found.'
      : state.phase === 'forbidden' ? "You don't have permission to edit this note."
      : state.message
    return (
      <div className="page">
        <p role="alert" className="center-note">{message}</p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }

  const save = () => {
    setSaving(true)
    setError(null)
    const write = id ? api.updateNote(id, draft) : api.createNote(draft)
    write
      .then((note) => navigate(notePath(note), { replace: true }))
      .catch((e: unknown) => {
        setError(errorMessage(e, 'Failed to save'))
        setSaving(false)
      })
  }

  const remove = () => {
    if (!id || !window.confirm('Delete this note? This cannot be undone.')) return
    setSaving(true)
    setError(null)
    api
      .deleteNote(id)
      .then(() => navigate('/', { replace: true }))
      .catch((e: unknown) => {
        setError(errorMessage(e, 'Failed to delete'))
        setSaving(false)
      })
  }

  return (
    <div className="page">
      <header className="page-header">
        <span className="page-kicker text-muted">{id ? 'Edit note' : 'New note'}</span>
        <h1 className="title-editor">{id ? 'Edit note' : 'Write something'}</h1>
      </header>

      <div className="field">
        <label htmlFor="app-title">Title</label>
        <input
          id="app-title"
          className="input"
          placeholder="Untitled note"
          value={draft.title}
          onChange={(e) => setDraft({ ...draft, title: e.target.value })}
        />
      </div>

      <div className="seg" role="radiogroup" aria-label="Visibility" style={{ marginBottom: 'var(--space-3)' }}>
        {(['public', 'private'] as Visibility[]).map((visibility) => (
          <label key={visibility} className="seg-opt">
            <input
              type="radio"
              name="visibility"
              checked={draft.visibility === visibility}
              onChange={() => setDraft({ ...draft, visibility })}
            />
            {visibility === 'public' ? 'Public' : 'Private'}
          </label>
        ))}
      </div>

      <div className="app-pane">
        <textarea
          className="app-editor"
          aria-label="Body"
          placeholder="Write something..."
          value={draft.body}
          onChange={(e) => setDraft({ ...draft, body: e.target.value })}
        />
      </div>

      {error && (
        <p role="alert" style={{ marginTop: 'var(--space-3)' }}>
          {error}
        </p>
      )}

      <div style={{ display: 'flex', gap: 'var(--space-3)', marginTop: 'var(--space-4)' }}>
        <button type="button" className="btn btn-primary" onClick={save} disabled={saving}>
          {saving ? 'Saving…' : id ? 'Save' : 'Publish'}
        </button>
        {id && (
          <button type="button" className="btn btn-ghost" onClick={remove} disabled={saving}>
            Delete
          </button>
        )}
      </div>
    </div>
  )
}
