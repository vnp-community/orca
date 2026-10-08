// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const gateState = vi.hoisted(() => ({ gates: {} as Record<string, unknown> }))
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) =>
      selector({
        activeTaskId: 't1',
        settings: {},
        tasks: [],
        templates: [],
        currentUser: { id: 'u1' },
        executionGateByTaskId: gateState.gates,
        setActiveWorkspaceTab: vi.fn()
      })
    ),
    { getState: () => ({ settings: {}, updateTask: vi.fn() }) }
  )
}))
vi.mock('../../../hooks/useTask', () => ({
  useTask: () => ({
    task: { id: 't1', title: 'My Task', status: 'todo', priority: 'high', projectId: 'p1' },
    updateTask: vi.fn()
  })
}))
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({
    project: { id: 'p1' },
    currentWorktree: null,
    setCurrentWorktree: vi.fn()
  })
}))
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async (_t: unknown, method: string) =>
    method === 'workflow.template.list'
      ? { templates: [] }
      : method === 'task.getDependencies'
        ? []
        : undefined
  ),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import { TaskDetail } from '../TaskDetail'

describe('TaskDetail execution gate', () => {
  beforeEach(() => {
    gateState.gates = {}
  })
  afterEach(cleanup)

  it('does not change the Run button without a gate', () => {
    render(<TaskDetail />)
    expect(screen.getByTestId('run-agent-btn')).toBeEnabled()
  })

  it('locks Run with a tooltip when the task is gated', () => {
    gateState.gates = { t1: { requestId: 'r1', reason: 'phase_not_approved' } }
    render(<TaskDetail />)
    const btn = screen.getByTestId('run-agent-btn')
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('title', 'Approve the Phase or Plan before running this task')
  })
})
