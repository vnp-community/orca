// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TaskAgentActivityPanel } from './TaskAgentActivityPanel'
import type { TaskAgentOutputState } from '../../../hooks/useTaskAgentOutput'
import type { OrcaTask } from '../../../../../shared/task-types'

const storeState = vi.hoisted(() => ({
  templates: [] as unknown[],
  tasks: [] as { parentId?: string }[]
}))
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector: (s: unknown) => unknown) => selector(storeState)),
    { getState: () => ({ settings: {} }) }
  )
}))

const outputMock = vi.hoisted(() => vi.fn())
vi.mock('../../../hooks/useTaskAgentOutput', () => ({ useTaskAgentOutput: outputMock }))

const TASK = { id: 't1', title: 'Negate', requestId: 'r1' } as OrcaTask

function output(over: Partial<TaskAgentOutputState> = {}): TaskAgentOutputState {
  return { lines: [], olderHidden: false, runId: null, status: 'live', lastFrameAt: null, ...over }
}

beforeEach(() => {
  storeState.templates = []
  storeState.tasks = []
  outputMock.mockReset()
  outputMock.mockReturnValue(output())
})
afterEach(() => cleanup())

const LINES: TaskAgentOutputState['lines'] = [
  { id: 0, kind: 'info', text: 'Session started (m)' },
  { id: 1, kind: 'tool', text: 'Read src/calc.py' },
  { id: 2, kind: 'result', text: 'ok · 3 lines' },
  { id: 3, kind: 'result', text: 'error: Exit code 1', tone: 'error' },
  { id: 4, kind: 'end', text: 'Done · 2 turns', tone: 'ok' }
]

