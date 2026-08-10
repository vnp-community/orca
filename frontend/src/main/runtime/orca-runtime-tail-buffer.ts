/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
pre-existing tail region (~1,945 lines moved verbatim, already covered by
orca-runtime.ts's own grandfathered max-lines disable before this move).
Registered in config/max-lines-baseline.txt per AGENTS.md — NEEDS PR REVIEW.
Still requires the "Giai doan 2/3" domain split from SOLUTION-FE-BIGFILE-002
to actually shrink below budget; tracked there, not silently deferred. */
/* eslint-disable no-control-regex -- Why: terminal normalization must strip ANSI and OSC control sequences from PTY output before returning bounded text to agents (carried over from orca-runtime.ts, same justification). */
// frontend/src/main/runtime/orca-runtime-tail-buffer.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-008): pure terminal-tail/output-
// scanning + branch-name-normalization helpers extracted from orca-runtime.ts.
// No OrcaRuntimeService state — safe standalone module. Re-exported from
// orca-runtime.ts so existing external importers are unaffected; a few
// symbols (normalizeLocalBranchName, MAX_TAIL_CHARS) are imported back into
// orca-runtime.ts because OrcaRuntimeService itself also uses them.
import type { AgentStatus } from '../../shared/agent-detection'
import {
  detectAgentStatusFromTitle,
  isClaudeManagementTitle,
  isShellProcess,
} from '../../shared/agent-detection'
import type { AgentStatusEntry } from '../../shared/agent-status-types'
import {
  isTerminalInputTooLargeWithYield,
  TERMINAL_INPUT_TOO_LARGE_ERROR,
} from '../../shared/terminal-input'
import {
  splitWorktreeId,
} from '../../shared/worktree-id'
import { parsePtySessionId } from '../../shared/pty-session-id-format'
import {
  isPathInsideOrEqual,
  normalizeRuntimePathForComparison
} from '../../shared/cross-platform-path'
import type {
  RuntimeTerminalRead,
  RuntimeTerminalAgentStatus,
  RuntimeTerminalState,
  RuntimeTerminalWait,
  RuntimeTerminalWaitBlockedReason,
  RuntimeTerminalWaitCondition,
  RuntimeWorktreePsSummary,
  RuntimeWorktreeStatus,
} from '../../shared/runtime-types'

const RECENT_PTY_OUTPUT_LIMIT = 64 * 1024
const RECENT_PTY_PATH_CANDIDATE_LIMIT = 1024
const RECENT_PTY_PATH_CANDIDATE_MAX_BYTES = 4 * 1024
const RECENT_PTY_PATH_CANDIDATE_TOTAL_BYTES = 64 * 1024

import type { RuntimeLeafRecord, RuntimePtyWorktreeRecord, ResolvedWorktree } from './orca-runtime'
// Why: these 3 types stay defined in orca-runtime.ts (used pervasively by
// OrcaRuntimeService itself, 25-30 call sites each) — imported here type-only
// (erased at compile time, so no runtime circular-import risk) purely for the
// signatures of a couple of tail functions below that reference them.

function normalizeLocalBranchName(branchName: string | undefined): string {
  return branchName?.replace(/^refs\/heads\//, '') ?? ''
}

const MAX_TAIL_LINES = 2000
const MAX_TAIL_CHARS = 256 * 1024
const MAX_TAIL_PARTIAL_CHARS = 4000
const MAX_TAIL_PENDING_ANSI_CHARS = 4096
const DEFAULT_TERMINAL_READ_LIMIT = 120
const MAX_TERMINAL_READ_LIMIT = 2000
const MAX_TERMINAL_PREVIEW_CHARS = 32 * 1024
const MAX_PREVIEW_LINES = 6
const MAX_PREVIEW_CHARS = 300
const WORKTREE_STATUS_PRIORITY: Record<RuntimeWorktreeStatus, number> = {
  inactive: 0,
  active: 1,
  done: 2,
  working: 3,
  permission: 4
}

export function appendRecentPtyOutput(previous: string | undefined, data: string): string {
  if (data.length >= RECENT_PTY_OUTPUT_LIMIT) {
    return data.slice(-RECENT_PTY_OUTPUT_LIMIT)
  }
  return `${previous ?? ''}${data}`.slice(-RECENT_PTY_OUTPUT_LIMIT)
}

export function appendRecentPtyPathCandidates(
  previous: string[] | undefined,
  data: string
): string[] {
  const extractedCandidates = extractTerminalOutputPathCandidates(data)
  if (extractedCandidates.length === 0) {
    // Why: pathless output is the common hot path. Reuse immutable history so
    // each PTY chunk does not clone and byte-scan up to 1,024 old candidates.
    return previous ?? []
  }
  const next = previous ? previous.slice() : []
  for (const candidate of extractedCandidates) {
    if (Buffer.byteLength(candidate, 'utf8') > RECENT_PTY_PATH_CANDIDATE_MAX_BYTES) {
      continue
    }
    next.push(candidate)
  }
  return pruneRecentPtyPathCandidates(next)
}

export function recentTerminalPathCandidatesIncludePath(
  recentCandidates: readonly string[],
  pathText: string,
  absolutePath: string
): boolean {
  const candidates = new Set(
    [
      pathText,
      absolutePath,
      ...wslTerminalOutputAliases(pathText),
      ...wslTerminalOutputAliases(absolutePath)
    ]
      .map((candidate) => candidate.trim())
      .filter((candidate) => candidate.length > 0)
  )
  for (const recent of recentCandidates) {
    if (candidates.has(recent)) {
      return true
    }
  }
  return false
}

function pruneRecentPtyPathCandidates(candidates: string[]): string[] {
  const countBounded =
    candidates.length > RECENT_PTY_PATH_CANDIDATE_LIMIT
      ? candidates.slice(-RECENT_PTY_PATH_CANDIDATE_LIMIT)
      : candidates
  let totalBytes = 0
  let startIndex = countBounded.length
  for (let index = countBounded.length - 1; index >= 0; index -= 1) {
    const nextTotal = totalBytes + Buffer.byteLength(countBounded[index]!, 'utf8')
    if (nextTotal > RECENT_PTY_PATH_CANDIDATE_TOTAL_BYTES) {
      break
    }
    totalBytes = nextTotal
    startIndex = index
  }
  return startIndex === 0 ? countBounded : countBounded.slice(startIndex)
}

export function recentTerminalOutputIncludesPath(
  recentOutput: string,
  pathText: string,
  absolutePath: string
): boolean {
  const candidates = new Set(
    [pathText, absolutePath]
      .map((candidate) => candidate.trim())
      .filter((candidate) => candidate.length > 0)
  )
  if (candidates.size === 0) {
    return false
  }
  for (const candidate of candidates) {
    if (outputContainsPathCandidate(recentOutput, candidate)) {
      return true
    }
  }
  const decodedOutput = decodeTerminalOutputPercentEscapes(recentOutput)
  if (decodedOutput !== recentOutput) {
    for (const candidate of candidates) {
      if (outputContainsPathCandidate(decodedOutput, candidate)) {
        return true
      }
    }
  }
  return false
}

function outputContainsPathCandidate(output: string, candidate: string): boolean {
  let start = output.indexOf(candidate)
  while (start !== -1) {
    const end = start + candidate.length
    if (isPathCandidateStartBoundary(output, start) && isPathCandidateEndBoundary(output, end)) {
      return true
    }
    start = output.indexOf(candidate, start + 1)
  }
  return false
}

function isPathCandidateStartBoundary(output: string, start: number): boolean {
  if (start === 0) {
    return true
  }
  if (output.slice(0, start).endsWith('file://')) {
    return true
  }
  if (
    /^[A-Za-z]:[\\/]/.test(output.slice(start)) &&
    /file:\/\/(?:localhost|127\.0\.0\.1|\[?::1\]?)?\/$/i.test(output.slice(0, start))
  ) {
    return true
  }
  if (/file:\/\/(?:localhost|127\.0\.0\.1|\[?::1\]?)$/i.test(output.slice(0, start))) {
    return true
  }
  return !isPathCandidateContinuationChar(output[start - 1]!)
}

function isPathCandidateEndBoundary(output: string, end: number): boolean {
  const next = output[end]
  if (!next) {
    return true
  }
  if (next === ':' && /^\d+(?::\d+)?(?:\D|$)/.test(output.slice(end + 1))) {
    return true
  }
  return !isPathCandidateContinuationChar(next)
}

function isPathCandidateContinuationChar(char: string): boolean {
  return /[A-Za-z0-9._~/%+@\\()[\]-]/.test(char)
}

function decodeTerminalOutputPercentEscapes(value: string): string {
  return value.replace(/(?:%[0-9a-f]{2})+/gi, (match) => {
    try {
      return decodeURIComponent(match)
    } catch {
      return match
    }
  })
}

// Why: extraction runs on the PTY hot path for every chunk, and the extension
// regex backtracks quadratically on pathological separator runs. No candidate
// can cross a newline (the regex classes exclude \r\n), so scan per line and
// skip lines already too long to yield a storable candidate —
// appendRecentPtyPathCandidates drops oversized candidates anyway, and the raw
// recent-output buffer still covers provenance inside oversized lines.
function extractTerminalOutputPathCandidates(data: string): string[] {
  const candidates: string[] = []
  const add = (value: string): void => {
    const candidate = trimTerminalOutputPathCandidate(value)
    if (candidate.length > 0) {
      candidates.push(candidate)
      const drivePath = normalizeTerminalOutputFileUriDrivePath(candidate)
      if (drivePath) {
        candidates.push(drivePath)
      }
    }
  }
  for (const line of data.split(/[\r\n]+/)) {
    if (line.length === 0 || line.length > RECENT_PTY_PATH_CANDIDATE_MAX_BYTES) {
      continue
    }
    collectTerminalOutputLinePathCandidates(line, add)
  }
  return candidates
}

function collectTerminalOutputLinePathCandidates(line: string, add: (value: string) => void): void {
  for (const match of line.matchAll(/file:\/\/([^/\s]*)(\/[^\s\x1b"'<>)]*)/gi)) {
    const authority = match[1] ?? ''
    const uriPath = match[2]
    if (uriPath) {
      const decoded = decodeTerminalOutputPercentEscapes(uriPath)
      add(isTerminalOutputLoopbackAuthority(authority) ? decoded : `//${authority}${decoded}`)
    }
  }
  for (const match of line.matchAll(
    /(?:\/(?:tmp|private\/tmp)\/|[A-Za-z]:[\\/])[^\r\n\x1b"'<>]+/g
  )) {
    if (isInsideNonLocalFileUri(line, match.index)) {
      continue
    }
    add(match[0])
  }
  for (const match of line.matchAll(
    /\/[^\r\n\x1b"'<>]*\.[A-Za-z0-9_+-]+(?:[#:\s][^\r\n\x1b"'<>]*)?/g
  )) {
    if (isInsideNonLocalFileUri(line, match.index)) {
      continue
    }
    add(match[0])
  }
}

function normalizeTerminalOutputFileUriDrivePath(candidate: string): string | null {
  return /^\/[A-Za-z]:[\\/]/.test(candidate) ? candidate.slice(1) : null
}

function trimTerminalOutputPathCandidate(value: string): string {
  let candidate = value.trim().replace(/[),;.]+$/g, '')
  if (Buffer.byteLength(candidate, 'utf8') > RECENT_PTY_PATH_CANDIDATE_MAX_BYTES) {
    return ''
  }
  let selected: string | null = null
  for (const match of candidate.matchAll(
    /.+?\.[A-Za-z0-9_+-]+(?:#L\d+(?:C\d+)?|(?::\d+)?(?::\d+)?)?(?=\s+|$)/gi
  )) {
    const end = match.index + match[0].length
    const text = candidate.slice(0, end)
    if (countTerminalOutputPathStarts(text) > 1) {
      continue
    }
    // Same rule as the tap parsers: a line-end extension token only extends
    // the candidate when the added segment is path-like, so trailing prose
    // ending in a filename is not swallowed into the candidate.
    if (
      end < candidate.length ||
      selected === null ||
      /[\\/]/.test(candidate.slice(selected.length, end))
    ) {
      selected = text
    }
  }
  return trimTerminalOutputPathLocator(selected ?? candidate)
}

function isTerminalOutputLoopbackAuthority(authority: string): boolean {
  const normalized = authority.toLowerCase()
  return (
    normalized === '' ||
    normalized === 'localhost' ||
    normalized === '127.0.0.1' ||
    normalized === '::1' ||
    normalized === '[::1]'
  )
}

function isInsideNonLocalFileUri(output: string, pathStart: number): boolean {
  const prefix = output.slice(0, pathStart)
  const match = /file:\/\/([^/\s]*)$/i.exec(prefix)
  return !!match && !isTerminalOutputLoopbackAuthority(match[1] ?? '')
}

function countTerminalOutputPathStarts(value: string): number {
  let count = 0
  for (const match of value.matchAll(/(?:^|\s)(?:~[\\/]|[\\/]|\.{1,2}[\\/]|[A-Za-z]:[\\/])/g)) {
    void match
    count += 1
  }
  return count
}

function trimTerminalOutputPathLocator(value: string): string {
  return value.replace(/#L\d+(?:C\d+)?$/i, '').replace(/:\d+(?::\d+)?$/, '')
}

function wslTerminalOutputAliases(value: string): string[] {
  const match = /^\\\\wsl(?:\.localhost|\$)\\[^\\]+(\\.*)$/i.exec(value)
  if (!match) {
    return []
  }
  const linuxPath = match[1]!.replace(/\\/g, '/')
  return linuxPath.startsWith('/') ? [linuxPath] : [`/${linuxPath}`]
}

export function buildPreview(lines: string[], partialLine: string): string {
  const previewLines: string[] = []
  const collectVisibleLine = (line: string): void => {
    const trimmed = line.trim()
    if (trimmed.length > 0) {
      previewLines.push(trimmed)
    }
  }

  if (partialLine.length > 0) {
    collectVisibleLine(partialLine)
  }
  for (
    let index = lines.length - 1;
    index >= 0 && previewLines.length < MAX_PREVIEW_LINES;
    index--
  ) {
    collectVisibleLine(lines[index])
  }
  previewLines.reverse()

  const preview = previewLines.join('\n')
  return preview.length > MAX_PREVIEW_CHARS
    ? preview.slice(preview.length - MAX_PREVIEW_CHARS)
    : preview
}

function buildTerminalWaitText(lines: string[], partialLine: string, preview: string): string {
  const waitText = buildTailLines(lines, partialLine)
    .map((line) => line.trim())
    .filter(Boolean)
    .join('\n')
  // Why: the user-facing preview is intentionally short, but wait readiness
  // needs the retained terminal tail so known ready headers are not truncated away.
  return waitText.length > 0 ? waitText : preview
}

export type TerminalTailWaitState = {
  waitText: string
  signal: { reason: RuntimeTerminalWaitBlockedReason; index: number } | null
  // Why: the retained tail is authoritative; `preview` is only a fallback for an
  // empty tail. A preview-derived state depends on a value that is recomputed
  // after each append, so it must not be reused as the next chunk's previous
  // state — reuse is gated on fromTail.
  fromTail: boolean
}

// Why: onPtyData runs per raw PTY chunk (hundreds/sec under load). Ordinary
// tails take one no-join sentinel pass; only candidate-bearing tails
// build, lowercase, and parse the full 256 KiB text. The cached post-append
// state also avoids repeating that work for the next chunk's previous state.
export function computeTerminalTailWaitState(
  lines: string[],
  partialLine: string,
  preview: string
): TerminalTailWaitState {
  const tailShape = inspectTerminalWaitTail(lines, partialLine)
  if (!tailShape.fromTail) {
    return {
      waitText: preview,
      signal: findActionableTerminalWaitBlockedSignal(preview.toLowerCase()),
      fromTail: false
    }
  }
  if (!tailShape.mayContainBlockedSignal) {
    // Why: tailGainedNewerBlockedReason reads waitText only when signal exists;
    // avoid retaining a rebuilt 256 KiB string for the overwhelmingly common case.
    return { waitText: '', signal: null, fromTail: true }
  }
  const tailText = buildTailLines(lines, partialLine)
    .map((line) => line.trim())
    .filter(Boolean)
    .join('\n')
  const fromTail = tailText.length > 0
  const waitText = fromTail ? tailText : preview
  return {
    waitText,
    signal: findActionableTerminalWaitBlockedSignal(waitText.toLowerCase()),
    fromTail
  }
}

function inspectTerminalWaitTail(
  lines: string[],
  partialLine: string
): { fromTail: boolean; mayContainBlockedSignal: boolean } {
  let fromTail = false
  let mayContainBlockedSignal = false
  for (const line of lines) {
    if (!fromTail && line.trim().length > 0) {
      fromTail = true
    }
    if (!mayContainBlockedSignal && TERMINAL_WAIT_BLOCKED_SENTINEL_RE.test(line)) {
      mayContainBlockedSignal = true
    }
  }
  if (!fromTail && partialLine.trim().length > 0) {
    fromTail = true
  }
  if (!mayContainBlockedSignal && TERMINAL_WAIT_BLOCKED_SENTINEL_RE.test(partialLine)) {
    mayContainBlockedSignal = true
  }
  return { fromTail, mayContainBlockedSignal }
}

// Why: decides whether the appended chunk introduced a newer actionable blocked
// prompt, consuming precomputed wait states so the full-tail scans are not
// repeated per chunk (replaces the former inline double full-tail scan).
export function tailGainedNewerBlockedReason(
  previous: TerminalTailWaitState,
  next: TerminalTailWaitState,
  appendedText: string
): boolean {
  if (next.signal === null) {
    return false
  }
  // Why: permission prompts can arrive split across PTY chunks. Stamp when the
  // accumulated tail first becomes blocked, or when a later prompt appears after
  // stale blocked text already in the tail.
  if (previous.signal === null) {
    return true
  }
  const appendCandidateSignal = findActionableTerminalWaitBlockedSignal(
    `${previous.waitText}${appendedText}`.toLowerCase()
  )
  return appendCandidateSignal !== null && appendCandidateSignal.index > previous.signal.index
}

export function appendNormalizedToTailBuffer(
  previousLines: string[],
  previousPartialLine: string,
  normalizedChunk: string,
  previousRedrawCursor: RetainedTailRedrawCursor | null = null
): {
  lines: string[]
  partialLine: string
  redrawCursor: RetainedTailRedrawCursor | null
  truncated: boolean
  newCompleteLines: number
} {
  if (normalizedChunk.length === 0) {
    return {
      lines: previousLines,
      partialLine: previousPartialLine,
      redrawCursor: previousRedrawCursor,
      truncated: false,
      newCompleteLines: 0
    }
  }

  // Why: fullscreen TUIs often emit long, newline-free redraw streams. Keep the
  // larger line transcript for pagination, but keep partial-line work bounded.
  const previousPartialWasCapped = previousPartialLine.length > MAX_TAIL_PARTIAL_CHARS
  const boundedPreviousPartialLine = previousPartialLine.slice(-MAX_TAIL_PARTIAL_CHARS)
  const combinedChunk = `${boundedPreviousPartialLine}${normalizedChunk}`
  if (previousRedrawCursor || containsTerminalVerticalLineControl(combinedChunk)) {
    return appendNormalizedToMultilineTailBuffer(
      previousLines,
      boundedPreviousPartialLine,
      normalizedChunk,
      previousPartialWasCapped,
      previousRedrawCursor
    )
  }

  // Why: status UIs redraw a single line with CR/backspace/ANSI erase controls.
  // Terminal previews are text, not a full screen model, so retain the latest
  // visible redraw segment instead of appending every spinner frame.
  const segments = splitRetainedTerminalTailSegments(combinedChunk)
  const pieces = processTerminalTailCompleteSegments(segments.completeSegments)
  const partialResult = applyTerminalLineControls(segments.partialSegment)
  const nextPartialLine = trimTerminalLineRight(partialResult.text)
  const retainedPartialLine = nextPartialLine.slice(-MAX_TAIL_PARTIAL_CHARS)
  const newCompleteLines = segments.completeLineCount
  const omittedNewCompleteLines = newCompleteLines - pieces.length
  let nextLines =
    newCompleteLines > 0
      ? [
          ...(omittedNewCompleteLines > 0 ? [] : previousLines),
          ...pieces.map((line) => line.replace(/[ \t]+$/g, ''))
        ]
      : previousLines
  let truncated =
    previousPartialWasCapped ||
    omittedNewCompleteLines > 0 ||
    nextPartialLine.length > MAX_TAIL_PARTIAL_CHARS

  if (nextLines.length > MAX_TAIL_LINES) {
    nextLines = nextLines.slice(nextLines.length - MAX_TAIL_LINES)
    truncated = true
  }

  if (newCompleteLines > 0 || retainedPartialLine.length > previousPartialLine.length) {
    if (nextLines === previousLines) {
      nextLines = [...previousLines]
    }
    let totalChars =
      nextLines.reduce((sum, line) => sum + line.length, 0) + retainedPartialLine.length
    let trimStartIndex = 0
    while (trimStartIndex < nextLines.length && totalChars > MAX_TAIL_CHARS) {
      totalChars -= nextLines[trimStartIndex].length
      trimStartIndex += 1
    }
    if (trimStartIndex > 0) {
      nextLines = nextLines.slice(trimStartIndex)
      truncated = true
    }
  }

  const redrawCursor =
    !partialResult.hadControl || partialResult.cursorColumn === nextPartialLine.length
      ? null
      : {
          rowFromEnd: 0,
          column: partialResult.cursorColumn
        }

  return {
    lines: nextLines,
    partialLine: retainedPartialLine,
    redrawCursor,
    truncated,
    newCompleteLines
  }
}

function trimTerminalLineRight(line: string): string {
  let end = line.length
  while (end > 0) {
    const code = line.charCodeAt(end - 1)
    if (code !== 0x20 && code !== 0x09) {
      break
    }
    end -= 1
  }
  return end === line.length ? line : line.slice(0, end)
}

// Why a window: the unwindowed implementation below materializes a row object
// per retained tail line and finalize re-allocates + regex-trims every row —
// O(tail) per chunk (~0.9ms at the 2,000-line cap), measured at ~93% of the
// main-process event loop under an agent-TUI flood (findings log 2026-07-03).
// A redraw can only touch rows the cursor can reach, so run the algorithm on
// a suffix window sized by the chunk's maximum upward cursor excursion and
// share the untouched prefix by reference. Equality with the unwindowed
// implementation is fuzz-verified in
// retained-tail-redraw-window.equivalence.test.ts.
const REDRAW_WINDOW_SAFETY_ROWS = 8

function maxUpwardCursorReach(
  normalizedChunk: string,
  previousRedrawCursor: RetainedTailRedrawCursor | null
): number {
  let reach = previousRedrawCursor ? previousRedrawCursor.rowFromEnd : 0
  const cursorUpPattern = /\x1b\[(\d*)(?:;[\d;]*)?A/g
  let match: RegExpExecArray | null
  while ((match = cursorUpPattern.exec(normalizedChunk)) !== null) {
    reach += match[1] ? Number.parseInt(match[1], 10) : 1
  }
  return reach
}

function appendNormalizedToMultilineTailBuffer(
  previousLines: string[],
  boundedPreviousPartialLine: string,
  normalizedChunk: string,
  previousPartialWasCapped: boolean,
  previousRedrawCursor: RetainedTailRedrawCursor | null
): {
  lines: string[]
  partialLine: string
  redrawCursor: RetainedTailRedrawCursor | null
  truncated: boolean
  newCompleteLines: number
} {
  const windowRows =
    maxUpwardCursorReach(normalizedChunk, previousRedrawCursor) + REDRAW_WINDOW_SAFETY_ROWS
  if (windowRows >= previousLines.length) {
    return appendNormalizedToMultilineTailBufferUnwindowed(
      previousLines,
      boundedPreviousPartialLine,
      normalizedChunk,
      previousPartialWasCapped,
      previousRedrawCursor
    )
  }
  const prefixLength = previousLines.length - windowRows
  const suffix = previousLines.slice(prefixLength)
  const windowed = appendNormalizedToMultilineTailBufferUnwindowed(
    suffix,
    boundedPreviousPartialLine,
    normalizedChunk,
    previousPartialWasCapped,
    previousRedrawCursor
  )
  let lines = previousLines.slice(0, prefixLength)
  // Why: the unwindowed finalize trims trailing spaces/tabs on every row; the
  // shared prefix must match without paying a regex per untouched row.
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]!
    const lastChar = line.charCodeAt(line.length - 1)
    if (lastChar === 32 || lastChar === 9) {
      lines[index] = line.replace(/[ \t]+$/g, '')
    }
  }
  for (const line of windowed.lines) {
    lines.push(line)
  }
  let truncated = windowed.truncated
  if (lines.length > MAX_TAIL_LINES) {
    lines = lines.slice(lines.length - MAX_TAIL_LINES)
    truncated = true
  }
  let totalChars = windowed.partialLine.length
  for (const line of lines) {
    totalChars += line.length
  }
  let dropCount = 0
  while (dropCount < lines.length && totalChars > MAX_TAIL_CHARS) {
    totalChars -= lines[dropCount]!.length
    dropCount += 1
  }
  if (dropCount > 0) {
    lines = lines.slice(dropCount)
    truncated = true
  }
  return {
    lines,
    partialLine: windowed.partialLine,
    redrawCursor: windowed.redrawCursor,
    truncated,
    newCompleteLines: windowed.newCompleteLines
  }
}

export function appendNormalizedToMultilineTailBufferUnwindowed(
  previousLines: string[],
  boundedPreviousPartialLine: string,
  normalizedChunk: string,
  previousPartialWasCapped: boolean,
  previousRedrawCursor: RetainedTailRedrawCursor | null
): {
  lines: string[]
  partialLine: string
  redrawCursor: RetainedTailRedrawCursor | null
  truncated: boolean
  newCompleteLines: number
} {
  const rows: RetainedTerminalRow[] = [
    ...previousLines.map((line) => ({ text: line, completed: true })),
    { text: boundedPreviousPartialLine, completed: false }
  ]
  let cursorRow = previousRedrawCursor
    ? Math.max(0, rows.length - 1 - previousRedrawCursor.rowFromEnd)
    : rows.length - 1
  let cursorColumn = previousRedrawCursor?.column ?? boundedPreviousPartialLine.length
  let newCompleteLines = 0
  let truncated = previousPartialWasCapped

  const ensureCursorRow = (): void => {
    while (cursorRow >= rows.length) {
      rows.push({ text: '', completed: false })
    }
  }
  const trimRows = (): void => {
    const maxRows = MAX_TAIL_LINES + 1
    if (rows.length <= maxRows) {
      return
    }
    const removeCount = rows.length - maxRows
    rows.splice(0, removeCount)
    cursorRow = Math.max(0, cursorRow - removeCount)
    truncated = true
  }
  const moveCursorToColumn = (nextColumn: number): void => {
    cursorColumn = clampTerminalPreviewCursor(nextColumn)
  }
  const markCursorRowRewritten = (): void => {
    ensureCursorRow()
    rows[cursorRow]!.completed = false
  }
  const writeChar = (char: string): void => {
    ensureCursorRow()
    markCursorRowRewritten()
    const row = rows[cursorRow]!
    if (cursorColumn > row.text.length) {
      row.text = `${row.text}${' '.repeat(cursorColumn - row.text.length)}`
    }
    row.text =
      cursorColumn >= row.text.length
        ? `${row.text}${char}`
        : `${row.text.slice(0, cursorColumn)}${char}${row.text.slice(cursorColumn + 1)}`
    cursorColumn += 1
  }
  const eraseLine = (mode: number): void => {
    ensureCursorRow()
    markCursorRowRewritten()
    const row = rows[cursorRow]!
    if (mode === 0) {
      row.text = row.text.slice(0, cursorColumn)
    } else if (mode === 1) {
      const deleteCount = Math.min(cursorColumn + 1, row.text.length)
      row.text = `${' '.repeat(deleteCount)}${row.text.slice(deleteCount)}`
    } else if (mode === 2) {
      row.text = ''
    }
  }

  for (let index = 0; index < normalizedChunk.length; index += 1) {
    const char = normalizedChunk[index]
    if (char === '\n') {
      ensureCursorRow()
      rows[cursorRow]!.completed = true
      newCompleteLines += 1
      cursorRow += 1
      cursorColumn = 0
      ensureCursorRow()
      trimRows()
      continue
    }
    if (char === '\r') {
      cursorColumn = 0
      continue
    }
    if (char === '\u0008') {
      cursorColumn = Math.max(0, cursorColumn - 1)
      continue
    }
    if (char === '\u001b') {
      const parsed = parseAnsiControlSequence(normalizedChunk, index)
      if (!parsed) {
        continue
      }
      index = parsed.endIndex
      if (parsed.kind !== 'csi' || !hasCanonicalNumericCsiParams(parsed.params)) {
        continue
      }
      const firstParam = parsed.firstParam ?? 1
      if (parsed.final === 'A') {
        cursorRow = Math.max(0, cursorRow - firstParam)
        rows.splice(cursorRow + 1)
      } else if (parsed.final === 'K') {
        eraseLine(parsed.firstParam ?? 0)
      } else if (parsed.final === 'G' || parsed.final === '`') {
        moveCursorToColumn(firstParam - 1)
      } else if (parsed.final === 'D') {
        cursorColumn = Math.max(0, cursorColumn - firstParam)
      } else if (parsed.final === 'C') {
        moveCursorToColumn(cursorColumn + firstParam)
      }
      continue
    }
    writeChar(char)
  }

  return finalizeRetainedTerminalRows(rows, cursorRow, cursorColumn, truncated, newCompleteLines)
}

type RetainedTailRedrawCursor = {
  rowFromEnd: number
  column: number
}

type RetainedTerminalRow = {
  text: string
  completed: boolean
}

function finalizeRetainedTerminalRows(
  rows: RetainedTerminalRow[],
  cursorRow: number,
  cursorColumn: number,
  initialTruncated: boolean,
  newCompleteLines: number
): {
  lines: string[]
  partialLine: string
  redrawCursor: RetainedTailRedrawCursor | null
  truncated: boolean
  newCompleteLines: number
} {
  let truncated = initialTruncated
  let retainedRows = rows.map((row) => ({ ...row, text: row.text.replace(/[ \t]+$/g, '') }))

  if (retainedRows.length > MAX_TAIL_LINES + 1) {
    const removeCount = retainedRows.length - (MAX_TAIL_LINES + 1)
    retainedRows = retainedRows.slice(removeCount)
    cursorRow = Math.max(0, cursorRow - removeCount)
    truncated = true
  }

  let totalChars = retainedRows.reduce((sum, row) => sum + row.text.length, 0)
  let trimStartIndex = 0
  while (trimStartIndex < retainedRows.length - 1 && totalChars > MAX_TAIL_CHARS) {
    totalChars -= retainedRows[trimStartIndex]!.text.length
    trimStartIndex += 1
  }
  if (trimStartIndex > 0) {
    retainedRows = retainedRows.slice(trimStartIndex)
    cursorRow = Math.max(0, cursorRow - trimStartIndex)
    truncated = true
  }
  while (
    retainedRows.length > 1 &&
    cursorRow < retainedRows.length - 1 &&
    retainedRows.at(-1)?.completed === false &&
    retainedRows.at(-1)?.text.length === 0
  ) {
    retainedRows.pop()
  }

  const lastRow = retainedRows.at(-1)
  let partialLine = lastRow && !lastRow.completed ? lastRow.text : ''
  let lines = (lastRow && !lastRow.completed ? retainedRows.slice(0, -1) : retainedRows).map(
    (row) => row.text
  )

  if (partialLine.length > MAX_TAIL_PARTIAL_CHARS) {
    partialLine = partialLine.slice(-MAX_TAIL_PARTIAL_CHARS)
    truncated = true
  }
  if (lines.length > MAX_TAIL_LINES) {
    lines = lines.slice(lines.length - MAX_TAIL_LINES)
    truncated = true
  }
  const outputRowCount = lines.length + 1
  const defaultCursorRow = outputRowCount - 1
  const defaultCursorColumn = partialLine.length
  const redrawCursor =
    cursorRow === defaultCursorRow && cursorColumn === defaultCursorColumn
      ? null
      : {
          rowFromEnd: Math.max(0, outputRowCount - 1 - cursorRow),
          column: clampTerminalPreviewCursor(cursorColumn)
        }

  return {
    lines,
    partialLine,
    redrawCursor,
    truncated,
    newCompleteLines
  }
}

function splitRetainedTerminalTailSegments(value: string): {
  completeSegments: string[]
  partialSegment: string
  completeLineCount: number
} {
  let completeLineCount = 0
  for (let index = 0; index < value.length; index += 1) {
    if (value[index] === '\n') {
      completeLineCount += 1
    }
  }

  const retainedCompleteCount = Math.min(completeLineCount, MAX_TAIL_LINES)
  const omittedCompleteCount = completeLineCount - retainedCompleteCount
  let startIndex = 0
  if (omittedCompleteCount > 0) {
    let seen = 0
    for (let index = 0; index < value.length; index += 1) {
      if (value[index] !== '\n') {
        continue
      }
      seen += 1
      if (seen === omittedCompleteCount) {
        startIndex = index + 1
        break
      }
    }
  }

  const completeSegments: string[] = []
  let segmentStart = startIndex
  for (let index = startIndex; index < value.length; index += 1) {
    if (value[index] !== '\n') {
      continue
    }
    completeSegments.push(value.slice(segmentStart, index))
    segmentStart = index + 1
  }

  return {
    completeSegments,
    partialSegment: value.slice(segmentStart),
    completeLineCount
  }
}

function processTerminalTailCompleteSegments(segments: string[]): string[] {
  const processed: string[] = []
  let totalChars = 0
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const line = applyTerminalLineControls(segments[index]!).text
    processed.push(line)
    totalChars += line.length
    if (totalChars > MAX_TAIL_CHARS) {
      break
    }
  }
  processed.reverse()
  return processed
}

function applyTerminalLineControls(line: string): {
  text: string
  cursorColumn: number
  hadControl: boolean
} {
  const carriageIndex = line.lastIndexOf('\r')
  const latestRedraw = carriageIndex >= 0 ? line.slice(carriageIndex + 1) : line
  if (!latestRedraw.includes('\u0008') && !latestRedraw.includes('\u001b')) {
    return {
      text: latestRedraw,
      cursorColumn: latestRedraw.length,
      hadControl: carriageIndex >= 0
    }
  }

  const chars: string[] = []
  let cursor = 0
  const moveCursorTo = (nextCursor: number): void => {
    cursor = clampTerminalPreviewCursor(nextCursor)
  }
  const writeChar = (char: string): void => {
    if (cursor > chars.length) {
      const oldLength = chars.length
      chars.length = cursor
      chars.fill(' ', oldLength, cursor)
    }
    if (cursor >= chars.length) {
      chars.push(char)
    } else {
      chars[cursor] = char
    }
    cursor += 1
  }
  for (let index = 0; index < latestRedraw.length; index += 1) {
    const char = latestRedraw[index]
    if (char === '\u0008') {
      if (cursor > 0) {
        cursor -= 1
      }
    } else if (char === '\u001b') {
      const parsed = parseAnsiControlSequence(latestRedraw, index)
      if (!parsed) {
        continue
      }
      index = parsed.endIndex
      if (parsed.kind !== 'csi') {
        continue
      }
      if (!hasCanonicalNumericCsiParams(parsed.params)) {
        continue
      }
      if (parsed.final === 'K') {
        const mode = parsed.firstParam ?? 0
        if (mode === 0) {
          chars.length = cursor
        } else if (mode === 1) {
          const deleteCount = Math.min(cursor + 1, chars.length)
          chars.fill(' ', 0, deleteCount)
        } else if (mode === 2) {
          chars.length = 0
        }
      } else if (parsed.final === 'G' || parsed.final === '`') {
        moveCursorTo((parsed.firstParam ?? 1) - 1)
      } else if (parsed.final === 'D') {
        cursor = Math.max(0, cursor - (parsed.firstParam ?? 1))
      } else if (parsed.final === 'C') {
        moveCursorTo(cursor + (parsed.firstParam ?? 1))
      }
    } else {
      writeChar(char)
    }
  }
  return { text: chars.join(''), cursorColumn: cursor, hadControl: true }
}

function clampTerminalPreviewCursor(nextCursor: number): number {
  if (!Number.isFinite(nextCursor)) {
    return MAX_TAIL_PARTIAL_CHARS
  }
  return Math.min(MAX_TAIL_PARTIAL_CHARS, Math.max(0, Math.floor(nextCursor)))
}

function parseAnsiControlSequence(
  value: string,
  escapeIndex: number
):
  | { kind: 'csi'; final: string; params: string; firstParam: number | null; endIndex: number }
  | {
      kind: 'other'
      endIndex: number
    }
  | null {
  const introducer = value[escapeIndex + 1]
  if (introducer === '[') {
    for (let index = escapeIndex + 2; index < value.length; index += 1) {
      const code = value.charCodeAt(index)
      if (code < 0x40 || code > 0x7e) {
        continue
      }
      const params = value.slice(escapeIndex + 2, index)
      const firstParamMatch = /^(\d+)/.exec(params)
      return {
        kind: 'csi',
        final: value[index] ?? '',
        params,
        firstParam: firstParamMatch ? Number(firstParamMatch[1]) : null,
        endIndex: index
      }
    }
    return null
  }
  if (introducer === ']') {
    for (let index = escapeIndex + 2; index < value.length; index += 1) {
      if (value[index] === '\u0007') {
        return { kind: 'other', endIndex: index }
      }
      if (value[index] === '\u001b' && value[index + 1] === '\\') {
        return { kind: 'other', endIndex: index + 1 }
      }
    }
    return null
  }
  if (isStTerminatedStringControlIntroducer(introducer)) {
    for (let index = escapeIndex + 2; index < value.length; index += 1) {
      if (value[index] === '\u001b' && value[index + 1] === '\\') {
        return { kind: 'other', endIndex: index + 1 }
      }
    }
    return null
  }
  return { kind: 'other', endIndex: escapeIndex + 1 }
}

function isStTerminatedStringControlIntroducer(introducer: string | undefined): boolean {
  return introducer === 'P' || introducer === 'X' || introducer === '^' || introducer === '_'
}

function hasCanonicalNumericCsiParams(params: string): boolean {
  return /^[0-9;]*$/.test(params)
}

function containsTerminalVerticalLineControl(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    if (value[index] !== '\u001b') {
      continue
    }
    const parsed = parseAnsiControlSequence(value, index)
    if (!parsed) {
      return false
    }
    index = parsed.endIndex
    if (
      parsed.kind === 'csi' &&
      parsed.final === 'A' &&
      hasCanonicalNumericCsiParams(parsed.params)
    ) {
      return true
    }
  }
  return false
}

function tailStateMatches(
  lines: string[],
  partialLine: string,
  pendingAnsi: string,
  redrawCursor: RetainedTailRedrawCursor | null,
  truncated: boolean,
  linesTotal: number,
  snapshot: {
    lines: string[]
    partialLine: string
    pendingAnsi: string
    redrawCursor: RetainedTailRedrawCursor | null
    truncated: boolean
    linesTotal: number
  }
): boolean {
  if (
    partialLine !== snapshot.partialLine ||
    pendingAnsi !== snapshot.pendingAnsi ||
    !tailRedrawCursorsMatch(redrawCursor, snapshot.redrawCursor) ||
    truncated !== snapshot.truncated ||
    linesTotal !== snapshot.linesTotal ||
    lines.length !== snapshot.lines.length
  ) {
    return false
  }
  if (lines === snapshot.lines) {
    return true
  }
  for (let index = 0; index < lines.length; index++) {
    if (lines[index] !== snapshot.lines[index]) {
      return false
    }
  }
  return true
}

function tailRedrawCursorsMatch(
  left: RetainedTailRedrawCursor | null,
  right: RetainedTailRedrawCursor | null
): boolean {
  if (left === right) {
    return true
  }
  if (!left || !right) {
    return false
  }
  return left.rowFromEnd === right.rowFromEnd && left.column === right.column
}

function buildTailLines(lines: string[], partialLine: string): string[] {
  return partialLine.length > 0 ? [...lines, partialLine] : lines
}

function terminalReadLimit(limit: number | undefined, defaultLimit: number): number {
  if (typeof limit !== 'number' || !Number.isFinite(limit) || limit <= 0) {
    return defaultLimit
  }
  return Math.min(Math.max(1, Math.floor(limit)), MAX_TERMINAL_READ_LIMIT)
}

function trimTerminalPreviewToCharacterBudget(
  lines: string[],
  characterBudget: number
): { tail: string[]; limited: boolean; omittedLineCount: number; slicedFirstLine: boolean } {
  let totalCharacters = lines.reduce((sum, line) => sum + line.length, 0)
  if (totalCharacters <= characterBudget) {
    return { tail: lines, limited: false, omittedLineCount: 0, slicedFirstLine: false }
  }

  let omittedLineCount = 0
  while (
    omittedLineCount < lines.length &&
    totalCharacters - lines[omittedLineCount].length >= characterBudget
  ) {
    totalCharacters -= lines[omittedLineCount].length
    omittedLineCount += 1
  }
  const tail = omittedLineCount > 0 ? lines.slice(omittedLineCount) : [...lines]

  let slicedFirstLine = false
  if (tail.length > 0 && totalCharacters > characterBudget) {
    tail[0] = tail[0].slice(totalCharacters - characterBudget)
    slicedFirstLine = true
  }

  return { tail, limited: true, omittedLineCount, slicedFirstLine }
}

function readTerminalTail(args: {
  handle: string
  status: RuntimeTerminalState
  completedLines: string[]
  partialLine: string
  completedLineCount: number
  bufferTruncated: boolean
  cursor?: number
  limit?: number
}): RuntimeTerminalRead {
  const oldestCursor = Math.max(0, args.completedLineCount - args.completedLines.length)
  const latestCursor = args.completedLineCount

  if (typeof args.cursor === 'number' && args.cursor >= 0) {
    const limit = terminalReadLimit(args.limit, MAX_TERMINAL_READ_LIMIT)
    if (args.cursor > latestCursor) {
      return {
        handle: args.handle,
        status: args.status,
        tail: [],
        truncated: false,
        limited: false,
        oldestCursor: String(oldestCursor),
        nextCursor: String(latestCursor),
        latestCursor: String(latestCursor),
        returnedLineCount: 0
      }
    }
    // Why: cursor reads are transcript/pagination reads. They return completed
    // lines only so a partial line is not delivered once as "hel" and again as
    // "hello" after the newline arrives.
    const startCursor = Math.max(args.cursor, oldestCursor)
    const startIndex = startCursor - oldestCursor
    const available = args.completedLines.slice(startIndex)
    const tail = available.slice(0, limit)
    const nextCursor = startCursor + tail.length
    return {
      handle: args.handle,
      status: args.status,
      tail,
      truncated: args.cursor < oldestCursor,
      limited: tail.length < available.length,
      oldestCursor: String(oldestCursor),
      nextCursor: String(nextCursor),
      latestCursor: String(latestCursor),
      returnedLineCount: tail.length
    }
  }

  // Why: un-cursored reads are preview reads for humans/agents. Return the
  // latest bounded view, while the larger retained buffer remains available
  // through cursor reads plus --limit.
  const limit = terminalReadLimit(args.limit, DEFAULT_TERMINAL_READ_LIMIT)
  const allLines = buildTailLines(args.completedLines, args.partialLine)
  const lineBoundedTail = allLines.slice(-limit)
  const charBoundedTail = trimTerminalPreviewToCharacterBudget(
    lineBoundedTail,
    MAX_TERMINAL_PREVIEW_CHARS
  )
  const lineBoundedStartIndex = Math.max(0, allLines.length - lineBoundedTail.length)
  const charBoundedStartIndex = lineBoundedStartIndex + charBoundedTail.omittedLineCount
  const hasPageableOmittedCompletedLines =
    Math.min(args.completedLineCount, charBoundedStartIndex) > 0 ||
    (charBoundedTail.slicedFirstLine && charBoundedStartIndex < args.completedLineCount)
  // Why: a long unterminated partial line can exceed the preview character
  // budget, but cursor reads only page completed lines, so the trimmed bytes
  // cannot be recovered by asking for nextCursor again.
  const truncatedByNonPageablePartial = charBoundedTail.limited && !hasPageableOmittedCompletedLines
  return {
    handle: args.handle,
    status: args.status,
    tail: charBoundedTail.tail,
    truncated: args.bufferTruncated || truncatedByNonPageablePartial,
    limited: lineBoundedTail.length < allLines.length || charBoundedTail.limited,
    oldestCursor: String(oldestCursor),
    nextCursor: String(latestCursor),
    latestCursor: String(latestCursor),
    returnedLineCount: charBoundedTail.tail.length
  }
}

function shouldFallbackToVisibleTerminalSnapshot(
  read: RuntimeTerminalRead,
  opts: { cursor?: number; limit?: number }
): boolean {
  if (typeof opts.cursor === 'number') {
    return false
  }
  if (read.tail.length === 0) {
    return false
  }
  const hasSubstantialBlankTail =
    read.limited === true || read.truncated || read.tail.length >= DEFAULT_TERMINAL_READ_LIMIT
  return hasSubstantialBlankTail && read.tail.every((line) => line.trim().length === 0)
}

function buildVisibleSnapshotReadFallback(
  read: RuntimeTerminalRead,
  visibleLines: string[],
  limit: number | undefined
): RuntimeTerminalRead {
  const lineLimit = terminalReadLimit(limit, DEFAULT_TERMINAL_READ_LIMIT)
  const lineBoundedTail = visibleLines.slice(-lineLimit)
  const charBoundedTail = trimTerminalPreviewToCharacterBudget(
    lineBoundedTail,
    MAX_TERMINAL_PREVIEW_CHARS
  )
  return {
    ...read,
    tail: charBoundedTail.tail,
    limited:
      read.limited || lineBoundedTail.length < visibleLines.length || charBoundedTail.limited,
    returnedLineCount: charBoundedTail.tail.length
  }
}

function getTerminalState(leaf: RuntimeLeafRecord): RuntimeTerminalState {
  if (leaf.connected) {
    return 'running'
  }
  if (leaf.lastExitCode !== null) {
    return 'exited'
  }
  return 'unknown'
}

function buildSendPayload(action: {
  text?: string
  enter?: boolean
  interrupt?: boolean
}): string | null {
  let payload = ''
  if (typeof action.text === 'string' && action.text.length > 0) {
    payload += action.text
  }
  if (action.enter) {
    payload += '\r'
  }
  if (action.interrupt) {
    payload += '\x03'
  }
  return payload.length > 0 ? payload : null
}

async function assertTerminalInputWithinLimitWithYield(text: string | undefined): Promise<void> {
  if (!text) {
    return
  }
  if (await isTerminalInputTooLargeWithYield(text)) {
    throw new Error(TERMINAL_INPUT_TOO_LARGE_ERROR)
  }
}

// Why: tui-idle relies on recognized agent CLIs setting OSC titles. If the
// terminal runs an unsupported CLI (or a plain shell), no title transition
// will ever fire. A 5-minute ceiling prevents indefinite hangs while still
// giving real agent tasks plenty of time to complete.
const TUI_IDLE_DEFAULT_TIMEOUT_MS = 5 * 60 * 1000
const TUI_IDLE_POLL_INTERVAL_MS = 2000
const TUI_IDLE_QUIESCENCE_MS = 3000
const MESSAGE_WAIT_DEFAULT_TIMEOUT_MS = 2 * 60 * 1000
const EXPLICIT_IDLE_TITLE_RE = /(^|\s)(ready|idle|done)(\s|$|[.!?])/i
const CLAUDE_IDLE_PREFIX = '\u2733'
const GEMINI_IDLE_PREFIX = '\u25c7'
const PI_IDLE_PREFIX = '\u03c0 - '

// Clamp range for the user-facing mobileAutoRestoreFitMs preference.
// MIN floor: a couple of seconds is the smallest useful auto-restore
// (anything tighter is the legacy 300ms debounce).
// MAX ceiling: one hour — a held PTY beyond that is almost certainly
// "I forgot" rather than intentional.
const MOBILE_AUTO_RESTORE_FIT_MIN_MS = 5_000
const MOBILE_AUTO_RESTORE_FIT_MAX_MS = 60 * 60 * 1000

function detectExplicitIdleStatusFromTitle(title: string): AgentStatus | null {
  const status = detectAgentStatusFromTitle(title)
  if (status !== 'idle') {
    return null
  }
  // Why: user-supplied launch titles like "Codex YOLO" contain an agent name
  // but are not readiness signals. terminal.wait needs explicit idle evidence.
  if (
    EXPLICIT_IDLE_TITLE_RE.test(title) ||
    title.startsWith(CLAUDE_IDLE_PREFIX) ||
    title.startsWith('* ') ||
    title.includes(GEMINI_IDLE_PREFIX) ||
    title.startsWith(PI_IDLE_PREFIX)
  ) {
    return 'idle'
  }
  return null
}

function isKnownReadyPromptPreview(preview: string): boolean {
  const normalized = preview.toLowerCase()
  const readyIndex = findKnownReadyPromptIndex(normalized)
  if (readyIndex === null) {
    return false
  }
  const blockedSignal = findTerminalWaitBlockedSignal(normalized)
  if (blockedSignal !== null && blockedSignal.index > readyIndex) {
    return false
  }
  return true
}

function detectTerminalWaitBlockedReason(preview: string): RuntimeTerminalWaitBlockedReason | null {
  const normalized = preview.toLowerCase()
  return findActionableTerminalWaitBlockedSignal(normalized)?.reason ?? null
}

function findActionableTerminalWaitBlockedSignal(
  normalized: string
): { reason: RuntimeTerminalWaitBlockedReason; index: number } | null {
  const blockedSignal = findTerminalWaitBlockedSignal(normalized)
  if (blockedSignal === null) {
    return null
  }
  const dismissedModalIndex = findDismissedStartupModalIndex(normalized)
  // Why: retained terminal tails can include stale startup modals. If a known
  // agent's live prompt appears after that modal, the modal was dismissed and
  // the signal is no longer actionable — even if the agent is still mid-run
  // (Cursor never reports idle via OSC title, so its busy prompt clears too).
  return dismissedModalIndex !== null && dismissedModalIndex > blockedSignal.index
    ? null
    : blockedSignal
}

// Why: a recognized agent's live prompt (idle OR busy) proves its startup modal
// was dismissed. Broader than the idle-only ready set so a mid-run Cursor lane
// stops reporting a stale trust hit for the rest of the session.
function findDismissedStartupModalIndex(normalized: string): number | null {
  const indexes = [
    findCodexReadyPromptIndex(normalized),
    findAntigravityReadyPromptIndex(normalized),
    findCursorActivePromptIndex(normalized)
  ].filter((index): index is number => index !== null)
  return indexes.length > 0 ? Math.max(...indexes) : null
}

function findKnownReadyPromptIndex(normalized: string): number | null {
  const indexes = [
    findCodexReadyPromptIndex(normalized),
    findAntigravityReadyPromptIndex(normalized),
    findCursorReadyPromptIndex(normalized)
  ].filter((index): index is number => index !== null)
  return indexes.length > 0 ? Math.max(...indexes) : null
}

// Why: cursor-agent keeps a persistent TUI — a printed "Cursor Agent" banner and
// a "→" input-prompt line appear once its trust dialog is dismissed, in both
// busy and idle states. The banner is matched by its last occurrence so the
// trust dialog's own "Cursor Agent" body text (which precedes the banner) does
// not win. The "→" glyph is cursor-agent's input prompt marker ("→ Plan,
// search, build anything" fresh, "→ Add a follow-up" after the first turn).
function findCursorActivePromptIndex(normalized: string): number | null {
  const headerIndex = normalized.lastIndexOf('cursor agent')
  if (headerIndex === -1) {
    return null
  }
  return normalized.includes('→', headerIndex) ? headerIndex : null
}

// Why: cursor-agent never emits an idle OSC title (its bare title is dropped),
// so tui-idle can only resolve from the tail. Busy frames draw a braille
// spinner in the on-screen status line; its absence past the banner is idle.
const CURSOR_BUSY_SPINNER_RE = /[⠁-⣿]/

function findCursorReadyPromptIndex(normalized: string): number | null {
  const activeIndex = findCursorActivePromptIndex(normalized)
  if (activeIndex === null) {
    return null
  }
  return CURSOR_BUSY_SPINNER_RE.test(normalized.slice(activeIndex)) ? null : activeIndex
}

function findCodexReadyPromptIndex(normalized: string): number | null {
  const headerIndex = normalized.lastIndexOf('openai codex')
  if (headerIndex === -1) {
    return null
  }
  const readySegment = normalized.slice(headerIndex)
  // Why: current Codex prints permissions only in YOLO mode. The stable ready
  // header is OpenAI Codex + model + directory.
  return readySegment.includes('model:') && readySegment.includes('directory:') ? headerIndex : null
}

function findAntigravityReadyPromptIndex(normalized: string): number | null {
  const headerIndex = normalized.lastIndexOf('antigravity cli')
  if (headerIndex === -1) {
    return null
  }
  let lineStart = headerIndex
  let modelIndex: number | null = null
  let promptIndex: number | null = null

  // Why: ready previews can include echoed pasted output after the header;
  // scan line bounds directly instead of splitting the whole terminal tail.
  for (let cursor = headerIndex; cursor <= normalized.length; cursor += 1) {
    if (cursor < normalized.length && normalized.charCodeAt(cursor) !== 10) {
      continue
    }
    let trimmedStart = lineStart
    let trimmedEnd = cursor
    while (trimmedStart < trimmedEnd && isTerminalWaitWhitespace(normalized, trimmedStart)) {
      trimmedStart += 1
    }
    while (trimmedEnd > trimmedStart && isTerminalWaitWhitespace(normalized, trimmedEnd - 1)) {
      trimmedEnd -= 1
    }
    if (lineStart > headerIndex && trimmedStart < trimmedEnd) {
      if (modelIndex === null && normalized.startsWith('gemini', trimmedStart)) {
        modelIndex = trimmedStart
      }
      if (
        promptIndex === null &&
        trimmedEnd - trimmedStart === 1 &&
        normalized.charCodeAt(trimmedStart) === 62
      ) {
        promptIndex = trimmedStart
      }
    }
    lineStart = cursor + 1
  }

  return modelIndex !== null && promptIndex !== null ? Math.max(modelIndex, promptIndex) : null
}

function isTerminalWaitWhitespace(value: string, index: number): boolean {
  const code = value.charCodeAt(index)
  return code === 32 || (code >= 9 && code <= 13)
}

const TERMINAL_WAIT_BLOCKED_SENTINEL_RE =
  /update available|choose working directory to|codex just got an upgrade|hooks need review|do you trust|trust this|trusted workspace|press enter to (?:confirm|continue|view|insert)|press t to trust/i

function findTerminalWaitBlockedSignal(
  normalized: string
): { reason: RuntimeTerminalWaitBlockedReason; index: number } | null {
  // Why: this runs once per PTY chunk over a tail up to 256 KiB. One combined
  // negative scan avoids a dozen full-tail searches when no prompt can match.
  if (!TERMINAL_WAIT_BLOCKED_SENTINEL_RE.test(normalized)) {
    return null
  }
  const candidates: { reason: RuntimeTerminalWaitBlockedReason; index: number }[] = []
  const updateIndex = normalized.lastIndexOf('update available')
  if (updateIndex !== -1 && normalized.includes('press enter to continue', updateIndex)) {
    candidates.push({ reason: 'codex-update-prompt', index: updateIndex })
  }
  const cwdIndex = normalized.lastIndexOf('choose working directory to')
  if (cwdIndex !== -1 && normalized.includes('press enter to continue', cwdIndex)) {
    candidates.push({ reason: 'codex-cwd-prompt', index: cwdIndex })
  }
  const modelMigrationIndex = normalized.lastIndexOf('codex just got an upgrade')
  if (
    modelMigrationIndex !== -1 &&
    normalized.includes('press enter to continue', modelMigrationIndex)
  ) {
    candidates.push({ reason: 'codex-model-migration-prompt', index: modelMigrationIndex })
  }
  const hooksIndex = normalized.lastIndexOf('hooks need review')
  if (hooksIndex !== -1 && normalized.includes('press enter to confirm', hooksIndex)) {
    candidates.push({ reason: 'codex-hooks-review-prompt', index: hooksIndex })
  }
  const trustIndex = Math.max(
    normalized.lastIndexOf('do you trust'),
    normalized.lastIndexOf('trust this'),
    normalized.lastIndexOf('trusted workspace')
  )
  const trustSegment = trustIndex === -1 ? '' : normalized.slice(trustIndex)
  if (
    trustIndex !== -1 &&
    (trustSegment.includes('workspace') ||
      trustSegment.includes('folder') ||
      trustSegment.includes('directory') ||
      trustSegment.includes('repo'))
  ) {
    candidates.push({ reason: 'codex-trust-workspace', index: trustIndex })
  }
  const interactivePromptIndex = Math.max(
    normalized.lastIndexOf('press enter to confirm'),
    normalized.lastIndexOf('press enter to continue'),
    normalized.lastIndexOf('press enter to view'),
    normalized.lastIndexOf('press enter to insert'),
    normalized.lastIndexOf('press t to trust')
  )
  const interactivePromptContext =
    interactivePromptIndex === -1
      ? ''
      : normalized.slice(Math.max(0, interactivePromptIndex - 600), interactivePromptIndex + 200)
  const hasCodexInteractiveContext =
    interactivePromptContext.includes('codex') ||
    interactivePromptContext.includes('permission') ||
    interactivePromptContext.includes('sandbox') ||
    interactivePromptContext.includes('trust') ||
    interactivePromptContext.includes('hook')
  if (interactivePromptIndex !== -1 && hasCodexInteractiveContext) {
    const contextStart = Math.max(0, interactivePromptIndex - 600)
    const hasSpecificPromptInContext = candidates.some(
      (candidate) => candidate.index >= contextStart && candidate.index <= interactivePromptIndex
    )
    if (!hasSpecificPromptInContext) {
      candidates.push({ reason: 'codex-interactive-prompt', index: interactivePromptIndex })
    }
  }
  return candidates.length > 0
    ? candidates.reduce((latest, candidate) =>
        candidate.index > latest.index ? candidate : latest
      )
    : null
}

function buildTerminalWaitResult(
  handle: string,
  condition: RuntimeTerminalWaitCondition,
  leaf: RuntimeLeafRecord
): RuntimeTerminalWait {
  return buildTerminalWait(handle, condition, getTerminalState(leaf), leaf.lastExitCode)
}

function buildTerminalWaitBlockedResult(
  handle: string,
  condition: RuntimeTerminalWaitCondition,
  leaf: RuntimeLeafRecord,
  blockedReason: RuntimeTerminalWaitBlockedReason
): RuntimeTerminalWait {
  return buildTerminalWait(
    handle,
    condition,
    getTerminalState(leaf),
    leaf.lastExitCode,
    blockedReason
  )
}

function buildPtyTerminalWaitResult(
  handle: string,
  condition: RuntimeTerminalWaitCondition,
  pty: RuntimePtyWorktreeRecord
): RuntimeTerminalWait {
  return buildTerminalWait(handle, condition, getPtyTerminalState(pty), pty.lastExitCode)
}

function buildPtyTerminalWaitBlockedResult(
  handle: string,
  condition: RuntimeTerminalWaitCondition,
  pty: RuntimePtyWorktreeRecord,
  blockedReason: RuntimeTerminalWaitBlockedReason
): RuntimeTerminalWait {
  return buildTerminalWait(
    handle,
    condition,
    getPtyTerminalState(pty),
    pty.lastExitCode,
    blockedReason
  )
}

function buildTerminalWait(
  handle: string,
  condition: RuntimeTerminalWaitCondition,
  status: RuntimeTerminalState,
  exitCode: number | null,
  blockedReason?: RuntimeTerminalWaitBlockedReason
): RuntimeTerminalWait {
  return {
    handle,
    condition,
    satisfied: blockedReason === undefined,
    status,
    exitCode,
    ...(blockedReason ? { blockedReason } : {})
  }
}

function getPtyTerminalState(pty: RuntimePtyWorktreeRecord): RuntimeTerminalState {
  return pty.connected ? 'running' : pty.lastExitCode !== null ? 'exited' : 'unknown'
}

function branchSelectorMatches(branch: string, selector: string): boolean {
  // Why: Git worktree data can report local branches as either `refs/heads/foo`
  // or `foo` depending on which plumbing path produced the record. Orca's
  // branch selectors should accept either form so newly created worktrees stay
  // discoverable without exposing internal ref-shape differences to users.
  return normalizeLocalBranchName(branch) === normalizeLocalBranchName(selector)
}

function runtimePathsEqual(left: string, right: string): boolean {
  return normalizeRuntimePathForComparison(left) === normalizeRuntimePathForComparison(right)
}

function inferWorktreeIdFromPtyId(ptyId: string): string | null {
  return parsePtySessionId(ptyId).worktreeId
}

function setsEqual<T>(a: ReadonlySet<T>, b: ReadonlySet<T>): boolean {
  if (a.size !== b.size) {
    return false
  }
  for (const value of a) {
    if (!b.has(value)) {
      return false
    }
  }
  return true
}

function parseRuntimeWorktreeId(
  worktreeId: string
): { repoId: string; worktreePath: string } | null {
  const parsed = splitWorktreeId(worktreeId)
  if (!parsed?.repoId) {
    return null
  }
  if (!parsed.worktreePath) {
    return null
  }
  return parsed
}

function includeTargetResolvedWorktree(
  resolvedWorktrees: ResolvedWorktree[],
  targetWorktree: ResolvedWorktree | null
): ResolvedWorktree[] {
  if (!targetWorktree || resolvedWorktrees.some((worktree) => worktree.id === targetWorktree.id)) {
    return resolvedWorktrees
  }
  return [...resolvedWorktrees, targetWorktree]
}

function findResolvedWorktreeIdForPath(
  resolvedWorktrees: ResolvedWorktree[],
  cwd: string
): string | null {
  if (!cwd) {
    return null
  }
  const matches = resolvedWorktrees
    .filter((worktree) => isPathInsideOrEqual(worktree.path, cwd))
    .sort((left, right) => right.path.length - left.path.length)
  return matches[0]?.id ?? null
}

function getLeafWorktreeStatus(
  leaf: RuntimeLeafRecord,
  tabTitle: string | null
): RuntimeWorktreeStatus {
  // Why: recompute from the live title each call so worktree.ps mirrors what
  // the desktop sidebar's getWorktreeStatus does (no sticky state). Prefer
  // the freshest pane/OSC title, then tab title. Falling back to lastAgentStatus
  // only when no title is available preserves a sensible signal for very fresh
  // leaves before any title has been observed.
  const titleCandidates = [
    { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
    { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt },
    { title: tabTitle, updatedAt: 0 }
  ]
  const latestTitle = getLatestAgentCandidateTitle(...titleCandidates)
  const detected = latestTitle ? detectAgentStatusFromTitle(latestTitle) : leaf.lastAgentStatus
  return getDetectedWorktreeStatus(detected, leaf.ptyId !== null)
}

function classifyLatestAgentTitle(
  ...titles: { title: string | null | undefined; updatedAt: number | null | undefined }[]
): 'agent' | 'management' | 'neutral' {
  return classifyAgentTitle(getLatestAgentCandidateTitle(...titles))
}

function getLatestPtyTitle(pty: RuntimePtyWorktreeRecord): string | null {
  return getLatestAgentCandidateTitle(
    { title: pty.title, updatedAt: pty.titleUpdatedAt },
    { title: pty.lastOscTitle, updatedAt: pty.lastOscTitleAt }
  )
}

function getLatestLeafTitle(leaf: RuntimeLeafRecord, tabTitle: string | null): string | null {
  return getLatestAgentCandidateTitle(
    { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
    { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt },
    { title: tabTitle, updatedAt: 0 }
  )
}

function classifyAgentTitle(title: string | null): 'agent' | 'management' | 'neutral' {
  if (!title) {
    return 'neutral'
  }
  if (isClaudeManagementTitle(title)) {
    return 'management'
  }
  return detectAgentStatusFromTitle(title) !== null ? 'agent' : 'neutral'
}

function terminalTitleBlocksExplicitAgentStatus(title: string | null): boolean {
  if (!title) {
    return false
  }
  return isClaudeManagementTitle(title) || isShellProcess(title)
}

function getLatestAgentCandidateTitle(
  ...titles: { title: string | null | undefined; updatedAt: number | null | undefined }[]
): string | null {
  return getLatestAgentCandidateTitleInfo(...titles)?.title ?? null
}

function getLatestAgentCandidateTitleInfo(
  ...titles: { title: string | null | undefined; updatedAt: number | null | undefined }[]
): { title: string; updatedAt: number } | null {
  let latest: { title: string; updatedAt: number } | null = null
  for (const candidate of titles) {
    const title = candidate.title?.trim()
    if (!title) {
      continue
    }
    const updatedAt = candidate.updatedAt ?? 0
    if (!latest || updatedAt > latest.updatedAt) {
      latest = { title, updatedAt }
    }
  }
  return latest
}

function getSavedTabWorktreeStatus(title: string, hasPty: boolean): RuntimeWorktreeStatus {
  return getDetectedWorktreeStatus(detectAgentStatusFromTitle(title), hasPty)
}

function getDetectedWorktreeStatus(
  detected: AgentStatus | null,
  hasPty: boolean
): RuntimeWorktreeStatus {
  if (detected === 'permission') {
    return 'permission'
  }
  if (detected === 'working') {
    return 'working'
  }
  return hasPty ? 'active' : 'inactive'
}

function mapExplicitAgentStateToRuntimeTerminalStatus(
  state: AgentStatusEntry['state']
): NonNullable<RuntimeTerminalAgentStatus['status']> {
  switch (state) {
    case 'blocked':
    case 'waiting':
      return 'permission'
    case 'working':
      return 'working'
    case 'done':
      return 'idle'
  }
}

function mergeWorktreeStatus(
  current: RuntimeWorktreeStatus,
  next: RuntimeWorktreeStatus
): RuntimeWorktreeStatus {
  return WORKTREE_STATUS_PRIORITY[next] > WORKTREE_STATUS_PRIORITY[current] ? next : current
}

function normalizeTerminalChunk(
  chunk: string,
  pendingAnsi: string = ''
): { text: string; pendingAnsi: string } {
  // Why: most high-throughput PTY chunks are plain printable text. Avoid
  // running every ANSI/OSC regex over megabytes that do not need normalization.
  if (pendingAnsi.length === 0 && !terminalChunkNeedsNormalization(chunk)) {
    return { text: chunk, pendingAnsi: '' }
  }
  const combined = `${pendingAnsi}${chunk}`
  const parts: string[] = []
  let textStart = 0
  for (let index = 0; index < combined.length; index += 1) {
    const char = combined[index]
    if (char === '\x1b') {
      appendTerminalNormalizedSpan(parts, combined, textStart, index)
      if (index + 1 >= combined.length) {
        return { text: parts.join(''), pendingAnsi: combined.slice(index) }
      }
      const parsed = parseAnsiControlSequence(combined, index)
      if (!parsed) {
        return {
          text: parts.join(''),
          pendingAnsi: trimPendingAnsiControl(combined.slice(index))
        }
      }
      if (parsed.kind === 'csi' && isTerminalPreviewLineControl(parsed)) {
        // Why: Codex can redraw status text with ANSI controls but no CR; keep
        // those controls so the tail buffer overwrites the previous frame.
        parts.push(combined.slice(index, parsed.endIndex + 1))
      }
      index = parsed.endIndex
      textStart = index + 1
      continue
    }
    if (char === '\r' && combined[index + 1] === '\n') {
      appendTerminalNormalizedSpan(parts, combined, textStart, index)
      parts.push('\n')
      index += 1
      textStart = index + 1
      continue
    }
    const code = combined.charCodeAt(index)
    if (code === 0x08 || code === 0x09 || code === 0x0a || code === 0x0d) {
      appendTerminalNormalizedSpan(parts, combined, textStart, index)
      parts.push(char)
      textStart = index + 1
    } else if (!isTerminalPreviewPrintableCodeUnit(code)) {
      appendTerminalNormalizedSpan(parts, combined, textStart, index)
      textStart = index + 1
    }
  }
  appendTerminalNormalizedSpan(parts, combined, textStart, combined.length)
  return { text: parts.join(''), pendingAnsi: '' }
}

function appendTerminalNormalizedSpan(
  parts: string[],
  value: string,
  start: number,
  end: number
): void {
  if (end > start) {
    parts.push(value.slice(start, end))
  }
}

function isTerminalPreviewPrintableCodeUnit(code: number): boolean {
  return code >= 0x20 && code !== 0x7f && (code < 0x80 || code > 0x9f)
}

function terminalChunkNeedsNormalization(chunk: string): boolean {
  for (let index = 0; index < chunk.length; index++) {
    const code = chunk.charCodeAt(index)
    if (
      code === 0x1b ||
      code === 0x7f ||
      code === 0x0d ||
      code < 0x09 ||
      (code > 0x0a && code < 0x20) ||
      (code >= 0x80 && code <= 0x9f)
    ) {
      return true
    }
  }
  return false
}

function trimPendingAnsiControl(value: string): string {
  if (value.length <= MAX_TAIL_PENDING_ANSI_CHARS) {
    return value
  }
  const introducer = value.slice(0, Math.min(2, value.length))
  const suffixBudget = Math.max(0, MAX_TAIL_PENDING_ANSI_CHARS - introducer.length)
  return `${introducer}${value.slice(-suffixBudget)}`
}

function isTerminalPreviewLineControl(parsed: {
  final: string
  params: string
  firstParam: number | null
}): boolean {
  if (!hasCanonicalNumericCsiParams(parsed.params)) {
    return false
  }
  if (parsed.final === 'K') {
    const mode = parsed.firstParam ?? 0
    return mode === 0 || mode === 1 || mode === 2
  }
  return (
    parsed.final === 'A' ||
    parsed.final === 'G' ||
    parsed.final === '`' ||
    parsed.final === 'D' ||
    parsed.final === 'C'
  )
}

function maxTimestamp(left: number | null, right: number | null): number | null {
  if (left === null) {
    return right
  }
  if (right === null) {
    return left
  }
  return Math.max(left, right)
}

function compareWorktreePs(
  left: RuntimeWorktreePsSummary,
  right: RuntimeWorktreePsSummary
): number {
  // Pinned and unread worktrees sort above others so they survive truncation.
  if (left.isPinned !== right.isPinned) {
    return left.isPinned ? -1 : 1
  }
  if (left.unread !== right.unread) {
    return left.unread ? -1 : 1
  }
  const leftLast = left.lastOutputAt ?? -1
  const rightLast = right.lastOutputAt ?? -1
  if (leftLast !== rightLast) {
    return rightLast - leftLast
  }
  if (left.liveTerminalCount !== right.liveTerminalCount) {
    return right.liveTerminalCount - left.liveTerminalCount
  }
  return left.path.localeCompare(right.path)
}


// Why: OrcaRuntimeService calls the vast majority of this module's helpers
// directly (bare names) throughout its body — not just the 10 originally
// public exports. Bulk-exporting here (rather than scattering `export` at
// each declaration) keeps the accounting in one place; see the matching
// bulk import in orca-runtime.ts.
export {
  normalizeLocalBranchName,
  MAX_TAIL_CHARS,
  assertTerminalInputWithinLimitWithYield,
  branchSelectorMatches,
  buildPtyTerminalWaitBlockedResult,
  buildPtyTerminalWaitResult,
  buildSendPayload,
  buildTerminalWaitBlockedResult,
  buildTerminalWaitResult,
  buildTerminalWaitText,
  buildVisibleSnapshotReadFallback,
  classifyAgentTitle,
  classifyLatestAgentTitle,
  compareWorktreePs,
  detectExplicitIdleStatusFromTitle,
  detectTerminalWaitBlockedReason,
  findResolvedWorktreeIdForPath,
  getLatestAgentCandidateTitle,
  getLatestAgentCandidateTitleInfo,
  getLatestLeafTitle,
  getLatestPtyTitle,
  getLeafWorktreeStatus,
  getSavedTabWorktreeStatus,
  getTerminalState,
  includeTargetResolvedWorktree,
  inferWorktreeIdFromPtyId,
  isKnownReadyPromptPreview,
  mapExplicitAgentStateToRuntimeTerminalStatus,
  maxTimestamp,
  mergeWorktreeStatus,
  MESSAGE_WAIT_DEFAULT_TIMEOUT_MS,
  MOBILE_AUTO_RESTORE_FIT_MAX_MS,
  MOBILE_AUTO_RESTORE_FIT_MIN_MS,
  normalizeTerminalChunk,
  parseRuntimeWorktreeId,
  readTerminalTail,
  type RetainedTailRedrawCursor,
  runtimePathsEqual,
  setsEqual,
  shouldFallbackToVisibleTerminalSnapshot,
  tailStateMatches,
  terminalTitleBlocksExplicitAgentStatus,
  TUI_IDLE_DEFAULT_TIMEOUT_MS,
  TUI_IDLE_POLL_INTERVAL_MS,
  TUI_IDLE_QUIESCENCE_MS
}
