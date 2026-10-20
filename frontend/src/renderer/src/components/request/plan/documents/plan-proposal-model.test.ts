import { describe, expect, it } from 'vitest'
import type {
  PlanProposalDocument,
  PlanTaskProposal
} from '../../../../../../shared/request-plan-revision'
import { formatDependencies, formatEstimatedHours, toProposalOutline } from './plan-proposal-model'

const task = (title: string, over: Partial<PlanTaskProposal> = {}): PlanTaskProposal => ({
  title,
  description: '',
  taskType: 'task',
  promptTemplate: '',
  dependsOnIndices: [],
  labels: [],
  irreversible: false,
  ...over
})
const doc = (over: Partial<PlanProposalDocument> = {}): PlanProposalDocument => ({
  title: 'Plan',
  summary: '',
  notes: '',
  phases: [],
  tasks: [],
  ...over
})

describe('toProposalOutline', () => {
  it('numbers phases and their tasks from 1 and turns dependencies into the numbers shown', () => {
    const outline = toProposalOutline(
      doc({
        phases: [
          {
            title: 'Phase A',
            description: '',
            dependsOnPhaseIndices: [],
            tasks: [
              task('a'),
              task('b', { dependsOnIndices: [0] }),
              task('c', { dependsOnIndices: [1, 0] })
            ]
          },
          { title: 'Phase B', description: '', dependsOnPhaseIndices: [0], tasks: [task('d')] }
        ]
      })
    )
    expect(outline.phases.map((p) => p.number)).toEqual([1, 2])
    expect(outline.phases[1].dependsOn).toEqual([1])
    expect(outline.phases[0].tasks.map((t) => [t.number, t.dependsOn])).toEqual([
      [1, []],
      [2, [1]],
      [3, [1, 2]]
    ])
    expect(outline.taskCount).toBe(4)
    expect(outline.tasks).toEqual([])
  })

  it('reads a plan with no phase as a flat list', () => {
    const outline = toProposalOutline(
      doc({ tasks: [task('a'), task('b', { dependsOnIndices: [0] })] })
    )
    expect(outline.phases).toEqual([])
    expect(outline.tasks.map((t) => t.dependsOn)).toEqual([[], [1]])
    expect(outline.taskCount).toBe(2)
  })

  it('drops a dependency that points nowhere, at itself, or twice, instead of failing', () => {
    const outline = toProposalOutline(
      doc({
        tasks: [task('a', { dependsOnIndices: [0, 5, 1, 1] }), task('b', { dependsOnIndices: [] })],
        phases: [{ title: 'P', description: '', dependsOnPhaseIndices: [0, 9], tasks: [] }]
      })
    )
    expect(outline.tasks[0].dependsOn).toEqual([2])
    expect(outline.phases[0].dependsOn).toEqual([])
  })

  it('keeps what the task says: type, hours, irreversible flag, labels and prompt', () => {
    const outline = toProposalOutline(
      doc({
        tasks: [
          task('migrate', {
            taskType: 'migration',
            estimatedHours: 3.5,
            irreversible: true,
            labels: ['db'],
            promptTemplate: 'Do it',
            description: 'Move data'
          })
        ]
      })
    )
    expect(outline.tasks[0]).toMatchObject({
      taskType: 'migration',
      estimatedHours: 3.5,
      irreversible: true,
      labels: ['db'],
      promptTemplate: 'Do it',
      description: 'Move data'
    })
  })

  it('names an untitled phase or task by its number and handles an empty proposal', () => {
    const outline = toProposalOutline(
      doc({
        phases: [{ title: ' ', description: '', dependsOnPhaseIndices: [], tasks: [task('')] }]
      })
    )
    expect(outline.phases[0].title).toBe('#1')
    expect(outline.phases[0].tasks[0].title).toBe('#1')
    const empty = toProposalOutline(doc())
    expect(empty).toMatchObject({ phases: [], tasks: [], taskCount: 0 })
  })
})

describe('formatting', () => {
  it('formats hours and leaves out a missing or non-positive estimate', () => {
    expect(formatEstimatedHours(2)).toBe('2 h')
    expect(formatEstimatedHours(1.25)).toBe('1.3 h')
    expect(formatEstimatedHours(0)).toBeNull()
    expect(formatEstimatedHours(undefined)).toBeNull()
    expect(formatEstimatedHours(Number.NaN)).toBeNull()
  })

  it('lists dependencies as #n', () => {
    expect(formatDependencies([2, 3])).toBe('#2, #3')
    expect(formatDependencies([])).toBe('')
  })
})
