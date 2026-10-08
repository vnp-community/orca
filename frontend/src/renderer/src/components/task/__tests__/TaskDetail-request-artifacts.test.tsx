// @vitest-environment happy-dom
// TaskDetail with readiness (FE-REQ-TASK-036-07) and the Result tab (FE-REQ-TASK-036-08).
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const h = vi.hoisted(() => ({
  task: {} as Record<string, unknown>,
  rpc: {} as Record<string, unknown>,
  openSettingsPage: vi.fn(),
  openSettingsTarget: vi.fn()
}))
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) =>
      selector({
        activeTaskId: 't1',
        settings: {},
        tasks: [],
        templates: [],
        currentUser: { id: 'u1' },
        executionGateByTaskId: {},
        setActiveWorkspaceTab: vi.fn(),
        openSettingsPage: h.openSettingsPage,
        openSettingsTarget: h.openSettingsTarget
      })
    ),
    { getState: () => ({ settings: {}, updateTask: vi.fn() }) }
  )
}))
vi.mock('../../../hooks/useTask', () => ({
  useTask: () => ({ task: h.task, updateTask: vi.fn() })
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
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: vi.fn(async (method: string) =>
    method in h.rpc
      ? { ok: true, value: h.rpc[method] }
      : { ok: false, error: { kind: 'unsupported', code: 'method_not_found', message: 'm' } }
  )
}))
vi.mock('../../../hooks/useTaskPermission', () => ({
  useTaskPermission: () => ({ canExecute: true, canEdit: true })
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import { TaskDetail } from '../TaskDetail'

const requestTask = {
  id: 't1',
  title: 'Add index',
  status: 'todo',
  priority: 'high',
  projectId: 'p1',
  requestId: 'r1'
}

beforeEach(() => {
  h.task = requestTask
  h.rpc = {}
  h.openSettingsPage.mockReset()
  h.openSettingsTarget.mockReset()
})
afterEach(cleanup)

describe('TaskDetail + readiness', () => {
  it('a needs_info report shows the badge and locks Run with an explanation', async () => {
    h.rpc = {
      'readiness.get': {
        report: {
          task_id: 't1',
          outcome: 'needs_info',
          findings: [{ code: 'MISSING_INPUT', tier: 'semantic', message: 'Which table?' }]
        }
      }
    }
    render(<TaskDetail />)
    expect(await screen.findByText('Needs information')).toBeInTheDocument()
    const run = screen.getByTestId('run-agent-btn')
    await waitFor(() => expect(run).toBeDisabled())
    expect(run).toHaveAttribute(
      'title',
      'This task is not ready yet. Open the readiness report for details.'
    )
  })

  it('env_defect without a dev server: the report sheet offers "Connect a dev server" and opens Settings > Servers', async () => {
    h.rpc = {
      'readiness.get': {
        report: {
          task_id: 't1',
          outcome: 'env_defect',
          findings: [{ code: 'ENV_MISSING', tier: 'environment', message: 'DATABASE_URL' }]
        }
      }
    }
    render(<TaskDetail />)
    await screen.findByText('Environment issue')
    fireEvent.click(screen.getByTestId('check-readiness-btn'))
    const sheet = await screen.findByTestId('readiness-report-sheet')
    expect(sheet).toHaveTextContent('Only variable names are shown, never values.')
    fireEvent.click(screen.getByRole('button', { name: 'Connect a dev server' }))
    expect(h.openSettingsTarget).toHaveBeenCalledWith({ pane: 'servers', repoId: null })
    expect(h.openSettingsPage).toHaveBeenCalled()
  })

  it('a ready report keeps Run enabled; tasks outside a Request show no readiness UI', async () => {
    h.rpc = { 'readiness.get': { report: { task_id: 't1', outcome: 'ready', findings: [] } } }
    const first = render(<TaskDetail />)
    expect(await screen.findByText('Ready')).toBeInTheDocument()
    expect(screen.getByTestId('run-agent-btn')).toBeEnabled()
    first.unmount()

    h.task = { ...requestTask, requestId: undefined }
    render(<TaskDetail />)
    expect(screen.queryByTestId('check-readiness-btn')).toBeNull()
    expect(screen.queryByRole('tab', { name: 'Result' })).toBeNull()
  })

  it('unsupported readiness channels never lock Run (pre-CR-029 behaviour)', async () => {
    render(<TaskDetail />)
    await waitFor(() => expect(screen.queryByTestId('check-readiness-btn')).toBeNull())
    expect(screen.getByTestId('run-agent-btn')).toBeEnabled()
  })
})

describe('TaskDetail Result tab (execution.get)', () => {
  it('shows the structured result with the agent-vs-Orca columns', async () => {
    h.rpc = {
      'execution.get': {
        result: {
          task_id: 't1',
          attempt: 2,
          parse_status: 'ok',
          status: 'done',
          summary: 'Added index <b>idx</b>',
          files_changed: ['db/migrations/001.sql'],
          checks_run: [{ id: 'unit', exit: 0 }],
          verdict: {
            status: 'failed',
            findings: [{ code: 'CHECK_MISMATCH', message: 'unit failed on re-run' }]
          }
        }
      }
    }
    render(<TaskDetail />)
    const tab = await screen.findByRole('tab', { name: 'Result' })
    fireEvent.mouseDown(tab)
    fireEvent.click(tab)
    expect(await screen.findByText('Attempt 2')).toBeInTheDocument()
    expect(screen.getByText('Added index <b>idx</b>')).toBeInTheDocument()
    expect(screen.getByText('db/migrations/001.sql')).toBeInTheDocument()
    expect(screen.getByTestId('execution-result-panel')).toHaveTextContent('unit')
    expect(screen.getByTestId('execution-secret-scan')).toBeInTheDocument()
  })

  it('hides the Result tab when execution.get is unsupported', async () => {
    render(<TaskDetail />)
    await waitFor(() => expect(screen.queryByRole('tab', { name: 'Result' })).toBeNull())
  })
})
