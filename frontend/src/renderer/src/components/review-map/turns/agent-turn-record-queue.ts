/**
 * agent-turn-record-queue.ts — FE-CV-TASK-089-03
 *
 * Deduplicating retry queue for quality.turn.record submissions.
 * Rules:
 * - Idempotent by clientTurnId (backend guarantees idempotency)
 * - Retry up to 3 times with backoff 2/6/18s for transient errors
 * - Drop immediately for permanent errors (disabled/forbidden/validation/etc.)
 * - Never throws into caller; never blocks UI
 *
 * @module components/review-map/turns/agent-turn-record-queue
 */

import type { CodeIntelErrorKind } from '../../../../../shared/code-intel-parsers'
import type { AgentTurnRecordParams } from './agent-turn-record-params'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type TurnRecordSender = (params: AgentTurnRecordParams) => Promise<void>

type QueueEntry = {
  params: AgentTurnRecordParams
  attempts: number
  timerId: ReturnType<typeof setTimeout> | null
}

// ---------------------------------------------------------------------------
// Error classification
// ---------------------------------------------------------------------------

// Errors that should not be retried (permanent / misconfigured)
const DROP_IMMEDIATELY_KINDS = new Set<CodeIntelErrorKind>([
  'disabled', 'unsupported', 'forbidden', 'validation', 'not_found'
])

// Errors that should be retried with backoff
const RETRYABLE_KINDS = new Set<CodeIntelErrorKind>([
  'offline', 'rate_limited', 'unknown'
])

const BACKOFF_MS = [2000, 6000, 18000] as const
const MAX_ATTEMPTS = 3

function isRetryable(kind: CodeIntelErrorKind): boolean {
  return RETRYABLE_KINDS.has(kind)
}

function isDrop(kind: CodeIntelErrorKind): boolean {
  return DROP_IMMEDIATELY_KINDS.has(kind)
}

// ---------------------------------------------------------------------------
// Queue factory
// ---------------------------------------------------------------------------

/**
 * Create a submission queue.
 *
 * @param sender - async function that sends the turn record; throws CodeIntelRpcError-like on failure
 * @param classifyError - extracts `kind` from a caught error
 */
export function createAgentTurnRecordQueue(
  sender: TurnRecordSender,
  classifyError: (err: unknown) => { kind: CodeIntelErrorKind }
): {
  enqueue(params: AgentTurnRecordParams): void
  dispose(): void
} {
  const queue = new Map<string, QueueEntry>()
  let disposed = false

  function scheduleRetry(entry: QueueEntry): void {
    if (disposed || entry.attempts >= MAX_ATTEMPTS) {
      queue.delete(entry.params.clientTurnId)
      return
    }

    const delay = BACKOFF_MS[entry.attempts - 1] ?? BACKOFF_MS[BACKOFF_MS.length - 1]
    entry.timerId = setTimeout(() => {
      void send(entry)
    }, delay)
  }

  async function send(entry: QueueEntry): Promise<void> {
    if (disposed) return
    try {
      await sender(entry.params)
      // Success — remove from queue
      queue.delete(entry.params.clientTurnId)
    } catch (err) {
      const { kind } = classifyError(err)

      if (isDrop(kind) || disposed) {
        queue.delete(entry.params.clientTurnId)
        return
      }

      if (isRetryable(kind) && entry.attempts < MAX_ATTEMPTS) {
        entry.attempts++
        scheduleRetry(entry)
      } else {
        // Non-retryable or exhausted
        queue.delete(entry.params.clientTurnId)
      }
    }
  }

  return {
    enqueue(params) {
      if (disposed) return

      const existing = queue.get(params.clientTurnId)
      if (existing) {
        // Already queued — deduplication: update params and reset attempts if pending
        existing.params = params
        return
      }

      const entry: QueueEntry = { params, attempts: 0, timerId: null }
      queue.set(params.clientTurnId, entry)

      // First attempt immediately (not via retry delay)
      void send(entry)
    },

    dispose() {
      disposed = true
      for (const entry of queue.values()) {
        if (entry.timerId !== null) clearTimeout(entry.timerId)
      }
      queue.clear()
    }
  }
}
