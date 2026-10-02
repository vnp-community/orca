// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpPane } from './McpPane'

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

const baseInfo = {
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
  useAppStore.setState({
    mcpServerInfo: baseInfo,
    mcpServerInfoStatus: 'ready',
    mcpAdminSetupAvailable: false,
    currentUser: { id: 'u', role: 'developer' }
  } as never)
})
afterEach(cleanup)

describe('McpPane', () => {
  it('shows unavailable for disabled MCP', () => {
    useAppStore.setState({
      mcpServerInfo: { ...baseInfo, enabled: false }
    } as never)
    render(<McpPane />)
    expect(screen.getByText('MCP is not available.')).toBeTruthy()
  })

  it('shows the admin setup card only to admins', () => {
    useAppStore.setState({
      mcpServerInfo: { ...baseInfo, tenantEnabled: false },
      mcpAdminSetupAvailable: true,
      currentUser: { id: 'u', role: 'admin' }
    } as never)
    render(<McpPane />)
    expect(screen.getByText('MCP is turned off for your organization')).toBeTruthy()
  })

  it('shows the kill switch banner when enabled', () => {
    useAppStore.setState({
      mcpServerInfo: {
        ...baseInfo,
        killSwitch: { active: true, reason: 'incident' }
      }
    } as never)
    render(<McpPane />)
    expect(screen.getByRole('status').textContent).toContain('incident')
  })
})
