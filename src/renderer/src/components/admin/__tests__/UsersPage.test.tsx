// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { UsersPage } from '../UsersPage'
import * as adminApiClient from '../admin-api-client'

vi.mock('../admin-api-client')

const mockUsers = [
  { id: '1', email: 'admin@co.com', name: 'Admin', role: 'admin', provider: 'none', isActive: true, lastLoginAt: null },
  { id: '2', email: 'dev@co.com', name: 'Dev 1', role: 'developer', provider: 'github', isActive: true, lastLoginAt: null },
  { id: '3', email: 'dev2@co.com', name: 'Dev 2', role: 'developer', provider: 'none', isActive: false, lastLoginAt: null }
] as any

describe('UsersPage', () => {
  beforeEach(() => {
    vi.mocked(adminApiClient.fetchAdminUsers).mockResolvedValue(mockUsers)
    vi.mocked(adminApiClient.deactivateAdminUser).mockResolvedValue(undefined)
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('Renders user list sau load', async () => {
    render(<UsersPage onNavigate={vi.fn()} />)
    await waitFor(() => {
      expect(screen.getByText('admin@co.com')).toBeInTheDocument()
      expect(screen.getByText('dev@co.com')).toBeInTheDocument()
    })
  })

  it('Filter bằng role select', async () => {
    render(<UsersPage onNavigate={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('admin@co.com')).toBeInTheDocument())
    
    fireEvent.change(screen.getByLabelText(/Role/i), { target: { value: 'admin' } })
    
    expect(screen.getByText('admin@co.com')).toBeInTheDocument()
    expect(screen.queryByText('dev@co.com')).not.toBeInTheDocument()
  })

  it('Filter bằng search text', async () => {
    const user = userEvent.setup()
    render(<UsersPage onNavigate={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('admin@co.com')).toBeInTheDocument())
    
    await user.type(screen.getByRole('searchbox'), 'Dev')
    
    expect(screen.queryByText('admin@co.com')).not.toBeInTheDocument()
    expect(screen.getByText('dev@co.com')).toBeInTheDocument()
    expect(screen.getByText('dev2@co.com')).toBeInTheDocument()
  })

  it('Call deactivateAdminUser khi click Deactivate', async () => {
    window.confirm = vi.fn().mockReturnValue(true)
    render(<UsersPage onNavigate={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('admin@co.com')).toBeInTheDocument())
    
    const deactivateBtns = screen.getAllByRole('button', { name: /Deactivate/i })
    fireEvent.click(deactivateBtns[0])
    
    expect(window.confirm).toHaveBeenCalled()
    expect(adminApiClient.deactivateAdminUser).toHaveBeenCalledWith('1')
  })

  it('Create User button hiển thị và gọi onNavigate', async () => {
    const onNavigate = vi.fn()
    render(<UsersPage onNavigate={onNavigate} />)
    await waitFor(() => expect(screen.getByText('admin@co.com')).toBeInTheDocument())
    
    fireEvent.click(screen.getByRole('button', { name: /Create User/i }))
    expect(onNavigate).toHaveBeenCalledWith('/users/new')
  })
})
