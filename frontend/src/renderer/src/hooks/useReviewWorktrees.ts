/**
 * useReviewWorktrees — FE-OR-T322
 *
 * The worktree each task of a Request ran in, from the task list the Plan tab also reads. The Outcome Review knows which tasks
 * its heads came from; the code review opens by worktree. Read once when asked (the tab is open on a delivered review) and not
 * again, because a worktree of a finished task does not move.
 *
 * @module hooks/useReviewWorktrees
 */

import { useEffect, useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'
import { keepTasksOfRequest, planTaskListParams } from './plan-task-list-filter'
import { PLAN_TREE_MAX_PAGES, PLAN_TREE_PAGE_SIZE } from './usePlanTree'
import { normalizeTask } from '../../../shared/task-status-normalization'
import type { OrcaTask } from '../../../shared/task-types'
import type { OrcaRequest } from '../../../shared/request-types'

const NONE: ReadonlyMap<string, string> = new Map()

export function useReviewWorktrees(
  request: Pick<OrcaRequest, 'id' | 'projectId' | 'planTaskId'>,
  enabled: boolean
): { taskWorktrees: ReadonlyMap<string, string>; loading: boolean } {
  const [state, setState] = useState<{ key: string; map: ReadonlyMap<string, string> } | null>(null)
  const key = `${request.id}:${request.planTaskId ?? ''}`

  useEffect(() => {
    if (!enabled || !request.planTaskId || !request.projectId) {
      return undefined
    }
    let live = true
    void (async () => {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const tasks: OrcaTask[] = []
      let pageToken: string | undefined
      try {
        for (let page = 0; page < PLAN_TREE_MAX_PAGES; page++) {
          const res = await callRuntimeRpc<{ tasks: OrcaTask[] | null; nextPageToken?: string }>(
            target,
            'task.list',
            planTaskListParams({
              projectId: request.projectId,
              requestId: request.id,
              pageSize: PLAN_TREE_PAGE_SIZE,
              pageToken
            })
          )
          tasks.push(...keepTasksOfRequest((res.tasks ?? []).map(normalizeTask), request.id))
          pageToken = res.nextPageToken || undefined
          if (!pageToken) {
            break
          }
        }
      } catch {
        // Without the list the code review simply cannot be opened from here, and the screen says so.
      }
      if (live) {
        setState({
          key,
          map: new Map(
            tasks.filter((t) => !!t.worktreeId).map((t) => [t.id, t.worktreeId as string])
          )
        })
      }
    })()
    return () => {
      live = false
    }
  }, [enabled, key, request.id, request.planTaskId, request.projectId])

  const mine = state?.key === key ? state.map : NONE
  return { taskWorktrees: mine, loading: enabled && !!request.planTaskId && state?.key !== key }
}
