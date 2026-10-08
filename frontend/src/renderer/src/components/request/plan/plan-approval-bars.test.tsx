// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
// Why: the risk gate (036-06) reads impact.* on mount; keep those calls apart so the
// approval/plan assertions below still see only their own RPCs.
const impactRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (m: string, ...a: unknown[]) =>
    String(m).startsWith('impact.') ? impactRpc(m, ...a) : callRequestRpc(m, ...a)
}))
const platform = vi.hoisted(() => ({ value: 'linux' }))
vi.mock('@/lib/shortcut-platform', () => ({ getShortcutPlatform: () => platform.value }))

import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { PlanApprovalBar } from './PlanApprovalBar'
import { PhaseApprovalBar } from './PhaseApprovalBar'
import { PlanGateChips } from './PlanGateChips'
import { attachApprovals } from './plan-approval-model'
import { planApproval, planTask, twoPhaseTree } from './plan-test-fixtures'
import type { Approval } from '../../../../../shared/request-types'

const settled = vi.fn()

function PlanHarness({ approval }: { approval: Approval | null }): React.JSX.Element {
  const d = usePlanDecision({ id: 'r1' }, settled)
  return <PlanApprovalBar approval={approval} decision={d} />
}
function PhaseHarness({
  approval,
  status
}: {
  approval: Approval | null
  status?: 'todo' | 'in_progress'
}): React.JSX.Element {
  const d = usePlanDecision({ id: 'r1' }, settled)
  return (
    <PhaseApprovalBar
      phase={planTask('ph1', { type: 'phase', status: status ?? 'todo' })}
      approval={approval}
      decision={d}
    />
  )
}
function GateHarness({ approvals }: { approvals: Approval[] }): React.JSX.Element {
  const d = usePlanDecision({ id: 'r1' }, settled)
  return <PlanGateChips approvals={attachApprovals(twoPhaseTree(), approvals)} decision={d} />
}

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: {} })
  impactRpc.mockReset()
  impactRpc.mockResolvedValue({
    ok: false,
    error: { kind: 'unsupported', code: 'method_not_found', message: 'm' }
  })
  settled.mockReset()
  platform.value = 'linux'
})
afterEach(cleanup)

