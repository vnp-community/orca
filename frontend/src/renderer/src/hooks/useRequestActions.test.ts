// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useRequestActions } from './useRequestActions'

beforeEach(() => {
  callRequestRpc.mockReset()
})

describe('useRequestActions', () => {
  it('rejects empty reasons for changeType and returnToBacklog without an RPC call', async () => {
    const { result } = renderHook(() => useRequestActions())
    const a = await result.current.changeType({ id: 'r', toType: 'bug', reason: '  ' })
    const b = await result.current.returnToBacklog({ id: 'r', stage: 'plan', reason: '' })
    expect(a.ok).toBe(false)
    expect(b.ok).toBe(false)
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('blocks a double submit for the same action and id until it settles', async () => {
    let release: (v: unknown) => void = () => {}
    callRequestRpc.mockReturnValue(new Promise((r) => (release = r)))
    const { result } = renderHook(() => useRequestActions())
    const first = result.current.cancel({ id: 'r', reason: 'x' })
    const second = await result.current.cancel({ id: 'r', reason: 'x' })
    expect(second.ok).toBe(false)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    release({ ok: true, value: {} })
    await first
    callRequestRpc.mockResolvedValue({ ok: true, value: {} })
    await result.current.cancel({ id: 'r', reason: 'x' })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })
  it('generatePlan proposes then commits with the returned proposal', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: { proposal: { phases: [] }, rawAiResponse: 'raw' } })
      .mockResolvedValueOnce({ ok: true, value: { planTaskId: 'pl' } })
    const { result } = renderHook(() => useRequestActions())
    const res = await result.current.generatePlan('r')
    expect(res.ok).toBe(true)
    expect(callRequestRpc).toHaveBeenNthCalledWith(1, 'request.generatePlan', { id: 'r', mode: 'propose' })
    expect(callRequestRpc).toHaveBeenNthCalledWith(2, 'request.generatePlan', {
      id: 'r', mode: 'commit', proposal: { phases: [] }, rawAiResponse: 'raw'
    })
  })

  it('generatePlan stops after propose when the plan already exists or propose fails', async () => {
    callRequestRpc.mockResolvedValueOnce({ ok: true, value: { alreadyExists: true, proposal: {} } })
    const { result } = renderHook(() => useRequestActions())
    await result.current.generatePlan('r')
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'unknown', code: 'X', message: 'm' } })
    const res = await result.current.generatePlan('r2')
    expect(res.ok).toBe(false)
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('startPhase sends phaseTaskId per contract', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: {} })
    const { result } = renderHook(() => useRequestActions())
    await result.current.startPhase({ id: 'r', phaseTaskId: 'ph1' })
    expect(callRequestRpc).toHaveBeenCalledWith('request.startPhase', { id: 'r', phaseTaskId: 'ph1' })
  })
})
