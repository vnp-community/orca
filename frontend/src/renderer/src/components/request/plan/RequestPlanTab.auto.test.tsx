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

import { useAppStore } from '../../../store'
import { RequestPlanTab } from './RequestPlanTab'
import { twoPhaseTree } from './plan-test-fixtures'
import type { OrcaRequest } from '../../../../../shared/request-types'

const req = (o: Partial<OrcaRequest> = {}): OrcaRequest =>
  ({
    id: 'r1',
    projectId: 'p1',
    type: 'bug',
    status: 'planning',
    planTaskId: undefined,
    ...o
  }) as OrcaRequest

const auto = (o: Record<string, unknown> = {}) => ({
  enabled: true,
  state: '',
  origin: 'event',
  attempt: 1,
  maxAttempts: 2,
  reasonCode: '',
  ...o
})

function serve(proposal: unknown, extra?: (method: string, params: unknown) => unknown) {
  callRequestRpc.mockImplementation(async (method: string, params: unknown) => {
    const custom = extra?.(method, params)
    if (custom !== undefined) {
      return custom
    }
    if (method === 'request.planProposal') {
      return proposal === null
        ? {
            ok: false,
            error: {
              kind: 'not_found',
              code: 'runtime_error',
              message: 'REQUEST_PLAN_RUN_NOT_FOUND: none'
            }
          }
        : { ok: true, value: proposal }
    }
    if (method === 'approval.list') {
      return { ok: true, value: { approvals: [] } }
    }
    return { ok: true, value: {} }
  })
}

beforeEach(() => {
  callRuntimeRpc.mockReset()
  callRequestRpc.mockReset()
  callRuntimeRpc.mockResolvedValue({ tasks: [] })
  useAppStore.setState({
    requestFlowSupport: 'supported',
    requestFlowFeatures: new Set(['request.planAuto']),
    executionGateByTaskId: {}
  })
})
afterEach(cleanup)

describe('RequestPlanTab with automatic plan generation', () => {
  it('shows "writing" and no Generate button while the server job runs', async () => {
    serve({ autoPlan: auto({ state: 'running' }) })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-auto-writing')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-generate')).toBeNull()
  })

  it('shows the failed card when the page is opened fresh, and Retry goes through the server', async () => {
    serve({
      runId: 'run1',
      status: 'failed',
      errorCode: 'REQUEST_PLAN_AI_INVALID_JSON',
      autoPlan: auto({ state: 'gave_up', attempt: 2, reasonCode: 'REQUEST_PLAN_AI_INVALID_JSON' })
    })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-run-failed')).toHaveTextContent(
      'REQUEST_PLAN_AI_INVALID_JSON'
    )
    expect(screen.getByTestId('plan-generate')).toHaveTextContent('Generate manually')

    fireEvent.click(screen.getByTestId('plan-run-retry'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith(
        'request.retryPlan',
        { id: 'r1' },
        { idempotent: true }
      )
    )
    expect(
      callRequestRpc.mock.calls.some(
        ([m, p]) => m === 'request.generatePlan' || (p as { mode?: string })?.mode
      )
    ).toBe(false)
  })

  it('shows the failed card of a gateway without the automatic mode, and Retry runs the manual flow', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    serve({ runId: 'run1', status: 'failed', errorCode: 'REQUEST_AI_COMPLETE_TIMEOUT' })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-run-failed')).toHaveTextContent(
      'REQUEST_AI_COMPLETE_TIMEOUT'
    )
    fireEvent.click(screen.getByTestId('plan-run-retry'))
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', {
        id: 'r1',
        mode: 'propose'
      })
    )
    expect(callRequestRpc.mock.calls.some(([m]) => m === 'request.retryPlan')).toBe(false)
  })

  it('shows "writing" for a plan being rewritten in the revision panel, over the existing tree', async () => {
    const t = twoPhaseTree()
    callRuntimeRpc.mockResolvedValue({
      tasks: [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
    })
    serve({ status: 'running', autoPlan: auto({ state: 'running' }) })
    render(
      <RequestPlanTab
        request={req({ planTaskId: 'plan1', type: 'change_request' })}
        onChanged={vi.fn()}
      />
    )
    // CR-REQ-045: the panel says it (Writing ... version), so the strip would only repeat it.
    expect(await screen.findByTestId('plan-revision-status')).toHaveTextContent('Writing')
    expect(screen.queryByTestId('plan-rewrite-strip')).toBeNull()
    expect(screen.getByTestId('phase-node-ph1')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-auto-writing')).toBeNull()
  })

  it('shows the failure of a rewrite over the existing tree, with Retry', async () => {
    const t = twoPhaseTree()
    callRuntimeRpc.mockResolvedValue({
      tasks: [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
    })
    serve({
      runId: 'run2',
      status: 'failed',
      errorCode: 'REQUEST_AI_COMPLETE_TIMEOUT',
      autoPlan: auto({ state: 'gave_up', reasonCode: 'REQUEST_AI_COMPLETE_TIMEOUT' })
    })
    render(
      <RequestPlanTab
        request={req({ planTaskId: 'plan1', type: 'change_request' })}
        onChanged={vi.fn()}
      />
    )
    expect(await screen.findByTestId('plan-run-failed')).toBeInTheDocument()
    expect(screen.getByTestId('phase-node-ph1')).toBeInTheDocument()
    expect(screen.getByTestId('plan-run-retry')).toBeInTheDocument()
  })

  it('shows the skipped note with the old Generate button', async () => {
    serve({ autoPlan: auto({ state: 'skipped', reasonCode: 'REQUEST_PLAN_AI_UNAVAILABLE' }) })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-auto-skipped')).toBeInTheDocument()
    expect(screen.getByTestId('plan-generate')).toHaveTextContent('Generate plan')
  })

  it('shows "starting" while the server is expected to begin, never the old button', async () => {
    serve({ autoPlan: auto({ attempt: 0, maxAttempts: 0 }) })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-auto-writing')).toHaveTextContent('starting')
    expect(screen.queryByTestId('plan-generate')).toBeNull()
  })

  it('keeps the old drafting state when automatic mode is off for the project', async () => {
    serve({ autoPlan: auto({ enabled: false, attempt: 0, maxAttempts: 0 }) })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-generating')).toHaveTextContent('AI is drafting')
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', { id: 'r1' })
    )
    expect(screen.getByTestId('plan-generate')).toHaveTextContent('Generate plan')
  })

  it('adds nothing when the Request awaits plan approval', async () => {
    const t = twoPhaseTree()
    callRuntimeRpc.mockResolvedValue({
      tasks: [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
    })
    serve({ status: 'failed', errorCode: 'X', autoPlan: auto({ state: 'gave_up' }) })
    render(
      <RequestPlanTab
        request={req({
          status: 'awaiting_plan_approval',
          planTaskId: 'plan1',
          type: 'change_request'
        })}
        onChanged={vi.fn()}
      />
    )
    expect(await screen.findByTestId('phase-node-ph1')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-run-failed')).toBeNull()
    expect(screen.queryByTestId('plan-rewrite-strip')).toBeNull()
    expect(callRequestRpc.mock.calls.some(([m]) => m === 'request.planProposal')).toBe(false)
  })
})
