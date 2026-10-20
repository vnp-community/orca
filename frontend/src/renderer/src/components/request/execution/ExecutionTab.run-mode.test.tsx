// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../hooks/usePlanTree', () => ({
  usePlanTree: () => ({ tree: null, approvals: [], refetch: vi.fn() })
}))
vi.mock('../../../hooks/usePlanDecision', () => ({ usePlanDecision: () => ({}) }))
vi.mock('./ExecutionNeedsYou', () => ({ ExecutionNeedsYou: () => null }))
vi.mock('./ExecutionStepList', () => ({ ExecutionStepList: () => null }))
vi.mock('./HandoverRow', () => ({ HandoverRow: () => null }))
vi.mock('./ExecutionTaskSheet', () => ({
  ExecutionTaskSheet: () => null,
  useExecutionTaskSheet: () => ({ open: false, onOpenChange: vi.fn(), openTask: vi.fn() })
}))

import { ExecutionTab } from './ExecutionTab'
import { fallbackRequestFlow } from '../../../../../shared/request-flow-view'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

const request = {
  id: 'r1',
  projectId: 'p1',
  type: 'change_request',
  status: 'executing'
} as OrcaRequest

function summaryWith(executionMode: ExecutionSummary['executionMode']): ExecutionSummary {
  return {
    requestId: 'r1',
    status: 'executing',
    executionMode,
    progress: {
      phasesTotal: 2,
      phasesDone: 0,
      tasksTotal: 4,
      tasksDone: 1,
      tasksFailed: 0,
      tasksRunning: 1
    },
    running: [],
    failed: [],
    needsYou: [],
    handover: { state: 'none', archiveAttempts: 0 },
    maxAttempts: 2
  }
}

function renderTab(
  executionMode: ExecutionSummary['executionMode'],
  flowType: 'change_request' | 'docs'
) {
  return render(
    <ExecutionTab
      request={request}
      flow={fallbackRequestFlow(flowType, 'L')}
      execution={{
        summary: summaryWith(executionMode),
        isLoading: false,
        error: null,
        unsupported: false,
        refetch: vi.fn()
      }}
      busy={false}
      onChanged={vi.fn()}
      onReopen={vi.fn()}
      onStop={vi.fn()}
      onAnswerClarification={vi.fn()}
    />
  )
}

afterEach(cleanup)

describe('ExecutionTab run mode label', () => {
  it('shows "Runs all steps on its own" for a continuous request', () => {
    renderTab('continuous', 'change_request')
    expect(screen.getByTestId('execution-run-mode')).toHaveTextContent('Runs all steps on its own')
  })

  it('shows "Asks before each step" for step by step when the flow has phases', () => {
    renderTab('step_by_step', 'change_request')
    expect(screen.getByTestId('execution-run-mode')).toHaveTextContent('Asks before each step')
  })

  it('shows nothing for step by step without phases, or when the mode is not known', () => {
    renderTab('step_by_step', 'docs')
    expect(screen.queryByTestId('execution-run-mode')).toBeNull()
    cleanup()
    renderTab(undefined, 'change_request')
    expect(screen.queryByTestId('execution-run-mode')).toBeNull()
  })
})
