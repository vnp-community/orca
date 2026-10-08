// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))
import { emitRequestEvent } from '../lib/request-event-bus'

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)
const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string, message = 'm') => ({ ok: false, error: { kind, code: 'x', message } })
import { summarizePhaseReadiness, useTaskReadiness } from './useTaskReadiness'

const rep = (taskId: string, outcome: string) => ({ task_id: taskId, outcome, findings: [] })

describe('useTaskReadiness', () => {
  it('loads a task report; check updates it', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ report: rep('t1', 'needs_info') }))
    const { result } = renderHook(() => useTaskReadiness({ taskId: 't1' }))
    await waitFor(() => expect(result.current.report?.outcome).toBe('needs_info'))
    callRequestRpc.mockResolvedValueOnce(ok({ report: rep('t1', 'ready') }))
    await act(async () => { await result.current.check('t1') })
    expect(result.current.report?.outcome).toBe('ready')
  })

  it('phase list builds a summary including unchecked tasks', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ reports: [rep('a', 'ready'), rep('b', 'env_defect')] }))
    const { result } = renderHook(() => useTaskReadiness({ phaseId: 'p', phaseTaskIds: ['a', 'b', 'c'] }))
    await waitFor(() => expect(result.current.phaseSummary.ready).toBe(1))
    expect(result.current.phaseSummary).toEqual({ ready: 1, needsInfo: 0, specDefect: 0, envDefect: 1, unchecked: 1 })
    expect(summarizePhaseReadiness([], {})).toMatchObject({ unchecked: 0 })
  })

  it('unsupported never errors', async () => {
    callRequestRpc.mockResolvedValue(fail('unsupported'))
    const { result } = renderHook(() => useTaskReadiness({ taskId: 't' }))
    await waitFor(() => expect(result.current.status).toBe('unsupported'))
    expect(result.current.report).toBeNull()
  })

  it('refetches on readiness.reported', async () => {
    callRequestRpc.mockResolvedValue(ok({ report: rep('t1', 'ready') }))
    renderHook(() => useTaskReadiness({ requestId: 'r1', taskId: 't1' }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    act(() => emitRequestEvent({ requestId: 'r1', eventType: 'readiness.reported', occurredAt: '' }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
  })
})
