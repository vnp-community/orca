// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { emitRequestEvent } from '../lib/request-event-bus'
import { usePhaseDrift } from './usePhaseDrift'

beforeEach(() => callRequestRpc.mockReset())

describe('usePhaseDrift', () => {
  it('returns drifted task ids from impact.drift and stays idle when disabled', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: {
        drift: {
          phase_id: 'ph1',
          drifted: true,
          items: [{ task_id: 't1', expected: 'a', actual: 'b' }]
        }
      }
    })
    const { result, rerender } = renderHook(
      (p: { enabled: boolean }) =>
        usePhaseDrift({ requestId: 'r1', phaseId: 'ph1', enabled: p.enabled }),
      {
        initialProps: { enabled: false }
      }
    )
    expect(callRequestRpc).not.toHaveBeenCalled()
    rerender({ enabled: true })
    await waitFor(() => expect([...result.current]).toEqual(['t1']))
    expect(callRequestRpc).toHaveBeenCalledWith('impact.drift', { phaseId: 'ph1' })
  })

  it('unsupported runtime yields no drift; drift_detected refetches', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'unsupported', code: 'x', message: 'm' }
    })
    const { result } = renderHook(() =>
      usePhaseDrift({ requestId: 'r1', phaseId: 'ph1', enabled: true })
    )
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    expect(result.current.size).toBe(0)
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { phase_id: 'ph1', items: [{ task_id: 't2' }] }
    })
    emitRequestEvent({ requestId: 'r1', eventType: 'impact.drift_detected' } as never)
    await waitFor(() => expect([...result.current]).toEqual(['t2']))
  })
})
