// @vitest-environment happy-dom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChartFrame, type ChartFrameProps } from '../ChartFrame'

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
  vi.unstubAllGlobals()
})

const TABLE = {
  caption: 'Findings by turn',
  columns: [
    { key: 'turn', label: 'Turn' },
    { key: 'n', label: 'Count', align: 'end' as const }
  ],
  rows: [
    { turn: 'T1', n: 3 },
    { turn: 'T2', n: null }
  ]
}

function frame(overrides: Partial<ChartFrameProps> = {}): React.JSX.Element {
  return (
    <ChartFrame
      id="f1"
      title="Findings"
      summary="Findings: rose from 1 to 3"
      status="ready"
      table={TABLE}
      minHeight={120}
      {...overrides}
    >
      {({ width, height }) => <svg data-testid="drawing" data-w={width} data-h={height} />}
    </ChartFrame>
  )
}

describe('ChartFrame', () => {
  it('draws children with role=img and the summary label when ready', () => {
    const c = mount(frame())
    const img = c.querySelector('[role="img"]')
    expect(img?.getAttribute('aria-label')).toBe('Findings: rose from 1 to 3')
    expect(c.querySelector('[data-testid="drawing"]')).not.toBeNull()
    expect(c.querySelector('[data-chart-surface]')?.getAttribute('style')).toContain(
      'min-height: 120px'
    )
  })

  it('shows a skeleton at the reserved height while loading and does not draw', () => {
    const c = mount(frame({ status: 'loading' }))
    expect(c.querySelector('[data-testid="drawing"]')).toBeNull()
    expect(c.querySelector('figure')?.getAttribute('aria-busy')).toBe('true')
    expect(c.innerHTML).toContain('height: 120px')
  })

  it('explains why an empty chart is empty without claiming health', () => {
    const c = mount(frame({ status: 'empty', emptyReason: 'Checks have not run for this scope.' }))
    expect(c.textContent).toContain('Checks have not run for this scope.')
    expect(c.textContent).not.toMatch(/no issues|clean|safe/i)
    expect(c.querySelector('[data-testid="drawing"]')).toBeNull()
    const d = mount(frame({ status: 'empty' }))
    expect(d.textContent).not.toMatch(/no issues|clean|safe/i)
  })

  it('shows a persistent inline error with a working Retry and no drawing', () => {
    const onRetry = vi.fn()
    const c = mount(frame({ status: 'error', error: { message: 'Backend unavailable', onRetry } }))
    expect(c.querySelector('[role="alert"]')?.textContent).toContain('Backend unavailable')
    const button = [...c.querySelectorAll('button')].find((b) => b.textContent === 'Retry')!
    act(() => button.click())
    expect(onRetry).toHaveBeenCalledTimes(1)
    expect(c.querySelector('[data-testid="drawing"]')).toBeNull()
  })

  it('keeps drawing a stale chart and shows the stale note', () => {
    const c = mount(frame({ status: 'stale', staleNote: 'HEAD a41c9e0' }))
    expect(c.querySelector('[data-testid="drawing"]')).not.toBeNull()
    expect(c.querySelector('[data-chart-stale]')?.textContent).toContain('HEAD a41c9e0')
  })

  it('keeps the table in the DOM and toggles its visibility with aria-pressed', () => {
    const c = mount(frame())
    const table = c.querySelector('[data-chart-table]')!
    expect(table.className).toContain('sr-only')
    expect(c.querySelector('table caption')?.textContent).toBe('Findings by turn')
    const toggle = c.querySelector('button[aria-pressed]')!
    expect(toggle.getAttribute('aria-pressed')).toBe('false')
    act(() => (toggle as HTMLButtonElement).click())
    expect(toggle.getAttribute('aria-pressed')).toBe('true')
    expect(c.querySelector('[data-chart-table]')?.className).not.toContain('sr-only')
    expect(c.querySelector('[data-testid="drawing"]')).toBeNull()
  })

  it('lets grid charts own their role when surface is custom', () => {
    const c = mount(frame({ surface: 'custom' }))
    expect(c.querySelector('[role="img"]')).toBeNull()
  })

  it('has no transition or animation classes in the chart surface', () => {
    const c = mount(frame({ status: 'loading' }))
    const surface = c.querySelector('[data-chart-surface]')!
    expect(surface.innerHTML).not.toMatch(/transition|animate-(?!none)/)
  })

  it('requires the table prop at compile time', () => {
    const element = (
      // @ts-expect-error table is mandatory
      <ChartFrame id="x" title="t" summary="s" status="ready" minHeight={10}>
        {() => null}
      </ChartFrame>
    )
    expect(element).toBeTruthy()
  })

  it('creates exactly one ResizeObserver per frame', () => {
    const created: unknown[] = []
    class FakeResizeObserver {
      constructor() {
        created.push(this)
      }
      observe(): void {}
      disconnect(): void {}
    }
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
    mount(frame())
    expect(created).toHaveLength(1)
  })
})
