import { useEffect, useState } from 'react'
import { FeedList } from '../components/FeedList'
import { PageMeta } from '../components/PageMeta'
import { useApp } from '../context/AppContext'
import { errorMessage, type Note } from '../lib/api'

const FEED_SIZE = 10

type State =
  | { phase: 'unconfigured' }
  | { phase: 'loading' }
  | { phase: 'error'; message: string }
  | { phase: 'ready'; notes: Note[]; hasMore: boolean; loadingMore: boolean; loadMoreError?: string }

/**
 * The most recent notes the caller can read, newest first, one page at a time - the backend pages
 * `GET /notes` so this never fetches more than a screen's worth in one call.
 */
export function Landing() {
  const { api } = useApp()
  const [state, setState] = useState<State>(api ? { phase: 'loading' } : { phase: 'unconfigured' })

  useEffect(() => {
    if (!api) return
    let current = true
    api
      .listNotes({ limit: FEED_SIZE })
      .then((page) => current && setState({ phase: 'ready', notes: page.notes, hasMore: page.hasMore, loadingMore: false }))
      .catch((e: unknown) => current && setState({ phase: 'error', message: errorMessage(e, 'Failed to load the feed') }))
    return () => {
      current = false
    }
  }, [api])

  const loadMore = () => {
    if (!api || state.phase !== 'ready' || state.loadingMore) return
    const cursor = state.notes.at(-1)?.createdAt
    setState({ ...state, loadingMore: true, loadMoreError: undefined })

    api
      .listNotes({ limit: FEED_SIZE, startAfter: cursor })
      .then((page) =>
        setState((prev) =>
          prev.phase === 'ready'
            ? { phase: 'ready', notes: [...prev.notes, ...page.notes], hasMore: page.hasMore, loadingMore: false }
            : prev,
        ),
      )
      .catch((e: unknown) =>
        setState((prev) =>
          prev.phase === 'ready'
            ? { ...prev, loadingMore: false, loadMoreError: errorMessage(e, 'Failed to load more notes') }
            : prev,
        ),
      )
  }

  return (
    <div className="page">
      <PageMeta path="/" />
      <header className="page-header">
        <span className="page-kicker text-muted">Feed</span>
        <h1 className="title-feed">Recent notes</h1>
      </header>

      {state.phase === 'unconfigured' && (
        <p className="text-muted">No backend deployed yet - no backend URL is configured.</p>
      )}
      {state.phase === 'loading' && <p className="text-muted">Loading…</p>}
      {state.phase === 'error' && <p role="alert">{state.message}</p>}
      {state.phase === 'ready' && state.notes.length === 0 && (
        <p className="text-muted">No notes yet. Be the first to write something.</p>
      )}
      {state.phase === 'ready' && state.notes.length > 0 && (
        <>
          <FeedList notes={state.notes} />
          {(state.hasMore || state.loadMoreError) && (
            <div className="feed-load-more">
              {state.loadMoreError && <p role="alert">{state.loadMoreError}</p>}
              {state.hasMore && (
                <button type="button" className="btn btn-ghost" onClick={loadMore} disabled={state.loadingMore}>
                  {state.loadingMore ? 'Loading…' : 'Load more'}
                </button>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
