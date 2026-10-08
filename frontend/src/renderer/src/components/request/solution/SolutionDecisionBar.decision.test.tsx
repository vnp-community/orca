// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const useDecisions = vi.fn()
vi.mock('../../../hooks/useDecisions', () => ({ useDecisions: (...a: unknown[]) => useDecisions(...a) }))
const useImpactAssessment = vi.fn()
vi.mock('../../../hooks/useImpactAssessment', async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  useImpactAssessment: (...a: unknown[]) => useImpactAssessment(...a)
}))

import { SolutionDecisionBar } from './SolutionDecisionBar'
import type { SolutionDecision } from './useSolutionDecision'
import type { Approval, Solution } from '../../../../../shared/request-types'
import type { Decision } from '../../../../../shared/request-artifact-types'

const solution: Solution = {
  id: 's1', requestId: 'r1', kind: 'solution', status: 'ready',
  options: [{ id: 'o1', title: 'Option A' }, { id: 'o2', title: 'Option B' }]
}
const approval = { id: 'ap', requestId: 'r1', version: 1, status: 'pending', subjectDigest: 'dg' } as Approval
const presentation = { bannerKey: 'k', tone: 'neutral', actions: ['approve', 'reject'], readOnly: false } as never
const dec = (over: Partial<Decision> = {}): Decision => ({
  id: 'd1', displayId: 'D1', subjectKind: 'solution_option', subjectId: 's1', subjectDigest: 'dg', rationale: '',
  riskLevel: 'normal', riskReasons: [], status: 'chosen', version: 1, recommendedOptionId: 'o1', ...over
})
function decisionHook(over: Partial<SolutionDecision> = {}): SolutionDecision {
  return {
    submitting: false, failure: null, chosenButNotApproved: false, conflicted: false, acknowledgeConflict: vi.fn(),
    isDenied: () => false, approveSelected: vi.fn().mockResolvedValue({ ok: true }), reject: vi.fn(), regenerate: vi.fn(), ...over
  } as SolutionDecision
}
function mockDecisions(current: Decision | null, confirm = vi.fn().mockResolvedValue({ ok: true, value: {} })) {
  useDecisions.mockReturnValue({ decisions: current ? [current] : [], current, loading: false, error: null, refetch: vi.fn(), confirm })
  return confirm
}
const bar = (decision: SolutionDecision, selectedId: string | null = 'o2') => (
  <SolutionDecisionBar solution={solution} requestType="change_request" presentation={presentation} approval={approval} selectedId={selectedId} decision={decision} />
)

const impactApi = (over: Record<string, unknown> = {}) => ({
  summary: null, findings: [], comparison: null, canOverride: undefined, status: 'idle', request: vi.fn(),
  acceptRisk: vi.fn().mockResolvedValue({ ok: true, value: {} }), override: vi.fn(), refetch: vi.fn(), ...over
})

beforeEach(() => {
  useDecisions.mockReset()
  useImpactAssessment.mockReset()
  useImpactAssessment.mockReturnValue(impactApi())
})
afterEach(cleanup)

