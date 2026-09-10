// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { ReactNode } from 'react'
import { AttachWorkflowTemplateAction } from '../AttachWorkflowTemplateAction'
import type { OrcaTask } from '../../../../../shared/task-types'

type TestSelectProps = {
  children: ReactNode
  onValueChange?: (value: string) => void
  value?: string
}
type TestSelectContentProps = {
  children: ReactNode
}
type TestSelectItemProps = {
  value: string
  children: ReactNode
}

// Drive the dropdown through a native <select> so the test doesn't have to deal
// with Radix's portal/pointer-capture machinery in happy-dom (same recipe as
// ModelSelector.test.tsx).
vi.mock('../../ui/select', () => ({
  Select: ({ children, onValueChange, value }: TestSelectProps) => (
    <select
      data-testid="workflow-template-select"
      value={value}
      onChange={(e) => onValueChange?.(e.target.value)}
    >
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: TestSelectContentProps) => <>{children}</>,
  SelectItem: ({ value, children }: TestSelectItemProps) => (
    <option value={value}>{children}</option>
  )
}))

const updateTaskSpy = vi.fn()

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(vi.fn(), {
    getState: () => ({ settings: {}, updateTask: updateTaskSpy })
  })
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))
import { toast } from 'sonner'
const mockToast = vi.mocked(toast)

const task: OrcaTask = { id: 't1', title: 'My Task', status: 'todo', priority: 'high' } as OrcaTask

describe('AttachWorkflowTemplateAction', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('mounts → calls workflow.template.list, populates dropdown from res.templates (id/name)', async () => {
    mockRpc.mockResolvedValueOnce({
      templates: [
        { id: 'wt1', tenant_id: 'tn1', name: 'Template One', dag_json: '{}', scope: 'company' },
        { id: 'wt2', tenant_id: 'tn1', name: 'Template Two', dag_json: '{}', scope: 'company' }
      ]
    })
    render(<AttachWorkflowTemplateAction task={task} />)

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.list', {})

    await waitFor(() => {
      expect(screen.getByText('Template One')).toBeInTheDocument()
      expect(screen.getByText('Template Two')).toBeInTheDocument()
    })
  })

  it('selecting an item → calls task.update with patch { workflowTemplateId } + optimistic store update immediately', async () => {
    mockRpc.mockResolvedValueOnce({ templates: [{ id: 'wt1', name: 'Template One' }] })
    mockRpc.mockResolvedValueOnce(undefined) // task.update

    render(<AttachWorkflowTemplateAction task={task} />)
    await waitFor(() => expect(screen.getByText('Template One')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('workflow-template-select'), { target: { value: 'wt1' } })

    // optimistic store update fires synchronously, before the RPC resolves
    expect(updateTaskSpy).toHaveBeenCalledWith('t1', { workflowTemplateId: 'wt1' })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.update', {
        taskId: 't1',
        patch: { workflowTemplateId: 'wt1' }
      })
    })
  })

  it('workflow.template.list fails → toast.error, dropdown still renders empty without crashing', async () => {
    mockRpc.mockRejectedValueOnce(new Error('rpc down'))
    render(<AttachWorkflowTemplateAction task={task} />)

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalled()
    })
    expect(screen.getByTestId('workflow-template-select')).toBeInTheDocument()
  })

  it('task.update fails (backend field not supported yet) → does not throw, optimistic state is kept', async () => {
    mockRpc.mockResolvedValueOnce({ templates: [{ id: 'wt1', name: 'Template One' }] })
    mockRpc.mockRejectedValueOnce(new Error('unknown field'))

    render(<AttachWorkflowTemplateAction task={task} />)
    await waitFor(() => expect(screen.getByText('Template One')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('workflow-template-select'), { target: { value: 'wt1' } })

    expect(updateTaskSpy).toHaveBeenCalledWith('t1', { workflowTemplateId: 'wt1' })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.update', {
        taskId: 't1',
        patch: { workflowTemplateId: 'wt1' }
      })
    })
    // no unhandled rejection / thrown error reaches the test — optimistic update stays as the only call
    expect(updateTaskSpy).toHaveBeenCalledTimes(1)
  })
})
