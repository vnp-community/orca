// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render } from '@testing-library/react'
import { useAppStore } from '@/store'
import { emitMcpEvent } from '@/lib/mcp-event-bus'
import McpGlobalLayer from './McpGlobalLayer'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), info: vi.fn() }) }))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  const { createMcpApprovalSlice } = await import('@/store/slices/mcp-approval-slice')
  return {
    useAppStore: create((...a: unknown[]) => ({
      ...(createMcpApprovalSlice as (...x: unknown[]) => object)(...a),
      mcpServerInfo: { enabled: true, killSwitch: { active: false } },
      currentUser: { id: 'u' },
      mcpResyncCounter: 0,
      mcpNavigation: null as unknown,
      refreshMcpServerInfo: vi.fn()
    }))
  }
})

const ap = (id: string) => ({
  id,
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 60_000).toISOString(),
  status: 'pending',
  tool: { name: 't', title: 'T', risk: 'exec' },
  clientName: 'c',
  sessionId: 's',
  argsPreview: { text: 'x', redacted: false },
  paramsHash: 'h'
})

beforeEach(() => {
  call.mockReset()
  call.mockResolvedValue({ approvals: [] })
  useAppStore.getState().clearMcpApprovals()
})
afterEach(cleanup)

describe('McpGlobalLayer', () => {
  it('loads pending approvals on mount, enqueues events and resolves them', async () => {
    call.mockResolvedValue({ approvals: [ap('a')] })
    render(<McpGlobalLayer />)
    await act(async () => {})
    expect(call).toHaveBeenCalledWith('mcp.approval.list', { status: 'pending', limit: 50 })
    expect(useAppStore.getState().mcpApprovalQueue.map((q) => q.id)).toEqual(['a'])
    act(() => emitMcpEvent({ type: 'approval.requested', approval: ap('b') as never }))
    expect(useAppStore.getState().mcpApprovalQueue).toHaveLength(2)
    act(() => emitMcpEvent({ type: 'approval.resolved', id: 'a', status: 'approved' }))
    expect(useAppStore.getState().mcpApprovalQueue.map((q) => q.id)).toEqual(['b'])
  })

  it('clears the queue and stays inert when MCP is disabled', async () => {
    act(() => useAppStore.getState().enqueueMcpApproval(ap('a') as never, 0))
    act(() =>
      useAppStore.setState({
        mcpServerInfo: { enabled: false, killSwitch: { active: false } }
      } as never)
    )
    render(<McpGlobalLayer />)
    await act(async () => {})
    expect(useAppStore.getState().mcpApprovalQueue).toEqual([])
    expect(call).not.toHaveBeenCalled()
    act(() =>
      useAppStore.setState({
        mcpServerInfo: { enabled: true, killSwitch: { active: false } }
      } as never)
    )
  })

  it('captures the deep-link approval id before McpPane clears the navigation', async () => {
    call.mockResolvedValue({ approvals: [ap('a')] })
    render(<McpGlobalLayer />)
    await act(async () => {})
    act(() => useAppStore.setState({ mcpNavigation: { tab: 'approvals', focusId: 'a' } } as never))
    expect(useAppStore.getState().mcpApprovalFocusId).toBe('a')
  })
})
