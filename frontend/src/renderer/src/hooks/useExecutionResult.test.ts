// @vitest-environment happy-dom
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)
const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string, message = 'm') => ({ ok: false, error: { kind, code: 'x', message } })
import { useExecutionResult } from './useExecutionResult'

describe('useExecutionResult', () => {
  it('requests the latest record and parses it', async () => {
    callRequestRpc.mockResolvedValue(ok({ result: { task_id: 't', parse_status: 'ok', status: 'done' } }))
    const { result } = renderHook(() => useExecutionResult('t'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(callRequestRpc).toHaveBeenCalledWith('execution.get', { taskId: 't', latestOnly: true })
    expect(result.current.result?.status).toBe('done')
  })

  it('distinguishes unsupported, legacy (no record) and error', async () => {
    callRequestRpc.mockResolvedValueOnce(fail('unsupported'))
    const a = renderHook(() => useExecutionResult('t1'))
    await waitFor(() => expect(a.result.current.status).toBe('unsupported'))
    callRequestRpc.mockResolvedValueOnce(ok({}))
    const b = renderHook(() => useExecutionResult('t2'))
    await waitFor(() => expect(b.result.current.status).toBe('legacy'))
    callRequestRpc.mockResolvedValueOnce(fail('network'))
    const c = renderHook(() => useExecutionResult('t3'))
    await waitFor(() => expect(c.result.current.status).toBe('error'))
  })

  it('stays idle without a task id', () => {
    const { result } = renderHook(() => useExecutionResult(null))
    expect(result.current.status).toBe('idle')
    expect(callRequestRpc).not.toHaveBeenCalled()
  })
})
