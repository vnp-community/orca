/**
 * use-pane-width-below.ts — FE-CV-TASK-056-05
 *
 * True while the observed element is narrower than `limit` px. Measures the lens pane, not the
 * window: the Review tab can sit in a narrow split.
 *
 * @module components/review-map/dataflow/use-pane-width-below
 */

import { useEffect, useState } from 'react'

export function usePaneWidthBelow(
  ref: React.RefObject<HTMLElement | null>,
  limit: number
): boolean {
  const [below, setBelow] = useState(false)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') {
      return
    }
    const measure = (width: number): void => setBelow(width > 0 && width < limit)
    measure(el.getBoundingClientRect().width)
    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) {
        measure(entry.contentRect.width)
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref, limit])
  return below
}
