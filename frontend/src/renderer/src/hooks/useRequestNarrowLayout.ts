/**
 * True when the viewport is narrower than 768px: the Requests page then shows
 * list and detail one at a time instead of side by side.
 *
 * @module hooks/useRequestNarrowLayout
 */

import { useEffect, useState } from 'react'

const QUERY = '(max-width: 767px)'

export function useRequestNarrowLayout(): boolean {
  const [narrow, setNarrow] = useState(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(QUERY).matches
      : false
  )
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') {return}
    const mql = window.matchMedia(QUERY)
    const onChange = (e: MediaQueryListEvent): void => setNarrow(e.matches)
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [])
  return narrow
}
