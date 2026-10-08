/**
 * data-flow-mermaid.ts — FE-CV-TASK-056-02
 *
 * Prints a backend SequenceModel as Mermaid `sequenceDiagram` text. Every backend string is
 * untrusted (U9): identifiers are generated (p1..pn), labels go through escapeMermaidLabel,
 * and nothing that can open a directive, click handler or HTML is ever emitted.
 */

import type { SequenceModel } from '../../../../../shared/code-intel-architecture-types'

export const SEQUENCE_MAX_MESSAGES = 60
export const SEQUENCE_MAX_PARTICIPANTS = 14
export const SEQUENCE_MAX_CHARS = 40_000
const LABEL_MAX_CHARS = 48

function isStrippedCodePoint(code: number): boolean {
  // C0 controls except tab/LF/CR (handled separately), DEL, NEL, and Unicode line/paragraph separators.
  return (
    (code < 0x20 && code !== 0x09 && code !== 0x0a && code !== 0x0d) ||
    code === 0x7f ||
    code === 0x85 ||
    code === 0x2028 ||
    code === 0x2029
  )
}
const LABEL_ENTITIES: Record<string, string> = {
  '#': '#35;',
  ';': '#59;',
  '%': '#37;',
  '<': '#lt;',
  '>': '#gt;',
  '"': '#quot;',
  '`': '#96;'
}
const LINE_BREAKS = /[\r\n]+/g

/**
 * Single choke point for text placed in a diagram. characters that can start a directive, comment, tag or
 * statement end become Mermaid entities.
 */
export function escapeMermaidLabel(raw: string, maxChars = LABEL_MAX_CHARS): string {
  const spaced = String(raw ?? '')
    .replace(LINE_BREAKS, ' ')
  const flat = Array.from(spaced)
    .filter((ch) => !isStrippedCodePoint(ch.codePointAt(0) ?? 0))
    .join('')
    .trim()
  const cut = Array.from(flat).slice(0, maxChars).join('')
  // One pass: sequential replaces would re-escape the ';' of entities already inserted.
  return cut.replace(/[#;%<>"`]/g, (ch) => LABEL_ENTITIES[ch])
}

export type SequenceBuildOptions = {
  /** Message numbers (`n`) that changed; they get a textual prefix since Mermaid has no tokens. */
  changedMessages?: ReadonlySet<number>
  changedPrefix?: string
  maxMessages?: number
  maxParticipants?: number
  maxChars?: number
}

export type SequenceBuildResult =
  | { ok: true; source: string; renderedMessages: number; totalMessages: number }
  | { ok: false; reason: 'too-many-participants' | 'too-long' | 'empty' }

function arrowFor(sync: boolean, dashedReturn: boolean): string {
  if (sync) {
    return dashedReturn ? '-->>' : '->>'
  }
  return dashedReturn ? '--)' : '-)'
}

export function buildSequenceDiagram(
  seq: SequenceModel,
  options: SequenceBuildOptions = {}
): SequenceBuildResult {
  const {
    changedMessages = new Set<number>(),
    changedPrefix = '[changed] ',
    maxMessages = SEQUENCE_MAX_MESSAGES,
    maxParticipants = SEQUENCE_MAX_PARTICIPANTS,
    maxChars = SEQUENCE_MAX_CHARS
  } = options
  const all = Array.isArray(seq?.messages) ? seq.messages : []
  if (all.length === 0) {
    return { ok: false, reason: 'empty' }
  }
  const messages = all.slice(0, maxMessages)

  const labels = new Map<string, string>()
  for (const p of Array.isArray(seq.participants) ? seq.participants : []) {
    labels.set(p.id, p.label)
  }
  // Participants in order of first appearance; ids are generated, never taken from data.
  const ids = new Map<string, string>()
  const touch = (key: string): string => {
    let id = ids.get(key)
    if (!id) {
      id = `p${ids.size + 1}`
      ids.set(key, id)
    }
    return id
  }
  for (const m of messages) {
    touch(m.from)
    touch(m.to)
  }
  if (ids.size > maxParticipants) {
    return { ok: false, reason: 'too-many-participants' }
  }

  const lines: string[] = ['sequenceDiagram', '  autonumber']
  for (const [key, id] of ids) {
    lines.push(`  participant ${id} as ${escapeMermaidLabel(labels.get(key) ?? key) || id}`)
  }
  for (const m of messages) {
    const from = ids.get(m.from) as string
    const to = ids.get(m.to) as string
    const prefix = changedMessages.has(m.n) ? changedPrefix : ''
    lines.push(`  ${from}${arrowFor(m.sync, m.dashedReturn)}${to}: ${escapeMermaidLabel(prefix + m.label)}`)
    if (m.note) {
      lines.push(`  Note over ${from},${to}: ${escapeMermaidLabel(m.note)}`)
    }
  }
  const source = lines.join('\n')
  if (source.length > maxChars) {
    return { ok: false, reason: 'too-long' }
  }
  return { ok: true, source, renderedMessages: messages.length, totalMessages: all.length }
}
