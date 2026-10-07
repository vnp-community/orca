/**
 * Request Store Slice — CR-REQ-018-04
 *
 * Manages request-service state: flow support status, cached requests,
 * pending approval count and current page navigation data.
 *
 * Pattern: every action returns a partial object, never mutates (no immer).
 * See slices/task.ts for the same convention and why it exists.
 *
 * @module store/slices/request
 */

import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { OrcaRequest, BacklogView, RequestStatus, RequestType } from '../../../../shared/request-types'

// ---------------------------------------------------------------------------
// Slice state types
// ---------------------------------------------------------------------------

export type RequestFlowSupport = 'supported' | 'unsupported' | 'unknown'

export type RequestListFilters = {
  status?: RequestStatus[]
  type?: RequestType[]
  sourceProvider?: string
  pageSize?: number
}

export type RequestPageData = {
  section: 'requests' | 'approvals' | 'backlog'
  requestId: string | null
  backlogView: BacklogView
  listFilters: RequestListFilters
}

export type RequestSlice = {
  requestFlowSupport: RequestFlowSupport
  requestsById: Record<string, OrcaRequest>
  pendingApprovalCount: number
  requestPage: RequestPageData

  setRequestFlowSupport(support: RequestFlowSupport): void
  upsertRequests(requests: OrcaRequest[]): void
  removeRequest(id: string): void
  setPendingApprovalCount(count: number): void
  setRequestPageData(data: Partial<RequestPageData>): void
  setRequestPageSection(section: RequestPageData['section']): void
  setRequestPageRequest(requestId: string | null): void
}

// ---------------------------------------------------------------------------
// Slice creator
// ---------------------------------------------------------------------------

export const createRequestSlice: StateCreator<AppState, [], [], RequestSlice> = (set) => ({
  requestFlowSupport: 'unknown',
  requestsById: {},
  pendingApprovalCount: 0,
  requestPage: {
    section: 'requests',
    requestId: null,
    backlogView: 'requests',
    listFilters: {}
  },

  setRequestFlowSupport: (support) =>
    set(() => ({ requestFlowSupport: support })),

  upsertRequests: (requests) =>
    set((s) => {
      const updated = { ...s.requestsById }
      for (const r of requests) {
        updated[r.id] = { ...updated[r.id], ...r }
      }
      return { requestsById: updated }
    }),

  removeRequest: (id) =>
    set((s) => {
      const updated = { ...s.requestsById }
      delete updated[id]
      return { requestsById: updated }
    }),

  setPendingApprovalCount: (count) =>
    // Guard against negative counts from stale events
    set(() => ({ pendingApprovalCount: Math.max(0, count) })),

  setRequestPageData: (data) =>
    set((s) => ({
      requestPage: { ...s.requestPage, ...data }
    })),

  setRequestPageSection: (section) =>
    set((s) => ({
      requestPage: { ...s.requestPage, section }
      // Note: intentionally does NOT reset requestId so detail pane stays open
    })),

  setRequestPageRequest: (requestId) =>
    set((s) => ({
      requestPage: { ...s.requestPage, requestId }
    }))
})
