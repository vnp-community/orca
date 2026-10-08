import { describe, expect, it } from 'vitest'
import en from '../../i18n/locales/en.json'
import { getTaskDagStatusPresentation } from './task-dag-status-presentation'
import type { TaskStatus } from '../../../../shared/task-types'

const STATUSES: TaskStatus[] = ['open', 'todo', 'in_progress', 'review', 'done', 'blocked', 'cancelled']

describe('getTaskDagStatusPresentation', () => {
  it('covers every task status with a token class and no hex', () => {
    for (const s of STATUSES) {
      const p = getTaskDagStatusPresentation(s)
      expect(p.nodeClass).not.toMatch(/#[0-9a-fA-F]{3,6}/)
      expect(p.Icon).toBeTruthy()
    }
    expect(getTaskDagStatusPresentation('done').nodeClass).toContain('status-success')
  })

  it('maps todo and unknown statuses to open', () => {
    expect(getTaskDagStatusPresentation('todo')).toBe(getTaskDagStatusPresentation('open'))
    expect(getTaskDagStatusPresentation('weird')).toBe(getTaskDagStatusPresentation('open'))
  })

  it('has en labels for each label key', () => {
    const status = (en as unknown as { auto: { components: { task: { TaskDAGView: { status: Record<string, unknown> } } } } }).auto.components.task.TaskDAGView.status
    for (const s of STATUSES) {
      expect(typeof status[getTaskDagStatusPresentation(s).labelKey.split('.').pop()!]).toBe('string')
    }
  })
})
