// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
const impactRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (m: string, ...a: unknown[]) =>
    String(m).startsWith('impact.') ? impactRpc(m, ...a) : callRequestRpc(m, ...a)
}))
vi.mock('@/lib/shortcut-platform', () => ({ getShortcutPlatform: () => 'linux' }))
vi.mock('sonner', () => ({ toast: { warning: vi.fn() } }))

import { useAppStore } from '../../../store'
import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { PlanApprovalBar } from './PlanApprovalBar'
import { PlanDecisionButtons } from './PlanDecisionButtons'
import { backendFlowOf, planApproval } from './plan-test-fixtures'
import { fallbackRequestFlow } from '../../../../../shared/request-flow-view'
import type { RequestFlowView } from '../../../../../shared/request-execution-types'
import type { Approval, OrcaRequest } from '../../../../../shared/request-types'

const settled = vi.fn()

type HarnessProps = {
  approval?: Approval
  flow?: RequestFlowView | null
  executionMode?: OrcaRequest['executionMode']
  stepCount?: number
  approveBlockedReason?: string | null
}

function Harness({
  approval = planApproval('ap1'),
  flow = backendFlowOf('change_request'),
  executionMode = 'step_by_step',
  stepCount = 3,
  approveBlockedReason
}: HarnessProps): React.JSX.Element {
  const d = usePlanDecision({ id: 'r1' }, settled)
  return (
    <PlanApprovalBar
      approval={approval}
      decision={d}
      approveBlockedReason={approveBlockedReason}
      runAll={{ request: { executionMode }, flow, stepCount }}
    />
  )
}

const pinnedError = {
  ok: false,
  error: {
    kind: 'unknown',
    code: 'runtime_error',
    message: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED: pinned',
    backendCode: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED'
  }
}

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: { executionMode: 'continuous' } })
  impactRpc.mockReset()
  impactRpc.mockResolvedValue({
    ok: false,
    error: { kind: 'unsupported', code: 'method_not_found', message: 'm' }
  })
  settled.mockReset()
  useAppStore.setState({ requestFlowFeatures: new Set(['approval.runAll']) })
})
afterEach(cleanup)

