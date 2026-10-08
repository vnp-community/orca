// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, waitFor, act, fireEvent } from '@testing-library/react'
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

import { useAppStore } from '../../../store'
import { emitRequestEvent } from '../../../lib/request-event-bus'
import { RequestPlanTab } from './RequestPlanTab'
import { derivePlanViewState } from './plan-view-state'
import { computeExecutionGates, attachApprovals } from './plan-approval-model'
import { planApproval, twoPhaseTree } from './plan-test-fixtures'
import type { OrcaRequest } from '../../../../../shared/request-types'

const req = (o: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    type: 'change_request',
    status: 'awaiting_plan_approval',
    planTaskId: 'plan1',
    ...o
  }) as OrcaRequest

function wireTasks() {
  const t = twoPhaseTree()
  return [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
}

beforeEach(() => {
  callRuntimeRpc.mockReset()
  callRequestRpc.mockReset()
  callRuntimeRpc.mockResolvedValue({ tasks: wireTasks() })
  callRequestRpc.mockImplementation(async (method: string) =>
    method === 'approval.list'
      ? {
          ok: true,
          value: {
            approvals: [
              {
                id: 'ap1',
                requestId: 'r1',
                subjectType: 'plan',
                subjectId: 'plan1',
                status: 'pending',
                version: 1,
                subjectDigest: 'd'
              }
            ]
          }
        }
      : { ok: true, value: {} }
  )
  useAppStore.setState({ requestFlowSupport: 'supported', executionGateByTaskId: {} })
})
afterEach(cleanup)

describe('RequestPlanTab', () => {
  it('renders the tree with approval controls and registers execution gates', async () => {
    const onChanged = vi.fn()
    const { unmount } = render(<RequestPlanTab request={req()} onChanged={onChanged} />)
    expect(await screen.findByTestId('plan-summary-header')).toBeInTheDocument()
    expect(screen.getByTestId('phase-node-ph1')).toBeInTheDocument()
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
    await waitFor(() =>
      expect(useAppStore.getState().executionGateByTaskId['a1']?.reason).toBe('plan_not_approved')
    )
    unmount()
    expect(useAppStore.getState().executionGateByTaskId).toEqual({})
  })

  it('shows "no plan yet" with the current step before planning', async () => {
    render(
      <RequestPlanTab
        request={req({ status: 'analyzing', planTaskId: undefined })}
        onChanged={vi.fn()}
      />
    )
    expect(await screen.findByTestId('plan-state-not_yet')).toBeInTheDocument()
    expect(screen.getByTestId('plan-current-step')).toBeInTheDocument()
    expect(callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('shows the generating skeleton while planning and generates on demand', async () => {
    render(
      <RequestPlanTab
        request={req({ status: 'planning', planTaskId: undefined })}
        onChanged={vi.fn()}
      />
    )
    expect(await screen.findByTestId('plan-generating')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('plan-generate'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', { id: 'r1', mode: 'propose' })
    )
  })

  it('shows a calm "plan not found" state when the plan task is missing', async () => {
    callRuntimeRpc.mockResolvedValue({ tasks: [] })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-state-not_found')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows the empty-plan state', async () => {
    callRuntimeRpc.mockResolvedValue({ tasks: [wireTasks()[0]] })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-state-empty')).toBeInTheDocument()
  })

  it('maps forbidden, network and unsupported load errors', async () => {
    callRuntimeRpc.mockRejectedValue({ code: 'forbidden', message: 'REQUEST_FORBIDDEN: x' })
    const a = render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-state-forbidden')).toBeInTheDocument()
    a.unmount()

    callRuntimeRpc.mockRejectedValue(new Error('network down'))
    const b = render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-error')).toBeInTheDocument()
    b.unmount()

    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByText('Request management not supported')).toBeInTheDocument()
  })

  it('renders nothing for flows without a plan', () => {
    const { container } = render(
      <RequestPlanTab request={req({ type: 'question' })} onChanged={vi.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('refreshes when plan.generated arrives', async () => {
    callRuntimeRpc.mockResolvedValue({ tasks: [] })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-state-not_found')).toBeInTheDocument()
    callRuntimeRpc.mockResolvedValue({ tasks: wireTasks() })
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: ''
      })
    )
    expect(await screen.findByTestId('phase-node-ph1')).toBeInTheDocument()
  })
})

describe('derivePlanViewState', () => {
  const base = { tree: null, isLoading: false, error: null, flowSupported: true }
  it('classifies by status when there is no plan task', () => {
    expect(derivePlanViewState({ ...base, request: { status: 'analyzing' } })).toBe('not_yet')
    expect(derivePlanViewState({ ...base, request: { status: 'planning' } })).toBe('generating')
    expect(derivePlanViewState({ ...base, request: { status: 'executing' } })).toBe('not_found')
  })
})

describe('computeExecutionGates', () => {
  it('gates by phase when only the phase is unapproved', () => {
    const tree = twoPhaseTree()
    const map = attachApprovals(tree, [
      planApproval('p', { status: 'approved' }),
      planApproval('x', { subjectType: 'phase', subjectId: 'ph2', status: 'pending' }),
      planApproval('y', { subjectType: 'phase', subjectId: 'ph1', status: 'approved' })
    ])
    const gates = computeExecutionGates(tree, map, true)
    expect(gates['a1']).toBeUndefined()
    expect(gates['b1']).toBe('phase_not_approved')
  })
  it('does not gate when no approval is required or present', () => {
    const tree = twoPhaseTree()
    expect(computeExecutionGates(tree, attachApprovals(tree, []), false)).toEqual({})
  })
})
