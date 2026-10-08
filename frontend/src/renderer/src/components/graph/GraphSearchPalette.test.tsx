// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GraphSearchPalette } from './GraphSearchPalette'
import { gNode } from './graph-test-fixtures'

afterEach(cleanup)
;(globalThis as { ResizeObserver?: unknown }).ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} }
Element.prototype.scrollIntoView ??= () => {}

const nodes = [gNode('1', { label: 'Alpha service', kind: 'service', group: 'core' }), gNode('2', { label: 'Beta table', kind: 'table' })]

describe('GraphSearchPalette', () => {
  it('filters by query and picks a node', () => {
    const onPick = vi.fn()
    render(<GraphSearchPalette open nodes={nodes} onOpenChange={vi.fn()} onPick={onPick} />)
    fireEvent.change(screen.getByPlaceholderText('Search nodes...'), { target: { value: 'beta' } })
    expect(screen.queryByText('Alpha service')).toBeNull()
    fireEvent.click(screen.getByText('Beta table'))
    expect(onPick).toHaveBeenCalledWith(nodes[1])
  })

  it('shows an empty message when nothing matches', () => {
    render(<GraphSearchPalette open nodes={nodes} onOpenChange={vi.fn()} onPick={vi.fn()} />)
    fireEvent.change(screen.getByPlaceholderText('Search nodes...'), { target: { value: 'zzz' } })
    expect(screen.getByText('No matching nodes')).toBeInTheDocument()
  })
})
