// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, waitFor, fireEvent, within } from '@testing-library/react'
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
    type: 'change_request',
    engine: 'openspec',
    status: 'awaiting_plan_approval',
    planTaskId: 'plan1',
    version: 4,
    updatedAt: '2026-10-20T10:00:00Z',
    ...o
  }) as OrcaRequest

const PENDING_APPROVAL = {
  id: 'ap1',
  requestId: 'r1',
  subjectType: 'plan',
  subjectId: 'plan1',
  status: 'pending',
  version: 1,
  subjectDigest: 'd'
}
const REVISIONS = {
  revisions: [
    {
      revision: 2,
      runId: 'run-2',
      status: 'current',
      revisionFeedback: 'Add tests',
      fileCount: 2,
      filesChanged: 1
    },
    { revision: 1, runId: 'run-1', status: 'superseded' }
  ]
}
const BUNDLE = {
  branch: 'request/7-proposal',
  tasksMdDigest: 'dg',
  files: [
    { path: 'proposal.md', size: 10, change: 'unchanged' },
    { path: 'tasks.md', size: 10, change: 'changed' }
  ]
}

function serve(over: Record<string, unknown> = {}) {
  callRequestRpc.mockImplementation(async (method: string, params: { path?: string }) => {
    if (method in over) {
      return over[method]
    }
    switch (method) {
      case 'approval.list':
        return { ok: true, value: { approvals: [PENDING_APPROVAL] } }
      case 'request.planProposal':
        return {
          ok: true,
          value: { runId: 'run-2', status: 'succeeded', engine: 'openspec', bundle: BUNDLE }
        }
      case 'request.planBundleFile':
        return {
          ok: true,
          value: { path: params.path, content: `text of ${params.path}`, omitted: false }
        }
      case 'request.planRevisions':
        return { ok: true, value: REVISIONS }
      case 'request.generatePlan':
        return { ok: true, value: { runId: 'run-3' } }
      default:
        return { ok: true, value: {} }
    }
  })
}

const methods = () => callRequestRpc.mock.calls.map(([m]) => m)

beforeEach(() => {
  const t = twoPhaseTree()
  callRuntimeRpc.mockReset()
  callRequestRpc.mockReset()
  callRuntimeRpc.mockResolvedValue({
    tasks: [t.plan!, ...t.phases, ...Object.values(t.tasksByPhase).flat()]
  })
  useAppStore.setState({
    requestFlowSupport: 'supported',
    requestFlowFeatures: new Set(['request.planAuto', 'request.planRevisions']),
    executionGateByTaskId: {}
  })
  serve()
})
afterEach(cleanup)

describe('RequestPlanTab documents entry', () => {
  it('offers Documents for an OpenSpec change and opens the documents under the same header and approval', async () => {
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    fireEvent.click(await screen.findByRole('radio', { name: 'Documents' }))
    expect(await screen.findByTestId('plan-documents-view')).toBeInTheDocument()
    expect(screen.queryByTestId('phase-node-ph1')).toBeNull()
    expect(screen.getByTestId('plan-summary-header')).toBeInTheDocument()
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
    await screen.findByText('text of proposal.md')
    fireEvent.click(screen.getByRole('radio', { name: 'Tree' }))
    expect(await screen.findByTestId('phase-node-ph1')).toBeInTheDocument()
  })

  it('offers Proposal, not Documents, for a native plan', async () => {
    serve({
      'request.planProposal': {
        ok: true,
        value: {
          runId: 'run-2',
          status: 'succeeded',
          engine: 'native',
          proposal: { title: 'Native', tasks: [{ title: 'Only' }] }
        }
      }
    })
    render(<RequestPlanTab request={req({ type: 'bug', engine: 'native' })} onChanged={vi.fn()} />)
    expect(await screen.findByRole('radio', { name: 'Proposal' })).toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: 'Documents' })).toBeNull()
    fireEvent.click(screen.getByRole('radio', { name: 'Proposal' }))
    expect(await screen.findByTestId('plan-proposal-view')).toBeInTheDocument()
  })

  it('offers neither for a hotfix, whose plan is one task made from the diagnosis', async () => {
    render(
      <RequestPlanTab request={req({ type: 'hotfix', engine: 'native' })} onChanged={vi.fn()} />
    )
    await screen.findByTestId('plan-summary-header')
    expect(screen.queryByRole('radio', { name: 'Documents' })).toBeNull()
    expect(screen.queryByRole('radio', { name: 'Proposal' })).toBeNull()
  })

  it('leaves the existing graph switch alone', async () => {
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    const radios = await screen.findAllByRole('radio')
    expect(radios.map((r) => r.textContent)).toEqual(['Tree', 'Graph', 'Documents'])
  })
})