describe('PlanApprovalBar run-all choice', () => {
  it('shows both buttons for a change_request with phases, in Tab order Approve, Run all, Reject', () => {
    render(<Harness />)
    const buttons = screen
      .getAllByRole('button')
      .map((b) => b.getAttribute('data-testid'))
      .filter((id) => id?.startsWith('plan-'))
    expect(buttons.slice(0, 3)).toEqual(['plan-approve', 'plan-approve-run-all', 'plan-reject'])
    expect(screen.getByTestId('plan-approve-run-all')).toHaveTextContent(
      'Approve and run all steps'
    )
  })

  it('also shows for a task-list approval', () => {
    render(<Harness approval={planApproval('ap2', { rawSubjectType: 'task_list' })} />)
    expect(screen.getByTestId('plan-approve-run-all')).toBeInTheDocument()
  })

  it.each<[string, () => void, HarnessProps]>([
    [
      'the gateway has no approval.runAll flag',
      () => useAppStore.setState({ requestFlowFeatures: new Set() }),
      {}
    ],
    ['the flow has no phases', () => undefined, { flow: backendFlowOf('docs', 'M') }],
    ['the request already runs continuously', () => undefined, { executionMode: 'continuous' }],
    [
      'only the built-in table answered',
      () => undefined,
      { flow: fallbackRequestFlow('change_request', 'L') }
    ],
    ['the flow has not loaded', () => undefined, { flow: null }],
    ['the plan has no steps to count', () => undefined, { stepCount: 0 }]
  ])('hides it when %s', (_why, arrange, props) => {
    arrange()
    render(<Harness {...props} />)
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
  })

  it('hides it when the risk rule hides Approve, keeping Reject', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'forbidden',
        code: 'runtime_error',
        message: 'REQUEST_RISK_APPROVER_NOT_ALLOWED: another approver is needed',
        backendCode: 'REQUEST_RISK_APPROVER_NOT_ALLOWED'
      }
    })
    render(<Harness />)
    fireEvent.click(screen.getByTestId('plan-approve'))
    await waitFor(() => expect(screen.queryByTestId('plan-approve')).toBeNull())
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    expect(screen.getByTestId('plan-reject')).toBeInTheDocument()
  })

  it('the main button still approves without an execution mode', async () => {
    render(<Harness />)
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

  it('opening and cancelling the dialog sends nothing; confirming sends executionMode once', async () => {
    render(<Harness />)
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Run all 3 steps without asking again?')
    fireEvent.click(screen.getByTestId('run-all-cancel'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(callRequestRpc).not.toHaveBeenCalled()

    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    await waitFor(() => expect(settled).toHaveBeenCalledTimes(1))
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).toHaveBeenCalledWith('approval.approve', {
      id: 'ap1',
      approvalId: 'ap1',
      expectedVersion: 2,
      expectedDigest: 'dg',
      comment: undefined,
      executionMode: 'continuous'
    })
  })

  it('locks the choice while the main Approve is in flight (one call for both buttons)', async () => {
    let release: (v: unknown) => void = () => {}
    callRequestRpc.mockReturnValue(new Promise((resolve) => (release = resolve)))
    render(<Harness />)
    fireEvent.click(screen.getByTestId('plan-approve'))
    await waitFor(() => expect(screen.getByTestId('plan-approve-run-all')).toBeDisabled())
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    release({ ok: true, value: {} })
    await waitFor(() => expect(settled).toHaveBeenCalled())
  })

  it('is locked with the reason while a decision is still missing', () => {
    render(<Harness approveBlockedReason="No decision recorded for the chosen option yet" />)
    expect(screen.getByTestId('plan-approve-run-all')).toBeDisabled()
    expect(screen.getByTestId('plan-approve-run-all')).toHaveAttribute(
      'title',
      'No decision recorded for the chosen option yet'
    )
  })

  it('explains a pinned request and keeps the choice hidden afterwards', async () => {
    callRequestRpc.mockResolvedValueOnce(pinnedError)
    render(<Harness />)
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    expect(await screen.findByTestId('plan-decision-error')).toHaveTextContent(
      'already runs step by step'
    )
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    expect(screen.getByTestId('plan-approve')).not.toBeDisabled()
    expect(settled).not.toHaveBeenCalled()
  })
})

describe('PlanDecisionButtons run-all', () => {
  it('sends the risk-gate fields together with the execution mode', async () => {
    function Buttons(): React.JSX.Element {
      const d = usePlanDecision({ id: 'r1' }, settled)
      return (
        <PlanDecisionButtons
          approval={planApproval('ap1')}
          decision={d}
          approveLabel="Approve plan"
          rejectLabel="Reject"
          rejectTitle="Reject plan"
          testIdPrefix="plan"
          approveExtras={{ viewedImpactDigest: 'imp', acceptedFindingIds: ['f1', 'f2'] }}
          runAll={{ stepCount: 2, mentionsPreDeploy: false }}
        />
      )
    }
    render(<Buttons />)
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(callRequestRpc.mock.calls[0][1]).toMatchObject({
      viewedImpactDigest: 'imp',
      acceptedFindingIds: ['f1', 'f2'],
      executionMode: 'continuous'
    })
  })

  it('shows no run-all button unless asked to', () => {
    function Buttons(): React.JSX.Element {
      const d = usePlanDecision({ id: 'r1' }, settled)
      return (
        <PlanDecisionButtons
          approval={planApproval('ap1')}
          decision={d}
          approveLabel="Approve plan"
          rejectLabel="Reject"
          rejectTitle="Reject plan"
          testIdPrefix="plan"
        />
      )
    }
    render(<Buttons />)
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
  })
})
