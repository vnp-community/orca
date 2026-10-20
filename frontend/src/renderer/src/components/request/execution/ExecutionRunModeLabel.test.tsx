// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ExecutionRunModeLabel } from './ExecutionRunModeLabel'
import { ExecutionSummaryStrip } from './ExecutionSummaryStrip'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'

afterEach(cleanup)

const summary: ExecutionSummary = {
  requestId: 'r',
  status: 'executing',
  running: [],
  failed: [],
  needsYou: [],
  maxAttempts: 2,
  progress: {
    phasesTotal: 3,
    phasesDone: 1,
    tasksTotal: 9,
    tasksDone: 3,
    tasksFailed: 0,
    tasksRunning: 1
  },
  handover: { state: 'none', archiveAttempts: 0 }
}

describe('ExecutionRunModeLabel', () => {
  it('says a continuous run goes on by itself', () => {
    render(<ExecutionRunModeLabel mode="continuous" />)
    expect(screen.getByTestId('execution-run-mode')).toHaveTextContent('Runs all steps on its own')
  })

  it('says a step-by-step run asks first', () => {
    render(<ExecutionRunModeLabel mode="step_by_step" />)
    expect(screen.getByTestId('execution-run-mode')).toHaveTextContent('Asks before each step')
  })

  it('is reachable by keyboard and explains who the steps run as', async () => {
    render(<ExecutionRunModeLabel mode="continuous" />)
    const chip = screen.getByTestId('execution-run-mode')
    expect(chip).toHaveAttribute('tabindex', '0')
    fireEvent.focus(chip)
    expect(
      (await screen.findAllByText('Steps run with the access of the person who approved the plan.'))
        .length
    ).toBeGreaterThan(0)
  })
})

describe('ExecutionSummaryStrip run mode', () => {
  it('shows the label next to the state and keeps Stop run', () => {
    render(
      <ExecutionSummaryStrip
        status="executing"
        summary={summary}
        onStop={vi.fn()}
        runMode="continuous"
      />
    )
    expect(screen.getByTestId('execution-run-label')).toHaveTextContent('Running')
    expect(screen.getByTestId('execution-run-mode')).toBeInTheDocument()
    expect(screen.getByTestId('execution-stop-run')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /pause/i })).toBeNull()
  })

  it.each([null, undefined])('shows nothing when the mode is %s', (runMode) => {
    render(<ExecutionSummaryStrip status="executing" summary={summary} runMode={runMode} />)
    expect(screen.queryByTestId('execution-run-mode')).toBeNull()
  })
})
