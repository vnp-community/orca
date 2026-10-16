/**
 * Tests for request-stage-timeline-model.ts (CR-REQ-019-01, redesigned by CR-REQ-037 2.3)
 */

import { describe, it, expect } from 'vitest'
import {
  buildStageTimeline,
  isLowConfidence,
  CHILD_REQUEST_RULES,
  type BuildInput,
  type StepId,
  type StepState
} from './request-stage-timeline-model'
import type { ExecutionSummary } from '../../../../shared/request-execution-types'
import type { RequestStatus, RequestType } from '../../../../shared/request-types'

const ids = (input: BuildInput): StepId[] => buildStageTimeline(input).steps.map((s) => s.id)
const states = (input: BuildInput): string =>
  buildStageTimeline(input)
    .steps.map((s) => `${s.id}:${s.state}`)
    .join(' ')

function summary(over: Partial<ExecutionSummary> = {}): ExecutionSummary {
  return {
    requestId: 'r1',
    status: 'executing',
    running: [],
    failed: [],
    needsYou: [],
    maxAttempts: 2,
    progress: {
      phasesTotal: 7,
      phasesDone: 6,
      tasksTotal: 28,
      tasksDone: 27,
      tasksFailed: 0,
      tasksRunning: 1
    },
    handover: { state: 'none', archiveAttempts: 0 },
    ...over
  }
}

describe('buildStageTimeline — unknown inputs', () => {
  it('returns empty steps for unknown type or status', () => {
    expect(buildStageTimeline({ type: 'unknown', status: 'submitted' }).steps).toHaveLength(0)
    expect(buildStageTimeline({ type: 'bug', status: 'unknown' }).steps).toHaveLength(0)
  })
})

describe('stages come from the flow of the type', () => {
  const cases: [RequestType, 'native' | 'openspec', StepId[]][] = [
    ['change_request', 'openspec', ['classification', 'analysis', 'plan', 'execution', 'handover']],
    ['refactor', 'openspec', ['classification', 'analysis', 'plan', 'execution', 'handover']],
    ['bug', 'native', ['classification', 'analysis', 'plan', 'execution']],
    ['security', 'native', ['classification', 'analysis', 'plan', 'execution']],
    ['performance', 'native', ['classification', 'analysis', 'plan', 'execution']],
    ['task', 'native', ['classification', 'plan', 'execution']],
    ['docs', 'native', ['classification', 'plan', 'execution']],
    ['ops_request', 'native', ['classification', 'plan', 'execution']],
    ['hotfix', 'native', ['classification', 'analysis', 'execution']],
    ['question', 'native', ['classification', 'analysis']],
    ['spike', 'native', ['classification', 'analysis']]
  ]
  it.each(cases)('%s on %s', (type, engine, want) => {
    expect(ids({ type, engine, status: 'submitted', size: 'M' })).toEqual(want)
  })

  it('there is no Phases stage, whatever the size', () => {
    expect(ids({ type: 'bug', status: 'executing', size: 'L' })).not.toContain('phase')
    expect(ids({ type: 'change_request', status: 'executing', size: 'L' })).not.toContain('phase')
  })

  it('only an OpenSpec request has a Handover stage, whatever its type', () => {
    expect(ids({ type: 'bug', engine: 'openspec', status: 'executing' })).toContain('handover')
    expect(ids({ type: 'change_request', engine: 'native', status: 'executing' })).not.toContain(
      'handover'
    )
    expect(ids({ type: 'change_request', status: 'executing' })).not.toContain('handover')
  })

  it('a question or a spike never shows Execution', () => {
    expect(ids({ type: 'question', status: 'analyzing' })).not.toContain('execution')
    expect(ids({ type: 'spike', engine: 'openspec', status: 'analyzing' })).not.toContain(
      'handover'
    )
  })

  it('the plan stage is a task list for task, docs and none of the others', () => {
    const label = (type: RequestType) =>
      buildStageTimeline({ type, status: 'planning' }).steps.find((s) => s.id === 'plan')?.labelKey
    expect(label('task')).toMatch(/\.taskList$/)
    expect(label('docs')).toMatch(/\.taskList$/)
    expect(label('bug')).toMatch(/\.plan$/)
    expect(label('ops_request')).toMatch(/\.plan$/)
  })

  it('the analysis stage names the kind of analysis', () => {
    const kind = (type: RequestType) =>
      buildStageTimeline({ type, status: 'analyzing' }).steps.find((s) => s.id === 'analysis')
        ?.variant
    expect(kind('change_request')).toBe('solution')
    expect(kind('bug')).toBe('diagnosis')
    expect(kind('spike')).toBe('findings')
    expect(kind('question')).toBe('answer')
  })

  it('uses the flow it is given instead of the built-in table', () => {
    const flow = {
      type: 'task',
      analysisKind: 'diagnosis',
      analysisGate: 'solution',
      planKind: 'plan',
      hasPhases: false,
      startGate: 'plan',
      executionGates: [],
      completesAfterAnalysis: false,
      humanConfirmRequired: false,
      openSpecProfile: 'none',
      source: 'backend'
    } as const
    expect(ids({ type: 'task', status: 'analyzing', flow })).toContain('analysis')
  })
})

