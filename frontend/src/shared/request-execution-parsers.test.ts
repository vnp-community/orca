import { describe, expect, it } from 'vitest'
import { parseActivityPage, parseExecutionSummary } from './request-execution-parsers'
import { fallbackRequestFlow, flowHasExecutionGate, parseRequestFlow } from './request-flow-view'

describe('parseExecutionSummary', () => {
  const wire = {
    requestId: 'r1',
    status: 'request_backlog',
    engine: 'openspec',
    executionMode: 'continuous',
    maxAttempts: 2,
    progress: {
      phasesTotal: 7,
      phasesDone: 6,
      tasksTotal: 28,
      tasksDone: 27,
      tasksFailed: 1,
      tasksRunning: 0
    },
    currentPhaseId: 'p7',
    running: [{ taskId: 't9', title: 'run', attempt: 0, maxAttempts: 2 }, { title: 'no id' }],
    failed: [
      {
        taskId: 't1',
        title: 'acceptance',
        attempts: 2,
        maxAttempts: 2,
        lastCause: 'timed_out',
        lastError: 'x',
        lastAt: '2026-10-15T10:56:29Z'
      }
    ],
    needsYou: [
      { kind: 'failed_task', refId: 't1', title: 'acceptance', since: '2026-10-15T10:56:29Z' },
      { kind: 'something_new', refId: 'z' }
    ],
    handover: { state: 'none', archiveAttempts: 0 }
  }

  it('parses a full summary', () => {
    const s = parseExecutionSummary(wire)!
    expect(s.progress).toEqual({
      phasesTotal: 7,
      phasesDone: 6,
      tasksTotal: 28,
      tasksDone: 27,
      tasksFailed: 1,
      tasksRunning: 0
    })
    expect(s.engine).toBe('openspec')
    expect(s.executionMode).toBe('continuous')
    expect(s.failed[0]).toMatchObject({ taskId: 't1', attempts: 2, lastCause: 'timed_out' })
    expect(s.needsYou.map((n) => n.kind)).toEqual(['failed_task', 'unknown'])
  })

  it('drops items without an id and never reports attempt 0', () => {
    const s = parseExecutionSummary(wire)!
    expect(s.running).toHaveLength(1)
    expect(s.running[0].attempt).toBe(1)
  })

  it('keeps unknown values harmless', () => {
    const s = parseExecutionSummary({
      requestId: 'r',
      failed: [{ taskId: 't', lastCause: 'cosmic_ray' }],
      handover: { state: 'weird' },
      engine: 'quantum'
    })!
    expect(s.failed[0].lastCause).toBe('unknown')
    expect(s.handover.state).toBe('none')
    expect(s.engine).toBeUndefined()
    expect(s.running).toEqual([])
  })

  it('returns null for something that is not a summary', () => {
    expect(parseExecutionSummary(null)).toBeNull()
    expect(parseExecutionSummary({})).toBeNull()
    expect(parseExecutionSummary('x')).toBeNull()
  })
})

describe('parseActivityPage', () => {
  it('parses entries, keeps unknown kinds and falls back for unknown enums', () => {
    const page = parseActivityPage({
      nextPageToken: 'tok',
      entries: [
        {
          id: 'a',
          kind: 'task.failed',
          occurredAt: '2026-10-15T10:56:29Z',
          actorKind: 'agent',
          severity: 'error',
          attempt: 2,
          details: { cause: 'timed_out' },
          summary: 's'
        },
        { id: 'b', kind: 'future.kind', occurredAt: 't', actorKind: 'robot', severity: 'loud' },
        { id: '', kind: 'x' },
        { id: 'c', kind: '' }
      ]
    })
    expect(page.nextPageToken).toBe('tok')
    expect(page.entries.map((e) => e.id)).toEqual(['a', 'b'])
    expect(page.entries[0]).toMatchObject({
      attempt: 2,
      severity: 'error',
      details: { cause: 'timed_out' }
    })
    expect(page.entries[1]).toMatchObject({
      kind: 'future.kind',
      actorKind: 'system',
      severity: 'info',
      details: {}
    })
  })

  it('handles a missing page', () => {
    expect(parseActivityPage(undefined)).toEqual({ entries: [], nextPageToken: undefined })
  })
})

describe('request flow view', () => {
  it('parses request.flow', () => {
    const f = parseRequestFlow({
      type: 'change_request',
      analysisKind: 'solution',
      analysisGate: 'solution',
      planKind: 'plan',
      hasPhases: true,
      startGate: 'plan',
      executionGates: ['phase', 'junk'],
      completesAfterAnalysis: false,
      openSpecProfile: 'full'
    })!
    expect(f).toMatchObject({
      planKind: 'plan',
      hasPhases: true,
      startGate: 'plan',
      executionGates: ['phase'],
      openSpecProfile: 'full',
      source: 'backend'
    })
  })

  it('refuses a payload that is not a flow', () => {
    expect(parseRequestFlow({ type: 'x', planKind: 'weird' })).toBeNull()
    expect(parseRequestFlow(null)).toBeNull()
  })

  it('answers from the fallback table with the size rule', () => {
    expect(fallbackRequestFlow('bug', 'L')?.hasPhases).toBe(true)
    expect(fallbackRequestFlow('bug', 'M')?.hasPhases).toBe(false)
    expect(fallbackRequestFlow('change_request', undefined)?.hasPhases).toBe(true)
    expect(fallbackRequestFlow('unknown', 'L')).toBeNull()
  })

  it('drops the Phase gate of a continuous run and keeps pre_deploy', () => {
    const cr = fallbackRequestFlow('change_request', 'M')!
    expect(flowHasExecutionGate(cr, 'phase', 'step_by_step')).toBe(true)
    expect(flowHasExecutionGate(cr, 'phase', 'continuous')).toBe(false)
    const ops = fallbackRequestFlow('ops_request', 'M')!
    expect(flowHasExecutionGate(ops, 'pre_deploy', 'continuous')).toBe(true)
  })
})
