// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { sortMcpClients } from '@/hooks/use-mcp-oauth-clients'
import { McpAllGrantsTab } from './McpAllGrantsTab'
import { McpOAuthClientsTab } from './McpOAuthClientsTab'

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

const client = (clientId: string, over: Record<string, unknown> = {}) => ({
  clientId,
  name: `Client ${clientId}`,
  redirectUris: ['https://a.example.com/cb', 'https://a.example.com/cb2'],
  registeredVia: 'dcr',
  status: 'allowed',
  createdAt: '2026-10-01T00:00:00Z',
  activeGrants: 3,
  ...over
})
const grant = (id: string, userName: string) => ({
  id,
  clientId: 'c',
  clientName: 'App',
  scopes: ['orca:read'],
  createdAt: '2026-10-01T00:00:00Z',
  status: 'active',
  userId: `u-${userName}`,
  userName
})

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
})
afterEach(cleanup)

describe('McpOAuthClientsTab', () => {
  it('sorts pending first', () => {
    expect(
      sortMcpClients([client('b'), client('a', { status: 'pending' })] as never).map(
        (c) => c.clientId
      )
    ).toEqual(['a', 'b'])
  })

  it('shows a pending banner and unique redirect hosts', async () => {
    call.mockResolvedValue([client('a', { status: 'pending' }), client('b')])
    render(<McpOAuthClientsTab />)
    await screen.findByText('1 apps waiting for approval')
    expect(screen.getAllByText('a.example.com')).toHaveLength(2)
  })

  it('blocks only after destructive confirmation and replaces the row', async () => {
    call.mockImplementation((m: string, p: { status?: string }) =>
      Promise.resolve(
        m === 'mcp.admin.client.setStatus' ? client('b', { status: p.status }) : [client('b')]
      )
    )
    confirm.mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    render(<McpOAuthClientsTab />)
    const block = await screen.findByRole('button', { name: 'Block' })
    fireEvent.click(block)
    await waitFor(() => expect(confirm).toHaveBeenCalledTimes(1))
    expect(call).not.toHaveBeenCalledWith('mcp.admin.client.setStatus', expect.anything())
    fireEvent.click(block)
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.client.setStatus', {
        clientId: 'b',
        status: 'blocked'
      })
    )
    expect(confirm.mock.calls[1][0].confirmVariant).toBe('destructive')
    await screen.findByRole('button', { name: 'Allow' })
  })

  it('allows without confirmation', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.admin.client.setStatus'
          ? client('a', { status: 'allowed' })
          : [client('a', { status: 'blocked' })]
      )
    )
    render(<McpOAuthClientsTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Allow' }))
    await screen.findByRole('button', { name: 'Block' })
    expect(confirm).not.toHaveBeenCalled()
  })

  it('shows the admin-required state on MCP_NOT_ADMIN', async () => {
    call.mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpOAuthClientsTab />)
    await screen.findByText('Administrator access required.')
  })
})

describe('McpAllGrantsTab', () => {
  it('passes userId, lists owners and filters client-side', async () => {
    call.mockResolvedValue([grant('1', 'alice'), grant('2', 'bob')])
    render(<McpAllGrantsTab userId="u-alice" />)
    await screen.findByText('alice')
    expect(call).toHaveBeenCalledWith('mcp.admin.grant.list', { userId: 'u-alice' })
    fireEvent.change(screen.getByLabelText('Filter by user or app'), { target: { value: 'bob' } })
    expect(screen.queryByText('alice')).toBeNull()
    expect(screen.getByText('bob')).toBeTruthy()
  })

  it('names the user in the revoke confirmation and uses the admin channel', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(m === 'mcp.admin.grant.revoke' ? { ok: true } : [grant('1', 'alice')])
    )
    confirm.mockResolvedValue(true)
    render(<McpAllGrantsTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Revoke access for App' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.admin.grant.revoke', { grantId: '1' })
    )
    expect(confirm.mock.calls[0][0].title).toContain('alice')
  })

  it('handles MCP_NOT_ADMIN', async () => {
    call.mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpAllGrantsTab />)
    await screen.findByText('Administrator access required.')
  })
})