describe('RequestPlanTab change request', () => {
  it('asks for a change in one step: no rejection, the server saves the new version', async () => {
    const onChanged = vi.fn()
    render(<RequestPlanTab request={req()} onChanged={onChanged} />)
    const box = await screen.findByTestId('plan-revision-input')
    fireEvent.change(box, { target: { value: 'Split the migration' } })
    fireEvent.click(screen.getByTestId('plan-revision-submit'))
    await waitFor(() => expect(methods()).toContain('request.generatePlan'))
    const call = callRequestRpc.mock.calls.find(([m]) => m === 'request.generatePlan')!
    expect(call[1]).toMatchObject({ id: 'r1', mode: 'propose', feedback: 'Split the migration' })
    expect(call[1].idempotencyKey).toEqual(expect.any(String))
    expect(methods()).not.toContain('approval.reject')
    expect(methods().filter((m) => m === 'request.generatePlan')).toHaveLength(1)
    expect(onChanged).toHaveBeenCalled()
  })

  it('then says a version is being written, locks the box and takes the approval buttons away', async () => {
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-approve')).toBeInTheDocument()
    fireEvent.change(screen.getByTestId('plan-revision-input'), { target: { value: 'Add tests' } })
    fireEvent.click(screen.getByTestId('plan-revision-submit'))
    await waitFor(() =>
      expect(screen.getByTestId('plan-revision-status')).toHaveTextContent('Writing version 3…')
    )
    expect(screen.getByTestId('plan-revision-input')).toBeDisabled()
    expect(screen.queryByTestId('plan-approve')).toBeNull()
    expect(screen.queryByTestId('plan-reject')).toBeNull()
  })

  it('shows the panel in every view and not for a hotfix or a plan past its gate', async () => {
    const { unmount } = render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    expect(await screen.findByTestId('plan-revision-panel')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('radio', { name: 'Documents' }))
    expect(screen.getByTestId('plan-revision-panel')).toBeInTheDocument()
    unmount()
    const hotfix = render(
      <RequestPlanTab request={req({ type: 'hotfix', engine: 'native' })} onChanged={vi.fn()} />
    )
    await screen.findByTestId('plan-summary-header')
    expect(screen.queryByTestId('plan-revision-panel')).toBeNull()
    hotfix.unmount()
    render(<RequestPlanTab request={req({ status: 'executing' })} onChanged={vi.fn()} />)
    await screen.findByTestId('plan-summary-header')
    expect(screen.queryByTestId('plan-revision-panel')).toBeNull()
  })

  it('keeps the typed text and shows the reason when the server refuses', async () => {
    serve({
      'request.generatePlan': {
        ok: false,
        error: { kind: 'forbidden', code: 'runtime_error', message: 'REQUEST_PLAN_FORBIDDEN: no' }
      }
    })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    fireEvent.change(await screen.findByTestId('plan-revision-input'), {
      target: { value: 'Add tests' }
    })
    fireEvent.click(screen.getByTestId('plan-revision-submit'))
    expect(await screen.findByTestId('plan-revision-error')).toHaveTextContent(
      'Only the approvers of this plan can request changes. Your text is kept.'
    )
    expect(screen.getByTestId('plan-revision-input')).toHaveValue('Add tests')
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
  })

  it('on a gateway without server-side saving, saves the revised plan from the tab', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    serve({
      'request.generatePlan': { ok: true, value: { runId: 'run-3', proposal: { title: 'T' } } }
    })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    fireEvent.change(await screen.findByTestId('plan-revision-input'), { target: { value: 'x' } })
    fireEvent.click(screen.getByTestId('plan-revision-submit'))
    await waitFor(() =>
      expect(
        callRequestRpc.mock.calls
          .filter(([m]) => m === 'request.generatePlan')
          .map(([, p]) => p.mode)
      ).toEqual(['propose', 'commit'])
    )
  })

  it('shows the failure of a round with Try again, which asks the server again and never uses the retry of the first plan', async () => {
    serve({
      'request.planProposal': {
        ok: true,
        value: {
          runId: 'run-3',
          status: 'failed',
          errorCode: 'REQUEST_PLAN_REVISION_NO_CHANGE',
          autoPlan: {
            enabled: true,
            state: 'gave_up',
            origin: 'revision',
            attempt: 1,
            maxAttempts: 1,
            reasonCode: 'REQUEST_PLAN_REVISION_NO_CHANGE'
          }
        }
      }
    })
    render(<RequestPlanTab request={req({ status: 'planning' })} onChanged={vi.fn()} />)
    const card = await screen.findByTestId('plan-run-failed')
    expect(card).toHaveTextContent(
      'The plan could not be revised (REQUEST_PLAN_REVISION_NO_CHANGE)'
    )
    expect(card).toHaveTextContent(
      'The agent did not change any document. Say more precisely what to change.'
    )
    expect(card).toHaveTextContent('Or write a new request below.')
    expect(screen.getByTestId('plan-revision-input')).toBeEnabled()
    fireEvent.click(within(card).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(methods()).toContain('request.generatePlan'))
    expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', {
      id: 'r1',
      mode: 'propose'
    })
    expect(methods()).not.toContain('request.retryPlan')
  })
})

