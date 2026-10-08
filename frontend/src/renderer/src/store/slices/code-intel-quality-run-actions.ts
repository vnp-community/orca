/**
 * code-intel-quality-run-actions.ts — FE-CV-TASK-087-02
 *
 * Run lifecycle of the quality slice: start/cancel/attach, push-event handling and the
 * `quality.run` polling fallback used while no push stream is open.
 *
 * @module store/slices/code-intel-quality-run-actions
 */

import { CODE_INTEL_RPC_METHODS } from '../../../../shared/code-intel-rpc-methods'
import {
  parseEnvNotReadyData,
  parseProfileUnknownData,
  parseRetryAfter,
  parseRunInProgressData
} from '../../../../shared/code-intel-quality-errors'
import { parseQualityRun } from '../../../../shared/code-intel-quality-wire-parsers'
import type { QualityRun, QualityRunScope } from '../../../../shared/code-intel-quality-types'
import {
  patchExistingWorktree,
  patchWorktree,
  readWorktreeState
} from './code-intel-quality-slice-context'
import type { QualitySliceContext } from './code-intel-quality-slice-context'
import type { QualityLoadActions } from './code-intel-quality-load-actions'
import type { ActiveQualityRun, QualityRunError } from './code-intel-quality-state-types'

export const QUALITY_RUN_POLL_INTERVAL_MS = 2000

/** Push events the slice reacts to (normalized names from the code-intel event bus). */
export type QualityPushEvent =
  | {
      event: 'qualityProgress'
      worktreeId: string
      runId: string
      percent: number | null
      stage?: string
      stepIndex?: number
      stepCount?: number
      message?: string
    }
  | {
      event: 'qualityFinished'
      worktreeId: string
      runId: string
      success: boolean
      error: string | null
      status?: 'succeeded' | 'failed' | 'cancelled' | 'interrupted'
    }
  | { event: 'gateChanged'; worktreeId: string }
  | { event: 'changed'; worktreeId: string }

export type QualityStartRequest = {
  profile: string
  scope: Exclude<QualityRunScope, 'unknown'>
  base?: string
}

const ACTIVE_PHASES = new Set(['starting', 'queued', 'running', 'cancelling'])

export function isQualityRunActive(run: ActiveQualityRun | null): boolean {
  return run !== null && ACTIVE_PHASES.has(run.phase)
}

function activeFromRun(run: QualityRun, startedAt: number): ActiveQualityRun {
  return {
    runId: run.id,
    profile: run.profile,
    scope: run.scope,
    phase: run.status === 'queued' ? 'queued' : 'running',
    stage: '',
    percent: null,
    message: '',
    startedAt
  }
}

function startError(error: {
  kind: QualityRunError['kind']
  message: string
  data: Record<string, unknown> | null
}): QualityRunError {
  const out: QualityRunError = { kind: error.kind, message: error.message }
  const env = parseEnvNotReadyData(error.data)
  if (error.kind === 'env-not-ready' && env) {
    out.missing = env.missing
  }
  const unknownProfile = parseProfileUnknownData(error.data)
  if (error.kind === 'profile-unknown' && unknownProfile) {
    out.available = unknownProfile.available
  }
  const retry = parseRetryAfter(error.data)
  if (retry !== undefined) {
    out.retryAfterSeconds = retry
  }
  return out
}

