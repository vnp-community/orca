// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const actions = {
  classify: vi.fn(),
  confirmType: vi.fn(),
  changeType: vi.fn(),
  returnToBacklog: vi.fn(),
  reopen: vi.fn(),
  cancel: vi.fn(),
  spawnChild: vi.fn(),
  generatePlan: vi.fn(),
  startPhase: vi.fn()
}
vi.mock('../../hooks/useRequestActions', () => ({ useRequestActions: () => actions }))
vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }) }))

import { useAppStore } from '@/store'
import { resetRequestCooldowns, startRequestCooldown } from '../../runtime/request-rate-limit'
import { TypeConfirmationCard } from './TypeConfirmationCard'
import type { OrcaRequest } from '../../../../shared/request-types'

const req = (over: Partial<OrcaRequest> = {}): OrcaRequest => ({
  id: 'r1',
  projectId: 'p',
  number: 3,
  title: 'T',
  type: 'unknown',
  status: 'awaiting_type_confirmation',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: new Date().toISOString(),
  ...over
})

beforeEach(() => {
  Object.values(actions).forEach((f) => f.mockReset().mockResolvedValue({ ok: true, value: {} }))
  resetRequestCooldowns()
  useAppStore.setState({ currentUser: { id: 'u1', role: 'member' } } as never)
})
afterEach(cleanup)

describe('TypeConfirmationCard AI setup (TASK-FE-E2E-005)', () => {
  it.each([
    ['REQUEST_AI_NOT_CONFIGURED', 'not set up'],
    ['REQUEST_AI_SESSION_EXPIRED', 'has expired'],
    ['REQUEST_AI_QUOTA_EXHAUSTED', 'quota'],
    ['REQUEST_AI_AGENT_UNAVAILABLE', 'No dev server'],
    ['no dev server connected', 'No dev server']
  ])('shows the banner for classificationReason %s instead of the raw reason', (reason, title) => {
    render(
      <TypeConfirmationCard request={req({ classificationReason: reason })} onChanged={vi.fn()} />
    )
    const banner = screen.getByTestId('request-ai-setup-banner')
    expect(banner.textContent).toContain(title)
    expect(screen.queryByText(reason)).toBeNull()
    expect(screen.getByRole('button', { name: 'Confirm type' })).toBeDisabled()
  })

  it('tells the person to run claude auth login and shows the code', () => {
    render(
      <TypeConfirmationCard
        request={req({ classificationReason: 'REQUEST_AI_NOT_CONFIGURED' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByText('claude auth login').tagName).toBe('CODE')
    expect(screen.getByTestId('request-ai-setup-code').textContent).toContain(
      'REQUEST_AI_NOT_CONFIGURED'
    )
  })

  it('points non-admins to their administrator and admins to the dev server settings', () => {
    const { unmount } = render(
      <TypeConfirmationCard
        request={req({ classificationReason: 'REQUEST_AI_NOT_CONFIGURED' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByTestId('request-ai-setup-ask-admin')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open dev server settings' })).toBeNull()
    unmount()
    useAppStore.setState({ currentUser: { id: 'u1', role: 'admin' } } as never)
    render(
      <TypeConfirmationCard
        request={req({ classificationReason: 'REQUEST_AI_NOT_CONFIGURED' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: 'Open dev server settings' })).toBeInTheDocument()
    expect(screen.queryByTestId('request-ai-setup-ask-admin')).toBeNull()
  })

  it('retries the classification from the banner', async () => {
    render(
      <TypeConfirmationCard
        request={req({ classificationReason: 'REQUEST_AI_NOT_CONFIGURED' })}
        onChanged={vi.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(actions.classify).toHaveBeenCalledWith('r1'))
  })

  it('does not show the banner for a normal AI rationale or when AI works', () => {
    render(
      <TypeConfirmationCard
        request={req({
          type: 'bug',
          confidence: 0.9,
          classificationReason: 'The stack trace points at a regression.'
        })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.queryByTestId('request-ai-setup-banner')).toBeNull()
    expect(screen.getByText('The stack trace points at a regression.')).toBeInTheDocument()
  })

  it('explains a generic classifier failure in plain words', () => {
    render(
      <TypeConfirmationCard
        request={req({ classificationReason: 'classifier error' })}
        onChanged={vi.fn()}
      />
    )
    expect(screen.getByTestId('classification-failed').textContent).toContain('could not classify')
  })
})

describe('TypeConfirmationCard throttling (TASK-FE-E2E-006)', () => {
  it('a short throttle does not claim the 5-attempt limit was reached', async () => {
    actions.classify.mockResolvedValue({
      ok: false,
      error: {
        kind: 'rate_limited',
        code: 'runtime_error',
        backendCode: 'REQUEST_RATE_LIMITED',
        message: 'x'
      }
    })
    render(<TypeConfirmationCard request={req({ type: 'bug' })} onChanged={vi.fn()} />)
    fireEvent.click(screen.getByText('Reclassify'))
    await waitFor(() => expect(actions.classify).toHaveBeenCalled())
    expect(screen.queryByText(/limit reached/)).toBeNull()
    expect(screen.getByText('Reclassify')).toBeEnabled()
  })

  it('locks the AI and write buttons while their cooldown runs, then frees them', async () => {
    vi.useFakeTimers()
    render(<TypeConfirmationCard request={req({ type: 'bug' })} onChanged={vi.fn()} />)
    expect(screen.getByText('Reclassify')).toBeEnabled()
    act(() => startRequestCooldown('ai', 1500))
    expect(screen.getByText('Reclassify')).toBeDisabled()
    expect(screen.getByText('Confirm type')).toBeEnabled()
    act(() => startRequestCooldown('write', 800))
    expect(screen.getByText('Confirm type')).toBeDisabled()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(900)
    })
    expect(screen.getByText('Confirm type')).toBeEnabled()
    expect(screen.getByText('Reclassify')).toBeDisabled()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(700)
    })
    expect(screen.getByText('Reclassify')).toBeEnabled()
    vi.useRealTimers()
  })
})
