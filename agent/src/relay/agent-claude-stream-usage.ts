// src/relay/agent-claude-stream-usage.ts
// What a stream-json run says about itself, apart from its answer: which tools it called (names and counts, never their
// input or output, which may be document text) and the usage figures of the closing `result` event.

import { isObject, type JsonObject } from './agent-claude-tool-activity-text'

/** Every figure is optional: claude reports only what it knows, and nothing here is guessed. */
export type StreamUsage = {
  turns?: number
  durationMs?: number
  costUsd?: number
  inputTokens?: number
  outputTokens?: number
  cacheReadTokens?: number
  cacheCreationTokens?: number
}

const MAX_TOOL_NAMES = 64
const MAX_TOOL_NAME_CHARS = 100
const OVERFLOW_NAME = '(other)'

/** Counts calls per tool name, with a ceiling on distinct names so a hostile stream cannot grow the result. */
export class ToolCallCounter {
  private readonly counts = new Map<string, number>()

  add(name: string): void {
    const key = name.slice(0, MAX_TOOL_NAME_CHARS)
    const known = this.counts.has(key)
    const slot = known || this.counts.size < MAX_TOOL_NAMES ? key : OVERFLOW_NAME
    this.counts.set(slot, (this.counts.get(slot) ?? 0) + 1)
  }

  snapshot(): Record<string, number> {
    return Object.fromEntries(this.counts)
  }
}

function finite(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) && v >= 0 ? v : undefined
}

/** The figures of a `result` event, or null when it carried none. */
export function readStreamUsage(event: JsonObject): StreamUsage | null {
  const usage = isObject(event.usage) ? event.usage : {}
  const out: StreamUsage = {
    turns: finite(event.num_turns),
    durationMs: finite(event.duration_ms),
    costUsd: finite(event.total_cost_usd),
    inputTokens: finite(usage.input_tokens),
    outputTokens: finite(usage.output_tokens),
    cacheReadTokens: finite(usage.cache_read_input_tokens),
    cacheCreationTokens: finite(usage.cache_creation_input_tokens)
  }
  const present = Object.fromEntries(Object.entries(out).filter(([, v]) => v !== undefined))
  return Object.keys(present).length > 0 ? (present as StreamUsage) : null
}
