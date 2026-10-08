// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import { makeQualityFinding, makeQualityFindings } from '../test-support/quality-finding-fixtures'

const call = vi.fn()

vi.mock('../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: (...args: unknown[]) => call(...args) })
}))
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../test-support/code-intel-quality-test-store')
  return {
    useAppStore: createCodeIntelQualityTestStore(() =>
      Promise.resolve({ ok: true as const, result: {} })
    )
  }
})

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../test-support/code-intel-quality-test-store'
import {
  mergeQualityFindingPages,
  QUALITY_FINDINGS_MAX_ITEMS,
  useQualityFindings
} from './useQualityFindings'

const FILTER = { severities: [], categories: [], inScope: true } as const
const page = (findings: unknown[], extra: object = {}) =>
  Promise.resolve({
    ok: true as const,
    result: { findings, totalCount: 1203, truncated: false, outsideScopeCount: 12, ...extra }
  })

async function flush(): Promise<void> {
  await act(async () => {
    for (let i = 0; i < 8; i++) {
      await Promise.resolve()
    }
  })
}

beforeEach(() => {
  call.mockReset()
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {}
  } as never)
})
afterEach(() => cleanup())

describe('useQualityFindings', () => {
  it('sends the server filters and limit, and exposes totals', async () => {
    call.mockReturnValue(page(makeQualityFindings(3), { nextPageToken: 't1' }))
    const { result } = renderHook(() =>
      useQualityFindings('wt', {
        severities: ['error'],
        categories: ['lint'],
        inScope: true,
        file: 'src/a.ts'
      })
    )
    await flush()
    expect(call.mock.calls[0][2]).toEqual({
      limit: 500,
      severities: ['error'],
      categories: ['lint'],
      inScope: true,
      file: 'src/a.ts'
    })
    expect(result.current).toMatchObject({
      status: 'ready',
      total: 1203,
      outsideScopeCount: 12,
      hasMore: true
    })
    expect(result.current.items).toHaveLength(3)
  })

  it('omits inScope when off and joins pages by pageToken without duplicates', async () => {
    call.mockReturnValueOnce(page(makeQualityFindings(3), { nextPageToken: 't1' }))
    const { result } = renderHook(() => useQualityFindings('wt', { ...FILTER, inScope: false }))
    await flush()
    expect(call.mock.calls[0][2]).not.toHaveProperty('inScope')
    call.mockReturnValueOnce(page([...makeQualityFindings(5)], {}))
    act(() => result.current.loadMore())
    await flush()
    expect(call.mock.calls[1][2]).toMatchObject({ pageToken: 't1' })
    expect(result.current.items).toHaveLength(5)
    expect(result.current.hasMore).toBe(false)
  })

  it('stops at the 5000 cap and reports it', async () => {
    call.mockReturnValueOnce(page(makeQualityFindings(500, 'a'), { nextPageToken: 't' }))
    const { result } = renderHook(() => useQualityFindings('wt', FILTER))
    await flush()
    for (let i = 1; i < 10; i++) {
      call.mockReturnValueOnce(page(makeQualityFindings(500, `p${i}`), { nextPageToken: 't' }))
      act(() => result.current.loadMore())
      await flush()
    }
    expect(result.current.items).toHaveLength(QUALITY_FINDINGS_MAX_ITEMS)
    expect(result.current.capped).toBe(true)
    const calls = call.mock.calls.length
    act(() => result.current.loadMore())
    await flush()
    expect(call.mock.calls.length).toBe(calls)
  })

  it('reloads when the epoch changes and on filter change', async () => {
    call.mockReturnValue(page([makeQualityFinding()]))
    const { rerender } = renderHook(
      (p: { sev: 'error'[] }) => useQualityFindings('wt', { ...FILTER, severities: p.sev }),
      {
        initialProps: { sev: [] as 'error'[] }
      }
    )
    await flush()
    expect(call).toHaveBeenCalledTimes(1)
    act(() => useAppStore.getState().invalidateQuality('wt'))
    await flush()
    expect(call).toHaveBeenCalledTimes(2)
    rerender({ sev: ['error'] })
    await flush()
    expect(call).toHaveBeenCalledTimes(3)
  })

  it('exposes an inline error and retries', async () => {
    call.mockReturnValueOnce(
      Promise.resolve({
        ok: false,
        error: { kind: 'offline', code: null, message: 'x', data: null, retryable: true }
      })
    )
    const { result } = renderHook(() => useQualityFindings('wt', FILTER))
    await flush()
    expect(result.current.status).toBe('error')
    call.mockReturnValueOnce(page([makeQualityFinding()]))
    act(() => result.current.retry())
    await flush()
    expect(result.current.status).toBe('ready')
  })

  it('makes no call when support is off or there is no worktree', async () => {
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled' } } as never)
    const off = renderHook(() => useQualityFindings('wt', FILTER))
    await flush()
    off.unmount()
    useAppStore.setState({ codeIntelSupportState: ENABLED_QUALITY_SUPPORT } as never)
    renderHook(() => useQualityFindings(null, FILTER))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('patchWaiver updates one row and the returned function restores it', async () => {
    call.mockReturnValue(page(makeQualityFindings(2)))
    const { result } = renderHook(() => useQualityFindings('wt', FILTER))
    await flush()
    let restore = () => {}
    act(() => {
      restore = result.current.patchWaiver('fp-00001', { by: '', reason: 'r', expiresAt: 'x' })
    })
    expect(result.current.items[1].waiver?.reason).toBe('r')
    act(() => restore())
    expect(result.current.items[1].waiver).toBeUndefined()
  })
})

describe('mergeQualityFindingPages', () => {
  it('drops duplicate fingerprints', () => {
    const a = makeQualityFindings(3)
    expect(
      mergeQualityFindingPages(a, [a[1], makeQualityFinding({ fingerprint: 'new' })]).items
    ).toHaveLength(4)
  })
})
