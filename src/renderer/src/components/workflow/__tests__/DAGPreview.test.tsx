// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { DAGPreview } from '../DAGPreview'

// Mock ReactFlow since it requires a real DOM size and ResizeObserver to render properly
vi.mock('@xyflow/react', () => {
  return {
    ReactFlow: ({ nodes, edges }: any) => (
      <div data-testid="mock-react-flow">
        <div data-testid="nodes-count">{nodes.length}</div>
        <div data-testid="edges-count">{edges.length}</div>
        <div data-testid="nodes-json">{JSON.stringify(nodes.map((n: any) => ({ id: n.id, position: n.position, style: n.style })))}</div>
      </div>
    ),
    Background: () => <div />,
    Controls: () => <div />
  }
})

describe('DAGPreview', () => {
  beforeEach(() => cleanup())

  it('empty steps array → shows empty state', () => {
    render(<DAGPreview steps={[]} />)
    expect(screen.getByTestId('dag-preview-empty')).toBeInTheDocument()
    expect(screen.getByText('Add steps to see the DAG')).toBeInTheDocument()
  })

  it('linear deps: 2 steps with A→B dependency → 2 waves', () => {
    const steps = [
      { id: 'A', name: 'Step A', type: 'shell', dependsOn: [] },
      { id: 'B', name: 'Step B', type: 'shell', dependsOn: ['A'] }
    ] as any
    render(<DAGPreview steps={steps} />)
    
    expect(screen.getByTestId('nodes-count')).toHaveTextContent('2')
    expect(screen.getByTestId('edges-count')).toHaveTextContent('1')
    
    const nodesJson = screen.getByTestId('nodes-json').textContent!
    const nodes = JSON.parse(nodesJson)
    expect(nodes.find((n: any) => n.id === 'A').position.x).toBe(0)     // Wave 0
    expect(nodes.find((n: any) => n.id === 'B').position.x).toBe(200)   // Wave 1
  })

  it('parallel steps (no deps): all in wave 0, positioned at x=0', () => {
    const steps = [
      { id: 'A', name: 'Step A', type: 'shell', dependsOn: [] },
      { id: 'B', name: 'Step B', type: 'shell', dependsOn: [] }
    ] as any
    render(<DAGPreview steps={steps} />)
    
    const nodesJson = screen.getByTestId('nodes-json').textContent!
    const nodes = JSON.parse(nodesJson)
    expect(nodes.find((n: any) => n.id === 'A').position.x).toBe(0)
    expect(nodes.find((n: any) => n.id === 'B').position.x).toBe(0)
  })

  it('dependency creates an edge between the two nodes', () => {
    const steps = [
      { id: 'A', name: 'Step A', type: 'shell', dependsOn: [] },
      { id: 'B', name: 'Step B', type: 'shell', dependsOn: ['A'] }
    ] as any
    render(<DAGPreview steps={steps} />)
    
    expect(screen.getByTestId('edges-count')).toHaveTextContent('1')
  })

  it('selectedStepId → that node has highlighted border style', () => {
    const steps = [
      { id: 'A', name: 'Step A', type: 'shell', dependsOn: [] }
    ] as any
    render(<DAGPreview steps={steps} selectedStepId="A" />)
    
    const nodesJson = screen.getByTestId('nodes-json').textContent!
    const nodes = JSON.parse(nodesJson)
    expect(nodes[0].style.border).toBe('2px solid #3b82f6')
  })
})
