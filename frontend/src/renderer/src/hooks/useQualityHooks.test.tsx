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

import { useAppStore } from '@/store'
import { ENABLED_QUALITY_SUPPORT } from '../test-support/code-intel-quality-test-store'
import { useQualityGate } from './useQualityGate'
import { useQualityProfiles } from './useQualityProfiles'
import { useQualityRun } from './useQualityRun'
import { useQualitySupport } from './useQualitySupport'
import { publishCodeIntelEvent, resetCodeIntelEventBus } from '../lib/code-intel-event-bus'

const ok = (result: unknown) => Promise.resolve({ ok: true as const, result })
const fail = (kind: string) =>
  Promise.resolve({
    ok: false as const,
    error: { kind, code: null, message: '', data: null, retryable: false } as never
  })

async function flush(): Promise<void> {
  await act(async () => {
    for (let i = 0; i < 6; i++) {
      await Promise.resolve()
    }
  })
}

function replyByMethod(map: Record<string, unknown>) {
  call.mockImplementation((_w, method) => ok(map[method] ?? {}))
}

beforeEach(() => {
  call.mockReset()
  resetCodeIntelEventBus()
  useAppStore.setState({
    codeIntelSupportState: ENABLED_QUALITY_SUPPORT,
    codeIntelQualityByWorktree: {},
    codeIntelResyncCounter: 0,
    codeIntelEventsState: 'streaming'
  })
})
afterEach(() => cleanup())

describe('useQualitySupport', () => {
  it('is enabled only with the quality flag on', () => {
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('enabled')
    useAppStore.setState({
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: false }
      }
    })
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('disabled')
    useAppStore.setState({ codeIntelSupportState: { state: 'unsupported' } })
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('unsupported')
    useAppStore.setState({ codeIntelSupportState: { state: 'unknown' } })
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('unknown')
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled' } })
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('disabled')
  })

  it('a quality-disabled gate error switches the surface off', () => {
    useAppStore.setState({
      codeIntelQualityByWorktree: {
        wt: { errors: { gate: { kind: 'quality-disabled' } } } as never
      }
    })
    expect(renderHook(() => useQualitySupport('wt')).result.current).toBe('disabled')
  })
})

describe('no RPC unless enabled', () => {
  it.each([['unknown'], ['disabled'], ['unsupported']] as const)(
    'makes no call when support is %s',
    async (state) => {
      useAppStore.setState({ codeIntelSupportState: { state } })
      renderHook(() => useQualityGate('wt'))
      renderHook(() => useQualityRun('wt'))
      renderHook(() => useQualityProfiles('wt'))
      await flush()
      expect(call).not.toHaveBeenCalled()
    }
  )

  it('makes no call without a worktree id', async () => {
    renderHook(() => useQualityGate(null))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })
})

