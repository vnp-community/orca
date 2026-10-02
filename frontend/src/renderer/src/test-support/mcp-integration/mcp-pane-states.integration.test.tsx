// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useAppStore } from '@/store'
import { resetMcpStreamStateForTests } from '@/store/slices/mcp-slice'
import { useMcpSync } from '@/hooks/useMcpSync'
import { McpPane } from '@/components/settings/mcp/McpPane'
import { createFakeMcpBackend, mcpError, type FakeMcpBackend } from '../mcp-fake-backend'

vi.mock('@/store', async () => ({
  useAppStore: (await import('../mcp-test-store')).createMcpTestStore()
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), info: vi.fn() }) }))
vi.mock('@/components/confirmation-dialog', () => ({
  useConfirmationDialog: () => () => Promise.resolve(true)
}))

function Settings(): React.JSX.Element {
  useMcpSync()
  return <McpPane />
}

let backend: FakeMcpBackend
let uninstall: () => void
const settle = (): Promise<void> =>
  act(async () => void (await new Promise((r) => setTimeout(r, 20))))
const refreshViaVisibility = (): Promise<void> =>
  act(async () => {
    document.dispatchEvent(new Event('visibilitychange'))
    await new Promise((r) => setTimeout(r, 20))
  })
const tabs = (): string[] => screen.queryAllByRole('tab').map((t) => t.textContent ?? '')

async function mount(role: 'admin' | 'user' = 'user'): Promise<void> {
  backend.setRole(role)
  useAppStore.setState({ currentUser: { id: 'u1', role } } as never)
  render(<Settings />)
  await settle()
}

beforeEach(() => {
  resetMcpStreamStateForTests()
  backend = createFakeMcpBackend()
  uninstall = backend.install()
  useAppStore.getState().resetMcp()
  useAppStore.setState({ currentUser: null } as never)
})
afterEach(() => {
  cleanup()
  resetMcpStreamStateForTests()
  uninstall()
  vi.restoreAllMocks()
})

describe('kill switch propagation', () => {
  it('shows and clears the banner live from the event stream, without reloading', async () => {
    await mount()
    expect(backend.streamCount()).toBe(1)
    expect(screen.queryByText(/paused by an administrator/)).toBeNull()
    act(() => backend.setKillSwitch(true, 'incident 42'))
    expect(screen.getByRole('status').textContent).toContain('paused by an administrator')
    expect(screen.getByRole('status').textContent).toContain('incident 42')
    act(() => backend.setKillSwitch(false))
    expect(screen.queryByText(/paused by an administrator/)).toBeNull()
  })

  it('keeps the banner across tabs and still lets the Connect tab render', async () => {
    backend.setInfo({ killSwitch: { active: true, reason: 'maintenance' } })
    await mount()
    expect(screen.getByRole('status').textContent).toContain('maintenance')
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Access tokens' }), { button: 0 })
    fireEvent.click(screen.getByRole('tab', { name: 'Access tokens' }))
    await settle()
    expect(screen.getByRole('status').textContent).toContain('maintenance')
  })
})

describe('disabled / forbidden / unavailable states', () => {
  it('MCP_DISABLED hides the UI quietly and opens no event stream', async () => {
    backend.setInfo({ enabled: false })
    await mount()
    expect(screen.getByText('MCP is not available.')).toBeTruthy()
    expect(backend.streamCount()).toBe(0)
  })

  it('treats a gateway without mcp channels ("not yet implemented") as disabled', async () => {
    backend.failNext(
      'mcp.server.info',
      new Error('channel "mcp.server.info" is not yet implemented in backend-go')
    )
    await mount()
    expect(screen.getByText('MCP is not available.')).toBeTruthy()
    expect(backend.streamCount()).toBe(0)
  })

  it('recovers when an admin later turns MCP on (next refresh)', async () => {
    backend.setInfo({ enabled: false })
    await mount()
    backend.setInfo({ enabled: true })
    await refreshViaVisibility()
    expect(tabs()).toContain('Connect')
    expect(backend.streamCount()).toBe(1)
  })

  it('tenant off: only admins see the enable card, users see nothing actionable', async () => {
    backend.setInfo({ enabled: true, tenantEnabled: false })
    await mount('user')
    expect(screen.getByText('MCP is not available.')).toBeTruthy()
    expect(backend.calls.map((c) => c.method)).not.toContain('mcp.admin.settings.get')
    cleanup()
    resetMcpStreamStateForTests()
    useAppStore.getState().resetMcp()
    await mount('admin')
    expect(screen.getByText('MCP is turned off for your organization')).toBeTruthy()
  })

  it('a regular user gets no admin tabs and the UI never calls mcp.admin.*', async () => {
    await mount('user')
    expect(tabs()).toEqual(expect.arrayContaining(['Connect', 'Access tokens']))
    for (const admin of ['OAuth clients', 'All grants', 'Tools', 'Policies', 'Audit log']) {
      expect(tabs()).not.toContain(admin)
    }
    expect(backend.calls.filter((c) => c.method.startsWith('mcp.admin.'))).toEqual([])
    cleanup()
    resetMcpStreamStateForTests()
    useAppStore.getState().resetMcp()
    await mount('admin')
    expect(tabs()).toEqual(expect.arrayContaining(['OAuth clients', 'Policies', 'Audit log']))
  })

  it('MCP_UNAVAILABLE after load shows a retryable status and keeps the last good view', async () => {
    await mount()
    backend.failNext('mcp.server.info', mcpError('MCP_UNAVAILABLE', 'mcp-service is down'))
    await refreshViaVisibility()
    const status = screen.getByText('mcp-service is down')
    expect(status).toBeTruthy()
    expect(tabs()).toContain('Connect')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await settle()
    expect(screen.queryByText('mcp-service is down')).toBeNull()
  })
})
