/**
 * code-intel-event-bus.ts — FE-CV-TASK-050-11
 *
 * Per-worktree event bus for code-intel push events.
 * Listeners receive strongly-typed events. Errors in one listener
 * are caught and do not affect others.
 *
 * @module lib/code-intel-event-bus
 */

// ---------------------------------------------------------------------------
// Event types (subset matching §5 push events)
// ---------------------------------------------------------------------------

export type CodeIntelPushEvent =
  | { event: 'changed'; worktreeId: string; reason?: string; resync?: boolean }
  | { event: 'reindexProgress'; worktreeId: string; percent: number | null; overall: string }
  | {
      event: 'qualityProgress'
      worktreeId: string
      runId: string
      percent: number | null
      stage?: string
      stepIndex?: number
      stepCount?: number
      message?: string
    }
  | {
      event: 'qualityFinished'
      worktreeId: string
      runId: string
      success: boolean
      error: string | null
      status?: 'succeeded' | 'failed' | 'cancelled' | 'interrupted'
      headCommit?: string
    }
  | { event: 'gateChanged'; worktreeId: string; gate?: unknown; headCommit?: string; profile?: string }

export type CodeIntelEventListener = (event: CodeIntelPushEvent) => void

// ---------------------------------------------------------------------------
// Bus singleton
// ---------------------------------------------------------------------------

const listeners = new Set<CodeIntelEventListener>()

/**
 * Subscribe to code-intel push events.
 * Returns an unsubscribe function.
 */
export function subscribeCodeIntelEvents(listener: CodeIntelEventListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/**
 * Publish a code-intel push event to all listeners.
 * Swallows errors from individual listeners.
 */
export function publishCodeIntelEvent(event: CodeIntelPushEvent): void {
  for (const listener of listeners) {
    try {
      listener(event)
    } catch {
      // Swallow per-listener errors (bus isolation)
    }
  }
}

/** Clear all listeners (for tests). */
export function resetCodeIntelEventBus(): void {
  listeners.clear()
}
