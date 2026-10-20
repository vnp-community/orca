// @vitest-environment happy-dom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import {
  PLAN_RUN_OPENSPEC_TIMEOUT_MS,
  PLAN_RUN_TIMEOUT_MS
} from '../../../../hooks/request-plan-generation'
import { emitRequestEvent } from '../../../../lib/request-event-bus'
import type { PlanAutoDisplay } from '../plan-auto-display'
import { usePlanRevision, type UsePlanRevisionParams } from './usePlanRevision'

const ok = (value: unknown = {}) => ({ ok: true as const, value })
const fail = (kind: string, message: string) => ({
  ok: false as const,
  error: { kind, code: 'runtime_error', message }
})

type Req = UsePlanRevisionParams['request']
const atGate: Req = {
  id: 'r1',
  status: 'awaiting_plan_approval',
  planTaskId: 'plan1',
  engine: 'native',
  version: 7,
  updatedAt: '2026-10-20T10:00:00Z'
}
const NONE: PlanAutoDisplay = { kind: 'none' }
const WRITING: PlanAutoDisplay = {
  kind: 'writing',
  attempt: 1,
  maxAttempts: 1,
  waiting: false,
  replan: true,
  starting: false
}

type Props = {
  request: Req
  display?: PlanAutoDisplay
  busy?: boolean
  runId?: string
  kind?: UsePlanRevisionParams['flowPlanKind']
  serverCommit?: boolean
  viewingCurrent?: boolean
}

function setup(initial: Props = { request: atGate }) {
  const onSettled = vi.fn()
  const refresh = vi.fn()
  const hook = renderHook(
    (p: Props) =>
      usePlanRevision({
        request: p.request,
        flowPlanKind: p.kind ?? 'plan',
        serverCommit: p.serverCommit ?? true,
        runState: {
          display: p.display ?? NONE,
          busy: p.busy ?? false,
          run: p.runId === undefined ? null : ({ runId: p.runId } as never),
          refresh
        },
        currentRevision: 2,
        viewingCurrent: p.viewingCurrent ?? true,
        onSettled
      }),
    { initialProps: initial }
  )
  return { onSettled, refresh, ...hook }
}

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue(ok({ runId: 'run-new' }))
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usePlanRevision submit', () => {
  it('asks the server to write the next version once, with the trimmed text and a key, and never rejects', async () => {
    const { result, onSettled, refresh } = setup()
    let outcome: Awaited<ReturnType<typeof result.current.submit>> | undefined
    await act(async () => {
      outcome = await result.current.submit('  Split the migration  ')
    })
    expect(outcome).toEqual({ ok: true })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).toHaveBeenCalledWith(
      'request.generatePlan',
      {
        id: 'r1',
        mode: 'propose',
        feedback: 'Split the migration',
        idempotencyKey: expect.stringMatching(/^[0-9a-f-]{36}$/)
      },
      { idempotent: true }
    )
    expect(callRequestRpc.mock.calls.some(([m]) => String(m).startsWith('approval.'))).toBe(false)
    expect(onSettled).toHaveBeenCalled()
    expect(refresh).toHaveBeenCalled()
    expect(result.current.nextVersion).toBe(3)
  })

  it('without a server commit it saves the revised plan from the tab too', async () => {
    callRequestRpc.mockImplementation(async (_m: string, p: { mode?: string } = {}) =>
      p.mode === 'commit'
        ? ok({ planTaskId: 'plan2' })
        : ok({ runId: 'run-new', proposal: { title: 'T' } })
    )
    const { result } = setup({ request: atGate, serverCommit: false })
    await act(async () => {
      await result.current.submit('change it')
    })
    expect(callRequestRpc.mock.calls.map(([, p]) => (p as { mode?: string }).mode)).toEqual([
      'propose',
      'commit'
    ])
  })

  it('keeps the text and reports an empty or too long request without calling', async () => {
    const { result } = setup()
    let outcome: Awaited<ReturnType<typeof result.current.submit>> | undefined
    await act(async () => {
      outcome = await result.current.submit('   ')
    })
    expect(outcome).toMatchObject({ ok: false, error: { kind: 'empty' } })
    await act(async () => {
      outcome = await result.current.submit('😀'.repeat(2001))
    })
    expect(outcome).toMatchObject({ ok: false, error: { kind: 'tooLong' } })
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('counts the 2000 characters by code point, not by UTF-16 unit', async () => {
    const { result } = setup()
    await act(async () => {
      await result.current.submit('😀'.repeat(2000))
    })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('does not let a second click start a second turn', async () => {
    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementation(() => new Promise((resolve) => (release = resolve)))
    const { result } = setup()
    let first: Promise<unknown> = Promise.resolve()
    act(() => {
      first = result.current.submit('one')
    })
    await act(async () => {
      await result.current.submit('two')
    })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    await act(async () => {
      release(ok({ runId: 'x' }))
      await first
    })
  })

  it.each([
    ['REQUEST_PLAN_FORBIDDEN: no', 'forbidden', false],
    ['REQUEST_PLAN_REVISION_IN_PROGRESS: busy', 'wrongState', true],
    ['REQUEST_STATE_STALE: stale', 'wrongState', true],
    ['REQUEST_PLAN_AI_UNAVAILABLE: none', 'noConnection', false],
    ['REQUEST_PLAN_REVISION_NOT_APPLICABLE: hotfix', 'notApplicable', false]
  ])('keeps the typed text on %s', async (message, kind, refreshes) => {
    callRequestRpc.mockResolvedValue(fail('unknown', message))
    const { result, onSettled } = setup()
    act(() => result.current.setDraft('my request'))
    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.error?.kind).toBe(kind)
    expect(result.current.draft).toBe('my request')
    expect(result.current.busy).toBe(false)
    expect(onSettled).toHaveBeenCalledTimes(refreshes ? 1 : 0)
  })

  it('clears the text only once the server accepted the request', async () => {
    const { result } = setup()
    act(() => result.current.setDraft('my request'))
    expect(result.current.draft).toBe('my request')
    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.draft).toBe('')
  })

  it('keeps one draft per Request', () => {
    const { result, rerender } = setup()
    act(() => result.current.setDraft('mine'))
    rerender({ request: { ...atGate, id: 'r2' } })
    expect(result.current.draft).toBe('')
    rerender({ request: atGate })
    expect(result.current.draft).toBe('mine')
  })
})

