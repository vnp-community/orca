// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpConsentPage } from '@/components/mcp/consent/McpConsentPage'
import { createFakeMcpBackend, mcpError, type FakeMcpBackend } from '../mcp-fake-backend'
import { makeMcpConsentRequest } from '../mcp-fixtures'

const REQ = makeMcpConsentRequest()
const assign = vi.fn()
let backend: FakeMcpBackend
let uninstall: () => void

beforeEach(() => {
  assign.mockReset()
  backend = createFakeMcpBackend()
  uninstall = backend.install()
  backend.addConsent(REQ)
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, search: `?request_id=${REQ.requestId}`, assign }
  })
})
afterEach(() => {
  cleanup()
  uninstall()
})

const allow = (): HTMLElement => screen.getByRole('button', { name: 'Allow access' })

describe('consent flow against the fake MCP backend', () => {
  it('consent.get -> narrowed approve -> validated redirect', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    expect(screen.getByText(/localhost:33418/)).toBeTruthy()
    expect(screen.queryByText(/callback\?code/)).toBeNull()
    fireEvent.click(screen.getByRole('checkbox', { name: /Write/ }))
    fireEvent.click(allow())
    await waitFor(() => expect(assign).toHaveBeenCalledTimes(1))
    expect(assign).toHaveBeenCalledWith('http://localhost:33418/callback?code=abc&state=xyz')
    expect(backend.decisions).toEqual([
      { requestId: REQ.requestId, decision: 'approve', scopes: ['orca:read'] }
    ])
    expect(backend.calls.map((c) => c.method)).toEqual(['mcp.consent.get', 'mcp.consent.decide'])
  })

  it('deny sends an empty scope list', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(assign).toHaveBeenCalled())
    expect(backend.decisions[0]).toMatchObject({ decision: 'deny', scopes: [] })
  })

  it.each([
    'javascript:alert(1)',
    'data:text/html,x',
    'http://evil.example.com/cb',
    'https://u:p@x.io/'
  ])('never navigates to a hostile redirect (%s)', async (url) => {
    backend.setRedirectUrl(url)
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(allow())
    await screen.findByText('Something went wrong.')
    expect(assign).not.toHaveBeenCalled()
  })

  it('flags an unverified DCR client', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    expect(screen.getByRole('note').textContent).toContain('registered itself')
  })

  it('shows expired / unknown / disabled without a retry loop', async () => {
    backend.failNext('mcp.consent.get', mcpError('MCP_CONSENT_EXPIRED', 'late'))
    render(<McpConsentPage />)
    await screen.findByText(/has expired/)
    expect(screen.queryByRole('button', { name: 'Allow access' })).toBeNull()
    cleanup()
    backend.failNext('mcp.consent.get', mcpError('MCP_DISABLED', 'off'))
    render(<McpConsentPage />)
    await screen.findByText(/turned off/)
    cleanup()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { search: '?request_id=00000000-0000-4000-8000-000000000000', assign }
    })
    render(<McpConsentPage />)
    await screen.findByText(/no longer valid/)
  })

  it('keeps the page usable when the server rejects the scope selection', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    backend.failNext('mcp.consent.decide', mcpError('MCP_SCOPE_NOT_ALLOWED', 'no'))
    fireEvent.click(allow())
    await screen.findByText(/can't be granted/)
    expect(assign).not.toHaveBeenCalled()
  })
})
