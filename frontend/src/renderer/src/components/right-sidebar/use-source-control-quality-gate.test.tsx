// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: false }
const call = vi.fn()

vi.mock('@/store', () => ({ useAppStore: { getState: () => ({}) } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('@/lib/ensure-review-tab', () => ({ ensureReviewTab: vi.fn() }))
vi.mock('../../hooks/useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call }) }))

import { ensureReviewTab } from '@/lib/ensure-review-tab'
import { publishCodeIntelEvent, resetCodeIntelEventBus } from '../../lib/code-intel-event-bus'
import { useSourceControlQualityGate } from './use-source-control-quality-gate'

const baseOpts = { worktreeId: 'wt-1', projectId: 'p-1', headOid: 'abc', base: 'main' }

function gate(result: string) {
  return { ok: true, result: { gate: { result, reasons: [], stale: false, unavailable: false } } }
}

beforeEach(() => {
  vi.useFakeTimers()
  call.mockReset()
  flags.quality = true
})
afterEach(() => {
  vi.useRealTimers()
  resetCodeIntelEventBus()
})

describe('useSourceControlQualityGate', () => {
  it('makes no RPC and stays hidden when the quality flag is off', async () => {
    flags.quality = false
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(call).not.toHaveBeenCalled()
    expect(result.current.viewModel).toEqual({ visible: false })
  })

  it('makes no RPC without a projectId', async () => {
    renderHook(() => useSourceControlQualityGate({ ...baseOpts, projectId: null }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(call).not.toHaveBeenCalled()
  })

  it('loads the gate after the debounce and hides pass', async () => {
    call.mockResolvedValue(gate('pass'))
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(call).toHaveBeenCalledTimes(1)
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.gate')
    expect(call.mock.calls[0][2]).toEqual({ projectId: 'p-1', worktreeId: 'wt-1', base: 'main' })
    expect(result.current.viewModel.visible).toBe(false)
  })

  it('shows a fail notice and never blocks (informational only)', async () => {
    call.mockResolvedValue(gate('fail'))
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(result.current.viewModel).toMatchObject({ visible: true, severity: 'fail' })
  })

  it('hides silently on a disabled error', async () => {
    call.mockResolvedValue({ ok: false, error: { kind: 'disabled', message: 'x' } })
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(result.current.viewModel.visible).toBe(false)
  })

  it('shows unknown after the 3 s display timeout and still applies a late result', async () => {
    let resolveCall: (v: unknown) => void = () => {}
    call.mockReturnValue(new Promise((r) => (resolveCall = r)))
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100)
    })
    expect(result.current.timedOut).toBe(true)
    expect(result.current.viewModel).toMatchObject({ visible: true, severity: 'unknown' })
    await act(async () => {
      resolveCall(gate('warn'))
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(result.current.viewModel).toMatchObject({ severity: 'warn' })
  })

  it('reloads on gateChanged for the same worktree only', async () => {
    call.mockResolvedValue(gate('pass'))
    renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500)
    })
    await act(async () => {
      publishCodeIntelEvent({ event: 'gateChanged', worktreeId: 'other' })
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(call).toHaveBeenCalledTimes(1)
    await act(async () => {
      publishCodeIntelEvent({ event: 'gateChanged', worktreeId: 'wt-1' })
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(call).toHaveBeenCalledTimes(2)
  })

  it('debounces headOid changes into one call', async () => {
    call.mockResolvedValue(gate('pass'))
    const { rerender } = renderHook((p) => useSourceControlQualityGate(p), {
      initialProps: baseOpts
    })
    rerender({ ...baseOpts, headOid: 'def' })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500)
    })
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('runChecks picks a ready non-heavy profile and starts a changed-scope run', async () => {
    call.mockImplementation(async (_wt: string, method: string) => {
      if (method === 'codeIntel.quality.profile.get') {
        return {
          ok: true,
          result: {
            runnableProfiles: [
              { name: 'heavy', ready: true, heavy: true },
              { name: 'fast', ready: true, heavy: false }
            ]
          }
        }
      }
      if (method === 'codeIntel.quality.start') {
        return { ok: true, result: { run: {} } }
      }
      return gate('pass')
    })
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    await act(async () => {
      result.current.runChecks()
      await vi.advanceTimersByTimeAsync(0)
    })
    const start = call.mock.calls.find((c) => c[1] === 'codeIntel.quality.start')
    expect(start?.[2]).toMatchObject({ profile: 'fast', scope: 'changed' })
    expect(result.current.running).toBe(true)
    await act(async () => {
      publishCodeIntelEvent({
        event: 'qualityFinished',
        worktreeId: 'wt-1',
        runId: 'r',
        success: true,
        error: null
      })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(result.current.running).toBe(false)
  })

  it('openReason opens the review tab', () => {
    const { result } = renderHook(() => useSourceControlQualityGate(baseOpts))
    result.current.openReason('lint')
    expect(ensureReviewTab).toHaveBeenCalledWith('wt-1')
  })
})
