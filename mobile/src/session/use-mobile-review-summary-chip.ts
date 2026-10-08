import { useEffect, useRef, useState } from 'react'
import type { ConnectionState } from '../transport/types'
import type { RpcClient } from '../transport/rpc-client'
import { loadMobileReviewSummary } from './mobile-review-summary-loaders'
import {
  buildMobileReviewSummaryChip,
  type MobileReviewSummaryChip
} from './mobile-review-summary-chip'
import { createMobileReviewSummaryRequestGuard } from './mobile-review-summary-request-guard'

type Input = {
  client: RpcClient | null
  connState: ConnectionState
  worktreeId: string
  enabled: boolean
}

// Why: one probe per worktree+connection (no polling) so the Source Control hub
// adds no standing load on the desktop; the chip appears only after it succeeds.
export function useMobileReviewSummaryChip({
  client,
  connState,
  worktreeId,
  enabled
}: Input): MobileReviewSummaryChip | null {
  const guardRef = useRef(createMobileReviewSummaryRequestGuard())
  const [chip, setChip] = useState<MobileReviewSummaryChip | null>(null)
  const connected = connState === 'connected'

  useEffect(() => {
    const isCurrent = guardRef.current.begin()
    if (!enabled || !client || !connected || !worktreeId) {
      return
    }
    setChip(null)
    void loadMobileReviewSummary(client, worktreeId).then((state) => {
      if (isCurrent()) {
        setChip(buildMobileReviewSummaryChip(state))
      }
    })
  }, [client, connected, enabled, worktreeId])

  return enabled ? chip : null
}