describe('SolutionDecisionBar decision flow', () => {
  it('requires a 10+ character reason when choosing something other than the recommendation', () => {
    mockDecisions(dec())
    const decision = decisionHook()
    render(bar(decision))
    const approve = screen.getByRole('button', { name: 'Approve this option' })
    expect(approve).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'we need option B for latency' } })
    expect(approve).toBeEnabled()
    fireEvent.click(approve)
    expect(decision.approveSelected).toHaveBeenCalledWith('o2', undefined, expect.objectContaining({ rationale: 'we need option B for latency' }))
  })

  it('does not require a reason for the recommended option', () => {
    mockDecisions(dec())
    render(bar(decisionHook(), 'o1'))
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeEnabled()
  })

  it('opens the high-risk dialog when choose returns needsConfirmation, then confirms and approves', async () => {
    const high = dec({ riskLevel: 'high', riskReasons: ['Irreversible migration'], chosenOptionId: 'o1' })
    const confirm = mockDecisions(dec())
    const approveSelected = vi.fn().mockResolvedValueOnce({ ok: true, needsConfirmation: true, decision: high }).mockResolvedValue({ ok: true })
    render(bar(decisionHook({ approveSelected }), 'o1'))
    fireEvent.click(screen.getByRole('button', { name: 'Approve this option' }))
    expect(await screen.findByTestId('high-risk-decision-dialog')).toBeInTheDocument()
    expect(screen.getByText('Irreversible migration')).toBeInTheDocument()
    fireEvent.change(screen.getAllByRole('textbox').at(-1) as HTMLElement, { target: { value: 'option a' } })
    fireEvent.click(screen.getAllByRole('button', { name: 'Confirm choice' }).at(-1) as HTMLElement)
    await waitFor(() => expect(confirm).toHaveBeenCalledWith('d1', 'option a', 1))
    await waitFor(() => expect(approveSelected).toHaveBeenCalledTimes(2))
  })

  it('shows the pending-confirmation banner for a chosen high-risk Decision after reload', () => {
    mockDecisions(dec({ riskLevel: 'high' }))
    render(bar(decisionHook(), 'o1'))
    expect(screen.getByTestId('decision-pending-confirmation')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Confirm choice' })).toBeEnabled()
  })

  it('locks Approve when the Decision digest differs from the approval digest', () => {
    mockDecisions(dec({ subjectDigest: 'stale' }))
    render(bar(decisionHook(), 'o1'))
    expect(screen.getByTestId('decision-digest-changed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeDisabled()
  })

  it('hides Approve for a forbidden self-choice', () => {
    mockDecisions(dec())
    const failure = { errorClass: 'forbidden', error: { kind: 'forbidden', code: 'REQUEST_DECISION_SELF_CHOICE_FORBIDDEN', message: 'm' } } as never
    render(bar(decisionHook({ failure }), 'o1'))
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeDisabled()
    expect(screen.getByText('The reporter cannot choose their own request')).toBeInTheDocument()
  })

  const enforced = (level: string, digest = 'dg1') => ({
    assessmentId: 'a1', digest, level, score: null, topReasons: [], confidence: null, assessedAt: null, tool: null,
    stale: false, mode: 'enforce', status: 'ready', hardRules: []
  })
  const highFinding = { id: 'f1', dimension: 'data', level: 'high', title: 'Column removed' }

  it('medium risk blocks Approve until the impact section is opened, then sends viewedImpactDigest', async () => {
    mockDecisions(dec())
    useImpactAssessment.mockReturnValue(impactApi({ summary: enforced('medium'), status: 'ready' }))
    const decision = decisionHook()
    render(bar(decision, 'o1'))
    const approve = screen.getByRole('button', { name: 'Approve this option' })
    expect(approve).toBeDisabled()
    fireEvent.click(screen.getByTestId('view-impact-trigger'))
    await waitFor(() => expect(approve).toBeEnabled())
    fireEvent.click(approve)
    expect(decision.approveSelected).toHaveBeenCalledWith('o1', undefined, expect.objectContaining({ viewedImpactDigest: 'dg1' }))
  })

  it('high risk blocks Approve until each high finding is accepted with a reason, then sends acceptedFindingIds', async () => {
    mockDecisions(dec())
    const api = impactApi({ summary: enforced('high'), findings: [highFinding], status: 'ready' })
    useImpactAssessment.mockReturnValue(api)
    const decision = decisionHook()
    render(bar(decision, 'o1'))
    const approve = screen.getByRole('button', { name: 'Approve this option' })
    expect(approve).toBeDisabled()
    const reason = screen.getByLabelText('Reason for accepting')
    fireEvent.change(reason, { target: { value: 'too short' } })
    expect(screen.getByRole('button', { name: 'Record acceptance' })).toBeDisabled()
    fireEvent.change(reason, { target: { value: 'tested in staging, rollback ready' } })
    fireEvent.click(screen.getByRole('button', { name: 'Record acceptance' }))
    await waitFor(() => expect(api.acceptRisk).toHaveBeenCalledWith({ findingId: 'f1', rationale: 'tested in staging, rollback ready' }))
    await waitFor(() => expect(approve).toBeEnabled())
    fireEvent.click(approve)
    expect(decision.approveSelected).toHaveBeenCalledWith('o1', undefined, expect.objectContaining({ acceptedFindingIds: ['f1'] }))
  })

  it('a changed assessment digest voids acceptances and shows the banner', async () => {
    mockDecisions(dec())
    let api = impactApi({ summary: enforced('high', 'dg1'), findings: [highFinding], status: 'ready' })
    useImpactAssessment.mockImplementation(() => api)
    const { rerender } = render(bar(decisionHook(), 'o1'))
    fireEvent.change(screen.getByLabelText('Reason for accepting'), { target: { value: 'tested in staging, rollback ready' } })
    fireEvent.click(screen.getByRole('button', { name: 'Record acceptance' }))
    await waitFor(() => expect(screen.getByText('Accepted')).toBeInTheDocument())
    api = impactApi({ summary: enforced('high', 'dg2'), findings: [highFinding], status: 'ready' })
    rerender(bar(decisionHook(), 'o1'))
    await waitFor(() => expect(screen.getByTestId('risk-acceptance-invalidated')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeDisabled()
  })

  it('shadow mode and unassessed never block, and the second-approver note shows for critical', () => {
    mockDecisions(dec())
    useImpactAssessment.mockReturnValue(impactApi({ summary: { ...enforced('critical'), mode: 'shadow' }, findings: [highFinding], status: 'ready' }))
    render(bar(decisionHook(), 'o1'))
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeEnabled()
    cleanup()
    useImpactAssessment.mockReturnValue(impactApi({ summary: enforced('critical'), findings: [{ ...highFinding, level: 'critical' }], status: 'ready' }))
    render(bar(decisionHook(), 'o1'))
    expect(screen.getByTestId('risk-second-approver')).toBeInTheDocument()
  })

  it('shows the override menu only when the backend allows it, and the approver-not-allowed note', () => {
    mockDecisions(dec())
    render(bar(decisionHook(), 'o1'))
    expect(screen.queryByRole('button', { name: 'More gate options' })).toBeNull()
    cleanup()
    useImpactAssessment.mockReturnValue(impactApi({ canOverride: true }))
    const failure = { errorClass: 'forbidden', error: { kind: 'forbidden', code: 'forbidden', message: 'REQUEST_RISK_APPROVER_NOT_ALLOWED: x' } } as never
    render(bar(decisionHook({ failure }), 'o1'))
    expect(screen.getByRole('button', { name: 'More gate options' })).toBeInTheDocument()
    expect(screen.getByTestId('risk-approver-not-allowed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Approve this option' })).toBeDisabled()
  })
})
