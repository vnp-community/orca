// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useSolutions } from './useSolutions'

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({
    ok: true,
    value: { solutions: [{ id: 's1', requestId: 'r1' }] }
  })
})

describe('useSolutions', () => {
  it('lists solutions for the request and skips when id is null', async () => {
    const { result } = renderHook(() => useSolutions('r1'))
    await waitFor(() => expect(result.current.solutions).toHaveLength(1))
    expect(callRequestRpc).toHaveBeenCalledWith('solution.list', { requestId: 'r1' })
    callRequestRpc.mockClear()
    renderHook(() => useSolutions(null))
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('generate sends a fresh idempotencyKey; choose sends the option id', async () => {
    const { result } = renderHook(() => useSolutions('r1'))
    await waitFor(() => expect(result.current.solutions).toHaveLength(1))
    await result.current.generate({ feedback: 'more' })
    await result.current.generate()
    const gen = callRequestRpc.mock.calls.filter((c) => c[0] === 'solution.generate')
    expect(gen[0][1].idempotencyKey).toBeTruthy()
    expect(gen[0][1].idempotencyKey).not.toBe(gen[1][1].idempotencyKey)
    await result.current.choose({ solutionId: 's1', optionId: '2' })
    expect(callRequestRpc).toHaveBeenLastCalledWith('solution.choose', {
      requestId: 'r1',
      solutionId: 's1',
      optionId: '2'
    })
  })

  it('exposes the newest run so a failed AI run explains why there is no solution', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: {
        solutions: [],
        runs: [
          {
            id: 'run1',
            kind: 'solution',
            status: 'failed',
            errorCode: 'REQUEST_SOLUTION_INVALID_OUTPUT',
            startedAt: '2026-10-10T01:00:00Z'
          },
          {
            id: 'run2',
            kind: 'solution',
            status: 'failed',
            errorCode: 'REQUEST_AI_NOT_CONFIGURED',
            errorMessage: 'log in',
            startedAt: '2026-10-10T02:00:00Z',
            finishedAt: '2026-10-10T02:00:01Z'
          }
        ]
      }
    })
    const { result } = renderHook(() => useSolutions('r1'))
    await waitFor(() => expect(result.current.latestRun?.id).toBe('run2'))
    expect(result.current.latestRun).toMatchObject({
      status: 'failed',
      errorCode: 'REQUEST_AI_NOT_CONFIGURED'
    })
  })

  it('has no run when the service sends none', async () => {
    const { result } = renderHook(() => useSolutions('r1'))
    await waitFor(() => expect(result.current.solutions).toHaveLength(1))
    expect(result.current.latestRun).toBeNull()
  })

  describe('generateWhenIdle', () => {
    const answerList = (value: unknown): void => {
      callRequestRpc.mockImplementation(async (method: string) =>
        method === 'solution.list' ? { ok: true, value } : { ok: true, value: { runId: 'run9' } }
      )
    }
    const generateCalls = (): unknown[][] =>
      callRequestRpc.mock.calls.filter((c) => c[0] === 'solution.generate')

    it('re-reads the list and generates once when nothing exists or runs', async () => {
      answerList({
        solutions: [],
        runs: [{ id: 'old', kind: 'solution', status: 'failed', startedAt: '2026-10-10T01:00:00Z' }]
      })
      const { result } = renderHook(() => useSolutions('r1'))
      await waitFor(() => expect(result.current.isLoading).toBe(false))
      callRequestRpc.mockClear()
      const res = await result.current.generateWhenIdle()
      expect(res).toEqual({ ok: true, value: { skipped: false } })
      expect(callRequestRpc.mock.calls[0][0]).toBe('solution.list')
      expect(generateCalls()).toHaveLength(1)
      expect(generateCalls()[0][1]).toMatchObject({ requestId: 'r1' })
      expect((generateCalls()[0][1] as { idempotencyKey: string }).idempotencyKey).toBeTruthy()
    })

    it('skips when a run is already running, even if the cached list was empty', async () => {
      answerList({ solutions: [], runs: [] })
      const { result } = renderHook(() => useSolutions('r1'))
      await waitFor(() => expect(result.current.isLoading).toBe(false))
      answerList({
        solutions: [],
        runs: [{ id: 'r', kind: 'solution', status: 'running', startedAt: '2026-10-10T02:00:00Z' }]
      })
      callRequestRpc.mockClear()
      expect(await result.current.generateWhenIdle()).toEqual({
        ok: true,
        value: { skipped: true }
      })
      expect(generateCalls()).toHaveLength(0)
    })

    it('skips when a Solution already exists', async () => {
      answerList({ solutions: [{ id: 's1', requestId: 'r1' }], runs: [] })
      const { result } = renderHook(() => useSolutions('r1'))
      await waitFor(() => expect(result.current.solutions).toHaveLength(1))
      expect(await result.current.generateWhenIdle()).toEqual({
        ok: true,
        value: { skipped: true }
      })
      expect(generateCalls()).toHaveLength(0)
    })

    it('returns the error when the list read or the generate call fails', async () => {
      answerList({ solutions: [], runs: [] })
      const { result } = renderHook(() => useSolutions('r1'))
      await waitFor(() => expect(result.current.isLoading).toBe(false))
      const error = { kind: 'network', code: 'x', message: 'down' }
      callRequestRpc.mockResolvedValueOnce({ ok: false, error })
      expect(await result.current.generateWhenIdle()).toEqual({ ok: false, error })
      expect(generateCalls()).toHaveLength(0)
      callRequestRpc.mockResolvedValueOnce({ ok: true, value: { solutions: [], runs: [] } })
      callRequestRpc.mockResolvedValueOnce({ ok: false, error: { ...error, kind: 'rate_limited' } })
      expect(await result.current.generateWhenIdle()).toMatchObject({
        ok: false,
        error: { kind: 'rate_limited' }
      })
    })
  })
})
