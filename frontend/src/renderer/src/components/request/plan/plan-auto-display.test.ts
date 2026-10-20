import { describe, expect, it } from 'vitest'
import {
  derivePlanAutoDisplay,
  isPlanRunBusy,
  type PlanAutoDisplayInput
} from './plan-auto-display'
import type {
  PlanAutoState,
  PlanAutoView,
  PlanRunStatus,
  PlanRunView
} from '../../../../../shared/request-plan-auto'

const auto = (o: Partial<PlanAutoView> = {}): PlanAutoView => ({
  enabled: true,
  state: '',
  origin: 'event',
  attempt: 0,
  maxAttempts: 0,
  reasonCode: '',
  ...o
})

const run = (status: PlanRunStatus, autoPlan: PlanAutoView | null, o: Partial<PlanRunView> = {}) =>
  ({
    runId: status ? 'run1' : '',
    status,
    errorCode: status === 'failed' ? 'REQUEST_PLAN_AI_INVALID_JSON' : '',
    errorMessage: '',
    engine: 'native',
    autoPlan,
    ...o
  }) as PlanRunView

const derive = (o: Partial<PlanAutoDisplayInput> & { status?: string; planTaskId?: string } = {}) =>
  derivePlanAutoDisplay({
    request: { status: (o.status ?? 'planning') as 'planning', planTaskId: o.planTaskId },
    run: o.run === undefined ? null : o.run,
    supportsPlanAuto: o.supportsPlanAuto ?? true,
    graceElapsed: o.graceElapsed ?? false
  })

describe('derivePlanAutoDisplay: Request status', () => {
  it.each(['awaiting_plan_approval', 'analyzing', 'executing', 'awaiting_analysis_approval'])(
    'says nothing while the Request is %s, whatever the run says',
    (status) => {
      expect(
        derive({ status, run: run('running', auto({ state: 'running', attempt: 1 })) })
      ).toEqual({
        kind: 'none'
      })
      expect(derive({ status, run: run('failed', auto({ state: 'gave_up' })) })).toEqual({
        kind: 'none'
      })
    }
  )
})

describe('derivePlanAutoDisplay: writing', () => {
  it.each<[string, PlanRunView]>([
    ['a running run, no block (older gateway)', run('running', null)],
    [
      'a running run with a running job',
      run('running', auto({ state: 'running', attempt: 1, maxAttempts: 2 }))
    ],
    [
      'a pending first attempt, no run yet',
      run('', auto({ state: 'pending', attempt: 1, maxAttempts: 2 }))
    ],
    [
      'a committing job after a succeeded run',
      run('succeeded', auto({ state: 'committing', attempt: 1, maxAttempts: 2 }))
    ]
  ])('shows writing for %s', (_label, view) => {
    expect(derive({ run: view })).toMatchObject({
      kind: 'writing',
      waiting: false,
      replan: false,
      starting: false
    })
  })

  it('reports the attempt and the total', () => {
    expect(
      derive({ run: run('running', auto({ state: 'running', attempt: 2, maxAttempts: 2 })) })
    ).toEqual({
      kind: 'writing',
      attempt: 2,
      maxAttempts: 2,
      waiting: false,
      replan: false,
      starting: false
    })
  })

  it('waits between attempts: pending from the second attempt on, after a failed run', () => {
    expect(
      derive({ run: run('failed', auto({ state: 'pending', attempt: 2, maxAttempts: 2 })) })
    ).toMatchObject({ kind: 'writing', attempt: 2, maxAttempts: 2, waiting: true })
  })

  it('is not waiting while the first attempt is merely queued', () => {
    expect(
      derive({ run: run('', auto({ state: 'pending', attempt: 1, maxAttempts: 2 })) })
    ).toMatchObject({ kind: 'writing', attempt: 1, waiting: false })
  })

  it('counts a missing attempt as the first one', () => {
    expect(derive({ run: run('running', null) })).toMatchObject({
      attempt: 1,
      maxAttempts: 0
    })
  })

  it('marks a rewrite when the Request already has a Plan', () => {
    expect(
      derive({ planTaskId: 'plan1', run: run('running', auto({ state: 'running', attempt: 1 })) })
    ).toMatchObject({
      kind: 'writing',
      replan: true
    })
  })

  it('shows a running manual run even when the gateway has no automatic mode', () => {
    expect(derive({ supportsPlanAuto: false, run: run('running', null) })).toMatchObject({
      kind: 'writing'
    })
  })
})

describe('derivePlanAutoDisplay: starting', () => {
  const idle = run('', auto())

  it('shows "starting" when automatic mode is on and nothing exists yet', () => {
    expect(derive({ run: idle })).toEqual({
      kind: 'writing',
      attempt: 1,
      maxAttempts: 0,
      waiting: false,
      replan: false,
      starting: true
    })
  })

  it('gives the manual button back once the grace period ran out', () => {
    expect(derive({ run: idle, graceElapsed: true })).toEqual({ kind: 'none' })
  })

  it('does not wait when the automatic mode is off', () => {
    expect(derive({ run: run('', auto({ enabled: false })) })).toEqual({ kind: 'none' })
  })

  it('does not wait on a gateway without the flag, or one that sent no block', () => {
    expect(derive({ run: idle, supportsPlanAuto: false })).toEqual({ kind: 'none' })
    expect(derive({ run: run('', null) })).toEqual({ kind: 'none' })
    expect(derive({ run: null })).toEqual({ kind: 'none' })
  })

  it('does not wait once a run exists', () => {
    expect(derive({ run: run('succeeded', auto()) })).toEqual({ kind: 'none' })
  })

  it('does not call a finished job "starting"', () => {
    expect(derive({ run: run('', auto({ state: 'done' })) })).toEqual({ kind: 'none' })
    expect(derive({ run: run('', auto({ state: 'superseded' })) })).toEqual({ kind: 'none' })
  })
})