describe('RequestPlanTab versions', () => {
  it('reads an older version read only: banner, no panel, its documents, the tree untouched', async () => {
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    fireEvent.click(await screen.findByRole('button', { name: /Versions \(2\)/ }))
    fireEvent.click(screen.getByRole('button', { name: /Version 1/ }))
    expect(await screen.findByTestId('plan-older-version-banner')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-revision-panel')).toBeNull()
    expect(await screen.findByTestId('plan-documents-view')).toBeInTheDocument()
    expect(screen.getByTestId('plan-approve')).toBeInTheDocument()
    expect(screen.getByTestId('plan-tree-current-note')).toBeInTheDocument()
    await waitFor(() =>
      expect(callRequestRpc).toHaveBeenCalledWith('request.planBundleFile', {
        id: 'r1',
        path: 'proposal.md',
        revision: 1
      })
    )
    fireEvent.click(screen.getByTestId('plan-back-to-current'))
    expect(await screen.findByTestId('plan-revision-panel')).toBeInTheDocument()
    expect(screen.queryByTestId('plan-older-version-banner')).toBeNull()
  })

  it('shows what was asked for the version on screen', async () => {
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    fireEvent.click(await screen.findByRole('button', { name: 'What was asked for this version' }))
    expect(screen.getByText('Add tests')).toBeInTheDocument()
  })

  it('has no version list on a gateway that does not list versions', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set(['request.planAuto']) })
    render(<RequestPlanTab request={req()} onChanged={vi.fn()} />)
    await screen.findByTestId('plan-revision-panel')
    expect(screen.queryByTestId('plan-version-bar')).toBeNull()
    expect(methods()).not.toContain('request.planRevisions')
  })
})