describe('usePlanRevision visibility', () => {
  it('shows at the gate of a plan or task list, not for a hotfix, another status or an older version', () => {
    expect(setup().result.current.canShow).toBe(true)
    expect(setup({ request: atGate, kind: 'task_list' }).result.current.canShow).toBe(true)
    expect(setup({ request: atGate, kind: 'single_task' }).result.current.canShow).toBe(false)
    expect(setup({ request: atGate, kind: 'none' }).result.current.canShow).toBe(false)
    expect(setup({ request: { ...atGate, status: 'executing' } }).result.current.canShow).toBe(
      false
    )
    expect(setup({ request: { ...atGate, planTaskId: undefined } }).result.current.canShow).toBe(
      false
    )
    const older = setup({ request: atGate, viewingCurrent: false }).result.current
    expect(older.canShow).toBe(false)
    expect(older.visible).toBe(false)
  })

  it('takes a new request after a failed round, from planning with the plan still there', () => {
    const failed: PlanAutoDisplay = { kind: 'failed', code: 'X', viaServer: true }
    const { result } = setup({ request: { ...atGate, status: 'planning' }, display: failed })
    expect(result.current.canShow).toBe(true)
    expect(result.current.busy).toBe(false)
  })
})

describe('usePlanRevision busy', () => {
  it('follows the server: a rewrite that someone else started locks this tab too', () => {
    const { result } = setup({
      request: { ...atGate, status: 'planning' },
      display: WRITING,
      busy: true
    })
    expect(result.current.busy).toBe(true)
    expect(result.current.visible).toBe(true)
    expect(result.current.nextVersion).toBe(3)
  })

  it('stays busy from the accepted request until the Request has moved on', async () => {
    const { result, rerender } = setup()
    await act(async () => {
      await result.current.submit('change it')
    })
    // Back at the gate with nothing changed: the round has not even started on screen.
    expect(result.current.busy).toBe(true)
    rerender({ request: { ...atGate, status: 'planning' } })
    expect(result.current.busy).toBe(true)
    rerender({ request: { ...atGate, status: 'planning' }, display: WRITING, busy: true })
    expect(result.current.busy).toBe(true)
    rerender({ request: { ...atGate, version: 8, planTaskId: 'plan2' } })
    expect(result.current.busy).toBe(false)
  })

  it('stops being busy when the round fails, but not on the failure of the round before', async () => {
    const failed: PlanAutoDisplay = { kind: 'failed', code: 'X', viaServer: true }
    const planning = { ...atGate, status: 'planning' as const }
    const { result, rerender } = setup({ request: planning, display: failed, runId: 'old' })
    await act(async () => {
      await result.current.submit('try this instead')
    })
    // The old failure is still what the server reports: the new run is not visible yet.
    rerender({ request: planning, display: failed, runId: 'old' })
    expect(result.current.busy).toBe(true)
    rerender({ request: planning, display: failed, runId: 'new' })
    expect(result.current.busy).toBe(false)
  })

  it('asks the Request to refresh on plan events while busy, and not otherwise', async () => {
    const { result, onSettled } = setup()
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: 'now'
      })
    )
    expect(onSettled).not.toHaveBeenCalled()
    await act(async () => {
      await result.current.submit('change it')
    })
    onSettled.mockClear()
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: 'now'
      })
    )
    expect(onSettled).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['native', PLAN_RUN_TIMEOUT_MS],
    ['openspec', PLAN_RUN_OPENSPEC_TIMEOUT_MS]
  ] as const)(
    'reports a %s round that outlasts its budget without failing it',
    async (engine, budget) => {
      vi.useFakeTimers()
      const { result } = setup({
        request: { ...atGate, status: 'planning', engine },
        display: WRITING,
        busy: true
      })
      expect(result.current.slow).toBe(false)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(budget - 1000)
      })
      expect(result.current.slow).toBe(false)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1500)
      })
      expect(result.current.slow).toBe(true)
      expect(result.current.busy).toBe(true)
    }
  )
})

