// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../task/agent-activity/TaskAgentActivityPanel', () => ({
  TaskAgentActivityPanel: ({ task }: { task: { id: string } }) => (
    <div data-testid={`agent-log-${task.id}`} />
  )
}))

import { ExecutionFailureCard } from './ExecutionFailureCard'
import { ExecutionStepList } from './ExecutionStepList'
import { ExecutionSummaryStrip, formatElapsed, runLabelOf } from './ExecutionSummaryStrip'
import { HandoverRow } from './HandoverRow'
import { buildExecutionSteps } from './execution-step-model'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../../../shared/task-types'

afterEach(cleanup)

const task = (id: string, status: OrcaTask['status'], over: Partial<OrcaTask> = {}): OrcaTask =>
  ({ id, title: `Task ${id}`, status, ...over }) as OrcaTask
const phase = (id: string): OrcaTask =>
  ({ id, title: `Phase ${id}`, status: 'todo', type: 'phase' }) as OrcaTask

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

describe('ExecutionSummaryStrip', () => {
  it('shows progress with the step, the state label and Stop run only while the run can be stopped', () => {
    const onStop = vi.fn()
    const { rerender } = render(
      <ExecutionSummaryStrip status="executing" summary={summary()} onStop={onStop} />
    )
    expect(screen.getByTestId('execution-progress-text')).toHaveTextContent(
      '27/28 tasks · step 7/7'
    )
    expect(screen.getByTestId('execution-run-label')).toHaveTextContent('Running')
    fireEvent.click(screen.getByTestId('execution-stop-run'))
    expect(onStop).toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: /pause/i })).toBeNull()

    rerender(<ExecutionSummaryStrip status="request_backlog" summary={summary()} onStop={onStop} />)
    expect(screen.getByTestId('execution-run-label')).toHaveTextContent('Stopped')
    expect(screen.queryByTestId('execution-stop-run')).toBeNull()
  })

  it('labels the state', () => {
    expect(runLabelOf('completed', summary())).toBe('done')
    expect(runLabelOf('executing', summary({ needsYou: [{ kind: 'approval', refId: 'a' }] }))).toBe(
      'needs_you'
    )
    expect(runLabelOf('awaiting_information', summary())).toBe('needs_you')
    expect(runLabelOf('request_backlog', summary())).toBe('stopped')
  })

  it('formats the elapsed time', () => {
    const now = Date.parse('2026-10-16T12:00:00Z')
    expect(formatElapsed('2026-10-16T11:48:00Z', now)).toBe('12m')
    expect(formatElapsed('2026-10-16T10:55:00Z', now)).toBe('1h 05m')
    expect(formatElapsed('2026-10-14T09:00:00Z', now)).toBe('2d 3h')
    expect(formatElapsed(undefined, now)).toBeNull()
  })
})

describe('HandoverRow', () => {
  it('OpenSpec describes the archive and handover; native says it is done with the last task', () => {
    const { rerender } = render(
      <HandoverRow engine="openspec" handover={{ state: 'none', archiveAttempts: 0 }} />
    )
    expect(screen.getByTestId('execution-handover')).toHaveAttribute('data-handover-state', 'none')
    rerender(<HandoverRow engine="openspec" handover={{ state: 'failed', archiveAttempts: 3 }} />)
    expect(screen.getByTestId('execution-handover')).toHaveTextContent(
      'Archiving failed after 3 attempts'
    )
    rerender(<HandoverRow engine="native" handover={{ state: 'none', archiveAttempts: 0 }} />)
    expect(screen.queryByTestId('execution-handover')).toBeNull()
    expect(screen.getByTestId('execution-footer-native')).toBeInTheDocument()
  })
})

