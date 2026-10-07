/**
 * Tests for task-status-normalization.ts
 */

import { describe, it, expect } from 'vitest'
import { normalizeTaskStatus, normalizeTask } from './task-status-normalization'
import type { OrcaTask } from './task-types'

describe('normalizeTaskStatus', () => {
  it.each(['open', 'todo', 'in_progress', 'review', 'done', 'blocked', 'cancelled'] as const)(
    'keeps valid status "%s" unchanged',
    (status) => {
      expect(normalizeTaskStatus(status)).toBe(status)
    }
  )

  it('coerces "backlog" to "open"', () => {
    expect(normalizeTaskStatus('backlog')).toBe('open')
  })

  it('coerces unknown string to "open"', () => {
    expect(normalizeTaskStatus('weird_status')).toBe('open')
  })

  it('coerces undefined to "open"', () => {
    expect(normalizeTaskStatus(undefined)).toBe('open')
  })

  it('coerces null to "open"', () => {
    expect(normalizeTaskStatus(null)).toBe('open')
  })

  it('coerces number to "open"', () => {
    expect(normalizeTaskStatus(42)).toBe('open')
  })
})

describe('normalizeTask', () => {
  const baseTask: OrcaTask = {
    id: 't1',
    title: 'Test task',
    type: 'task',
    status: 'in_progress',
    priority: 'medium',
    labels: [],
    visibility: 'team',
    progressPercent: 40,
    createdAt: new Date(),
    updatedAt: new Date()
  }

  it('returns same reference when status is already valid', () => {
    const result = normalizeTask(baseTask)
    expect(result).toBe(baseTask)
  })

  it('returns a new object with normalised status when "backlog"', () => {
    const task = { ...baseTask, status: 'backlog' as never }
    const result = normalizeTask(task)
    expect(result).not.toBe(task)
    expect(result.status).toBe('open')
  })

  it('preserves all other fields when normalising', () => {
    const task = { ...baseTask, status: 'backlog' as never, title: 'Keep this' }
    const result = normalizeTask(task)
    expect(result.title).toBe('Keep this')
    expect(result.id).toBe('t1')
  })
})
