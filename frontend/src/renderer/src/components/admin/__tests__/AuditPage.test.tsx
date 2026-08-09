// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi, beforeEach } from 'vitest'
import { AuditPage } from '../AuditPage'
import * as adminApiClient from '../admin-api-client'

vi.mock('../admin-api-client')

const mockAudit = [
  { id: '1', createdAt: 1600000000000, action: 'login.success', userEmail: 'user@co.com', ipAddress: '10.0.0.1' },
  { id: '2', createdAt: 1600001000000, action: 'user.create', userEmail: 'admin@co.com', ipAddress: '10.0.0.2', detail: 'Created user test' }
] as any

describe('AuditPage', () => {
  beforeEach(() => {
    vi.mocked(adminApiClient.fetchAdminAudit).mockResolvedValue([...mockAudit])
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('Renders audit table với action và user email', async () => {
    render(<AuditPage />)
    await waitFor(() => {
      expect(screen.getByRole('cell', { name: 'login.success' })).toBeInTheDocument()
      expect(screen.getByText('user@co.com')).toBeInTheDocument()
    })
  })

  it('Filter bằng action dropdown → gọi fetchAdminAudit với đúng action', async () => {
    render(<AuditPage />)
    await waitFor(() => expect(screen.getByRole('cell', { name: 'login.success' })).toBeInTheDocument())
    
    // Changing the select box
    fireEvent.change(screen.getByLabelText(/Action/i), { target: { value: 'user.create' } })
    
    // Should trigger fetchAdminAudit again due to useEffect dependency
    await waitFor(() => {
      expect(adminApiClient.fetchAdminAudit).toHaveBeenCalledWith(
        expect.objectContaining({ action: 'user.create' })
      )
    })
  })

  it('Filter bằng date range → truyền đúng from/to', async () => {
    render(<AuditPage />)
    await waitFor(() => expect(screen.getByRole('cell', { name: 'login.success' })).toBeInTheDocument())
    
    fireEvent.change(screen.getByLabelText(/From/i), { target: { value: '2023-01-01' } })
    fireEvent.change(screen.getByLabelText(/To/i), { target: { value: '2023-12-31' } })
    
    await waitFor(() => {
      expect(adminApiClient.fetchAdminAudit).toHaveBeenCalledWith(
        expect.objectContaining({
          from: new Date('2023-01-01').getTime(),
          to: new Date('2023-12-31').getTime()
        })
      )
    })
  })
})
