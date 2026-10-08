/**
 * code-intel-parsers.ts — FE-CV-TASK-050-03
 *
 * Envelope and push-event parsing for the code-intelligence protocol; also re-exports the error
 * code/kind table (code-intel-error-codes) and the status parser (code-intel-index-status-parser).
 *
 * Invariants:
 *  - Parsers never throw except parseCodeIntelEnvelope on a non-object (caller bug)
 *  - Unknown enum values become 'unknown'; unknown push events are dropped (null)
 *
 * @module shared/code-intel-parsers
 */

import type { CodeIntelEnvelope, EnvelopeSource, CodeIntelPushEvent } from './code-intel-types'
import { CODE_INTEL_PUSH_EVENTS } from './code-intel-rpc-methods'

export * from './code-intel-error-codes'
export * from './code-intel-index-status-parser'

// ---------------------------------------------------------------------------
// §2.2 Envelope parser
// ---------------------------------------------------------------------------

function parseEnvelopeSource(raw: unknown): EnvelopeSource | null {
  if (typeof raw !== 'object' || raw === null) {
    return null
  }
  const r = raw as Record<string, unknown>
  if (r.tool !== 'gitnexus' && r.tool !== 'codegraph') {
    return null
  }
  return {
    tool: r.tool,
    version: typeof r.version === 'string' ? r.version : '',
    indexedAt: typeof r.indexedAt === 'string' ? r.indexedAt : null,
    commit: typeof r.commit === 'string' ? r.commit : null,
    ...(r.lineBase === 1 ? { lineBase: 1 as const } : {})
  }
}

/**
 * Parse a raw envelope with safe defaults. A notModified envelope never calls parseData
 * (the backend sends no data in that case).
 */
export function parseCodeIntelEnvelope<T>(
  raw: unknown,
  parseData: (raw: unknown) => T
): CodeIntelEnvelope<T> {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('[code-intel] unexpected non-object envelope')
  }
  const r = raw as Record<string, unknown>
  const notModified = r.notModified === true

  return {
    ...(typeof r.repo === 'string' ? { repo: r.repo } : {}),
    worktreeId: typeof r.worktreeId === 'string' ? r.worktreeId : '',
    view: typeof r.view === 'string' ? r.view : '',
    sources: Array.isArray(r.sources)
      ? r.sources.map(parseEnvelopeSource).filter((s): s is EnvelopeSource => s !== null)
      : [],
    headCommit: typeof r.headCommit === 'string' ? r.headCommit : null,
    stale: r.stale === true,
    truncated: r.truncated === true,
    totalCount: typeof r.totalCount === 'number' ? r.totalCount : 0,
    etag: typeof r.etag === 'string' ? r.etag : '',
    fromCache: r.fromCache === true,
    generatedAt: typeof r.generatedAt === 'string' ? r.generatedAt : '',
    ...(notModified ? { notModified: true as const } : {}),
    ...(typeof r.nextPageToken === 'string' ? { nextPageToken: r.nextPageToken } : {}),
    ...(notModified ? {} : { data: parseData(r.data) })
  }
}

// ---------------------------------------------------------------------------
// §5 Push event parser
// ---------------------------------------------------------------------------

const PUSH_EVENT_NAMES = new Set<string>(CODE_INTEL_PUSH_EVENTS)

/**
 * Parse a raw push frame.
 * Returns null for unknown/malformed events (caller should discard).
 */
// Unrecognized reasons collapse to 'unknown' so consumers can switch exhaustively.
const CHANGED_REASONS = new Set(['commit', 'file_save', 'branch_switch', 'reindex', 'manual'])

// Contract §5 wire reasons -> internal reasons (resync is carried by `resync`).
const WIRE_CHANGED_REASON: Record<string, 'commit' | 'file_save' | 'branch_switch' | 'reindex' | 'manual'> = {
  index_changed: 'reindex',
  reindex_finished: 'reindex',
  head_changed: 'commit'
}

const FINISHED_STATUSES = new Set(['succeeded', 'failed', 'cancelled', 'interrupted'])

