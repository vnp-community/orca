// src/relay/agent-claude-final-stdout.ts
// Chooses the stdout returned by agent.execPrompt from a parsed stream-json run.
// Kept apart from the line parser so each file stays under the max-lines limit.

import { BoundedOutputBuffer } from './agent-bounded-output-buffer'
import type { ClaudeStreamOutcome } from './agent-claude-stream-activity'

export type ResolvedStdout = { stdout: string; truncated: boolean }

/**
 * stdout keeps its old meaning: the final answer text. A stream that never looked
 * like stream-json falls back to the raw bytes, exactly as text mode would have returned.
 */
export function resolveFinalStdout(
  outcome: ClaudeStreamOutcome,
  rawBuf: BoundedOutputBuffer,
  budgetBytes: number
): ResolvedStdout {
  if (!outcome.structured) {
    return { stdout: rawBuf.toString(), truncated: rawBuf.truncated }
  }
  const answer = outcome.resultText ?? outcome.assistantText
  // Text mode ended the answer with a newline; callers (result-block parser, interpolation) were built on that.
  const withNewline = answer.length === 0 || answer.endsWith('\n') ? answer : `${answer}\n`
  const bounded = new BoundedOutputBuffer(budgetBytes)
  bounded.append(Buffer.from(withNewline, 'utf8'))
  return { stdout: bounded.toString(), truncated: bounded.truncated }
}
