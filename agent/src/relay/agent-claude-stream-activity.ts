// src/relay/agent-claude-stream-activity.ts
// Turns `claude --print --output-format stream-json --verbose` into (a) readable
// activity lines for `agent.execPrompt.output` and (b) the same final text the
// old text mode printed. Why: text mode only prints the final answer, so the web
// UI could never show what the agent was reading or running.
//
// Activity line glyphs (the frontend styles lines by their first character):
//   ·  session info   »  assistant text   ▸  tool call   ↳  tool result   ■  final status

import { StringDecoder } from 'node:string_decoder'
import { BoundedOutputBuffer } from './agent-bounded-output-buffer'
import {
  describeToolInput,
  isObject,
  oneLine,
  summarizeToolResult,
  toPosix,
  type JsonObject
} from './agent-claude-tool-activity-text'
import { readStreamUsage, ToolCallCounter, type StreamUsage } from './agent-claude-stream-usage'

export const ACTIVITY_GLYPH = {
  info: '·',
  text: '»',
  tool: '▸',
  result: '↳',
  end: '■'
} as const

const DEFAULT_MAX_LINE_CHARS = 8 * 1024 * 1024
// Why: task-service re-sends the whole accumulated output on every flush, so the
// total must stay small enough for one NATS message.
const DEFAULT_MAX_ACTIVITY_BYTES = 96 * 1024
const TEXT_BLOCK_INPUT_CHARS = 4000
const TEXT_BLOCK_MAX_LINES = 6
const TEXT_LINE_CHARS = 240
const ERROR_DETAIL_CHARS = 160
const PLAIN_LINE_CHARS = 240
const MAX_TRACKED_TOOL_CALLS = 200
const ASSISTANT_FALLBACK_BYTES = 64 * 1024

export type ClaudeStreamActivityOptions = {
  emit: (text: string) => void
  /** Must hide secrets; applied to every piece of text BEFORE it is shortened. */
  redact: (text: string) => string
  /** Working directories (and their real paths) whose prefix is stripped from file paths. */
  cwdPrefixes?: string[]
  maxLineChars?: number
  maxActivityBytes?: number
}

export type ClaudeStreamOutcome = {
  /** At least one event of the stream-json protocol was recognised. */
  structured: boolean
  /** `result` of the final result event; null when none arrived. */
  resultText: string | null
  isError: boolean
  /** Text of the assistant messages, used only when the stream ended without a result event. */
  assistantText: string
  /** Reason to surface on stderr when the run ended without any answer text. */
  errorNote: string | null
  /** Calls per tool name (names only). */
  toolCalls: Record<string, number>
  /** Figures of the closing result event; null when it carried none. */
  usage: StreamUsage | null
}

export class ClaudeStreamActivityParser {
  private readonly decoder = new StringDecoder('utf8')
  private readonly maxLineChars: number
  private readonly maxActivityBytes: number
  private readonly prefixes: string[]
  private pending = ''
  private skippingOversized = false
  private activityBytes = 0
  private limitNoticeSent = false
  private structured = false
  private resultText: string | null = null
  private isError = false
  private errorNote: string | null = null
  private usage: StreamUsage | null = null
  private readonly toolCalls = new ToolCallCounter()
  private readonly assistantBuf = new BoundedOutputBuffer(ASSISTANT_FALLBACK_BYTES)
  private readonly toolNames = new Map<string, string>()
  private batch: string[] = []
  private outcome: ClaudeStreamOutcome | undefined

  constructor(private readonly opts: ClaudeStreamActivityOptions) {
    this.maxLineChars = opts.maxLineChars ?? DEFAULT_MAX_LINE_CHARS
    this.maxActivityBytes = opts.maxActivityBytes ?? DEFAULT_MAX_ACTIVITY_BYTES
    this.prefixes = (opts.cwdPrefixes ?? [])
      .map((p) => toPosix(p).replace(/\/+$/, ''))
      .filter(Boolean)
  }

