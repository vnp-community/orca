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
import type {
  OrcaRequest,
  BacklogView,
  RequestStatus,
  RequestType
} from '../../../../shared/request-types'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import { APPROVAL_RUN_ALL_FEATURE } from '../../../../shared/request-run-all'
import { REQUEST_PLAN_AUTO_FEATURE } from '../../../../shared/request-plan-auto'
import { isPlanRevisionsSupported } from '../../../../shared/request-plan-revision'

// ---------------------------------------------------------------------------
// Slice state types
// ---------------------------------------------------------------------------

export type RequestFlowSupport = 'supported' | 'unsupported' | 'unknown'

export type RequestListFilters = {
  /** Project scope chosen in the Request page header; undefined = all projects. */
  projectId?: string
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
  /** Which detail tab to land on when opened from the approval inbox or backlog. */
  focus?: 'type_confirmation' | 'analysis' | 'plan'
}

export type TaskExecutionGateReason = 'phase_not_approved' | 'plan_not_approved'

export type TaskExecutionGate = { requestId: string; reason: TaskExecutionGateReason }

export type RequestSlice = {
  /** Tasks that must not be run yet; written by the Request Plan tab, read by TaskDetail. */
  executionGateByTaskId: Record<string, TaskExecutionGate>
  requestFlowSupport: RequestFlowSupport
  /** Feature flags of `request.flowStatus`; empty for a gateway that predates them, so an absent flag always reads as unsupported. */
  requestFlowFeatures: ReadonlySet<string>
  requestsById: Record<string, OrcaRequest>
  pendingApprovalCount: number
  requestPage: RequestPageData

  setRequestFlowSupport(support: RequestFlowSupport): void
  setRequestFlowFeatures(features: Iterable<string>): void
  upsertRequests(requests: OrcaRequest[]): void
  removeRequest(id: string): void
  setPendingApprovalCount(count: number): void
  setRequestPageData(data: Partial<RequestPageData>): void
  setRequestPageSection(section: RequestPageData['section']): void
  setRequestPageRequest(requestId: string | null): void
  /** Replaces this request's gates; other requests' gates are kept. Pass {} to clear. */
  setTaskExecutionGates(requestId: string, gates: Record<string, TaskExecutionGateReason>): void
}

// ---------------------------------------------------------------------------
// Slice creator
// ---------------------------------------------------------------------------

/** True when the connected gateway stores and enforces `Request.baseRef`; false means hide the picker, never send the field. */
export function supportsBaseRef(state: Pick<RequestSlice, 'requestFlowFeatures'>): boolean {
  return state.requestFlowFeatures.has(REQUEST_BASE_REF_FEATURE)
}

/** True when the connected gateway forwards `executionMode` on `approval.approve`; false means never offer "run all steps". */
export function supportsApproveRunAll(state: Pick<RequestSlice, 'requestFlowFeatures'>): boolean {
  return state.requestFlowFeatures.has(APPROVAL_RUN_ALL_FEATURE)
}

/** True when the connected gateway writes the Plan on its own and serves `request.retryPlan`; false keeps the manual Generate flow. */
export function supportsPlanAuto(state: Pick<RequestSlice, 'requestFlowFeatures'>): boolean {
  return state.requestFlowFeatures.has(REQUEST_PLAN_AUTO_FEATURE)
}

/** True when the connected gateway lists Plan versions and reads their documents; false hides the version list. */
export function supportsPlanRevisions(state: Pick<RequestSlice, 'requestFlowFeatures'>): boolean {
  return isPlanRevisionsSupported(state.requestFlowFeatures)
}

export const createRequestSlice: StateCreator<AppState, [], [], RequestSlice> = (set) => ({
  executionGateByTaskId: {},
  requestFlowSupport: 'unknown',
  requestFlowFeatures: new Set<string>(),
  requestsById: {},
  pendingApprovalCount: 0,
  requestPage: {
    section: 'requests',
    requestId: null,
    backlogView: 'requests',
    listFilters: {}
  },

  setRequestFlowSupport: (support) => set(() => ({ requestFlowSupport: support })),

  setRequestFlowFeatures: (features) => set(() => ({ requestFlowFeatures: new Set(features) })),

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
    })),

  setTaskExecutionGates: (requestId, gates) =>
    set((s) => {
      const next: Record<string, TaskExecutionGate> = {}
      for (const [taskId, gate] of Object.entries(s.executionGateByTaskId)) {
        if (gate.requestId !== requestId) {
          next[taskId] = gate
        }
      }
      for (const [taskId, reason] of Object.entries(gates)) {
        next[taskId] = { requestId, reason }
      }
      return { executionGateByTaskId: next }
    })
})
