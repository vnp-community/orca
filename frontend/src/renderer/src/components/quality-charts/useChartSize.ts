import { useEffect, useLayoutEffect, useState, type RefObject } from 'react'
import type { ChartSize } from './chart-frame-types'

const UNMEASURED: ChartSize = { width: 0, height: 0 }

function roundSize(width: number, height: number): ChartSize {
  return { width: Math.round(width), height: Math.round(height) }
}

/**
 * One ResizeObserver per frame, coalesced through requestAnimationFrame. Why: cells must never
 * own observers (hundreds of tiles); environments without ResizeObserver keep the initial size.
 */
export function useChartSize(ref: RefObject<HTMLElement | null>): ChartSize {
  const [size, setSize] = useState<ChartSize>(UNMEASURED)

  useLayoutEffect(() => {
    const rect = ref.current?.getBoundingClientRect()
    if (rect) {
      setSize((prev) => {
        const next = roundSize(rect.width, rect.height)
        return prev.width === next.width && prev.height === next.height ? prev : next
      })
    }
  }, [ref])

  useEffect(() => {
    const element = ref.current
    if (!element || typeof ResizeObserver === 'undefined') {
      return
    }
    let frame: number | null = null
    let latest: ChartSize | null = null
    const observer = new ResizeObserver((entries) => {
      const entry = entries.at(-1)
      if (!entry) {
        return
      }
      latest = roundSize(entry.contentRect.width, entry.contentRect.height)
      if (frame !== null) {
        return
      }
      frame = requestAnimationFrame(() => {
        frame = null
        const next = latest
        if (next) {
          setSize((prev) =>
            prev.width === next.width && prev.height === next.height ? prev : next
          )
        }
      })
    })
    observer.observe(element)
    return () => {
      observer.disconnect()
      if (frame !== null) {
        cancelAnimationFrame(frame)
      }
    }
  }, [ref])

  return size
}
