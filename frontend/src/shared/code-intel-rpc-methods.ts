/**
 * code-intel-rpc-methods.ts — FE-CV-TASK-050-02
 *
 * RPC method name constants, stream methods, push event names,
 * method limits (byte caps per UI-API §2.4) and contract types.
 *
 * @module shared/code-intel-rpc-methods
 */

// ---------------------------------------------------------------------------
// §3 RPC Methods — all 46
// ---------------------------------------------------------------------------

export const CODE_INTEL_RPC_METHODS = {
  // Status & index
  STATUS: 'codeIntel.status',
  REINDEX: 'codeIntel.reindex',
  REINDEX_STATUS: 'codeIntel.reindexStatus',
  BIND_REPO: 'codeIntel.bindRepo',
  SUBSCRIBE: 'codeIntel.subscribe',

  // Settings
  SETTINGS_GET: 'codeIntel.settings.get',
  SETTINGS_SAVE: 'codeIntel.settings.save',

  // Review state
  REVIEW_STATE_GET: 'codeIntel.reviewState.get',
  REVIEW_STATE_SAVE: 'codeIntel.reviewState.save',
  REVIEW_STATE_APPROVE: 'codeIntel.reviewState.approve',
  REVIEW_STATE_RESET: 'codeIntel.reviewState.reset',

  // Review comments
  REVIEW_COMMENT_ADD: 'codeIntel.reviewComment.add',
  REVIEW_COMMENT_RESOLVE: 'codeIntel.reviewComment.resolve',
  REVIEW_COMMENT_DELETE: 'codeIntel.reviewComment.delete',

  // Review checklist
  REVIEW_CHECKLIST_SET: 'codeIntel.reviewChecklist.set',

  // Overlay
  OVERLAY_GET: 'codeIntel.overlay.get',

  // Impact
  IMPACT_QUERY: 'codeIntel.impact.query',
  IMPACT_GRAPH: 'codeIntel.impact.graph',

  // Symbol
  SYMBOL_SEARCH: 'codeIntel.symbol.search',
  SYMBOL_HOVER: 'codeIntel.symbol.hover',
  SYMBOL_REFERENCES: 'codeIntel.symbol.references',
  SYMBOL_DEFINITION: 'codeIntel.symbol.definition',

  // Findings
  FINDING_LIST: 'codeIntel.finding.list',
  FINDING_WAIVE: 'codeIntel.finding.waive',
  FINDING_REVOKE: 'codeIntel.finding.revoke',
  DISMISS_FINDING: 'codeIntel.dismissFinding',

  // Quality
  QUALITY_PROFILE_GET: 'codeIntel.quality.profile.get',
  QUALITY_PROFILE_SAVE: 'codeIntel.quality.profile.save',
  QUALITY_GATE_GET: 'codeIntel.quality.gate.get',
  QUALITY_RUN_START: 'codeIntel.quality.run.start',
  QUALITY_RUN_STATUS: 'codeIntel.quality.run.status',
  QUALITY_RUN_CANCEL: 'codeIntel.quality.run.cancel',
  QUALITY_TRACE_LIST: 'codeIntel.quality.trace.list',
  QUALITY_TRACE_CONFIRM: 'codeIntel.quality.trace.confirm',
  QUALITY_TRACE_LINK: 'codeIntel.quality.trace.link',
  QUALITY_COVERAGE_GET: 'codeIntel.quality.coverage.get',
  QUALITY_TREND_LIST: 'codeIntel.quality.trend.list',
  QUALITY_HOTSPOT_LIST: 'codeIntel.quality.hotspot.list',
  QUALITY_DEPENDENCY_LIST: 'codeIntel.quality.dependency.list',

  // C4
  C4_GET: 'codeIntel.c4.get',
  C4_SAVE: 'codeIntel.c4.save',

  // Contract
  CONTRACT_DIFF_GET: 'codeIntel.contract.diff.get',
  CONTRACT_DIFF_APPROVE: 'codeIntel.contract.diff.approve',

  // Security
  SECURITY_SCAN_START: 'codeIntel.security.scan.start',
  SECURITY_SCAN_STATUS: 'codeIntel.security.scan.status',
  SECURITY_SCAN_CANCEL: 'codeIntel.security.scan.cancel',
} as const

export type CodeIntelMethod = (typeof CODE_INTEL_RPC_METHODS)[keyof typeof CODE_INTEL_RPC_METHODS]

// ---------------------------------------------------------------------------
// Stream methods
// ---------------------------------------------------------------------------

export const CODE_INTEL_STREAM_METHODS = ['codeIntel.subscribe'] as const
export type CodeIntelStreamMethod = (typeof CODE_INTEL_STREAM_METHODS)[number]

// ---------------------------------------------------------------------------
// §5 Push event names
// ---------------------------------------------------------------------------

export const CODE_INTEL_PUSH_EVENTS = [
  'changed',
  'reindexProgress',
  'qualityProgress',
  'qualityFinished',
  'gateChanged'
] as const

export type CodeIntelPushEventName = (typeof CODE_INTEL_PUSH_EVENTS)[number]

// ---------------------------------------------------------------------------
// §2.4 Per-method argument size limits (bytes, UTF-8 JSON)
// ---------------------------------------------------------------------------

/** 16 KiB default */
const DEFAULT_MAX_ARGS_BYTES = 16 * 1024

export const CODE_INTEL_METHOD_LIMITS: Record<string, { maxArgsBytes: number }> = {
  [CODE_INTEL_RPC_METHODS.REVIEW_STATE_SAVE]: { maxArgsBytes: 256 * 1024 },
  [CODE_INTEL_RPC_METHODS.C4_SAVE]: { maxArgsBytes: 96 * 1024 },
  [CODE_INTEL_RPC_METHODS.QUALITY_PROFILE_SAVE]: { maxArgsBytes: 96 * 1024 },
  [CODE_INTEL_RPC_METHODS.QUALITY_TRACE_CONFIRM]: { maxArgsBytes: 8 * 1024 },
  [CODE_INTEL_RPC_METHODS.QUALITY_TRACE_LINK]: { maxArgsBytes: 8 * 1024 },
}

export function getMethodMaxArgsBytes(method: string): number {
  return CODE_INTEL_METHOD_LIMITS[method]?.maxArgsBytes ?? DEFAULT_MAX_ARGS_BYTES
}

// ---------------------------------------------------------------------------
// §2.3 Contract type stubs (review channels; quality remains unknown until 085/087)
// ---------------------------------------------------------------------------

export type CodeIntelRpcContract = {
  [M in CodeIntelMethod]: {
    params: unknown
    result: unknown
  }
}
