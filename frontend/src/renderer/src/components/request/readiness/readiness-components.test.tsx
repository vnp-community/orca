// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../../../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))
const openRequestPage = vi.fn()
vi.mock('../request-page-navigation', () => ({ openRequestPage: (...a: unknown[]) => openRequestPage(...a) }))

import { PhaseReadinessSummary } from './PhaseReadinessSummary'
import { PlanDriftBanner, findDriftApproval } from './PlanDriftBanner'
import { PlanDriftReviewSheet } from './PlanDriftReviewSheet'
import { ReadinessBadge } from './ReadinessBadge'
import { ReadinessReportSheet } from './ReadinessReportSheet'
import type { Approval } from '../../../../../shared/request-types'
import type { TaskReadinessReport } from '../../../../../shared/request-artifact-types'

beforeEach(() => {
  callRequestRpc.mockReset()
  openRequestPage.mockReset()
})
afterEach(cleanup)

const report = (outcome: TaskReadinessReport['outcome'], findings: TaskReadinessReport['findings'] = []): TaskReadinessReport => ({ taskId: 't1', outcome, findings, checkedAt: '2026-10-07', specDigest: 'abcdef123456' })

describe('ReadinessBadge', () => {
  it('shows text + icon per outcome and nothing for null/unknown', () => {
    for (const [o, text] of [['ready', 'Ready'], ['needs_info', 'Needs information'], ['spec_defect', 'Spec defect'], ['env_defect', 'Environment issue']] as const) {
      const { container, unmount } = render(<ReadinessBadge report={report(o)} />)
      expect(container.textContent).toContain(text)
      expect(container.querySelector('svg')).not.toBeNull()
      unmount()
    }
    expect(render(<ReadinessBadge report={null} />).container).toBeEmptyDOMElement()
    expect(render(<ReadinessBadge report={report('unknown')} />).container).toBeEmptyDOMElement()
  })
})

describe('ReadinessReportSheet', () => {
  const base = { open: true, onOpenChange: vi.fn(), canWrite: true, hasDevServer: false, onCheck: vi.fn().mockResolvedValue({ ok: true, value: {} }), requestId: 'r1' }

  it('groups findings by tier and shows the env-var-names note', () => {
    render(<ReadinessReportSheet {...base} report={report('env_defect', [
      { code: 'ENV_MISSING', tier: 'environment', message: 'Missing DATABASE_URL' },
      { code: 'PATH_EMPTY', tier: 'structure', message: 'no files', path: 'src/a.ts' }
    ])} />)
    const headings = screen.getAllByRole('heading', { level: 4 }).map((h) => h.textContent)
    expect(headings).toEqual(['Structure', 'Environment'])
    expect(screen.getByText('Only variable names are shown, never values.')).toBeInTheDocument()
    expect(screen.getByTestId('readiness-note')).toHaveTextContent('Does not count as an attempt')
  })

  it('needs_info opens the request page; spec_defect navigates to the Plan tab without any RPC', () => {
    const { unmount } = render(<ReadinessReportSheet {...base} report={report('needs_info')} />)
    fireEvent.click(screen.getByText('Answer the questions'))
    expect(openRequestPage).toHaveBeenCalledWith({ section: 'requests', requestId: 'r1' })
    unmount()
    render(<ReadinessReportSheet {...base} report={report('spec_defect')} />)
    fireEvent.click(screen.getByText('Return to the Plan step to regenerate the task'))
    expect(openRequestPage).toHaveBeenLastCalledWith({ section: 'requests', requestId: 'r1', focus: 'plan' })
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('ready adds no run button of its own; the check button calls onCheck', async () => {
    render(<ReadinessReportSheet {...base} report={report('ready')} />)
    expect(screen.queryByRole('button', { name: 'Run' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Check readiness' }))
    await waitFor(() => expect(base.onCheck).toHaveBeenCalled())
  })

  it('env_defect offers connect only when the host provides the handler', () => {
    const onConnect = vi.fn()
    const { unmount } = render(<ReadinessReportSheet {...base} report={report('env_defect')} />)
    expect(screen.queryByText('Connect a dev server')).toBeNull()
    unmount()
    render(<ReadinessReportSheet {...base} report={report('env_defect')} onConnectDevServer={onConnect} />)
    fireEvent.click(screen.getByText('Connect a dev server'))
    expect(onConnect).toHaveBeenCalled()
  })
})

describe('PhaseReadinessSummary', () => {
  it('renders nothing without any checked task and lists only non-zero buckets', () => {
    expect(render(<PhaseReadinessSummary summary={{ ready: 0, needsInfo: 0, specDefect: 0, envDefect: 0, unchecked: 4 }} />).container).toBeEmptyDOMElement()
    render(<PhaseReadinessSummary summary={{ ready: 7, needsInfo: 1, specDefect: 0, envDefect: 1, unchecked: 0 }} />)
    expect(screen.getByTestId('phase-readiness-summary')).toHaveTextContent('7 ready, 1 need information, 1 environment issues')
  })
})

describe('plan drift', () => {
  const approval = (over: Partial<Approval> = {}): Approval => ({
    id: 'a1', requestId: 'r1', subjectType: 'phase', subjectId: 'p1', status: 'pending', stage: 'drift_review', version: 3, subjectDigest: 'dg', createdAt: '', updatedAt: '', ...over
  })

  it('finds only a pending phase drift_review approval', () => {
    expect(findDriftApproval([approval()])?.id).toBe('a1')
    expect(findDriftApproval([approval({ stage: 'normal' }), approval({ status: 'approved' }), approval({ subjectType: 'plan' })])).toBeNull()
  })

  it('banner is an alert when enforced and a quiet status when advisory', () => {
    const onReview = vi.fn()
    const { unmount } = render(<PlanDriftBanner phaseName="Phase 2" onReview={onReview} />)
    expect(screen.getByRole('alert')).toHaveTextContent('Phase Phase 2 is paused')
    fireEvent.click(screen.getByText('Review and decide'))
    expect(onReview).toHaveBeenCalled()
    unmount()
    render(<PlanDriftBanner advisory onReview={onReview} />)
    expect(screen.getByRole('status')).toHaveTextContent('Advisory')
  })

  it('review sheet needs a 10+ char reason, offers accept/return only, and no cancel-phase', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { drift: { phase_id: 'p1', items: [{ task_id: 't9', expected: '2 files', actual: '9 files' }] } } })
    const onAccept = vi.fn().mockResolvedValue({ ok: true, value: {} })
    const onReturn = vi.fn().mockResolvedValue({ ok: true, value: {} })
    render(<PlanDriftReviewSheet open onOpenChange={vi.fn()} phaseId="p1" phaseName="Phase 1" onAccept={onAccept} onReturn={onReturn} />)
    expect(await screen.findByText('9 files')).toBeInTheDocument()
    expect(screen.queryByText(/cancel phase/i)).toBeNull()
    const accept = screen.getByRole('button', { name: 'Accept the drift and continue' })
    expect(accept).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'reviewed with the owner' } })
    fireEvent.click(accept)
    await waitFor(() => expect(onAccept).toHaveBeenCalledWith('reviewed with the owner'))
    expect(onReturn).not.toHaveBeenCalled()
    expect(screen.getByText('Tasks already running continue until they finish.')).toBeInTheDocument()
  })
})
