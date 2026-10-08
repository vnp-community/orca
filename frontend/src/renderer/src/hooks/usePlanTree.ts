/**
 * usePlanTree — CR-REQ-021-02
 *
 * Loads the project's tasks into LOCAL state (never the shared `tasks` store, which
 * useTasks owns) and builds the Plan → Phase → Task subtree for one Request.
 *
 * @module hooks/usePlanTree
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'
import { useApprovals } from './useApprovals'
import { useRequestSubscription } from './useRequestSubscription'
import { classifyRequestRpcError } from '../../../shared/request-errors'
import { normalizeTask } from '../../../shared/task-status-normalization'
import { buildPlanSubtree, type PlanSubtree } from '../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../shared/task-types'
import type { Approval, OrcaRequest, RequestEvent } from '../../../shared/request-types'

export const PLAN_TREE_PAGE_SIZE = 200
export const PLAN_TREE_MAX_PAGES = 20
export const PLAN_TREE_POLL_MS = 15_000

const REFRESH_EVENT_RE =
  /(plan\.generated|phase\.started|phase\.completed|approval\.[a-z_]+|request\.status_changed)$/

export type UsePlanTreeResult = {
  tree: PlanSubtree | null
  approvals: Approval[]
  isLoading: boolean
  /** RequestRpcErrorKind of the last failed load (task list or approvals). */
  error: string | null
  /** True when the project has more than PLAN_TREE_MAX_PAGES pages of tasks. */
  truncated: boolean
  refetch: () => void
}

export function usePlanTree(request: OrcaRequest): UsePlanTreeResult {
  const planTaskId = request.planTaskId
  const projectId = request.projectId
  const [tasks, setTasks] = useState<OrcaTask[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [truncated, setTruncated] = useState(false)
  const inFlight = useRef(false)
  const alive = useRef(true)
  const approvalsApi = useApprovals({ requestId: request.id })
  const refetchApprovals = approvalsApi.refetch

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])

  const load = useCallback(async () => {
    // Without a Plan there is nothing to fetch; also one load at a time.
    if (!planTaskId || !projectId || inFlight.current) {
      return
    }
    inFlight.current = true
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const collected: OrcaTask[] = []
      let pageToken: string | undefined
      let pages = 0
      let more = false
      do {
        // Swap to requestId/planTaskId filter here once task.list supports it (CR-REQ-011/016).
        const res = await callRuntimeRpc<{ tasks: OrcaTask[] | null; nextPageToken?: string }>(
          target,
          'task.list',
          { projectId, pageSize: PLAN_TREE_PAGE_SIZE, ...(pageToken ? { pageToken } : {}) }
        )
        pages++
        collected.push(...(res.tasks ?? []).map(normalizeTask))
        pageToken = res.nextPageToken || undefined
        more = !!pageToken
      } while (more && pages < PLAN_TREE_MAX_PAGES)
      if (!alive.current) {
        return
      }
      setTasks(collected)
      setTruncated(more)
      setError(null)
    } catch (err) {
      if (!alive.current) {
        return
      }
      setError(classifyRequestRpcError(err).kind)
    } finally {
      inFlight.current = false
      if (alive.current) {
        setIsLoading(false)
      }
    }
  }, [planTaskId, projectId])

  const refetch = useCallback(() => {
    void load()
    refetchApprovals()
  }, [load, refetchApprovals])

  useEffect(() => {
    void load()
  }, [load])

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event: RequestEvent) => {
      if (REFRESH_EVENT_RE.test(event.eventType)) {
        refetch()
      }
    }
  })

  // Polling is only a fallback while executing; skipped when the tab is hidden.
  const executing = request.status === 'executing'
  useEffect(() => {
    if (!executing || !planTaskId) {
      return
    }
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') {
        refetch()
      }
    }, PLAN_TREE_POLL_MS)
    return () => clearInterval(timer)
  }, [executing, planTaskId, refetch])

  const tree = useMemo(
    () => (planTaskId ? buildPlanSubtree(tasks, planTaskId) : null),
    [tasks, planTaskId]
  )

  return {
    tree,
    approvals: approvalsApi.approvals,
    isLoading: isLoading || approvalsApi.isLoading,
    error: error ?? approvalsApi.error,
    truncated,
    refetch
  }
}
