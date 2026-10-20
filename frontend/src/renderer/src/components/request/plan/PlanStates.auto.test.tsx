// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PlanStates } from './PlanStates'
import type { PlanAutoDisplay } from './plan-auto-display'

afterEach(cleanup)

const writing = (
  o: Partial<Extract<PlanAutoDisplay, { kind: 'writing' }>> = {}
): PlanAutoDisplay => ({
  kind: 'writing',
  attempt: 1,
  maxAttempts: 2,
  waiting: false,
  replan: false,
  starting: false,
  ...o
})

function renderStates(
  autoDisplay?: PlanAutoDisplay,
  o: Partial<Parameters<typeof PlanStates>[0]> = {}
) {
  const props = { onGenerate: vi.fn(), onRetry: vi.fn(), onRetryRun: vi.fn(), ...o }
  render(
    <PlanStates
      state="generating"
      canGenerate
      generating={false}
      autoDisplay={autoDisplay}
      {...props}
    />
  )
  return props
}

describe('PlanStates while Orca writes the plan', () => {
  it('hides Generate and shows the progress', () => {
    renderStates(writing())
    expect(screen.getByTestId('plan-auto-writing')).toHaveTextContent('Orca is writing the plan...')
    expect(screen.queryByTestId('plan-generate')).toBeNull()
    expect(screen.queryByTestId('plan-auto-attempt')).toBeNull()
    expect(screen.getByTestId('plan-generating')).toBeInTheDocument()
  })

  it('names the attempt from the second one on', () => {
    renderStates(writing({ attempt: 2, maxAttempts: 2, waiting: true }))
    expect(screen.getByTestId('plan-auto-attempt')).toHaveTextContent('Attempt 2 of 2')
    expect(screen.getByTestId('plan-auto-waiting')).toHaveTextContent('Trying again shortly')
  })

  it('says "starting" before anything exists', () => {
    renderStates(writing({ starting: true }))
    expect(screen.getByTestId('plan-auto-writing')).toHaveTextContent(
      'Orca is starting the plan...'
    )
    expect(screen.queryByTestId('plan-generate')).toBeNull()
  })
})

describe('PlanStates after a failed run', () => {
  const failed: PlanAutoDisplay = {
    kind: 'failed',
    code: 'REQUEST_PLAN_AI_INVALID_JSON',
    viaServer: true
  }

  it('shows the card with the code, Retry, and Generate as a quiet second choice', () => {
    const props = renderStates(failed)
    expect(screen.getByTestId('plan-run-failed')).toHaveTextContent(
      '(REQUEST_PLAN_AI_INVALID_JSON)'
    )
    const manual = screen.getByTestId('plan-generate')
    expect(manual).toHaveTextContent('Generate manually')
    expect(manual).toHaveAttribute('data-variant', 'ghost')
    fireEvent.click(screen.getByTestId('plan-run-retry'))
    expect(props.onRetryRun).toHaveBeenCalledTimes(1)
    expect(props.onGenerate).not.toHaveBeenCalled()
    fireEvent.click(manual)
    expect(props.onGenerate).toHaveBeenCalledTimes(1)
  })

  it('passes the busy state and the error of Retry to the card', () => {
    renderStates(failed, { retryingRun: true, retryRunError: 'Could not retry' })
    expect(screen.getByTestId('plan-run-retry')).toBeDisabled()
    expect(screen.getByTestId('plan-run-retry-error')).toHaveTextContent('Could not retry')
  })

  it('does not repeat the failure of the same code inline', () => {
    renderStates(failed, {
      generateError: 'The AI answered badly',
      generateErrorCode: 'REQUEST_PLAN_AI_INVALID_JSON'
    })
    expect(screen.queryByTestId('plan-generate-error')).toBeNull()
  })

  it('still shows a refusal to start, which the card knows nothing about', () => {
    renderStates(failed, {
      generateError: 'Too many requests',
      generateErrorCode: 'REQUEST_RATE_LIMITED'
    })
    expect(screen.getByTestId('plan-generate-error')).toHaveTextContent('Too many requests')
  })

  it('shows one AI setup banner when the failed run needs AI setup', () => {
    renderStates(
      { kind: 'failed', code: 'REQUEST_AI_NOT_CONFIGURED', viaServer: false },
      {
        generateAiCode: 'REQUEST_AI_NOT_CONFIGURED',
        generateErrorCode: 'REQUEST_AI_NOT_CONFIGURED'
      }
    )
    expect(screen.getAllByTestId('request-ai-setup-banner')).toHaveLength(1)
  })
})

describe('PlanStates when Orca could not start on its own', () => {
  it('shows the note with the reason and keeps Generate as it was', () => {
    renderStates({ kind: 'skipped', reasonCode: 'REQUEST_PLAN_AI_UNAVAILABLE' })
    expect(screen.getByTestId('plan-auto-skipped')).toHaveTextContent(
      'Orca could not start the plan on its own (no dev server is connected). Generate it when you are ready.'
    )
    const generate = screen.getByTestId('plan-generate')
    expect(generate).toHaveTextContent('Generate plan')
    expect(generate).not.toHaveAttribute('data-variant', 'ghost')
  })

  it.each([
    ['REQUEST_ANALYSIS_NO_REPO_PATH', 'no repository path'],
    ['REQUEST_ENGINE_UNAVAILABLE', 'not available'],
    ['REQUEST_SOMETHING_NEW', 'REQUEST_SOMETHING_NEW']
  ])('explains %s', (reasonCode, text) => {
    renderStates({ kind: 'skipped', reasonCode })
    expect(screen.getByTestId('plan-auto-skipped')).toHaveTextContent(text)
  })
})

describe('PlanStates without anything from the server', () => {
  it.each([undefined, { kind: 'none' } as PlanAutoDisplay])(
    'keeps the old drafting state for %j',
    (display) => {
      renderStates(display)
      expect(screen.getByTestId('plan-generating')).toHaveTextContent('AI is drafting the plan...')
      expect(screen.getByTestId('plan-generate')).toHaveTextContent('Generate plan')
      expect(screen.queryByTestId('plan-auto-writing')).toBeNull()
    }
  )

  it('shows an inline error as before', () => {
    renderStates({ kind: 'none' }, { generateError: 'Boom', generateErrorCode: 'REQUEST_INTERNAL' })
    expect(screen.getByTestId('plan-generate-error')).toHaveTextContent('Boom')
  })

  it('keeps Generate in the empty state', () => {
    render(
      <PlanStates
        state="empty"
        canGenerate
        generating={false}
        onGenerate={vi.fn()}
        onRetry={vi.fn()}
      />
    )
    expect(screen.getByTestId('plan-generate')).toBeInTheDocument()
  })
})
