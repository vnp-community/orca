// @vitest-environment happy-dom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { ChartTextAlternative } from '../ChartTextAlternative'

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

const data = {
  caption: 'Caption text',
  columns: [
    { key: 'file', label: 'File' },
    { key: 'n', label: 'Count', align: 'end' as const }
  ],
  rows: [
    { file: '<script>alert(1)</script>', n: null },
    { file: 'b.ts', n: 0 }
  ]
}

describe('ChartTextAlternative', () => {
  it('has a caption and scoped column headers', () => {
    const c = mount(<ChartTextAlternative data={data} visible />)
    expect(c.querySelector('caption')?.textContent).toBe('Caption text')
    const heads = [...c.querySelectorAll('th')]
    expect(heads.map((h) => h.getAttribute('scope'))).toEqual(['col', 'col'])
  })

  it('renders null as a dash and real zero as 0', () => {
    const c = mount(<ChartTextAlternative data={data} visible />)
    const cells = [...c.querySelectorAll('tbody td')].map((td) => td.textContent)
    expect(cells).toEqual(['<script>alert(1)</script>', '—', 'b.ts', '0'])
  })

  it('renders backend strings as plain text', () => {
    const c = mount(<ChartTextAlternative data={data} visible />)
    expect(c.querySelector('script')).toBeNull()
  })

  it('uses sr-only, never display:none, when not visible', () => {
    const c = mount(<ChartTextAlternative data={data} visible={false} />)
    const wrapper = c.querySelector('[data-chart-table]') as HTMLElement
    expect(wrapper.className).toContain('sr-only')
    expect(wrapper.style.display).not.toBe('none')
    expect(c.querySelectorAll('tbody tr')).toHaveLength(2)
  })
})
