import '@testing-library/jest-dom/vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Api, Note, NotePage } from '../lib/api'
import { note, renderWithApp } from '../testUtils'
import { Landing } from './Landing'

function page(notes: Note[], hasMore = false): NotePage {
  return { notes, hasMore }
}

function fakeApi(listNotes = vi.fn().mockResolvedValue(page([]))): Api {
  return { listNotes } as unknown as Api
}

describe('Landing', () => {
  it('shows a message when no backend is configured', () => {
    renderWithApp(<Landing />, { context: { api: null } })
    expect(screen.getByText(/No backend deployed yet/)).toBeInTheDocument()
  })

  // The backend answers newest first; Landing renders the page as it arrives rather than
  // re-sorting it.
  it('renders notes in the order the backend returns them, each linked by id', async () => {
    const notes = [note({ id: 'new', title: 'New note' }), note({ id: 'old', title: 'Old note' })]
    renderWithApp(<Landing />, { context: { api: fakeApi(vi.fn().mockResolvedValue(page(notes))) } })

    const titles = await screen.findAllByRole('heading', { level: 2 })
    expect(titles.map((t) => t.textContent)).toEqual(['New note', 'Old note'])
    expect(screen.getByRole('link', { name: /New note/ })).toHaveAttribute('href', '/note/new')
  })

  it('shows an empty state when there are no notes', async () => {
    renderWithApp(<Landing />, { context: { api: fakeApi() } })
    expect(await screen.findByText(/No notes yet/)).toBeInTheDocument()
  })

  it('surfaces a load failure instead of an empty feed', async () => {
    renderWithApp(<Landing />, { context: { api: fakeApi(vi.fn().mockRejectedValue(new Error('boom'))) } })
    expect(await screen.findByText('boom')).toBeInTheDocument()
  })

  // Clicking "Load more" continues from the last note already shown, and appends rather than
  // replaces what is on screen.
  it('requests one page, then appends the next on "Load more"', async () => {
    const first = note({ id: 'first', title: 'First', createdAt: '2026-08-20T00:00:00Z' })
    const second = note({ id: 'second', title: 'Second', createdAt: '2026-01-01T00:00:00Z' })
    const listNotes = vi.fn().mockResolvedValueOnce(page([first], true)).mockResolvedValueOnce(page([second]))
    renderWithApp(<Landing />, { context: { api: fakeApi(listNotes) } })

    await userEvent.click(await screen.findByRole('button', { name: /Load more/ }))

    expect(await screen.findByText('Second')).toBeInTheDocument()
    expect(screen.getByText('First')).toBeInTheDocument()
    expect(listNotes).toHaveBeenNthCalledWith(1, { limit: 10 })
    expect(listNotes).toHaveBeenLastCalledWith({ limit: 10, startAfter: first.createdAt })
    await waitFor(() => expect(screen.queryByRole('button', { name: /Load more/ })).not.toBeInTheDocument())
  })
})
