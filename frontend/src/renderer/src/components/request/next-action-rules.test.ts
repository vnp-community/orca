import { describe, expect, it } from 'vitest'
import { pickNextAction, type NextActionInput } from './next-action-rules'
import type { ExecutionSummary } from '../../../../shared/request-execution-types'

const TABS = { analysis: true, plan: true, execution: true }

function summary(over: Partial<ExecutionSummary> = {}): ExecutionSummary {
  return {
    requestId: 'r',
    status: 'executing',
    running: [],
    failed: [],
    needsYou: [],
    maxAttempts: 2,
    progress: {
      phasesTotal: 7,
      phasesDone: 6,
      tasksTotal: 28,
      tasksDone: 27,
      tasksFailed: 0,
      tasksRunning: 0
    },
    handover: { state: 'none', archiveAttempts: 0 },
    ...over
  }
}

const pick = (over: Partial<NextActionInput> & Pick<NextActionInput, 'request'>) =>
  pickNextAction({ summary: null, clarificationAnswerable: false, tabs: TABS, ...over })

const failedTask = {
  taskId: 't',
  title: 'TG-1037',
  attempts: 2,
  maxAttempts: 2,
  lastCause: 'timed_out' as const
}

describe('pickNextAction', () => {
  it('request #1242: back in the backlog with a task out of attempts offers to reopen and rerun', () => {
    const a = pick({
      request: { status: 'request_backlog', type: 'change_request' },
      summary: summary({ failed: [failedTask] })
    })
    expect(a.kind).toBe('failed')
    expect(a.title).toContain('TG-1037')
    expect(a.title).toContain('2/2')
    expect(a.primary?.command).toBe('reopen')
    expect(a.tab).toBe('execution')
  })

  it('a failure outranks something waiting for the viewer', () => {
    const a = pick({
      request: { status: 'request_backlog', type: 'change_request' },
      summary: summary({ failed: [failedTask], needsYou: [{ kind: 'approval', refId: 'a' }] })
    })
    expect(a.kind).toBe('failed')
  })

  it('a backlog request without a failed task has nothing to offer', () => {
    expect(
      pick({ request: { status: 'request_backlog', type: 'bug' }, summary: summary() }).kind
    ).toBe('none')
  })

  it('an open question the viewer can answer comes first', () => {
    const a = pick({
      request: { status: 'awaiting_information', type: 'bug' },
      clarificationAnswerable: true
    })
    expect(a).toMatchObject({ kind: 'needs_you', primary: { command: 'answer' } })
  })

  it('a question for someone else is only waiting', () => {
    expect(pick({ request: { status: 'awaiting_information', type: 'bug' } }).kind).toBe('waiting')
  })

  it('a clarification in the summary asks to answer', () => {
    const a = pick({
      request: { status: 'executing', type: 'task' },
      summary: summary({ needsYou: [{ kind: 'clarification', refId: 'c' }] })
    })
    expect(a.primary?.command).toBe('answer')
  })

  it('a plan approval the viewer may decide points at the Plan tab; a hotfix pre-deploy at Execution', () => {
    const needs = summary({ needsYou: [{ kind: 'approval', refId: 'a' }] })
    expect(
      pick({ request: { status: 'awaiting_plan_approval', type: 'bug' }, summary: needs }).tab
    ).toBe('plan')
    const hotfix = pick({
      request: { status: 'awaiting_plan_approval', type: 'hotfix' },
      summary: summary({ needsYou: [{ kind: 'pre_deploy', refId: 'a' }] }),
      tabs: { analysis: true, plan: false, execution: true }
    })
    expect(hotfix).toMatchObject({ kind: 'needs_you', tab: 'execution' })
  })

  it('an analysis approval points at Analysis', () => {
    const a = pick({
      request: { status: 'awaiting_analysis_approval', type: 'bug' },
      summary: summary({ needsYou: [{ kind: 'approval', refId: 'a' }] })
    })
    expect(a.tab).toBe('analysis')
  })

  it('an approval the viewer may not decide is only waiting', () => {
    const a = pick({
      request: { status: 'awaiting_plan_approval', type: 'bug' },
      summary: summary()
    })
    expect(a.kind).toBe('waiting')
  })

  it('a drift review while executing points at Execution', () => {
    const a = pick({
      request: { status: 'executing', type: 'change_request' },
      summary: summary({ needsYou: [{ kind: 'drift', refId: 'a' }] })
    })
    expect(a).toMatchObject({ kind: 'needs_you', tab: 'execution' })
  })

  it('a running task says nothing needs the viewer', () => {
    const a = pick({
      request: { status: 'executing', type: 'change_request' },
      summary: summary({ running: [{ taskId: 't', title: 'TG-1036', attempt: 1, maxAttempts: 2 }] })
    })
    expect(a.kind).toBe('running')
    expect(a.detail).toContain('Nothing needs you')
  })

  it('executing with nothing running is waiting for the next task', () => {
    expect(pick({ request: { status: 'executing', type: 'task' }, summary: summary() }).kind).toBe(
      'waiting'
    )
  })

  it('completed is done, a cancelled request shows nothing', () => {
    expect(pick({ request: { status: 'completed', type: 'bug' } }).kind).toBe('done')
    expect(pick({ request: { status: 'cancelled', type: 'bug' } }).kind).toBe('none')
  })

  it.each(['classifying', 'analyzing', 'planning'] as const)('%s is waiting on Orca', (status) => {
    expect(pick({ request: { status, type: 'bug' } }).kind).toBe('waiting')
  })
})
