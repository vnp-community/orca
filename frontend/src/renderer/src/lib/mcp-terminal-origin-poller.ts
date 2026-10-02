import type { OriginByHandle } from './mcp-terminal-origin'
import { subscribeMcpEvents } from './mcp-event-bus'

export const ACTIVE_POLL_MS = 20_000
export const IDLE_POLL_MS = 60_000
const RECENT_ORIGIN_MS = 5 * 60_000

type PollerDeps = {
  fetchOrigins: () => Promise<OriginByHandle>
  apply: (next: OriginByHandle) => void
  now?: () => number
}

/**
 * Polls terminal.list for MCP origins. Why back off when none were seen: users who never
 * run agents should not pay a 20s poll; focus and session.closed still refresh immediately.
 * Returns a teardown that removes every timer and listener.
 */
export function startMcpTerminalOriginPolling(deps: PollerDeps): () => void {
  const now = deps.now ?? Date.now
  let seq = 0
  let stopped = false
  let timer: ReturnType<typeof setTimeout> | null = null
  let lastSeenAt: number | null = null

  const refresh = (): Promise<void> => {
    const id = ++seq
    return deps.fetchOrigins().then(
      (next) => {
        // Stale responses and post-teardown responses must not write.
        if (stopped || id !== seq) {
          return
        }
        if (Object.keys(next).length > 0) {
          lastSeenAt = now()
        }
        deps.apply(next)
      },
      () => {
        // Keep the previous data on network errors; no toast for a background poll.
      }
    )
  }

  const schedule = (): void => {
    if (stopped) {
      return
    }
    const recent = lastSeenAt !== null && now() - lastSeenAt < RECENT_ORIGIN_MS
    timer = setTimeout(
      () => {
        if (typeof document === 'undefined' || document.visibilityState === 'visible') {
          // Re-arm after the response so the next delay reflects what this poll saw.
          void refresh().then(schedule)
        } else {
          schedule()
        }
      },
      recent ? ACTIVE_POLL_MS : IDLE_POLL_MS
    )
  }

  const onFocus = (): void => void refresh()
  window.addEventListener('focus', onFocus)
  const unsubscribe = subscribeMcpEvents((event) => {
    if (event.type === 'session.closed') {
      void refresh()
    }
  })
  void refresh()
  schedule()

  return () => {
    stopped = true
    if (timer !== null) {
      clearTimeout(timer)
    }
    window.removeEventListener('focus', onFocus)
    unsubscribe()
  }
}
