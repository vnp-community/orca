/**
 * quality-run-scope-model.ts — FE-CV-TASK-087-04
 *
 * Maps the Review scope to the `quality.start` scope/base and limits the choice to what the
 * selected profile can run.
 *
 * @module components/review-map/quality/quality-run-scope-model
 */

import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'
import type { ReviewScope } from '../review-scope-model'

export type QualityRunScopeChoice = 'worktree' | 'changed' | 'commitRange'

export const QUALITY_RUN_SCOPES: readonly QualityRunScopeChoice[] = [
  'changed',
  'commitRange',
  'worktree'
]

export type QualityRunTarget = { scope: QualityRunScopeChoice; base?: string }

/** The scope the Review workspace is looking at, expressed as a quality run target. */
export function toQualityRunTarget(scope: ReviewScope | null | undefined): QualityRunTarget {
  if (!scope) {
    return { scope: 'changed' }
  }
  switch (scope.kind) {
    case 'branch':
      // Why: with uncommitted work included, "changed" is the files the user is reviewing right now.
      return scope.includeUncommitted
        ? { scope: 'changed', base: scope.baseRef }
        : { scope: 'commitRange', base: scope.baseRef }
    case 'range':
      return { scope: 'commitRange', base: scope.baseCommit }
    case 'hostedReview':
      return scope.baseRefName
        ? { scope: 'commitRange', base: scope.baseRefName }
        : { scope: 'changed' }
  }
}

/** An empty `scopes` list means the backend did not restrict the profile. */
export function allowedRunScopes(
  profile: RunnableProfile | null | undefined
): QualityRunScopeChoice[] {
  if (!profile || profile.scopes.length === 0) {
    return [...QUALITY_RUN_SCOPES]
  }
  return QUALITY_RUN_SCOPES.filter((s) => profile.scopes.includes(s))
}

/** Keeps the preferred scope when the profile allows it, otherwise falls to the first allowed one. */
export function resolveRunScope(
  preferred: QualityRunScopeChoice,
  profile: RunnableProfile | null | undefined
): QualityRunScopeChoice | null {
  const allowed = allowedRunScopes(profile)
  return allowed.includes(preferred) ? preferred : (allowed[0] ?? null)
}

/** `base` only matters for ranges; other scopes must not send one. */
export function buildQualityRunRequest(
  profile: string,
  preferred: QualityRunTarget,
  runnable: RunnableProfile | null | undefined
): { profile: string; scope: QualityRunScopeChoice; base?: string } | null {
  const scope = resolveRunScope(preferred.scope, runnable)
  if (!scope) {
    return null
  }
  return {
    profile,
    scope,
    ...(scope === 'commitRange' && preferred.base ? { base: preferred.base } : {})
  }
}
