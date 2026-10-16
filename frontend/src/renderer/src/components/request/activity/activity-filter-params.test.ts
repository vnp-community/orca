import { describe, expect, it } from 'vitest'
import { ACTIVITY_GROUP_KINDS } from './activity-entry-presentation'
import {
  ACTIVITY_FILTER_DEFAULT,
  activityFilterParams,
  isActivityFilterDefault,
  type ActivityFilterState
} from './activity-filter-params'

describe('activityFilterParams', () => {
  it.each<[string, ActivityFilterState, object]>([
    ['default sends nothing', ACTIVITY_FILTER_DEFAULT, {}],
    [
      'approvals',
      { ...ACTIVITY_FILTER_DEFAULT, group: 'approvals' },
      { kinds: [...ACTIVITY_GROUP_KINDS.approvals] }
    ],
    [
      'tasks',
      { ...ACTIVITY_FILTER_DEFAULT, group: 'tasks' },
      { kinds: [...ACTIVITY_GROUP_KINDS.tasks] }
    ],
    [
      'clarifications',
      { ...ACTIVITY_FILTER_DEFAULT, group: 'clarifications' },
      { kinds: [...ACTIVITY_GROUP_KINDS.clarifications] }
    ],
    [
      'changes',
      { ...ACTIVITY_FILTER_DEFAULT, group: 'changes' },
      { kinds: [...ACTIVITY_GROUP_KINDS.changes] }
    ],
    [
      'needs attention',
      { ...ACTIVITY_FILTER_DEFAULT, needsAttentionOnly: true },
      { minSeverity: 'attention' }
    ],
    ['actor ai', { ...ACTIVITY_FILTER_DEFAULT, actor: 'ai' }, { actorKind: 'ai' }],
    [
      'everything together',
      { group: 'tasks', needsAttentionOnly: true, actor: 'agent' },
      { kinds: [...ACTIVITY_GROUP_KINDS.tasks], minSeverity: 'attention', actorKind: 'agent' }
    ]
  ])('%s', (_name, state, expected) => {
    expect(activityFilterParams(state)).toEqual(expected)
  })

  it('does not share the kinds array with the table', () => {
    const params = activityFilterParams({ ...ACTIVITY_FILTER_DEFAULT, group: 'approvals' })
    expect(params.kinds).not.toBe(ACTIVITY_GROUP_KINDS.approvals)
  })
})

describe('isActivityFilterDefault', () => {
  it('is true only for the untouched state', () => {
    expect(isActivityFilterDefault(ACTIVITY_FILTER_DEFAULT)).toBe(true)
    expect(isActivityFilterDefault({ ...ACTIVITY_FILTER_DEFAULT, actor: 'user' })).toBe(false)
    expect(isActivityFilterDefault({ ...ACTIVITY_FILTER_DEFAULT, needsAttentionOnly: true })).toBe(
      false
    )
    expect(isActivityFilterDefault({ ...ACTIVITY_FILTER_DEFAULT, group: 'tasks' })).toBe(false)
  })
})
