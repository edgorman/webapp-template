import { screen } from '@testing-library/react'
import { ApiError, type Api, type CurrentUser } from '../lib/api'
import { note, renderWithApp } from '../testUtils'
import { Note } from './Note'

const profile: CurrentUser = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: '',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

function renderNote(getNote: Api['getNote'], context = {}) {
  const api = { getNote } as unknown as Api
  return renderWithApp(<Note />, { context: { api, ...context }, route: '/note/note1', path: '/note/:id' })
}

describe('Note', () => {
  it('renders the note as plain text with its author', async () => {
    renderNote(vi.fn().mockResolvedValue(note({ title: 'Hello', body: '<b>not html</b>' })))

    expect(await screen.findByRole('heading', { level: 1, name: 'Hello' })).toBeInTheDocument()
    expect(screen.getByText('<b>not html</b>')).toBeInTheDocument()
    expect(screen.getByText('by calm-smiling-kestrel')).toBeInTheDocument()
  })

  it('offers Edit to the author alone', async () => {
    const { unmount } = renderNote(vi.fn().mockResolvedValue(note()), { profile })
    expect(await screen.findByRole('link', { name: 'Edit' })).toHaveAttribute('href', '/note/note1/edit')
    unmount()

    renderNote(vi.fn().mockResolvedValue(note({ authorUsername: 'someone-else' })), { profile })
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByRole('link', { name: 'Edit' })).not.toBeInTheDocument()
  })

  // A private note the caller may not read is a 404 from the backend, exactly like a missing one.
  it('says not found for a 404', async () => {
    renderNote(vi.fn().mockRejectedValue(new ApiError(404, 'note not found')))
    expect(await screen.findByText('Note not found.')).toBeInTheDocument()
  })
})
