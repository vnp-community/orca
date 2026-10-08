import { describe, expect, it } from 'vitest'
import {
  applyQuickFilter,
  isQuickFilterActive,
  QUICK_FILTER_STATUSES
} from './request-list-filters'

describe('request-list-filters quick chips', () => {
  it('"awaitingInfo" filters to awaiting_information and toggles off', () => {
    expect(QUICK_FILTER_STATUSES.awaitingInfo).toEqual(['awaiting_information'])
    const on = applyQuickFilter({ projectId: 'p' }, 'awaitingInfo')
    expect(on).toEqual({ projectId: 'p', status: ['awaiting_information'] })
    expect(isQuickFilterActive(on, 'awaitingInfo')).toBe(true)
    expect(isQuickFilterActive(on, 'running')).toBe(false)
    expect(applyQuickFilter(on, 'awaitingInfo').status).toBeUndefined()
  })

  it('chips replace each other instead of merging', () => {
    const running = applyQuickFilter({}, 'running')
    expect(applyQuickFilter(running, 'awaitingInfo').status).toEqual(['awaiting_information'])
  })
})
