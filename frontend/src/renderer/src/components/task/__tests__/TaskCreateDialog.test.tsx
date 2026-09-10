// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskCreateDialog } from '../TaskCreateDialog'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (state: Record<string, unknown>) => unknown) =>
      fn ? fn({ settings: {} }) : { settings: {} },
    {
      getState: () => ({ settings: {} })
    }
  )
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('TaskCreateDialog', () => {
  const onCreated = vi.fn()
  const onCancel = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(cleanup)

  it('title rỗng → nút Create disabled', () => {
    render(<TaskCreateDialog projectId="p1" onCreated={onCreated} onCancel={onCancel} />)
    expect(screen.getByTestId('task-create-submit')).toBeDisabled()
  })

  it('nhập title, bấm Create → gọi task.create({title, projectId}), gọi onCreated khi thành công', async () => {
    mockRpc.mockResolvedValueOnce({ id: 't-new' })
    render(<TaskCreateDialog projectId="p1" onCreated={onCreated} onCancel={onCancel} />)

    fireEvent.change(screen.getByTestId('task-create-title-input'), {
      target: { value: 'My New Task' }
    })
    expect(screen.getByTestId('task-create-submit')).not.toBeDisabled()
    fireEvent.click(screen.getByTestId('task-create-submit'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.create', {
        title: 'My New Task',
        projectId: 'p1'
      })
      expect(onCreated).toHaveBeenCalled()
    })
  })

  it('Enter trong input title → submit giống bấm Create', async () => {
    mockRpc.mockResolvedValueOnce({ id: 't-new' })
    render(<TaskCreateDialog projectId="p1" onCreated={onCreated} onCancel={onCancel} />)

    const input = screen.getByTestId('task-create-title-input')
    fireEvent.change(input, { target: { value: 'Enter Task' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.create', {
        title: 'Enter Task',
        projectId: 'p1'
      })
    })
  })

  it('RPC lỗi → toast.error, dialog không tự đóng (onCreated không được gọi)', async () => {
    mockRpc.mockRejectedValueOnce(new Error('boom'))
    render(<TaskCreateDialog projectId="p1" onCreated={onCreated} onCancel={onCancel} />)

    fireEvent.change(screen.getByTestId('task-create-title-input'), {
      target: { value: 'Failing Task' }
    })
    fireEvent.click(screen.getByTestId('task-create-submit'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalled()
    })
    expect(onCreated).not.toHaveBeenCalled()
  })

  it('bấm Cancel → gọi onCancel, không gọi RPC nào', () => {
    render(<TaskCreateDialog projectId="p1" onCreated={onCreated} onCancel={onCancel} />)
    fireEvent.click(screen.getByTestId('task-create-cancel'))
    expect(onCancel).toHaveBeenCalled()
    expect(mockRpc).not.toHaveBeenCalled()
  })
})