// Contract §5 wire names are dotted; internal names are camelCase.
const WIRE_EVENT_ALIASES: Record<string, string> = {
  'quality.progress': 'qualityProgress',
  'quality.finished': 'qualityFinished',
  'quality.gateChanged': 'gateChanged'
}

function normalizeWirePushFrame(r: Record<string, unknown>): Record<string, unknown> {
  const alias = typeof r.event === 'string' ? WIRE_EVENT_ALIASES[r.event] : undefined
  if (!alias) {
    return r
  }
  const out: Record<string, unknown> = { ...r, event: alias }
  if (alias === 'qualityFinished' && typeof r.status === 'string' && out.success === undefined) {
    out.success = r.status === 'succeeded'
    out.error = r.status === 'succeeded' ? null : (r.status as string)
  }
  if (alias === 'gateChanged' && out.gate === undefined && typeof r.verdict === 'string') {
    out.gate = String(r.verdict).toLowerCase()
  }
  return out
}

export function parseCodeIntelPushEvent(raw: unknown): CodeIntelPushEvent | null {
  if (typeof raw !== 'object' || raw === null) {return null}
  const r = normalizeWirePushFrame(raw as Record<string, unknown>)
  const eventName = r.event
  if (typeof eventName !== 'string' || !PUSH_EVENT_NAMES.has(eventName)) {return null}

  switch (eventName) {
    case 'changed':
      return {
        event: 'changed',
        worktreeId: String(r.worktreeId ?? ''),
        reason: CHANGED_REASONS.has(r.reason as string)
          ? (r.reason as 'commit' | 'file_save' | 'branch_switch' | 'reindex' | 'manual')
          : (WIRE_CHANGED_REASON[r.reason as string] ?? 'unknown'),
        resync: r.resync === true || r.reason === 'resync'
      }
    case 'reindexProgress':
      return {
        event: 'reindexProgress',
        worktreeId: String(r.worktreeId ?? ''),
        percent: typeof r.percent === 'number' ? r.percent : null,
        running: r.running === true
      }
    case 'qualityProgress':
      return {
        event: 'qualityProgress',
        worktreeId: String(r.worktreeId ?? ''),
        runId: String(r.runId ?? ''),
        percent: typeof r.percent === 'number' ? r.percent : null,
        phase: typeof r.phase === 'string' ? (r.phase as 'collect' | 'analyze' | 'report') : 'unknown',
        ...(typeof r.stage === 'string' ? { stage: r.stage } : {}),
        ...(typeof r.stepIndex === 'number' ? { stepIndex: r.stepIndex } : {}),
        ...(typeof r.stepCount === 'number' ? { stepCount: r.stepCount } : {}),
        ...(typeof r.message === 'string' ? { message: r.message } : {})
      }
    case 'qualityFinished':
      return {
        event: 'qualityFinished',
        worktreeId: String(r.worktreeId ?? ''),
        runId: String(r.runId ?? ''),
        success: r.success === true,
        error: typeof r.error === 'string' ? r.error : null,
        ...(FINISHED_STATUSES.has(r.status as string)
          ? { status: r.status as 'succeeded' | 'failed' | 'cancelled' | 'interrupted' }
          : {}),
        ...(typeof r.headCommit === 'string' ? { headCommit: r.headCommit } : {})
      }
    case 'gateChanged':
      return {
        event: 'gateChanged',
        worktreeId: String(r.worktreeId ?? ''),
        gate: typeof r.gate === 'string' ? (r.gate as 'pass' | 'warn' | 'fail' | 'unknown') : 'unknown',
        ...(typeof r.previousVerdict === 'string'
          ? { previousVerdict: r.previousVerdict as 'pass' | 'warn' | 'fail' | 'unknown' }
          : r.previousVerdict === null
            ? { previousVerdict: null }
            : {}),
        ...(typeof r.headCommit === 'string' ? { headCommit: r.headCommit } : {}),
        ...(typeof r.profile === 'string' ? { profile: r.profile } : {})
      }
    default:
      return null
  }
}
