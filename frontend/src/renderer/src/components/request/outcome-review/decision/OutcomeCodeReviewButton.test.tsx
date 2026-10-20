// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OutcomeCodeReviewButton, type CodeReviewLink } from './OutcomeCodeReviewButton'

afterEach(cleanup)

const wt = (id: string, branch: string) => ({ taskId: `t-${id}`, worktreeId: id, branch })
const link = (over: Partial<CodeReviewLink> = {}): CodeReviewLink => ({
  enabled: true,
  worktrees: [wt('wt-1', 'feat/one')],
  open: vi.fn(),
  ...over
})

describe('OutcomeCodeReviewButton', () => {
  it('opens the code review at the lens of the row for the one worktree there is', () => {
    const l = link()
    render(<OutcomeCodeReviewButton lens="dataflow" link={l} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open in code review' }))
    expect(l.open).toHaveBeenCalledWith('wt-1', 'dataflow')
  })

  it('with several worktrees the person chooses which', () => {
    const l = link({ worktrees: [wt('wt-1', 'feat/one'), wt('wt-2', 'feat/two')] })
    render(<OutcomeCodeReviewButton lens="erd" link={l} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open in code review' }))
    expect(l.open).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'feat/two' }))
    expect(l.open).toHaveBeenCalledWith('wt-2', 'erd')
  })

  it.each([
    [
      'the code review is off',
      { enabled: false },
      'architecture',
      'unsupported',
      'Code review is not available here.'
    ],
    [
      'the review has no worktree',
      { worktrees: [] },
      'architecture',
      'no_worktree',
      'Code review is not available for this Request: the review has no worktree to open.'
    ],
    ['this part has no view', {}, null, 'no_lens', 'This part has no view in the code review.']
  ] as [string, Partial<CodeReviewLink>, 'architecture' | null, string, string][])(
    'stays on screen, dimmed, and says why when %s',
    (_name, over, lens, state, why) => {
      const l = link(over)
      render(<OutcomeCodeReviewButton lens={lens} link={l} />)
      const button = screen.getByTestId('outcome-code-review')
      expect(button).toHaveAttribute('aria-disabled', 'true')
      expect(button).toHaveAttribute('data-state', state)
      expect(button).toHaveAttribute('title', why)
      expect(button).toHaveTextContent(why)
      fireEvent.click(button)
      expect(l.open).not.toHaveBeenCalled()
    }
  )
})
