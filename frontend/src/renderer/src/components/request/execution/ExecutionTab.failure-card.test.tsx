// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const cardProps = vi.fn()
vi.mock('./ExecutionFailureCard', () => ({
  ExecutionFailureCard: (props: Record<string, unknown>) => {
    cardProps(props)
    return <div data-testid="execution-failure-card" />
  }
}))
vi.mock('../../../hooks/usePlanTree', () => ({
  usePlanTree: () => ({ tree: null, approvals: [], refetch: vi.fn() })
}))
vi.mock('../../../hooks/usePlanDecision', () => ({ usePlanDecision: () => ({}) }))
vi.mock('./ExecutionNeedsYou', () => ({ ExecutionNeedsYou: () => null }))
vi.mock('./ExecutionStepList', () => ({ ExecutionStepList: () => null }))
vi.mock('./ExecutionSummaryStrip', () => ({ ExecutionSummaryStrip: () => null }))
vi.mock('./HandoverRow', () => ({ HandoverRow: () => null }))
vi.mock('./ExecutionTaskSheet', () => ({
  ExecutionTaskSheet: () => null,
  useExecutionTaskSheet: () => ({ open: false, onOpenChange: vi.fn(), openTask: vi.fn() })
}))

import { ExecutionTab } from './ExecutionTab'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../../shared/request-types'

const request = {
  id: 'r1',
  projectId: 'p1',
  title: 'T',
  type: 'change_request',
  status: 'request_backlog',
  baseRef: 'gone/branch',
  baseRefLocked: false,
  version: 3
} as OrcaRequest

const summary: ExecutionSummary = {
  requestId: 'r1',
  status: 'request_backlog',
  progress: {
    phasesTotal: 1,
    phasesDone: 0,
    tasksTotal: 2,
    tasksDone: 0,
    tasksFailed: 1,
    tasksRunning: 0
  },
  running: [],
  failed: [
    { taskId: 't1', title: 'TG-1', attempts: 2, maxAttempts: 2, lastCause: 'base_ref_not_found' }
  ],
  needsYou: [],
  handover: { state: 'none', archiveAttempts: 0 },
  maxAttempts: 2
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('ExecutionTab failure card', () => {
  it('hands the card the Request it is about and the pane refetch', () => {
    const onChanged = vi.fn()
    render(
      <ExecutionTab
        request={request}
        execution={{ summary, isLoading: false, error: null, unsupported: false, refetch: vi.fn() }}
        busy={false}
        onChanged={onChanged}
        onReopen={vi.fn()}
        onStop={vi.fn()}
        onAnswerClarification={vi.fn()}
      />
    )
    expect(screen.getByTestId('execution-failure-card')).toBeInTheDocument()
    const props = cardProps.mock.calls.at(-1)?.[0] as {
      request: OrcaRequest
      onChanged: () => void
    }
    expect(props.request).toBe(request)
    expect(props.onChanged).toBe(onChanged)
  })
})
