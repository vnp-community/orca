import { useEffect, useState } from 'react'

/** Remaining ms until `deadline` (client clock), ticking at 1 Hz; no network calls. */
export function useMcpApprovalDeadline(deadline: number | undefined): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    setNow(Date.now())
    if (deadline === undefined) {
      return
    }
    const timer = window.setInterval(() => {
      const t = Date.now()
      setNow(t)
      if (t >= deadline) {
        window.clearInterval(timer)
      }
    }, 1000)
    return () => window.clearInterval(timer)
  }, [deadline])
  return deadline === undefined ? 0 : Math.max(0, deadline - now)
}
