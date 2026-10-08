/**
 * useQualityFindingsForFile.ts — FE-CV-TASK-087-10
 *
 * Findings of one file for diff annotations, served from the per-worktree cache (at most 16
 * files) and loaded with `quality.findings {file, runId, limit: 500}` only when `enabled`.
 * A finished run bumps `epoch` and clears the cache, which triggers the reload below.
 *
 * @module hooks/useQualityFindingsForFile
 */

import { useEffect } from 'react'
import { useAppStore } from '@/store'
import type { QualityFinding } from '../../../shared/code-intel-quality-types'

export function useQualityFindingsForFile(
  worktreeId: string | null | undefined,
  relativePath: string,
  opts: { enabled: boolean; runId?: string }
): QualityFinding[] | null {
  const { enabled, runId } = opts
  const findings = useAppStore((s) =>
    worktreeId
      ? s.codeIntelQualityByWorktree[worktreeId]?.findingsByFile[relativePath]?.data
      : undefined
  )
  const epoch = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.epoch ?? 0) : 0
  )
  const cached = findings !== undefined

  useEffect(() => {
    if (!enabled || !worktreeId || !relativePath || cached) {
      return
    }
    void useAppStore.getState().loadQualityFindingsForFile(worktreeId, relativePath, { runId })
    // Why: epoch is a dependency so an invalidated cache is fetched again for the new run.
  }, [enabled, worktreeId, relativePath, runId, cached, epoch])

  return enabled && cached ? findings : null
}
