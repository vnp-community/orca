import { describe, expect, it } from 'vitest'
import { withReviewHint } from './next-action-review'
import type { NextAction } from './next-action-rules'

const waiting: NextAction = {
  kind: 'waiting',
  title: 'Waiting for someone to approve',
  tab: 'plan'
}
const done: NextAction = { kind: 'done', title: 'This request is done', tab: 'execution' }

describe('withReviewHint', () => {
  it('never covers a failure or something that waits for the viewer', () => {
    const failed: NextAction = {
      kind: 'failed',
      title: 'x',
      primary: { label: 'Reopen', command: 'reopen' }
    }
    const needs: NextAction = {
      kind: 'needs_you',
      title: 'An approval is waiting for you',
      tab: 'plan'
    }
    for (const base of [failed, needs]) {
      expect(withReviewHint(base, 'decide', 2)).toBe(base)
      expect(withReviewHint(base, 'review_design', 0)).toBe(base)
    }
  })

  it('adds a button beside the main message and keeps that message and its tab', () => {
    const a = withReviewHint(waiting, 'review_design', 0)
    expect(a.title).toBe(waiting.title)
    expect(a.tab).toBe('plan')
    expect(a.primary).toEqual({ label: 'Review the design', command: 'open_tab' })
    expect(a.primaryTab).toBe('outcome-review')
    const b = withReviewHint(done, 'generate_delivered', 0)
    expect(b.kind).toBe('done')
    expect(b.primary?.label).toBe('Generate after-code review')
    expect(b.primaryTab).toBe('outcome-review')
  })

  it('open decisions become something that needs the viewer, in the review tab', () => {
    const a = withReviewHint(done, 'decide', 3)
    expect(a).toMatchObject({ kind: 'needs_you', tab: 'outcome-review' })
    expect(a.title).toContain('3')
  })

  it('does not replace a button the bar already has, and no hint changes nothing', () => {
    const withButton: NextAction = { ...waiting, primary: { label: 'Answer', command: 'answer' } }
    expect(withReviewHint(withButton, 'review_design', 0)).toBe(withButton)
    expect(withReviewHint(done, 'none', 0)).toBe(done)
  })
})
