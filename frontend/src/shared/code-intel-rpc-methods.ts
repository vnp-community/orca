/**
 * code-intel-rpc-methods.ts — FE-CV-TASK-050-02
 *
 * RPC method name constants, stream methods, push event names,
 * method limits (byte caps per UI-API §2.4) and contract types.
 *
 * @module shared/code-intel-rpc-methods
 */

// ---------------------------------------------------------------------------
// §3 RPC Methods — all 46 (26 + 20), names verbatim from CONTRACT-codeintel-ui-api
// ---------------------------------------------------------------------------

export const CODE_INTEL_RPC_METHODS = {
  // CONTRACT-codeintel-ui-api §3.1 (26 channels, incl. the single stream)
  STATUS: 'codeIntel.status',
  REINDEX: 'codeIntel.reindex',
  REINDEX_STATUS: 'codeIntel.reindexStatus',
  STRUCTURE: 'codeIntel.structure',
  ARCHITECTURE: 'codeIntel.architecture',
  DATA_FLOWS: 'codeIntel.dataFlows',
  DATA_FLOW: 'codeIntel.dataFlow',
  ERD: 'codeIntel.erd',
  STORAGE: 'codeIntel.storage',
  SUBGRAPH: 'codeIntel.subgraph',
  IMPACT: 'codeIntel.impact',
  SYMBOL: 'codeIntel.symbol',
  ROUTES: 'codeIntel.routes',
  CHANGE_OVERLAY: 'codeIntel.changeOverlay',
  READING_ORDER: 'codeIntel.readingOrder',
  FINDINGS: 'codeIntel.findings',
  DISMISS_FINDING: 'codeIntel.dismissFinding',
  CONTRACT_DIFF: 'codeIntel.contractDiff',
  REVIEW_STATE_GET: 'codeIntel.reviewState.get',
  REVIEW_STATE_SAVE: 'codeIntel.reviewState.save',
  C4_GET: 'codeIntel.c4.get',
  C4_SAVE: 'codeIntel.c4.save',
  BIND_REPO: 'codeIntel.bindRepo',
  SETTINGS_GET: 'codeIntel.settings.get',
  SETTINGS_SET: 'codeIntel.settings.set',
  SUBSCRIBE: 'codeIntel.subscribe',

  // CONTRACT-codeintel-ui-api §3.2 (20 channels)
  QUALITY_START: 'codeIntel.quality.start',
  QUALITY_CANCEL: 'codeIntel.quality.cancel',
  QUALITY_RUN: 'codeIntel.quality.run',
  QUALITY_RUNS: 'codeIntel.quality.runs',
  QUALITY_FINDINGS: 'codeIntel.quality.findings',
  QUALITY_WAIVE: 'codeIntel.quality.waive',
  QUALITY_GATE: 'codeIntel.quality.gate',
  QUALITY_PROFILE_GET: 'codeIntel.quality.profile.get',
  QUALITY_PROFILE_SAVE: 'codeIntel.quality.profile.save',
  QUALITY_TREND: 'codeIntel.quality.trend',
  QUALITY_COVERAGE: 'codeIntel.quality.coverage',
  QUALITY_TRACE: 'codeIntel.quality.trace',
  QUALITY_TRACE_CONFIRM: 'codeIntel.quality.trace.confirm',
  QUALITY_TRACE_LINK: 'codeIntel.quality.trace.link',
  QUALITY_SUMMARY: 'codeIntel.quality.summary',
  QUALITY_REPORT: 'codeIntel.quality.report',
  QUALITY_CI: 'codeIntel.quality.ci',
  QUALITY_TURN_RECORD: 'codeIntel.quality.turn.record',
  QUALITY_TURNS: 'codeIntel.quality.turns',
  QUALITY_TURN: 'codeIntel.quality.turn'
} as const

/** Channels whose result is wrapped in the §2.2 envelope (`Env<...>` in the contract). */
export const CODE_INTEL_ENVELOPE_METHODS: ReadonlySet<string> = new Set([
  CODE_INTEL_RPC_METHODS.STRUCTURE,
  CODE_INTEL_RPC_METHODS.ARCHITECTURE,
  CODE_INTEL_RPC_METHODS.DATA_FLOWS,
  CODE_INTEL_RPC_METHODS.DATA_FLOW,
  CODE_INTEL_RPC_METHODS.ERD,
  CODE_INTEL_RPC_METHODS.STORAGE,
  CODE_INTEL_RPC_METHODS.SUBGRAPH,
  CODE_INTEL_RPC_METHODS.IMPACT,
  CODE_INTEL_RPC_METHODS.SYMBOL,
  CODE_INTEL_RPC_METHODS.ROUTES,
  CODE_INTEL_RPC_METHODS.CHANGE_OVERLAY,
  CODE_INTEL_RPC_METHODS.READING_ORDER,
  CODE_INTEL_RPC_METHODS.FINDINGS,
  CODE_INTEL_RPC_METHODS.CONTRACT_DIFF
])

/** Accepts `quality.trace` or `codeIntel.quality.trace`; returns the full channel name. */
export function toCodeIntelMethod(method: string): string {
  return method.startsWith('codeIntel.') ? method : `codeIntel.${method}`
}

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

export type CodeIntelRpcContract = Record<CodeIntelMethod, {
    params: unknown
    result: unknown
  }>
