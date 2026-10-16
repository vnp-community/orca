/**
 * activity-filter-params — FE-REQ-TASK-037-07
 *
 * Maps the filter bar state to the `request.activity` filter.
 *
 * @module components/request/activity/activity-filter-params
 */

import type {
  ActivityActorKind,
  RequestActivityFilter
} from '../../../../../shared/request-execution-types'
import { ACTIVITY_GROUP_KINDS, type ActivityKindGroup } from './activity-entry-presentation'

export type ActivityActorChoice = 'any' | ActivityActorKind

export type ActivityFilterState = {
  group: ActivityKindGroup
  needsAttentionOnly: boolean
  actor: ActivityActorChoice
}

export const ACTIVITY_FILTER_DEFAULT: ActivityFilterState = {
  group: 'all',
  needsAttentionOnly: false,
  actor: 'any'
}

export const ACTIVITY_GROUP_CHOICES: readonly ActivityKindGroup[] = [
  'all',
  'approvals',
  'tasks',
  'clarifications',
  'changes'
]
export const ACTIVITY_ACTOR_CHOICES: readonly ActivityActorChoice[] = [
  'any',
  'user',
  'ai',
  'agent',
  'system'
]

export function isActivityFilterDefault(state: ActivityFilterState): boolean {
  return state.group === 'all' && !state.needsAttentionOnly && state.actor === 'any'
}

export function activityFilterParams(state: ActivityFilterState): RequestActivityFilter {
  return {
    ...(state.group !== 'all' ? { kinds: [...ACTIVITY_GROUP_KINDS[state.group]] } : {}),
    ...(state.needsAttentionOnly ? { minSeverity: 'attention' as const } : {}),
    ...(state.actor !== 'any' ? { actorKind: state.actor } : {})
  }
}
