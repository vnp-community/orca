// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { UserAvatarMenu } from '../UserAvatarMenu'

const mockUser = {
  id: 'u1',
  email: 'alice@co.com',
  name: 'Alice Smith',
  role: 'developer' as const,
  avatarUrl: 'https://github.com/avatar.png',
  teams: [],
  projects: []
}

describe('UserAvatarMenu', () => {
  afterEach(cleanup)

  it('renders user avatar image when avatarUrl is provided', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    const img = screen.getByRole('img', { name: /alice smith/i })
    expect(img).toHaveAttribute('src', 'https://github.com/avatar.png')
  })

  it('renders initials fallback when no avatarUrl', () => {
    const userNoAvatar = { ...mockUser, avatarUrl: undefined }
    render(<UserAvatarMenu user={userNoAvatar} onLogout={vi.fn()} />)
    // "Alice Smith" → "AS"
    expect(screen.getByText('AS')).toBeInTheDocument()
  })

  it('opens dropdown on avatar button click and shows email and name', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByText('alice@co.com')).toBeInTheDocument()
    expect(screen.getByText('Alice Smith')).toBeInTheDocument()
  })

  it('shows role badge in dropdown', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByText('developer')).toBeInTheDocument()
  })

  it('calls onLogout when Logout menu item is clicked', async () => {
    const onLogout = vi.fn().mockResolvedValue(undefined)
    render(<UserAvatarMenu user={mockUser} onLogout={onLogout} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    fireEvent.click(screen.getByRole('menuitem', { name: /logout/i }))
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce())
  })

  it('closes dropdown when Escape key is pressed', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('closes dropdown when clicking outside the menu', () => {
    render(
      <div>
        <UserAvatarMenu user={mockUser} onLogout={vi.fn()} />
        <div data-testid="outside">outside</div>
      </div>
    )
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.mouseDown(screen.getByTestId('outside'))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('dropdown is not visible initially', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})
