/**
 * Request Types — v6 (CR-REQ-018)
 *
 * Shared wire-level types for the Orca Request management system.
 * All union values that come from the backend include 'unknown' in the
 * parsed output type (never in send params) so parsers never throw on
 * unexpected server data.
 *
 * @module shared/request-types
 */

// ---------------------------------------------------------------------------
// Request classification
// ---------------------------------------------------------------------------

export type RequestType =
  | 'bug'
  | 'task'
  | 'docs'
  | 'question'
  | 'hotfix'
  | 'security'
  | 'ops_request'
  | 'change_request'
  | 'refactor'
  | 'spike'
  | 'performance'
  | 'unknown'

export type RequestStatus =
  | 'submitted'
  | 'classifying'
  | 'awaiting_type_confirmation'
  | 'analyzing'
  | 'awaiting_analysis_approval'
  | 'awaiting_information'
  | 'planning'
  | 'awaiting_plan_approval'
  | 'executing'
  | 'completed'
  | 'cancelled'
  | 'request_backlog'
  | 'unknown'

export type RequestSize = 'S' | 'M' | 'L'

export type RequestUrgency = 'normal' | 'urgent' | 'unknown'

/** How the request type was last set */
export type TypeSource = 'ai' | 'human' | 'unknown'

export type RequestSourceProvider =
  | 'jira'
  | 'github'
  | 'gitlab'
  | 'linear'
  | 'mcp'
  | 'manual'
  | 'webhook'
  | 'unknown'

/** Which stage the request was returned from (to request_backlog) */
export type ReturnedFromStage =
  | 'classification'
  | 'analysis'
  | 'plan'
  | 'phase'
  | 'task'
  | 'unknown'

// ---------------------------------------------------------------------------
// Core Request entity
// ---------------------------------------------------------------------------

export type RequestLink = {
  id: string
  requestId: string
  relatedRequestId: string
  reason: RequestLinkReason
  createdAt: string
}

export type RequestLinkReason =
  | 'spawned_by_spike'
  | 'spawned_by_question'
  | 'followup_hotfix'
  | 'escalation'
  | 'parent'
  | 'child'
  | 'unknown'

export type OrcaRequest = {
  id: string
  projectId: string
  /** 1-based sequential number within project */
  number: number
  title: string
  body?: string
  type: RequestType
  status: RequestStatus
  size?: RequestSize
  urgency?: RequestUrgency
  confidence?: number
  classificationReason?: string
  /** Set when type was provided/changed by a human */
  typeSource?: TypeSource
  source?: {
    provider: RequestSourceProvider
    ref?: string
    url?: string
    site?: string
  }
  reporterId?: string
  returnedFromStage?: ReturnedFromStage
  returnReason?: string
  returnedById?: string
  returnedAt?: string
  /** Populated by request.get when backend supports it (CR-016 Q1) */
  links?: RequestLink[]
  /** Used for optimistic-update conflict detection */
  version?: number
  /** Task id in the plan that this request is attached to */
  planTaskId?: string
  createdAt: string
  updatedAt: string
}

export type RequestTypeHistoryEntry = {
  id: string
  requestId: string
  fromType: RequestType
  toType: RequestType
  reason?: string
  actorId?: string
  /** 'ai' | 'human' */
  actorKind?: string
  occurredAt: string
}

// ---------------------------------------------------------------------------
// Solution
// ---------------------------------------------------------------------------

export type SolutionKind =
  | 'solution'
  | 'diagnosis'
  | 'findings'
  | 'answer'
  | 'unknown'

export type SolutionStatus =
  | 'generating'
  | 'ready'
  | 'chosen'
  | 'rejected'
  | 'superseded'
  | 'unknown'

export type SolutionOption = {
  id: string
  title: string
  summary?: string
  pros?: string[]
  cons?: string[]
  estimatedEffort?: string
  /** Raw payload remainder — kept for forward compat */
  raw?: Record<string, unknown>
}

export type Solution = {
  id: string
  requestId: string
  kind: SolutionKind
  status: SolutionStatus
  content?: string
  options?: SolutionOption[]
  chosenOptionId?: string
  rejectionReason?: string
  generatedAt?: string
  reviewedAt?: string
  reviewedById?: string
  version?: number
}

// ---------------------------------------------------------------------------
// Approval
// ---------------------------------------------------------------------------

export type ApprovalSubjectType =
  | 'solution'
  | 'plan'
  | 'phase'
  | 'pre_deploy'
  | 'unknown'

export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'expired' | 'unknown'

export type Approval = {
  id: string
  requestId: string
  subjectType: ApprovalSubjectType
  subjectId: string
  /** Digest of the subject at approval time — sent back as expectedDigest */
  subjectDigest?: string
  status: ApprovalStatus
  comment?: string
  approverId?: string
  approvedAt?: string
  /** Used for optimistic-update conflict detection */
  version?: number
  expiresAt?: string
  /** Approval deadline (CONTRACT `dueAt`); the inbox sorts and flags overdue rows by it. */
  dueAt?: string
  /** Who raised the approval; 'system' when the state machine did. */
  requestedBy?: string
  /** Wire subject type before aliasing (findings/answer/task_list/request_type are folded in subjectType). */
  rawSubjectType?: string
  /** Approval stage when the backend names one (e.g. `drift_review` for plan-drift phase approvals). */
  stage?: string
  createdAt: string
  updatedAt: string
}

// ---------------------------------------------------------------------------
// Backlog views
// ---------------------------------------------------------------------------

export type BacklogView = 'requests' | 'tasks' | 'execute'

export type RequestBacklogItem = {
  kind: 'request'
  request: OrcaRequest
}

export type TaskBacklogItem = {
  kind: 'task'
  taskId: string
  taskTitle: string
  taskType?: string
  requestId: string
  requestNumber: number
  requestTitle: string
  planId?: string
  phaseId?: string
}

export type ExecuteBacklogItem = {
  kind: 'execute'
  taskId: string
  taskTitle: string
  taskType?: string
  requestId: string
  requestNumber: number
  requestTitle: string
  planId?: string
  phaseId?: string
}

export type BacklogItem = RequestBacklogItem | TaskBacklogItem | ExecuteBacklogItem

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

export type RequestEvent = {
  requestId: string
  /** Raw event type string; may be prefixed e.g. 'orca.request.request.status_changed' */
  eventType: string
  status?: RequestStatus
  type?: RequestType
  occurredAt: string
}
