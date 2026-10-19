import { maskSecretShapes } from '../../task/agent-activity/agent-activity-lines'

// The gateway already hides secrets and keeps the last 8 KiB; this is the same defense-in-depth
// the live activity panel applies, plus a bound on what one <pre> has to lay out.
export const AGENT_OUTPUT_MAX_LINES = 200
export const AGENT_OUTPUT_MAX_LINE_CHARS = 400

export type PreparedAgentOutput = { text: string; cut: boolean }

export function prepareAgentOutputTail(raw: string): PreparedAgentOutput {
  const lines = maskSecretShapes(raw.replace(/\r\n/g, '\n')).split('\n')
  while (lines.length > 0 && lines.at(-1)?.trim() === '') {
    lines.pop()
  }
  let cut = lines.length > AGENT_OUTPUT_MAX_LINES
  const kept = lines.slice(-AGENT_OUTPUT_MAX_LINES).map((line) => {
    if (line.length <= AGENT_OUTPUT_MAX_LINE_CHARS) {
      return line
    }
    cut = true
    return `${line.slice(0, AGENT_OUTPUT_MAX_LINE_CHARS - 1)}…`
  })
  return { text: kept.join('\n'), cut }
}
