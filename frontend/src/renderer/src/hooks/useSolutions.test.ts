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
  callRequestRpc.mockResolvedValue({ ok: true, value: { solutions: [{ id: 's1', requestId: 'r1' }] } })
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
})
