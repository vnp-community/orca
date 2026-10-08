// @vitest-environment happy-dom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { ChartLegend } from '../ChartLegend'
import { GateVerdictBadge } from '../GateVerdictBadge'
import { SeverityBadge } from '../SeverityBadge'

globalThis.IS_REACT_ACT_ENVIRONMENT = true

const roots: Root[] = []

function mount(node: React.ReactNode): HTMLElement {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  roots.push(root)
  act(() => root.render(node))
  return container
}

afterEach(() => {
  roots.splice(0).forEach((r) => act(() => r.unmount()))
  document.body.replaceChildren()
})

describe('SeverityBadge / GateVerdictBadge / ChartLegend', () => {
  it('always renders a text label with a glyph', () => {
    const c = mount(<SeverityBadge severity="error" count={3} />)
    expect(c.textContent).toContain('Error')
    expect(c.textContent).toContain('3')
    expect(c.querySelector('svg[data-shape="octagon"]')).not.toBeNull()
  })

  it('gives an aria-label when the text label is hidden', () => {
    const c = mount(<SeverityBadge severity="warning" count={2} withLabel={false} />)
    expect(c.querySelector('[aria-label]')?.getAttribute('aria-label')).toBe('Warning: 2')
  })

  it('treats unknown severity as unknown and sanitises counts', () => {
    const c = mount(<SeverityBadge severity="critical" count={-4} />)
    expect(c.textContent).toContain('Unknown')
    expect(c.textContent).not.toContain('-4')
  })

  it('unknown verdict never uses the pass glyph or conclusive copy', () => {
    const c = mount(<GateVerdictBadge verdict="unknown" />)
    expect(c.querySelector('svg[data-shape="circle-check"]')).toBeNull()
    expect(c.textContent).toBe('Not enough data to conclude')
    expect(c.textContent).not.toMatch(/safe|clean|met/i)
  })

  it('adds stale as text', () => {
    const c = mount(<GateVerdictBadge verdict="pass" stale />)
    expect(c.textContent).toContain('stale')
  })

  it('builds the legend from the encoding table', () => {
    const c = mount(<ChartLegend kind="severity" levels={['error', 'warning', 'info']} />)
    expect(c.querySelectorAll('li')).toHaveLength(3)
    expect(c.textContent).toContain('Warning')
  })
})
