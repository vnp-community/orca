/**
 * ApprovalSubjectIcon — CR-REQ-022-05
 *
 * Icon plus text label per approval subject type; the label keeps meaning
 * available without relying on the icon or colour.
 *
 * @module components/request/approval/ApprovalSubjectIcon
 */

import React from 'react'
import {
  CircleHelp, ClipboardList, FileSearch, ListChecks, Lightbulb, MessageSquareText, Rocket, Tags, Layers
} from 'lucide-react'
import { translate } from '@/i18n/i18n'
import type { ApprovalSubjectKind } from './approval-inbox-rules'

const ICONS: Record<ApprovalSubjectKind, React.ComponentType<{ className?: string }>> = {
  request_type: Tags,
  solution: Lightbulb,
  findings: FileSearch,
  answer: MessageSquareText,
  plan: ClipboardList,
  phase: Layers,
  task_list: ListChecks,
  pre_deploy: Rocket,
  unknown: CircleHelp
}

const LABELS: Record<ApprovalSubjectKind, string> = {
  request_type: 'Request type',
  solution: 'Solution',
  findings: 'Findings',
  answer: 'Answer',
  plan: 'Plan',
  phase: 'Phase',
  task_list: 'Task list',
  pre_deploy: 'Pre-deploy',
  unknown: 'Other'
}

export function approvalSubjectLabel(kind: ApprovalSubjectKind): string {
  return translate(`auto.components.request.approval.ApprovalSubjectType.${kind}`, LABELS[kind])
}

export function ApprovalSubjectIcon({ kind }: { kind: ApprovalSubjectKind }): React.JSX.Element {
  const Icon = ICONS[kind]
  return (
    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
      <Icon className="size-3.5" aria-hidden />
      <span>{approvalSubjectLabel(kind)}</span>
    </span>
  )
}
