// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useAppStore } from '@/store'
import { McpAccessTokensTab } from './McpAccessTokensTab'

const call = vi.fn()
const confirm = vi.fn()
const track = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/lib/telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))
vi.mock('@/components/confirmation-dialog', () => ({ useConfirmationDialog: () => confirm }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: null as unknown,
      currentUser: null as unknown,
      refreshMcpServerInfo: () => {}
    }))
  }
})

const SECRET = 'orca_pat_SUPER_SECRET_VALUE_123'
const token = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  name: `tok-${id}`,
  scopes: ['orca:read'],
  createdAt: '2026-10-01T00:00:00Z',
  expiresAt: '2030-10-01T00:00:00Z',
  status: 'active',
  ...over
})
const info = (maxTokenDays: number) => ({
  enabled: true,
  resourceUrl: 'https://orca.example.com/mcp',
  protocolVersions: [],
  authorizationServer: '',
  scopesSupported: [{ id: 'orca:read', label: 'Read', description: 'Read data', risk: 'read' }],
  dcrEnabled: false,
  maxTokenDays,
  killSwitch: { active: false }
})

function mockRpc(create: () => Promise<unknown>): void {
  call.mockImplementation((m: string) => {
    if (m === 'mcp.token.list') {
      return Promise.resolve([token('a')])
    }
    return m === 'mcp.token.create' ? create() : Promise.resolve({ ok: true })
  })
}

async function openAndSubmit(): Promise<void> {
  fireEvent.click(await screen.findByRole('button', { name: 'Create token' }))
  fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'ci-SECRET-NAME' } })
  fireEvent.click(screen.getAllByRole('button', { name: 'Create token' }).at(-1)!)
}

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  track.mockReset()
  useAppStore.setState({
    mcpServerInfo: info(90),
    currentUser: { id: 'me', role: 'developer' },
    refreshMcpServerInfo: () => {}
  } as never)
})
afterEach(cleanup)

describe('access token telemetry', () => {
  it('reports lifetime and scope buckets on create, never the secret or name', async () => {
    mockRpc(() =>
      Promise.resolve({ token: token('new', { name: 'ci-SECRET-NAME' }), secret: SECRET })
    )
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Copy your token now')
    expect(track).toHaveBeenCalledTimes(1)
    expect(track).toHaveBeenCalledWith('mcp_token_created', {
      lifetime_bucket: '<=30d',
      scope_count: '1'
    })
    const dump = JSON.stringify(track.mock.calls)
    for (const leak of [SECRET, 'ci-SECRET-NAME', 'orca:read', 'new']) {
      expect(dump).not.toContain(leak)
    }
  })

  it('does not report a failed create', async () => {
    mockRpc(() => Promise.reject(new McpRpcError('MCP_TOKEN_LIMIT', 'x')))
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText(/maximum number of active tokens/)
    expect(track).not.toHaveBeenCalled()
  })

  it('reports a revoke with an empty payload (no token id or name)', async () => {
    mockRpc(() => Promise.resolve({}))
    confirm.mockResolvedValue(true)
    render(<McpAccessTokensTab />)
    await screen.findByText('tok-a')
    fireEvent.click(screen.getByRole('button', { name: 'Revoke token tok-a' }))
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1))
    expect(track).toHaveBeenCalledWith('mcp_token_revoked', {})
    expect(JSON.stringify(track.mock.calls)).not.toContain('tok-a')
  })
})

describe('token lifetime cap after MCP_TOKEN_TOO_LONG', () => {
  it('shows the refreshed cap, not the stale one', async () => {
    mockRpc(() => Promise.reject(new McpRpcError('MCP_TOKEN_TOO_LONG', 'x')))
    // The rejected create triggers a server.info refresh that lowers the cap.
    useAppStore.setState({
      refreshMcpServerInfo: () => useAppStore.setState({ mcpServerInfo: info(30) } as never)
    } as never)
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Maximum lifetime is 30 days.')
    expect(screen.queryByText('Maximum lifetime is 90 days.')).toBeNull()
    expect(track).not.toHaveBeenCalled()
  })
})
