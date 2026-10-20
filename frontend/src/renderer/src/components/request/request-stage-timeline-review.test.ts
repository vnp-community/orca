import { describe, expect, it } from 'vitest'
import { buildStageTimeline } from './request-stage-timeline-model'
import type { ReviewTimelineInput } from './request-stage-timeline-review'

const base = { type: 'change_request' as const, size: 'M' as const, status: 'executing' as const }
const on = (over: Partial<ReviewTimelineInput> = {}): ReviewTimelineInput => ({
  enabled: true,
  row: 'none',
  openDecisions: 0,
  ...over
})

describe('the Outcome Review row of the timeline', () => {
  it('is absent when the review is off or not asked about, so the timeline is the one it always was', () => {
    const plain = buildStageTimeline(base)
    expect(buildStageTimeline({ ...base, review: on({ enabled: false }) })).toEqual(plain)
    expect(plain.steps.some((s) => s.id === 'review')).toBe(false)
  })

  it('comes after Execution (and after Handover on OpenSpec), and is never the current stage', () => {
    const native = buildStageTimeline({ ...base, engine: 'native', review: on() })
    expect(native.steps.map((s) => s.id).slice(-2)).toEqual(['execution', 'review'])
    const openspec = buildStageTimeline({ ...base, engine: 'openspec', review: on() })
    expect(openspec.steps.map((s) => s.id).slice(-3)).toEqual(['execution', 'handover', 'review'])
    expect(native.current).toBe('execution')
  })

  it('stays pending with a caption by phase, waits for the person on stale or open decisions, and is done when approved', () => {
    const row = (over: Partial<ReviewTimelineInput>) =>
      buildStageTimeline({ ...base, review: on(over) }).steps.find((s) => s.id === 'review')
    expect(row({ row: 'none' })).toMatchObject({ state: 'pending' })
    expect(row({ row: 'none' })?.caption).toBeUndefined()
    expect(row({ row: 'design' })).toMatchObject({
      state: 'pending',
      caption: { kind: 'review', row: 'design' }
    })
    expect(row({ row: 'delivered' })).toMatchObject({
      state: 'pending',
      caption: { kind: 'review', row: 'delivered' }
    })
    expect(row({ row: 'delivered', openDecisions: 2 })).toMatchObject({
      state: 'waiting',
      caption: { kind: 'review', row: 'decisions', openDecisions: 2 }
    })
    expect(row({ row: 'stale' })).toMatchObject({ state: 'waiting', caption: { row: 'stale' } })
    expect(row({ row: 'approved' })).toMatchObject({ state: 'done', caption: { row: 'approved' } })
    expect(row({ row: 'unknown' })?.caption).toMatchObject({ row: 'unknown' })
  })

  it('does not mark the row done when the Request completes: only an approval does', () => {
    const done = buildStageTimeline({
      ...base,
      status: 'completed',
      review: on({ row: 'delivered' })
    })
    expect(done.steps.filter((s) => s.id !== 'review').every((s) => s.state === 'done')).toBe(true)
    expect(done.steps.find((s) => s.id === 'review')?.state).toBe('pending')
  })

  it('adds nothing to a flow that has no Execution', () => {
    const question = buildStageTimeline({
      type: 'question',
      size: 'S',
      status: 'analyzing',
      review: on()
    })
    expect(question.steps.some((s) => s.id === 'review')).toBe(false)
  })
})