  push(chunk: Buffer): void {
    if (this.outcome) {
      return
    }
    this.consume(this.decoder.write(chunk))
    this.flushBatch()
  }

  finish(): ClaudeStreamOutcome {
    if (this.outcome) {
      return this.outcome
    }
    this.consume(this.decoder.end())
    if (this.pending.length > 0 && !this.skippingOversized) {
      this.handleLine(this.pending)
    }
    this.pending = ''
    this.flushBatch()
    this.outcome = {
      structured: this.structured,
      resultText: this.resultText,
      isError: this.isError,
      assistantText: this.assistantBuf.toString(),
      errorNote: this.errorNote,
      toolCalls: this.toolCalls.snapshot(),
      usage: this.usage
    }
    return this.outcome
  }

  private consume(text: string): void {
    if (!text) {
      return
    }
    let start = 0
    for (;;) {
      const nl = text.indexOf('\n', start)
      if (nl === -1) {
        break
      }
      const piece = text.slice(start, nl)
      if (this.skippingOversized) {
        // The oversized line ends here; resume normal parsing after it.
        this.skippingOversized = false
        this.pending = ''
      } else {
        this.handleLine(this.pending + piece)
        this.pending = ''
      }
      start = nl + 1
    }
    const rest = text.slice(start)
    if (this.skippingOversized) {
      return
    }
    this.pending += rest
    if (this.pending.length > this.maxLineChars) {
      this.pending = ''
      this.skippingOversized = true
      this.add(`${ACTIVITY_GLYPH.info} skipped an oversized agent event`)
    }
  }

  private handleLine(rawLine: string): void {
    const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine
    if (line.trim() === '') {
      return
    }
    if (line.length > this.maxLineChars) {
      this.add(`${ACTIVITY_GLYPH.info} skipped an oversized agent event`)
      return
    }
    let parsed: unknown
    try {
      parsed = JSON.parse(line)
    } catch {
      parsed = undefined
    }
    if (isObject(parsed) && typeof parsed.type === 'string' && this.handleEvent(parsed)) {
      return
    }
    // Not a stream-json event (older claude, wrapper noise, stray text): show it as plain text.
    this.add(`${ACTIVITY_GLYPH.text} ${this.clean(line, PLAIN_LINE_CHARS)}`)
  }

  /** Returns false when the event type is not part of the protocol, so the caller shows it as text. */
  private handleEvent(event: JsonObject): boolean {
    switch (event.type) {
      case 'system':
        this.structured = true
        if (event.subtype === 'init' && typeof event.model === 'string') {
          this.add(`${ACTIVITY_GLYPH.info} Session started (${this.clean(event.model, 60)})`)
        }
        return true
      case 'assistant':
        this.structured = true
        this.handleAssistant(event)
        return true
      case 'user':
        this.structured = true
        this.handleUser(event)
        return true
      case 'result':
        this.structured = true
        this.handleResult(event)
        return true
      case 'rate_limit_event':
      case 'stream_event':
        this.structured = true
        return true
      default:
        // Unknown event types of a stream we already recognise are protocol additions, not text to show.
        if (this.structured || typeof event.session_id === 'string') {
          this.structured = true
          return true
        }
        return false
    }
  }

  private handleAssistant(event: JsonObject): void {
    const message = event.message
    const content = isObject(message) && Array.isArray(message.content) ? message.content : []
    for (const block of content) {
      if (!isObject(block)) {
        continue
      }
      if (block.type === 'text' && typeof block.text === 'string') {
        this.assistantBuf.append(Buffer.from(`${block.text}\n`, 'utf8'))
        this.addTextBlock(block.text)
      } else if (
        (block.type === 'tool_use' || block.type === 'server_tool_use') &&
        typeof block.name === 'string'
      ) {
        this.addToolUse(block)
      }
      // thinking blocks carry opaque signatures and are not shown.
    }
  }

