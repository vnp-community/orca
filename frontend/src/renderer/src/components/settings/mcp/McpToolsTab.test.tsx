// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useAppStore } from '@/store'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { McpToolsTab } from './McpToolsTab'

const call = vi.fn()
const openMcpTab = vi.fn()
const registry = vi.hoisted(() => ({
  tabs: [{ id: 'tools' }, { id: 'policy' }] as { id: string }[]
}))
vi.mock('./mcp-tab-registry', () => ({
  get MCP_TABS() {
    return registry.tabs
  }
}))
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return {
    useAppStore: create(() => ({
      mcpServerInfo: { enabled: true, killSwitch: { active: false } },
      mcpResyncCounter: 0,
      openMcpTab: (...a: unknown[]) => openMcpTab(...a)
    }))
  }
})

const tool = (name: string, over: Record<string, unknown> = {}) => ({
  name,
  channel: name,
  title: `Title ${name}`,
  description: 'd',
  namespace: 'git',
  risk: 'read',
  requiredScope: 'orca:read',
  pack: 1,
  hardDenied: false,
  effective: 'allow',
  effectiveSource: 'default',
  annotations: { readOnly: true, destructive: false, idempotent: false, openWorld: false },
  ...over
})

beforeEach(() => {
  call.mockReset()
  openMcpTab.mockReset()
  registry.tabs = [{ id: 'tools' }, { id: 'policy' }]
  useAppStore.setState({
    mcpServerInfo: { enabled: true, killSwitch: { active: false } },
    mcpResyncCounter: 0
  } as never)
})
afterEach(cleanup)

describe('McpToolsTab', () => {
  it('calls mcp.admin.tool.list and shows the summary line', async () => {
    call.mockResolvedValue([
      tool('git.status'),
      tool('terminal.run', { effective: 'require_approval' })
    ])
    render(<McpToolsTab />)
    await screen.findByText(/2 tools · 1 allowed · 1 need approval/)
    expect(call).toHaveBeenCalledWith('mcp.admin.tool.list', {})
  })

  it('links to the policy tab when it is registered', async () => {
    call.mockResolvedValue([tool('git.status')])
    render(<McpToolsTab />)
    fireEvent.click(await screen.findByRole('button', { name: 'Edit policy for git.status' }))
    expect(openMcpTab).toHaveBeenCalledWith('policy', 'git.status')
  })

  it('hides the link and wording when the policy tab is not registered', async () => {
    registry.tabs = [{ id: 'tools' }]
    call.mockResolvedValue([tool('git.status')])
    render(<McpToolsTab />)
    await screen.findByText('git.status')
    expect(screen.queryByRole('button', { name: /Edit policy/ })).toBeNull()
    expect(screen.queryByText(/Policy tab/)).toBeNull()
  })

  it('shows a kill switch banner when entries are killed', async () => {
    call.mockResolvedValue([
      tool('git.status', { effective: 'deny', effectiveSource: 'kill_switch' })
    ])
    render(<McpToolsTab />)
    await screen.findByText('The MCP kill switch is on: agents cannot call any tool.')
  })

  it('shows the empty catalog message and a no-match message with Clear filters', async () => {
    call.mockResolvedValueOnce([])
    const first = render(<McpToolsTab />)
    await screen.findByText('No tools are exposed yet. Tool packs are enabled on the server.')
    first.unmount()
    call.mockResolvedValueOnce([tool('git.status')])
    render(<McpToolsTab />)
    fireEvent.change(await screen.findByLabelText('Search tools'), { target: { value: 'zzz' } })
    await screen.findByText('No tools match these filters.')
    fireEvent.click(screen.getAllByRole('button', { name: 'Clear filters' })[0])
    await screen.findByText('git.status')
  })

  it('error state offers Retry; MCP_DISABLED and MCP_NOT_ADMIN have their own text without Retry', async () => {
    call.mockRejectedValueOnce(new McpRpcError('MCP_INTERNAL', 'boom'))
    const a = render(<McpToolsTab />)
    await screen.findByText('boom')
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy()
    a.unmount()
    call.mockRejectedValueOnce(new McpRpcError('MCP_DISABLED', 'off'))
    const b = render(<McpToolsTab />)
    await screen.findByText('MCP is turned off for this organization.')
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
    b.unmount()
    call.mockRejectedValueOnce(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    render(<McpToolsTab />)
    await screen.findByText('You need admin rights to view tools.')
  })
})
