// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { StepApprovalPanel } from '../StepApprovalPanel'
import { useStepApprovals, type StepApproval } from '../../../hooks/useStepApprovals'

vi.mock('../../../hooks/useStepApprovals', () => ({ useStepApprovals: vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const pending: StepApproval = {
  id: 'a1',
  executionId: 'e1',
  stepId: 'approve_g1',
  projectId: 'p1',
  kind: 'tdd_agent',
  subject: { runId: 'run-1', gate: 'G1', role: 'BA' },
  decision: { srs: 'ready' },
  decisionHash: 'h1',
  status: 'pending',
  createdAt: '2026-10-14T00:00:00Z'
}

describe('StepApprovalPanel', () => {
  const load = vi.fn()
  const resolve = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useStepApprovals).mockReturnValue({
      approvals: [pending],
      unavailable: false,
      refresh: vi.fn(),
      load,
      resolve
    })
  })

  it('renders nothing when no step waits', () => {
    vi.mocked(useStepApprovals).mockReturnValue({
      approvals: [],
      unavailable: false,
      refresh: vi.fn(),
      load,
      resolve
    })
    const { container } = render(<StepApprovalPanel executionId="e1" active />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the decision and approves with the hash that was shown and a rationale', async () => {
    load.mockResolvedValue({ ...pending, canDecide: true })
    resolve.mockResolvedValue(undefined)
    render(<StepApprovalPanel executionId="e1" active />)
    expect(screen.getByTestId('step-approval-a1')).toHaveTextContent('Gate G1 · BA')
    fireEvent.click(screen.getByTestId('step-approval-review'))
    await waitFor(() =>
      expect(screen.getByTestId('step-approval-decision')).toHaveTextContent('"srs": "ready"')
    )
    const approve = screen.getByTestId('step-approval-approve')
    expect(approve).toBeDisabled()
    fireEvent.change(screen.getByTestId('step-approval-rationale'), {
      target: { value: 'reviewed' }
    })
    expect(approve).not.toBeDisabled()
    fireEvent.click(approve)
    await waitFor(() =>
      expect(resolve).toHaveBeenCalledWith(
        expect.objectContaining({ decisionHash: 'h1' }),
        'approved',
        'reviewed'
      )
    )
  })

  it('does not let a role without the right decide', async () => {
    load.mockResolvedValue({ ...pending, canDecide: false })
    render(<StepApprovalPanel executionId="e1" active />)
    fireEvent.click(screen.getByTestId('step-approval-review'))
    await waitFor(() => expect(screen.getByTestId('step-approval-readonly')).toBeInTheDocument())
    fireEvent.change(screen.getByTestId('step-approval-rationale'), { target: { value: 'x' } })
    expect(screen.getByTestId('step-approval-approve')).toBeDisabled()
    expect(screen.getByTestId('step-approval-reject')).toBeDisabled()
  })
})
