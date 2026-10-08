/**
 * code-intel-types.ts — FE-CV-TASK-050-01
 *
 * Mirror of CONTRACT-codeintel-ui-api §2.2 and §4.1-§4.6, split by domain and re-exported here.
 * Field names are protocol-level: do not rename. Enum values outside the known set fall through
 * to 'unknown' (U4); parsers live in code-intel-parsers.ts.
 *
 * Casing (PQ-32): IndexStatus.overall and risk levels are UPPERCASE; every other enum is lowercase.
 * Quality types (§4.7) live in code-intel-quality-types.ts.
 *
 * @module shared/code-intel-types
 */

export type * from './code-intel-enum-fallback'
export type * from './code-intel-index-types'
export type * from './code-intel-graph-types'
export type * from './code-intel-overlay-types'
export type * from './code-intel-architecture-types'
export type * from './code-intel-findings-types'
export type * from './code-intel-review-state-types'
export type * from './code-intel-push-types'

// ---------------------------------------------------------------------------
// §2.1 / §2.2  Selector and envelope
// ---------------------------------------------------------------------------

/** (sel) in the contract: every view channel takes projectId + worktreeId. */
export type WorktreeSel = {
  projectId: string
  worktreeId: string
}

export type EnvelopeSource = {
  tool: 'gitnexus' | 'codegraph'
  version: string
  indexedAt: string | null
  commit: string | null
  lineBase?: 1
}

export type CodeIntelEnvelope<T> = {
  repo?: string
  worktreeId: string
  view: string
  sources: EnvelopeSource[]
  headCommit: string | null
  stale: boolean
  truncated: boolean
  /** 0 = unknown; only meaningful when > 0 */
  totalCount: number
  etag: string
  fromCache: boolean
  generatedAt: string
  /** Present (true) when ifNoneMatch matched: there is NO data */
  notModified?: true
  nextPageToken?: string
  data?: T
}
