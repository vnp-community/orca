// Turns the cumulative `agent_output_partial` text of a task run into display lines.
// The agent writes one activity line per event, led by a glyph (agent/src/relay/agent-claude-stream-activity.ts):
//   ·  session info   »  assistant text   ▸  tool call   ↳  tool result   ■  final status

export type AgentActivityKind =
  | 'info'
  | 'text'
  | 'tool'
  | 'result'
  | 'end'
  | 'diagnostic'
  | 'stderr'
  | 'raw'
export type AgentActivityTone = 'ok' | 'error'

export type AgentActivityLine = {
  /** Position in the whole run, so keys stay stable while the window slides. */
  id: number
  kind: AgentActivityKind
  text: string
  tone?: AgentActivityTone
}

export type ParsedAgentOutput = {
  lines: AgentActivityLine[]
  /** Older stdout lines fell out of the window (memory bound). */
  olderHidden: boolean
}

export const MAX_ACTIVITY_LINES = 400
const MAX_STDERR_LINES = 20
const MAX_LINE_CHARS = 400
// Why: a long run re-sends its whole output each time; only the tail is ever shown.
const MAX_STDOUT_CHARS = 256 * 1024

const GLYPH_KIND: Record<string, AgentActivityKind> = {
  '·': 'info',
  '»': 'text',
  '▸': 'tool',
  '↳': 'result',
  '■': 'end'
}

// Defense in depth: the agent already redacts, this catches credential shapes that slipped through.
const SECRET_SHAPES: RegExp[] = [
  /sk-[A-Za-z0-9_-]{20,}/g,
  /gh[pousr]_[A-Za-z0-9]{20,}/g,
  /github_pat_[A-Za-z0-9_]{20,}/g,
  /AKIA[0-9A-Z]{16}/g,
  /Bearer\s+[A-Za-z0-9._~+/=-]{16,}/g,
  /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g,
  /-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*/g
]

export function maskSecretShapes(text: string): string {
  return SECRET_SHAPES.reduce((acc, shape) => acc.replace(shape, '***'), text)
}

function toneOf(kind: AgentActivityKind, text: string): AgentActivityTone | undefined {
  if (kind === 'result' && text.startsWith('error')) {
    return 'error'
  }
  if (kind === 'end') {
    return text.startsWith('Failed') ? 'error' : 'ok'
  }
  return undefined
}

function classify(rawLine: string, id: number): AgentActivityLine {
  const masked = maskSecretShapes(rawLine)
  const kind = GLYPH_KIND[masked.charAt(0)]
  const body = kind ? masked.slice(1).trimStart() : masked
  const text = body.length > MAX_LINE_CHARS ? `${body.slice(0, MAX_LINE_CHARS - 1)}…` : body
  const finalKind = kind ?? 'raw'
  return { id, kind: finalKind, text, tone: toneOf(finalKind, text) }
}

function splitLines(text: string): string[] {
  return text
    .replace(/\r\n/g, '\n')
    .split('\n')
    .filter((l) => l.trim().length > 0)
}

// Why: the Claude Code CLI writes `[claude-code:<code>] {…}` notices on stderr (for example `unrecognized_model` when it does
// not know a model id yet) while the run itself goes on. They are not errors unless the run ends badly or the code is a blocker.
const CLI_DIAGNOSTIC = /^\[claude-code:([a-z0-9_-]+)\]/i
const BLOCKING_DIAGNOSTIC_CODES = new Set([
  'auth_failed',
  'authentication_failed',
  'rate_limit',
  'rate_limited',
  'overloaded'
])

function stderrLine(id: number, masked: string, runFailed: boolean): AgentActivityLine {
  const text = masked.length > MAX_LINE_CHARS ? `${masked.slice(0, MAX_LINE_CHARS - 1)}…` : masked
  const code = CLI_DIAGNOSTIC.exec(masked)?.[1]?.toLowerCase()
  if (!code) {
    return { id, kind: 'stderr', text }
  }
  return {
    id,
    kind: 'diagnostic',
    text,
    ...(runFailed || BLOCKING_DIAGNOSTIC_CODES.has(code) ? { tone: 'error' as const } : {})
  }
}

export function parseAgentOutput(stdout: string, stderr: string): ParsedAgentOutput {
  const cutHead = stdout.length > MAX_STDOUT_CHARS
  const tail = cutHead ? stdout.slice(stdout.length - MAX_STDOUT_CHARS) : stdout
  const all = splitLines(tail)
  // A tail cut mid-line would show half an event; drop that first fragment.
  const complete = cutHead ? all.slice(1) : all
  const droppedLines = Math.max(0, complete.length - MAX_ACTIVITY_LINES)
  const lines = complete.slice(droppedLines).map((l, i) => classify(l, droppedLines + i))
  const errLines = splitLines(stderr).slice(-MAX_STDERR_LINES)
  const base = droppedLines + lines.length
  const runFailed = lines.some((l) => l.kind === 'end' && l.tone === 'error')
  errLines.forEach((l, i) => {
    lines.push(stderrLine(base + i, maskSecretShapes(l), runFailed))
  })
  return { lines, olderHidden: cutHead || droppedLines > 0 }
}
