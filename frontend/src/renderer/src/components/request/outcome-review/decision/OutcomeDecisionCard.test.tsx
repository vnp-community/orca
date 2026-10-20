// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { OutcomeDecisionCard } from './OutcomeDecisionCard'
import { makeItem } from '../outcome-review.fixture'
import type { DecisionResult } from './use-outcome-decisions'
import type { ReviewDecision } from '../../../../../../shared/outcome-review-types'

afterEach(cleanup)

const ok: DecisionResult = { ok: true }
const decision = (over: Partial<ReviewDecision> = {}): ReviewDecision => ({
  decisionId: 'd1',
  reviewId: 'v1',
  itemKey: 'k1',
  decision: 'accept_exception',
  note: 'ADR-7 allows it',
  decidedBy: 'u1',
  decidedAt: '',
  active: true,
  orphaned: false,
  ...over
})

function show(over: Partial<React.ComponentProps<typeof OutcomeDecisionCard>> = {}) {
  const onDecide = vi.fn(async (): Promise<DecisionResult> => ok)
  const utils = render(
    <OutcomeDecisionCard
      item={makeItem({ key: 'k1', needsDecision: true, origin: 'agent' })}
      decision={null}
      currentUserId="u1"
      busy={false}
      onDecide={onDecide}
      {...over}
    />
  )
  return { onDecide, ...utils }
}

describe('OutcomeDecisionCard', () => {
  it('offers accepting, sending back, and, for what an agent only claimed, dismissing', () => {
    show()
    expect(screen.getByRole('button', { name: 'Accept as exception' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send back' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dismiss claim' })).toBeInTheDocument()
    cleanup()
    show({ item: makeItem({ key: 'k1', needsDecision: true, origin: 'fact' }) })
    expect(screen.queryByRole('button', { name: 'Dismiss claim' })).toBeNull()
  })

  it('accepting needs a note and the error appears beside the field, not in a toast', async () => {
    const { onDecide } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Accept as exception' }))
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    const error = screen.getByTestId('outcome-decision-error')
    expect(error).toHaveTextContent('A note is required.')
    const field = screen.getByLabelText('Why (required)')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(field.getAttribute('aria-describedby')).toBe(error.id)
    expect(onDecide).not.toHaveBeenCalled()
    fireEvent.change(field, { target: { value: '   ' } })
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    expect(onDecide).not.toHaveBeenCalled()
    fireEvent.change(field, { target: { value: 'ADR-7 allows it' } })
    expect(screen.queryByTestId('outcome-decision-error')).toBeNull()
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    await waitFor(() =>
      expect(onDecide).toHaveBeenCalledWith('accept_exception', 'ADR-7 allows it')
    )
  })

  it('refuses a note longer than the server accepts before sending it', () => {
    const { onDecide } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Accept as exception' }))
    fireEvent.change(screen.getByLabelText('Why (required)'), {
      target: { value: 'x'.repeat(501) }
    })
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    expect(screen.getByTestId('outcome-decision-error')).toHaveTextContent(
      'At most 500 characters.'
    )
    expect(onDecide).not.toHaveBeenCalled()
  })

  it('sending back needs no note', async () => {
    const { onDecide } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Send back' }))
    await waitFor(() => expect(onDecide).toHaveBeenCalledWith('send_back', ''))
  })

  it('dismissing a claim needs a note too', async () => {
    const { onDecide } = show()
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss claim' }))
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    expect(screen.getByTestId('outcome-decision-error')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Why (required)'), {
      target: { value: 'the edge does not exist' }
    })
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    await waitFor(() => expect(onDecide).toHaveBeenCalledWith('dismiss', 'the edge does not exist'))
  })

  it('says the sentence of the server code when it refuses, and keeps the form open', async () => {
    const onDecide = vi.fn(
      async (): Promise<DecisionResult> => ({
        ok: false,
        error: { code: 'REQUEST_REVIEW_STALE_REVISION', message: 'RAW' }
      })
    )
    show({ onDecide })
    fireEvent.click(screen.getByRole('button', { name: 'Accept as exception' }))
    fireEvent.change(screen.getByLabelText('Why (required)'), { target: { value: 'because' } })
    fireEvent.click(screen.getByTestId('outcome-decision-confirm'))
    await waitFor(() =>
      expect(screen.getByTestId('outcome-decision-error')).toHaveTextContent(
        'Someone changed this review. It was reloaded; try again.'
      )
    )
    expect(screen.getByTestId('outcome-decision-error')).not.toHaveTextContent('RAW')
    expect(screen.getByLabelText('Why (required)')).toHaveValue('because')
  })

  it('shows a decision already made, with who made it, and lets the person change it', () => {
    show({ decision: decision() })
    expect(screen.getByTestId('outcome-decision-made')).toHaveTextContent(
      'Accepted as an exception by you'
    )
    expect(screen.getByTestId('outcome-decision-made')).toHaveTextContent('ADR-7 allows it')
    fireEvent.click(screen.getByRole('button', { name: 'Change decision' }))
    expect(screen.getByRole('button', { name: 'Accept as exception' })).toBeInTheDocument()
    cleanup()
    show({ decision: decision({ decidedBy: 'someone-else' }) })
    expect(screen.getByTestId('outcome-decision-made')).toHaveTextContent('by a teammate')
  })

  it('a sent-back item points at its follow-up Request', () => {
    const onOpenChild = vi.fn()
    show({
      decision: decision({ decision: 'send_back', note: undefined, childRequestId: 'child-1' }),
      onOpenChild
    })
    expect(screen.getByTestId('outcome-decision-made')).toHaveTextContent('Sent back by you')
    expect(screen.getByTestId('outcome-decision-made')).toHaveTextContent(
      'Sent back in a follow-up request'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Open it' }))
    expect(onOpenChild).toHaveBeenCalledWith('child-1')
  })

  it('cannot be pressed twice while a write is going', () => {
    show({ busy: true })
    expect(screen.getByRole('button', { name: 'Send back' })).toBeDisabled()
  })
})
