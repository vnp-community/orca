/**
 * activity-entry-presentation — FE-REQ-TASK-037-07
 *
 * Pure facts about an activity entry: icon, filter group, tone and the pieces of `details` the row shows.
 * Wording lives in activity-entry-text.ts so this file stays free of i18n.
 *
 * @module components/request/activity/activity-entry-presentation
 */

import {
  ArrowLeftRight,
  Ban,
  CheckCheck,
  CircleCheck,
  CirclePlay,
  CircleX,
  ClipboardPen,
  FilePenLine,
  FilePlus2,
  FileText,
  Flag,
  ListChecks,
  MessageCircleQuestion,
  MessageSquareReply,
  Play,
  RotateCcw,
  ShieldCheck,
  ShieldQuestion,
  ShieldX,
  Sparkles,
  Undo2,
  type LucideIcon
} from 'lucide-react'
import type { RequestActivityEntry } from '../../../../../shared/request-execution-types'

/** The filter chips. `all` has no kinds, so the backend sends no filter. */
export type ActivityKindGroup = 'all' | 'approvals' | 'tasks' | 'clarifications' | 'changes'

// Why: a kind outside every group (e.g. request.created, or one a newer backend adds) only shows under "All".
export const ACTIVITY_GROUP_KINDS: Record<Exclude<ActivityKindGroup, 'all'>, readonly string[]> = {
  approvals: ['approval.requested', 'approval.decided'],
  tasks: [
    'phase.started',
    'phase.done',
    'plan.done',
    'task.started',
    'task.succeeded',
    'task.accepted',
    'task.failed',
    'task.cancelled'
  ],
  clarifications: ['clarification.asked', 'clarification.answered'],
  changes: [
    'request.classified',
    'request.type_changed',
    'analysis.generated',
    'analysis.revision_requested',
    'plan.revision_requested',
    'request.returned',
    'request.reopened',
    'request.cancelled'
  ]
}

export type ActivityTone = 'neutral' | 'success' | 'attention' | 'error'

type KindFacts = { icon: LucideIcon; successful?: boolean }

const KIND_FACTS: Record<string, KindFacts> = {
  'request.created': { icon: FilePlus2 },
  'request.classified': { icon: Sparkles },
  'request.type_changed': { icon: ArrowLeftRight },
  'analysis.generated': { icon: FileText },
  'analysis.revision_requested': { icon: FilePenLine },
  // Why: its own shape, so a change request on the plan is not mistaken for one on the analysis (FilePenLine) or for plan.done.
  'plan.revision_requested': { icon: ClipboardPen },
  'clarification.asked': { icon: MessageCircleQuestion },
  'clarification.answered': { icon: MessageSquareReply },
  'approval.requested': { icon: ShieldQuestion },
  'approval.decided': { icon: ShieldCheck },
  'phase.started': { icon: Play },
  'phase.done': { icon: Flag, successful: true },
  'plan.done': { icon: ListChecks, successful: true },
  'task.started': { icon: CirclePlay },
  'task.succeeded': { icon: CircleCheck, successful: true },
  'task.accepted': { icon: CheckCheck, successful: true },
  'task.failed': { icon: CircleX },
  'task.cancelled': { icon: Ban },
  'request.returned': { icon: Undo2 },
  'request.reopened': { icon: RotateCcw },
  'request.cancelled': { icon: Ban }
}

// Why: an unknown kind from a newer backend must still render, with a neutral icon and its own summary.
const UNKNOWN_KIND_ICON: LucideIcon = FileText

export function groupOfKind(kind: string): Exclude<ActivityKindGroup, 'all'> | null {
  for (const group of Object.keys(ACTIVITY_GROUP_KINDS) as Exclude<ActivityKindGroup, 'all'>[]) {
    if (ACTIVITY_GROUP_KINDS[group].includes(kind)) {
      return group
    }
  }
  return null
}

export function isKnownActivityKind(kind: string): boolean {
  return kind in KIND_FACTS
}

export function activityIcon(entry: Pick<RequestActivityEntry, 'kind' | 'details'>): LucideIcon {
  // Why: a rejected approval needs a different shape, not just a different colour.
  if (entry.kind === 'approval.decided' && detailString(entry.details, 'result') === 'rejected') {
    return ShieldX
  }
  return KIND_FACTS[entry.kind]?.icon ?? UNKNOWN_KIND_ICON
}

export function activityTone(
  entry: Pick<RequestActivityEntry, 'kind' | 'severity' | 'details'>
): ActivityTone {
  if (entry.severity === 'error') {
    return 'error'
  }
  if (entry.severity === 'attention') {
    return 'attention'
  }
  if (KIND_FACTS[entry.kind]?.successful) {
    return 'success'
  }
  if (entry.kind === 'approval.decided' && detailString(entry.details, 'result') === 'approved') {
    return 'success'
  }
  return 'neutral'
}

export function detailString(
  details: Record<string, unknown> | undefined,
  key: string
): string | undefined {
  const value = details?.[key]
  return typeof value === 'string' && value.trim() !== '' ? value.trim() : undefined
}

export function detailNumber(
  details: Record<string, unknown> | undefined,
  key: string
): number | undefined {
  const value = details?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

/** `#TG-12 Fix login` when the backend knows the number, the title alone otherwise, nothing when it knows neither. */
export function activityTaskName(entry: Pick<RequestActivityEntry, 'details'>): string | undefined {
  const title = detailString(entry.details, 'taskTitle')
  const number = detailNumber(entry.details, 'taskNumber')
  if (!title) {
    return number ? `#TG-${number}` : undefined
  }
  return number ? `#TG-${number} ${title}` : title
}

export type ActivityAttempt = { attempt: number; maxAttempts?: number }

export function activityAttempt(
  entry: Pick<RequestActivityEntry, 'attempt' | 'details'>
): ActivityAttempt | undefined {
  const attempt = entry.attempt ?? detailNumber(entry.details, 'attempt')
  if (!attempt || attempt < 1) {
    return undefined
  }
  const max = detailNumber(entry.details, 'maxAttempts')
  return max && max >= attempt ? { attempt, maxAttempts: max } : { attempt }
}

export type ActivityTargetTab = 'overview' | 'analysis' | 'plan' | 'execution'

/** The tab that holds what an entry is about, or null when the line has nowhere to go. */
export function activityTargetTab(
  entry: Pick<RequestActivityEntry, 'kind' | 'subjectType'>
): ActivityTargetTab | null {
  const { kind, subjectType } = entry
  if (kind.startsWith('task.') || kind.startsWith('phase.') || kind === 'plan.done') {
    return 'execution'
  }
  if (kind.startsWith('approval.')) {
    if (subjectType === 'pre_deploy') {
      return 'execution'
    }
    if (subjectType === 'plan' || subjectType === 'task_list' || subjectType === 'phase') {
      return 'plan'
    }
    return subjectType === 'solution' || subjectType === 'findings' || subjectType === 'answer'
      ? 'analysis'
      : null
  }
  if (kind === 'analysis.generated' || kind === 'analysis.revision_requested') {
    return 'analysis'
  }
  if (kind === 'plan.revision_requested') {
    return 'plan'
  }
  if (kind.startsWith('clarification.')) {
    return 'overview'
  }
  return null
}