describe('useQualityGate', () => {
  it('loads the gate once and exposes ready data', async () => {
    replyByMethod({
      'codeIntel.quality.gate': {
        gate: { verdict: 'warn', profile: 'full@repo/v1', basedOn: { runIds: ['r'] } }
      }
    })
    const { result, rerender } = renderHook(() => useQualityGate('wt'))
    await flush()
    rerender()
    expect(result.current.status).toBe('ready')
    expect(result.current.response?.gate.verdict).toBe('warn')
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.gate')).toHaveLength(1)
  })

  it('returns a stable result between unrelated renders', async () => {
    replyByMethod({ 'codeIntel.quality.gate': { gate: { verdict: 'pass' } } })
    const { result, rerender } = renderHook(() => useQualityGate('wt'))
    await flush()
    const first = result.current
    rerender()
    expect(result.current).toBe(first)
  })

  it('reports an error status when the load fails and nothing is cached', async () => {
    call.mockImplementation(() => fail('tool-failed'))
    const { result } = renderHook(() => useQualityGate('wt'))
    await flush()
    expect(result.current.status).toBe('error')
    expect(result.current.error?.kind).toBe('tool-failed')
  })

  it('forwards base and profile name', async () => {
    replyByMethod({})
    renderHook(() => useQualityGate('wt', { base: 'main', profileName: 'full' }))
    await flush()
    expect(call).toHaveBeenCalledWith('wt', 'codeIntel.quality.gate', {
      base: 'main',
      profileName: 'full'
    })
  })

  it('releases the push listener on unmount', async () => {
    replyByMethod({ 'codeIntel.quality.gate': { gate: { verdict: 'pass' } } })
    const { unmount } = renderHook(() => useQualityGate('wt'))
    await flush()
    unmount()
    call.mockClear()
    publishCodeIntelEvent({ event: 'gateChanged', worktreeId: 'wt' })
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('reloads after a gateChanged push while mounted', async () => {
    replyByMethod({ 'codeIntel.quality.gate': { gate: { verdict: 'pass' } } })
    renderHook(() => useQualityGate('wt'))
    await flush()
    publishCodeIntelEvent({ event: 'gateChanged', worktreeId: 'wt' })
    await flush()
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.gate')).toHaveLength(2)
  })
})

describe('useQualityRun', () => {
  it('re-attaches to a run already in progress after mount', async () => {
    replyByMethod({
      'codeIntel.quality.runs': {
        runs: [
          { id: 'live', status: 'running', source: 'local', profile: 'full', scope: 'changed' }
        ]
      }
    })
    const { result } = renderHook(() => useQualityRun('wt'))
    await flush()
    expect(result.current.run).toMatchObject({ runId: 'live', phase: 'running' })
  })

  it('start locks immediately and cancel only requests', async () => {
    replyByMethod({ 'codeIntel.quality.start': { run: { id: 'r1', status: 'running' } } })
    const { result } = renderHook(() => useQualityRun('wt'))
    await flush()
    await act(async () => {
      void result.current.start({ profile: 'full', scope: 'changed' })
    })
    expect(
      result.current.run?.phase === 'starting' || result.current.run?.phase === 'running'
    ).toBe(true)
    await flush()
    await act(async () => {
      await result.current.cancel()
    })
    expect(result.current.run?.phase).toBe('cancelling')
  })

  it('a stream resync re-reads the active run', async () => {
    replyByMethod({
      'codeIntel.quality.runs': { runs: [{ id: 'live', status: 'running', source: 'local' }] },
      'codeIntel.quality.run': { id: 'live', status: 'succeeded' }
    })
    const { result } = renderHook(() => useQualityRun('wt'))
    await flush()
    act(() => useAppStore.getState().triggerCodeIntelResync())
    await flush()
    expect(result.current.run).toMatchObject({ phase: 'finished', status: 'succeeded' })
  })
})

describe('useQualityProfiles', () => {
  const body = {
    profile: { name: 'full', mode: 'inform' },
    runnableProfiles: [
      { id: 'lint', ready: false },
      { id: 'full', ready: true }
    ]
  }

  it('selects the configured profile and keeps a stable runnable list', async () => {
    replyByMethod({ 'codeIntel.quality.profile.get': body })
    const { result, rerender } = renderHook(() => useQualityProfiles('wt'))
    await flush()
    expect(result.current.selected).toBe('full')
    const list = result.current.runnable
    rerender()
    expect(result.current.runnable).toBe(list)
  })

  it('select stores the pick in the UI state', async () => {
    replyByMethod({ 'codeIntel.quality.profile.get': body })
    const { result } = renderHook(() => useQualityProfiles('wt'))
    await flush()
    act(() => result.current.select('lint'))
    expect(result.current.selected).toBe('lint')
    expect(result.current.selectedProfile?.ready).toBe(false)
  })

  it('uses the gate profile reference when the user has not chosen', async () => {
    replyByMethod({ 'codeIntel.quality.profile.get': body })
    const { result } = renderHook(() => useQualityProfiles('wt', 'lint@repo/v2'))
    await flush()
    expect(result.current.selected).toBe('lint')
  })
})
