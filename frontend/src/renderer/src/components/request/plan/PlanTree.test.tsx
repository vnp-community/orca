// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../task/TaskDetail', () => ({
  TaskDetail: () => <div data-testid="task-detail-stub" />
}))

import { useAppStore } from '../../../store'
import { PlanTree } from './PlanTree'
import { PlanSummaryHeader } from './PlanSummaryHeader'
import { attachApprovals } from './plan-approval-model'
import { planApproval, planTask, twoPhaseTree } from './plan-test-fixtures'

afterEach(() => {
  cleanup()
  useAppStore.setState({ tasks: [], activeTaskId: null })
})

describe('PlanTree', () => {
  it('renders phases in order with stats and no #TG on plan/phase', () => {
    const tree = twoPhaseTree()
    render(<PlanTree tree={tree} approvals={attachApprovals(tree, [])} />)
    const phases = screen.getAllByTestId(/^phase-node-/)
    expect(phases.map((p) => p.getAttribute('data-testid'))).toEqual([
      'phase-node-ph1',
      'phase-node-ph2'
    ])
    expect(screen.getByTestId('phase-summary-ph1')).toHaveTextContent('1/3')
    expect(screen.getByTestId('phase-node-ph1')).toHaveTextContent('1 blocked')
    expect(screen.getByTestId('phase-node-ph1')).toHaveTextContent('1 running')
    // work task with number shows it; phase does not
    expect(screen.getByTestId('plan-task-row-a1')).toHaveTextContent('#TG-7')
    expect(screen.getByTestId('phase-node-ph1').textContent).not.toMatch(/#TG-\d+.*Phase One/)
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('collapses and expands a phase with ArrowLeft/ArrowRight', () => {
    const tree = twoPhaseTree()
    render(<PlanTree tree={tree} approvals={attachApprovals(tree, [])} />)
    const toggle = screen.getByTestId('phase-toggle-ph1')
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    fireEvent.keyDown(toggle, { key: 'ArrowLeft' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByTestId('plan-task-row-a1')).toBeNull()
    fireEvent.keyDown(toggle, { key: 'ArrowRight' })
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
  })

  it('lists flat tasks for task_list plans', () => {
    const tree = {
      ...twoPhaseTree(),
      phases: [],
      tasksByPhase: {},
      flatTasks: [planTask('f1'), planTask('f2')]
    }
    render(<PlanTree tree={tree} approvals={attachApprovals(tree, [])} />)
    expect(screen.queryAllByTestId(/^phase-node-/)).toHaveLength(0)
    expect(screen.getByTestId('plan-flat-tasks').children).toHaveLength(2)
  })

  it('clicking a task sets activeTaskId, opens the sheet, and closing clears it', async () => {
    const tree = twoPhaseTree()
    render(<PlanTree tree={tree} approvals={attachApprovals(tree, [])} />)
    fireEvent.click(screen.getByTestId('plan-task-row-b2'))
    expect(useAppStore.getState().activeTaskId).toBe('b2')
    expect(useAppStore.getState().tasks.some((t) => t.id === 'b2')).toBe(true)
    expect(await screen.findByTestId('task-detail-stub')).toBeInTheDocument()
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    await vi.waitFor(() => expect(useAppStore.getState().activeTaskId).toBeNull())
  })

  it('renders the phase actions slot', () => {
    const tree = twoPhaseTree()
    render(
      <PlanTree
        tree={tree}
        approvals={attachApprovals(tree, [])}
        renderPhaseActions={(p) => <span data-testid={`slot-${p.id}`} />}
      />
    )
    expect(screen.getByTestId('slot-ph2')).toBeInTheDocument()
  })
})

describe('PlanSummaryHeader', () => {
  it('shows backend progress without the estimate label', () => {
    const tree = twoPhaseTree()
    tree.plan = { ...tree.plan!, progressPercent: 55 }
    render(<PlanSummaryHeader tree={tree} approval={planApproval('ap1', { status: 'approved' })} />)
    expect(screen.getByTestId('plan-progress-value')).toHaveTextContent('55%')
    expect(screen.getByTestId('plan-progress-value').textContent).not.toMatch(/estimated/)
    expect(screen.getByTestId('plan-counts')).toHaveTextContent('2 phases, 6 tasks')
  })

  it('falls back to a labelled estimate from child statuses', () => {
    const tree = twoPhaseTree()
    render(<PlanSummaryHeader tree={tree} approval={null} />)
    // 100 + 40 + 0 + 0*3 over 6 tasks = 23
    expect(screen.getByTestId('plan-progress-value')).toHaveTextContent('23% (estimated)')
  })
})
