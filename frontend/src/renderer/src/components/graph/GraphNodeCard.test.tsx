// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { GraphNodeCard } from './GraphNodeCard'
import { GraphGroupNode } from './GraphGroupNode'
import { edgePresentation } from './graph-edge-presentation'
import { gNode } from './graph-test-fixtures'
import type { GraphRisk } from '../../../../shared/graph-types'

afterEach(cleanup)

describe('GraphNodeCard', () => {
  it.each(['low', 'medium', 'high', 'critical', 'unknown'] as GraphRisk[])('shows text and icon for risk %s', (risk) => {
    const { container } = render(<GraphNodeCard node={gNode('a', { risk })} />)
    expect(container.querySelectorAll('svg').length).toBeGreaterThan(1)
    expect(container.querySelector(`[data-risk-level="${risk}"]`)?.textContent).toBeTruthy()
  })

  it('marks removed with a text sign and dims with opacity class', () => {
    const { container } = render(<GraphNodeCard node={gNode('a')} change="removed" dimmed />)
    expect(container.textContent).toContain('−')
    expect(container.firstElementChild).toHaveClass('opacity-30')
  })

  it('opens on Enter, navigates on arrows, selects on click', () => {
    const onOpen = vi.fn()
    const onNavigate = vi.fn()
    const onSelect = vi.fn()
    render(<GraphNodeCard node={gNode('a', { label: 'Alpha' })} onOpen={onOpen} onNavigate={onNavigate} onSelect={onSelect} />)
    const btn = screen.getByRole('button')
    fireEvent.keyDown(btn, { key: 'Enter' })
    fireEvent.keyDown(btn, { key: 'ArrowRight' })
    fireEvent.click(btn)
    expect(onOpen).toHaveBeenCalledWith('a')
    expect(onNavigate).toHaveBeenCalledWith('a', 'right')
    expect(onSelect).toHaveBeenCalledWith('a')
    expect(btn.getAttribute('aria-label')).toContain('Alpha')
  })

  it('renders unknown status as raw text and has no hex in inline style', () => {
    const { container } = render(<GraphNodeCard node={gNode('a', { status: 'weird' })} />)
    expect(container.textContent).toContain('weird')
    expect(container.innerHTML).not.toMatch(/style="[^"]*#[0-9a-fA-F]{3,6}/)
  })
})

describe('GraphGroupNode', () => {
  it('shows the +N summary and toggles', () => {
    const onToggle = vi.fn()
    render(
      <GraphGroupNode
        group={{ id: 'svc', label: 'svc', count: 7, byKind: { module: 7 }, memberIds: [], maxRisk: 'unknown' }}
        onToggleGroup={onToggle}
      />
    )
    expect(screen.getByRole('button').textContent).toContain('+7')
    fireEvent.click(screen.getByRole('button'))
    expect(onToggle).toHaveBeenCalledWith('svc')
  })
})

describe('edgePresentation', () => {
  it('distinguishes the three change kinds without relying on color', () => {
    expect(edgePresentation('added').sign).toBe('+')
    expect(edgePresentation('removed')).toMatchObject({ sign: '−', strokeDasharray: '6 4', opacity: 0.5 })
    expect(edgePresentation('unchanged')).toMatchObject({ sign: null, stroke: 'var(--border)' })
  })
})
