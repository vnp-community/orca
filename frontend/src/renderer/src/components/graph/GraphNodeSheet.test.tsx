// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GraphNodeSheet } from './GraphNodeSheet'
import { gEdge, gNode, gPayload } from './graph-test-fixtures'

afterEach(cleanup)

describe('GraphNodeSheet', () => {
  it('renders labels as plain text, never as HTML', () => {
    const payload = gPayload([gNode('a', { label: '<script>alert(1)</script>' }), gNode('b')], [gEdge('a', 'b')])
    render(<GraphNodeSheet payload={payload} nodeId="a" onClose={vi.fn()} />)
    expect(screen.getAllByText('<script>alert(1)</script>').length).toBeGreaterThan(0)
    expect(document.querySelector('script')).toBeNull()
  })

  it('lists edges, offers Open and renders nothing for an unknown node', () => {
    const onOpenNode = vi.fn()
    const payload = gPayload([gNode('a'), gNode('b', { meta: { findingIds: ['f-1'] } })], [gEdge('a', 'b')])
    const { rerender } = render(<GraphNodeSheet payload={payload} nodeId="b" onClose={vi.fn()} onOpenNode={onOpenNode} />)
    expect(screen.getByText('f-1')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Open'))
    expect(onOpenNode).toHaveBeenCalled()
    rerender(<GraphNodeSheet payload={payload} nodeId="zzz" onClose={vi.fn()} />)
    expect(screen.queryByText('f-1')).toBeNull()
  })
})
