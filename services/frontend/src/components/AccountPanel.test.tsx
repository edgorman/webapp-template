import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError, type Api, type CurrentUser } from '../lib/api'
import { fakeAppContext, renderWithApp } from '../testUtils'
import { AccountPanel } from './AccountPanel'

const user = { id: 'uid-1', email: 'a@b.com', name: 'Ada Lovelace' }
const profile: CurrentUser = {
  id: 'uid-1',
  username: 'calm-smiling-kestrel',
  bio: '',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}
const site = fakeAppContext().config

describe('AccountPanel', () => {
  it('renders neither a version link nor a staging badge when neither is set', () => {
    renderWithApp(<AccountPanel onClose={() => {}} />)
    expect(screen.queryByText('staging')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /^v/ })).not.toBeInTheDocument()
  })

  // Only staging says so, and the version is always a release tag, linked to its GitHub Releases page.
  it('shows a staging badge and a version link to the GitHub release on staging', () => {
    const config = { ...site, version: 'v1.2.3', environment: 'staging', githubRepo: 'owner/repo' }
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { config } })
    expect(screen.getByText('staging')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: 'v1.2.3' })
    expect(link).toHaveAttribute('href', 'https://github.com/owner/repo/releases/tag/v1.2.3')
    expect(link).toHaveAttribute('target', '_blank')
  })

  it('shows the version link but no staging badge on production', () => {
    const config = { ...site, version: 'v1.4.0', environment: 'production', githubRepo: 'owner/repo' }
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { config } })
    expect(screen.getByRole('link', { name: 'v1.4.0' })).toBeInTheDocument()
    expect(screen.queryByText('staging')).not.toBeInTheDocument()
    expect(screen.queryByText('production')).not.toBeInTheDocument()
  })

  it('renames the caller and keeps the profile the backend answers with', async () => {
    const renamed = { ...profile, username: 'ada' }
    const putUser = vi.fn().mockResolvedValue(renamed)
    const setProfile = vi.fn()
    const api = { putUser } as unknown as Api
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { api, user, profile, setProfile } })

    const field = screen.getByLabelText('Username')
    expect(field).toHaveValue('calm-smiling-kestrel')
    await userEvent.clear(field)
    await userEvent.type(field, 'ada')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(putUser).toHaveBeenCalledWith({ username: 'ada' })
    expect(setProfile).toHaveBeenCalledWith(renamed)
  })

  it('says why a rename was refused', async () => {
    const putUser = vi.fn().mockRejectedValue(new ApiError(409, 'username already taken'))
    const api = { putUser } as unknown as Api
    renderWithApp(<AccountPanel onClose={() => {}} />, { context: { api, user, profile } })

    await userEvent.type(screen.getByLabelText('Username'), 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('username already taken')
  })
})
