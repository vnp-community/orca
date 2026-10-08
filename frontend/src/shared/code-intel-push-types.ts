/**
 * code-intel-push-types.ts — push events (normalized internal names; see code-intel-parsers)
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

import type { WithUnknown } from './code-intel-enum-fallback'

// ---------------------------------------------------------------------------
// §5  Push events
// ---------------------------------------------------------------------------

export type PushBase = {
  event: string
}

export type PushChanged = PushBase & {
  event: 'changed'
  worktreeId: string
  reason: WithUnknown<'commit' | 'file_save' | 'branch_switch' | 'reindex' | 'manual'>
  resync: boolean
}

export type PushReindexProgress = PushBase & {
  event: 'reindexProgress'
  worktreeId: string
  /** null means indeterminate */
  percent: number | null
  running: boolean
}

export type PushQualityProgress = PushBase & {
  event: 'qualityProgress'
  worktreeId: string
  runId: string
  percent: number | null
  phase: WithUnknown<'collect' | 'analyze' | 'report'>
  /** Contract §5 fields, absent on older frames. */
  stage?: string
  stepIndex?: number
  stepCount?: number
  message?: string
}

export type PushQualityFinished = PushBase & {
  event: 'qualityFinished'
  worktreeId: string
  runId: string
  success: boolean
  error: string | null
  /** Contract §5 terminal status; 'interrupted' is not a QualityRun.status in every backend. */
  status?: 'succeeded' | 'failed' | 'cancelled' | 'interrupted'
  headCommit?: string
}

export type PushGateChanged = PushBase & {
  event: 'gateChanged'
  worktreeId: string
  gate: WithUnknown<'pass' | 'warn' | 'fail' | 'unknown'>
  previousVerdict?: WithUnknown<'pass' | 'warn' | 'fail'> | null
  headCommit?: string
  profile?: string
}

export type CodeIntelPushEvent =
  | PushChanged
  | PushReindexProgress
  | PushQualityProgress
  | PushQualityFinished
  | PushGateChanged
