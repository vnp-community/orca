// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { TaskCard } from '../TaskCard'
import type { TaskWithPlanPath } from '../../../../../shared/task-hierarchy'

const base: TaskWithPlanPath = {
  id: 't1',
  projectId: 'p1',
  title: 'Do it',
  type: 'task',
  status: 'todo',
  priority: 'medium',
  labels: [],
  visibility: 'private',
  progressPercent: 0,
  createdAt: new Date(),
  updatedAt: new Date()
}

describe('TaskCard plan path chip', () => {
  afterEach(cleanup)
  it('shows Plan / Phase chip when planPath is set', () => {
    render(
      <TaskCard
        task={{ ...base, planPath: ['Plan A', 'Phase 1'] }}
        depth={0}
        isExpanded={false}
        onToggle={() => {}}
        onSelect={() => {}}
      />
    )
    expect(screen.getByTestId('task-plan-path-t1')).toHaveTextContent('Plan A / Phase 1')
  })
  it('shows no chip without planPath', () => {
    render(
      <TaskCard task={base} depth={0} isExpanded={false} onToggle={() => {}} onSelect={() => {}} />
    )
    expect(screen.queryByTestId('task-plan-path-t1')).toBeNull()
  })
})