describe('TaskAgentActivityPanel', () => {
  it('subscribes for a direct-agent task and shows the lines in order with their kind', () => {
    outputMock.mockReturnValue(output({ lines: LINES }))
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(outputMock).toHaveBeenCalledWith('t1')
    const rows = screen.getAllByTestId('task-agent-activity-line')
    expect(rows.map((r) => r.textContent)).toEqual([
      '·Session started (m)',
      '▸Read src/calc.py',
      '↳ok · 3 lines',
      '↳error: Exit code 1',
      '■Done · 2 turns'
    ])
    expect(rows.map((r) => r.getAttribute('data-kind'))).toEqual([
      'info',
      'tool',
      'result',
      'result',
      'end'
    ])
    expect(rows[3]).toHaveAttribute('data-tone', 'error')
    expect(screen.getByRole('log')).toBeInTheDocument()
  })

  it('shows the Live indicator only while the task runs, then Finished', () => {
    outputMock.mockReturnValue(output({ lines: LINES }))
    const { rerender } = render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.getByTestId('task-agent-activity-live')).toHaveTextContent('Live')
    expect(screen.queryByTestId('task-agent-activity-status')).toBeNull()
    expect(screen.getByTestId('task-agent-activity')).toHaveAttribute('data-state', 'running')

    rerender(<TaskAgentActivityPanel task={TASK} isRunning={false} />)
    expect(screen.queryByTestId('task-agent-activity-live')).toBeNull()
    expect(screen.getByTestId('task-agent-activity-status')).toHaveTextContent('Finished')
    expect(screen.getByTestId('task-agent-activity')).toHaveAttribute('data-state', 'finished')
  })

  it('points to the Result tab when idle with nothing live and the task has saved output', () => {
    const open = vi.fn()
    render(<TaskAgentActivityPanel task={TASK} isRunning={false} onOpenSavedResult={open} />)
    expect(screen.getByTestId('task-agent-activity')).toHaveAttribute('data-state', 'idle')
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent(
      'See the saved output in the Result tab'
    )
    fireEvent.click(screen.getByTestId('task-agent-activity-open-result'))
    expect(open).toHaveBeenCalledTimes(1)
  })

  it('shows no Result pointer without saved output, while running, or with lines', () => {
    const { rerender } = render(<TaskAgentActivityPanel task={TASK} isRunning={false} />)
    expect(screen.queryByTestId('task-agent-activity-open-result')).toBeNull()
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent('Nothing to show yet')

    const open = vi.fn()
    rerender(<TaskAgentActivityPanel task={TASK} isRunning onOpenSavedResult={open} />)
    expect(screen.queryByTestId('task-agent-activity-open-result')).toBeNull()

    outputMock.mockReturnValue(output({ lines: LINES }))
    rerender(<TaskAgentActivityPanel task={TASK} isRunning={false} onOpenSavedResult={open} />)
    expect(screen.queryByTestId('task-agent-activity-open-result')).toBeNull()
  })

  it('keeps the Result pointer when the live channel is unavailable (the saved output is all there is)', () => {
    outputMock.mockReturnValue(output({ status: 'unavailable' }))
    render(<TaskAgentActivityPanel task={TASK} isRunning={false} onOpenSavedResult={vi.fn()} />)
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent(
      'needs a connected runtime'
    )
    expect(screen.getByTestId('task-agent-activity-open-result')).toBeInTheDocument()
  })

  it('waits for output while running and explains the empty state when idle', () => {
    const { rerender } = render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent(
      'Waiting for the agent'
    )
    rerender(<TaskAgentActivityPanel task={TASK} isRunning={false} />)
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent(
      'not kept after you reload'
    )
    expect(screen.getByTestId('task-agent-activity-status')).toHaveTextContent('Idle')
  })

  it('says live activity is unavailable without a runtime environment, and when it is reconnecting', () => {
    outputMock.mockReturnValue(output({ status: 'unavailable' }))
    const { rerender } = render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.getByTestId('task-agent-activity-empty')).toHaveTextContent(
      'needs a connected runtime'
    )
    outputMock.mockReturnValue(output({ status: 'reconnecting', lines: LINES }))
    rerender(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.getByTestId('task-agent-activity-reconnecting')).toBeInTheDocument()
    expect(screen.getAllByTestId('task-agent-activity-line')).toHaveLength(5)
  })

  it('notes that older activity is hidden', () => {
    outputMock.mockReturnValue(output({ lines: LINES, olderHidden: true }))
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.getByTestId('task-agent-activity-older')).toBeInTheDocument()
  })

  it('renders agent text as text, never as markup', () => {
    outputMock.mockReturnValue(
      output({ lines: [{ id: 0, kind: 'text', text: '<img src=x onerror=alert(1)>' }] })
    )
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    const row = screen.getByTestId('task-agent-activity-line')
    expect(row).toHaveTextContent('<img src=x onerror=alert(1)>')
    expect(row.querySelector('img')).toBeNull()
  })

  it('collapses and expands', () => {
    outputMock.mockReturnValue(output({ lines: LINES }))
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    const toggle = screen.getByTestId('task-agent-activity-toggle')
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByTestId('task-agent-activity-lines')).toBeNull()
    fireEvent.click(toggle)
    expect(screen.getAllByTestId('task-agent-activity-line')).toHaveLength(5)
  })

  it('stops following when the user scrolls up and offers a way back to the latest line', () => {
    outputMock.mockReturnValue(output({ lines: LINES }))
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    const box = screen.getByTestId('task-agent-activity-lines')
    Object.defineProperty(box, 'scrollHeight', { configurable: true, value: 1000 })
    Object.defineProperty(box, 'clientHeight', { configurable: true, value: 200 })
    expect(screen.queryByTestId('task-agent-activity-jump')).toBeNull()

    box.scrollTop = 100
    fireEvent.scroll(box)
    const jump = screen.getByTestId('task-agent-activity-jump')
    fireEvent.click(jump)
    expect(box.scrollTop).toBe(1000)
    expect(screen.queryByTestId('task-agent-activity-jump')).toBeNull()
  })

  it('follows new lines while the user is at the bottom', () => {
    outputMock.mockReturnValue(output({ lines: LINES.slice(0, 2) }))
    const { rerender } = render(<TaskAgentActivityPanel task={TASK} isRunning />)
    const box = screen.getByTestId('task-agent-activity-lines')
    Object.defineProperty(box, 'scrollHeight', { configurable: true, value: 500 })
    outputMock.mockReturnValue(output({ lines: LINES }))
    rerender(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(box.scrollTop).toBe(500)
  })

  it('does not subscribe or render for an orchestration task', () => {
    storeState.tasks = [{ parentId: 't1' }]
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(screen.queryByTestId('task-agent-activity')).toBeNull()
    expect(outputMock).toHaveBeenCalledWith(null)
  })

  it('keeps the live indicator inside the panel', () => {
    render(<TaskAgentActivityPanel task={TASK} isRunning />)
    expect(
      within(screen.getByTestId('task-agent-activity')).getByTestId('task-agent-activity-live')
    ).toBeInTheDocument()
  })
})
