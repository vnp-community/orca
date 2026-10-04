import { describe, expect, it } from 'vitest'
import {
  derivePhase,
  withPhase,
  buildSpecPrompt,
  buildImplementPrompt,
  PHASE_LABEL_SPEC_PENDING,
  PHASE_LABEL_SPEC_APPROVED,
  PHASE_LABEL_CODE_PENDING
} from './task-spec-build-loop'

describe('derivePhase', () => {
  it('returns not-started for a fresh task with no phase label', () => {
    expect(derivePhase({ status: 'todo', labels: [] })).toBe('not-started')
  })

  it('returns spec-pending when the spec-pending label is set', () => {
    expect(derivePhase({ status: 'review', labels: [PHASE_LABEL_SPEC_PENDING] })).toBe(
      'spec-pending'
    )
  })

  it('returns spec-approved when the spec-approved label is set', () => {
    expect(derivePhase({ status: 'review', labels: [PHASE_LABEL_SPEC_APPROVED] })).toBe(
      'spec-approved'
    )
  })

  it('returns code-pending when the code-pending label is set', () => {
    expect(derivePhase({ status: 'review', labels: [PHASE_LABEL_CODE_PENDING] })).toBe(
      'code-pending'
    )
  })

  it('code-pending wins when multiple phase labels are somehow present', () => {
    expect(
      derivePhase({
        status: 'review',
        labels: [PHASE_LABEL_SPEC_APPROVED, PHASE_LABEL_CODE_PENDING]
      })
    ).toBe('code-pending')
  })

  it('returns done when status is done and no phase label remains', () => {
    expect(derivePhase({ status: 'done', labels: [] })).toBe('done')
  })
})

describe('withPhase', () => {
  it('adds a phase label to an empty list', () => {
    expect(withPhase([], PHASE_LABEL_SPEC_PENDING)).toEqual([PHASE_LABEL_SPEC_PENDING])
  })

  it('replaces an existing phase label with a new one', () => {
    expect(withPhase([PHASE_LABEL_SPEC_PENDING], PHASE_LABEL_SPEC_APPROVED)).toEqual([
      PHASE_LABEL_SPEC_APPROVED
    ])
  })

  it('clears the phase label when phase is null', () => {
    expect(withPhase([PHASE_LABEL_SPEC_APPROVED], null)).toEqual([])
  })

  it('never touches a non-phase label the user set independently', () => {
    expect(
      withPhase(['priority:urgent', PHASE_LABEL_SPEC_PENDING], PHASE_LABEL_SPEC_APPROVED)
    ).toEqual(['priority:urgent', PHASE_LABEL_SPEC_APPROVED])
    expect(withPhase(['priority:urgent', PHASE_LABEL_SPEC_APPROVED], null)).toEqual([
      'priority:urgent'
    ])
  })
})

describe('buildSpecPrompt / buildImplementPrompt', () => {
  const task = { taskNumber: 42, title: 'Add dark mode', description: 'Users want a dark theme.' }

  it('both reference the same deterministic spec file path', () => {
    const specPrompt = buildSpecPrompt(task)
    const implementPrompt = buildImplementPrompt(task)
    expect(specPrompt).toContain('specs/generated/TASK-42-spec.md')
    expect(implementPrompt).toContain('specs/generated/TASK-42-spec.md')
  })

  it('buildSpecPrompt tells the agent not to write implementation code yet', () => {
    expect(buildSpecPrompt(task)).toContain('Do NOT write implementation code yet')
  })

  it('buildSpecPrompt includes the task title and description', () => {
    const prompt = buildSpecPrompt(task)
    expect(prompt).toContain('Add dark mode')
    expect(prompt).toContain('Users want a dark theme.')
  })

  it('buildImplementPrompt references reading the spec file first', () => {
    expect(buildImplementPrompt(task)).toContain('Read that spec file first')
  })
})