describe('PlanApprovalBar', () => {
  it('approve sends id, expectedVersion and expectedDigest', async () => {
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-approve'))
    await waitFor(() => expect(settled).toHaveBeenCalled())
    expect(callRequestRpc).toHaveBeenCalledWith('approval.approve', {
      id: 'ap1',
      approvalId: 'ap1',
      expectedVersion: 2,
      expectedDigest: 'dg',
      comment: undefined
    })
  })

  it('uses the task-list label for task_list approvals', () => {
    render(<PlanHarness approval={planApproval('ap1', { subjectType: 'task_list' as never })} />)
    expect(screen.getByTestId('plan-approve')).toHaveTextContent('Approve task list')
  })

  it('blocks a short reject reason and sends a valid one (Ctrl+Enter)', async () => {
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-reject'))
    const box = await screen.findByRole('textbox')
    fireEvent.change(box, { target: { value: 'too short' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    expect(screen.getByRole('button', { name: 'Reject' })).toBeDisabled()
    expect(callRequestRpc).not.toHaveBeenCalled()

    fireEvent.change(box, { target: { value: 'The scope is far too large.' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(callRequestRpc.mock.calls[0][0]).toBe('approval.reject')
    expect(callRequestRpc.mock.calls[0][1]).toMatchObject({
      id: 'ap1',
      approvalId: 'ap1',
      comment: 'The scope is far too large.'
    })
  })

  it('uses metaKey on Mac', async () => {
    platform.value = 'darwin'
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-reject'))
    const box = await screen.findByRole('textbox')
    fireEvent.change(box, { target: { value: 'The scope is far too large.' } })
    fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    expect(callRequestRpc).not.toHaveBeenCalled()
    fireEvent.keyDown(box, { key: 'Enter', metaKey: true })
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
  })

  it('regenerate calls request.generatePlan {id, mode: propose}', async () => {
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-regenerate'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', {
        id: 'r1',
        mode: 'propose'
      })
    )
  })

  it('shows the not-approver message and locks approve on forbidden', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'forbidden',
        code: 'APPROVAL_NOT_APPROVER',
        message: 'APPROVAL_NOT_APPROVER: nope'
      }
    })
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-approve'))
    expect(await screen.findByTestId('plan-decision-error')).toHaveTextContent(
      'You are not allowed to approve this.'
    )
    expect(screen.getByTestId('plan-approve')).toBeDisabled()
  })

  it('is read-only once approved and shows the reason after rejection', () => {
    const { container, rerender } = render(
      <PlanHarness approval={planApproval('ap1', { status: 'approved' })} />
    )
    expect(container).toBeEmptyDOMElement()
    rerender(
      <PlanHarness
        approval={planApproval('ap1', { status: 'rejected', comment: 'Needs smaller scope' })}
      />
    )
    expect(screen.getByTestId('plan-rejected-notice')).toHaveTextContent('Needs smaller scope')
    expect(screen.getByTestId('plan-regenerate')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-approve')).toBeNull()
  })

  it('ignores a second click while approve is in flight', async () => {
    let resolve!: (v: unknown) => void
    callRequestRpc.mockReturnValue(new Promise((r) => (resolve = r)))
    render(<PlanHarness approval={planApproval('ap1')} />)
    fireEvent.click(screen.getByTestId('plan-approve'))
    fireEvent.click(screen.getByTestId('plan-approve'))
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    resolve({ ok: true, value: {} })
    await waitFor(() => expect(settled).toHaveBeenCalled())
  })
})

describe('PhaseApprovalBar', () => {
  it('start phase is locked with a tooltip until the phase is approved', () => {
    render(
      <PhaseHarness approval={planApproval('ap2', { subjectType: 'phase', subjectId: 'ph1' })} />
    )
    expect(screen.getByTestId('phase-ph1-start')).toBeDisabled()
    expect(screen.getByTestId('phase-ph1-start').parentElement).toHaveAttribute(
      'title',
      'Phase is not approved yet'
    )
  })

  it('approved phase starts via request.startPhase {id, phaseTaskId}', async () => {
    render(
      <PhaseHarness
        approval={planApproval('ap2', {
          subjectType: 'phase',
          subjectId: 'ph1',
          status: 'approved'
        })}
      />
    )
    fireEvent.click(screen.getByTestId('phase-ph1-start'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.startPhase', {
        id: 'r1',
        phaseTaskId: 'ph1'
      })
    )
  })

  it('hides start once the phase is running', () => {
    render(
      <PhaseHarness
        status="in_progress"
        approval={planApproval('ap2', {
          subjectType: 'phase',
          subjectId: 'ph1',
          status: 'approved'
        })}
      />
    )
    expect(screen.queryByTestId('phase-ph1-start')).toBeNull()
  })

  it('shows the backend message for REQUEST_TRANSITION_NOT_ALLOWED', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'REQUEST_TRANSITION_NOT_ALLOWED',
        message: 'REQUEST_TRANSITION_NOT_ALLOWED: phase 1 is not done'
      }
    })
    render(
      <PhaseHarness
        approval={planApproval('ap2', {
          subjectType: 'phase',
          subjectId: 'ph1',
          status: 'approved'
        })}
      />
    )
    fireEvent.click(screen.getByTestId('phase-ph1-start'))
    expect(await screen.findByTestId('phase-ph1-error')).toHaveTextContent('phase 1 is not done')
  })

  it('approve targets the phase approval and rejected phases show the notice', async () => {
    const { rerender } = render(
      <PhaseHarness approval={planApproval('ap2', { subjectType: 'phase', subjectId: 'ph1' })} />
    )
    fireEvent.click(screen.getByTestId('phase-ph1-approve'))
    await waitFor(() =>
      expect(callRequestRpc.mock.calls[0][1]).toMatchObject({ approvalId: 'ap2' })
    )
    rerender(
      <PhaseHarness
        approval={planApproval('ap2', {
          subjectType: 'phase',
          subjectId: 'ph1',
          status: 'rejected'
        })}
      />
    )
    expect(screen.getByTestId('phase-ph1-rejected')).toHaveTextContent('may return to planning')
  })
})

