import { useEffect, useState } from 'react'
import type { McpRisk } from '../../../../shared/mcp-types'
import { APPROVE_LOCK_MS } from './mcp-approval-display'

const canCount = (): boolean => document.visibilityState === 'visible' && document.hasFocus()

/**
 * Remaining lock ms. The clock only advances while the window is visible AND focused and is
 * restarted whenever focus returns or `resetKey` (approval id + hash) changes.
 */
export function useMcpApproveLock(risk: McpRisk, resetKey: string): number {
  const total = APPROVE_LOCK_MS[risk]
  const [remaining, setRemaining] = useState(total)

  useEffect(() => {
    let end = Date.now() + total
    setRemaining(total)
    if (total === 0) {
      return
    }
    const restart = (): void => {
      end = Date.now() + total
      setRemaining(total)
    }
    const tick = window.setInterval(() => {
      if (!canCount()) {
        restart()
        return
      }
      setRemaining(Math.max(0, end - Date.now()))
    }, 100)
    window.addEventListener('focus', restart)
    return () => {
      window.clearInterval(tick)
      window.removeEventListener('focus', restart)
    }
  }, [risk, resetKey, total])

  return remaining
}
