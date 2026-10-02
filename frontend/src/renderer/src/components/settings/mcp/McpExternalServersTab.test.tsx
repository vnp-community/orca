// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpExternalServersTab } from './McpExternalServersTab'

const call = vi.fn()
const confirm = vi.fn()
const state = { role: 'user', paused: false }
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/store', () => ({
  useAppStore: (sel: (s: unknown) => unknown) => sel({ currentUser: { role: state.role } })
}))
vi.mock('@/store/slices/mcp-slice', () => ({ selectMcpKillSwitchActive: () => state.paused }))

const srv = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  scope: 'user',
  name: id,
  transport: 'http',
  url: `https://${id}.example.com`,
  envRefs: [],
  headerRefs: [],
  status: 'approved',
  toolsChanged: false,
  createdBy: 'u',
  ...over
})

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  state.role = 'user'
  state.paused = false
})
afterEach(cleanup)

describe('McpExternalServersTab', () => {
  it('explains the difference from per-repo MCP Configs and shows the empty state', async () => {
    call.mockResolvedValue([])
    render(<McpExternalServersTab />)
    await screen.findByText(/No external MCP servers yet/)
    expect(screen.getByText(/separate from the MCP config files/)).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Add my server' })).toBeTruthy()
    expect(call).toHaveBeenCalledWith('mcp.externalServer.list', {})
  })

  it('shows an error with retry, a forbidden note, and the unavailable note', async () => {
    call.mockRejectedValueOnce(new Error('network down')).mockResolvedValue([])
    render(<McpExternalServersTab />)
    await screen.findByText('network down')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await screen.findByText(/No external MCP servers yet/)
    cleanup()
    call.mockReset()
    call.mockRejectedValue(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpExternalServersTab />)
    await screen.findByText(/do not have access/)
    cleanup()
    call.mockReset()
    call.mockRejectedValue(new McpRpcError('MCP_DISABLED', 'off'))
    render(<McpExternalServersTab />)
    await screen.findByText(/turned off/)
  })

  it('lets a user delete only after a destructive confirmation', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(m === 'mcp.externalServer.delete' ? { ok: true } : [srv('mine')])
    )
    confirm.mockResolvedValue(true)
    render(<McpExternalServersTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Delete server mine' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.externalServer.delete', { serverId: 'mine' })
    )
    expect(confirm.mock.calls[0][0].confirmVariant).toBe('destructive')
    await waitFor(() => expect(screen.queryByText('mine')).toBeNull())
  })

  it('shows admin controls, scope filter and opens the review dialog', async () => {
    state.role = 'admin'
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.externalServer.probe'
          ? { transport: 'http', digest: 'd', tools: [] }
          : [srv('p', { status: 'pending_review', scope: 'tenant' })]
      )
    )
    render(<McpExternalServersTab />)
    expect(await screen.findByRole('radio', { name: 'Organization' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Add server' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Review server p' }))
    await waitFor(() =>
      expect(call).toHaveBeenCalledWith('mcp.externalServer.probe', { serverId: 'p' })
    )
  })

  it('disables Add while MCP is paused', async () => {
    state.paused = true
    call.mockResolvedValue([])
    render(<McpExternalServersTab />)
    await screen.findByText('MCP is paused by an admin.')
    expect(
      (screen.getByRole('button', { name: 'Add my server' }) as HTMLButtonElement).disabled
    ).toBe(true)
  })
})