describe('ExecutionFailureCard', () => {
  it('request #1242: says which task failed how many times, why, and offers the three actions', () => {
    const calls = { reopen: vi.fn(), log: vi.fn(), cancel: vi.fn() }
    render(
      <ExecutionFailureCard
        failed={{
          taskId: 't1',
          title: 'TG-1037',
          attempts: 2,
          maxAttempts: 2,
          lastCause: 'timed_out',
          lastError: 'execution failed: retryable TIMED_OUT'
        }}
        progress={summary().progress}
        busy={false}
        onReopen={calls.reopen}
        onViewLog={calls.log}
        onCancel={calls.cancel}
      />
    )
    const card = screen.getByTestId('execution-failure-card')
    expect(card).toHaveTextContent('“TG-1037” failed 2/2 times')
    expect(card).toHaveTextContent('Cause: Timed out')
    expect(card).toHaveTextContent('Everything before it is done and kept (27/28 tasks)')
    fireEvent.click(screen.getByTestId('execution-failure-reopen'))
    fireEvent.click(screen.getByTestId('execution-failure-log'))
    fireEvent.click(screen.getByTestId('execution-failure-cancel'))
    expect(calls.reopen).toHaveBeenCalled()
    expect(calls.log).toHaveBeenCalledWith('t1')
    expect(calls.cancel).toHaveBeenCalled()
  })
})

describe('ExecutionStepList', () => {
  const tree: PlanSubtree = {
    plan: task('plan', 'in_progress', { type: 'plan' }),
    phases: [phase('p1'), phase('p2'), phase('p3')],
    tasksByPhase: {
      p1: [task('a', 'done'), task('b', 'done')],
      p2: [task('c', 'done'), task('d', 'in_progress'), task('e', 'todo')],
      p3: [task('f', 'todo')]
    },
    flatTasks: []
  } as unknown as PlanSubtree

  it('folds finished steps, opens the current one, counts later ones, and shows six task states', () => {
    const s = summary({ running: [{ taskId: 'd', title: 'Task d', attempt: 1, maxAttempts: 2 }] })
    const steps = buildExecutionSteps(tree, 'p2')
    expect(steps.map((x) => x.state)).toEqual(['done', 'current', 'later'])
    render(<ExecutionStepList steps={steps} summary={s} onOpenTask={vi.fn()} />)
    expect(screen.getByTestId('execution-step-p1')).toHaveAttribute('data-step-state', 'done')
    expect(screen.queryByTestId('execution-task-a')).toBeNull() // folded
    expect(screen.getByTestId('execution-task-c')).toHaveAttribute('data-task-state', 'done')
    expect(screen.getByTestId('execution-task-d')).toHaveAttribute('data-task-state', 'running')
    expect(screen.getByTestId('execution-task-e')).toHaveAttribute('data-task-state', 'waiting')
    expect(screen.getByTestId('execution-task-state-e')).toHaveTextContent('Waiting for “Task d”')
    expect(screen.getByTestId('execution-later-steps')).toHaveTextContent(
      '1 more steps · 1 tasks to go'
    )
    expect(screen.getByTestId('agent-log-d')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Step 1\/3/ }))
    expect(screen.getByTestId('execution-task-a')).toBeInTheDocument()
  })

  it('a failed task shows how many attempts, the cause and the advice', () => {
    const s = summary({
      failed: [
        {
          taskId: 'f',
          title: 'Task f',
          attempts: 2,
          maxAttempts: 2,
          lastCause: 'timed_out',
          lastError: 'execution failed: retryable TIMED_OUT'
        }
      ]
    })
    const steps = buildExecutionSteps(tree, 'p3')
    render(<ExecutionStepList steps={steps} summary={s} onOpenTask={vi.fn()} />)
    expect(screen.getByTestId('execution-task-state-f')).toHaveTextContent('Failed 2 of 2')
    expect(screen.getByTestId('execution-task-failure-f')).toHaveTextContent('Timed out')
    expect(screen.queryByText('Open')).toBeNull()
  })

  it('a plan without steps is one flat list', () => {
    const flat = {
      plan: tree.plan,
      phases: [],
      tasksByPhase: {},
      flatTasks: [task('x', 'done'), task('y', 'todo')]
    } as unknown as PlanSubtree
    render(
      <ExecutionStepList
        steps={buildExecutionSteps(flat)}
        summary={summary()}
        onOpenTask={vi.fn()}
      />
    )
    expect(screen.getByTestId('execution-flat-tasks')).toBeInTheDocument()
    expect(screen.getByTestId('execution-task-y')).toHaveAttribute('data-task-state', 'queued')
  })

  it('opens the task when asked', () => {
    const onOpen = vi.fn()
    render(
      <ExecutionStepList
        steps={buildExecutionSteps(tree, 'p2')}
        summary={summary()}
        onOpenTask={onOpen}
      />
    )
    fireEvent.click(screen.getByTestId('execution-task-open-e'))
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'e' }))
  })
})
