/**
 * useDocumentThemeVersion — FE-OR-T210
 *
 * A number that changes whenever the page's theme does (its `class` or `data-theme`), seen from the DOM itself rather than from
 * a setting, so that what is read from the computed style afterwards is already the new theme's. Used to repaint a drawing from
 * the tokens in force without asking the server again.
 *
 * @module hooks/useDocumentThemeVersion
 */

import { useEffect, useState } from 'react'

export function useDocumentThemeVersion(): number {
  const [version, setVersion] = useState(0)
  useEffect(() => {
    if (typeof MutationObserver === 'undefined' || typeof document === 'undefined') {
      return undefined
    }
    const observer = new MutationObserver(() => setVersion((v) => v + 1))
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class', 'data-theme', 'style']
    })
    return () => observer.disconnect()
  }, [])
  return version
}
