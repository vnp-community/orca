/**
 * Request Wire Parsers — v6 (CR-REQ-018)
 *
 * Parse raw JSON wire objects into typed request domain models.
 * Rules:
 *  - Unknown enum values → 'unknown'
 *  - Missing strings → ''
 *  - Missing arrays → []
 *  - Missing numbers → undefined
 *  - Request.baseRef* keep undefined when absent (CR-REQ-039): an old gateway must not look like "default branch"
 *  - SolutionOption.raw keeps the raw payload remainder (forward compat)
 *  - SolutionOption.id falls back to String(index) if missing
 *  - Never throws; always returns a valid typed value
 *
 * @module shared/request-wire-parsers
 */

import type {
  OrcaRequest,
  RequestType,
  RequestStatus,
  RequestSize,
  RequestUrgency,
  TypeSource,
  RequestSourceProvider,
  ReturnedFromStage,
  RequestLink,
  RequestLinkReason,
  RequestBaseRefLockReason,
  RequestTypeHistoryEntry,
  Approval,
  ApprovalSubjectType,
  ApprovalStatus,
  BacklogItem,
  BacklogView,
  RequestBacklogItem,
  TaskBacklogItem,
  ExecuteBacklogItem,
  RequestEvent
} from './request-types'
import { safeEnum, safeNum, safeOptNum, safeOptStr, safeStr } from './request-wire-scalars'
export { parseSolution } from './request-solution-wire-parser'

// ---------------------------------------------------------------------------
// Enum guard helpers
// ---------------------------------------------------------------------------

const REQUEST_TYPES = new Set<RequestType>([
  'bug',
  'task',
  'docs',
  'question',
  'hotfix',
  'security',
  'ops_request',
  'change_request',
  'refactor',
  'spike',
  'performance'
])

const REQUEST_STATUSES = new Set<RequestStatus>([
  'submitted',
  'classifying',
  'awaiting_type_confirmation',
  'analyzing',
  'awaiting_analysis_approval',
  'awaiting_information',
  'planning',
  'awaiting_plan_approval',
  'executing',
  'completed',
  'cancelled',
  'request_backlog'
])

const REQUEST_SIZES = new Set<RequestSize>(['S', 'M', 'L'])
const REQUEST_URGENCIES = new Set<RequestUrgency>(['normal', 'urgent'])
const TYPE_SOURCES = new Set<TypeSource>(['ai', 'human'])
const SOURCE_PROVIDERS = new Set<RequestSourceProvider>([
  'jira',
  'github',
  'gitlab',
  'linear',
  'mcp',
  'manual',
  'webhook'
])
const RETURNED_FROM_STAGES = new Set<ReturnedFromStage>([
  'classification',
  'analysis',
  'plan',
  'phase',
  'task'
])
const LINK_REASONS = new Set<RequestLinkReason>([
  'spawned_by_spike',
  'spawned_by_question',
  'followup_hotfix',
  'escalation',
  'parent',
  'child'
])
const APPROVAL_SUBJECT_TYPES = new Set<ApprovalSubjectType>([
  'solution',
  'plan',
  'phase',
  'pre_deploy'
])
const BASE_REF_LOCK_REASONS = new Set<RequestBaseRefLockReason>([
  '',
  'proposal',
  'task',
  'execution',
  'terminal'
])
const APPROVAL_STATUSES = new Set<ApprovalStatus>(['pending', 'approved', 'rejected', 'expired'])

// Why: CONTRACT-request-ui-api.md names the first status 'new' and flattens source fields;
// the parsers accept both that and the earlier draft shape.
function normalizeRequestStatus(value: unknown): unknown {
  return value === 'new' ? 'submitted' : value
}

const APPROVAL_SUBJECT_ALIASES: Record<string, ApprovalSubjectType> = {
  findings: 'solution',
  answer: 'solution',
  task_list: 'plan'
}

// ---------------------------------------------------------------------------
// RequestLink
// ---------------------------------------------------------------------------

