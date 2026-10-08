/**
 * Backlog wire types and parsers — CR-REQ-023-01
 *
 * Parsed shapes of backlog.requests / backlog.tasks / backlog.execute (CONTRACT 2.4).
 * Parsers tolerate missing fields and unknown enums so a lagging gateway never crashes the UI.
 *
 * @module shared/request-backlog-types
 */

import { REQUEST_RPC_METHODS } from './request-rpc-methods'
import type { BacklogView, RequestSourceProvider, RequestType, ReturnedFromStage } from './request-types'

export type GateStatus = 'approved' | 'pending' | 'rejected' | 'none' | 'unknown'

export type ReturnedCategory =
  | 'missing_info'
  | 'infeasible'
  | 'blocked_dependency'
  | 'rejected'
  | 'other'
  | 'unknown'

export type RequestBacklogRowData = {
  requestId: string
  number: number
  title: string
  type: RequestType | null
  sourceProvider: RequestSourceProvider
  sourceRef?: string
  sourceUrl?: string
  returnedFromStage: ReturnedFromStage
  returnedCategory: ReturnedCategory
  returnReason?: string
  returnedBy?: string
  returnedAt?: string
  parentRequestIds: string[]
}

export type BacklogTaskRowData = {
  taskId: string
  title: string
  status: string
  estimatedHours: number | null
  assigneeId?: string
  blockedByTaskIds: string[]
  lastEngine?: string
  lastLinkStatus?: string
  failedAttempts: number
  lastError?: string
  /** Not returned by the backend yet; read when present. */
  lastStartedAt?: string
}

export type BacklogGroupData = {
  requestId: string
  planTaskId?: string
  planTitle?: string
  phaseTaskId?: string
  phaseTitle?: string
  gateStatus: GateStatus
  tasks: BacklogTaskRowData[]
}

export type BacklogPage<T> = { items: T[]; nextPageToken: string | null }

export const BACKLOG_RPC_BY_VIEW = {
  requests: REQUEST_RPC_METHODS.BACKLOG_REQUESTS,
  tasks: REQUEST_RPC_METHODS.BACKLOG_TASKS,
  execute: REQUEST_RPC_METHODS.BACKLOG_EXECUTE
} as const satisfies Record<BacklogView, string>

const GATE_STATUSES = new Set<string>(['approved', 'pending', 'rejected', 'none'])
const CATEGORIES = new Set<string>(['missing_info', 'infeasible', 'blocked_dependency', 'rejected', 'other'])
const STAGES = new Set<string>(['classification', 'analysis', 'plan', 'phase', 'task'])
const PROVIDERS = new Set<string>(['jira', 'github', 'gitlab', 'linear', 'mcp', 'manual', 'webhook'])
const TYPES = new Set<string>([
  'bug', 'task', 'docs', 'question', 'hotfix', 'security', 'ops_request',
  'change_request', 'refactor', 'spike', 'performance'
])

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {}
}
function asString(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}
function asNumber(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}
function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : []
}
function asStringArray(v: unknown): string[] {
  return asArray(v).filter((x): x is string => typeof x === 'string' && x !== '')
}
/** Accepts camelCase first, then the snake_case spelling of the same field. */
function pick(r: Record<string, unknown>, camel: string): unknown {
  if (camel in r) {return r[camel]}
  return r[camel.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`)]
}
function enumOf<T extends string>(v: unknown, valid: Set<string>, fallback: T): T {
  return typeof v === 'string' && valid.has(v) ? (v as T) : fallback
}

export function normalizeNextPageToken(raw: unknown): string | null {
  return typeof raw === 'string' && raw !== '' ? raw : null
}

// Why: only http(s) links may reach an anchor; javascript: and friends are dropped.
function safeHttpUrl(raw: unknown): string | undefined {
  const url = asString(raw)
  if (!url) {return undefined}
  try {
    const protocol = new URL(url).protocol
    return protocol === 'http:' || protocol === 'https:' ? url : undefined
  } catch {
    return undefined
  }
}

function parseRequestRow(raw: unknown): RequestBacklogRowData | null {
  const r = asRecord(raw)
  const requestId = asString(pick(r, 'requestId'))
  if (!requestId) {return null}
  const type = pick(r, 'type')
  return {
    requestId,
    number: asNumber(pick(r, 'number')) ?? 0,
    title: asString(pick(r, 'title')) ?? '',
    type: typeof type === 'string' && TYPES.has(type) ? (type as RequestType) : null,
    sourceProvider: enumOf(pick(r, 'sourceProvider'), PROVIDERS, 'unknown'),
    sourceRef: asString(pick(r, 'sourceRef')),
    sourceUrl: safeHttpUrl(pick(r, 'sourceUrl')),
    returnedFromStage: enumOf(pick(r, 'returnedFromStage'), STAGES, 'unknown'),
    returnedCategory: enumOf(pick(r, 'returnedCategory'), CATEGORIES, 'unknown'),
    returnReason: asString(pick(r, 'returnReason')),
    returnedBy: asString(pick(r, 'returnedBy')),
    returnedAt: asString(pick(r, 'returnedAt')),
    parentRequestIds: asStringArray(pick(r, 'parentRequestIds'))
  }
}

function parseTaskRow(raw: unknown): BacklogTaskRowData | null {
  const r = asRecord(raw)
  const taskId = asString(pick(r, 'taskId'))
  if (!taskId) {return null}
  const hours = asNumber(pick(r, 'estimatedHours'))
  const failed = asNumber(pick(r, 'failedAttempts'))
  return {
    taskId,
    title: asString(pick(r, 'title')) ?? '',
    status: asString(pick(r, 'status')) ?? 'open',
    estimatedHours: hours !== undefined && hours >= 0 ? hours : null,
    assigneeId: asString(pick(r, 'assigneeId')),
    blockedByTaskIds: asStringArray(pick(r, 'blockedByTaskIds')),
    lastEngine: asString(pick(r, 'lastEngine')),
    lastLinkStatus: asString(pick(r, 'lastLinkStatus')),
    failedAttempts: failed !== undefined && failed >= 0 ? failed : 0,
    lastError: asString(pick(r, 'lastError')),
    lastStartedAt: asString(pick(r, 'lastStartedAt'))
  }
}

function parseGroup(raw: unknown): BacklogGroupData {
  const r = asRecord(raw)
  return {
    requestId: asString(pick(r, 'requestId')) ?? '',
    planTaskId: asString(pick(r, 'planTaskId')),
    planTitle: asString(pick(r, 'planTitle')),
    phaseTaskId: asString(pick(r, 'phaseTaskId')),
    phaseTitle: asString(pick(r, 'phaseTitle')),
    gateStatus: enumOf(pick(r, 'gateStatus'), GATE_STATUSES, 'unknown'),
    tasks: asArray(pick(r, 'tasks')).map(parseTaskRow).filter((t): t is BacklogTaskRowData => t !== null)
  }
}

export function parseRequestBacklogPage(raw: unknown): BacklogPage<RequestBacklogRowData> {
  const r = asRecord(raw)
  return {
    items: asArray(pick(r, 'requestRows'))
      .map(parseRequestRow)
      .filter((x): x is RequestBacklogRowData => x !== null),
    nextPageToken: normalizeNextPageToken(pick(r, 'nextPageToken'))
  }
}

/** Shared by backlog.tasks and backlog.execute. */
export function parseTaskBacklogPage(raw: unknown): BacklogPage<BacklogGroupData> {
  const r = asRecord(raw)
  return {
    items: asArray(r.groups).map(parseGroup),
    nextPageToken: normalizeNextPageToken(pick(r, 'nextPageToken'))
  }
}
