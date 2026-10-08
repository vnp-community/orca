// @vitest-environment happy-dom
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({ available: true }))
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))
vi.mock('./use-review-report', () => ({
  useReviewReport: () => ({ available: state.available, loading: false, fetchReport: vi.fn() })
}))

import { ReviewReportMenu } from './ReviewReportMenu'

afterEach(cleanup)

const props = { worktreeId: 'wt', projectId: 'p', provider: 'github' as const, repoSlug: 'orca' }

describe('ReviewReportMenu', () => {
  it('renders nothing while the report is unavailable (quality flag off)', () => {
    state.available = false
    const { container } = render(<ReviewReportMenu {...props} />)
    expect(container.innerHTML).toBe('')
  })

  it('renders an enabled trigger when available', () => {
    state.available = true
    render(<ReviewReportMenu {...props} />)
    const trigger = screen.getByRole('button', { name: /export report/i })
    expect((trigger as HTMLButtonElement).disabled).toBe(false)
  })
})
