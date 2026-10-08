/**
 * code-intel-index-types.ts — CONTRACT-codeintel-ui-api §4.1 common refs, index status, settings
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { WithUnknown } from './code-intel-enum-fallback'

// ---------------------------------------------------------------------------
// §4.1  Common, index, settings
// ---------------------------------------------------------------------------

export type SymbolKind =
  | 'function'
  | 'method'
  | 'type'
  | 'value'
  | 'file'
  | 'folder'
  | 'route'
  | 'component'
  | 'namespace'
  | 'import'
  | 'cluster'
  | 'flow'
  | 'doc'

export type SymbolRef = {
  key: string
  kind: WithUnknown<SymbolKind>
  nativeKind?: string
  name: string
  qualifiedName?: string
  filePath: string
  /** 1-based */
  startLine?: number
  endLine?: number
  language?: string
  gitnexusId?: string
  codegraphId?: string
  signature?: string
  isExported?: boolean
}

export type SourceRef = {
  path: string
  line?: number
  kind: WithUnknown<
    'compose' | 'config' | 'adapter' | 'migration' | 'code' | 'proto' | 'wscompat' | 'route' | 'sql'
  >
}

export type IndexScope = WithUnknown<'exact' | 'repo_root' | 'stale' | 'none'>
export type Freshness = WithUnknown<'fresh' | 'fresh_base' | 'stale'>

export type ToolIndexStatus = {
  tool: 'gitnexus' | 'codegraph'
  available: boolean
  version?: string
  supported: boolean
  state: WithUnknown<'missing' | 'building' | 'ready' | 'stale'>
  indexedCommit?: string
  indexedAt?: string
  headCommit?: string
  mergeBase?: string
  indexScope: IndexScope
  freshness: Freshness
  dirtySinceIndex?: boolean
  changedFilesNotInIndex?: number
  stats?: { files?: number; nodes?: number; edges?: number; communities?: number; processes?: number }
  pendingChanges?: { added: number; modified: number; removed: number } | null
  languages?: string[]
  indicators?: string[]
}

export type IndexPolicy = 'auto_in_place' | 'per_worktree' | 'off'

export type IndexBasis = {
  tool: 'gitnexus' | 'codegraph'
  indexScope: IndexScope
  freshness: Freshness
  indexedCommit?: string
  headCommit?: string
  mergeBase?: string
  indexedAt?: string
  dirtySinceIndex: boolean
  changedFilesNotInIndex: number
  refreshState: WithUnknown<'idle' | 'queued' | 'running' | 'deferred' | 'failed' | 'skipped'>
  trigger?: WithUnknown<'manual' | 'agent_done' | 'head_change'>
  toolVersion?: string
  indexPolicy: WithUnknown<IndexPolicy>
}

export type IndexOverall =
  | 'OFFLINE'
  | 'UNKNOWN'
  | 'NOT_INSTALLED'
  | 'BUILDING'
  | 'MISSING'
  | 'DEGRADED'
  | 'OVERLAY'
  | 'STALE'
  | 'READY'

/** codeIntel.status returns this flat object (no envelope `data`). */
export type IndexStatus = {
  overall: IndexOverall
  tools: ToolIndexStatus[]
  scopeMismatch: boolean
  /** percent is null when unknown */
  activeJob?: { id: string; stage: string; percent: number | null }
  indexBasis: IndexBasis[]
  lastStatusAt?: string
  errorCode?: string
  binding?: {
    id: string
    projectId: string
    repoId: string
    worktreeId?: string
    indexScope: 'exact' | 'repo_root' | 'unresolved'
    version: number
  }
}

export type ReindexMode = 'incremental' | 'full'

export type ReindexJob = {
  jobId: string
  status: WithUnknown<'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'>
  mode: WithUnknown<ReindexMode>
  trigger: WithUnknown<'manual' | 'agent_done' | 'head_change' | 'schedule'>
  stage: string
  percent: number | null
  message: string
  outcome?: string
  errorCode?: string
  startedAt?: string
  finishedAt?: string
}

export type CodeIntelSettings = {
  effective: {
    codeIntelEnabled: boolean
    qualityGateEnabled: boolean
    qualitySecurityScanEnabled: boolean
    aiReviewEnabled: boolean
  }
  tenant: {
    codeIntelEnabled: boolean
    qualityGateEnabled: boolean
    qualitySecurityScanEnabled: boolean
    indexPolicy: IndexPolicy
    aiReviewLevel: 'off' | 'metadata' | 'diff'
    aiReviewModel: string
    agentTurnStorePromptExcerpt: boolean
    agentClaimTextEnabled: boolean
    hotspotWindowDays: number
  }
  updatedBy?: string
  updatedAt?: string
}
