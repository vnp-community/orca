// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useApprovals } from './useApprovals'
import type { Approval } from '../../../shared/request-types'

const approval = { id: 'ap1', requestId: 'r1', version: 3, subjectDigest: 'dg' } as Approval

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: { approvals: [{ id: 'ap1', status: 'pending' }] } })
})

describe('useApprovals', () => {
  it('uses listPending without requestId and list with it', async () => {
    renderHook(() => useApprovals())
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(callRequestRpc.mock.calls[0][0]).toBe('approval.listPending')
    renderHook(() => useApprovals({ requestId: 'r1' }))
    await waitFor(() => expect(callRequestRpc.mock.calls.some((c) => c[0] === 'approval.list')).toBe(true))
  })

  it('approve sends expectedVersion and expectedDigest', async () => {
    const { result } = renderHook(() => useApprovals())
    await waitFor(() => expect(result.current.approvals).toHaveLength(1))
    await result.current.approve({ approval })
    const call = callRequestRpc.mock.calls.find((c) => c[0] === 'approval.approve')!
    expect(call[1]).toMatchObject({ approvalId: 'ap1', expectedVersion: 3, expectedDigest: 'dg' })
  })

  it('reject with a short comment returns validation without an RPC call', async () => {
    const { result } = renderHook(() => useApprovals())
    await waitFor(() => expect(result.current.approvals).toHaveLength(1))
    const res = await result.current.reject({ approval, comment: '  short ' })
    expect(res.ok).toBe(false)
    expect(callRequestRpc.mock.calls.some((c) => c[0] === 'approval.reject')).toBe(false)
  })
})
