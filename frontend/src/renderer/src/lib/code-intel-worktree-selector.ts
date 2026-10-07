/**
 * code-intel-worktree-selector.ts — FE-CV-TASK-050-09
 *
 * Resolves the code-intel selector (projectId + worktreeId + environmentId)
 * from app state. Used by all code-intel hooks/client to determine whether
 * a call can proceed and which transport path to use.
 *
 * Never throws — unsupported states return a tagged union variant with `reason`.
 *
 * @module lib/code-intel-worktree-selector
 */

import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from './worktree-runtime-owner'
import type { AppState } from '@/store/types'

// ---------------------------------------------------------------------------
// Return type
// ---------------------------------------------------------------------------

export type CodeIntelSelector =
  | {
      state: 'ready'
      worktreeId: string
      projectId: string
      environmentId: string | null
    }
  | {
      state: 'unsupported'
      reason: 'workspace-scope' | 'no-project' | 'no-environment' | 'unknown-worktree'
    }

// ---------------------------------------------------------------------------
// Pure resolver
// ---------------------------------------------------------------------------

/**
 * Resolve the code-intel selector from state.
 * Called on every render — must not create new objects unnecessarily.
 *
 * PQ-04: worktreeId must be `<repoId>::<path>`; workspace/folder roots are rejected.
 * O-1: projectId is required on the worktree (or its repo); absence → no-project.
 */
export function resolveCodeIntelSelector(
  state: Pick<AppState, 'worktrees' | 'repos' | 'preflightStatus'>,
  rawWorktreeId: string
): CodeIntelSelector {
  // Strip legacy 'id:' prefix if present
  const worktreeId = rawWorktreeId.startsWith('id:')
    ? rawWorktreeId.slice(3)
    : rawWorktreeId

  // Floating terminal / folder workspace roots are not addressable
  if (
    worktreeId === 'workspace' ||
    worktreeId.startsWith('::workspace:') ||
    worktreeId.startsWith('folder:')
  ) {
    return { state: 'unsupported', reason: 'workspace-scope' }
  }

  // Find the worktree
  const worktree = state.worktrees.find((wt) => wt.id === worktreeId)
  if (!worktree) {
    return { state: 'unsupported', reason: 'unknown-worktree' }
  }

  // Resolve projectId: prefer worktree-level, fall back to repo-level
  const repo = state.repos.find((r) => r.id === worktree.repoId)
  const projectId = worktree.projectId ?? repo?.projectId ?? null

  if (!projectId) {
    return { state: 'unsupported', reason: 'no-project' }
  }

  // Resolve environment id (null means local execution)
  const environmentId = getRuntimeEnvironmentIdForWorktree(state as AppState, worktreeId)

  return {
    state: 'ready',
    worktreeId,
    projectId,
    environmentId
  }
}

// ---------------------------------------------------------------------------
// Stable hook (avoids new object per render when selector is unsupported)
// ---------------------------------------------------------------------------

// Stable sentinel objects for unsupported states — returned by reference so
// selector identity is preserved across renders.
const SENTINELS: Record<string, CodeIntelSelector> = {
  'workspace-scope': { state: 'unsupported', reason: 'workspace-scope' },
  'no-project': { state: 'unsupported', reason: 'no-project' },
  'no-environment': { state: 'unsupported', reason: 'no-environment' },
  'unknown-worktree': { state: 'unsupported', reason: 'unknown-worktree' },
}

/**
 * Hook: returns a stable CodeIntelSelector for the given worktree.
 * The returned object reference is stable for unsupported states.
 */
export function useCodeIntelSelector(worktreeId: string): CodeIntelSelector {
  return useAppStore((state) => {
    const result = resolveCodeIntelSelector(state, worktreeId)
    if (result.state === 'unsupported') {
      // Return sentinel to avoid new object on every render
      return SENTINELS[result.reason] ?? result
    }
    return result
  })
}
