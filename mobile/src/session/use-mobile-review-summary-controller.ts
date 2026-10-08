import { useCallback, useEffect, useRef, useState } from 'react'
import type { ConnectionState } from '../transport/types'
import type { RpcClient } from '../transport/rpc-client'
import { buildMobileReviewFileRoute } from '../source-control/mobile-review-route'
import {
  loadMobileReviewSummary,
  type MobileReviewSummaryState
} from './mobile-review-summary-loaders'
import type { MobileReviewSummaryFilter } from './mobile-review-summary-model'
import { canOpenMobileReviewFindingDiff } from './mobile-review-summary-model'
import { createMobileReviewSummaryRequestGuard } from './mobile-review-summary-request-guard'
import type { MobileReviewSummaryFinding } from './mobile-review-summary-rpc'
import { getWorktreeLabel } from './worktree-label'

type ControllerInput = {
  client: RpcClient | null
  connState: ConnectionState
  hostId: string
  worktreeId: string
  name: string
  onNavigate: (route: string) => void
  onReconnect: (hostId: string) => void | Promise<void>
}

// Why: read-only screen — one load on entry and on explicit refresh; no
// polling or subscription so it adds no standing load on the desktop host.
export function useMobileReviewSummaryController(input: ControllerInput) {
  const { client, connState, hostId, worktreeId, name, onNavigate, onReconnect } = input
  const guardRef = useRef(createMobileReviewSummaryRequestGuard())
  const [screenState, setScreenState] = useState<MobileReviewSummaryState>({
    kind: 'loading'
  })
  const [filter, setFilter] = useState<MobileReviewSummaryFilter>('all')
  const [expandedKey, setExpandedKey] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const worktreeLabel = getWorktreeLabel(name, worktreeId)

  const load = useCallback(async () => {
    const isCurrent = guardRef.current.begin()
    if (!worktreeId) {
      setScreenState({ kind: 'error', message: 'Missing worktree' })
      return
    }
    if (!client || connState !== 'connected') {
      setScreenState({ kind: 'error', message: 'Waiting for desktop...' })
      return
    }
    setScreenState((prev) => (prev.kind === 'ready' ? prev : { kind: 'loading' }))
    const next = await loadMobileReviewSummary(client, worktreeId)
    if (isCurrent()) {
      setScreenState(next)
    }
  }, [client, connState, worktreeId])

  useEffect(() => {
    void load()
  }, [load])

  // Why: spec asks a dropped link to recover without a manual tap; fires once per
  // transition into 'disconnected' (not on 'reconnecting') so it cannot loop.
  useEffect(() => {
    if (connState === 'disconnected' && hostId) {
      void onReconnect(hostId)
    }
    // onReconnect identity may change each render; only the transition matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [connState, hostId])

  const refresh = useCallback(async () => {
    setRefreshing(true)
    try {
      await load()
    } finally {
      setRefreshing(false)
    }
  }, [load])

  const toggleExpanded = useCallback((key: string) => {
    setExpandedKey((prev) => (prev === key ? null : key))
  }, [])

  const openFileDiff = useCallback(
    (finding: MobileReviewSummaryFinding) => {
      if (!canOpenMobileReviewFindingDiff(finding) || !finding.filePath) {
        return
      }
      onNavigate(
        buildMobileReviewFileRoute({
          hostId,
          worktreeId,
          worktreeName: name,
          filePath: finding.filePath,
          area: 'branch'
        })
      )
    },
    [hostId, name, onNavigate, worktreeId]
  )

  const reconnect = useCallback(() => onReconnect(hostId), [hostId, onReconnect])

  return {
    screenState,
    connState,
    filter,
    setFilter,
    expandedKey,
    toggleExpanded,
    refreshing,
    refresh,
    reconnect,
    openFileDiff,
    worktreeLabel
  }
}
