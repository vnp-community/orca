// @vitest-environment happy-dom

import { act, useRef } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useChartSize } from '../useChartSize'

globalThis.IS_REACT_ACT_ENVIRONMENT = true
const roots: Root[] = []

type Callback = (entries: { contentRect: { width: number; height: number } }[]) => void
const observers: { callback: Callback; disconnected: boolean }[] = []

class FakeResizeObserver {
  record: { callback: Callback; disconnected: boolean }
  constructor(callback: Callback) {
    this.record = { callback, disconnected: false }
    observers.push(this.record)
  }
  observe(): void {}
  disconnect(): void {
    this.record.disconnected = true
  }
}

function Probe(): React.JSX.Element {
  const ref = useRef<HTMLDivElement | null>(null)
  const size = useChartSize(ref)
  return <div ref={ref} data-size={`${size.width}x${size.height}`} />
}

function mount(node: React.ReactNode): { container: HTMLElement; root: Root } {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  roots.push(root)
  act(() => root.render(node))
  return { container, root }
}

afterEach(() => {
  roots.splice(0).forEach((r) => act(() => r.unmount()))
  document.body.replaceChildren()
  observers.length = 0
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('useChartSize', () => {
  it('does not throw without ResizeObserver and reports 0x0', () => {
    vi.stubGlobal('ResizeObserver', undefined)
    const { container } = mount(<Probe />)
    expect(container.firstElementChild?.getAttribute('data-size')).toBe('0x0')
  })

  it('coalesces bursts through one animation frame and rounds sizes', () => {
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
    const frames: (() => void)[] = []
    vi.stubGlobal('requestAnimationFrame', (cb: () => void) => frames.push(cb))
    vi.stubGlobal('cancelAnimationFrame', () => {})
    const { container } = mount(<Probe />)
    expect(observers).toHaveLength(1)
    act(() => {
      observers[0].callback([{ contentRect: { width: 100.4, height: 50 } }])
      observers[0].callback([{ contentRect: { width: 200.6, height: 80 } }])
    })
    expect(frames).toHaveLength(1)
    act(() => frames[0]())
    expect(container.firstElementChild?.getAttribute('data-size')).toBe('201x80')
  })

  it('disconnects on unmount', () => {
    vi.stubGlobal('ResizeObserver', FakeResizeObserver)
    const { root } = mount(<Probe />)
    act(() => root.unmount())
    roots.length = 0
    expect(observers[0].disconnected).toBe(true)
  })
})
