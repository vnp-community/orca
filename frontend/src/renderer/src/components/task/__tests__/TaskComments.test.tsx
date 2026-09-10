// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskComments } from '../TaskComments'
import { useTaskComments } from '../../../hooks/useTaskComments'

vi.mock('../../../hooks/useTaskComments', () => ({
  useTaskComments: vi.fn()
}))

const mockUseTaskComments = vi.mocked(useTaskComments)

describe('TaskComments', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('isSupported: false → shows the "unavailable" message, no form', () => {
    mockUseTaskComments.mockReturnValue({
      comments: [],
      addComment: vi.fn(),
      isSupported: false
    })
    render(<TaskComments taskId="t1" />)
    expect(screen.getByTestId('task-comments-unsupported')).toBeInTheDocument()
    expect(screen.queryByTestId('task-comment-input')).not.toBeInTheDocument()
  })

  it('isSupported: true, empty comments → shows empty state', () => {
    mockUseTaskComments.mockReturnValue({
      comments: [],
      addComment: vi.fn(),
      isSupported: true
    })
    render(<TaskComments taskId="t1" />)
    expect(screen.getByText('Chưa có comment nào.')).toBeInTheDocument()
  })

  it('renders comments with userId and content', () => {
    mockUseTaskComments.mockReturnValue({
      comments: [
        {
          id: 1,
          taskId: 't1',
          userId: 'alice',
          content: 'looks good',
          type: 'comment',
          createdAt: new Date()
        }
      ],
      addComment: vi.fn(),
      isSupported: true
    })
    render(<TaskComments taskId="t1" />)
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('looks good')).toBeInTheDocument()
  })

  it('Gửi disabled when input empty', () => {
    mockUseTaskComments.mockReturnValue({ comments: [], addComment: vi.fn(), isSupported: true })
    render(<TaskComments taskId="t1" />)
    expect(screen.getByTestId('task-comment-send')).toBeDisabled()
  })

  it('typing then clicking Gửi calls addComment(text) and clears input', () => {
    const addComment = vi.fn()
    mockUseTaskComments.mockReturnValue({ comments: [], addComment, isSupported: true })
    render(<TaskComments taskId="t1" />)

    const input = screen.getByTestId('task-comment-input')
    fireEvent.change(input, { target: { value: 'a new comment' } })
    fireEvent.click(screen.getByTestId('task-comment-send'))

    expect(addComment).toHaveBeenCalledWith('a new comment')
    expect(input).toHaveValue('')
  })
})
