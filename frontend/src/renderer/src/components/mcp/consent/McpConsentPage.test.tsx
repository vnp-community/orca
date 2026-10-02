// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpConsentPage } from './McpConsentPage'
import { readConsentRequestId } from './use-mcp-consent'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))

const REQ_ID = '0b2f6c1e-1111-4222-8333-444455556666'
const request = {
  requestId: REQ_ID,
  clientId: 'c1',
  clientName: 'Claude Code',
  redirectHost: 'localhost:33418',
  scopes: [
    { id: 'orca:read', label: 'Read', description: 'Read data', risk: 'read' },
    { id: 'orca:write', label: 'Write', description: 'Change data', risk: 'write_reversible' },
    { id: 'orca:exec', label: 'Run', description: 'Run commands', risk: 'exec' }
  ],
  alreadyGranted: [],
  tenant: { id: 't', name: 'Acme' },
  isNewClient: true,
  registeredViaDcr: true,
  expiresAt: '2026-10-02T00:00:00Z'
}
const assign = vi.fn()

beforeEach(() => {
  call.mockReset()
  assign.mockReset()
  window.history.replaceState(null, '', `/oauth/consent?request_id=${REQ_ID}`)
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, search: `?request_id=${REQ_ID}`, assign }
  })
})
afterEach(cleanup)

const allow = (): HTMLButtonElement => screen.getByRole('button', { name: 'Allow access' })

describe('McpConsentPage', () => {
  it('parses request ids strictly', () => {
    expect(readConsentRequestId(`?request_id=${REQ_ID}`)).toBe(REQ_ID)
    expect(readConsentRequestId('?request_id=../../x')).toBeNull()
    expect(readConsentRequestId('')).toBeNull()
  })

  it('does not call the backend without a valid request id', async () => {
    Object.defineProperty(window, 'location', { configurable: true, value: { search: '', assign } })
    render(<McpConsentPage />)
    await screen.findByText(/no longer valid/)
    expect(call).not.toHaveBeenCalled()
  })

  it('renders scopes, leaves high risk unticked and low risk ticked', async () => {
    call.mockResolvedValue(request)
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    expect(call).toHaveBeenCalledWith('mcp.consent.get', { requestId: REQ_ID })
    const read = screen.getByRole('checkbox', { name: /Read/ })
    const exec = screen.getByRole('checkbox', { name: /Run commands/ })
    expect(read.getAttribute('aria-checked')).toBe('true')
    expect(exec.getAttribute('aria-checked')).toBe('false')
    expect(screen.getByText('High risk')).toBeTruthy()
    expect(screen.getByRole('note').textContent).toContain('registered itself')
  })

  it('disables Allow with zero scopes while Deny stays available', async () => {
    call.mockResolvedValue(request)
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('checkbox', { name: /Read/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /Write/ }))
    expect(allow().disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Deny' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('denies with an empty scope list and redirects to the validated url', async () => {
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.consent.get'
          ? request
          : { redirectUrl: 'http://localhost:33418/cb?error=access_denied' }
      )
    )
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith('http://localhost:33418/cb?error=access_denied')
    )
    expect(call).toHaveBeenCalledWith('mcp.consent.decide', {
      requestId: REQ_ID,
      decision: 'deny',
      scopes: []
    })
    expect(screen.getByRole('link', { name: 'Continue' })).toBeTruthy()
  })

  it('approves selected scopes and only sends once on double click', async () => {
    let resolveDecide: (v: unknown) => void = () => {}
    call.mockImplementation((m: string) =>
      m === 'mcp.consent.get' ? Promise.resolve(request) : new Promise((r) => (resolveDecide = r))
    )
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('checkbox', { name: /Run commands/ }))
    const button = allow()
    fireEvent.click(button)
    fireEvent.click(button)
    expect(call.mock.calls.filter((c) => c[0] === 'mcp.consent.decide')).toHaveLength(1)
    expect(call).toHaveBeenCalledWith('mcp.consent.decide', {
      requestId: REQ_ID,
      decision: 'approve',
      scopes: ['orca:read', 'orca:write', 'orca:exec']
    })
    resolveDecide({ redirectUrl: 'https://claude.ai/cb?code=1' })
    await waitFor(() => expect(assign).toHaveBeenCalledWith('https://claude.ai/cb?code=1'))
  })

  it.each(['javascript:alert(1)', 'data:text/html,x', 'http://evil.example.com/cb'])(
    'refuses to navigate to %s',
    async (redirectUrl) => {
      call.mockImplementation((m: string) =>
        Promise.resolve(m === 'mcp.consent.get' ? request : { redirectUrl })
      )
      render(<McpConsentPage />)
      await screen.findByText('Authorize Claude Code')
      fireEvent.click(allow())
      await screen.findByText('Something went wrong.')
      expect(assign).not.toHaveBeenCalled()
    }
  )

  it.each([
    ['MCP_CONSENT_EXPIRED', /has expired/],
    ['MCP_CONSENT_NOT_FOUND', /no longer valid/],
    ['MCP_DISABLED', /turned off/]
  ])('shows the %s state without retry', async (code, text) => {
    call.mockRejectedValue(new McpRpcError(code as never, 'x'))
    render(<McpConsentPage />)
    await screen.findByText(text)
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })

  it('keeps the selection and shows a notice on MCP_SCOPE_NOT_ALLOWED', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.consent.get'
        ? Promise.resolve(request)
        : Promise.reject(new McpRpcError('MCP_SCOPE_NOT_ALLOWED', 'no'))
    )
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(allow())
    await screen.findByText(/can't be granted/)
    expect(screen.getByRole('checkbox', { name: /Read/ }).getAttribute('aria-checked')).toBe('true')
  })

  it('offers retry for unknown errors', async () => {
    call.mockRejectedValueOnce(new Error('boom')).mockResolvedValue(request)
    render(<McpConsentPage />)
    await screen.findByText('Something went wrong.')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await screen.findByText('Authorize Claude Code')
  })

  it('never writes consent data to storage or the console', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const log = vi.spyOn(console, 'log')
    call.mockImplementation((m: string) =>
      Promise.resolve(
        m === 'mcp.consent.get' ? request : { redirectUrl: 'https://claude.ai/cb?code=SECRET' }
      )
    )
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(allow())
    await waitFor(() => expect(assign).toHaveBeenCalled())
    expect(setItem).not.toHaveBeenCalled()
    expect(log).not.toHaveBeenCalled()
  })
})
