// @vitest-environment happy-dom
import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@xyflow/react', () => ({
  BaseEdge: ({ style, className }: { style: Record<string, unknown>; className?: string }) => (
    <path data-testid="edge" data-dash={String(style.strokeDasharray ?? '')} data-opacity={String(style.opacity)} className={className} />
  ),
  EdgeLabelRenderer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  getBezierPath: () => ['M0,0', 0, 0]
}))

import { GraphEdgeLine, type GraphEdgeData } from './GraphEdgeLine'

afterEach(cleanup)
const BASE = { id: 'e', sourceX: 0, sourceY: 0, targetX: 1, targetY: 1, sourcePosition: 'right', targetPosition: 'left' }
const edge = (data: GraphEdgeData): React.JSX.Element => (
  <GraphEdgeLine {...(BASE as unknown as Parameters<typeof GraphEdgeLine>[0])} data={data as never} />
)

describe('GraphEdgeLine', () => {
  it('renders +, - or no label per change', () => {
    const a = render(edge({ change: 'added' }))
    expect(a.container.textContent).toBe('+')
    a.unmount()
    const r = render(edge({ change: 'removed' }))
    expect(r.container.textContent).toBe('−')
    expect(r.getByTestId('edge').dataset.dash).toBe('6 4')
    r.unmount()
    const u = render(edge({ change: 'unchanged' }))
    expect(u.container.textContent).toBe('')
  })

  it('only animates running edges when motion is allowed', () => {
    const on = render(edge({ running: true }))
    expect(on.getByTestId('edge').className).toContain('animated')
    on.unmount()
    window.matchMedia = ((q: string) => ({ matches: q.includes('reduce'), addEventListener() {}, removeEventListener() {} })) as never
    const off = render(edge({ running: true }))
    expect(off.getByTestId('edge').className ?? '').not.toContain('animated')
  })
})