describe('derivePlanAutoDisplay: failed', () => {
  it('shows a gave-up job with its reason and says it went through the server', () => {
    expect(
      derive({
        run: run('failed', auto({ state: 'gave_up', reasonCode: 'REQUEST_PLAN_AI_INVALID_JSON' }))
      })
    ).toEqual({ kind: 'failed', code: 'REQUEST_PLAN_AI_INVALID_JSON', viaServer: true })
  })

  it('shows a gave-up job that never got a run (the busy window ran out)', () => {
    expect(
      derive({ run: run('', auto({ state: 'gave_up', reasonCode: 'REQUEST_ANALYSIS_BUSY' })) })
    ).toEqual({ kind: 'failed', code: 'REQUEST_ANALYSIS_BUSY', viaServer: true })
  })

  it('prefers the reason of the job over the error of the run', () => {
    expect(
      derive({
        run: run('failed', auto({ state: 'gave_up', reasonCode: 'REQUEST_PLAN_COMMIT_FAILED' }), {
          errorCode: 'REQUEST_AI_COMPLETE_TIMEOUT'
        })
      })
    ).toMatchObject({ code: 'REQUEST_PLAN_COMMIT_FAILED' })
  })

  it('shows a failed run of an older gateway (no block) as failed, not via the server', () => {
    expect(derive({ supportsPlanAuto: false, run: run('failed', null) })).toEqual({
      kind: 'failed',
      code: 'REQUEST_PLAN_AI_INVALID_JSON',
      viaServer: false
    })
    expect(derive({ run: run('failed', null) })).toMatchObject({ kind: 'failed', viaServer: false })
  })

  it('shows a failed manual run when the automatic mode is off', () => {
    expect(derive({ run: run('failed', auto({ enabled: false })) })).toMatchObject({
      kind: 'failed',
      viaServer: false
    })
  })

  it('falls back to a generic code when the run failed without one', () => {
    expect(derive({ run: run('failed', null, { errorCode: '' }) })).toMatchObject({
      code: 'REQUEST_INTERNAL'
    })
  })

  it('keeps showing the failure of a rewrite, whose Request already has a Plan', () => {
    expect(
      derive({
        planTaskId: 'plan1',
        run: run('failed', auto({ state: 'gave_up', reasonCode: 'X' }))
      })
    ).toMatchObject({ kind: 'failed' })
  })

  it('does not call a run failed while a retry job is already waiting', () => {
    expect(
      derive({ run: run('failed', auto({ state: 'pending', attempt: 2, maxAttempts: 2 })) })
    ).toMatchObject({ kind: 'writing' })
  })
})

describe('derivePlanAutoDisplay: skipped and the rest', () => {
  it('shows a skipped job with its reason', () => {
    expect(
      derive({
        run: run('', auto({ state: 'skipped', reasonCode: 'REQUEST_PLAN_AI_UNAVAILABLE' }))
      })
    ).toEqual({ kind: 'skipped', reasonCode: 'REQUEST_PLAN_AI_UNAVAILABLE' })
  })

  it.each<[string, PlanRunView | null]>([
    ['no run at all', null],
    ['a succeeded run', run('succeeded', null)],
    ['a done job', run('succeeded', auto({ state: 'done' }))],
    ['a superseded job', run('succeeded', auto({ state: 'superseded' }))]
  ])('says nothing for %s', (_label, view) => {
    expect(derive({ run: view })).toEqual({ kind: 'none' })
  })
})

describe('derivePlanAutoDisplay: full table', () => {
  const statuses = ['planning', 'awaiting_plan_approval', 'analyzing']
  const runs: (PlanRunStatus | null)[] = [null, 'running', 'succeeded', 'failed']
  const states: (PlanAutoState | null)[] = [
    null,
    'pending',
    'running',
    'committing',
    'gave_up',
    'skipped',
    'done'
  ]

  it('is total, never throws, and only speaks while planning', () => {
    for (const status of statuses) {
      for (const runStatus of runs) {
        for (const state of states) {
          for (const planTaskId of [undefined, 'plan1']) {
            for (const graceElapsed of [false, true]) {
              const view =
                runStatus === null && state === null
                  ? null
                  : run(
                      runStatus ?? '',
                      state === null ? null : auto({ state, attempt: 1, maxAttempts: 2 })
                    )
              const out = derive({ status, planTaskId, run: view, graceElapsed })
              if (status !== 'planning') {
                expect(out).toEqual({ kind: 'none' })
              }
              expect(['none', 'writing', 'failed', 'skipped']).toContain(out.kind)
              if (out.kind === 'writing') {
                expect(out.replan).toBe(!!planTaskId)
              }
            }
          }
        }
      }
    }
  })
})

describe('isPlanRunBusy', () => {
  it('is busy for a running run and for pending, running and committing jobs only', () => {
    expect(isPlanRunBusy(null)).toBe(false)
    expect(isPlanRunBusy(run('running', null))).toBe(true)
    for (const state of ['pending', 'running', 'committing'] as const) {
      expect(isPlanRunBusy(run('', auto({ state })))).toBe(true)
    }
    for (const state of ['', 'done', 'gave_up', 'skipped', 'superseded'] as const) {
      expect(isPlanRunBusy(run('failed', auto({ state })))).toBe(false)
    }
  })
})
