// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../../../../store'
import { usePlanVersions } from './usePlanVersions'

const ok = (value: unknown) => ({ ok: true as const, value })
const rev = (n: number, status = 'superseded') => ({ revision: n, runId: `run-${n}`, status })
type Req = { id: string; status: string; version: number; planTaskId: string }
const at = (version: number, id = 'r1'): Req => ({
  id,
  status: 'awaiting_plan_approval',
  version,
  planTaskId: 'p1'
})

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestFlowFeatures: new Set(['request.planRevisions']) })
})
afterEach(cleanup)

async function setup(initial: Req = at(1)) {
  callRequestRpc.mockResolvedValue(ok({ revisions: [rev(3, 'current'), rev(2), rev(1)] }))
  const hook = renderHook((r: Req) => usePlanVersions(r as never), { initialProps: initial })
  await waitFor(() => expect(hook.result.current.list.current?.revision).toBe(3))
  return hook
}

describe('usePlanVersions', () => {
  it('shows the current version until an older one is chosen', async () => {
    const { result } = await setup()
    expect(result.current.shown).toBeNull()
    expect(result.current.viewingOlder).toBe(false)
    act(() => result.current.select(2))
    expect(result.current.shown?.revision).toBe(2)
    expect(result.current.viewingOlder).toBe(true)
    expect(result.current.newerAvailable).toBe(false)
  })

  it('goes back to the current version, and choosing it is the same', async () => {
    const { result } = await setup()
    act(() => result.current.select(1))
    act(() => result.current.backToCurrent())
    expect(result.current.viewingOlder).toBe(false)
    act(() => result.current.select(1))
    act(() => result.current.select(3))
    expect(result.current.viewingOlder).toBe(false)
  })

  it('keeps the older version on screen when a newer one appears, and says so', async () => {
    const { result, rerender } = await setup()
    act(() => result.current.select(1))
    callRequestRpc.mockResolvedValue(ok({ revisions: [rev(4, 'current'), rev(3), rev(2), rev(1)] }))
    rerender(at(2))
    await waitFor(() => expect(result.current.list.current?.revision).toBe(4))
    expect(result.current.shown?.revision).toBe(1)
    expect(result.current.newerAvailable).toBe(true)
    act(() => result.current.backToCurrent())
    expect(result.current.newerAvailable).toBe(false)
    expect(result.current.shown).toBeNull()
  })

  it('follows the current version as it moves on', async () => {
    const { result, rerender } = await setup()
    callRequestRpc.mockResolvedValue(ok({ revisions: [rev(4, 'current'), rev(3), rev(2), rev(1)] }))
    rerender(at(2))
    await waitFor(() => expect(result.current.list.current?.revision).toBe(4))
    expect(result.current.shown).toBeNull()
  })

  it('resets the choice for another Request', async () => {
    const { result, rerender } = await setup()
    act(() => result.current.select(1))
    callRequestRpc.mockResolvedValue(ok({ revisions: [rev(2, 'current'), rev(1)] }))
    rerender(at(1, 'r2'))
    await waitFor(() => expect(result.current.list.current?.revision).toBe(2))
    expect(result.current.viewingOlder).toBe(false)
  })
})
