// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { GraphMini } from './GraphMini'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

afterEach(cleanup)

describe('GraphMini', () => {
  it('draws at most 12 nodes and always shows a text summary', () => {
    const payload = gPayload(Array.from({ length: 40 }, (_, i) => gNode(`n${i}`)), [gEdge('n0', 'n1', { change: 'added' })])
    const { container } = render(<GraphMini payload={payload} />)
    expect(container.querySelectorAll('rect').length).toBeLessThanOrEqual(12)
    expect(container.textContent).toContain('40 nodes')
  })

  it('hides the drawing from assistive tech when a list equivalent exists', () => {
    const { container } = render(<GraphMini payload={gPayload([gNode('a')])} hasListEquivalent />)
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('shows "Not assessed" instead of drawing when there is no payload', () => {
    const { container } = render(<GraphMini payload={null} />)
    expect(container.textContent).toContain('Not assessed')
    expect(container.querySelector('svg')).toBeNull()
  })
})
