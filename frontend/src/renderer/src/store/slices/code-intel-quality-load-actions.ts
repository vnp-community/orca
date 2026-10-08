/**
 * code-intel-quality-load-actions.ts — FE-CV-TASK-087-02
 *
 * Read actions of the quality slice (gate, runs, profiles, trend, coverage, per-file findings).
 * Each load keeps the previous data on failure (marked stale) so a flaky network never blanks
 * a verdict, and drops responses that a newer request has superseded.
 *
 * @module store/slices/code-intel-quality-load-actions
 */

import { CODE_INTEL_RPC_METHODS } from '../../../../shared/code-intel-rpc-methods'
import {
  parseQualityCoverageResponse,
  parseQualityFindingsResponse,
  parseQualityGateResponse,
  parseQualityProfileResponse,
  parseQualityRunsResponse,
  parseQualityTrendResponse
} from '../../../../shared/code-intel-quality-wire-parsers'
import {
  isLatestSequence,
  nextSequence,
  patchExistingWorktree,
  patchWorktree,
  readWorktreeState
} from './code-intel-quality-slice-context'
import type { QualitySliceContext } from './code-intel-quality-slice-context'
import { MAX_FINDINGS_FILES_PER_WORKTREE } from './code-intel-quality-state-types'
import type {
  CachedQuality,
  QualityResource,
  QualityTrendGroup,
  QualityWorktreeState
} from './code-intel-quality-state-types'

export type QualityLoadOpts = { force?: boolean }
export type QualityGateLoadOpts = QualityLoadOpts & { base?: string; profileName?: string }

const RUNS_PAGE_LIMIT = 20
const TREND_LIMIT = 50

function fresh<T>(data: T, now: number): CachedQuality<T> {
  return { data, fetchedAt: now, stale: false }
}

/** Evicts the oldest cached file lists beyond the per-worktree cap. */
function capFindingsFiles(
  files: QualityWorktreeState['findingsByFile']
): QualityWorktreeState['findingsByFile'] {
  const entries = Object.entries(files)
  if (entries.length <= MAX_FINDINGS_FILES_PER_WORKTREE) {
    return files
  }
  entries.sort((a, b) => a[1].fetchedAt - b[1].fetchedAt)
  return Object.fromEntries(entries.slice(entries.length - MAX_FINDINGS_FILES_PER_WORKTREE))
}

