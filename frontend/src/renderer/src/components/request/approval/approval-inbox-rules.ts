/**
 * Approval inbox rules — CR-REQ-022-01
 *
 * Pure ordering, filtering and quick-approve rules for the inbox. No React,
 * store or i18n so the same rules stay testable and reusable.
 *
 * @module components/request/approval/approval-inbox-rules
 */

import type { Approval } from '../../../../../shared/request-types'

/** The 8 backend subject types (README v6 3.5) plus 'unknown' for forward compat. */
export type ApprovalSubjectKind =
  | 'request_type' | 'solution' | 'findings' | 'answer' | 'plan' | 'phase' | 'task_list' | 'pre_deploy' | 'unknown'

export type ApprovalSubjectGroup = 'all' | 'requestType' | 'solution' | 'plan' | 'phase' | 'preDeploy' | 'other'

export const SUBJECT_GROUPS: ApprovalSubjectGroup[] = [
  'all', 'requestType', 'solution', 'plan', 'phase', 'preDeploy', 'other'
]

export const SUBJECT_GROUP: Record<ApprovalSubjectKind, Exclude<ApprovalSubjectGroup, 'all'>> = {
  request_type: 'requestType', solution: 'solution', plan: 'plan', task_list: 'plan',
  phase: 'phase', pre_deploy: 'preDeploy', findings: 'other', answer: 'other', unknown: 'other'
}

const KINDS = new Set<string>(Object.keys(SUBJECT_GROUP))

/** Why: the parser folds findings/answer/task_list into solution/plan; the raw wire value keeps them distinct. */
export function subjectKindOf(a: Pick<Approval, 'subjectType' | 'rawSubjectType'>): ApprovalSubjectKind {
  const raw = a.rawSubjectType ?? a.subjectType
  return KINDS.has(raw) ? (raw as ApprovalSubjectKind) : 'unknown'
}

const SERVER_SUBJECT_TYPE: Partial<Record<ApprovalSubjectGroup, string>> = {
  requestType: 'request_type', solution: 'solution', phase: 'phase', preDeploy: 'pre_deploy'
}

/** Only groups that map to exactly one subject_type can be filtered by the server. */
export function serverSubjectTypeFor(group: ApprovalSubjectGroup): string | undefined {
  return SERVER_SUBJECT_TYPE[group]
}

export function matchesGroup(a: Pick<Approval, 'subjectType' | 'rawSubjectType'>, group: ApprovalSubjectGroup): boolean {
  return group === 'all' || SUBJECT_GROUP[subjectKindOf(a)] === group
}

// Why: solution and plan approvals commit the team to a direction; they always need the full page.
const QUICK_APPROVE_KINDS = new Set<ApprovalSubjectKind>([
  'request_type', 'findings', 'answer', 'task_list', 'phase', 'pre_deploy'
])

export function canQuickApprove(
  a: Pick<Approval, 'subjectType' | 'rawSubjectType' | 'subjectDigest' | 'status'>
): boolean {
  // Why: expectedDigest is mandatory on the wire; without it only "Open" is safe.
  return a.status === 'pending' && Boolean(a.subjectDigest) && QUICK_APPROVE_KINDS.has(subjectKindOf(a))
}

export function dueAtOf(a: Pick<Approval, 'dueAt' | 'expiresAt'>): string | undefined {
  return a.dueAt ?? a.expiresAt
}

function dueMs(a: Pick<Approval, 'dueAt' | 'expiresAt'>): number | null {
  const v = dueAtOf(a)
  const t = v ? Date.parse(v) : Number.NaN
  return Number.isNaN(t) ? null : t
}

export function isOverdue(a: Pick<Approval, 'dueAt' | 'expiresAt'>, now: number): boolean {
  const t = dueMs(a)
  return t !== null && t < now
}

function createdMs(a: Pick<Approval, 'createdAt'>): number {
  const t = Date.parse(a.createdAt)
  return Number.isNaN(t) ? 0 : t
}

/** Overdue first, then nearest due (none last), then newest created, then id for stability. */
export function compareApprovals(now: number): (a: Approval, b: Approval) => number {
  return (a, b) => {
    const ao = isOverdue(a, now)
    const bo = isOverdue(b, now)
    if (ao !== bo) {return ao ? -1 : 1}
    const ad = dueMs(a)
    const bd = dueMs(b)
    if (ad !== bd) {
      if (ad === null) {return 1}
      if (bd === null) {return -1}
      return ad - bd
    }
    const dc = createdMs(b) - createdMs(a)
    if (dc !== 0) {return dc}
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
  }
}

export function groupByRequest(rows: Approval[]): { requestId: string; rows: Approval[] }[] {
  const groups = new Map<string, Approval[]>()
  for (const row of rows) {
    const list = groups.get(row.requestId)
    if (list) {list.push(row)} else {groups.set(row.requestId, [row])}
  }
  return [...groups].map(([requestId, list]) => ({ requestId, rows: list }))
}

export type ApprovalOpenFocus = 'type_confirmation' | 'analysis' | 'plan'

const FOCUS: Partial<Record<ApprovalSubjectKind, ApprovalOpenFocus>> = {
  request_type: 'type_confirmation',
  solution: 'analysis', findings: 'analysis', answer: 'analysis',
  plan: 'plan', task_list: 'plan', phase: 'plan', pre_deploy: 'plan'
}

export function openTargetFor(a: Approval): { section: 'requests'; requestId: string; focus?: ApprovalOpenFocus } {
  return { section: 'requests', requestId: a.requestId, focus: FOCUS[subjectKindOf(a)] }
}

export function quickApproveConsequenceKey(a: Approval): string {
  return `auto.components.request.approval.ApprovalRow.confirmApprove.${subjectKindOf(a)}`
}