describe('usePlanRevision retry', () => {
  it('asks again without text and without the retry endpoint of the first plan', async () => {
    const { result, onSettled } = setup({
      request: { ...atGate, status: 'planning' },
      display: { kind: 'failed', code: 'X', viaServer: true }
    })
    await act(async () => {
      await result.current.retry()
    })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', {
      id: 'r1',
      mode: 'propose'
    })
    expect(onSettled).toHaveBeenCalled()
    expect(result.current.retryError).toBeNull()
  })

  it('without a server commit it saves the plan from the tab', async () => {
    callRequestRpc.mockImplementation(async (_m: string, p: { mode?: string } = {}) =>
      p.mode === 'commit' ? ok({ planTaskId: 'p2' }) : ok({ runId: 'r', proposal: { title: 'T' } })
    )
    const { result } = setup({ request: { ...atGate, status: 'planning' }, serverCommit: false })
    await act(async () => {
      await result.current.retry()
    })
    expect(callRequestRpc.mock.calls.map(([m]) => m)).toEqual([
      'request.generatePlan',
      'request.generatePlan'
    ])
  })

  it('reports why it could not start and is not busy afterwards', async () => {
    callRequestRpc.mockResolvedValue(fail('unavailable', 'REQUEST_PLAN_AI_UNAVAILABLE: none'))
    const { result } = setup({ request: { ...atGate, status: 'planning' } })
    await act(async () => {
      await result.current.retry()
    })
    expect(result.current.retryError).toMatch(/No dev server is connected/)
    expect(result.current.busy).toBe(false)
  })
})
