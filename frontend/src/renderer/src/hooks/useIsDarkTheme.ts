/**
 * useIsDarkTheme.ts — FE-CV-TASK-056-01
 *
 * Same formula as MarkdownPreview/MermaidViewer, shared and reactive to OS changes.
 */

import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia(DARK_QUERY).matches
}

export function useIsDarkTheme(): boolean {
  const theme = useAppStore((s) => s.settings?.theme)
  const [systemDark, setSystemDark] = useState(systemPrefersDark)

  useEffect(() => {
    if (theme !== 'system' || typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return
    }
    const mql = window.matchMedia(DARK_QUERY)
    setSystemDark(mql.matches)
    const onChange = (e: MediaQueryListEvent): void => setSystemDark(e.matches)
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [theme])

  return theme === 'dark' || (theme === 'system' && systemDark)
}
