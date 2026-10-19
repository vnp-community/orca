/**
 * ActivityFilterBar — FE-REQ-TASK-037-07
 *
 * Kind-group chips, "needs attention only" and the actor filter of the History feed.
 *
 * @module components/request/activity/ActivityFilterBar
 */

import React from 'react'
import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Toggle } from '@/components/ui/toggle'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { ActivityKindGroup } from './activity-entry-presentation'
import {
  ACTIVITY_ACTOR_CHOICES,
  ACTIVITY_GROUP_CHOICES,
  type ActivityActorChoice,
  type ActivityFilterState
} from './activity-filter-params'

const T = 'auto.components.request.activity.'
const ITEM_CLASS = 'h-7 min-w-0 px-2 text-xs'

function groupLabel(group: ActivityKindGroup): string {
  switch (group) {
    case 'approvals':
      return translate(`${T}filter.approvals`, 'Approvals')
    case 'tasks':
      return translate(`${T}filter.tasks`, 'Tasks')
    case 'clarifications':
      return translate(`${T}filter.clarifications`, 'Clarifications')
    case 'changes':
      return translate(`${T}filter.changes`, 'Type & plan changes')
    default:
      return translate(`${T}filter.all`, 'All')
  }
}

function actorChoiceLabel(actor: ActivityActorChoice): string {
  switch (actor) {
    case 'user':
      return translate(`${T}filter.actor.user`, 'User')
    case 'ai':
      return translate(`${T}filter.actor.ai`, 'AI')
    case 'agent':
      return translate(`${T}filter.actor.agent`, 'Agent')
    case 'system':
      return translate(`${T}filter.actor.system`, 'System')
    default:
      return translate(`${T}filter.actor.any`, 'Anyone')
  }
}

type Props = {
  state: ActivityFilterState
  onChange: (next: ActivityFilterState) => void
}

export function ActivityFilterBar({ state, onChange }: Props): React.JSX.Element {
  return (
    <div
      className="flex flex-col gap-2 border-b border-border px-4 py-2"
      data-testid="activity-filter-bar"
    >
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={state.group}
        // Why: Radix reports '' when the active chip is clicked again; a feed always has one group selected.
        onValueChange={(value) =>
          value && onChange({ ...state, group: value as ActivityKindGroup })
        }
        aria-label={translate(`${T}filter.groupLabel`, 'Filter by kind')}
        className="flex-wrap"
      >
        {ACTIVITY_GROUP_CHOICES.map((group) => (
          <ToggleGroupItem key={group} value={group} className={ITEM_CLASS}>
            {groupLabel(group)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <Toggle
          variant="outline"
          size="sm"
          className={ITEM_CLASS}
          pressed={state.needsAttentionOnly}
          onPressedChange={(pressed) => onChange({ ...state, needsAttentionOnly: pressed })}
        >
          <TriangleAlert className="size-3" aria-hidden />
          {translate(`${T}filter.needsAttention`, 'Needs attention only')}
        </Toggle>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={state.actor}
          onValueChange={(value) =>
            value && onChange({ ...state, actor: value as ActivityActorChoice })
          }
          aria-label={translate(`${T}filter.actorLabel`, 'Filter by who did it')}
          className="flex-wrap"
        >
          {ACTIVITY_ACTOR_CHOICES.map((actor) => (
            <ToggleGroupItem key={actor} value={actor} className={ITEM_CLASS}>
              {actorChoiceLabel(actor)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>
    </div>
  )
}
