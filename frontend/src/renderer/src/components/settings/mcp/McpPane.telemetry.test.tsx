// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpPane } from './McpPane'

const track = vi.fn()
vi.mock('@/lib/telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  const state = {
    mcpServerInfo: null as unknown,
    mcpServerInfoStatus: 'ready',
    mcpServerInfoError: null,
    mcpAdminSetupAvailable: false,
    currentUser: null as unknown,
    mcpNavigation: null,
    refreshMcpServerInfo: () => Promise.resolve(),
    clearMcpNavigation: () => {}
  }
  return { useAppStore: create(() => state) }
})

const info = {
  enabled: true,
  resourceUrl: '',
  protocolVersions: [],
  authorizationServer: '',
  scopesSupported: [],
  dcrEnabled: false,
  maxTokenDays: 1,
  killSwitch: { active: false }
}

beforeEach(() => {
  track.mockReset()
  useAppStore.setState({
    mcpServerInfo: info,
    mcpServerInfoStatus: 'ready',
    currentUser: { id: 'user-secret-id', role: 'developer' }
  } as never)
})
afterEach(() => {
  cleanup()
  vi.unstubAllEnvs()
})

describe('McpPane telemetry and rollout badge', () => {
  it('reports the opened tab and role once, with enums only', async () => {
    render(<McpPane />)
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1))
    const [name, props] = track.mock.calls[0] as [string, Record<string, unknown>]
    expect(name).toBe('mcp_settings_opened')
    expect(Object.keys(props).sort()).toEqual(['role', 'tab'])
    expect(props.role).toBe('user')
    expect(JSON.stringify(track.mock.calls)).not.toContain('user-secret-id')
  })

  it('hides the stage badge by default', () => {
    render(<McpPane />)
    expect(screen.queryByText('Beta')).toBeNull()
  })

  it('shows the Beta badge when VITE_MCP_UI_STAGE=beta', () => {
    vi.stubEnv('VITE_MCP_UI_STAGE', 'beta')
    render(<McpPane />)
    expect(screen.getByText('Beta')).toBeTruthy()
  })

  it('treats unknown stage values as GA', () => {
    vi.stubEnv('VITE_MCP_UI_STAGE', 'preview')
    render(<McpPane />)
    expect(screen.queryByText('Beta')).toBeNull()
  })
})
