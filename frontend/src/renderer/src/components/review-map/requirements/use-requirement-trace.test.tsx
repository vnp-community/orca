// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { requirementWire, traceWire } from './requirement-trace.fixture'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: false }
const call = vi.fn()

vi.mock('@/store', () => ({ useAppStore: { getState: () => ({}) } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('../../../hooks/useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../../../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call }) }))

import { publishCodeIntelEvent, resetCodeIntelEventBus } from '../../../lib/code-intel-event-bus'
import { useRequirementTrace } from './use-requirement-trace'

const args: { projectId: string | null; worktreeId: string } = { projectId: 'p', worktreeId: 'wt' }
const ok = (over: Record<string, unknown> = {}) => ({ ok: true, result: { trace: traceWire(over) } })

beforeEach(() => {
  vi.useFakeTimers()
  call.mockReset()
  flags.quality = true
})
afterEach(() => {
  vi.useRealTimers()
  resetCodeIntelEventBus()
})

async function mount(a = args) {
  const hook = renderHook((p) => useRequirementTrace(p), { initialProps: a })
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  return hook
}

describe('useRequirementTrace', () => {
  it('is disabled with zero RPCs when the quality flag is off', async () => {
    flags.quality = false
    const { result } = await mount()
    expect(result.current.status).toBe('disabled')
    expect(call).not.toHaveBeenCalled()
  })

  it('does nothing without a projectId', async () => {
    const { result } = await mount({ projectId: null, worktreeId: 'wt' })
    expect(result.current.status).toBe('disabled')
    expect(call).not.toHaveBeenCalled()
  })

  it('loads quality.trace and builds the view', async () => {
    call.mockResolvedValue(ok())
    const { result } = await mount()
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.trace')
    expect(call.mock.calls[0][2]).toMatchObject({ projectId: 'p', worktreeId: 'wt', includeInferred: false })
    expect(result.current.status).toBe('ready')
    expect(result.current.view?.counts.hasEvidence).toBe(1)
  })

  it('refetches with includeInferred when the toggle changes', async () => {
    call.mockResolvedValue(ok())
    const { result } = await mount()
    await act(async () => {
      result.current.setShowInferred(true)
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(call).toHaveBeenCalledTimes(2)
    expect(call.mock.calls[1][2]).toMatchObject({ includeInferred: true })
  })

  it.each(['disabled', 'forbidden', 'unsupported'])('hides silently on %s', async (kind) => {
    call.mockResolvedValue({ ok: false, error: { kind, message: 'x', data: null } })
    const { result } = await mount()
    expect(result.current.status).toBe('disabled')
  })

  it('retries inProgress timeouts, then reports an error for other failures', async () => {
    call
      .mockResolvedValueOnce({ ok: false, error: { kind: 'unknown', message: 'inProgress', data: { retryAfterMs: 1000 } } })
      .mockResolvedValueOnce({ ok: false, error: { kind: 'unknown', message: 'boom', data: null } })
    const { result } = await mount()
    expect(result.current.status).toBe('loading')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current.status).toBe('error')
  })

  it('reloads on qualityFinished for this worktree only; "changed" only marks stale', async () => {
    call.mockResolvedValue(ok())
    const { result } = await mount()
    await act(async () => {
      publishCodeIntelEvent({ event: 'qualityFinished', worktreeId: 'other', runId: 'r', success: true, error: null })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(call).toHaveBeenCalledTimes(1)
    await act(async () => {
      publishCodeIntelEvent({ event: 'changed', worktreeId: 'wt' })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(result.current.stale).toBe(true)
    expect(call).toHaveBeenCalledTimes(1)
    await act(async () => {
      publishCodeIntelEvent({ event: 'qualityFinished', worktreeId: 'wt', runId: 'r', success: true, error: null })
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current.stale).toBe(false)
  })

  it('replaces the trace with the action response (no optimism) and locks to read-only on forbidden', async () => {
    call.mockResolvedValueOnce(ok({ requirements: [requirementWire({ key: 'a', state: 'no_evidence', evidence: [] })] }))
    const { result } = await mount()
    expect(result.current.view?.counts.noEvidence).toBe(1)

    call.mockResolvedValueOnce(ok({ requirements: [requirementWire({ key: 'a' })] }))
    await act(async () => {
      await result.current.confirm('a', { kind: 'change', ref: 'src/a.ts' })
    })
    expect(call.mock.calls[1][1]).toBe('codeIntel.quality.trace.confirm')
    expect(result.current.view?.counts.hasEvidence).toBe(1)

    call.mockResolvedValueOnce({ ok: false, error: { kind: 'forbidden' } })
    await act(async () => {
      await result.current.reject('a', { kind: 'change', ref: 'src/a.ts' })
    })
    expect(result.current.readOnly).toBe(true)
    expect(result.current.view?.counts.hasEvidence).toBe(1)
  })
})
