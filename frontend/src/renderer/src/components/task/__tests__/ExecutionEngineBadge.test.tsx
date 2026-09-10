// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ExecutionEngineBadge } from '../ExecutionEngineBadge'
import type { OrcaTask } from '../../../../../shared/task-types'

let storeState: { tasks: OrcaTask[]; templates: { id: string; name: string }[] }

vi.mock('../../../store', () => ({
  useAppStore: (selector: (s: typeof storeState) => unknown) => selector(storeState)
}))

const baseTask: OrcaTask = {
  id: 't1',
  title: 'Parent Task',
  status: 'todo',
  priority: 'high'
} as OrcaTask

describe('ExecutionEngineBadge', () => {
  beforeEach(() => {
    cleanup()
    storeState = { tasks: [], templates: [] }
  })

  it('no subtasks, no workflowTemplateId → renders "Direct Agent"', () => {
    render(<ExecutionEngineBadge task={baseTask} />)
    expect(screen.getByTestId('execution-engine-badge')).toHaveTextContent('Direct Agent')
  })

  it('has subtasks (store.tasks has t.parentId === task.id) → renders "Orchestration"', () => {
    storeState.tasks = [{ id: 'c1', parentId: 't1', title: 'Child' } as OrcaTask]
    render(<ExecutionEngineBadge task={baseTask} />)
    expect(screen.getByTestId('execution-engine-badge')).toHaveTextContent('Orchestration')
  })

  it('has workflowTemplateId + template found in store.templates → renders "Workflow: <name>"', () => {
    storeState.templates = [{ id: 'wt1', name: 'My Template' }]
    const task = { ...baseTask, workflowTemplateId: 'wt1' }
    render(<ExecutionEngineBadge task={task} />)
    expect(screen.getByTestId('execution-engine-badge')).toHaveTextContent('Workflow: My Template')
  })

  it('has workflowTemplateId AND subtasks → still prefers "Workflow" (CR-001 priority order)', () => {
    storeState.tasks = [{ id: 'c1', parentId: 't1', title: 'Child' } as OrcaTask]
    storeState.templates = [{ id: 'wt1', name: 'My Template' }]
    const task = { ...baseTask, workflowTemplateId: 'wt1' }
    render(<ExecutionEngineBadge task={task} />)
    expect(screen.getByTestId('execution-engine-badge')).toHaveTextContent('Workflow: My Template')
  })
})
