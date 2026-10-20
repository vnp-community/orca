// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const toastWarning = vi.fn()
vi.mock('sonner', () => ({ toast: { warning: (...a: unknown[]) => toastWarning(...a) } }))

import { PlanRunAllButton } from './PlanRunAllButton'
import type { PlanDecisionOutcome } from '../../../hooks/usePlanDecision'

function setup(
  overrides: Partial<React.ComponentProps<typeof PlanRunAllButton>> = {},
  outcome: PlanDecisionOutcome | Promise<PlanDecisionOutcome> = { ok: true, runAllApplied: true }
) {
  const onConfirm = vi.fn(() => Promise.resolve(outcome))
  render(
    <PlanRunAllButton
      stepCount={7}
      mentionsPreDeploy={false}
      locked={false}
      onConfirm={onConfirm}
      {...overrides}
    />
  )
  return { onConfirm }
}

beforeEach(() => {
  toastWarning.mockReset()
})
afterEach(cleanup)

describe('PlanRunAllButton', () => {
  it('opens a dialog that names the step count and what it commits to', () => {
    const { onConfirm } = setup()
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAccessibleName('Run all 7 steps without asking again?')
    expect(dialog).toHaveTextContent('using your access')
    expect(dialog).toHaveTextContent('the agent needs information from you')
    expect(dialog).toHaveTextContent('goes back to the backlog')
    expect(dialog).toHaveTextContent('drifts from the plan and needs a review')
    expect(dialog).toHaveTextContent('You can stop the run at any time.')
    expect(dialog).not.toHaveTextContent('pre-deploy')
    expect(dialog).toHaveAccessibleDescription(/Orca starts each step/)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('adds the pre-deploy stop only for flows that have that gate', () => {
    setup({ mentionsPreDeploy: true })
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    expect(screen.getByRole('dialog')).toHaveTextContent(
      'needs a review, or it reaches the pre-deploy approval. You can stop'
    )
  })

  it('Not now and Escape close without sending anything', async () => {
    const { onConfirm } = setup()
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-cancel'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('gives focus back to the button that opened the dialog', async () => {
    setup()
    const opener = screen.getByTestId('plan-approve-run-all')
    opener.focus()
    fireEvent.click(opener)
    fireEvent.click(screen.getByTestId('run-all-cancel'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await waitFor(() => expect(opener).toHaveFocus())
  })

  it('confirm sends once, locks both buttons meanwhile and closes when done', async () => {
    let finish: (o: PlanDecisionOutcome) => void = () => {}
    const { onConfirm } = setup({}, new Promise<PlanDecisionOutcome>((r) => (finish = r)))
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('run-all-confirm')).toBeDisabled()
    expect(screen.getByTestId('run-all-cancel')).toBeDisabled()
    // Why: while the approval is in flight the dialog cannot be dismissed.
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await act(async () => finish({ ok: true, runAllApplied: true }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(toastWarning).not.toHaveBeenCalled()
  })

  it('warns when the server did not run the plan on its own', async () => {
    setup({}, { ok: true, runAllApplied: false })
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    await waitFor(() =>
      expect(toastWarning).toHaveBeenCalledWith(
        "This server didn't run the plan on its own. Approve each step as usual."
      )
    )
  })

  it('closes without a warning when the approval failed (the bar shows the reason)', async () => {
    setup({}, { ok: false, error: { kind: 'unknown', code: 'x', message: 'x' } })
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(toastWarning).not.toHaveBeenCalled()
  })

  it('is locked like the main Approve button, and says why when blocked', () => {
    const { onConfirm } = setup({ locked: true })
    expect(screen.getByTestId('plan-approve-run-all')).toBeDisabled()
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onConfirm).not.toHaveBeenCalled()
    cleanup()

    setup({ blockedReason: 'No decision recorded for the chosen option yet' })
    const button = screen.getByTestId('plan-approve-run-all')
    expect(button).toBeDisabled()
    expect(button).toHaveAttribute('title', 'No decision recorded for the chosen option yet')
  })
})
