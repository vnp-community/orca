// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { QualityCall } from '../store/slices/code-intel-quality-slice-context'

const call = vi.fn<QualityCall>()

vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../test-support/code-intel-quality-test-store')
  return { useAppStore: createCodeIntelQualityTestStore((...args) => call(...args)) }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({
    state: 'ready',
    worktreeId: 'wt',
    projectId: 'p',
    environmentId: null
  })
}))

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../test-support/code-intel-quality-test-store'
import {
  buildCoverageReport,
  buildHotspotFindings,
  buildStructureGraph,
  buildTrendResponse
} from '../test-support/code-intel-quality-visualization-fake-data'
import type { CodeIntelCallFn } from './useCodeIntelQuery'
import { useQualityCoverage } from './useQualityCoverage'
import { useQualityDependencyMatrix } from './useQualityDependencyMatrix'
import { useQualityHotspots } from './useQualityHotspots'
import { useQualityTrend } from './useQualityTrend'

const ok = (result: unknown) => Promise.resolve({ ok: true as const, result })
const fail = (kind = 'tool-failed') =>
  Promise.resolve({
    ok: false as const,
    error: { kind, code: null, message: '', data: null, retryable: false } as never
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
    codeIntelQualityByWorktree: {},
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0,
    codeIntelEventsState: 'streaming'
  })
})
afterEach(() => cleanup())

const DISABLED = [
  ['unknown', { state: 'unknown' }],
  ['disabled', { state: 'disabled' }],
  ['unsupported', { state: 'unsupported' }],
  [
    'quality flag off',
    { state: 'enabled', effective: { codeIntelEnabled: true, qualityGateEnabled: false } }
  ]
] as const

describe('no RPC unless quality support is enabled', () => {
  it.each(DISABLED)('makes no call when support is %s', async (_name, support) => {
    useAppStore.setState({ codeIntelSupportState: support as never })
    const query = vi.fn<CodeIntelCallFn>()
    renderHook(() => useQualityTrend('wt', 'turn'))
    renderHook(() => useQualityCoverage('wt'))
    renderHook(() => useQualityHotspots('wt', query))
    renderHook(() => useQualityDependencyMatrix('wt', query))
    await flush()
    expect(call).not.toHaveBeenCalled()
    expect(query).not.toHaveBeenCalled()
  })
})

describe('useQualityTrend', () => {
  it('loads the requested grouping and exposes ready points', async () => {
    call.mockImplementation(() => ok(buildTrendResponse(3, 9)))
    const { result } = renderHook(() => useQualityTrend('wt', 'turn'))
    expect(result.current.status).toBe('loading')
    await flush()
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.trend')
    expect(call.mock.calls[0][2]).toEqual({ limit: 50, groupBy: 'turn' })
    expect(result.current.status).toBe('ready')
    expect(result.current.points).toHaveLength(3)
    expect(result.current.truncated).toBe(true)
    expect(result.current.totalCount).toBe(9)
  })

  it('keeps separate data per grouping and refetches on change', async () => {
    call.mockImplementation((_w, _m, params) =>
      ok(buildTrendResponse((params as { groupBy: string }).groupBy === 'turn' ? 2 : 4))
    )
    const { result, rerender } = renderHook(({ g }) => useQualityTrend('wt', g), {
      initialProps: { g: 'turn' as const } as { g: 'turn' | 'commit' }
    })
    await flush()
    rerender({ g: 'commit' })
    await flush()
    expect(call.mock.calls.map((c) => (c[2] as { groupBy: string }).groupBy)).toEqual([
      'turn',
      'commit'
    ])
    expect(result.current.points).toHaveLength(4)
  })

  it('is empty for no points, error without data, stale after a failed refresh', async () => {
    call.mockImplementation(() => ok({ points: [], truncated: false, totalCount: 0 }))
    const { result } = renderHook(() => useQualityTrend('wt', 'turn'))
    await flush()
    expect(result.current.status).toBe('empty')

    call.mockReset()
    call.mockImplementation(() => fail())
    useAppStore.setState({ codeIntelQualityByWorktree: {} })
    const failing = renderHook(() => useQualityTrend('wt2', 'turn'))
    await flush()
    expect(failing.result.current.status).toBe('error')

    call.mockReset()
    call.mockImplementationOnce(() => ok(buildTrendResponse(3)))
    const stale = renderHook(() => useQualityTrend('wt3', 'turn'))
    await flush()
    expect(stale.result.current.status).toBe('ready')
    call.mockImplementation(() => fail())
    act(() => stale.result.current.refetch())
    await flush()
    expect(stale.result.current.status).toBe('stale')
    expect(stale.result.current.points).toHaveLength(3)
  })
})