describe('where a request is, by status', () => {
  const at = (status: RequestStatus, extra: Partial<BuildInput> = {}): string =>
    states({ type: 'change_request', engine: 'openspec', size: 'M', status, ...extra })

  it('classification', () => {
    expect(at('classifying')).toBe(
      'classification:running analysis:pending plan:pending execution:pending handover:pending'
    )
    expect(at('awaiting_type_confirmation')).toBe(
      'classification:waiting analysis:pending plan:pending execution:pending handover:pending'
    )
  })

  it('analysis', () => {
    expect(at('analyzing')).toBe(
      'classification:done analysis:running plan:pending execution:pending handover:pending'
    )
    expect(at('awaiting_analysis_approval')).toBe(
      'classification:done analysis:waiting plan:pending execution:pending handover:pending'
    )
  })

  it('plan', () => {
    expect(at('planning')).toBe(
      'classification:done analysis:done plan:running execution:pending handover:pending'
    )
    expect(at('awaiting_plan_approval')).toBe(
      'classification:done analysis:done plan:waiting execution:pending handover:pending'
    )
  })

  it('execution runs, and waits for the viewer when something needs them', () => {
    expect(at('executing')).toBe(
      'classification:done analysis:done plan:done execution:running handover:pending'
    )
    const waiting = at('executing', {
      summary: summary({ needsYou: [{ kind: 'clarification', refId: 'c' }] })
    })
    expect(waiting).toBe(
      'classification:done analysis:done plan:done execution:waiting handover:pending'
    )
  })

  it('completed finishes every stage', () => {
    expect(at('completed')).toBe(
      'classification:done analysis:done plan:done execution:done handover:done'
    )
    expect(
      buildStageTimeline({ type: 'change_request', engine: 'openspec', status: 'completed' })
        .current
    ).toBeNull()
  })

  it('completed with an archive still running keeps Handover going, and failed shows it stopped', () => {
    const archiving = buildStageTimeline({
      type: 'change_request',
      engine: 'openspec',
      status: 'completed',
      summary: summary({ handover: { state: 'archiving', archiveAttempts: 1 } })
    })
    expect(archiving.steps.at(-1)).toMatchObject({
      id: 'handover',
      state: 'running',
      caption: { kind: 'archiving' }
    })
    expect(archiving.current).toBe('handover')
    const failed = buildStageTimeline({
      type: 'change_request',
      engine: 'openspec',
      status: 'completed',
      summary: summary({ handover: { state: 'failed', archiveAttempts: 5 } })
    })
    expect(failed.steps.at(-1)).toMatchObject({
      state: 'stopped',
      caption: { kind: 'handover_failed' }
    })
  })

  it('a hotfix waiting for its pre-deploy approval holds the Execution stage', () => {
    const t = buildStageTimeline({ type: 'hotfix', status: 'awaiting_plan_approval' })
    expect(t.steps.map((s) => `${s.id}:${s.state}`)).toEqual([
      'classification:done',
      'analysis:done',
      'execution:waiting'
    ])
    expect(t.steps[2].caption).toEqual({ kind: 'waiting_approval' })
  })

  it('question and spike finish at Analysis', () => {
    expect(states({ type: 'question', status: 'completed' })).toBe(
      'classification:done analysis:done'
    )
    expect(states({ type: 'spike', status: 'awaiting_analysis_approval' })).toBe(
      'classification:done analysis:waiting'
    )
  })
})