export function createQualityRunActions(ctx: QualitySliceContext, loaders: QualityLoadActions) {
  function stopPolling(worktreeId: string): void {
    const timer = ctx.timers.get(worktreeId)
    if (timer) {
      clearTimeout(timer)
      ctx.timers.delete(worktreeId)
    }
  }

  function reloadAfterRun(worktreeId: string): void {
    const s = readWorktreeState(ctx, worktreeId)
    patchWorktree(ctx, worktreeId, (cur) => ({
      epoch: cur.epoch + 1,
      findingsByFile: {},
      ...(cur.gate ? { gate: { ...cur.gate, stale: true } } : {}),
      ...(cur.coverage ? { coverage: { ...cur.coverage, stale: true } } : {})
    }))
    if (s.gate) {
      void loaders.loadQualityGate(worktreeId, { force: true })
    }
    if (s.runs) {
      void loaders.loadQualityRuns(worktreeId, { force: true })
    }
  }

  function finish(worktreeId: string, status: ActiveQualityRun['status'], message?: string): void {
    stopPolling(worktreeId)
    patchWorktree(ctx, worktreeId, (s) =>
      s.run
        ? { run: { ...s.run, phase: 'finished', status, message: message ?? s.run.message } }
        : {}
    )
    reloadAfterRun(worktreeId)
  }

  /** Reads `quality.run` once; used by the polling loop and after a stream resync. */
  async function refreshQualityRun(worktreeId: string): Promise<void> {
    const run = readWorktreeState(ctx, worktreeId).run
    if (!run?.runId || !isQualityRunActive(run)) {
      return
    }
    const outcome = await ctx
      .call(worktreeId, CODE_INTEL_RPC_METHODS.QUALITY_RUN, { runId: run.runId })
      .catch(() => null)
    const current = readWorktreeState(ctx, worktreeId).run
    if (!outcome?.ok || current?.runId !== run.runId) {
      return
    }
    const fetched = parseQualityRun(outcome.result)
    if (fetched.status === 'queued' || fetched.status === 'running') {
      patchWorktree(ctx, worktreeId, (s) =>
        s.run && s.run.phase !== 'cancelling'
          ? { run: { ...s.run, phase: fetched.status === 'queued' ? 'queued' : 'running' } }
          : {}
      )
      return
    }
    finish(worktreeId, fetched.status === 'unknown' ? 'failed' : fetched.status)
  }

  function ensurePolling(worktreeId: string): void {
    stopPolling(worktreeId)
    const tick = (): void => {
      const run = readWorktreeState(ctx, worktreeId).run
      // Why: while the push stream is open the finished event ends the run; polling is only the fallback.
      if (!isQualityRunActive(run)) {
        ctx.timers.delete(worktreeId)
        return
      }
      const streaming = ctx.get().codeIntelEventsState === 'streaming'
      const next = (): void => {
        ctx.timers.set(worktreeId, setTimeout(tick, QUALITY_RUN_POLL_INTERVAL_MS))
      }
      if (streaming) {
        next()
        return
      }
      void refreshQualityRun(worktreeId).finally(next)
    }
    ctx.timers.set(worktreeId, setTimeout(tick, QUALITY_RUN_POLL_INTERVAL_MS))
  }

  function attachQualityRun(worktreeId: string, run: QualityRun): void {
    patchWorktree(ctx, worktreeId, () => ({ run: activeFromRun(run, ctx.now()), runError: null }))
    ensurePolling(worktreeId)
  }

  return {
    refreshQualityRun,

    attachQualityRun,

    /** Re-attaches to a run already in progress (reopened tab, run started elsewhere). */
    attachFromLoadedRuns(worktreeId: string): void {
      const s = readWorktreeState(ctx, worktreeId)
      if (isQualityRunActive(s.run)) {
        return
      }
      const live = s.runs?.data.find(
        (r) => r.source !== 'ci' && (r.status === 'running' || r.status === 'queued')
      )
      if (live) {
        attachQualityRun(worktreeId, live)
      }
    },

    async startQualityRun(worktreeId: string, request: QualityStartRequest): Promise<void> {
      // Why: lock synchronously so a double click (or a slow SSH round trip) cannot start twice.
      if (isQualityRunActive(readWorktreeState(ctx, worktreeId).run)) {
        return
      }
      patchWorktree(ctx, worktreeId, () => ({
        runError: null,
        run: {
          runId: null,
          profile: request.profile,
          scope: request.scope,
          phase: 'starting',
          stage: '',
          percent: null,
          message: '',
          startedAt: ctx.now()
        }
      }))
      const outcome = await ctx
        .call(worktreeId, CODE_INTEL_RPC_METHODS.QUALITY_START, {
          profile: request.profile,
          scope: request.scope,
          ...(request.base ? { base: request.base } : {})
        })
        .catch(() => null)
      if (outcome?.ok) {
        const started = parseQualityRun((outcome.result as { run?: unknown } | null)?.run)
        patchExistingWorktree(ctx, worktreeId, (s) => {
          // A push may already have attached the run id while the response was in flight.
          if (s.run?.runId) {
            return {}
          }
          return {
            run: {
              ...(s.run as ActiveQualityRun),
              runId: started.id || null,
              phase: started.status === 'queued' ? 'queued' : 'running'
            }
          }
        })
        ensurePolling(worktreeId)
        return
      }
      const error = outcome?.ok === false ? outcome.error : null
      const inProgress =
        error?.kind === 'run-in-progress' ? parseRunInProgressData(error.data) : undefined
      if (inProgress) {
        // Another client already runs it: follow that run instead of reporting an error.
        patchExistingWorktree(ctx, worktreeId, (s) => ({
          runError: null,
          run: { ...(s.run as ActiveQualityRun), runId: inProgress.runId, phase: 'running' }
        }))
        ensurePolling(worktreeId)
        return
      }
      patchExistingWorktree(ctx, worktreeId, (s) => ({
        run: null,
        runError: error ? startError(error) : { kind: 'unknown', message: 'Unexpected error' },
        // Why: an unknown profile means the cached list is outdated; drop the pick and refresh it.
        ...(error?.kind === 'profile-unknown' ? { ui: { ...s.ui, profile: null } } : {})
      }))
      if (error?.kind === 'profile-unknown') {
        void loaders.loadQualityProfiles(worktreeId, { force: true })
      }
    },

    async cancelQualityRun(worktreeId: string): Promise<void> {
      const run = readWorktreeState(ctx, worktreeId).run
      if (!run?.runId || run.phase === 'cancelling' || run.phase === 'finished') {
        return
      }
      // Why: "cancelling" is a request, not a fact; only quality.finished(cancelled) confirms it.
      patchWorktree(ctx, worktreeId, (s) => ({
        run: s.run ? { ...s.run, phase: 'cancelling' } : null
      }))
      const outcome = await ctx
        .call(worktreeId, CODE_INTEL_RPC_METHODS.QUALITY_CANCEL, { runId: run.runId })
        .catch(() => null)
      if (outcome?.ok === false) {
        patchExistingWorktree(ctx, worktreeId, (s) => ({
          run: s.run && s.run.phase === 'cancelling' ? { ...s.run, phase: 'running' } : s.run,
          runError: startError(outcome.error)
        }))
      }
    },

    dismissQualityRun(worktreeId: string): void {
      patchWorktree(ctx, worktreeId, (s) =>
        s.run?.phase === 'finished' || s.runError
          ? { run: s.run?.phase === 'finished' ? null : s.run, runError: null }
          : {}
      )
    },

    applyQualityPushEvent(event: QualityPushEvent): void {
      const { worktreeId } = event
      const current = ctx.get().codeIntelQualityByWorktree[worktreeId]
      // Why: only worktrees someone is looking at hold state; events for others are ignored.
      if (!current) {
        return
      }
      if (event.event === 'changed') {
        patchWorktree(ctx, worktreeId, (s) => (s.gate ? { gate: { ...s.gate, stale: true } } : {}))
        if (current.gate) {
          void loaders.loadQualityGate(worktreeId, { force: true })
        }
        return
      }
      if (event.event === 'gateChanged') {
        patchWorktree(ctx, worktreeId, (s) => ({ epoch: s.epoch + 1 }))
        if (current.gate) {
          void loaders.loadQualityGate(worktreeId, { force: true })
        }
        return
      }
      const run = current.run
      const ours = run === null || run.runId === null || run.runId === event.runId
      if (event.event === 'qualityProgress') {
        if (!ours && isQualityRunActive(run)) {
          return
        }
        patchWorktree(ctx, worktreeId, (s) => {
          const base: ActiveQualityRun = isQualityRunActive(s.run)
            ? (s.run as ActiveQualityRun)
            : {
                runId: event.runId,
                profile: '',
                scope: 'unknown',
                phase: 'running',
                stage: '',
                percent: null,
                message: '',
                startedAt: ctx.now()
              }
          return {
            run: {
              ...base,
              runId: event.runId,
              phase: base.phase === 'cancelling' ? 'cancelling' : 'running',
              percent: event.percent,
              stage: event.stage ?? base.stage,
              message: event.message ?? base.message,
              stepIndex: event.stepIndex ?? base.stepIndex,
              stepCount: event.stepCount ?? base.stepCount
            }
          }
        })
        if (!run || !isQualityRunActive(run)) {
          ensurePolling(worktreeId)
        }
        return
      }
      // qualityFinished
      if (!ours && isQualityRunActive(run)) {
        return
      }
      const status = event.status ?? (event.success ? 'succeeded' : 'failed')
      if (!run || run.phase === 'finished') {
        patchWorktree(ctx, worktreeId, () => ({
          run: {
            runId: event.runId,
            profile: '',
            scope: 'unknown',
            phase: 'finished',
            status,
            stage: '',
            percent: null,
            message: event.error ?? '',
            startedAt: ctx.now()
          }
        }))
        reloadAfterRun(worktreeId)
        return
      }
      finish(worktreeId, status, event.error ?? undefined)
    },

    /** Cancels timers of worktrees that no longer exist. */
    stopPollingFor(worktreeId: string): void {
      stopPolling(worktreeId)
    }
  }
}

export type QualityRunActions = ReturnType<typeof createQualityRunActions>
