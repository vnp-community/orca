import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, vi } from 'vitest'

globalThis.IS_REACT_ACT_ENVIRONMENT = true

const roots: Root[] = []

export function mountChart(node: React.ReactNode): HTMLElement {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  roots.push(root)
  act(() => root.render(node))
  return container
}

export function registerChartCleanup(): void {
  // Why: happy-dom ships an IntersectionObserver that never reports, which would keep lazy charts unmounted.
  beforeEach(() => {
    vi.stubGlobal('IntersectionObserver', undefined)
  })
  afterEach(() => {
    roots.splice(0).forEach((r) => act(() => r.unmount()))
    document.body.replaceChildren()
    vi.unstubAllGlobals()
  })
}