describe('awaiting_information', () => {
  const at = (resumeStatus?: string, type: RequestType = 'change_request') =>
    buildStageTimeline({ type, status: 'awaiting_information', size: 'M', resumeStatus }).current

  it('waits on the step the clarification resumes at', () => {
    expect(at('planning')).toBe('plan')
    expect(at('executing')).toBe('execution')
    expect(at('analyzing')).toBe('analysis')
    expect(at('bogus')).toBe('analysis')
    expect(at(undefined)).toBe('analysis')
  })

  it('falls back to an earlier stage when the type has none for it', () => {
    expect(at('analyzing', 'task')).toBe('classification')
    expect(at('planning', 'hotfix')).toBe('execution')
  })

  it('the step is waiting for the viewer, and resumeStatus is ignored outside awaiting_information', () => {
    expect(
      states({ type: 'bug', status: 'awaiting_information', resumeStatus: 'planning' })
    ).toContain('plan:waiting')
    expect(
      buildStageTimeline({ type: 'bug', status: 'analyzing', resumeStatus: 'executing' }).current
    ).toBe('analysis')
  })
})

describe('a request that left the flow', () => {
  it('request #1242: back to the backlog from a failed task stops Execution and keeps what came before done', () => {
    const t = buildStageTimeline({
      type: 'change_request',
      engine: 'openspec',
      status: 'request_backlog',
      returnedFromStage: 'task',
      size: 'M',
      summary: summary({
        progress: {
          phasesTotal: 7,
          phasesDone: 6,
          tasksTotal: 28,
          tasksDone: 27,
          tasksFailed: 1,
          tasksRunning: 0
        }
      })
    })
    expect(t.steps.map((s) => `${s.id}:${s.state}`)).toEqual([
      'classification:done',
      'analysis:done',
      'plan:done',
      'execution:stopped',
      'handover:pending'
    ])
    expect(t.current).toBe('execution')
    expect(t.steps[3].caption).toEqual({ kind: 'stopped_progress', tasksDone: 27, tasksTotal: 28 })
  })

  it.each([
    ['classification', 'classification'],
    ['analysis', 'analysis'],
    ['plan', 'plan'],
    ['phase', 'execution'],
    ['task', 'execution']
  ] as const)('returned from %s stops %s', (from, step) => {
    const t = buildStageTimeline({
      type: 'change_request',
      status: 'request_backlog',
      returnedFromStage: from
    })
    expect(t.current).toBe(step)
    expect(t.steps.find((s) => s.id === step)?.state).toBe('stopped')
  })

  it('falls back to classification without a stage, and to the stage that exists', () => {
    expect(buildStageTimeline({ type: 'change_request', status: 'request_backlog' }).current).toBe(
      'classification'
    )
    expect(
      buildStageTimeline({ type: 'task', status: 'request_backlog', returnedFromStage: 'analysis' })
        .current
    ).toBe('classification')
    expect(
      buildStageTimeline({ type: 'hotfix', status: 'request_backlog', returnedFromStage: 'plan' })
        .current
    ).toBe('execution')
  })

  it('a cancelled request with no recorded stage shows no stopped stage', () => {
    const t = buildStageTimeline({ type: 'change_request', status: 'cancelled', size: 'M' })
    expect(t.current).toBeNull()
    expect(t.steps.every((s) => s.state === 'pending')).toBe(true)
  })

  it('cancelled stops the stage it was cancelled at', () => {
    const t = buildStageTimeline({
      type: 'change_request',
      status: 'cancelled',
      size: 'M',
      returnedFromStage: 'plan'
    })
    expect(t.steps.find((s) => s.id === t.current)?.state).toBe('stopped')
  })
})

