// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { McpConsentPage } from './McpConsentPage'

const call = vi.fn()
const track = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/lib/telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))

const REQ_ID = '0b2f6c1e-1111-4222-8333-444455556666'
const request = {
  requestId: REQ_ID,
  clientId: 'client-secret-id',
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
const REDIRECT = 'http://localhost:33418/cb?code=SECRET_CODE&state=xyz'

beforeEach(() => {
  call.mockReset()
  track.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, search: `?request_id=${REQ_ID}`, assign: vi.fn() }
  })
  call.mockImplementation((m: string) =>
    Promise.resolve(m === 'mcp.consent.get' ? request : { redirectUrl: REDIRECT })
  )
})
afterEach(cleanup)

function expectCoarse(): void {
  const dump = JSON.stringify(track.mock.calls)
  for (const leak of [
    REQ_ID,
    'client-secret-id',
    'Claude Code',
    'localhost',
    'SECRET_CODE',
    'orca:'
  ]) {
    expect(dump).not.toContain(leak)
  }
}

describe('consent telemetry', () => {
  it('reports a narrowed approve as buckets and booleans only', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('checkbox', { name: /Write/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Allow access' }))
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1))
    // Defaults: read + write ticked, exec not; unticking write leaves one of three.
    expect(track).toHaveBeenCalledWith('mcp_consent_decided', {
      decision: 'approve',
      scope_count: '1',
      new_client: true,
      via_dcr: true,
      narrowed: true
    })
    expectCoarse()
  })

  it('reports deny as not narrowed with the 1 bucket', async () => {
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1))
    expect(track).toHaveBeenCalledWith('mcp_consent_decided', {
      decision: 'deny',
      scope_count: '1',
      new_client: true,
      via_dcr: true,
      narrowed: false
    })
    expectCoarse()
  })

  it('does not report when the decision fails', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.consent.get' ? Promise.resolve(request) : Promise.reject(new Error('boom'))
    )
    render(<McpConsentPage />)
    await screen.findByText('Authorize Claude Code')
    fireEvent.click(screen.getByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(call).toHaveBeenCalledWith('mcp.consent.decide', expect.anything()))
    expect(track).not.toHaveBeenCalled()
  })
})
