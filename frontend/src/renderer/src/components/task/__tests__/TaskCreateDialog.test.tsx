// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskCreateDialog } from '../TaskCreateDialog'

describe('TaskCreateDialog', () => {
  beforeEach(() => cleanup())

  it('renders "New Task" title when no parentId', () => {
    render(<TaskCreateDialog open onOpenChange={vi.fn()} onCreate={vi.fn()} />)
    expect(screen.getByText('New Task')).toBeInTheDocument()
  })

  it('renders "New Subtask" title when parentId is set', () => {
    render(<TaskCreateDialog open parentId="p1" onOpenChange={vi.fn()} onCreate={vi.fn()} />)
    expect(screen.getByText('New Subtask')).toBeInTheDocument()
  })

  it('submit button disabled when title is empty', () => {
    render(<TaskCreateDialog open onOpenChange={vi.fn()} onCreate={vi.fn()} />)
    expect(screen.getByTestId('new-task-submit')).toBeDisabled()
  })

  it('typing a title enables submit, clicking it calls onCreate(title, parentId)', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    const onOpenChange = vi.fn()
    render(
      <TaskCreateDialog open parentId="parent-1" onOpenChange={onOpenChange} onCreate={onCreate} />
    )

    fireEvent.change(screen.getByTestId('new-task-title-input'), {
      target: { value: '  My New Task  ' }
    })
    expect(screen.getByTestId('new-task-submit')).not.toBeDisabled()

    fireEvent.click(screen.getByTestId('new-task-submit'))

    await waitFor(() => {
      expect(onCreate).toHaveBeenCalledWith('My New Task', 'parent-1')
      expect(onOpenChange).toHaveBeenCalledWith(false)
    })
  })

  it('pressing Enter in the input submits', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    render(<TaskCreateDialog open onOpenChange={vi.fn()} onCreate={onCreate} />)

    const input = screen.getByTestId('new-task-title-input')
    fireEvent.change(input, { target: { value: 'Enter task' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(onCreate).toHaveBeenCalledWith('Enter task', undefined)
    })
  })
})