describe('captions', () => {
  it('shows progress with phases while executing', () => {
    const t = buildStageTimeline({
      type: 'change_request',
      status: 'executing',
      summary: summary({
        progress: {
          phasesTotal: 7,
          phasesDone: 2,
          tasksTotal: 28,
          tasksDone: 9,
          tasksFailed: 0,
          tasksRunning: 1
        }
      })
    })
    expect(t.steps.find((s) => s.id === 'execution')?.caption).toEqual({
      kind: 'progress',
      phase: 3,
      phases: 7,
      tasksDone: 9,
      tasksTotal: 28
    })
  })

  it('shows tasks only when there are no phases, and nothing without a summary', () => {
    const flat = buildStageTimeline({
      type: 'task',
      status: 'executing',
      summary: summary({
        progress: {
          phasesTotal: 0,
          phasesDone: 0,
          tasksTotal: 3,
          tasksDone: 1,
          tasksFailed: 0,
          tasksRunning: 1
        }
      })
    })
    expect(flat.steps.find((s) => s.id === 'execution')?.caption).toEqual({
      kind: 'progress',
      tasksDone: 1,
      tasksTotal: 3
    })
    expect(
      buildStageTimeline({ type: 'task', status: 'executing' }).steps.find(
        (s) => s.id === 'execution'
      )?.caption
    ).toBeUndefined()
  })

  it('a stage waiting for approval says so', () => {
    const t = buildStageTimeline({ type: 'bug', status: 'awaiting_plan_approval' })
    expect(t.steps.find((s) => s.id === 'plan')?.caption).toEqual({ kind: 'waiting_approval' })
  })

  it('execution waiting for the viewer says "waiting for you" instead of the progress', () => {
    const t = buildStageTimeline({
      type: 'bug',
      status: 'executing',
      summary: summary({ needsYou: [{ kind: 'approval', refId: 'a' }] })
    })
    const exec = t.steps.find((s) => s.id === 'execution')
    expect(exec?.state as StepState).toBe('waiting')
    expect(exec?.caption).toEqual({
      kind: 'progress',
      phase: 7,
      phases: 7,
      tasksDone: 27,
      tasksTotal: 28
    })
  })
})

describe('isLowConfidence', () => {
  it('returns true for values below threshold (0.59)', () => {
    expect(isLowConfidence(0.59)).toBe(true)
  })

  it('returns false at exactly threshold (0.6)', () => {
    expect(isLowConfidence(0.6)).toBe(false)
  })

  it('returns true for undefined', () => {
    expect(isLowConfidence(undefined)).toBe(true)
  })

  it('returns false for values above threshold', () => {
    expect(isLowConfidence(0.9)).toBe(false)
  })
})

describe('CHILD_REQUEST_RULES', () => {
  it('spike has reason spawned_by_spike', () => {
    expect(CHILD_REQUEST_RULES.spike?.reason).toBe('spawned_by_spike')
  })

  it('question has reason spawned_by_question', () => {
    expect(CHILD_REQUEST_RULES.question?.reason).toBe('spawned_by_question')
  })

  it('hotfix has reason followup_hotfix', () => {
    expect(CHILD_REQUEST_RULES.hotfix?.reason).toBe('followup_hotfix')
  })
})