function parseRequestLink(raw: unknown): RequestLink {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    id: safeStr(r.id),
    requestId: safeStr(r.requestId),
    relatedRequestId: safeStr(r.relatedRequestId),
    reason: safeEnum(r.reason, LINK_REASONS, 'unknown'),
    createdAt: safeStr(r.createdAt)
  }
}

// ---------------------------------------------------------------------------
// OrcaRequest
// ---------------------------------------------------------------------------

// A non-string is treated as absent; a string this client does not know stays locked-with-unknown-reason.
function parseBaseRefLockReason(value: unknown): RequestBaseRefLockReason | undefined {
  if (typeof value !== 'string') {
    return undefined
  }
  return BASE_REF_LOCK_REASONS.has(value as RequestBaseRefLockReason)
    ? (value as RequestBaseRefLockReason)
    : 'unknown'
}

export function parseRequest(raw: unknown): OrcaRequest {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>

  const flatProvider = SOURCE_PROVIDERS.has(r.sourceProvider as RequestSourceProvider)
    ? (r.sourceProvider as RequestSourceProvider)
    : undefined
  const source =
    r.source && typeof r.source === 'object'
      ? (() => {
          const s = r.source as Record<string, unknown>
          return {
            provider: safeEnum(s.provider, SOURCE_PROVIDERS, 'unknown'),
            ref: safeOptStr(s.ref),
            url: safeOptStr(s.url),
            site: safeOptStr(s.site)
          }
        })()
      : flatProvider
        ? {
            provider: flatProvider,
            ref: safeOptStr(r.sourceRef),
            url: safeOptStr(r.sourceUrl),
            site: safeOptStr(r.sourceSite)
          }
        : undefined

  const links = Array.isArray(r.links) ? r.links.map(parseRequestLink) : undefined

  return {
    id: safeStr(r.id),
    projectId: safeStr(r.projectId),
    number: safeNum(r.number, 0),
    title: safeStr(r.title),
    body: safeOptStr(r.body),
    type: safeEnum(r.type, REQUEST_TYPES, 'unknown'),
    status: safeEnum(normalizeRequestStatus(r.status), REQUEST_STATUSES, 'unknown'),
    size: REQUEST_SIZES.has(r.size as RequestSize) ? (r.size as RequestSize) : undefined,
    urgency: safeEnum(r.urgency, REQUEST_URGENCIES, 'unknown'),
    confidence: safeOptNum(r.confidence),
    classificationReason: safeOptStr(r.classificationReason),
    typeSource: TYPE_SOURCES.has(r.typeSource as TypeSource)
      ? (r.typeSource as TypeSource)
      : undefined,
    source,
    reporterId: safeOptStr(r.reporterId),
    returnedFromStage: RETURNED_FROM_STAGES.has(r.returnedFromStage as ReturnedFromStage)
      ? (r.returnedFromStage as ReturnedFromStage)
      : undefined,
    returnReason: safeOptStr(r.returnReason),
    returnedById: safeOptStr(r.returnedById) ?? safeOptStr(r.returnedBy),
    returnedAt: safeOptStr(r.returnedAt),
    links,
    version: safeOptNum(r.version),
    planTaskId: safeOptStr(r.planTaskId),
    engine: r.engine === 'native' || r.engine === 'openspec' ? r.engine : undefined,
    executionMode:
      r.executionMode === 'step_by_step' || r.executionMode === 'continuous'
        ? r.executionMode
        : undefined,
    baseRef: safeOptStr(r.baseRef),
    baseRefLocked: typeof r.baseRefLocked === 'boolean' ? r.baseRefLocked : undefined,
    baseRefLockReason: parseBaseRefLockReason(r.baseRefLockReason),
    baseRefUnverified: typeof r.baseRefUnverified === 'boolean' ? r.baseRefUnverified : undefined,
    createdAt: safeStr(r.createdAt),
    updatedAt: safeStr(r.updatedAt)
  }
}

// ---------------------------------------------------------------------------
// Approval
// ---------------------------------------------------------------------------

