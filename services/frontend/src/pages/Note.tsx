import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { PageMeta } from '../components/PageMeta'
import { useApp } from '../context/AppContext'
import { ApiError, errorMessage, isAuthor, notePath, type Note as NoteType } from '../lib/api'
import { formatDate, snippetFrom } from '../lib/format'

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  // The backend answers a private note the caller cannot read the same way it answers a missing
  // one, so there is no separate forbidden state here to render.
  | { phase: 'not-found' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; note: NoteType }

function Message({ children, alert }: { children: string; alert?: boolean }) {
  return (
    <div className="page">
      <p className={alert ? 'center-note' : 'text-muted center-note'} role={alert ? 'alert' : undefined}>
        {children}
      </p>
      <Link to="/">← Back to feed</Link>
    </div>
  )
}

export function Note() {
  const { id } = useParams<{ id: string }>()
  const { api, profile } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })

  useEffect(() => {
    if (!api || !id) return
    setState({ phase: 'loading' })
    let current = true
    api
      .getNote(id)
      .then((note) => current && setState({ phase: 'ready', note }))
      .catch((e: unknown) => {
        if (!current) return
        if (e instanceof ApiError && e.status === 404) return setState({ phase: 'not-found' })
        setState({ phase: 'error', message: errorMessage(e, 'Failed to load note') })
      })
    return () => {
      current = false
    }
  }, [api, id])

  if (state.phase === 'unconfigured') return <Message>No backend deployed yet - no backend URL is configured.</Message>
  if (state.phase === 'loading') return <Message>Loading…</Message>
  if (state.phase === 'not-found') return <Message alert>Note not found.</Message>
  if (state.phase === 'error') return <Message alert>{state.message}</Message>

  const { note } = state
  const href = notePath(note)

  return (
    <div className="page">
      <PageMeta title={note.title || '(untitled)'} description={snippetFrom(note.body)} path={href} />
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
        <Link to="/" className="text-muted back-link">
          ← Back to feed
        </Link>
        {isAuthor(profile, note.authorUsername) && (
          <Link to={`${href}/edit`} className="btn btn-secondary">
            Edit
          </Link>
        )}
      </div>
      <header style={{ paddingBottom: 'var(--space-4)' }}>
        <div className="post-meta">
          {note.createdAt && <span className="text-muted feed-row-date">{formatDate(note.createdAt)}</span>}
          {note.visibility === 'private' && <span className="tag tag-outline">private</span>}
        </div>
        <h1 className="title-post">{note.title || '(untitled)'}</h1>
        <span className="text-muted post-author">by {note.authorUsername || 'an unnamed author'}</span>
      </header>
      <hr className="hr" />
      <div className="post-body">
        {note.body}
      </div>
    </div>
  )
}
