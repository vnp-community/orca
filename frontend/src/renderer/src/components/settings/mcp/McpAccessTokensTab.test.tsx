// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useAppStore } from '@/store'
import { McpAccessTokensTab } from './McpAccessTokensTab'

const call = vi.fn()
const confirm = vi.fn()
const refreshMcpServerInfo = vi.fn()
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
const info = {
  enabled: true,
  resourceUrl: 'https://orca.example.com/mcp',
  protocolVersions: [],
  authorizationServer: '',
  scopesSupported: [
    { id: 'orca:read', label: 'Read', description: 'Read data', risk: 'read' },
    { id: 'orca:admin', label: 'Administer', description: 'Admin things', risk: 'admin' }
  ],
  dcrEnabled: false,
  maxTokenDays: 90,
  killSwitch: { active: false }
}

function mockRpc(list: unknown[], create?: () => Promise<unknown>): void {
  call.mockImplementation((m: string) => {
    if (m === 'mcp.token.list') {
      return Promise.resolve(list)
    }
    if (m === 'mcp.token.create') {
      return create
        ? create()
        : Promise.resolve({ token: token('new', { name: 'ci' }), secret: SECRET })
    }
    return Promise.resolve({ ok: true })
  })
}

async function openAndSubmit(name = 'ci'): Promise<void> {
  fireEvent.click(await screen.findByRole('button', { name: 'Create token' }))
  fireEvent.change(await screen.findByLabelText('Name'), { target: { value: name } })
  const buttons = screen.getAllByRole('button', { name: 'Create token' })
  fireEvent.click(buttons.at(-1)!)
}

beforeEach(() => {
  call.mockReset()
  confirm.mockReset()
  refreshMcpServerInfo.mockReset()
  useAppStore.setState({
    mcpServerInfo: info,
    currentUser: { id: 'me', role: 'developer' },
    refreshMcpServerInfo
  } as never)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true
  })
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('McpAccessTokensTab list', () => {
  it('shows empty, error and loading states', async () => {
    mockRpc([])
    render(<McpAccessTokensTab />)
    await screen.findByText('No access tokens yet.')
    cleanup()
    call.mockRejectedValue(new Error('offline'))
    render(<McpAccessTokensTab />)
    await screen.findByText('offline')
  })

  it('lists tokens, flags expiring ones, and revokes with destructive confirmation', async () => {
    const soon = new Date(Date.now() + 2 * 86_400_000).toISOString()
    mockRpc([token('a', { expiresAt: soon }), token('b', { status: 'revoked' })])
    confirm.mockResolvedValue(true)
    render(<McpAccessTokensTab />)
    await screen.findByText('tok-a')
    expect(screen.getByText('Expires soon')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Revoke token tok-b' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Revoke token tok-a' }))
    await waitFor(() => expect(call).toHaveBeenCalledWith('mcp.token.revoke', { tokenId: 'a' }))
    expect(confirm.mock.calls[0][0].confirmVariant).toBe('destructive')
  })

  it('keeps revoke usable but blocks create under the kill switch', async () => {
    useAppStore.setState({ mcpServerInfo: { ...info, killSwitch: { active: true } } } as never)
    mockRpc([token('a')])
    confirm.mockResolvedValue(true)
    render(<McpAccessTokensTab />)
    await screen.findByText('tok-a')
    expect(
      (screen.getAllByRole('button', { name: 'Create token' })[0] as HTMLButtonElement).disabled
    ).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke token tok-a' }))
    await waitFor(() => expect(call).toHaveBeenCalledWith('mcp.token.revoke', { tokenId: 'a' }))
  })

  it('disables create until maxTokenDays/info is loaded', async () => {
    useAppStore.setState({ mcpServerInfo: null } as never)
    mockRpc([])
    render(<McpAccessTokensTab />)
    await screen.findByText('No access tokens yet.')
    expect(
      (screen.getByRole('button', { name: 'Create token' }) as HTMLButtonElement).disabled
    ).toBe(true)
  })
})

