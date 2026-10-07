import { describe, it, expect, vi } from 'vitest'
import { dispatchCodeIntelRpc } from './agent-rpc-dispatch-codeintel'
import { AgentConfig } from './agent-config'

vi.mock('./codeintel-method-table', () => ({
  CODEINTEL_METHODS: {
    'codeintel.status': {
      timeoutMs: 1000,
      validate: (params: any) => params,
      handle: vi.fn().mockResolvedValue({ sources: [] })
    },
    'codeintel.error': {
      timeoutMs: 1000,
      validate: (params: any) => params,
      handle: vi.fn().mockRejectedValue(new Error('Test error'))
    }
  }
}))

describe('agent-rpc-dispatch-codeintel', () => {
  const mockConfig: AgentConfig = {} as any
  const mockLog = { error: vi.fn(), info: vi.fn() } as any
  const mockWs = { send: vi.fn(), readyState: 1 } as any
  const mockState = {} as any

  it('returns null for non-codeintel methods', async () => {
    const res = await dispatchCodeIntelRpc({ jsonrpc: '2.0', id: 1, method: 'agent.exec' }, mockConfig, mockLog, mockWs, mockState)
    expect(res).toBeNull()
  })

  it('returns MethodNotFound for unknown codeintel method', async () => {
    const res = await dispatchCodeIntelRpc({ jsonrpc: '2.0', id: 1, method: 'codeintel.nope' }, mockConfig, mockLog, mockWs, mockState)
    expect(res).toEqual({
      jsonrpc: '2.0',
      id: 1,
      error: { code: -32601, message: 'Method not found: codeintel.nope' }
    })
  })

  it('dispatches valid method and returns result', async () => {
    const res = await dispatchCodeIntelRpc({ jsonrpc: '2.0', id: 1, method: 'codeintel.status', params: {} }, mockConfig, mockLog, mockWs, mockState)
    expect(res).toEqual({
      jsonrpc: '2.0',
      id: 1,
      result: { sources: [] }
    })
  })

  it('handles unknown errors as CODEINTEL_TOOL_FAILED', async () => {
    const res = await dispatchCodeIntelRpc({ jsonrpc: '2.0', id: 1, method: 'codeintel.error', params: {} }, mockConfig, mockLog, mockWs, mockState)
    expect(res).toEqual({
      jsonrpc: '2.0',
      id: 1,
      error: {
        code: -32000,
        message: 'internal error',
        data: { code: 'CODEINTEL_TOOL_FAILED' }
      }
    })
  })
})
