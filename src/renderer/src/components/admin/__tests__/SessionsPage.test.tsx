// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { SessionsPage } from '../SessionsPage'
import * as adminApiClient from '../admin-api-client'

vi.mock('../admin-api-client')

const mockSessions = [
  { sessionId: 's1', userId: 'u1', userEmail: 'user1@co.com', ipAddress: '127.0.0.1', createdAt: Date.now() - 3600000, lastSeenAt: Date.now() - 60000 },
  { sessionId: 's2', userId: 'u2', userEmail: 'user2@co.com', ipAddress: '192.168.1.1', createdAt: Date.now() - 7200000, lastSeenAt: Date.now() - 120000 }
] as any

describe('SessionsPage', () => {
  beforeEach(() => {
    vi.mocked(adminApiClient.fetchAdminSessions).mockResolvedValue([...mockSessions])
    vi.mocked(adminApiClient.killAdminSession).mockResolvedValue(undefined)
    window.confirm = vi.fn().mockReturnValue(true)
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('Renders sessions table với user emails', async () => {
    render(<SessionsPage />)
    await waitFor(() => {
      expect(screen.getByText('user1@co.com')).toBeInTheDocument()
      expect(screen.getByText('user2@co.com')).toBeInTheDocument()
    })
  })

  it('Kill button gọi killAdminSession với đúng sessionId', async () => {
    render(<SessionsPage />)
    await waitFor(() => expect(screen.getByText('user1@co.com')).toBeInTheDocument())
    
    const killBtns = screen.getAllByRole('button', { name: /Kill/i })
    // The first button might be "Kill All" if rendered, but "Kill All" only renders if sessions > 0
    // Let's get the specific Kill button for s1
    const killBtn1 = killBtns.find(b => b.textContent === 'Kill')!
    
    fireEvent.click(killBtn1)
    
    expect(window.confirm).toHaveBeenCalled()
    expect(adminApiClient.killAdminSession).toHaveBeenCalledWith('s1')
  })

  it('Sau kill → session bị remove khỏi list UI', async () => {
    render(<SessionsPage />)
    await waitFor(() => expect(screen.getByText('user1@co.com')).toBeInTheDocument())
    
    const killBtns = screen.getAllByRole('button', { name: 'Kill' })
    fireEvent.click(killBtns[0]) // Kills s1
    
    expect(screen.queryByText('user1@co.com')).not.toBeInTheDocument()
    expect(screen.getByText('user2@co.com')).toBeInTheDocument()
  })
})
