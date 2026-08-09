// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { AdminDashboard } from '../AdminDashboard'
import * as useAdminStatsModule from '../../../hooks/useAdminStats'
import * as adminApiClient from '../admin-api-client'

vi.mock('../../../hooks/useAdminStats')
vi.mock('../admin-api-client')

describe('AdminDashboard', () => {
  beforeEach(() => {
    vi.mocked(useAdminStatsModule.useAdminStats).mockReturnValue({
      stats: { totalUsers: 10, activeSessions: 5, sshConnections: 2, pairedDevices: 4 },
      isLoading: false,
      error: null,
      refresh: vi.fn()
    })
    vi.mocked(adminApiClient.fetchAdminSessions).mockResolvedValue([
      { sessionId: 's1', userId: 'u1', userEmail: 'test@example.com', ipAddress: '127.0.0.1', createdAt: 1600000000000, lastSeenAt: 1600000000000 }
    ])
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('Loading state trước khi data', () => {
    vi.mocked(useAdminStatsModule.useAdminStats).mockReturnValue({
      stats: null,
      isLoading: true,
      error: null,
      refresh: vi.fn()
    })
    render(<AdminDashboard />)
    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument()
  })

  it('Renders stat cards với đúng số liệu sau load', async () => {
    render(<AdminDashboard />)
    expect(screen.getByText('10')).toBeInTheDocument() // Users
    expect(screen.getByText('5')).toBeInTheDocument()  // Active Sessions
    expect(screen.getByText('2')).toBeInTheDocument()  // SSH
    expect(screen.getByText('4')).toBeInTheDocument()  // Devices
  })

  it('Renders active sessions table với user email', async () => {
    render(<AdminDashboard />)
    await waitFor(() => {
      expect(screen.getByText('test@example.com')).toBeInTheDocument()
      expect(screen.getByText('127.0.0.1')).toBeInTheDocument()
    })
  })
})
