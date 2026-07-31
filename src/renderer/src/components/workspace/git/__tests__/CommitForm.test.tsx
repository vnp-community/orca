// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { CommitForm } from '../CommitForm'
import { useGit } from '../../../../hooks/useGit'

vi.mock('../../../../hooks/useGit', () => ({
  useGit: vi.fn()
}))

vi.mock('../../ui/textarea', () => ({
  Textarea: (props: any) => <textarea data-testid="commit-message-input" {...props} />
}))

describe('CommitForm', () => {
  const commit = vi.fn()
  const push = vi.fn()
  const aiCommitMessage = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useGit).mockReturnValue({
      commit,
      push,
      aiCommitMessage,
      stagedFiles: [{ path: 'test.ts' }], // non-empty so we can commit
      isPushing: false,
      isCommitting: false,
    } as any)
  })

  it('empty message → Commit button disabled', () => {
    render(<CommitForm />)
    const btn = screen.getByTestId('commit-btn')
    expect(btn).toBeDisabled()
  })

  it('non-empty message → Commit calls git.commit', async () => {
    render(<CommitForm />)
    const input = screen.getByTestId('commit-message-input')
    fireEvent.change(input, { target: { value: 'fix typo' } })
    
    const btn = screen.getByTestId('commit-btn')
    expect(btn).not.toBeDisabled()
    fireEvent.click(btn)
    
    expect(commit).toHaveBeenCalledWith('fix typo')
  })

  it('after commit: message field is cleared', async () => {
    render(<CommitForm />)
    const input = screen.getByTestId('commit-message-input')
    fireEvent.change(input, { target: { value: 'fix typo' } })
    
    fireEvent.click(screen.getByTestId('commit-btn'))
    
    await waitFor(() => {
      expect(input).toHaveValue('')
    })
  })

  it('AI assist button calls RPC to generate message', async () => {
    aiCommitMessage.mockResolvedValue('ai generated commit msg')
    render(<CommitForm />)
    fireEvent.click(screen.getByTestId('ai-commit-btn'))
    expect(aiCommitMessage).toHaveBeenCalled()
  })

  it('generated message populates the textarea', async () => {
    aiCommitMessage.mockResolvedValue('ai generated commit msg')
    render(<CommitForm />)
    fireEvent.click(screen.getByTestId('ai-commit-btn'))
    
    await waitFor(() => {
      expect(screen.getByTestId('commit-message-input')).toHaveValue('ai generated commit msg')
    })
  })

  it('isCommitting disables buttons and shows loader', () => {
    vi.mocked(useGit).mockReturnValue({
      commit, push, aiCommitMessage, stagedFiles: [{ path: 'test.ts' }],
      isPushing: false,
      isCommitting: true, // TRUE
    } as any)
    
    render(<CommitForm />)
    const input = screen.getByTestId('commit-message-input')
    fireEvent.change(input, { target: { value: 'fix typo' } })
    
    const btn = screen.getByTestId('commit-btn')
    expect(btn).toBeDisabled()
    expect(btn.querySelector('.animate-spin')).toBeInTheDocument()
  })
})
