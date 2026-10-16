import { describe, expect, it } from 'vitest'
import { deriveTaskDisplayState } from './execution-task-state'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { OrcaTask } from '../../../../../shared/task-types'

const task = (over: Partial<OrcaTask> = {}): OrcaTask =>
  ({ id: 't1', title: 'Task one', status: 'todo', ...over }) as OrcaTask

function summary(over: Partial<ExecutionSummary> = {}): ExecutionSummary {
  return {
    requestId: 'r',
    status: 'executing',
    running: [],
    failed: [],
    needsYou: [],
    maxAttempts: 2,
    progress: {
      phasesTotal: 1,
      phasesDone: 0,
      tasksTotal: 2,
      tasksDone: 0,
      tasksFailed: 0,
      tasksRunning: 0
    },
    handover: { state: 'none', archiveAttempts: 0 },
    ...over
  }
}

describe('deriveTaskDisplayState', () => {
  const ctx = { inCurrentStep: true }

  it('done and cancelled tasks are done', () => {
    expect(deriveTaskDisplayState(task({ status: 'done' }), null, ctx).state).toBe('done')
    expect(deriveTaskDisplayState(task({ status: 'cancelled' }), null, ctx).state).toBe('done')
  })

  it('a task the summary lists as failed shows its attempts and cause (request #1242: failed 2 of 2)', () => {
    const s = summary({
      failed: [
        {
          taskId: 't1',
          title: 'Task one',
          attempts: 2,
          maxAttempts: 2,
          lastCause: 'timed_out',
          lastError: 'execution failed: retryable TIMED_OUT'
        }
      ]
    })
    expect(deriveTaskDisplayState(task({ status: 'todo' }), s, ctx)).toMatchObject({
      state: 'failed',
      attempt: 2,
      maxAttempts: 2,
      cause: 'timed_out'
    })
  })

  it('failed once of two is still failed, with the attempt count', () => {
    const s = summary({
      failed: [
        { taskId: 't1', title: 'Task one', attempts: 1, maxAttempts: 2, lastCause: 'agent_error' }
      ]
    })
    expect(deriveTaskDisplayState(task(), s, ctx)).toMatchObject({
      state: 'failed',
      attempt: 1,
      maxAttempts: 2
    })
  })

  it('a failed task that was done afterwards is done', () => {
    const s = summary({
      failed: [
        { taskId: 't1', title: 'Task one', attempts: 1, maxAttempts: 2, lastCause: 'agent_error' }
      ]
    })
    expect(deriveTaskDisplayState(task({ status: 'done' }), s, ctx).state).toBe('done')
  })

  it('running comes from the summary with the attempt, or from the task status', () => {
    const s = summary({
      running: [{ taskId: 't1', title: 'Task one', attempt: 2, maxAttempts: 2 }]
    })
    expect(deriveTaskDisplayState(task(), s, ctx)).toEqual({
      state: 'running',
      attempt: 2,
      maxAttempts: 2
    })
    expect(deriveTaskDisplayState(task({ status: 'in_progress' }), null, ctx).state).toBe('running')
  })

  it('blocked is blocked', () => {
    expect(deriveTaskDisplayState(task({ status: 'blocked' }), null, ctx).state).toBe('blocked')
  })

  it('waits on the open task before it in the current step, and is queued elsewhere', () => {
    const before = task({ id: 't0', title: 'Task zero', status: 'in_progress' })
    expect(
      deriveTaskDisplayState(task(), null, { inCurrentStep: true, previousOpen: before })
    ).toEqual({ state: 'waiting', waitsOn: 'Task zero' })
    expect(
      deriveTaskDisplayState(task(), null, { inCurrentStep: false, previousOpen: before }).state
    ).toBe('queued')
    expect(deriveTaskDisplayState(task(), null, ctx).state).toBe('queued')
  })
})
