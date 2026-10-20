// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRuntimeRpc = vi.fn()
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...a: unknown[]) => callRuntimeRpc(...a),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))
vi.mock('../../task/TaskDetail', () => ({ TaskDetail: () => <div /> }))
vi.mock('sonner', () => ({ toast: { warning: vi.fn() } }))

import { useAppStore } from '../../../store'
import { RequestPlanTab } from './RequestPlanTab'
import { backendFlowOf, twoPhaseTree } from './plan-test-fixtures'
import type { OrcaRequest } from '../../../../../shared/request-types'

const req = (o: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    type: 'change_request',
    size: 'L',
    status: 'awaiting_plan_approval',
    planTaskId: 'plan1',
    ...o
  }) as OrcaRequest

const wireApproval = (status: string) => ({
  id: 'ap1',
  requestId: 'r1',
  subjectType: 'plan',
  subjectId: 'plan1',
  status,
  version: 1,
  subjectDigest: 'd'
})

function mockRpc(approvalStatus: string, approveResult: unknown = { ok: true, value: {} }): void {
  callRequestRpc.mockImplementation(async (method: string) => {
    if (method === 'approval.list') {
      return { ok: true, value: { approvals: [wireApproval(approvalStatus)] } }
    }
    if (method === 'approval.approve') {
      return approveResult
    }
    return { ok: true, value: {} }
  })
}

beforeEach(() => {
  callRuntimeRpc.mockReset()
  callRequestRpc.mockReset()
  const t = twoPhaseTree()
  callRuntimeRpc.mockResolvedValue({
    tasks: [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
  })
  mockRpc('pending')
  useAppStore.setState({
    requestFlowSupport: 'supported',
    executionGateByTaskId: {},
    requestFlowFeatures: new Set(['approval.runAll'])
  })
})
afterEach(cleanup)

describe('RequestPlanTab run-all choice', () => {
  it('a step-by-step request with phases offers Approve plan and Approve and run all steps', async () => {
    render(
      <RequestPlanTab request={req()} onChanged={vi.fn()} flow={backendFlowOf('change_request')} />
    )
    expect(await screen.findByTestId('plan-approve')).toHaveTextContent('Approve plan')
    expect(screen.getByTestId('plan-approve-run-all')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('plan-approve-run-all'))
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Run all 2 steps without asking again?')
  })

  it('confirming approves the plan with executionMode continuous and refreshes the request', async () => {
    mockRpc('pending', { ok: true, value: { executionMode: 'continuous' } })
    const onChanged = vi.fn()
    render(
      <RequestPlanTab
        request={req()}
        onChanged={onChanged}
        flow={backendFlowOf('change_request')}
      />
    )
    fireEvent.click(await screen.findByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
    expect(callRequestRpc).toHaveBeenCalledWith(
      'approval.approve',
      expect.objectContaining({ id: 'ap1', executionMode: 'continuous' })
    )
  })

  it('after the reload the outline and the "runs on its own" strip replace the per-step tree', async () => {
    mockRpc('approved')
    render(
      <RequestPlanTab
        request={req({ status: 'executing', executionMode: 'continuous' })}
        onChanged={vi.fn()}
        flow={backendFlowOf('change_request')}
      />
    )
    expect(await screen.findByTestId('plan-approved-strip')).toHaveTextContent(
      'The agent runs it on its own.'
    )
    expect(screen.getByTestId('plan-outline')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    expect(screen.queryByTestId('plan-gate-chips')).toBeNull()
  })

  it('a continuous request shows the card with its main button only', async () => {
    render(
      <RequestPlanTab
        request={req({ executionMode: 'continuous' })}
        onChanged={vi.fn()}
        flow={backendFlowOf('change_request')}
      />
    )
    expect(await screen.findByTestId('plan-approval-card')).toBeInTheDocument()
    expect(screen.getByTestId('plan-approve')).toHaveTextContent('Approve and start execution')
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
  })

  it('a pinned request shows the explanation and drops the choice', async () => {
    mockRpc('pending', {
      ok: false,
      error: {
        kind: 'unknown',
        code: 'runtime_error',
        message: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED: pinned',
        backendCode: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED'
      }
    })
    render(
      <RequestPlanTab request={req()} onChanged={vi.fn()} flow={backendFlowOf('change_request')} />
    )
    fireEvent.click(await screen.findByTestId('plan-approve-run-all'))
    fireEvent.click(screen.getByTestId('run-all-confirm'))
    expect(await screen.findByTestId('plan-decision-error')).toHaveTextContent(
      'Approve it normally.'
    )
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
  })

  it('offers nothing while the flow has not arrived or an older gateway lacks the flag', async () => {
    const a = render(<RequestPlanTab request={req()} onChanged={vi.fn()} flow={null} />)
    await screen.findByTestId('plan-approve')
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    a.unmount()

    useAppStore.setState({ requestFlowFeatures: new Set() })
    render(
      <RequestPlanTab request={req()} onChanged={vi.fn()} flow={backendFlowOf('change_request')} />
    )
    await screen.findByTestId('plan-approve')
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
  })
})
