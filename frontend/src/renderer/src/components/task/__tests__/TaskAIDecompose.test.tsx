// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskAIDecompose } from '../TaskAIDecompose'
import { useTask } from '../../../hooks/useTask'

vi.mock('../../../hooks/useTask', () => ({
  useTask: vi.fn()
}))

describe('TaskAIDecompose', () => {
  const aiDecompose = vi.fn()
  const acceptSubtasks = vi.fn()
  const parentTask = { id: 't1', projectId: 'p1' } as any

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useTask).mockReturnValue({
      aiDecompose,
      acceptSubtasks
    } as any)
  })

  it('Decompose button calls aiDecompose', async () => {
    aiDecompose.mockResolvedValue([])
    render(<TaskAIDecompose parentTask={parentTask} />)
    fireEvent.click(screen.getByTestId('decompose-btn'))
    expect(aiDecompose).toHaveBeenCalled()
  })

  it('shows loading indicator while waiting', async () => {
    aiDecompose.mockReturnValue(new Promise(() => {}))
    render(<TaskAIDecompose parentTask={parentTask} />)
    fireEvent.click(screen.getByTestId('decompose-btn'))
    const btn = screen.getByTestId('decompose-btn')
    expect(btn).toBeDisabled()
    expect(btn.querySelector('.animate-spin')).toBeInTheDocument()
  })

  it('shows suggestions list after resolve', async () => {
    aiDecompose.mockResolvedValue([
      { title: 'Subtask 1', type: 'feature' }
    ])
    render(<TaskAIDecompose parentTask={parentTask} />)
    fireEvent.click(screen.getByTestId('decompose-btn'))
    
    await waitFor(() => {
      expect(screen.getByTestId('proposed-subtasks')).toBeInTheDocument()
      expect(screen.getByText('Subtask 1')).toBeInTheDocument()
      expect(screen.getByText('feature')).toBeInTheDocument()
    })
  })

  it('Accept All button calls acceptSubtasks', async () => {
    const subtasks = [{ title: 'Subtask 1', type: 'feature' }]
    aiDecompose.mockResolvedValue(subtasks)
    render(<TaskAIDecompose parentTask={parentTask} />)
    fireEvent.click(screen.getByTestId('decompose-btn'))
    
    await waitFor(() => {
      expect(screen.getByTestId('accept-subtasks-btn')).toBeInTheDocument()
    })
    
    fireEvent.click(screen.getByTestId('accept-subtasks-btn'))
    expect(acceptSubtasks).toHaveBeenCalledWith(subtasks, 'p1')
  })

  it('Cancel button resets suggestions', async () => {
    aiDecompose.mockResolvedValue([{ title: 'Subtask 1', type: 'feature' }])
    render(<TaskAIDecompose parentTask={parentTask} />)
    fireEvent.click(screen.getByTestId('decompose-btn'))
    
    await waitFor(() => {
      expect(screen.getByTestId('proposed-subtasks')).toBeInTheDocument()
    })
    
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByTestId('proposed-subtasks')).not.toBeInTheDocument()
  })
})
