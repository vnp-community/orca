// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)
const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string, message = 'm') => ({ ok: false, error: { kind, code: 'x', message } })
import { useDecisions } from './useDecisions'

const dec = (id: string, status: string, subject = 's1') => ({ id, status, subject_kind: 'solution_option', subject_id: subject })

describe('useDecisions', () => {
  it('lists decisions and picks the latest non-superseded one for the solution', async () => {
    callRequestRpc.mockResolvedValue(ok({ decisions: [dec('d1', 'superseded'), dec('d2', 'chosen'), dec('d3', 'effective', 'other')] }))
    const { result } = renderHook(() => useDecisions('r1', 's1'))
    await waitFor(() => expect(result.current.current?.id).toBe('d2'))
  })

  it('confirm calls decision.confirm and reloads; mismatch is returned as a validation error', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ decisions: [dec('d2', 'chosen')] }))
    const { result } = renderHook(() => useDecisions('r1'))
    await waitFor(() => expect(result.current.current).not.toBeNull())
    callRequestRpc.mockResolvedValueOnce(fail('validation', 'REQUEST_DECISION_CONFIRMATION_MISMATCH: no'))
    const bad = await result.current.confirm('d2', 'nope', 1)
    expect(bad).toMatchObject({ ok: false, error: { kind: 'validation' } })
    expect(callRequestRpc).toHaveBeenLastCalledWith('decision.confirm', { decisionId: 'd2', confirmationText: 'nope', expectedVersion: 1 })
    callRequestRpc.mockResolvedValue(ok({ decisions: [dec('d2', 'effective')] }))
    await act(async () => { await result.current.confirm('d2', 'Title', 1) })
    await waitFor(() => expect(result.current.current?.status).toBe('effective'))
  })

  it('hides unsupported errors', async () => {
    callRequestRpc.mockResolvedValue(fail('unsupported'))
    const { result } = renderHook(() => useDecisions('r1'))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBeNull()
  })
})
