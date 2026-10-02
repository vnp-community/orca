// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpConnectTab } from './McpConnectTab'

const call = vi.fn()
const confirm = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: null as unknown,
      currentUser: null as unknown,
      mcpResyncCounter: 0,
      openMcpTab: () => {}
    }))
  }
})

const info = {
  enabled: true,
  resourceUrl: 'https://orca.example.com/mcp',
  protocolVersions: ['2025-06-18'],
  authorizationServer: 'https://orca.example.com',
  scopesSupported: [],
  dcrEnabled: true,
  maxTokenDays: 30,
  killSwitch: { active: false }
}
const sess = (id: string, userId: string) => ({
  id,
  clientName: `client-${id}`,
  createdAt: '2026-10-01T00:00:00Z',
  lastSeenAt: '2026-10-01T00:00:00Z',
  protocolVersion: '2025-06-18',
  activeStreams: 1,
  toolCalls: 2,
  userId,
  userName: userId
})

let clipboard: ReturnType<typeof vi.fn>
beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  clipboard = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: clipboard },
    configurable: true
  })
  useAppStore.setState({
    mcpServerInfo: info,
    currentUser: { id: 'me', role: 'developer' }
  } as never)
})
afterEach(cleanup)

describe('McpConnectTab', () => {
  it('shows the server resourceUrl and protocol versions', async () => {
    call.mockResolvedValue([])
    render(<McpConnectTab />)
    expect((screen.getByLabelText('Server address') as HTMLInputElement).value).toBe(
      info.resourceUrl
    )
    expect(screen.getByText('2025-06-18')).toBeTruthy()
    await waitFor(() => expect(screen.getByText(/No agents are connected/)).toBeTruthy())
  })

  it('copies the matching snippet', async () => {
    call.mockResolvedValue([])
    render(<McpConnectTab />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy snippet' }))
    await waitFor(() =>
      expect(clipboard).toHaveBeenCalledWith(
        'claude mcp add --transport http orca https://orca.example.com/mcp'
      )
    )
  })

  it('warns about non-HTTPS addresses and kill switch', () => {
    call.mockResolvedValue([])
    useAppStore.setState({
      mcpServerInfo: {
        ...info,
        resourceUrl: 'http://orca.example.com/mcp',
        killSwitch: { active: true }
      }
    } as never)
    render(<McpConnectTab />)
    expect(screen.getByText(/isn't HTTPS/)).toBeTruthy()
    expect(screen.getByText(/currently blocked by an administrator/)).toBeTruthy()
  })

  it('hides the all-users toggle from regular users and shows it to admins', async () => {
    call.mockResolvedValue([])
    const { unmount } = render(<McpConnectTab />)
    await waitFor(() => expect(call).toHaveBeenCalled())
    expect(screen.queryByLabelText('Show all users')).toBeNull()
    unmount()
    useAppStore.setState({ currentUser: { id: 'me', role: 'admin' } } as never)
    render(<McpConnectTab />)
    expect(screen.getByLabelText('Show all users')).toBeTruthy()
  })

  it('disables Close on other users sessions and closes own after confirmation', async () => {
    useAppStore.setState({ currentUser: { id: 'me', role: 'admin' } } as never)
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.admin.session.list'
          ? [sess('s1', 'me'), sess('s2', 'other')]
          : m === 'mcp.session.close'
            ? { ok: true }
            : [sess('s1', 'me')]
      )
    )
    confirm.mockResolvedValue(true)
    render(<McpConnectTab />)
    await screen.findByText('client-s1')
    fireEvent.click(screen.getByLabelText('Show all users'))
    await screen.findByText('client-s2')
    const other = screen.getByRole('button', { name: 'Close session for client-s2' })
    expect((other as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Close session for client-s1' }))
    await waitFor(() => expect(call).toHaveBeenCalledWith('mcp.session.close', { sessionId: 's1' }))
    expect(confirm).toHaveBeenCalledTimes(1)
  })

  it('shows an unavailable note when the channel is not implemented', async () => {
    call.mockRejectedValue(new Error('mcp.session.list is not yet implemented'))
    render(<McpConnectTab />)
    await screen.findByText(/isn't available on this server yet/)
  })

  it('shows an error with retry', async () => {
    call.mockRejectedValueOnce(new Error('network down')).mockResolvedValue([])
    render(<McpConnectTab />)
    await screen.findByText('network down')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await screen.findByText(/No agents are connected/)
  })
})
