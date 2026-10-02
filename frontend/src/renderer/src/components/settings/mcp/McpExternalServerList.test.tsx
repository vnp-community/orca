// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { McpExternalServerList } from './McpExternalServerList'

afterEach(cleanup)

const srv = (id: string, over: Partial<McpExternalServer> = {}): McpExternalServer => ({
  id,
  scope: 'user',
  name: id,
  transport: 'http',
  url: `https://${id}.example.com/mcp`,
  envRefs: [],
  headerRefs: [],
  status: 'approved',
  toolsChanged: false,
  createdBy: 'u',
  ...over
})

const base = {
  busyIds: new Set<string>(),
  paused: false,
  onEdit: vi.fn(),
  onReview: vi.fn(),
  onDelete: vi.fn()
}

describe('McpExternalServerList', () => {
  it('shows a status badge per state and the tools-changed warning', () => {
    render(
      <McpExternalServerList
        {...base}
        isAdmin
        servers={[
          srv('a'),
          srv('b', { status: 'pending_review' }),
          srv('c', { status: 'disabled' }),
          srv('d', { toolsChanged: true })
        ]}
      />
    )
    expect(screen.getAllByText('Approved').length).toBe(2)
    expect(screen.getByText('Pending review')).toBeTruthy()
    expect(screen.getByText('Disabled')).toBeTruthy()
    expect(screen.getByText('Tools changed — re-review required')).toBeTruthy()
  })

  it('hides probe/review from non-admins and shows the waiting note', () => {
    render(
      <McpExternalServerList
        {...base}
        isAdmin={false}
        servers={[srv('mine', { status: 'pending_review' })]}
      />
    )
    expect(screen.queryByRole('button', { name: /Review server|Probe server/ })).toBeNull()
    expect(screen.getByText('Waiting for an admin to review')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Delete server mine' }))
    expect(base.onDelete).toHaveBeenCalled()
  })

  it('offers Review for pending servers and Probe for approved ones (admin)', () => {
    render(
      <McpExternalServerList
        {...base}
        isAdmin
        servers={[srv('p', { status: 'pending_review' }), srv('q')]}
      />
    )
    expect(screen.getByRole('button', { name: 'Review server p' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Probe server q' })).toBeTruthy()
  })

  it('renders health text and disables writes when paused', () => {
    render(
      <McpExternalServerList
        {...base}
        isAdmin
        paused
        servers={[
          srv('bad', {
            health: { ok: false, checkedAt: '2026-10-01T00:00:00Z', error: '<b>x</b>' }
          })
        ]}
      />
    )
    expect(screen.getByText(/Unreachable/).textContent).toContain('<b>x</b>')
    expect(
      (screen.getByRole('button', { name: 'Edit server bad' }) as HTMLButtonElement).disabled
    ).toBe(true)
  })
})
