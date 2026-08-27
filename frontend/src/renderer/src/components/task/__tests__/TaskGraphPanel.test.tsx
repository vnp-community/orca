// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskGraphPanel } from '../TaskGraphPanel'

vi.mock('../TaskGraph', () => ({
  TaskGraph: ({ projectId }: { projectId: string }) => (
    <div data-testid="mock-task-graph">{projectId}</div>
  )
}))

describe('TaskGraphPanel', () => {
  beforeEach(() => cleanup())

  it('renders the panel container with data-testid', () => {
    render(<TaskGraphPanel projectId="p1" />)
    expect(screen.getByTestId('task-graph-panel')).toBeInTheDocument()
  })

  it('passes projectId through to TaskGraph', () => {
    render(<TaskGraphPanel projectId="p1" />)
    expect(screen.getByTestId('mock-task-graph')).toHaveTextContent('p1')
  })
})
