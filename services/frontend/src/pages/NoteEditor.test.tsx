import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import type { Api, CurrentUser } from '../lib/api'
import { note, renderWithApp } from '../testUtils'
import { NoteEditor } from './NoteEditor'

const user = { id: 'uid-1', email: 'a@b.com', name: 'Ada' }
const profile: CurrentUser = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: '',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

// The editor navigates to the saved note, so the test renders that route too.
function Routed() {
  return (
    <Routes>
      <Route path="/note/new" element={<NoteEditor />} />
      <Route path="/note/:id/edit" element={<NoteEditor />} />
      <Route path="/note/:id" element={<p>saved note page</p>} />
    </Routes>
  )
}

describe('NoteEditor', () => {
  it('asks a signed-out caller to sign in', () => {
    renderWithApp(<NoteEditor />, { context: { api: {} as Api }, route: '/note/new' })
    expect(screen.getByText('Sign in to write a note.')).toBeInTheDocument()
  })

  it('publishes a new note and opens it', async () => {
    const createNote = vi.fn().mockResolvedValue(note({ id: 'created' }))
    const api = { createNote } as unknown as Api
    renderWithApp(<Routed />, { context: { api, user, profile }, route: '/note/new', path: '*' })

    await userEvent.type(screen.getByLabelText('Title'), 'Hello')
    await userEvent.type(screen.getByLabelText('Body'), 'World')
    await userEvent.click(screen.getByRole('radio', { name: 'Private' }))
    await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

    expect(createNote).toHaveBeenCalledWith({ title: 'Hello', body: 'World', visibility: 'private' })
    expect(await screen.findByText('saved note page')).toBeInTheDocument()
  })

  it('loads an existing note for its author and saves the edit', async () => {
    const getNote = vi.fn().mockResolvedValue(note({ title: 'Old', body: 'Body' }))
    const updateNote = vi.fn().mockResolvedValue(note())
    const api = { getNote, updateNote } as unknown as Api
    renderWithApp(<Routed />, { context: { api, user, profile }, route: '/note/note1/edit', path: '*' })

    const title = await screen.findByLabelText('Title')
    expect(title).toHaveValue('Old')
    await userEvent.clear(title)
    await userEvent.type(title, 'New')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(updateNote).toHaveBeenCalledWith('note1', { title: 'New', body: 'Body', visibility: 'public' })
  })

  // A public note owned by someone else loads fine, so ownership needs its own check.
  it('refuses to edit somebody else\'s note', async () => {
    const getNote = vi.fn().mockResolvedValue(note({ authorUsername: 'someone-else' }))
    renderWithApp(<Routed />, {
      context: { api: { getNote } as unknown as Api, user, profile },
      route: '/note/note1/edit',
      path: '*',
    })

    expect(await screen.findByRole('alert')).toHaveTextContent("You don't have permission to edit this note.")
  })
})
