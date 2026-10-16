// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { PlanOutline } from './PlanOutline'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../../../shared/task-types'

afterEach(cleanup)

const t = (id: string, status: OrcaTask['status'], hours?: number): OrcaTask =>
  ({ id, title: `Task ${id}`, status, estimatedHours: hours }) as OrcaTask
const ph = (id: string): OrcaTask =>
  ({ id, title: `Phase ${id}`, status: 'todo', type: 'phase' }) as OrcaTask

describe('PlanOutline', () => {
  const tree = {
    plan: t('plan', 'todo'),
    phases: [ph('p1'), ph('p2')],
    tasksByPhase: { p1: [t('a', 'done', 2), t('b', 'todo', 1.5)], p2: [t('c', 'todo')] },
    flatTasks: []
  } as unknown as PlanSubtree

  it('lists steps with task counts and estimated hours, the first one open, with no status badges or run buttons', () => {
    render(<PlanOutline tree={tree} hasPhases />)
    const first = screen.getByTestId('plan-outline-step-p1')
    expect(first).toHaveTextContent('1. Phase p1')
    expect(first).toHaveTextContent('2 tasks')
    expect(first).toHaveTextContent('~3.5h estimated')
    expect(screen.getByTestId('plan-outline-task-a')).toHaveAttribute('data-done', 'true')
    expect(screen.queryByTestId('plan-outline-task-c')).toBeNull() // second step is collapsed
    fireEvent.click(screen.getByRole('button', { name: /2\. Phase p2/ }))
    expect(screen.getByTestId('plan-outline-task-c')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /run|approve|start/i })).toBeNull()
  })

  it('a plan without steps is a flat ordered list', () => {
    render(<PlanOutline tree={tree} hasPhases={false} />)
    expect(screen.getByTestId('plan-outline-flat')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-outline-step-p1')).toBeNull()
    expect(screen.getAllByTestId(/^plan-outline-task-/)).toHaveLength(3)
  })
})
