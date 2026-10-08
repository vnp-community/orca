/**
 * useRequestEvents — CR-REQ-018-03
 *
 * App-level hook (mounted once in App.tsx): probes request-service support,
 * opens the single global event stream and, when streaming is unavailable
 * (local target / unsupported), polls instead.
 *
 * @module hooks/useRequestEvents
 */

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc, subscribeRequestEvents } from '../runtime/request-rpc-client'
import { emitRequestEvent } from '../lib/request-event-bus'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { useRequestFlowSupport } from './useRequestFlowSupport'
import type { OrcaRequest, RequestEvent } from '../../../shared/request-types'

export const REQUEST_LIST_POLL_MS = 15_000
export const PENDING_APPROVAL_POLL_MS = 60_000

/** Synthetic event type emitted by polling so screens refetch through the bus. */
export const REQUEST_POLL_EVENT_TYPE = 'orca.request.poll'

function isVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

async function refreshPendingApprovalCount(): Promise<void> {
  const result = await callRequestRpc<{ approvals?: unknown[]; totalCount?: number }>(
    REQUEST_RPC_METHODS.APPROVAL_LIST_PENDING,
    // Why: no total in the contract; one page of 100 gives an exact count up to the sidebar's 99+ cap.
    { pageSize: 100 }
  )
  if (!result.ok) {return}
  const count = result.value.totalCount ?? result.value.approvals?.length ?? 0
  useAppStore.getState().setPendingApprovalCount(count)
}

export function useRequestEvents(): void {
  useRequestFlowSupport()
  const support = useAppStore((s) => s.requestFlowSupport)

  useEffect(() => {
    if (support !== 'supported') {return}

    let listTimer: ReturnType<typeof setInterval> | null = null
    let approvalTimer: ReturnType<typeof setInterval> | null = null

    function startPolling(): void {
      if (listTimer !== null) {return}
      // Why: list polling only matters while the Requests page is on screen.
      listTimer = setInterval(() => {
        if (!isVisible() || useAppStore.getState().activeView !== 'requests') {return}
        emitRequestEvent({ requestId: '', eventType: REQUEST_POLL_EVENT_TYPE, occurredAt: new Date().toISOString() })
      }, REQUEST_LIST_POLL_MS)
    }

    function onEvent(event: RequestEvent): void {
      emitRequestEvent(event)
      const store = useAppStore.getState()
      const existing = store.requestsById[event.requestId]
      // Why: events carry no body; only patch requests already cached.
      if (existing && (event.status || event.type)) {
        store.upsertRequests([
          {
            ...existing,
            ...(event.status ? { status: event.status } : {}),
            ...(event.type ? { type: event.type } : {}),
            updatedAt: event.occurredAt
          } as OrcaRequest
        ])
      }
      if (event.eventType.includes('approval')) {void refreshPendingApprovalCount()}
    }

    void refreshPendingApprovalCount()
    approvalTimer = setInterval(() => {
      if (isVisible()) {void refreshPendingApprovalCount()}
    }, PENDING_APPROVAL_POLL_MS)

    const unsubscribe = subscribeRequestEvents({ onEvent, onFallback: startPolling })

    return () => {
      unsubscribe()
      if (listTimer !== null) {clearInterval(listTimer)}
      if (approvalTimer !== null) {clearInterval(approvalTimer)}
    }
  }, [support])
}