export function parseApproval(raw: unknown): Approval {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    id: safeStr(r.id),
    requestId: safeStr(r.requestId),
    subjectType:
      typeof r.subjectType === 'string' && APPROVAL_SUBJECT_ALIASES[r.subjectType]
        ? APPROVAL_SUBJECT_ALIASES[r.subjectType]
        : safeEnum(r.subjectType, APPROVAL_SUBJECT_TYPES, 'unknown'),
    subjectId: safeStr(r.subjectId),
    subjectDigest: safeOptStr(r.subjectDigest),
    status: safeEnum(r.status, APPROVAL_STATUSES, 'unknown'),
    comment: safeOptStr(r.comment),
    approverId: safeOptStr(r.approverId) ?? safeOptStr(r.decidedBy),
    approvedAt: safeOptStr(r.approvedAt) ?? safeOptStr(r.decidedAt),
    version: safeOptNum(r.version),
    expiresAt: safeOptStr(r.expiresAt) ?? safeOptStr(r.dueAt),
    dueAt: safeOptStr(r.dueAt) ?? safeOptStr(r.expiresAt),
    requestedBy: safeOptStr(r.requestedBy) ?? safeOptStr(r.createdBy),
    rawSubjectType: safeOptStr(r.subjectType),
    stage: safeOptStr(r.stage),
    createdAt: safeStr(r.createdAt),
    updatedAt: safeStr(r.updatedAt) || safeStr(r.decidedAt) || safeStr(r.createdAt)
  }
}

// ---------------------------------------------------------------------------
// Backlog items
// ---------------------------------------------------------------------------

export function parseBacklogItem(view: BacklogView, raw: unknown): BacklogItem {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>

  if (view === 'requests') {
    const item: RequestBacklogItem = {
      kind: 'request',
      request: parseRequest(r.request ?? r)
    }
    return item
  }

  if (view === 'tasks') {
    const item: TaskBacklogItem = {
      kind: 'task',
      taskId: safeStr(r.taskId),
      taskTitle: safeStr(r.taskTitle),
      taskType: safeOptStr(r.taskType),
      requestId: safeStr(r.requestId),
      requestNumber: safeNum(r.requestNumber, 0),
      requestTitle: safeStr(r.requestTitle),
      planId: safeOptStr(r.planId),
      phaseId: safeOptStr(r.phaseId)
    }
    return item
  }

  // execute
  const item: ExecuteBacklogItem = {
    kind: 'execute',
    taskId: safeStr(r.taskId),
    taskTitle: safeStr(r.taskTitle),
    taskType: safeOptStr(r.taskType),
    requestId: safeStr(r.requestId),
    requestNumber: safeNum(r.requestNumber, 0),
    requestTitle: safeStr(r.requestTitle),
    planId: safeOptStr(r.planId),
    phaseId: safeOptStr(r.phaseId)
  }
  return item
}

// ---------------------------------------------------------------------------
// RequestEvent
// ---------------------------------------------------------------------------

export function parseRequestEvent(raw: unknown): RequestEvent {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    requestId: safeStr(r.requestId),
    // Keep full eventType including any service prefix; consumers normalise
    eventType: safeStr(r.eventType),
    status: REQUEST_STATUSES.has(normalizeRequestStatus(r.status) as RequestStatus)
      ? (normalizeRequestStatus(r.status) as RequestStatus)
      : undefined,
    type: REQUEST_TYPES.has(r.type as RequestType) ? (r.type as RequestType) : undefined,
    occurredAt: safeStr(r.occurredAt)
  }
}

// ---------------------------------------------------------------------------
// RequestTypeHistoryEntry
// ---------------------------------------------------------------------------

export function parseRequestTypeHistoryEntry(raw: unknown): RequestTypeHistoryEntry {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    id: safeStr(r.id) || `${safeStr(r.at)}:${safeStr(r.toType)}`,
    requestId: safeStr(r.requestId),
    fromType: safeEnum(r.fromType, REQUEST_TYPES, 'unknown'),
    toType: safeEnum(r.toType, REQUEST_TYPES, 'unknown'),
    reason: safeOptStr(r.reason),
    actorId: safeOptStr(r.actorId),
    actorKind: safeOptStr(r.actorKind),
    occurredAt: safeStr(r.occurredAt) || safeStr(r.at)
  }
}