export function createQualityLoadActions(ctx: QualitySliceContext) {
  async function load<T>(args: {
    worktreeId: string
    resource: QualityResource
    method: string
    params: Record<string, unknown>
    parse: (raw: unknown) => T
    apply: (current: QualityWorktreeState, data: T, now: number) => Partial<QualityWorktreeState>
    force?: boolean
    seqKey?: string
  }): Promise<void> {
    const { worktreeId, resource } = args
    if (readWorktreeState(ctx, worktreeId).loading[resource] && !args.force) {
      return
    }
    const seqKey = `${worktreeId}|${args.seqKey ?? resource}`
    const seq = nextSequence(ctx, seqKey)
    patchWorktree(ctx, worktreeId, (s) => ({
      loading: { ...s.loading, [resource]: true },
      errors: { ...s.errors, [resource]: undefined }
    }))
    const outcome = await ctx.call(worktreeId, args.method, args.params).catch(() => null)
    if (!isLatestSequence(ctx, seqKey, seq)) {
      return
    }
    if (outcome?.ok) {
      const data = args.parse(outcome.result)
      patchExistingWorktree(ctx, worktreeId, (s) => ({
        ...args.apply(s, data, ctx.now()),
        loading: { ...s.loading, [resource]: false }
      }))
      return
    }
    patchExistingWorktree(ctx, worktreeId, (s) => ({
      loading: { ...s.loading, [resource]: false },
      errors: {
        ...s.errors,
        [resource]:
          outcome?.ok === false
            ? outcome.error
            : {
                kind: 'unknown',
                code: null,
                message: 'Unexpected error',
                data: null,
                retryable: false
              }
      },
      // Why: keep showing the last known data, but never as current.
      ...(resource === 'gate' && s.gate ? { gate: { ...s.gate, stale: true } } : {})
    }))
  }

  return {
    loadQualityGate: (worktreeId: string, opts: QualityGateLoadOpts = {}) =>
      load({
        worktreeId,
        resource: 'gate',
        method: CODE_INTEL_RPC_METHODS.QUALITY_GATE,
        params: {
          ...(opts.base ? { base: opts.base } : {}),
          ...(opts.profileName ? { profileName: opts.profileName } : {})
        },
        parse: parseQualityGateResponse,
        force: opts.force,
        apply: (_s, data, now) => ({ gate: fresh(data, now) })
      }),

    loadQualityRuns: (worktreeId: string, opts: QualityLoadOpts = {}) =>
      load({
        worktreeId,
        resource: 'runs',
        method: CODE_INTEL_RPC_METHODS.QUALITY_RUNS,
        params: { limit: RUNS_PAGE_LIMIT },
        parse: parseQualityRunsResponse,
        force: opts.force,
        apply: (_s, data, now) => ({ runs: fresh(data.runs, now) })
      }),

    loadQualityProfiles: (worktreeId: string, opts: QualityLoadOpts & { name?: string } = {}) =>
      load({
        worktreeId,
        resource: 'profiles',
        method: CODE_INTEL_RPC_METHODS.QUALITY_PROFILE_GET,
        params: opts.name ? { name: opts.name } : {},
        parse: parseQualityProfileResponse,
        force: opts.force,
        apply: (_s, data, now) => ({ profiles: fresh(data, now) })
      }),

    loadQualityTrend: (worktreeId: string, group: QualityTrendGroup, opts: QualityLoadOpts = {}) =>
      load({
        worktreeId,
        resource: 'trend',
        seqKey: `trend:${group}`,
        method: CODE_INTEL_RPC_METHODS.QUALITY_TREND,
        params: { limit: TREND_LIMIT, groupBy: group },
        parse: parseQualityTrendResponse,
        force: opts.force,
        apply: (s, data, now) => ({ trend: { ...s.trend, [group]: fresh(data, now) } })
      }),

    loadQualityCoverage: (worktreeId: string, opts: QualityLoadOpts & { runId?: string } = {}) =>
      load({
        worktreeId,
        resource: 'coverage',
        method: CODE_INTEL_RPC_METHODS.QUALITY_COVERAGE,
        params: opts.runId ? { runId: opts.runId } : {},
        parse: parseQualityCoverageResponse,
        force: opts.force,
        apply: (_s, data, now) => ({ coverage: fresh(data, now) })
      }),

    async loadQualityFindingsForFile(
      worktreeId: string,
      file: string,
      opts: QualityLoadOpts & { runId?: string } = {}
    ): Promise<void> {
      const cached = readWorktreeState(ctx, worktreeId).findingsByFile[file]
      if (cached && !cached.stale && !opts.force) {
        return
      }
      const seqKey = `${worktreeId}|file:${file}`
      const seq = nextSequence(ctx, seqKey)
      // Why: an initiating read creates the entry; the response must not recreate a removed one.
      patchWorktree(ctx, worktreeId, () => ({}))
      const outcome = await ctx
        .call(worktreeId, CODE_INTEL_RPC_METHODS.QUALITY_FINDINGS, {
          file,
          limit: 500,
          ...(opts.runId ? { runId: opts.runId } : {})
        })
        .catch(() => null)
      if (!isLatestSequence(ctx, seqKey, seq) || !outcome?.ok) {
        return
      }
      const parsed = parseQualityFindingsResponse(outcome.result)
      patchExistingWorktree(ctx, worktreeId, (s) => ({
        findingsByFile: capFindingsFiles({
          ...s.findingsByFile,
          [file]: fresh(parsed.findings, ctx.now())
        })
      }))
    }
  }
}

export type QualityLoadActions = ReturnType<typeof createQualityLoadActions>
