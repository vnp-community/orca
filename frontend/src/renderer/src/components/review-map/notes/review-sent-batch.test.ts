import { describe, expect, it } from 'vitest'
import type { ReviewSentBatch } from '../../../../../shared/code-intel-types'
import { buildSentBatch, capBody, SENT_NOTE_BODY_MAX, trimSentBatches } from './review-sent-batch'
import { mergeReviewNotes, mergeTurnMarkers, readNotes } from './review-state-merge'

const comment = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  filePath: `src/${id}.ts`,
  lineNumber: 3,
  body: `note ${id}`,
  ...over
})

describe('buildSentBatch', () => {
  it('falls back to a diff-line anchor, masks and caps the body, records the file identity', () => {
    const batch = buildSentBatch({
      batchId: 'b1',
      sentAt: 10,
      turnId: 'p:1',
      targetPaneKey: 'p',
      agentType: 'claude',
      comments: [
        comment('a', { body: 'see postgres://u:hunter2@h/db' }),
        comment('b', { startLine: 2, body: 'x'.repeat(SENT_NOTE_BODY_MAX + 10) })
      ],
      anchors: {
        a: { kind: 'graph-node', lens: 'erd', nodeKey: 'orders', filePath: 'src/a.ts', label: 'orders' }
      },
      fileIdentityByPath: { 'src/b.ts': 'H1' }
    })
    expect(batch.notes[0].anchor.kind).toBe('graph-node')
    expect(batch.notes[0].body).not.toContain('hunter2')
    expect(batch.notes[1].anchor).toEqual({ kind: 'diff-line', filePath: 'src/b.ts', startLine: 2, lineNumber: 3 })
    expect(batch.notes[1].body.length).toBe(SENT_NOTE_BODY_MAX)
    expect(batch.notes[1].fileIdentityAtSend).toBe('H1')
    expect(batch.notes[0].fileIdentityAtSend).toBeUndefined()
    expect(capBody('short')).toBe('short')
  })
})

function batch(id: string, sentAt: number, count: number): ReviewSentBatch {
  return {
    batchId: id,
    sentAt,
    turnId: null,
    targetPaneKey: null,
    agentType: null,
    notes: Array.from({ length: count }, (_v, i) => ({
      commentId: `${id}-${i}`,
      anchor: { kind: 'diff-line' as const, filePath: 'a', lineNumber: 1 },
      filePath: 'a',
      lineNumber: 1,
      body: 'b'
    }))
  }
}

describe('trimSentBatches', () => {
  it('leaves a small payload alone', () => {
    const r = trimSentBatches({ anchors: {}, sentBatches: [batch('a', 1, 2), batch('b', 2, 2)] })
    expect(r.droppedBatches).toBe(0)
    expect(r.payload.sentBatches).toHaveLength(2)
  })

  it('drops the oldest batches first when over the entry cap', () => {
    const r = trimSentBatches(
      { anchors: {}, sentBatches: [batch('new', 30, 3), batch('old', 10, 3), batch('mid', 20, 3)] },
      { maxEntries: 6 }
    )
    expect(r.payload.sentBatches.map((b) => b.batchId)).toEqual(['mid', 'new'])
    expect(r.droppedBatches).toBe(1)
  })

  it('counts anchors toward the cap and respects the byte budget', () => {
    const anchors = Object.fromEntries(
      Array.from({ length: 4 }, (_v, i) => [`c${i}`, { kind: 'diff-line' as const, filePath: 'a', lineNumber: i }])
    )
    const r = trimSentBatches({ anchors, sentBatches: [batch('a', 1, 2), batch('b', 2, 2)] }, { maxEntries: 6 })
    expect(r.payload.sentBatches.map((b) => b.batchId)).toEqual(['b'])
    const bytes = trimSentBatches({ anchors: {}, sentBatches: [batch('a', 1, 40), batch('b', 2, 40)] }, { maxBytes: 5000 })
    expect(bytes.payload.sentBatches.map((b) => b.batchId)).toEqual(['b'])
  })

  it('keeps the newest batch even when it alone is too large, cutting its notes', () => {
    const r = trimSentBatches({ anchors: {}, sentBatches: [batch('only', 1, 100)] }, { maxEntries: 10 })
    expect(r.payload.sentBatches).toHaveLength(1)
    expect(r.payload.sentBatches[0].notes.length).toBeLessThanOrEqual(10)
    expect(r.payload.sentBatches[0].notes.length).toBeGreaterThan(0)
  })
})

describe('state merge', () => {
  it('merges notes: anchors by commentId, batches by batchId, local wins ties', () => {
    const a1 = { kind: 'diff-line' as const, filePath: 'x', lineNumber: 1 }
    const a2 = { kind: 'diff-line' as const, filePath: 'x', lineNumber: 2 }
    const merged = mergeReviewNotes(
      { anchors: { c1: a2 }, sentBatches: [batch('b2', 20, 1)] },
      { anchors: { c1: a1, c2: a1 }, sentBatches: [batch('b1', 10, 1), batch('b2', 99, 5)] }
    )
    expect(merged.anchors).toEqual({ c1: a2, c2: a1 })
    expect(merged.sentBatches.map((b) => b.batchId)).toEqual(['b1', 'b2'])
    expect(merged.sentBatches[1].notes).toHaveLength(1)
  })

  it('merges turn markers by turnId keeping the newest five', () => {
    const marker = (n: number) =>
      ({ turnId: `p:${n}`, endedAt: n, files: [], overlayAvailable: false }) as never
    const merged = mergeTurnMarkers([marker(6), marker(7)], [1, 2, 3, 4, 5, 6].map(marker))
    expect(merged.map((m) => m.endedAt)).toEqual([3, 4, 5, 6, 7])
  })

  it('reads a missing notes object as empty', () => {
    expect(readNotes(undefined)).toEqual({ anchors: {}, sentBatches: [] })
    expect(readNotes({ anchors: 3, sentBatches: 'x' })).toEqual({ anchors: {}, sentBatches: [] })
  })
})
