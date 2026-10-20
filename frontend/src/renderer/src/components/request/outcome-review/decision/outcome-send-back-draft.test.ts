import { describe, expect, it } from 'vitest'
import { makeItem } from '../outcome-review.fixture'
import {
  MAX_DRAFT_BODY,
  MAX_DRAFT_TITLE,
  buildSendBackDraft,
  suggestedTypeOf
} from './outcome-send-back-draft'

const review = { reviewId: '0123456789abcdef', phase: 'delivered', kgSnapshot: 'snap-4' }

describe('buildSendBackDraft', () => {
  it("writes one block per item with what it rests on, the KG ids with their snapshot, and the person's note", () => {
    const items = [
      makeItem({
        key: 'a',
        title: 'Gateway calls bug management',
        detail: 'Not in the design.',
        containers: ['ms-gateway'],
        origin: 'agent',
        evidence: [
          { kind: 'kg', ref: 'CT-02', snapshot: 'snap-4' },
          { kind: 'file', ref: 'backend-go/a.go' },
          { kind: 'source', ref: 'SAD', doc: 'SAD.md', section: '5.1' }
        ]
      }),
      makeItem({
        key: 'b',
        title: 'Old call removed',
        severity: 'breaking',
        section: 'flow',
        change: 'removed'
      })
    ]
    const d = buildSendBackDraft({
      requestTitle: 'Add review',
      review,
      items,
      note: '  Please check the gateway first.  '
    })
    expect(d.title).toBe('Address review findings for Add review')
    expect(d.body).toContain('review 01234567')
    expect(d.body).toContain('Please check the gateway first.')
    expect(d.body).toContain('### Gateway calls bug management')
    expect(d.body).toContain('kg CT-02 (snapshot snap-4)')
    expect(d.body).toContain('file backend-go/a.go')
    expect(d.body).toContain('sad SAD.md §5.1')
    expect(d.body).toContain('containers: ms-gateway')
    expect(d.body).toContain('agent reported')
    expect(d.body).toContain('### Old call removed')
    expect(d.omitted).toBe(0)
  })

  it('masks a secret in the title, the detail or the note, and says it did', () => {
    const canary = 'AKIAZ7Q3LMNOPQRST2UV'
    const d = buildSendBackDraft({
      requestTitle: `rotate api_key=${canary}`,
      review,
      items: [makeItem({ detail: `uses password=hunter2hunter2 and token: ${'Ab1'.repeat(14)}` })],
      note: `see https://user:pa55w0rd@host/x`
    })
    expect(d.masked).toBe(true)
    for (const text of [d.title, d.body]) {
      expect(text).not.toContain(canary)
      expect(text).not.toContain('hunter2hunter2')
      expect(text).not.toContain('pa55w0rd')
    }
  })

  it('never goes past the size a Request accepts, and counts the findings it left out', () => {
    const items = Array.from({ length: 80 }, (_, i) =>
      makeItem({ key: `k${i}`, title: `Finding number ${i}`, detail: 'x'.repeat(200) })
    )
    const d = buildSendBackDraft({
      requestTitle: 'T'.repeat(900),
      review,
      items,
      note: 'n'.repeat(5000)
    })
    expect(d.title.length).toBeLessThanOrEqual(MAX_DRAFT_TITLE)
    expect(d.body.length).toBeLessThanOrEqual(MAX_DRAFT_BODY)
    expect(d.omitted).toBeGreaterThan(0)
    expect(d.body).toContain(`${d.omitted} more finding(s) not listed here`)
    expect(d.body.match(/^### /gm)?.length).toBe(items.length - d.omitted)
  })

  it('suggests a bug when most of what was sent back is a regression, a task otherwise', () => {
    const reg = (n: number, total: number) =>
      Array.from({ length: total }, (_, i) =>
        makeItem({ key: `k${i}`, severity: i < n ? 'regression' : 'info' })
      )
    expect(suggestedTypeOf(reg(2, 3))).toBe('bug')
    expect(suggestedTypeOf(reg(1, 2))).toBe('task')
    expect(suggestedTypeOf(reg(0, 3))).toBe('task')
    expect(suggestedTypeOf([])).toBe('task')
  })

  it('is the same for the same input', () => {
    const items = [makeItem({ key: 'a' })]
    expect(buildSendBackDraft({ requestTitle: 'T', review, items, note: 'n' })).toEqual(
      buildSendBackDraft({ requestTitle: 'T', review, items, note: 'n' })
    )
  })
})
