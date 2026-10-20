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

import { usePlanDecision } from '../../../hooks/usePlanDecision'
import { PlanApprovalCard } from './PlanApprovalCard'
import { planApproval } from './plan-test-fixtures'
import { fallbackRequestFlow } from '../../../../../shared/request-flow-view'

const settled = vi.fn()

function Harness({
  type,
  steps,
  tasks
}: {
  type: 'change_request' | 'security' | 'docs'
  steps: number
  tasks: number
}): React.JSX.Element {
  const d = usePlanDecision({ id: 'r1' }, settled)
  return (
    <PlanApprovalCard
      approval={planApproval('ap1')}
      decision={d}
      flow={fallbackRequestFlow(type, 'L')}
      stepCount={steps}
      taskCount={tasks}
    />
  )
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
})
afterEach(cleanup)

describe('PlanApprovalCard', () => {
  it('says the agent runs every step after one approval and has no per-step controls', () => {
    render(<Harness type="change_request" steps={7} tasks={28} />)
    const card = screen.getByTestId('plan-approval-card')
    expect(card).toHaveTextContent('Approve once and the agent runs all 7 steps')
    expect(card).toHaveTextContent('It stops and asks you only when:')
    expect(card).toHaveTextContent('the change drifts from this plan')
    expect(card).not.toHaveTextContent('pre-deploy')
    expect(screen.getByTestId('plan-risk-whole')).toBeInTheDocument()
    expect(screen.queryByText(/Approve phase|Start phase/)).toBeNull()
  })

  it('counts tasks when the plan has no steps, and mentions the pre-deploy gate for security', () => {
    render(<Harness type="security" steps={0} tasks={4} />)
    const card = screen.getByTestId('plan-approval-card')
    expect(card).toHaveTextContent('runs all 4 tasks')
    expect(card).toHaveTextContent('pre-deploy approval')
    expect(screen.queryByTestId('plan-risk-whole')).toBeNull()
  })

  it('the primary button starts execution with one approve call', async () => {
    render(<Harness type="change_request" steps={7} tasks={28} />)
    fireEvent.click(screen.getByRole('button', { name: 'Approve and start execution' }))
    await waitFor(() => expect(settled).toHaveBeenCalled())
    expect(callRequestRpc).toHaveBeenCalledWith(
      'approval.approve',
      expect.objectContaining({ id: 'ap1' })
    )
  })

  it('the main button states continuous itself and there is no second button', async () => {
    render(<Harness type="change_request" steps={7} tasks={28} />)
    expect(screen.queryByTestId('plan-approve-run-all')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Approve and start execution' }))
    await waitFor(() => expect(settled).toHaveBeenCalled())
    expect(callRequestRpc).toHaveBeenCalledWith(
      'approval.approve',
      expect.objectContaining({ id: 'ap1', executionMode: 'continuous' })
    )
  })

  it('a task-list approval says "task list" (BUG-QA-06: subjectType is folded to plan, rawSubjectType keeps the kind)', () => {
    function TaskListHarness(): React.JSX.Element {
      const d = usePlanDecision({ id: 'r1' }, settled)
      return (
        <PlanApprovalCard
          approval={planApproval('ap2', { rawSubjectType: 'task_list' })}
          decision={d}
          flow={fallbackRequestFlow('docs', 'M')}
          stepCount={0}
          taskCount={3}
        />
      )
    }
    render(<TaskListHarness />)
    expect(screen.getByTestId('plan-approval-card')).toHaveTextContent(
      'Approve once and the agent runs all 3 tasks'
    )
  })

  it('Request changes needs a reason of at least 10 characters', () => {
    render(<Harness type="change_request" steps={7} tasks={28} />)
    fireEvent.click(screen.getByRole('button', { name: 'Request changes' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })
})
