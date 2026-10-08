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
import { useCodeIntelPagedQuery } from './useCodeIntelPagedQuery'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'

type Page = { routes: string[] }
const page = (routes: string[], next?: string): CodeIntelCallOutcome => ({
  ok: true,
  result: { routes },
  meta: { nextPageToken: next } as never
})

async function flush() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

beforeEach(() => {
  useAppStore.setState({
    codeIntelSupportState: { state: 'enabled', effective: { codeIntelEnabled: true } },
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0
  })
})
afterEach(() => cleanup())

const OPTS = {
  method: 'routes',
  params: { limit: 2 },
  select: (p: Page) => p.routes
}

describe('useCodeIntelPagedQuery', () => {
  it('loads page 1 and appends page 2 with the opaque token', async () => {
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(page(['a', 'b'], 'tok-2'))
      .mockResolvedValueOnce(page(['c']))
    const { result } = renderHook(() => useCodeIntelPagedQuery<Page, string>('wt', null, OPTS, call))
    await flush()
    expect(result.current.items).toEqual(['a', 'b'])
    expect(result.current.hasNextPage).toBe(true)

    act(() => result.current.fetchNextPage())
    await flush()
    expect(call.mock.calls[1][2]).toEqual({ limit: 2, pageToken: 'tok-2' })
    expect(result.current.items).toEqual(['a', 'b', 'c'])
    expect(result.current.hasNextPage).toBe(false)
  })

  it('ignores fetchNextPage while a page is in flight (no duplicate request)', async () => {
    let resolve!: (o: CodeIntelCallOutcome) => void
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(page(['a'], 'tok-2'))
      .mockImplementationOnce(() => new Promise((r) => (resolve = r)))
    const { result } = renderHook(() => useCodeIntelPagedQuery<Page, string>('wt', null, OPTS, call))
    await flush()
    act(() => {
      result.current.fetchNextPage()
      result.current.fetchNextPage()
    })
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current.isFetchingNextPage).toBe(true)
    await act(async () => resolve(page(['b'])))
    expect(result.current.items).toEqual(['a', 'b'])
    expect(result.current.isFetchingNextPage).toBe(false)
  })

  it('changing params resets the appended pages', async () => {
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(page(['a'], 'tok-2'))
      .mockResolvedValueOnce(page(['b']))
      .mockResolvedValueOnce(page(['x']))
    const { result, rerender } = renderHook(
      ({ q }) =>
        useCodeIntelPagedQuery<Page, string>('wt', null, { ...OPTS, params: { limit: 2, q } }, call),
      { initialProps: { q: '1' } }
    )
    await flush()
    act(() => result.current.fetchNextPage())
    await flush()
    expect(result.current.items).toEqual(['a', 'b'])
    rerender({ q: '2' })
    await flush()
    expect(result.current.items).toEqual(['x'])
  })

  it('a failing next page keeps page 1 and reports nextPageError', async () => {
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(page(['a'], 'tok-2'))
      .mockResolvedValueOnce({ ok: false, error: { kind: 'offline', code: null, message: 'down', retryable: true } })
    const { result } = renderHook(() => useCodeIntelPagedQuery<Page, string>('wt', null, OPTS, call))
    await flush()
    act(() => result.current.fetchNextPage())
    await flush()
    expect(result.current.items).toEqual(['a'])
    expect(result.current.nextPageError?.kind).toBe('offline')
    expect(result.current.hasNextPage).toBe(true)
  })

  it('fetchNextPage is a no-op without a token', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(page(['a']))
    const { result } = renderHook(() => useCodeIntelPagedQuery<Page, string>('wt', null, OPTS, call))
    await flush()
    act(() => result.current.fetchNextPage())
    expect(call).toHaveBeenCalledTimes(1)
  })
})
