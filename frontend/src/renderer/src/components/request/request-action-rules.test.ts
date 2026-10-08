import { describe, expect, it } from 'vitest'
import { defaultReturnStage, getRequestActionAvailability, getVisibleDetailTabs } from './request-action-rules'
import type { RequestStatus, RequestType } from '../../../../shared/request-types'

const a = (type: RequestType, status: RequestStatus) => getRequestActionAvailability({ type, status })

describe('getRequestActionAvailability', () => {
  it('offers cancel until terminal', () => {
    expect(a('bug', 'analyzing').canCancel).toBe(true)
    expect(a('bug', 'completed').canCancel).toBe(false)
    expect(a('bug', 'cancelled').canCancel).toBe(false)
  })

  it('offers reopen only from request_backlog', () => {
    expect(a('bug', 'request_backlog').canReopen).toBe(true)
    expect(a('bug', 'executing').canReopen).toBe(false)
  })

  it('offers return-to-backlog only while a stage is in progress', () => {
    for (const s of ['analyzing', 'awaiting_analysis_approval', 'planning', 'awaiting_plan_approval', 'executing'] as const) {
      expect(a('change_request', s).canReturnToBacklog, s).toBe(true)
    }
    for (const s of ['submitted', 'awaiting_type_confirmation', 'completed', 'request_backlog'] as const) {
      expect(a('change_request', s).canReturnToBacklog, s).toBe(false)
    }
  })

  it('allows changing type only after the type was confirmed', () => {
    expect(a('bug', 'awaiting_type_confirmation').canChangeType).toBe(false)
    expect(a('bug', 'analyzing').canChangeType).toBe(true)
    expect(a('bug', 'completed').canChangeType).toBe(false)
  })

  it('gates spawn-child by type rule and, for hand-off types, by analysis progress', () => {
    expect(a('task', 'executing').canSpawnChild).toBe(false) // no child rule
    expect(a('spike', 'analyzing').canSpawnChild).toBe(false)
    expect(a('spike', 'awaiting_analysis_approval').canSpawnChild).toBe(true)
    expect(a('bug', 'analyzing').canSpawnChild).toBe(true) // escalation any time
    expect(a('bug', 'awaiting_type_confirmation').canSpawnChild).toBe(false)
  })
})

describe('defaultReturnStage / getVisibleDetailTabs', () => {
  it('maps statuses to a return stage', () => {
    expect(defaultReturnStage('planning')).toBe('plan')
    expect(defaultReturnStage('executing')).toBe('task')
    expect(defaultReturnStage('analyzing')).toBe('analysis')
  })

  it('hides analysis for task/docs/ops_request and plan for question/spike', () => {
    expect(getVisibleDetailTabs('task').analysis).toBe(false)
    expect(getVisibleDetailTabs('docs').analysis).toBe(false)
    expect(getVisibleDetailTabs('ops_request').analysis).toBe(false)
    expect(getVisibleDetailTabs('question').plan).toBe(false)
    expect(getVisibleDetailTabs('spike').plan).toBe(false)
    expect(getVisibleDetailTabs('change_request')).toEqual({ analysis: true, plan: true })
  })
})
