import { useEffect, useRef, useState, type RefObject } from 'react'

function mountsImmediately(): boolean {
  // Why: a hidden Electron window never reports intersections, so do not wait on it.
  return (
    typeof IntersectionObserver === 'undefined' ||
    (typeof document !== 'undefined' && document.visibilityState !== 'visible')
  )
}

/** Defers heavy chart drawing until the placeholder first scrolls into view; never unmounts. */
export function useLazyChartMount<T extends HTMLElement = HTMLDivElement>(): {
  ref: RefObject<T | null>
  mounted: boolean
} {
  const ref = useRef<T | null>(null)
  const [mounted, setMounted] = useState(mountsImmediately)

  useEffect(() => {
    if (mounted) {
      return
    }
    const element = ref.current
    if (!element) {
      return
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        setMounted(true)
        observer.disconnect()
      }
    })
    observer.observe(element)
    return () => observer.disconnect()
  }, [mounted])

  return { ref, mounted }
}
