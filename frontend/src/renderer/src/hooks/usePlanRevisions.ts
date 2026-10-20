/**
 * usePlanRevisions — CR-REQ-045
 *
 * The versions of a Request's Plan, newest first, read from `request.planRevisions` in pages of 20. Re-read when the Request
 * moves on (version, status, Plan) or when the server says something about a Plan or an Approval, because a change request
 * adds a version without any action of this tab.
 *
 * A gateway without the channel (older, or the flag absent) is a normal case, not an error: `supported` is false and the tab
 * shows the current version only.
 *
 * @module hooks/usePlanRevisions
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parsePlanRevisions } from '../../../shared/request-plan-revision-parsers'
import { useAppStore } from '../store'
import { supportsPlanRevisions } from '../store/slices/request'
import { REQUEST_POLL_EVENT_TYPE } from './useRequestEvents'
import { useRequestSubscription } from './useRequestSubscription'
import type { PlanRevision } from '../../../shared/request-plan-revision'
import type { OrcaRequest } from '../../../shared/request-types'

export const PLAN_REVISIONS_PAGE_SIZE = 20

const PROGRESS_EVENT_RE = /(?:^|\.)(?:plan\.[a-z_]+|approval\.[a-z_]+|request\.status_changed)$/

type Input = Pick<OrcaRequest, 'id' | 'status'> &
  Partial<Pick<OrcaRequest, 'version' | 'planTaskId'>>

type Loaded = {
  requestId: string
  revisions: PlanRevision[]
  nextPageToken: string
  /** The reader asked for older pages, so a refresh must not forget where they ended. */
  pagedBack: boolean
}

export type PlanRevisionsApi = {
  /** Newest first. */
  revisions: PlanRevision[]
  /** The version the approval gate is about; null before any version is known. */
  current: PlanRevision | null
  hasMore: boolean
  loadMore: () => void
  isLoading: boolean
  isLoadingMore: boolean
  /** RequestRpcErrorKind of the last failed read; the list read before it stays. */
  error: string | null
  /** False on a gateway that does not serve the channel: show the current version only. */
  supported: boolean
  refetch: () => void
}

function mergeNewestFirst(kept: PlanRevision[], incoming: PlanRevision[]): PlanRevision[] {
  const byNumber = new Map<number, PlanRevision>()
  for (const r of kept) {
    byNumber.set(r.revision, r)
  }
  for (const r of incoming) {
    byNumber.set(r.revision, r)
  }
  return [...byNumber.values()].sort((a, b) => b.revision - a.revision)
}

export function usePlanRevisions(request: Input): PlanRevisionsApi {
  const flagged = useAppStore(supportsPlanRevisions)
  const [rejectedByGateway, setRejectedByGateway] = useState(false)
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [errorState, setErrorState] = useState<{ requestId: string; kind: string } | null>(null)
  const requestIdRef = useRef(request.id)
  requestIdRef.current = request.id
  const inFlight = useRef(false)
  const again = useRef(false)
  const loadedRef = useRef<Loaded | null>(null)
  loadedRef.current = loaded

  const hasPlan = !!request.planTaskId
  const enabled = flagged && !rejectedByGateway && hasPlan

  const readFirstPage = useCallback(async (): Promise<void> => {
    // Why: a burst of triggers (version, status and an event together) must make one read, then at most one more.
    if (inFlight.current) {
      again.current = true
      return
    }
    inFlight.current = true
    const id = requestIdRef.current
    setIsLoading(loadedRef.current?.requestId !== id)
    try {
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.PLAN_REVISIONS, {
        id,
        pageSize: PLAN_REVISIONS_PAGE_SIZE
      })
      if (requestIdRef.current !== id) {
        return
      }
      if (!res.ok) {
        if (res.error.kind === 'unsupported') {
          setRejectedByGateway(true)
        } else {
          setErrorState({ requestId: id, kind: res.error.kind })
        }
        return
      }
      const page = parsePlanRevisions(res.value)
      setErrorState(null)
      setLoaded((prev) => {
        const kept = prev?.requestId === id ? prev : null
        return {
          requestId: id,
          revisions: mergeNewestFirst(kept?.revisions ?? [], page.revisions),
          nextPageToken: kept?.pagedBack ? kept.nextPageToken : page.nextPageToken,
          pagedBack: kept?.pagedBack ?? false
        }
      })
    } finally {
      inFlight.current = false
      setIsLoading(false)
      if (again.current) {
        again.current = false
        void readFirstPage()
      }
    }
  }, [])

  useEffect(() => {
    if (!enabled) {
      return
    }
    void readFirstPage()
  }, [enabled, request.id, request.version, request.status, request.planTaskId, readFirstPage])

  useRequestSubscription({
    requestId: request.id,
    onEvent: (event) => {
      if (
        enabled &&
        (PROGRESS_EVENT_RE.test(event.eventType) ||
          (request.status === 'planning' && event.eventType === REQUEST_POLL_EVENT_TYPE))
      ) {
        void readFirstPage()
      }
    }
  })

  const loadMore = useCallback((): void => {
    const current = loadedRef.current
    const id = requestIdRef.current
    if (!current || current.requestId !== id || current.nextPageToken === '' || inFlight.current) {
      return
    }
    inFlight.current = true
    setIsLoadingMore(true)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.PLAN_REVISIONS, {
      id,
      pageSize: PLAN_REVISIONS_PAGE_SIZE,
      pageToken: current.nextPageToken
    })
      .then((res) => {
        if (requestIdRef.current !== id) {
          return
        }
        if (!res.ok) {
          setErrorState({ requestId: id, kind: res.error.kind })
          return
        }
        const page = parsePlanRevisions(res.value)
        setErrorState(null)
        setLoaded((prev) =>
          prev?.requestId === id
            ? {
                ...prev,
                revisions: mergeNewestFirst(prev.revisions, page.revisions),
                nextPageToken: page.nextPageToken,
                pagedBack: true
              }
            : prev
        )
      })
      .finally(() => {
        inFlight.current = false
        setIsLoadingMore(false)
        if (again.current) {
          again.current = false
          void readFirstPage()
        }
      })
  }, [readFirstPage])

  const refetch = useCallback((): void => {
    void readFirstPage()
  }, [readFirstPage])

  const scoped = enabled && loaded?.requestId === request.id ? loaded : null
  const revisions = useMemo(() => scoped?.revisions ?? [], [scoped])
  const current = useMemo(
    () => revisions.find((r) => r.status === 'current') ?? revisions[0] ?? null,
    [revisions]
  )

  return {
    revisions,
    current,
    hasMore: !!scoped && scoped.nextPageToken !== '',
    loadMore,
    isLoading: enabled && isLoading,
    isLoadingMore,
    error: enabled && errorState?.requestId === request.id ? errorState.kind : null,
    supported: flagged && !rejectedByGateway,
    refetch
  }
}
