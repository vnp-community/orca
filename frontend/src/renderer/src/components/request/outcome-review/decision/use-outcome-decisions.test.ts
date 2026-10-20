// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const rpc = vi.hoisted(() => vi.fn())
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))

import { emitRequestEvent } from '../../../../lib/request-event-bus'
import { makeItem, makeReview } from '../outcome-review.fixture'
import { useOutcomeDecisions } from './use-outcome-decisions'

const items = [
  makeItem({ key: 'a', needsDecision: true }),
  makeItem({ key: 'b', needsDecision: true }),
  makeItem({ key: 'c', needsDecision: true, advisory: true })
]
const review = (version = 4) => makeReview({ version, model: { items } })
const dec = (itemKey: string, over: Record<string, unknown> = {}) => ({
  decisionId: `d-${itemKey}`,
  reviewId: 'v1',
  itemKey,
  decision: 'send_back',
  decidedBy: 'u1',
  decidedAt: '',
  active: true,
  orphaned: false,
  ...over
})

let decisions: unknown[] = []
beforeEach(() => {
  rpc.mockReset()
  decisions = []
  rpc.mockImplementation(async (method: string, p: Record<string, unknown>) => {
    switch (method) {
      case 'request.review.decisions':
        return { ok: true, value: { decisions, openDecisions: 0 } }
      case 'request.review.decide':
        return {
          ok: true,
          value: {
            decision: dec(String(p.itemKey), { decision: p.decision }),
            reviewVersion: Number(p.expectedVersion) + 1,
            reviewState: 'ready',
            openDecisions: 1
          }
        }
      case 'request.review.approve':
        return {
          ok: true,
          value: {
            review: { reviewId: 'v1', version: Number(p.expectedVersion) + 1, state: 'approved' }
          }
        }
      case 'request.review.linkChild':
        return { ok: true, value: { linked: 2 } }
      default:
        return { ok: false, error: { kind: 'not_found' } }
    }
  })
})
afterEach(cleanup)

const mount = (r = review(), onChanged = vi.fn(), enabled = true) =>
  renderHook(() => useOutcomeDecisions({ requestId: 'r1', review: r, enabled, onChanged }))

describe('useOutcomeDecisions', () => {
  it('reads nothing until a delivered review is shown', () => {
    mount(review(), vi.fn(), false)
    mount(null as never)
    expect(rpc).not.toHaveBeenCalled()
  })

  it('lists the decisions in force and the items still open; advisory items never ask, orphaned decisions are history', async () => {
    decisions = [dec('a'), dec('b', { active: false }), dec('gone', { orphaned: true })]
    const { result } = mount()
    await waitFor(() => expect(result.current.loaded).toBe(true))
    expect([...result.current.inForce.keys()]).toEqual(['a'])
    expect(result.current.openKeys).toEqual(['b'])
    expect(rpc).toHaveBeenCalledWith('request.review.decisions', { id: 'r1', reviewId: 'v1' })
  })

  it('a decision carries the version of the review it saw and the trimmed note, then asks the screen to read again', async () => {
    const onChanged = vi.fn()
    const { result } = mount(review(4), onChanged)
    await waitFor(() => expect(result.current.loaded).toBe(true))
    let out: Awaited<ReturnType<typeof result.current.decide>> | undefined
    await act(async () => {
      out = await result.current.decide('a', 'accept_exception', '  ADR-7  ')
    })
    expect(out).toEqual({ ok: true })
    expect(rpc).toHaveBeenCalledWith('request.review.decide', {
      id: 'r1',
      reviewId: 'v1',
      itemKey: 'a',
      decision: 'accept_exception',
      note: 'ADR-7',
      expectedVersion: 4
    })
    expect(onChanged).toHaveBeenCalled()
  })

  it('several in a row each carry the version the one before produced, not the one still on screen', async () => {
    const { result } = mount(review(4))
    await waitFor(() => expect(result.current.loaded).toBe(true))
    await act(async () => {
      await result.current.decide('a', 'send_back', '')
      await result.current.decide('b', 'send_back', '')
    })
    const sent = rpc.mock.calls
      .filter((c) => c[0] === 'request.review.decide')
      .map((c) => c[1].expectedVersion)
    expect(sent).toEqual([4, 5])
  })

  it('approving sends the version and the note; linking sends the keys and the child', async () => {
    const { result } = mount(review(7))
    await waitFor(() => expect(result.current.loaded).toBe(true))
    await act(async () => {
      expect(await result.current.approve(' fine ')).toEqual({ ok: true })
      expect(await result.current.linkChild(['a', 'b'], 'child-1')).toEqual({ ok: true })
    })
    expect(rpc).toHaveBeenCalledWith('request.review.approve', {
      id: 'r1',
      reviewId: 'v1',
      note: 'fine',
      expectedVersion: 7
    })
    expect(rpc).toHaveBeenCalledWith('request.review.linkChild', {
      id: 'r1',
      reviewId: 'v1',
      itemKeys: ['a', 'b'],
      childRequestId: 'child-1'
    })
  })

  it('reports the server code of a refusal and changes nothing on screen', async () => {
    rpc.mockImplementation(async (method: string) =>
      method === 'request.review.decisions'
        ? { ok: true, value: { decisions: [], openDecisions: 0 } }
        : {
            ok: false,
            error: { kind: 'conflict', backendCode: 'REQUEST_REVIEW_STALE_REVISION', message: 'm' }
          }
    )
    const onChanged = vi.fn()
    const { result } = mount(review(), onChanged)
    await waitFor(() => expect(result.current.loaded).toBe(true))
    let out: Awaited<ReturnType<typeof result.current.approve>> | undefined
    await act(async () => {
      out = await result.current.approve('')
    })
    expect(out).toEqual({
      ok: false,
      error: { code: 'REQUEST_REVIEW_STALE_REVISION', message: 'm' }
    })
    expect(onChanged).not.toHaveBeenCalled()
    expect(result.current.busy).toBe(false)
  })

  it('an unreadable answer to a write is a failure, not a success', async () => {
    rpc.mockImplementation(async (method: string) =>
      method === 'request.review.decisions'
        ? { ok: true, value: { decisions: [] } }
        : { ok: true, value: {} }
    )
    const { result } = mount()
    await waitFor(() => expect(result.current.loaded).toBe(true))
    let out: Awaited<ReturnType<typeof result.current.decide>> | undefined
    await act(async () => {
      out = await result.current.decide('a', 'send_back', '')
    })
    expect(out?.ok).toBe(false)
  })

  it('reads again on a review event of this Request only', async () => {
    const { result } = mount()
    await waitFor(() => expect(result.current.loaded).toBe(true))
    const before = rpc.mock.calls.length
    act(() => emitRequestEvent({ requestId: 'other', eventType: 'review.changed', occurredAt: '' }))
    act(() =>
      emitRequestEvent({ requestId: 'r1', eventType: 'request.status_changed', occurredAt: '' })
    )
    expect(rpc.mock.calls.length).toBe(before)
    decisions = [dec('a')]
    act(() => emitRequestEvent({ requestId: 'r1', eventType: 'review.changed', occurredAt: '' }))
    await waitFor(() => expect(result.current.inForce.has('a')).toBe(true))
  })
})