describe('useQualityCoverage', () => {
  it('turns report:null into an empty state with the backend reason, not a report', async () => {
    call.mockImplementation(() => ok({ report: null, reason: 'no coverage artifact' }))
    const { result } = renderHook(() => useQualityCoverage('wt'))
    await flush()
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.coverage')
    expect(result.current.status).toBe('empty')
    expect(result.current.report).toBeNull()
    expect(result.current.reason).toBe('no coverage artifact')
  })

  it('exposes a ready report and refetches when a run bumps the epoch', async () => {
    call.mockImplementation(() => ok({ report: buildCoverageReport('estimated') }))
    const { result } = renderHook(() => useQualityCoverage('wt'))
    await flush()
    expect(result.current.status).toBe('ready')
    expect(result.current.report?.source).toBe('estimated')
    act(() => {
      useAppStore.setState({
        codeIntelQualityByWorktree: {
          wt: { ...useAppStore.getState().codeIntelQualityByWorktree.wt, epoch: 5 }
        }
      })
    })
    await flush()
    expect(call.mock.calls.length).toBeGreaterThan(1)
  })
})

describe('useQualityHotspots', () => {
  it('asks only for hotspot.file findings and exposes them', async () => {
    const query = vi.fn<CodeIntelCallFn>().mockResolvedValue({
      ok: true,
      result: { findings: buildHotspotFindings(3), dismissedCount: 0 },
      meta: { totalCount: 5 } as never
    })
    const { result } = renderHook(() => useQualityHotspots('wt', query))
    await flush()
    expect(query.mock.calls[0][1]).toBe('codeIntel.findings')
    expect(query.mock.calls[0][2]).toMatchObject({ rules: ['hotspot.file'], scope: 'all' })
    expect(result.current.status).toBe('ready')
    expect(result.current.findings).toHaveLength(3)
    expect(result.current.totalCount).toBe(5)
  })

  it('is empty when the backend has no hotspots, and aborts the request on unmount', async () => {
    let signal: AbortSignal | undefined
    const query = vi.fn<CodeIntelCallFn>().mockImplementation((_w, _m, _p, s) => {
      signal = s
      return new Promise(() => undefined)
    })
    const { unmount } = renderHook(() => useQualityHotspots('wt', query))
    await flush()
    expect(signal?.aborted).toBe(false)
    unmount()
    expect(signal?.aborted).toBe(true)

    const empty = vi.fn<CodeIntelCallFn>().mockResolvedValue({ ok: true, result: { findings: [] } })
    const { result } = renderHook(() => useQualityHotspots('wt', empty))
    await flush()
    expect(result.current.status).toBe('empty')
  })
})

describe('useQualityDependencyMatrix', () => {
  it('requests structure at depth 2 and reports truncation from the envelope', async () => {
    const query = vi.fn<CodeIntelCallFn>().mockResolvedValue({
      ok: true,
      result: buildStructureGraph('cycle'),
      meta: { truncated: true, totalCount: 400 } as never
    })
    const { result } = renderHook(() => useQualityDependencyMatrix('wt', query))
    await flush()
    expect(query.mock.calls[0][1]).toBe('codeIntel.structure')
    expect(query.mock.calls[0][2]).toEqual({ depth: 2 })
    expect(result.current.status).toBe('ready')
    expect(result.current.truncated).toBe(true)
    expect(result.current.totalCount).toBe(400)
  })

  it('is empty for a graph without edges and error on failure', async () => {
    const empty = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValue({ ok: true, result: { nodes: [], edges: [] } })
    const a = renderHook(() => useQualityDependencyMatrix('wt', empty))
    await flush()
    expect(a.result.current.status).toBe('empty')
    cleanup()
    const failing = vi.fn<CodeIntelCallFn>().mockResolvedValue({
      ok: false,
      error: { kind: 'too-large', code: null, message: 'x', retryable: false }
    })
    const b = renderHook(() => useQualityDependencyMatrix('wt-other', failing))
    await flush()
    expect(b.result.current.status).toBe('error')
    expect(b.result.current.error?.kind).toBe('too-large')
  })
})