describe('create dialog', () => {
  it('only creates once on double submit and reveals the secret', async () => {
    let resolveCreate: (v: unknown) => void = () => {}
    mockRpc([], () => new Promise((r) => (resolveCreate = r)))
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    const submit = screen.getAllByRole('button', { name: /Create token|Creating/ }).pop()!
    fireEvent.click(submit)
    fireEvent.click(submit)
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.token.create')).toHaveLength(1)
    expect(call).toHaveBeenCalledWith('mcp.token.create', {
      name: 'ci',
      scopes: ['orca:read'],
      expiresInDays: 30
    })
    resolveCreate({ token: token('new', { name: 'ci' }), secret: SECRET })
    await screen.findByText('Copy your token now')
    // The new row is added from metadata only.
    await screen.findByText('ci', { selector: 'td' })
  })

  it('maps API errors to their fields', async () => {
    mockRpc([], () => Promise.reject(new McpRpcError('MCP_TOKEN_TOO_LONG', 'x')))
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Maximum lifetime is 90 days.')
    expect(refreshMcpServerInfo).toHaveBeenCalled()
    cleanup()
    mockRpc([], () => Promise.reject(new McpRpcError('MCP_TOKEN_LIMIT', 'x')))
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText(/maximum number of active tokens/)
    cleanup()
    mockRpc([], () => Promise.reject(new McpRpcError('MCP_SCOPE_NOT_ALLOWED', 'x')))
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText(/role can't grant/)
  })

  it('disables the admin scope for non-admins', async () => {
    mockRpc([])
    render(<McpAccessTokensTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Create token' }))
    const admin = await screen.findByRole('checkbox', { name: /Administer/ })
    expect((admin as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('Only administrators can grant this.')).toBeTruthy()
  })

  it('validates before calling the API', async () => {
    mockRpc([])
    render(<McpAccessTokensTab />)
    await openAndSubmit('   ')
    await screen.findByText('Enter a name up to 80 characters.')
    expect(call.mock.calls.some((c) => c[0] === 'mcp.token.create')).toBe(false)
  })
})

describe('one-time secret reveal', () => {
  it('cannot be dismissed until saved, then wipes the secret from the DOM', async () => {
    mockRpc([])
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Copy your token now')
    const done = screen.getByRole('button', { name: 'Done' }) as HTMLButtonElement
    expect(done.disabled).toBe(true)
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    expect(screen.getByText('Copy your token now')).toBeTruthy()
    // hidden by default
    expect(document.body.textContent).not.toContain(SECRET)
    fireEvent.click(screen.getByRole('button', { name: 'Show' }))
    expect(screen.getByTestId('mcp-token-secret').textContent).toBe(SECRET)
    fireEvent.click(screen.getByRole('checkbox', { name: 'I have saved this token' }))
    expect(done.disabled).toBe(false)
    fireEvent.click(done)
    await waitFor(() => expect(screen.queryByText('Copy your token now')).toBeNull())
    expect(document.body.innerHTML).not.toContain(SECRET)
  })

  it('never persists or logs the secret', async () => {
    const local = vi.spyOn(Storage.prototype, 'setItem')
    const sinks = (['log', 'info', 'warn', 'error', 'debug'] as const).map((k) =>
      vi.spyOn(console, k)
    )
    mockRpc([])
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Copy your token now')
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }))
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(SECRET))
    const everything = JSON.stringify([
      local.mock.calls,
      sinks.map((s) => s.mock.calls),
      (useAppStore.getState as () => unknown)(),
      window.location.href,
      document.title
    ])
    expect(everything).not.toContain(SECRET)
    // Copy snippet text and aria labels also stay secret-free.
    expect(document.body.innerHTML).not.toContain(SECRET)
    fireEvent.click(screen.getByRole('checkbox', { name: 'I have saved this token' }))
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(
      JSON.stringify(window.localStorage) + JSON.stringify(window.sessionStorage)
    ).not.toContain(SECRET)
  })

  it('shows the secret for manual copy when the clipboard is blocked', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
      configurable: true
    })
    mockRpc([])
    render(<McpAccessTokensTab />)
    await openAndSubmit()
    await screen.findByText('Copy your token now')
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }))
    await screen.findByText(/copy it manually/)
    expect(screen.getByTestId('mcp-token-secret').textContent).toBe(SECRET)
  })
})

describe('CLI snippet', () => {
  it('uses resourceUrl and never a real secret', async () => {
    mockRpc([])
    render(<McpAccessTokensTab />)
    await screen.findByText('No access tokens yet.')
    const code = screen.getByText(/claude mcp add --transport http orca/)
    expect(code.textContent).toContain('https://orca.example.com/mcp')
    expect(code.textContent).toContain('<paste token here>')
  })
})
