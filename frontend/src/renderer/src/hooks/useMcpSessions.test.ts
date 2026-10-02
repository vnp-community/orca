// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { emitMcpEvent } from '@/lib/mcp-event-bus'
import { useMcpSessions } from './useMcpSessions'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))
vi.mock('@/store', async () => {
  const { create } = await import('zustand')
  return { useAppStore: create(() => ({ mcpResyncCounter: 0 })) }
})

const session = (id: string) => ({
  id,
  clientName: 'c',
  createdAt: '2026-10-01T00:00:00Z',
  lastSeenAt: '2026-10-01T00:00:00Z',
  protocolVersion: '2025-06-18',
  activeStreams: 0,
  toolCalls: 0
})

beforeEach(() => call.mockReset())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useMcpSessions', () => {
  it('loads own sessions with no params', async () => {
    call.mockResolvedValue([session('a')])
    const { result } = renderHook(() => useMcpSessions('mine'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(call).toHaveBeenCalledWith('mcp.session.list')
    expect(result.current.sessions).toHaveLength(1)
  })

  it('maps MCP_NOT_ADMIN to forbidden and not-implemented to unavailable', async () => {
    call.mockRejectedValueOnce(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    const a = renderHook(() => useMcpSessions('all'))
    await waitFor(() => expect(a.result.current.status).toBe('forbidden'))
    call.mockRejectedValueOnce(new Error('method mcp.session.list is not yet implemented'))
    const b = renderHook(() => useMcpSessions('mine'))
    await waitFor(() => expect(b.result.current.status).toBe('unavailable'))
  })

  it('removes a row on session.closed events', async () => {
    call.mockResolvedValue([session('a'), session('b')])
    const { result } = renderHook(() => useMcpSessions('mine'))
    await waitFor(() => expect(result.current.sessions).toHaveLength(2))
    act(() => emitMcpEvent({ type: 'session.closed', sessionId: 'a' }))
    expect(result.current.sessions.map((s) => s.id)).toEqual(['b'])
  })

  it('drops stale results when the scope changes quickly', async () => {
    let resolveMine: (v: unknown) => void = () => {}
    call.mockImplementation((m: string) =>
      m === 'mcp.session.list'
        ? new Promise((r) => (resolveMine = r))
        : Promise.resolve([session('all-1')])
    )
    const { result, rerender } = renderHook(({ s }) => useMcpSessions(s), {
      initialProps: { s: 'mine' as 'mine' | 'all' }
    })
    rerender({ s: 'all' })
    await waitFor(() => expect(result.current.sessions.map((s) => s.id)).toEqual(['all-1']))
    await act(async () => resolveMine([session('stale')]))
    expect(result.current.sessions.map((s) => s.id)).toEqual(['all-1'])
  })

  it('closes with an object param and reloads; MCP_NOT_FOUND counts as closed', async () => {
    call.mockResolvedValue([session('a')])
    const { result } = renderHook(() => useMcpSessions('mine'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    call.mockResolvedValueOnce({ ok: true })
    await act(async () => result.current.closeSession('a'))
    expect(call).toHaveBeenCalledWith('mcp.session.close', { sessionId: 'a' })
    call.mockRejectedValueOnce(new McpRpcError('MCP_NOT_FOUND', 'gone'))
    await act(async () => result.current.closeSession('a'))
    call.mockRejectedValueOnce(new McpRpcError('MCP_INTERNAL', 'boom'))
    await expect(result.current.closeSession('a')).rejects.toThrow('boom')
  })

  it('polls while mounted and stops after unmount', async () => {
    vi.useFakeTimers()
    call.mockResolvedValue([])
    const { unmount } = renderHook(() => useMcpSessions('mine'))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_000)
    })
    const callsWhileMounted = call.mock.calls.length
    expect(callsWhileMounted).toBeGreaterThanOrEqual(2)
    unmount()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(call.mock.calls.length).toBe(callsWhileMounted)
  })
})
