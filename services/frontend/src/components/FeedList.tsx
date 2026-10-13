import { Link } from 'react-router-dom'
import { notePath, type Note } from '../lib/api'
import { formatDate, snippetFrom } from '../lib/format'

/** The newest-first list of notes on the landing page. */
export function FeedList({ notes }: { notes: Note[] }) {
  return (
    <div>
      {notes.map((note, i) => (
        <FeedRow key={note.id} note={note} delayMs={i * 20} />
      ))}
    </div>
  )
}

function FeedRow({ note, delayMs }: { note: Note; delayMs: number }) {
  return (
    <Link to={notePath(note)} className="feed-row" style={{ animationDelay: `${delayMs}ms` }}>
      <div className="feed-row-meta">
        {/* Optional because the wire says so: createdAt is a google.protobuf.Timestamp, and
            protojson may leave a message field out of the body. */}
        {note.createdAt && <span className="text-muted feed-row-date">{formatDate(note.createdAt)}</span>}
        {note.authorUsername && <span className="text-muted feed-row-author">{note.authorUsername}</span>}
        {note.visibility === 'private' && <span className="tag tag-outline">private</span>}
      </div>
      <h2 className="feed-title">{note.title || '(untitled)'}</h2>
      <p className="text-muted feed-snippet">{snippetFrom(note.body)}</p>
    </Link>
  )
}
