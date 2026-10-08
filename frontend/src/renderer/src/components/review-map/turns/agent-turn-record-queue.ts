/**
 * agent-turn-record-queue.ts — FE-CV-TASK-089-03
 *
 * In-memory dedupe + retry queue for `quality.turn.record`. Never throws into the
 * caller and never blocks the UI; a lost turn is acceptable (not persisted).
 *
 * @module components/review-map/turns/agent-turn-record-queue
 */

import type { AgentTurnRecordParams } from './agent-turn-record-params'

export type TurnRecordSender = (params: AgentTurnRecordParams) => Promise<void>

// Permanent conditions: retrying cannot help, so drop silently.
const DROP_KINDS: ReadonlySet<string> = new Set([
  'disabled', 'unsupported', 'forbidden', 'validation', 'not-found', 'quality-disabled', 'no-binding'
])
const BACKOFF_MS = [2000, 6000, 18000] as const

type Entry = {
  params: AgentTurnRecordParams
  retries: number
  timer: ReturnType<typeof setTimeout> | null
}

export function createAgentTurnRecordQueue(
  sender: TurnRecordSender,
  classifyError: (err: unknown) => { kind: string }
): {
  enqueue(params: AgentTurnRecordParams): void
  pending(): number
  dispose(): void
} {
  const entries = new Map<string, Entry>()
  // Why: a turn already sent in this session must not be re-sent when the same completion replays.
  const sent = new Set<string>()
  let disposed = false

  async function attempt(entry: Entry): Promise<void> {
    if (disposed) {
      return
    }
    const id = entry.params.clientTurnId
    try {
      await sender(entry.params)
      entries.delete(id)
      sent.add(id)
    } catch (error) {
      let kind = 'unknown'
      try {
        kind = classifyError(error).kind
      } catch {
        // Classifier failure is treated as a transient unknown error.
      }
      if (disposed || DROP_KINDS.has(kind) || entry.retries >= BACKOFF_MS.length) {
        entries.delete(id)
        return
      }
      entry.timer = setTimeout(() => {
        entry.timer = null
        void attempt(entry)
      }, BACKOFF_MS[entry.retries])
      entry.retries++
    }
  }

  return {
    enqueue(params) {
      if (disposed || entries.has(params.clientTurnId) || sent.has(params.clientTurnId)) {
        return
      }
      const entry: Entry = { params, retries: 0, timer: null }
      entries.set(params.clientTurnId, entry)
      void attempt(entry)
    },
    pending: () => entries.size,
    dispose() {
      disposed = true
      for (const entry of entries.values()) {
        if (entry.timer) {
          clearTimeout(entry.timer)
        }
      }
      entries.clear()
    }
  }
}
