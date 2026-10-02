import { describe, expect, it, vi } from 'vitest'
import { createMcpApi, createUnavailableMcpApi, type McpTransport } from './web-mcp-api'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'

const ok = (result: unknown): RuntimeRpcResponse<unknown> => ({
  id: '1',
  ok: true,
  result,
  _meta: { runtimeId: 'r' }
})

function setup(openStream?: McpTransport['openStream']) {
  const callRuntimeResult = vi.fn().mockResolvedValue({ ok: true })
  const api = createMcpApi({
    callRuntimeResult,
    openStream: openStream ?? (() => null)
  } as McpTransport)
  return { api, callRuntimeResult }
}

describe('createMcpApi', () => {
  it('sends exactly one object arg', async () => {
    const { api, callRuntimeResult } = setup()
    await api.call('mcp.session.close', { sessionId: 'a' })
    await api.call('mcp.session.list')
    expect(callRuntimeResult).toHaveBeenNthCalledWith(1, 'mcp.session.close', {
      sessionId: 'a'
    })
    expect(callRuntimeResult).toHaveBeenNthCalledWith(2, 'mcp.session.list', {})
  })

  it('ignores the null ack, forwards bare events, drops junk', async () => {
    let handlers!: Parameters<McpTransport['openStream']>[2]
    const unsubscribe = vi.fn()
    const { api } = setup((_m, _p, h) => {
      handlers = h
      return Promise.resolve({ unsubscribe })
    })
    const onEvent = vi.fn()
    const onClose = vi.fn()
    const stop = api.subscribeEvents(onEvent, onClose)
    handlers.onResponse(ok(null))
    handlers.onResponse(ok({ type: 'nope' }))
    handlers.onResponse(ok({ type: 'session.closed', sessionId: 's' }))
    expect(onEvent).toHaveBeenCalledTimes(1)
    expect(onEvent).toHaveBeenCalledWith({
      type: 'session.closed',
      sessionId: 's'
    })
    handlers.onClose()
    expect(onClose).toHaveBeenCalledTimes(1)
    await Promise.resolve()
    stop()
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('unsubscribes when cancelled before the stream opens', async () => {
    const unsubscribe = vi.fn()
    const { api } = setup(() => Promise.resolve({ unsubscribe }))
    api.subscribeEvents(vi.fn())()
    await new Promise((r) => setTimeout(r, 0))
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('returns an empty teardown without an environment', () => {
    const { api } = setup()
    expect(() => api.subscribeEvents(vi.fn())()).not.toThrow()
  })

  it('unavailable api rejects calls', async () => {
    await expect(createUnavailableMcpApi().call('mcp.session.list')).rejects.toThrow()
  })
})
