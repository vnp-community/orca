// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OutcomeDecisionBar } from './OutcomeDecisionBar'
import { makeReview } from '../outcome-review.fixture'
import type { DecisionResult } from './use-outcome-decisions'

afterEach(cleanup)

function show(over: Partial<React.ComponentProps<typeof OutcomeDecisionBar>> = {}) {
  const props = {
    review: makeReview(),
    openKeys: [] as string[],
    sentBackCount: 0,
    currentUserId: 'u1',
    busy: false,
    onApprove: vi.fn(async (): Promise<DecisionResult> => ({ ok: true })),
    onRequestChanges: vi.fn(),
    onShowItem: vi.fn(),
    ...over
  }
  render(<OutcomeDecisionBar {...props} />)
  return props
}

describe('OutcomeDecisionBar', () => {
  it('Approve is the one primary button, and it is never disabled', () => {
    show({ openKeys: ['a', 'b'] })
    expect(screen.getByTestId('outcome-approve')).toBeEnabled()
    expect(screen.getByTestId('outcome-request-changes')).toBeEnabled()
  })

  it('with decisions open, says so next to the button, takes the viewer to the first one and sends nothing', () => {
    const p = show({ openKeys: ['first', 'second'] })
    fireEvent.click(screen.getByTestId('outcome-approve'))
    expect(screen.getByTestId('outcome-approve-error')).toHaveTextContent(
      'Resolve 2 open decisions first'
    )
    expect(screen.getByTestId('outcome-approve-error')).toHaveAttribute('role', 'alert')
    expect(p.onShowItem).toHaveBeenCalledWith('first')
    expect(p.onApprove).not.toHaveBeenCalled()
  })

  it('approves with the note once nothing is open', async () => {
    const p = show()
    fireEvent.change(screen.getByLabelText('Note for the approval (optional)'), {
      target: { value: '  looks fine  ' }
    })
    fireEvent.click(screen.getByTestId('outcome-approve'))
    await waitFor(() => expect(p.onApprove).toHaveBeenCalledWith('  looks fine  '))
    expect(screen.queryByTestId('outcome-approve-error')).toBeNull()
  })

  it('shows the sentence of the server code when it refuses, as for a call that skipped this screen', async () => {
    show({
      onApprove: vi.fn(
        async (): Promise<DecisionResult> => ({
          ok: false,
          error: { code: 'REQUEST_REVIEW_DECISIONS_OPEN', message: 'RAW' }
        })
      )
    })
    fireEvent.click(screen.getByTestId('outcome-approve'))
    await waitFor(() =>
      expect(screen.getByTestId('outcome-approve-error')).toHaveTextContent(
        'Some decisions are still open.'
      )
    )
    expect(screen.getByTestId('outcome-approve-error')).not.toHaveTextContent('RAW')
  })

  it('after approval it says who, and that this is not a release sign-off; no Approve button until the review is ready again', () => {
    show({
      review: makeReview({
        state: 'approved',
        approvedBy: 'u1',
        approvalNote: 'checked the gateway'
      })
    })
    const done = screen.getByTestId('outcome-approved')
    expect(done).toHaveTextContent('Review approved by you. This is not a release sign-off.')
    expect(done).toHaveTextContent('checked the gateway')
    expect(screen.queryByTestId('outcome-approve')).toBeNull()
    cleanup()
    show({ review: makeReview({ state: 'approved', approvedBy: 'other' }) })
    expect(screen.getByTestId('outcome-approved')).toHaveTextContent('approved by a teammate')
  })

  it('counts what was sent back and opens the dialog', () => {
    const p = show({ sentBackCount: 3 })
    const btn = screen.getByTestId('outcome-request-changes')
    expect(btn).toHaveTextContent('Request changes (3)')
    fireEvent.click(btn)
    expect(p.onRequestChanges).toHaveBeenCalled()
  })

  it('does not claim the work is safe to release anywhere', () => {
    show({ review: makeReview({ state: 'approved', approvedBy: 'u1' }) })
    expect(document.body.textContent).not.toMatch(/safe to release|ready to ship|requirements met/i)
  })
})
