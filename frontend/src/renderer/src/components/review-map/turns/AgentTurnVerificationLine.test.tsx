// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AgentTurnVerificationLine } from './AgentTurnVerificationLine'
import { buildAgentTurnVerificationViewModel } from './agent-turn-verification-view-model'

afterEach(cleanup)

// Echo keys so the test does not depend on English copy.
const translate = (key: string, params?: Record<string, unknown>) =>
  params ? `${key}${JSON.stringify(params)}` : key

function vmFor(agreement: string, extra: Record<string, unknown> = {}) {
  return buildAgentTurnVerificationViewModel({
    claims: { items: [{ kind: 'tests_pass', basis: 'ran_command', agreement, ...extra }] },
    commandsSummary: {
      v: 1, totalToolUses: 1, toolCounts: { Bash: 1 }, truncated: false,
      commands: [{ name: 'pnpm', sub: 'test', category: 'test', count: 3 }]
    }
  })
}

describe('AgentTurnVerificationLine', () => {
  it('renders nothing for an empty view model', () => {
    const { container } = render(
      <AgentTurnVerificationLine viewModel={buildAgentTurnVerificationViewModel({})} translate={translate} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows the ran line and the recorded-not-verified note', () => {
    render(<AgentTurnVerificationLine viewModel={vmFor('consistent')} translate={translate} />)
    expect(screen.getAllByText(/verification\.ran/).length).toBeGreaterThan(0)
    expect(screen.getByText('auto.components.reviewMap.turns.verification.recordedNote')).toBeTruthy()
  })

  it.each(['consistent', 'contradicted', 'unverified', 'unknown'])('renders the %s state', (agreement) => {
    render(<AgentTurnVerificationLine viewModel={vmFor(agreement)} translate={translate} />)
    expect(screen.getByText(new RegExp(`agreement\\.${agreement}`))).toBeTruthy()
  })

  it('offers re-run only for unverified checks when canRun, and calls back', () => {
    const onRunChecks = vi.fn()
    const { rerender } = render(
      <AgentTurnVerificationLine viewModel={vmFor('unverified')} canRun onRunChecks={onRunChecks} translate={translate} />
    )
    fireEvent.click(screen.getByText('auto.components.reviewMap.turns.verification.runChecks'))
    expect(onRunChecks).toHaveBeenCalledTimes(1)
    rerender(
      <AgentTurnVerificationLine viewModel={vmFor('consistent')} canRun onRunChecks={onRunChecks} translate={translate} />
    )
    expect(screen.queryByText('auto.components.reviewMap.turns.verification.runChecks')).toBeNull()
  })

  it('links to the verifying run', () => {
    const onViewRun = vi.fn()
    render(
      <AgentTurnVerificationLine
        viewModel={vmFor('contradicted', { verifyingRunId: 'run-9' })}
        onViewRun={onViewRun}
        translate={translate}
      />
    )
    fireEvent.click(screen.getByText('auto.components.reviewMap.turns.verification.viewRun'))
    expect(onViewRun).toHaveBeenCalledWith('run-9')
  })
})
