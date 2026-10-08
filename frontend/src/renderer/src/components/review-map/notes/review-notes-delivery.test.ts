import { describe, expect, it, vi } from 'vitest'
import type { DiffComment } from '../../../../../shared/types'
import { handleNotesDelivered } from './review-notes-delivery'
import type { DeliveryDeps } from './review-notes-delivery'

const comment = (id: string): DiffComment =>
  ({ id, worktreeId: 'wt', filePath: `${id}.ts`, lineNumber: 2, body: `body ${id}`, createdAt: 1, side: 'modified' }) as DiffComment

function deps(over: Partial<DeliveryDeps> = {}) {
  const calls: string[] = []
  const d: DeliveryDeps = {
    updateNotes: vi.fn(() => {
      calls.push('record')
      return true
    }),
    clearDelivered: vi.fn(async () => {
      calls.push('clear')
    }),
    markAnnotationsSent: vi.fn(() => {
      calls.push('mark')
    }),
    now: () => 100,
    newBatchId: () => 'batch-1',
    ...over
  }
  return { d, calls }
}

describe('handleNotesDelivered', () => {
  it('records the batch before clearing, then marks annotations for the all scope', async () => {
    const { d, calls } = deps()
    const out = await handleNotesDelivered(d, {
      worktreeId: 'wt',
      delivered: [comment('a'), comment('b')],
      anchors: { a: { kind: 'graph-node', lens: 'erd', nodeKey: 't', filePath: 'a.ts', label: 'orders' } },
      isAllScope: true,
      annotationIds: ['ann-1'],
      fileIdentityByPath: { 'a.ts': 'H' }
    })
    expect(calls).toEqual(['record', 'clear', 'mark'])
    expect(d.markAnnotationsSent).toHaveBeenCalledWith(['ann-1'])
    expect(out.recorded).toBe(true)
    expect(out.batch).toMatchObject({ batchId: 'batch-1', sentAt: 100, turnId: null })
    expect(out.batch.notes.map((n) => n.commentId)).toEqual(['a', 'b'])
    expect(out.batch.notes[0].fileIdentityAtSend).toBe('H')
    expect(out.batch.notes[1].anchor.kind).toBe('diff-line')
  })

  it('does not call annotation.markSent for lens / selection scopes', async () => {
    const { d, calls } = deps()
    await handleNotesDelivered(d, {
      worktreeId: 'wt',
      delivered: [comment('a')],
      anchors: {},
      isAllScope: false,
      annotationIds: ['ann-1']
    })
    expect(calls).toEqual(['record', 'clear'])
  })

  it('still clears and reports recorded=false when the history row is not loaded', async () => {
    const { d, calls } = deps({ updateNotes: vi.fn(() => false) })
    const out = await handleNotesDelivered(d, {
      worktreeId: 'wt',
      delivered: [comment('a')],
      anchors: {},
      isAllScope: false,
      annotationIds: []
    })
    expect(out.recorded).toBe(false)
    expect(d.clearDelivered).toHaveBeenCalled()
    expect(calls).toEqual(['clear'])
  })

  it('treats a throwing record as not recorded and does not block clearing', async () => {
    const { d } = deps({
      updateNotes: vi.fn(() => {
        throw new Error('boom')
      })
    })
    const out = await handleNotesDelivered(d, {
      worktreeId: 'wt',
      delivered: [comment('a')],
      anchors: {},
      isAllScope: false,
      annotationIds: []
    })
    expect(out.recorded).toBe(false)
    expect(d.clearDelivered).toHaveBeenCalledWith('wt', [expect.objectContaining({ id: 'a' })])
  })
})
