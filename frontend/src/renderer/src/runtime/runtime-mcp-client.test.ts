import { afterEach, describe, expect, it, vi } from 'vitest'
import { mcpClient } from './runtime-mcp-client'
import { McpRpcError } from './runtime-mcp-error'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('mcpClient', () => {
  it('reports bridge availability', () => {
    vi.stubGlobal('window', { api: {} })
    expect(mcpClient.isBridgeAvailable()).toBe(false)
    vi.stubGlobal('window', { api: { mcp: {} } })
    expect(mcpClient.isBridgeAvailable()).toBe(true)
  })

  it('passes a single params object and parses errors', async () => {
    const call = vi
      .fn()
      .mockResolvedValueOnce({ ok: true })
      .mockRejectedValueOnce(new Error('rpc error: code = X desc = MCP_NOT_ADMIN: nope'))
    vi.stubGlobal('window', {
      api: { mcp: { call, subscribeEvents: vi.fn() } }
    })
    await expect(mcpClient.call('mcp.session.close', { sessionId: 's' })).resolves.toEqual({
      ok: true
    })
    expect(call).toHaveBeenCalledWith('mcp.session.close', { sessionId: 's' })
    const err = await mcpClient.call('mcp.admin.session.list').catch((e) => e)
    expect(err).toBeInstanceOf(McpRpcError)
    expect(err.code).toBe('MCP_NOT_ADMIN')
  })
})
