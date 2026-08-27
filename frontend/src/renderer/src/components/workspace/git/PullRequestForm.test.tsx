// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { PullRequestForm } from './PullRequestForm'

// Why: 'git.pr.create' has never existed as an RPC method
// (backend/src/main/runtime/rpc/methods/git.ts has no git.pr.* group) — this
// form used to call it unconditionally and crash. It now surfaces an honest
// "not available" error instead — see PullRequestForm.tsx's file header.
describe('PullRequestForm (unavailable, see file header)', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  // Why: the error text is rendered as `⚠ {error}` — two adjacent text nodes
  // in the same <div>, so a plain string/regex getByText match (which only
  // matches a single text node) can miss it. Matching on element.textContent
  // alone over-matches (every ancestor up to <body> also "contains" the
  // substring) and getByText throws on multiple matches — the children-check
  // picks the single deepest element that contains it.
  function hasText(text: string) {
    return (_content: string, element: Element | null) => {
      if (!element?.textContent?.includes(text)) {
        return false
      }
      return Array.from(element.children).every((child) => !child.textContent?.includes(text))
    }
  }

  it('submit button is disabled while the title is empty', () => {
    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    expect(screen.getByText('Create Pull Request')).toBeDisabled()
  })

  it('submitting with a title shows the not-available message instead of creating a PR', async () => {
    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My PR' } })
    fireEvent.click(screen.getByText('Create Pull Request'))
    expect(await screen.findByText(hasText("isn't supported yet"))).toBeInTheDocument()
    expect(screen.queryByText('✓ Pull Request Created')).not.toBeInTheDocument()
  })

  it('generating an AI description shows the not-available message', async () => {
    render(<PullRequestForm projectId="p1" worktreePath="/repo" currentBranch="feature" />)
    fireEvent.click(screen.getByText('✨ AI Description'))
    expect(await screen.findByText(hasText("isn't supported yet"))).toBeInTheDocument()
  })
})