  private addTextBlock(text: string): void {
    const redacted = this.opts.redact(text.slice(0, TEXT_BLOCK_INPUT_CHARS))
    const lines = redacted
      .split('\n')
      .map((l) => oneLine(l, TEXT_LINE_CHARS))
      .filter((l) => l.length > 0)
    for (const line of lines.slice(0, TEXT_BLOCK_MAX_LINES)) {
      this.add(`${ACTIVITY_GLYPH.text} ${line}`)
    }
    if (lines.length > TEXT_BLOCK_MAX_LINES || text.length > TEXT_BLOCK_INPUT_CHARS) {
      this.add(`${ACTIVITY_GLYPH.text} …`)
    }
  }

  private addToolUse(block: JsonObject): void {
    const name = block.name as string
    this.toolCalls.add(name)
    if (typeof block.id === 'string') {
      if (this.toolNames.size >= MAX_TRACKED_TOOL_CALLS) {
        const oldest = this.toolNames.keys().next().value
        if (oldest !== undefined) {
          this.toolNames.delete(oldest)
        }
      }
      this.toolNames.set(block.id, name)
    }
    const { detail, sep } = describeToolInput(
      name,
      isObject(block.input) ? block.input : {},
      this.prefixes,
      this.cleaner
    )
    const head = `${ACTIVITY_GLYPH.tool} ${this.clean(name, 60)}`
    this.add(detail ? `${head}${sep}${detail}` : head)
  }

  private handleUser(event: JsonObject): void {
    const message = event.message
    const content = isObject(message) && Array.isArray(message.content) ? message.content : []
    for (const block of content) {
      if (isObject(block) && block.type === 'tool_result') {
        this.addToolResult(block)
      }
    }
  }

  private addToolResult(block: JsonObject): void {
    const id = typeof block.tool_use_id === 'string' ? block.tool_use_id : ''
    this.add(
      `${ACTIVITY_GLYPH.result} ${summarizeToolResult(this.toolNames.get(id) ?? '', block, this.cleaner)}`
    )
  }

  private handleResult(event: JsonObject): void {
    this.isError = event.is_error === true
    this.usage = readStreamUsage(event)
    const subtype = typeof event.subtype === 'string' ? event.subtype : ''
    this.resultText = typeof event.result === 'string' ? event.result : null
    if (this.resultText === null && (this.isError || (subtype && subtype !== 'success'))) {
      this.errorNote = `claude ended without an answer (${subtype || 'error'})`
    }
    const turns =
      typeof event.num_turns === 'number'
        ? `${event.num_turns} turn${event.num_turns === 1 ? '' : 's'}`
        : ''
    const secs =
      typeof event.duration_ms === 'number' ? `${(event.duration_ms / 1000).toFixed(1)}s` : ''
    const stats = [turns, secs].filter(Boolean).join(' · ')
    if (this.isError) {
      const why = this.resultText
        ? this.clean(this.resultText, ERROR_DETAIL_CHARS)
        : subtype || 'error'
      this.add(`${ACTIVITY_GLYPH.end} Failed: ${why}`, true)
    } else {
      this.add(`${ACTIVITY_GLYPH.end} Done${stats ? ` · ${stats}` : ''}`, true)
    }
  }

  private readonly cleaner = (text: string, max: number): string =>
    oneLine(this.opts.redact(text), max)

  private clean(text: string, max: number): string {
    return this.cleaner(text, max)
  }

  // `force` lets the closing status line through even after the size limit was hit.
  private add(line: string, force = false): void {
    const bytes = Buffer.byteLength(line, 'utf8') + 1
    if (!force && this.activityBytes + bytes > this.maxActivityBytes) {
      if (!this.limitNoticeSent) {
        this.limitNoticeSent = true
        this.batch.push(`${ACTIVITY_GLYPH.info} activity limit reached; later steps are not shown`)
      }
      return
    }
    this.activityBytes += bytes
    this.batch.push(line)
  }

  private flushBatch(): void {
    if (this.batch.length === 0) {
      return
    }
    const text = `${this.batch.join('\n')}\n`
    this.batch = []
    this.opts.emit(text)
  }
}
