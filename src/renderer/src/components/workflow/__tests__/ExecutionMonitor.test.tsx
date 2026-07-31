// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ExecutionMonitor } from '../ExecutionMonitor'
import { useWorkflowExecution } from '../../../hooks/useWorkflowExecution'

vi.mock('../../../hooks/useWorkflowExecution', () => ({
  useWorkflowExecution: vi.fn()
}))

describe('ExecutionMonitor', () => {
  const cancelExecution = vi.fn()
  const execution = {
    id: 'e1',
    status: 'running',
    startedAt: new Date('2026-07-30T10:00:00Z').toISOString(),
    triggeredBy: 'user',
    definition: {
      name: 'Test Exec',
      steps: [
        { id: 's1', name: 'Step 1', type: 'shell', dependsOn: [] },
        { id: 's2', name: 'Step 2', type: 'shell', dependsOn: ['s1'] }
      ]
    }
  }

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkflowExecution).mockReturnValue({
      execution,
      stepStatuses: { s1: 'completed', s2: 'running' },
      streamingOutput: { e1: ['Log line 1', 'Log line 2'] },
      cancelExecution
    } as any)
  })

  it('renders step list from execution object', () => {
    render(<ExecutionMonitor executionId="e1" />)
    expect(screen.getByText('Test Exec')).toBeInTheDocument()
    expect(screen.getByText('Step 1')).toBeInTheDocument()
    expect(screen.getByText('Step 2')).toBeInTheDocument()
  })

  it('step status=running → shows spinner icon', () => {
    render(<ExecutionMonitor executionId="e1" />)
    const s2Row = screen.getByTestId('step-row-s2')
    expect(s2Row.querySelector('.animate-spin')).toBeInTheDocument()
    expect(s2Row).toHaveTextContent('Running')
  })

  it('step status=completed → shows completed label', () => {
    render(<ExecutionMonitor executionId="e1" />)
    const s1Row = screen.getByTestId('step-row-s1')
    expect(s1Row).toHaveTextContent('Completed')
  })

  it('step status=failed → shows failed label', () => {
    vi.mocked(useWorkflowExecution).mockReturnValue({
      execution,
      stepStatuses: { s1: 'failed', s2: 'pending' },
      streamingOutput: {},
      cancelExecution
    } as any)
    render(<ExecutionMonitor executionId="e1" />)
    const s1Row = screen.getByTestId('step-row-s1')
    expect(s1Row).toHaveTextContent('Failed')
  })

  it('Cancel button calls cancelExecution', () => {
    render(<ExecutionMonitor executionId="e1" />)
    fireEvent.click(screen.getByTestId('cancel-btn'))
    expect(cancelExecution).toHaveBeenCalled()
  })
})
