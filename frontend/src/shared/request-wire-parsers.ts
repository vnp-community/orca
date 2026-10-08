/**
 * Request Wire Parsers — v6 (CR-REQ-018)
 *
 * Parse raw JSON wire objects into typed request domain models.
 * Rules:
 *  - Unknown enum values → 'unknown'
 *  - Missing strings → ''
 *  - Missing arrays → []
 *  - Missing numbers → undefined
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
  RequestTypeHistoryEntry,
  Solution,
  SolutionKind,
  SolutionStatus,
  SolutionOption,
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

// ---------------------------------------------------------------------------
// Enum guard helpers
// ---------------------------------------------------------------------------

const REQUEST_TYPES = new Set<RequestType>([
  'bug', 'task', 'docs', 'question', 'hotfix', 'security',
  'ops_request', 'change_request', 'refactor', 'spike', 'performance'
])

const REQUEST_STATUSES = new Set<RequestStatus>([
  'submitted', 'classifying', 'awaiting_type_confirmation', 'analyzing',
  'awaiting_analysis_approval', 'awaiting_information', 'planning',
  'awaiting_plan_approval', 'executing', 'completed', 'cancelled', 'request_backlog'
])

const REQUEST_SIZES = new Set<RequestSize>(['S', 'M', 'L'])
const REQUEST_URGENCIES = new Set<RequestUrgency>(['normal', 'urgent'])
const TYPE_SOURCES = new Set<TypeSource>(['ai', 'human'])
const SOURCE_PROVIDERS = new Set<RequestSourceProvider>([
  'jira', 'github', 'gitlab', 'linear', 'mcp', 'manual', 'webhook'
])
const RETURNED_FROM_STAGES = new Set<ReturnedFromStage>([
  'classification', 'analysis', 'plan', 'phase', 'task'
])
const LINK_REASONS = new Set<RequestLinkReason>([
  'spawned_by_spike', 'spawned_by_question', 'followup_hotfix', 'escalation', 'parent', 'child'
])
const SOLUTION_KINDS = new Set<SolutionKind>(['solution', 'diagnosis', 'findings', 'answer'])
const SOLUTION_STATUSES = new Set<SolutionStatus>([
  'generating', 'ready', 'chosen', 'rejected', 'superseded'
])
const APPROVAL_SUBJECT_TYPES = new Set<ApprovalSubjectType>([
  'solution', 'plan', 'phase', 'pre_deploy'
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

function safeEnum<T extends string>(value: unknown, valid: Set<T>, fallback: T): T {
  return typeof value === 'string' && valid.has(value as T) ? (value as T) : fallback
}

function safeStr(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function safeOptStr(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

function safeOptNum(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function safeNum(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function safeArr<T>(value: unknown, mapper: (item: unknown, i: number) => T): T[] {
  return Array.isArray(value) ? value.map(mapper) : []
}

function safeRaw(obj: Record<string, unknown>, ...known: string[]): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (!known.includes(k)) {result[k] = v}
  }
  return result
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

  const links =
    Array.isArray(r.links) ? r.links.map(parseRequestLink) : undefined

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
    createdAt: safeStr(r.createdAt),
    updatedAt: safeStr(r.updatedAt)
  }
}

// ---------------------------------------------------------------------------
// Solution
// ---------------------------------------------------------------------------

function parseSolutionOption(raw: unknown, index: number): SolutionOption {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const KNOWN = ['id', 'title', 'summary', 'pros', 'cons', 'estimatedEffort']
  return {
    // Fall back to String(index) when id is missing, as spec requires
    id: typeof r.id === 'string' && r.id ? r.id : String(index),
    title: safeStr(r.title),
    summary: safeOptStr(r.summary),
    pros: safeArr(r.pros, (x) => safeStr(x)),
    cons: safeArr(r.cons, (x) => safeStr(x)),
    estimatedEffort: safeOptStr(r.estimatedEffort) ?? safeOptStr(r.estimated_effort),
    raw: safeRaw(r, ...KNOWN, 'estimated_effort')
  }
}

// CONTRACT SolutionView statuses -> UI statuses (older drafts already used the UI names).
const SOLUTION_STATUS_ALIASES: Record<string, string> = {
  draft: 'generating',
  proposed: 'ready',
  approved: 'chosen'
}

function recommendedOptionId(rec: unknown): string | undefined {
  const r = rec && typeof rec === 'object' ? (rec as Record<string, unknown>) : { id: rec }
  return [r.optionId, r.option_id, r.id].find((v): v is string => typeof v === 'string' && v !== '')
}

export function parseSolution(raw: unknown): Solution {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  // CONTRACT: options is an object { options[], recommendation, ... } for kind=solution
  // and an AI document for other kinds; older drafts sent a plain array.
  const optObj =
    r.options && typeof r.options === 'object' && !Array.isArray(r.options)
      ? (r.options as Record<string, unknown>)
      : undefined
  const optList = Array.isArray(r.options) ? r.options : Array.isArray(optObj?.options) ? optObj.options : undefined
  let options = optList?.map((item, i) => parseSolutionOption(item, i))
  const recId = recommendedOptionId(optObj?.recommendation)
  if (options && recId) {
    options = options.map((o) => (o.id === recId ? { ...o, raw: { ...o.raw, recommended: true } } : o))
  }
  const statusRaw = typeof r.status === 'string' ? (SOLUTION_STATUS_ALIASES[r.status] ?? r.status) : r.status
  let chosenOptionId = safeOptStr(r.chosenOptionId)
  if (!chosenOptionId && typeof r.chosenOption === 'number' && r.chosenOption >= 0 && options) {
    chosenOptionId = options[r.chosenOption]?.id
  }
  let content = safeOptStr(r.content)
  content ??= optObj && !optList ? JSON.stringify(optObj) : undefined
  return {
    id: safeStr(r.id),
    requestId: safeStr(r.requestId),
    kind: safeEnum(r.kind, SOLUTION_KINDS, 'unknown'),
    status: safeEnum(statusRaw, SOLUTION_STATUSES, 'unknown'),
    content,
    options,
    chosenOptionId,
    rejectionReason: safeOptStr(r.rejectionReason),
    generatedAt: safeOptStr(r.generatedAt) ?? safeOptStr(r.createdAt),
    reviewedAt: safeOptStr(r.reviewedAt),
    reviewedById: safeOptStr(r.reviewedById),
    version: safeOptNum(r.version)
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
