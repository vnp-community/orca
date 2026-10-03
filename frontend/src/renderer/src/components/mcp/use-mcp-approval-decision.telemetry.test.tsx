// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { McpApproval } from '../../../../shared/mcp-types'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useMcpApprovalDecision } from './use-mcp-approval-decision'

const call = vi.fn()
const track = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/lib/telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))
vi.mock('@/lib/mcp-approval-sync', () => ({ syncMcpPendingApprovals: () => Promise.resolve() }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { success: vi.fn(), info: vi.fn() }) }))
vi.mock('@/store', () => {
  const state = { resolveMcpApproval: vi.fn(), refreshMcpServerInfo: vi.fn() }
  return { useAppStore: Object.assign(() => state, { getState: () => state }) }
})

const approval = (over: Partial<McpApproval> = {}): McpApproval => ({
  id: 'approval-id-123',
  createdAt: new Date(Date.now() - 20_000).toISOString(),
  expiresAt: new Date(Date.now() + 100_000).toISOString(),
  status: 'pending',
  tool: { name: 'terminal_send', title: 'Send to terminal', risk: 'exec' },
  clientName: 'Claude',
  sessionId: 'session-abcdef',
  argsPreview: { text: 'rm -rf /tmp/SECRET_ARGS', redacted: false },
  paramsHash: 'hash-xyz',
  ...over
})

beforeEach(() => {
  call.mockReset()
  track.mockReset()
})
afterEach(cleanup)

describe('approval decision telemetry', () => {
  it('reports decision, risk, surface and latency bucket only', async () => {
    call.mockResolvedValue({})
    const { result } = renderHook(() => useMcpApprovalDecision(approval()))
    await act(async () => {
      await result.current.decide('deny')
    })
    expect(track).toHaveBeenCalledTimes(1)
    expect(track).toHaveBeenCalledWith('mcp_approval_decided', {
      decision: 'deny',
      risk: 'exec',
      via: 'dialog',
      latency_bucket: '<60s'
    })
    const dump = JSON.stringify(track.mock.calls)
    for (const leak of ['approval-id-123', 'SECRET_ARGS', 'hash-xyz', 'terminal_send', 'Claude']) {
      expect(dump).not.toContain(leak)
    }
  })

  it('does not report when the server rejects the decision', async () => {
    call.mockRejectedValue(new McpRpcError('MCP_APPROVAL_EXPIRED', 'expired'))
    const { result } = renderHook(() => useMcpApprovalDecision(approval()))
    await act(async () => {
      await result.current.decide('approve')
    })
    expect(track).not.toHaveBeenCalled()
  })
})
