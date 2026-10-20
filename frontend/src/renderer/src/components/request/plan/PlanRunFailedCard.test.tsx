// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PlanRunFailedCard } from './PlanRunFailedCard'
import { startRequestCooldown, resetRequestCooldowns } from '../../../runtime/request-rate-limit'

afterEach(() => {
  cleanup()
  resetRequestCooldowns()
})

describe('PlanRunFailedCard', () => {
  it.each([
    ['REQUEST_AI_COMPLETE_TIMEOUT', 'took too long'],
    ['REQUEST_OPENSPEC_TIMEOUT', 'took too long'],
    ['REQUEST_SOLUTION_RUN_INTERRUPTED', 'interrupted'],
    ['REQUEST_PLAN_AI_INVALID_JSON', 'not a valid plan'],
    ['REQUEST_OPENSPEC_INVALID_OUTPUT', 'not a valid plan'],
    ['REQUEST_ANALYSIS_BUSY', 'busy'],
    ['REQUEST_PLAN_COMMIT_FAILED', 'could not be saved'],
    ['REQUEST_SOMETHING_NEW', 'Something went wrong']
  ])('gives the advice for %s', (code, advice) => {
    render(<PlanRunFailedCard code={code} onRetry={vi.fn()} />)
    expect(screen.getByTestId('plan-run-failed')).toHaveTextContent(
      `Plan generation failed (${code})`
    )
    expect(screen.getByTestId('plan-run-failed-hint')).toHaveTextContent(advice)
    expect(screen.queryByTestId('request-ai-setup-banner')).toBeNull()
  })

  it('shows how to fix an AI setup problem instead of the one-line advice', () => {
    render(<PlanRunFailedCard code="REQUEST_AI_NOT_CONFIGURED" onRetry={vi.fn()} />)
    expect(screen.getByTestId('request-ai-setup-banner')).toHaveAttribute(
      'data-ai-code',
      'REQUEST_AI_NOT_CONFIGURED'
    )
    expect(screen.queryByTestId('plan-run-failed-hint')).toBeNull()
    expect(screen.getAllByRole('button', { name: /retry|try again/i })).toHaveLength(1)
  })

  it('calls onRetry once per press', () => {
    const onRetry = vi.fn()
    render(<PlanRunFailedCard code="REQUEST_PLAN_AI_INVALID_JSON" onRetry={onRetry} />)
    fireEvent.click(screen.getByTestId('plan-run-retry'))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('locks Retry while retrying', () => {
    const onRetry = vi.fn()
    render(<PlanRunFailedCard code="X" onRetry={onRetry} retrying />)
    expect(screen.getByTestId('plan-run-retry')).toBeDisabled()
    fireEvent.click(screen.getByTestId('plan-run-retry'))
    expect(onRetry).not.toHaveBeenCalled()
  })

  it('locks Retry while the AI limiter cools down', () => {
    render(<PlanRunFailedCard code="X" onRetry={vi.fn()} />)
    expect(screen.getByTestId('plan-run-retry')).toBeEnabled()
    act(() => startRequestCooldown('ai', 5000))
    expect(screen.getByTestId('plan-run-retry')).toBeDisabled()
  })

  it('shows the failure of Retry under the button', () => {
    render(<PlanRunFailedCard code="X" onRetry={vi.fn()} retryError="Could not retry" />)
    expect(screen.getByTestId('plan-run-retry-error')).toHaveTextContent('Could not retry')
  })

  it('shows no retry error when there is none', () => {
    render(<PlanRunFailedCard code="X" onRetry={vi.fn()} />)
    expect(screen.queryByTestId('plan-run-retry-error')).toBeNull()
  })
})