describe('PlanGateChips', () => {
  it('shows plan / phase / pre_deploy chips and actions for pending pre_deploy', async () => {
    render(
      <GateHarness
        approvals={[
          planApproval('a1', { status: 'approved' }),
          planApproval('a2', { subjectType: 'phase', subjectId: 'ph1', status: 'approved' }),
          planApproval('a3', { subjectType: 'pre_deploy', subjectId: 'x' })
        ]}
      />
    )
    expect(screen.getByTestId('gate-chip-plan')).toBeInTheDocument()
    expect(screen.getByTestId('gate-chip-phase')).toHaveTextContent('1/1')
    expect(screen.getByTestId('gate-chip-pre-deploy-a3')).toBeInTheDocument()
    fireEvent.click(
      screen.getByTestId('pre-deploy-a3-approve-approve'.replace('-approve-approve', '-approve'))
    )
    await waitFor(() => expect(callRequestRpc.mock.calls[0][1]).toMatchObject({ approvalId: 'a3' }))
  })

  it('renders nothing without any gate', () => {
    const { container } = render(<GateHarness approvals={[]} />)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('risk gate on plan / phase approval (FE-REQ-TASK-036-06)', () => {
  const summary = (level: string) => ({
    assessment_id: 'as1',
    digest: 'dg1',
    level,
    status: 'ready',
    mode: 'enforce'
  })
  function serveImpact(level: string, findings: unknown[] = []) {
    impactRpc.mockImplementation(async (m: string) => {
      if (m === 'impact.get') {
        return { ok: true, value: summary(level) }
      }
      if (m === 'impact.findings') {
        return { ok: true, value: { findings } }
      }
      if (m === 'impact.accept') {
        return { ok: true, value: { acceptance: { acceptedBy: 'u1' } } }
      }
      return { ok: false, error: { kind: 'unsupported', code: 'x', message: 'm' } }
    })
  }

  it('plan: medium risk locks Approve until the impact is opened, then sends viewedImpactDigest', async () => {
    serveImpact('medium')
    render(<PlanHarness approval={planApproval('ap1')} />)
    const trigger = await screen.findByTestId('plan-view-impact-trigger')
    expect(impactRpc).toHaveBeenCalledWith('impact.get', {
      subjectType: 'plan',
      subjectId: 'plan1'
    })
    expect(screen.getByTestId('plan-approve')).toBeDisabled()
    fireEvent.click(trigger)
    await waitFor(() => expect(screen.getByTestId('plan-approve')).toBeEnabled())
    fireEvent.click(screen.getByTestId('plan-approve'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith(
        'approval.approve',
        expect.objectContaining({ approvalId: 'ap1', viewedImpactDigest: 'dg1' })
      )
    )
  })

  it('phase: high finding must be accepted with a reason; approve sends acceptedFindingIds', async () => {
    serveImpact('high', [{ id: 'f1', dimension: 'data', level: 'high', title: 'Column removed' }])
    render(
      <PhaseHarness approval={planApproval('ap2', { subjectType: 'phase', subjectId: 'ph1' })} />
    )
    const reason = await screen.findByLabelText('Reason for accepting')
    expect(screen.getByTestId('phase-ph1-approve')).toBeDisabled()
    fireEvent.change(reason, { target: { value: 'rollback rehearsed in staging' } })
    fireEvent.click(screen.getByRole('button', { name: 'Record acceptance' }))
    await waitFor(() => expect(screen.getByTestId('phase-ph1-approve')).toBeEnabled())
    expect(impactRpc).toHaveBeenCalledWith(
      'impact.accept',
      expect.objectContaining({ findingId: 'f1', assessmentDigest: 'dg1' })
    )
    fireEvent.click(screen.getByTestId('phase-ph1-approve'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith(
        'approval.approve',
        expect.objectContaining({ approvalId: 'ap2', acceptedFindingIds: ['f1'] })
      )
    )
  })

  it('REQUEST_RISK_APPROVER_NOT_ALLOWED hides Approve; ACCEPTANCE_REQUIRED reloads the assessment', async () => {
    serveImpact('low')
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'forbidden',
        code: 'REQUEST_RISK_APPROVER_NOT_ALLOWED',
        message: 'REQUEST_RISK_APPROVER_NOT_ALLOWED: team'
      }
    })
    render(<PlanHarness approval={planApproval('ap1')} />)
    await waitFor(() => expect(impactRpc).toHaveBeenCalledWith('impact.get', expect.anything()))
    fireEvent.click(screen.getByTestId('plan-approve'))
    expect(await screen.findByTestId('plan-risk-approver-not-allowed')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-approve')).toBeNull()
    expect(screen.getByTestId('plan-reject')).toBeInTheDocument()
    cleanup()

    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'REQUEST_RISK_ACCEPTANCE_REQUIRED',
        message: 'REQUEST_RISK_ACCEPTANCE_REQUIRED: f1'
      }
    })
    render(<PlanHarness approval={planApproval('ap3')} />)
    await waitFor(() => expect(impactRpc).toHaveBeenCalledWith('impact.get', expect.anything()))
    const before = impactRpc.mock.calls.filter((c) => c[0] === 'impact.get').length
    fireEvent.click(screen.getByTestId('plan-approve'))
    await waitFor(() =>
      expect(impactRpc.mock.calls.filter((c) => c[0] === 'impact.get').length).toBeGreaterThan(
        before
      )
    )
  })

  it('unsupported impact keeps the SOL-021 behaviour (no gate, Approve enabled)', async () => {
    render(<PlanHarness approval={planApproval('ap1')} />)
    await waitFor(() => expect(impactRpc).toHaveBeenCalled())
    expect(screen.queryByTestId('risk-gate-plan')).toBeNull()
    expect(screen.getByTestId('plan-approve')).toBeEnabled()
  })
})
