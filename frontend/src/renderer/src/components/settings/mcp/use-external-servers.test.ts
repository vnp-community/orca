// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useExternalServers } from './use-external-servers'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))

const srv = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  scope: 'user',
  name: id,
  transport: 'http',
  envRefs: [],
  headerRefs: [],
  status: 'pending_review',
  toolsChanged: false,
  createdBy: 'u',
  ...over
})

beforeEach(() => {
  call.mockReset()
})
afterEach(cleanup)

describe('useExternalServers', () => {
  it('loads, upserts in place and removes', async () => {
    call.mockImplementation((m: string) => {
      if (m === 'mcp.externalServer.list') {
        return Promise.resolve([srv('a')])
      }
      if (m === 'mcp.externalServer.upsert') {
        return Promise.resolve(srv('b'))
      }
      return Promise.resolve({ ok: true })
    })
    const { result } = renderHook(() => useExternalServers())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(call).toHaveBeenCalledWith('mcp.externalServer.list', {})
    await act(() => result.current.upsert({ name: 'b' }).then(() => undefined))
    expect(result.current.data.map((s) => s.id)).toEqual(['a', 'b'])
    await act(() => result.current.remove('a'))
    expect(result.current.data.map((s) => s.id)).toEqual(['b'])
  })

  it('sends setSecret as one plaintext object and scrubs the value from errors', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.externalServer.list'
        ? Promise.resolve([])
        : Promise.reject(new Error('MCP_SERVER_INVALID: bad value hunter2-secret'))
    )
    const { result } = renderHook(() => useExternalServers())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    const p = { serverId: 's', kind: 'env' as const, name: 'K', value: 'hunter2-secret' }
    let err: McpRpcError | undefined
    await act(async () => {
      await result.current.setSecret(p).catch((e) => (err = e))
    })
    expect(call).toHaveBeenCalledWith('mcp.externalServer.setSecret', p)
    expect(err?.code).toBe('MCP_SERVER_INVALID')
    expect(err?.detail).not.toContain('hunter2-secret')
    expect(JSON.stringify(result.current.data)).not.toContain('hunter2')
  })

  it('reloads on MCP_NOT_FOUND', async () => {
    call.mockImplementation((m: string) =>
      m === 'mcp.externalServer.list'
        ? Promise.resolve([])
        : Promise.reject(new Error('MCP_NOT_FOUND: gone'))
    )
    const { result } = renderHook(() => useExternalServers())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    await act(async () => {
      await result.current.remove('x').catch(() => undefined)
    })
    await waitFor(() =>
      expect(call.mock.calls.filter((c) => c[0] === 'mcp.externalServer.list').length).toBe(2)
    )
  })
})
