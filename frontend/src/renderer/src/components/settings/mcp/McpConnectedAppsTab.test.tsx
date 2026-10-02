// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { emitMcpEvent } from '@/lib/mcp-event-bus'
import { useAppStore } from '@/store'
import { sortMcpGrants } from '@/hooks/use-mcp-grants'
import { McpConnectedAppsTab } from './McpConnectedAppsTab'

const call = vi.fn()
const confirm = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return { useAppStore: create(() => ({ mcpServerInfo: null as unknown })) }
})

const grant = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  clientId: `c-${id}`,
  clientName: `App ${id}`,
  clientUri: 'https://app.example.com/home',
  scopes: ['orca:read', 'orca:exec'],
  createdAt: '2026-10-01T00:00:00Z',
  status: 'active',
  ...over
})

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  useAppStore.setState({
    mcpServerInfo: { resourceUrl: 'https://orca.example.com/mcp', scopesSupported: [] }
  } as never)
})
afterEach(cleanup)

describe('sortMcpGrants', () => {
  it('puts active first then newest first', () => {
    const sorted = sortMcpGrants([
      grant('old', { createdAt: '2026-01-01T00:00:00Z' }),
      grant('rev', { status: 'revoked', createdAt: '2026-12-01T00:00:00Z' }),
      grant('new', { createdAt: '2026-09-01T00:00:00Z' })
    ] as never)
    expect(sorted.map((g) => g.id)).toEqual(['new', 'old', 'rev'])
  })
})

describe('McpConnectedAppsTab', () => {
  it('shows the empty state with the server address', async () => {
    call.mockResolvedValue([])
    render(<McpConnectedAppsTab />)
    await screen.findByText('No apps connected yet.')
    expect(screen.getByText('https://orca.example.com/mcp')).toBeTruthy()
  })

  it('shows an error with retry and the unavailable state', async () => {
    call.mockRejectedValueOnce(new Error('offline')).mockResolvedValue([])
    render(<McpConnectedAppsTab />)
    await screen.findByText('offline')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await screen.findByText('No apps connected yet.')
    cleanup()
    call.mockReset()
    call.mockRejectedValue(new Error('mcp.grant.list is not yet implemented'))
    render(<McpConnectedAppsTab />)
    await screen.findByText(/aren't available on this server yet/)
  })

  it('lists grants with host, high-risk marker and revoked rows without a button', async () => {
    call.mockResolvedValue([grant('a'), grant('b', { status: 'revoked' })])
    render(<McpConnectedAppsTab />)
    await screen.findByText('App a')
    expect(screen.getAllByText('app.example.com')).toHaveLength(2)
    expect(screen.getAllByLabelText('High risk').length).toBeGreaterThan(0)
    expect(screen.getByText('Revoked')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Revoke access for App b' })).toBeNull()
  })

  it('revokes only after confirmation, with an object param', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(m === 'mcp.grant.revoke' ? { ok: true } : [grant('a')])
    )
    confirm.mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    render(<McpConnectedAppsTab />)
    const btn = await screen.findByRole('button', { name: 'Revoke access for App a' })
    fireEvent.click(btn)
    await waitFor(() => expect(confirm).toHaveBeenCalledTimes(1))
    expect(call).not.toHaveBeenCalledWith('mcp.grant.revoke', expect.anything())
    fireEvent.click(btn)
    await waitFor(() => expect(call).toHaveBeenCalledWith('mcp.grant.revoke', { grantId: 'a' }))
    expect(confirm.mock.calls[1][0].confirmVariant).toBe('destructive')
    await screen.findByText('Revoked')
  })

  it('marks a grant revoked on grant.revoked events', async () => {
    call.mockResolvedValue([grant('a')])
    render(<McpConnectedAppsTab />)
    await screen.findByText('App a')
    emitMcpEvent({ type: 'grant.revoked', grantId: 'a' })
    await screen.findByText('Revoked')
  })

  it('resyncs when the grant is already gone', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.grant.revoke'
        ? Promise.reject(new McpRpcError('MCP_NOT_FOUND', 'gone'))
        : Promise.resolve([grant('a')])
    )
    confirm.mockResolvedValue(true)
    render(<McpConnectedAppsTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Revoke access for App a' }))
    await waitFor(() =>
      expect(call.mock.calls.filter((c) => c[0] === 'mcp.grant.list').length).toBeGreaterThan(1)
    )
  })
})
