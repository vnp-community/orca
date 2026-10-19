// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useExecutionSummary } from './useExecutionSummary'
import { emitRequestEvent } from '../lib/request-event-bus'

const summary = (done: number) => ({
  summary: {
    requestId: 'r1',
    status: 'executing',
    engine: 'openspec',
    executionMode: 'continuous',
    maxAttempts: 2,
    progress: {
      phasesTotal: 3,
      phasesDone: 1,
      tasksTotal: 10,
      tasksDone: done,
      tasksFailed: 0,
      tasksRunning: 1
    }
  }
})

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: summary(3) })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useExecutionSummary', () => {
  it('reads the summary of the request', async () => {
    const { result } = renderHook(() => useExecutionSummary('r1'))
    await waitFor(() => expect(result.current.summary?.progress.tasksDone).toBe(3))
    expect(callRequestRpc).toHaveBeenCalledWith('request.executionSummary', { id: 'r1' })
    expect(result.current.summary?.engine).toBe('openspec')
    expect(result.current.unsupported).toBe(false)
  })

  it('does not read while disabled or without a request', () => {
    renderHook(() => useExecutionSummary('r1', { enabled: false }))
    renderHook(() => useExecutionSummary(null))
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('refreshes on an event of the same request and ignores other requests', async () => {
    const { result } = renderHook(() => useExecutionSummary('r1', { pollMs: 0 }))
    await waitFor(() => expect(result.current.summary).not.toBeNull())
    callRequestRpc.mockResolvedValue({ ok: true, value: summary(7) })
    act(() =>
      emitRequestEvent({
        requestId: 'other',
        eventType: 'orca.request.phase.completed',
        occurredAt: ''
      })
    )
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.phase.completed',
        occurredAt: ''
      })
    )
    await waitFor(() => expect(result.current.summary?.progress.tasksDone).toBe(7))
  })

  it('polls while enabled and visible', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const { result } = renderHook(() => useExecutionSummary('r1', { pollMs: 1000 }))
    await vi.waitFor(() => expect(result.current.summary).not.toBeNull())
    const before = callRequestRpc.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    expect(callRequestRpc.mock.calls.length).toBeGreaterThanOrEqual(before + 2)
  })

  it('reports unsupported instead of an error and stops polling', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported' } })
    const { result } = renderHook(() => useExecutionSummary('r1', { pollMs: 50 }))
    await waitFor(() => expect(result.current.unsupported).toBe(true))
    expect(result.current.error).toBeNull()
    const calls = callRequestRpc.mock.calls.length
    await new Promise((r) => setTimeout(r, 160))
    expect(callRequestRpc.mock.calls.length).toBe(calls)
  })

  it('shows a failure and keeps the last good summary out of the way', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'forbidden' } })
    const { result } = renderHook(() => useExecutionSummary('r1', { pollMs: 0 }))
    await waitFor(() => expect(result.current.error).toBe('forbidden'))
    expect(result.current.summary).toBeNull()
  })

  it('drops the answer of a request that was left', async () => {
    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementationOnce(
      () =>
        new Promise((r) => {
          release = r
        })
    )
    const { result, rerender } = renderHook(({ id }) => useExecutionSummary(id, { pollMs: 0 }), {
      initialProps: { id: 'r1' }
    })
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { summary: { requestId: 'r2', status: 'executing' } }
    })
    rerender({ id: 'r2' })
    await waitFor(() => expect(result.current.summary?.requestId).toBe('r2'))
    await act(async () => release({ ok: true, value: summary(9) }))
    expect(result.current.summary?.requestId).toBe('r2')
  })
})
