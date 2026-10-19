import { fallbackRequestFlow } from '../../../../shared/request-flow-view'
import { describe, expect, it } from 'vitest'
import {
  defaultReturnStage,
  getRequestActionAvailability,
  getVisibleDetailTabs
} from './request-action-rules'
import type { RequestStatus, RequestType } from '../../../../shared/request-types'

const a = (type: RequestType, status: RequestStatus) =>
  getRequestActionAvailability({ type, status })

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
    for (const s of [
      'analyzing',
      'awaiting_analysis_approval',
      'planning',
      'awaiting_plan_approval',
      'executing'
    ] as const) {
      expect(a('change_request', s).canReturnToBacklog, s).toBe(true)
    }
    for (const s of [
      'submitted',
      'awaiting_type_confirmation',
      'completed',
      'request_backlog'
    ] as const) {
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

  it('shows the tabs the flow has', () => {
    const tabs = (type: Parameters<typeof fallbackRequestFlow>[0]) =>
      getVisibleDetailTabs(fallbackRequestFlow(type, 'M'))
    expect(tabs('task')).toEqual({ analysis: false, plan: true, execution: true })
    expect(tabs('docs')).toEqual({ analysis: false, plan: true, execution: true })
    expect(tabs('ops_request')).toEqual({ analysis: false, plan: true, execution: true })
    expect(tabs('change_request')).toEqual({ analysis: true, plan: true, execution: true })
    expect(tabs('bug')).toEqual({ analysis: true, plan: true, execution: true })
    // A single-task flow has no Plan tab but still has something to execute.
    expect(tabs('hotfix')).toEqual({ analysis: true, plan: false, execution: true })
    expect(tabs('question')).toEqual({ analysis: true, plan: false, execution: false })
    expect(tabs('spike')).toEqual({ analysis: true, plan: false, execution: false })
  })

  it('shows no flow-specific tab for an unknown flow', () => {
    expect(getVisibleDetailTabs(null)).toEqual({ analysis: false, plan: false, execution: false })
  })
})
