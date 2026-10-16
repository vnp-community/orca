import { describe, expect, it } from 'vitest'
import type { OrcaTask } from '../../../../shared/task-types'
import { summarizeTaskDependencies } from './task-dependency-progress'

const dep = (id: string, status?: string): OrcaTask => ({ id, title: id, status }) as OrcaTask

describe('summarizeTaskDependencies', () => {
  it('splits finished from unfinished dependencies', () => {
    const r = summarizeTaskDependencies([
      dep('a', 'done'),
      dep('b', 'in_progress'),
      dep('c', 'done')
    ])
    expect(r.total).toBe(3)
    expect(r.done.map((d) => d.id)).toEqual(['a', 'c'])
    expect(r.pending.map((d) => d.id)).toEqual(['b'])
  })

  it('treats a dependency without a status as unfinished', () => {
    expect(summarizeTaskDependencies([dep('a')]).pending).toHaveLength(1)
  })

  it('has nothing pending when every dependency is done', () => {
    const r = summarizeTaskDependencies([dep('a', 'done')])
    expect(r.pending).toEqual([])
  })
})
