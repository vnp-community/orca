// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})

import { useAppStore } from '@/store'
import { publishCodeIntelEvent, resetCodeIntelEventBus } from '../lib/code-intel-event-bus'
import { REINDEX_POLL_MS, useCodeIntelReindex } from './useCodeIntelReindex'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'

const job = (status: string, percent: number | null = null): CodeIntelCallOutcome => ({
  ok: true,
  result: { jobId: 'job-1', status, mode: 'incremental', trigger: 'manual', stage: 'parse', percent, message: '' }
})
const err = (kind: string, data: Record<string, unknown>): CodeIntelCallOutcome => ({
  ok: false,
  error: { kind: kind as never, code: null, message: kind, retryable: false, data }
})

beforeEach(() => {
  vi.useFakeTimers()
  resetCodeIntelEventBus()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useCodeIntelReindex.start', () => {
  it('locks synchronously: two quick calls make one request', async () => {
    let resolve!: (o: CodeIntelCallOutcome) => void
    const call = vi.fn<CodeIntelCallFn>().mockImplementation(() => new Promise((r) => (resolve = r)))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    act(() => {
      void result.current.start('full')
      void result.current.start('full')
    })
    expect(result.current.isStarting).toBe(true)
    expect(call).toHaveBeenCalledTimes(1)
    expect(call.mock.calls[0][1]).toBe('codeIntel.reindex')
    expect(call.mock.calls[0][2]).toEqual({ mode: 'full' })
    await act(async () => resolve(job('running', 10)))
    expect(result.current.isStarting).toBe(false)
    expect(result.current.job).toMatchObject({ jobId: 'job-1', status: 'running', percent: 10 })
    expect(result.current.isRunning).toBe(true)
  })

  it('defaults to incremental', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(job('queued'))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    await act(async () => result.current.start())
    expect(call.mock.calls[0][2]).toEqual({ mode: 'incremental' })
  })

  it('REINDEX_IN_PROGRESS attaches to the running job without an error', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(err('reindex-in-progress', { jobId: 'job-9', stage: 'scan' }))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    await act(async () => result.current.start())
    expect(result.current.job).toMatchObject({ jobId: 'job-9', status: 'running', stage: 'scan' })
    expect(result.current.error).toBeNull()
  })

  it('REINDEX_COOLDOWN sets cooldownUntil from retryAfterSeconds', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(err('rate-limited', { retryAfterSeconds: 300 }))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call, now: () => 1_000 }))
    await act(async () => result.current.start())
    expect(result.current.cooldownUntil).toBe(1_000 + 300_000)
    expect(result.current.job).toBeNull()
  })

  it('other errors are returned as a snapshot, not thrown', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(err('forbidden', {}))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    await act(async () => result.current.start())
    expect(result.current.error?.kind).toBe('forbidden')
  })
})

describe('useCodeIntelReindex progress', () => {
  it('push progress updates percent and keeps null', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(job('running', 10))
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    await act(async () => result.current.start())
    act(() => publishCodeIntelEvent({ event: 'reindexProgress', worktreeId: 'wt', percent: 55, overall: 'BUILDING' }))
    expect(result.current.job?.percent).toBe(55)
    act(() => publishCodeIntelEvent({ event: 'reindexProgress', worktreeId: 'wt', percent: null, overall: 'BUILDING' }))
    expect(result.current.job?.percent).toBeNull()
    act(() => publishCodeIntelEvent({ event: 'reindexProgress', worktreeId: 'other', percent: 99, overall: 'BUILDING' }))
    expect(result.current.job?.percent).toBeNull()
  })

  it('polls reindexStatus every 2 s; success refreshes status and bumps the resync counter', async () => {
    const onSucceeded = vi.fn()
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(job('running'))
      .mockResolvedValueOnce(job('running', 50))
      .mockResolvedValueOnce(job('succeeded', 100))
    useAppStore.setState({ codeIntelResyncCounter: 0 })
    const { result } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call, onSucceeded }))
    await act(async () => result.current.start())

    await act(async () => vi.advanceTimersByTimeAsync(REINDEX_POLL_MS))
    expect(call.mock.calls[1][1]).toBe('codeIntel.reindexStatus')
    expect(call.mock.calls[1][2]).toEqual({ jobId: 'job-1' })
    expect(result.current.job?.percent).toBe(50)

    await act(async () => vi.advanceTimersByTimeAsync(REINDEX_POLL_MS))
    expect(result.current.job?.status).toBe('succeeded')
    expect(onSucceeded).toHaveBeenCalledTimes(1)
    expect(useAppStore.getState().codeIntelResyncCounter).toBe(1)

    // Polling stops once the job is terminal
    await act(async () => vi.advanceTimersByTimeAsync(REINDEX_POLL_MS * 3))
    expect(call).toHaveBeenCalledTimes(3)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('stops polling on unmount', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(job('running'))
    const { result, unmount } = renderHook(() => useCodeIntelReindex('wt', null, { callFn: call }))
    await act(async () => result.current.start())
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
