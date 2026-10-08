/**
 * ApprovalInboxToolbar — CR-REQ-022-05
 *
 * Subject-group filter and overdue toggle. The project scope lives in
 * RequestPageHeader and is read from the store by the tab, so there is no
 * second project picker here.
 *
 * @module components/request/approval/ApprovalInboxToolbar
 */

import React from 'react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Toggle } from '@/components/ui/toggle'
import { translate } from '@/i18n/i18n'
import { SUBJECT_GROUPS, type ApprovalSubjectGroup } from './approval-inbox-rules'

const T = 'auto.components.request.approval.'

const GROUP_LABELS: Record<ApprovalSubjectGroup, string> = {
  all: 'All', requestType: 'Request type', solution: 'Solution', plan: 'Plan',
  phase: 'Phase', preDeploy: 'Pre-deploy', other: 'Other'
}

export function ApprovalSubjectFilter({
  value,
  onChange
}: {
  value: ApprovalSubjectGroup
  onChange: (value: ApprovalSubjectGroup) => void
}): React.JSX.Element {
  return (
    <ToggleGroup
      type="single"
      size="sm"
      variant="outline"
      value={value}
      // Radix reports '' when the active item is clicked again: fall back to "all".
      onValueChange={(v) => onChange((v || 'all') as ApprovalSubjectGroup)}
      aria-label={translate(`${T}ApprovalInboxTab.title`, 'Approvals')}
      className="flex-wrap"
    >
      {SUBJECT_GROUPS.map((g) => (
        <ToggleGroupItem key={g} value={g}>
          {translate(`${T}ApprovalSubjectFilter.${g}`, GROUP_LABELS[g])}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}

export function ApprovalInboxToolbar({
  subjectGroup,
  overdueOnly,
  onSubjectGroupChange,
  onOverdueChange
}: {
  subjectGroup: ApprovalSubjectGroup
  overdueOnly: boolean
  onSubjectGroupChange: (value: ApprovalSubjectGroup) => void
  onOverdueChange: (value: boolean) => void
}): React.JSX.Element {
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
      <ApprovalSubjectFilter value={subjectGroup} onChange={onSubjectGroupChange} />
      <Toggle size="sm" variant="outline" pressed={overdueOnly} onPressedChange={onOverdueChange}>
        {translate(`${T}OverdueToggle.label`, 'Overdue only')}
      </Toggle>
    </div>
  )
}
