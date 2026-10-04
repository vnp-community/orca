import { useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

export type MergeStrategy = 'merge' | 'squash' | 'rebase'

export type TaskMergeOutcome =
  | { ok: true; resultSha: string }
  | { ok: false; error: string }
  | { ok: 'conflict'; paths: string[] }

type MergeResultView = {
  resultSha?: string
  hasConflicts: boolean
  conflictedPaths?: string[]
  conflictDispatchKey?: string
}

// BL-TG-06 §D: merge dispatches into the repo's OWN base checkout
// (MergeWorktreeIntoBase's own doc comment — a deliberate, correct
// difference from BUG-028's direct_agent bug: a merge target must be the
// shared base checkout, that's the point of merging). Multiple merges
// running concurrently would all race on git checkout/merge against that
// SAME shared directory — unlike Gen Spec/Code's bounded-but-parallel
// dispatch (each task's own isolated worktree), merge must run strictly
// one at a time, and stop at the first failure/conflict rather than keep
// merging into a checkout that may already be mid-conflict.
export function useTaskBatchMerge() {
  const [running, setRunning] = useState(false)

  const batchMergeWorktrees = async (
    tasks: OrcaTask[],
    baseBranch: string,
    strategy: MergeStrategy
  ): Promise<Map<string, TaskMergeOutcome>> => {
    setRunning(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const outcomes = new Map<string, TaskMergeOutcome>()

    for (const task of tasks) {
      if (!task.worktreeId) {
        outcomes.set(task.id, { ok: false, error: 'Task has no worktreeId — nothing to merge' })
        break
      }
      try {
        const resp = await callRuntimeRpc<MergeResultView>(target, 'worktree.merge', {
          worktreeId: task.worktreeId,
          baseBranch,
          strategy,
          commitMessage: `merge(task-${task.taskNumber ?? 0}): ${task.title}`,
          // BR-WT-18 — dọn worktree ngay sau merge thành công, đã hỗ trợ sẵn ở backend.
          cleanupWorktreeIds: [task.worktreeId]
        })
        if (resp.hasConflicts) {
          outcomes.set(task.id, { ok: 'conflict', paths: resp.conflictedPaths ?? [] })
          // Dừng batch — xem doc comment ở trên: base checkout đang ở
          // trạng thái conflict dở dang, merge tiếp task khác vào đó là
          // không an toàn cho tới khi người dùng xử lý xong.
          break
        }
        outcomes.set(task.id, { ok: true, resultSha: resp.resultSha ?? '' })
      } catch (err) {
        outcomes.set(task.id, {
          ok: false,
          error: err instanceof Error ? err.message : String(err)
        })
        break
      }
    }

    setRunning(false)
    return outcomes
  }

  return { running, batchMergeWorktrees }
}
