// @vitest-environment happy-dom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useLazyChartMount } from '../useLazyChartMount'

globalThis.IS_REACT_ACT_ENVIRONMENT = true
const roots: Root[] = []

const observers: { callback: (e: { isIntersecting: boolean }[]) => void; disconnected: boolean }[] =
  []
class FakeIntersectionObserver {
  record: (typeof observers)[number]
  constructor(callback: (e: { isIntersecting: boolean }[]) => void) {
    this.record = { callback, disconnected: false }
    observers.push(this.record)
  }
  observe(): void {}
  disconnect(): void {
    this.record.disconnected = true
  }
}

function Probe(): React.JSX.Element {
  const { ref, mounted } = useLazyChartMount()
  return <div ref={ref} data-mounted={String(mounted)} />
}

function mount(): HTMLElement {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  roots.push(root)
  act(() => root.render(<Probe />))
  return container
}

afterEach(() => {
  roots.splice(0).forEach((r) => act(() => r.unmount()))
  document.body.replaceChildren()
  observers.length = 0
  vi.unstubAllGlobals()
})

describe('useLazyChartMount', () => {
  it('mounts immediately without IntersectionObserver', () => {
    vi.stubGlobal('IntersectionObserver', undefined)
    expect(mount().firstElementChild?.getAttribute('data-mounted')).toBe('true')
  })

  it('waits for intersection then stays mounted', () => {
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    const c = mount()
    expect(c.firstElementChild?.getAttribute('data-mounted')).toBe('false')
    act(() => observers[0].callback([{ isIntersecting: false }]))
    expect(c.firstElementChild?.getAttribute('data-mounted')).toBe('false')
    act(() => observers[0].callback([{ isIntersecting: true }]))
    expect(c.firstElementChild?.getAttribute('data-mounted')).toBe('true')
    expect(observers[0].disconnected).toBe(true)
    act(() => observers[0].callback([{ isIntersecting: false }]))
    expect(c.firstElementChild?.getAttribute('data-mounted')).toBe('true')
  })

  it('mounts immediately when the document is not visible', () => {
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    expect(mount().firstElementChild?.getAttribute('data-mounted')).toBe('true')
  })
})
