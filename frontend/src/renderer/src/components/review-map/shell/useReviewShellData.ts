import { useCallback, useEffect, useRef, useState } from 'react'
import { subscribeCodeIntelEvents } from '@/lib/code-intel-event-bus'
import type { ReviewDataApi } from '../review-shell-data'
import { scopeKey, toChangeOverlayParams, type ReviewScope } from '../review-scope-model'
import type { ChangeOverlayView, IndexStatusView, ReviewError } from '../review-wire-types'

/** Poll while an index job runs; push events (reindexProgress) are the primary signal. */
export const REVIEW_STATUS_POLL_WHILE_BUILDING_MS = 3000

export type ReviewShellData = {
  status: IndexStatusView | null
  statusError: ReviewError | null
  overlay: ChangeOverlayView | null
  overlayError: ReviewError | null
  overlayPending: boolean
  hasNewDataSignal: boolean
  refreshStatus: (opts?: { refresh?: boolean }) => void
  refetchOverlay: () => void
  /** Applies the "new data" signal: refetches overlay + status. */
  applyNewData: () => void
}

export function useReviewShellData(opts: {
  worktreeId: string
  scope: ReviewScope | null
  api: ReviewDataApi
  enabled: boolean
}): ReviewShellData {
  const { worktreeId, scope, api, enabled } = opts
  const [status, setStatus] = useState<IndexStatusView | null>(null)
  const [statusError, setStatusError] = useState<ReviewError | null>(null)
  const [overlay, setOverlay] = useState<{ key: string; data: ChangeOverlayView } | null>(null)
  const [overlayError, setOverlayError] = useState<ReviewError | null>(null)
  const [overlayPending, setOverlayPending] = useState(false)
  const [hasNewDataSignal, setNewData] = useState(false)
  const statusSeq = useRef(0)
  const overlaySeq = useRef(0)
  const mounted = useRef(true)

  const key = scope ? scopeKey(scope) : null

  const refreshStatus = useCallback(
    (o?: { refresh?: boolean }) => {
      if (!enabled) {
        return
      }
      const seq = ++statusSeq.current
      void api.getStatus(worktreeId, { refresh: o?.refresh }).then((res) => {
        if (!mounted.current || seq !== statusSeq.current) {
          return
        }
        if (res.ok) {
          setStatus(res.value)
          setStatusError(null)
        } else {
          setStatusError(res.error)
        }
      })
    },
    [api, enabled, worktreeId]
  )

  const refetchOverlay = useCallback(() => {
    if (!enabled || !scope) {
      return
    }
    const seq = ++overlaySeq.current
    setOverlayPending(true)
    void api.getChangeOverlay(worktreeId, toChangeOverlayParams(scope)).then((res) => {
      if (!mounted.current || seq !== overlaySeq.current) {
        return
      }
      setOverlayPending(false)
      if (res.ok) {
        setOverlay({ key: key ?? '', data: res.value })
        setOverlayError(null)
        setNewData(false)
      } else {
        // Keep the previous overlay on failure so offline shows cached data + banner.
        setOverlayError(res.error)
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps -- scope identity is captured by `key`
  }, [api, enabled, key, worktreeId])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  useEffect(() => {
    refreshStatus()
  }, [refreshStatus])

  useEffect(() => {
    refetchOverlay()
  }, [refetchOverlay])

  // Poll while a job runs so a missed push does not leave a stale progress bar.
  const building = status?.overall === 'BUILDING' || Boolean(status?.activeJob)
  useEffect(() => {
    if (!building || !enabled) {
      return
    }
    const id = setInterval(() => refreshStatus(), REVIEW_STATUS_POLL_WHILE_BUILDING_MS)
    return () => clearInterval(id)
  }, [building, enabled, refreshStatus])

  useEffect(() => {
    if (!enabled) {
      return
    }
    return subscribeCodeIntelEvents((event) => {
      if (event.worktreeId !== worktreeId) {
        return
      }
      if (event.event === 'reindexProgress') {
        refreshStatus()
      }
      if (event.event === 'changed') {
        setNewData(true)
        refreshStatus()
      }
    })
  }, [enabled, refreshStatus, worktreeId])

  const applyNewData = useCallback(() => {
    refreshStatus()
    refetchOverlay()
  }, [refreshStatus, refetchOverlay])

  // An overlay of another scope must never be shown under the new scope's header.
  const visibleOverlay = overlay && overlay.key === key ? overlay.data : null

  return {
    status,
    statusError,
    overlay: visibleOverlay,
    overlayError,
    overlayPending,
    hasNewDataSignal,
    refreshStatus,
    refetchOverlay,
    applyNewData
  }
}
