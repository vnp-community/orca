// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpAccessTokensTab } from '@/components/settings/mcp/McpAccessTokensTab'
import { createFakeMcpBackend, mcpError, type FakeMcpBackend } from '../mcp-fake-backend'

vi.mock('@/store', async () => ({
  useAppStore: (await import('../mcp-test-store')).createMcpTestStore()
}))
vi.mock('@/components/confirmation-dialog', () => ({
  useConfirmationDialog: () => () => Promise.resolve(true)
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }))

let backend: FakeMcpBackend
let uninstall: () => void

beforeEach(async () => {
  backend = createFakeMcpBackend({ role: 'user' })
  uninstall = backend.install()
  useAppStore.setState({ currentUser: { id: 'u1', role: 'user' } } as never)
  await useAppStore.getState().refreshMcpServerInfo()
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true
  })
})
afterEach(() => {
  cleanup()
  uninstall()
  vi.restoreAllMocks()
})

async function createToken(name: string): Promise<void> {
  fireEvent.click(await screen.findByRole('button', { name: 'Create token' }))
  fireEvent.change(await screen.findByLabelText('Name'), { target: { value: name } })
  fireEvent.click(screen.getAllByRole('button', { name: 'Create token' }).at(-1)!)
}

describe('PAT create / reveal / revoke against the fake backend', () => {
  it('reveals the secret once and never lets it reach store, storage, console or URL', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const sinks = (['log', 'info', 'warn', 'error', 'debug'] as const).map((k) =>
      vi.spyOn(console, k)
    )
    render(<McpAccessTokensTab />)
    await createToken('ci')
    await screen.findByText('Copy your token now')
    const [secret] = backend.allSecrets()
    expect(secret).toMatch(/^orca_pat_/)

    // Hidden until "Show", copied via clipboard only.
    expect(document.body.innerHTML).not.toContain(secret)
    fireEvent.click(screen.getByRole('button', { name: 'Show' }))
    expect(screen.getByTestId('mcp-token-secret').textContent).toBe(secret)
    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }))
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(secret))

    const leakSurface = (): string =>
      JSON.stringify([
        useAppStore.getState(),
        setItem.mock.calls,
        sinks.map((s) => s.mock.calls),
        window.localStorage,
        window.sessionStorage,
        window.location.href,
        document.title
      ])
    expect(leakSurface()).not.toContain(secret)

    fireEvent.click(screen.getByRole('checkbox', { name: 'I have saved this token' }))
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByText('Copy your token now')).toBeNull())
    expect(document.body.innerHTML).not.toContain(secret)
    expect(leakSurface()).not.toContain(secret)
    // Row is metadata only.
    expect(screen.getByTestId('mcp-token-tok-1').textContent).not.toContain(secret)
  })

  it('a reload (fresh mount) lists the token without the secret, and the list RPC never carries it', async () => {
    const first = render(<McpAccessTokensTab />)
    await createToken('ci')
    await screen.findByText('Copy your token now')
    const [secret] = backend.allSecrets()
    first.unmount()
    cleanup()
    render(<McpAccessTokensTab />)
    await screen.findByText('ci', { selector: 'td' })
    expect(document.body.innerHTML).not.toContain(secret)
    expect(await backend.bridge.call('mcp.token.list')).not.toContainEqual(
      expect.objectContaining({ secret })
    )
    expect(JSON.stringify(await backend.bridge.call('mcp.token.list'))).not.toContain(secret)
  })

  it('revoke flips the row and the server stops accepting the PAT', async () => {
    render(<McpAccessTokensTab />)
    await createToken('ci')
    await screen.findByText('Copy your token now')
    const [secret] = backend.allSecrets()
    expect(backend.patAccepted(secret)).toBe(true)
    fireEvent.click(screen.getByRole('checkbox', { name: 'I have saved this token' }))
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Revoke token ci' }))
    await waitFor(() => expect(backend.patAccepted(secret)).toBe(false))
    await screen.findByText('Revoked')
    expect(screen.queryByRole('button', { name: 'Revoke token ci' })).toBeNull()
  })

  it('maps MCP_TOKEN_TOO_LONG and re-reads the lifetime cap from server.info', async () => {
    render(<McpAccessTokensTab />)
    await screen.findByText('No access tokens yet.')
    // The admin lowers the cap after this page loaded server.info (cap 90): the stale 30d default
    // is rejected, then the UI re-reads server.info and shows the new cap.
    backend.setInfo({ maxTokenDays: 7 })
    await createToken('ci')
    await screen.findByText(/Maximum lifetime is \d+ days\./)
    await waitFor(() => expect(useAppStore.getState().mcpServerInfo?.maxTokenDays).toBe(7))
    expect(backend.allSecrets()).toHaveLength(0)
  })

  it('blocks create under the kill switch but keeps revoke working', async () => {
    const { token } = await backend.bridge.call('mcp.token.create', {
      name: 'old',
      scopes: ['orca:read'],
      expiresInDays: 7
    })
    backend.setKillSwitch(true, 'incident')
    useAppStore.getState().applyMcpEvent({ type: 'killswitch.changed', active: true })
    render(<McpAccessTokensTab />)
    await screen.findByText('old', { selector: 'td' })
    expect(
      (screen.getAllByRole('button', { name: 'Create token' })[0] as HTMLButtonElement).disabled
    ).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Revoke token old' }))
    await waitFor(() => expect(backend.patAccepted(backend.allSecrets()[0])).toBe(false))
    expect(token.id).toBeTruthy()
  })

  it('shows forbidden / unavailable / generic errors from the list call', async () => {
    backend.failNext('mcp.token.list', mcpError('MCP_NOT_ADMIN', 'nope'))
    render(<McpAccessTokensTab />)
    await screen.findByText('nope')
    cleanup()
    backend.failNext(
      'mcp.token.list',
      new Error('channel "mcp.token.list" is not yet implemented in backend-go')
    )
    render(<McpAccessTokensTab />)
    await screen.findByText("Access tokens aren't available on this server yet.")
    cleanup()
    backend.failNext('mcp.token.list', mcpError('MCP_UNAVAILABLE', 'mcp-service down'))
    render(<McpAccessTokensTab />)
    await screen.findByText('mcp-service down')
  })
})
