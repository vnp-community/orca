// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})
const selectorState = vi.hoisted(() => ({ value: { state: 'ready', worktreeId: 'wt', projectId: 'p', environmentId: null } as unknown }))
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => selectorState.value
}))

import { useAppStore } from '@/store'
import { buildCodeIntelCacheKey, useCodeIntelQuery } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'

const ok = (result: unknown, meta?: unknown): CodeIntelCallOutcome => ({
  ok: true,
  result,
  meta: meta as never
})
const fail = (kind: string, data: Record<string, unknown> | null = null): CodeIntelCallOutcome => ({
  ok: false,
  error: { kind: kind as never, code: null, message: kind, retryable: false, data }
})

function enable() {
  act(() => {
    useAppStore.getState().setCodeIntelSupportState({ state: 'enabled', effective: { codeIntelEnabled: true } })
  })
}

async function flush() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  selectorState.value = { state: 'ready', worktreeId: 'wt', projectId: 'p', environmentId: null }
  useAppStore.setState({
    codeIntelSupportState: { state: 'unknown' },
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0
  })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

const OPTS = { method: 'structure', params: { path: 'a' } }

describe('useCodeIntelQuery gating', () => {
  it('does not call when support is not enabled', async () => {
    const call = vi.fn<CodeIntelCallFn>()
    const { result } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    expect(call).not.toHaveBeenCalled()
    expect(result.current.status).toBe('idle')
  })

  it('does not call when disabled via opts.enabled or unsupported selector', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() => useCodeIntelQuery('wt', null, { ...OPTS, enabled: false }, call))
    selectorState.value = { state: 'unsupported', reason: 'no-project' }
    renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('does not call without a worktree', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() => useCodeIntelQuery(null, null, OPTS, call))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })
})

describe('useCodeIntelQuery loading', () => {
  it('loads, normalizes the method, caches, and serves the second hook from cache', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(ok({ n: 1 }, { worktreeId: 'wt', truncated: true, stale: false } as never))
    const first = renderHook(() => useCodeIntelQuery<{ n: number }>('wt', 'env-1', OPTS, call))
    await flush()
    expect(call).toHaveBeenCalledWith('wt', 'codeIntel.structure', { path: 'a' }, expect.any(AbortSignal), 'env-1')
    expect(first.result.current.status).toBe('success')
    expect(first.result.current.data).toEqual({ n: 1 })
    expect(first.result.current.truncated).toBe(true)

    const second = renderHook(() => useCodeIntelQuery<{ n: number }>('wt', 'env-1', OPTS, call))
    await flush()
    expect(second.result.current.data).toEqual({ n: 1 })
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('cache keys differ by scopeKey and param order does not matter', () => {
    expect(buildCodeIntelCacheKey('structure', { a: 1, b: 2 })).toBe(buildCodeIntelCacheKey('codeIntel.structure', { b: 2, a: 1 }))
    expect(buildCodeIntelCacheKey('structure', { a: 1 }, 's1')).not.toBe(buildCodeIntelCacheKey('structure', { a: 1 }, 's2'))
  })

  it('classified errors surface in error', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(fail('forbidden'))
    const { result } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    expect(result.current.status).toBe('error')
    expect(result.current.error?.kind).toBe('forbidden')
  })

  it('a throwing parseResult becomes tool-failed', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(ok('garbage'))
    const { result } = renderHook(() =>
      useCodeIntelQuery('wt', null, { ...OPTS, parseResult: () => { throw new Error('shape') } }, call)
    )
    await flush()
    expect(result.current.error?.kind).toBe('tool-failed')
  })

  it('aborts the in-flight call on unmount and drops its result', async () => {
    enable()
    let signal!: AbortSignal
    let resolve!: (o: CodeIntelCallOutcome) => void
    const call = vi.fn<CodeIntelCallFn>().mockImplementation((_w, _m, _p, s) => {
      signal = s
      return new Promise((r) => { resolve = r })
    })
    const { result, unmount } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    unmount()
    expect(signal.aborted).toBe(true)
    await act(async () => { resolve(ok({ late: true })) })
    expect(result.current.data).toBeNull()
    expect(useAppStore.getState().getCacheResult('wt', buildCodeIntelCacheKey('structure', { path: 'a' }))).toBeUndefined()
  })

  it('changing params aborts the old call and ignores its late result', async () => {
    enable()
    const resolvers: ((o: CodeIntelCallOutcome) => void)[] = []
    const signals: AbortSignal[] = []
    const call = vi.fn<CodeIntelCallFn>().mockImplementation((_w, _m, _p, s) => {
      signals.push(s)
      return new Promise((r) => resolvers.push(r))
    })
    const { result, rerender } = renderHook(
      ({ path }) => useCodeIntelQuery<{ v: string }>('wt', null, { method: 'structure', params: { path } }, call),
      { initialProps: { path: 'a' } }
    )
    await flush()
    rerender({ path: 'b' })
    await flush()
    expect(signals[0].aborted).toBe(true)
    await act(async () => { resolvers[0](ok({ v: 'old' })) })
    expect(result.current.data).toBeNull()
    await act(async () => { resolvers[1](ok({ v: 'new' })) })
    expect(result.current.data).toEqual({ v: 'new' })
  })
})

describe('useCodeIntelQuery timeout retry', () => {
  it('retries timeout+inProgress after retryAfterMs and then succeeds', async () => {
    enable()
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(fail('timeout', { inProgress: true, retryAfterMs: 3000 }))
      .mockResolvedValueOnce(ok({ done: true }))
    const { result } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    expect(call).toHaveBeenCalledTimes(1)
    expect(result.current.status).toBe('loading')
    await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current.data).toEqual({ done: true })
  })

  it('gives up after 90 s of retries', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(fail('timeout', { inProgress: true, retryAfterMs: 30_000 }))
    const { result } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000) })
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000) })
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000) })
    expect(result.current.status).toBe('error')
    expect(result.current.error?.kind).toBe('timeout')
    expect(call).toHaveBeenCalledTimes(4)
  })

  it('does not retry a timeout without inProgress', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(fail('timeout'))
    const { result } = renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    expect(result.current.status).toBe('error')
    expect(call).toHaveBeenCalledTimes(1)
  })
})

describe('useCodeIntelQuery resync and stale signal', () => {
  it('a plain changed only sets staleSignal; applyNow reloads', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValueOnce(ok({ n: 1 })).mockResolvedValueOnce(ok({ n: 2 }))
    const { result } = renderHook(() => useCodeIntelQuery<{ n: number }>('wt', null, OPTS, call))
    await flush()
    act(() => useAppStore.getState().invalidateCodeIntelWorktree('wt'))
    expect(result.current.staleSignal).toBe(true)
    expect(call).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual({ n: 1 })
    act(() => result.current.applyNow())
    await flush()
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current.data).toEqual({ n: 2 })
    expect(result.current.staleSignal).toBe(false)
  })

  it('resync counter reloads the view', async () => {
    enable()
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(ok({ n: 1 }))
    renderHook(() => useCodeIntelQuery('wt', null, OPTS, call))
    await flush()
    act(() => {
      useAppStore.getState().invalidateCodeIntelWorktree('wt')
      useAppStore.getState().triggerCodeIntelResync()
    })
    await flush()
    expect(call).toHaveBeenCalledTimes(2)
  })
})
