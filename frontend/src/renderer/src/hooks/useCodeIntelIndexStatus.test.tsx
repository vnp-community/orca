// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({ state: 'ready', worktreeId: 'wt', projectId: 'p', environmentId: null })
}))

import { useAppStore } from '@/store'
import { INDEX_STATUS_POLL_MS, isIndexStale, useCodeIntelIndexStatus } from './useCodeIntelIndexStatus'
import type { CodeIntelCallFn } from './useCodeIntelQuery'
import { INDEX_STATUS_FIXTURES } from '../test-support/code-intel-fixtures'
import { parseIndexStatus } from '../../../shared/code-intel-parsers'

const result = (overall: string, tools: unknown[] = []) => ({ ok: true as const, result: { overall, tools } })

async function flush() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  useAppStore.setState({
    codeIntelSupportState: { state: 'enabled', effective: { codeIntelEnabled: true } },
    codeIntelEventsState: 'streaming',
    codeIntelResyncCounter: 0
  })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('isIndexStale', () => {
  it('is stale for STALE/OVERLAY or stale tool freshness', () => {
    expect(isIndexStale(INDEX_STATUS_FIXTURES.stale)).toBe(true)
    expect(isIndexStale(INDEX_STATUS_FIXTURES.overlay)).toBe(true)
    expect(isIndexStale(INDEX_STATUS_FIXTURES.ready)).toBe(false)
    expect(isIndexStale(null)).toBe(false)
    expect(
      isIndexStale(parseIndexStatus({ overall: 'READY', tools: [{ tool: 'gitnexus', freshness: 'stale' }] }))
    ).toBe(true)
  })
})

describe('useCodeIntelIndexStatus', () => {
  it('loads once on mount and exposes the flat fields (tools is not an IndexStatus[])', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(
      result('READY', [{ tool: 'gitnexus', available: true, supported: true, state: 'ready', indexScope: 'exact', freshness: 'fresh' }])
    )
    const { result: hook } = renderHook(() => useCodeIntelIndexStatus('wt', 'env-1', call))
    await flush()
    expect(call).toHaveBeenCalledTimes(1)
    expect(call.mock.calls[0][1]).toBe('codeIntel.status')
    expect(call.mock.calls[0][2]).toEqual({})
    expect(hook.current.overall).toBe('READY')
    expect(hook.current.tools[0].tool).toBe('gitnexus')
    expect(hook.current.stale).toBe(false)
  })

  it('refresh({force}) asks with refresh: true', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(result('READY'))
    const { result: hook } = renderHook(() => useCodeIntelIndexStatus('wt', null, call))
    await flush()
    await act(async () => hook.current.refresh({ force: true }))
    expect(call.mock.calls[1][2]).toEqual({ refresh: true })
  })

  it('does not poll while the stream is healthy', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(result('READY'))
    renderHook(() => useCodeIntelIndexStatus('wt', null, call))
    await flush()
    await act(async () => vi.advanceTimersByTimeAsync(INDEX_STATUS_POLL_MS * 3))
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('polls every 30 s only in polling mode and stops when it ends', async () => {
    useAppStore.setState({ codeIntelEventsState: 'polling' })
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(result('READY'))
    renderHook(() => useCodeIntelIndexStatus('wt', null, call))
    await flush()
    await act(async () => vi.advanceTimersByTimeAsync(INDEX_STATUS_POLL_MS))
    expect(call).toHaveBeenCalledTimes(2)
    act(() => useAppStore.setState({ codeIntelEventsState: 'streaming' }))
    await act(async () => vi.advanceTimersByTimeAsync(INDEX_STATUS_POLL_MS * 2))
    expect(call).toHaveBeenCalledTimes(2)
  })

  it('makes no call when support is not enabled', async () => {
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled' } })
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() => useCodeIntelIndexStatus('wt', null, call))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('reports errors without throwing', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue({
      ok: false,
      error: { kind: 'offline', code: null, message: 'down', retryable: true }
    })
    const { result: hook } = renderHook(() => useCodeIntelIndexStatus('wt', null, call))
    await flush()
    expect(hook.current.error?.kind).toBe('offline')
    expect(hook.current.status).toBeNull()
  })
})
