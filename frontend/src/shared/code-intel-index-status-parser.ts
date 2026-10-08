/**
 * code-intel-index-status-parser.ts — FE-CV-TASK-050-03
 *
 * Safe parser for the flat `codeIntel.status` object (CONTRACT-codeintel-ui-api §4.1).
 * Re-exported from code-intel-parsers.ts.
 *
 * @module shared/code-intel-index-status-parser
 */

import type { IndexStatus, IndexOverall, IndexBasis, ToolIndexStatus } from './code-intel-types'

// ---------------------------------------------------------------------------
// §4.1 IndexStatus parser
// ---------------------------------------------------------------------------

const VALID_OVERALL: ReadonlySet<string> = new Set<IndexOverall>([
  'OFFLINE',
  'UNKNOWN',
  'NOT_INSTALLED',
  'BUILDING',
  'MISSING',
  'DEGRADED',
  'OVERLAY',
  'STALE',
  'READY'
])

const SCOPES: ReadonlySet<string> = new Set(['exact', 'repo_root', 'stale', 'none'])
const FRESHNESS: ReadonlySet<string> = new Set(['fresh', 'fresh_base', 'stale'])
const TOOL_STATES: ReadonlySet<string> = new Set(['missing', 'building', 'ready', 'stale'])
const REFRESH_STATES: ReadonlySet<string> = new Set([
  'idle',
  'queued',
  'running',
  'deferred',
  'failed',
  'skipped'
])
const INDEX_POLICIES: ReadonlySet<string> = new Set(['auto_in_place', 'per_worktree', 'off'])
const TRIGGERS: ReadonlySet<string> = new Set(['manual', 'agent_done', 'head_change'])

function oneOf<T extends string>(value: unknown, allowed: ReadonlySet<string>): T | 'unknown' {
  return typeof value === 'string' && allowed.has(value) ? (value as T) : 'unknown'
}

function optStr(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : {}
}

function parseToolStatus(raw: unknown): ToolIndexStatus | null {
  const r = asRecord(raw)
  if (r.tool !== 'gitnexus' && r.tool !== 'codegraph') {
    return null
  }
  return {
    tool: r.tool,
    available: r.available === true,
    ...(optStr(r.version) !== undefined ? { version: optStr(r.version) } : {}),
    supported: r.supported === true,
    state: oneOf(r.state, TOOL_STATES),
    ...(optStr(r.indexedCommit) !== undefined ? { indexedCommit: optStr(r.indexedCommit) } : {}),
    ...(optStr(r.indexedAt) !== undefined ? { indexedAt: optStr(r.indexedAt) } : {}),
    ...(optStr(r.headCommit) !== undefined ? { headCommit: optStr(r.headCommit) } : {}),
    ...(optStr(r.mergeBase) !== undefined ? { mergeBase: optStr(r.mergeBase) } : {}),
    indexScope: oneOf(r.indexScope, SCOPES),
    freshness: oneOf(r.freshness, FRESHNESS),
    ...(typeof r.dirtySinceIndex === 'boolean' ? { dirtySinceIndex: r.dirtySinceIndex } : {}),
    ...(typeof r.changedFilesNotInIndex === 'number'
      ? { changedFilesNotInIndex: r.changedFilesNotInIndex }
      : {}),
    ...(typeof r.stats === 'object' && r.stats !== null
      ? { stats: r.stats as ToolIndexStatus['stats'] }
      : {}),
    ...(r.pendingChanges === null || typeof r.pendingChanges === 'object'
      ? { pendingChanges: r.pendingChanges as ToolIndexStatus['pendingChanges'] }
      : {}),
    ...(Array.isArray(r.languages) ? { languages: r.languages.filter((l) => typeof l === 'string') } : {}),
    ...(Array.isArray(r.indicators)
      ? { indicators: r.indicators.filter((l) => typeof l === 'string') }
      : {})
  }
}

export function parseIndexBasis(raw: unknown): IndexBasis | null {
  const r = asRecord(raw)
  if (r.tool !== 'gitnexus' && r.tool !== 'codegraph') {
    return null
  }
  return {
    tool: r.tool,
    indexScope: oneOf(r.indexScope, SCOPES),
    freshness: oneOf(r.freshness, FRESHNESS),
    ...(optStr(r.indexedCommit) !== undefined ? { indexedCommit: optStr(r.indexedCommit) } : {}),
    ...(optStr(r.headCommit) !== undefined ? { headCommit: optStr(r.headCommit) } : {}),
    ...(optStr(r.mergeBase) !== undefined ? { mergeBase: optStr(r.mergeBase) } : {}),
    ...(optStr(r.indexedAt) !== undefined ? { indexedAt: optStr(r.indexedAt) } : {}),
    dirtySinceIndex: r.dirtySinceIndex === true,
    changedFilesNotInIndex:
      typeof r.changedFilesNotInIndex === 'number' ? r.changedFilesNotInIndex : 0,
    refreshState: oneOf(r.refreshState, REFRESH_STATES),
    ...(typeof r.trigger === 'string' ? { trigger: oneOf(r.trigger, TRIGGERS) } : {}),
    ...(optStr(r.toolVersion) !== undefined ? { toolVersion: optStr(r.toolVersion) } : {}),
    indexPolicy: oneOf(r.indexPolicy, INDEX_POLICIES)
  }
}

/** Parse codeIntel.status (flat object). Missing/garbled input yields overall 'UNKNOWN'. */
export function parseIndexStatus(raw: unknown): IndexStatus {
  const r = asRecord(raw)
  const rawOverall = typeof r.overall === 'string' ? r.overall.toUpperCase() : ''
  const job = asRecord(r.activeJob)
  const binding = asRecord(r.binding)
  const bindingScope = binding.indexScope

  return {
    overall: VALID_OVERALL.has(rawOverall) ? (rawOverall as IndexOverall) : 'UNKNOWN',
    tools: Array.isArray(r.tools)
      ? r.tools.map(parseToolStatus).filter((t): t is ToolIndexStatus => t !== null)
      : [],
    scopeMismatch: r.scopeMismatch === true,
    ...(typeof job.id === 'string'
      ? {
          activeJob: {
            id: job.id,
            stage: typeof job.stage === 'string' ? job.stage : '',
            percent: typeof job.percent === 'number' ? job.percent : null
          }
        }
      : {}),
    indexBasis: Array.isArray(r.indexBasis)
      ? r.indexBasis.map(parseIndexBasis).filter((b): b is IndexBasis => b !== null)
      : [],
    ...(optStr(r.lastStatusAt) !== undefined ? { lastStatusAt: optStr(r.lastStatusAt) } : {}),
    ...(optStr(r.errorCode) !== undefined ? { errorCode: optStr(r.errorCode) } : {}),
    ...(typeof binding.id === 'string'
      ? {
          binding: {
            id: binding.id,
            projectId: typeof binding.projectId === 'string' ? binding.projectId : '',
            repoId: typeof binding.repoId === 'string' ? binding.repoId : '',
            ...(typeof binding.worktreeId === 'string' ? { worktreeId: binding.worktreeId } : {}),
            indexScope:
              bindingScope === 'exact' || bindingScope === 'repo_root' ? bindingScope : 'unresolved',
            version: typeof binding.version === 'number' ? binding.version : 0
          }
        }
      : {})
  }
}
