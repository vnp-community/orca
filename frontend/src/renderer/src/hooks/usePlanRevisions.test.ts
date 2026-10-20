// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { emitRequestEvent } from '../lib/request-event-bus'
import { REQUEST_POLL_EVENT_TYPE } from './useRequestEvents'
import { PLAN_REVISIONS_PAGE_SIZE, usePlanRevisions } from './usePlanRevisions'

const ok = (value: unknown) => ({ ok: true as const, value })
const rev = (n: number, over: Record<string, unknown> = {}) => ({
  revision: n,
  runId: `run-${n}`,
  status: 'superseded',
  generatedAt: '2026-10-20T10:00:00Z',
  ...over
})
type Req = { id: string; status: string; version?: number; planTaskId?: string }
const approving: Req = { id: 'r1', status: 'awaiting_plan_approval', version: 3, planTaskId: 'p1' }

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestFlowFeatures: new Set(['request.planRevisions']) })
})
afterEach(cleanup)

function setup(initial: Req = approving) {
  return renderHook((props: Req) => usePlanRevisions(props as never), { initialProps: initial })
}

describe('usePlanRevisions', () => {
  it('reads the first page of 20 and exposes the current version', async () => {
    callRequestRpc.mockResolvedValue(
      ok({ revisions: [rev(2, { status: 'current' }), rev(1)], nextPageToken: '' })
    )
    const { result } = setup()
    await waitFor(() => expect(result.current.revisions).toHaveLength(2))
    expect(callRequestRpc).toHaveBeenCalledWith('request.planRevisions', {
      id: 'r1',
      pageSize: PLAN_REVISIONS_PAGE_SIZE
    })
    expect(PLAN_REVISIONS_PAGE_SIZE).toBe(20)
    expect(result.current.current?.revision).toBe(2)
    expect(result.current.hasMore).toBe(false)
    expect(result.current.supported).toBe(true)
  })

  it('loads the next page with the token and merges it', async () => {
    callRequestRpc
      .mockResolvedValueOnce(
        ok({ revisions: [rev(3, { status: 'current' }), rev(2)], nextPageToken: 'tok' })
      )
      .mockResolvedValueOnce(ok({ revisions: [rev(1)], nextPageToken: '' }))
    const { result } = setup()
    await waitFor(() => expect(result.current.hasMore).toBe(true))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.revisions.map((r) => r.revision)).toEqual([3, 2, 1]))
    expect(callRequestRpc).toHaveBeenLastCalledWith('request.planRevisions', {
      id: 'r1',
      pageSize: 20,
      pageToken: 'tok'
    })
    expect(result.current.hasMore).toBe(false)
  })

  it('reads again when the Request moves on and keeps the older pages already loaded', async () => {
    callRequestRpc
      .mockResolvedValueOnce(
        ok({ revisions: [rev(3, { status: 'current' }), rev(2)], nextPageToken: 'tok' })
      )
      .mockResolvedValueOnce(ok({ revisions: [rev(1)], nextPageToken: '' }))
    const { result, rerender } = setup()
    await waitFor(() => expect(result.current.hasMore).toBe(true))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.revisions).toHaveLength(3))
    callRequestRpc.mockResolvedValue(
      ok({ revisions: [rev(4, { status: 'current' }), rev(3)], nextPageToken: 'x' })
    )
    rerender({ ...approving, version: 4 })
    await waitFor(() =>
      expect(result.current.revisions.map((r) => r.revision)).toEqual([4, 3, 2, 1])
    )
    expect(result.current.current?.revision).toBe(4)
    expect(result.current.hasMore).toBe(false)
  })

  it('reads again on a plan event and on the poll frame while a change is written, not on others', async () => {
    callRequestRpc.mockResolvedValue(ok({ revisions: [rev(1, { status: 'current' })] }))
    const { rerender } = setup()
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: 'now'
      })
    )
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.comment.added',
        occurredAt: 'now'
      })
    )
    act(() =>
      emitRequestEvent({ requestId: '', eventType: REQUEST_POLL_EVENT_TYPE, occurredAt: 'now' })
    )
    await act(async () => undefined)
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    rerender({ ...approving, status: 'planning' })
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(3))
    act(() =>
      emitRequestEvent({ requestId: '', eventType: REQUEST_POLL_EVENT_TYPE, occurredAt: 'now' })
    )
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(4))
  })

  it('merges a burst of triggers into one read in flight and at most one more', async () => {
    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementation(() => new Promise((resolve) => (release = resolve)))
    const { rerender } = setup()
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    rerender({ ...approving, version: 4 })
    rerender({ ...approving, version: 5 })
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: 'now'
      })
    )
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    callRequestRpc.mockImplementation(async () => ok({ revisions: [rev(1)] }))
    await act(async () => release(ok({ revisions: [rev(1)] })))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    await act(async () => undefined)
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('is unsupported without the gateway flag and never calls the channel', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    const { result } = setup()
    await act(async () => undefined)
    expect(callRequestRpc).not.toHaveBeenCalled()
    expect(result.current.supported).toBe(false)
    expect(result.current.revisions).toEqual([])
    expect(result.current.error).toBeNull()
  })

  it('is unsupported, not an error, when the gateway answers that the channel does not exist', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'unsupported', code: 'METHOD_NOT_FOUND', message: 'x' }
    })
    const { result } = setup()
    await waitFor(() => expect(result.current.supported).toBe(false))
    expect(result.current.error).toBeNull()
  })

  it('does not call before the Request has a Plan', async () => {
    const { result } = setup({ id: 'r1', status: 'planning' })
    await act(async () => undefined)
    expect(callRequestRpc).not.toHaveBeenCalled()
    expect(result.current.revisions).toEqual([])
  })

  it('keeps the list it has when a later read fails on the network', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ revisions: [rev(2, { status: 'current' }), rev(1)] }))
    const { result, rerender } = setup()
    await waitFor(() => expect(result.current.revisions).toHaveLength(2))
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'network', code: 'NETWORK_ERROR', message: 'offline' }
    })
    rerender({ ...approving, version: 9 })
    await waitFor(() => expect(result.current.error).toBe('network'))
    expect(result.current.revisions).toHaveLength(2)
    expect(result.current.supported).toBe(true)
  })

  it('shows nothing of another Request while the new one loads', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ revisions: [rev(2, { status: 'current' }), rev(1)] }))
    const { result, rerender } = setup()
    await waitFor(() => expect(result.current.revisions).toHaveLength(2))
    callRequestRpc.mockImplementation(() => new Promise(() => undefined))
    rerender({ ...approving, id: 'r2' })
    expect(result.current.revisions).toEqual([])
    expect(result.current.current).toBeNull()
  })
})
