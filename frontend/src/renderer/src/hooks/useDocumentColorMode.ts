import { useEffect, useState } from 'react'

export type DocumentColorMode = 'light' | 'dark'

function readColorMode(): DocumentColorMode {
  if (typeof document === 'undefined') {return 'light'}
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

/** Why: the app drives themes via the `.dark` class on <html>, not prefers-color-scheme. */
export function useDocumentColorMode(): DocumentColorMode {
  const [mode, setMode] = useState<DocumentColorMode>(readColorMode)

  useEffect(() => {
    if (typeof document === 'undefined' || typeof MutationObserver === 'undefined') {return}
    const observer = new MutationObserver(() => setMode(readColorMode()))
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
    setMode(readColorMode())
    return () => observer.disconnect()
  }, [])

  return mode
}
