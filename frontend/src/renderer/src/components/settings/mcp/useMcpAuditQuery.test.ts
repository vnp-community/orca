// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { EMPTY_AUDIT_FILTERS } from './mcp-audit-filters'
import { useMcpAuditQuery } from './useMcpAuditQuery'

const call = vi.fn()
vi.mock('@/runtime/runtime-mcp-client', () => ({
  mcpClient: { call: (...a: unknown[]) => call(...a) }
}))

const e = (id: string) => ({ id, at: '2026-10-01T00:00:00Z', userId: 'u', tool: 't' })
beforeEach(() => call.mockReset())

describe('useMcpAuditQuery', () => {
  it('appends pages, de-dupes by id, keeps server order', async () => {
    call.mockResolvedValueOnce({ entries: [e('b'), e('a')], nextCursor: 'c1' })
    const { result } = renderHook(() => useMcpAuditQuery(EMPTY_AUDIT_FILTERS))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    call.mockResolvedValueOnce({ entries: [e('a'), e('c')] })
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.entries.map((x) => x.id)).toEqual(['b', 'a', 'c']))
    expect(call).toHaveBeenLastCalledWith('mcp.admin.audit.query', { cursor: 'c1', limit: 50 })
    expect(result.current.nextCursor).toBeUndefined()
  })

  it('drops stale responses when filters change', async () => {
    let resolveFirst: (v: unknown) => void = () => {}
    call.mockImplementationOnce(() => new Promise((r) => (resolveFirst = r)))
    call.mockResolvedValueOnce({ entries: [e('new')] })
    const { result, rerender } = renderHook((f) => useMcpAuditQuery(f), {
      initialProps: EMPTY_AUDIT_FILTERS
    })
    rerender({ ...EMPTY_AUDIT_FILTERS, tool: 'x' })
    await waitFor(() => expect(result.current.entries.map((x) => x.id)).toEqual(['new']))
    await act(async () => resolveFirst({ entries: [e('old')] }))
    expect(result.current.entries.map((x) => x.id)).toEqual(['new'])
  })

  it('keeps loaded rows when load more fails', async () => {
    call.mockResolvedValueOnce({ entries: [e('a')], nextCursor: 'c' })
    const { result } = renderHook(() => useMcpAuditQuery(EMPTY_AUDIT_FILTERS))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    call.mockRejectedValueOnce(new Error('boom'))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.moreError).toBe('boom'))
    expect(result.current.entries).toHaveLength(1)
  })

  it('maps MCP_NOT_ADMIN to forbidden and skips the call for an invalid user id', async () => {
    call.mockRejectedValueOnce(new McpRpcError('MCP_NOT_ADMIN', 'no'))
    const a = renderHook(() => useMcpAuditQuery(EMPTY_AUDIT_FILTERS))
    await waitFor(() => expect(a.result.current.status).toBe('forbidden'))
    call.mockClear()
    const b = renderHook(() => useMcpAuditQuery({ ...EMPTY_AUDIT_FILTERS, userId: 'nope' }))
    await waitFor(() => expect(b.result.current.status).toBe('idle'))
    expect(call).not.toHaveBeenCalled()
  })
})
