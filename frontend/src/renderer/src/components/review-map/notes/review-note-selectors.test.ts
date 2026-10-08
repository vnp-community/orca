import { describe, expect, it } from 'vitest'
import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import type { DiffComment } from '../../../../../shared/types'
import { anchoredNotes, groupNotesByLens, noteCountByNodeKey } from './review-note-selectors'

const c = (id: string, createdAt: number, sentAt?: number): DiffComment =>
  ({ id, worktreeId: 'w', filePath: 'f', lineNumber: 1, body: id, createdAt, sentAt, side: 'modified' }) as DiffComment
const graph = (lens: string, nodeKey: string): ReviewNoteAnchor =>
  ({ kind: 'graph-node', lens, nodeKey, filePath: 'f', label: nodeKey }) as ReviewNoteAnchor

describe('review note selectors', () => {
  const anchors: Record<string, ReviewNoteAnchor> = {
    a: graph('erd', 'orders'),
    b: graph('erd', 'orders'),
    s: graph('erd', 'orders'),
    f: { kind: 'finding', findingKey: 'fk', filePath: 'f', label: 'x' },
    d: { kind: 'diff-line', filePath: 'f', lineNumber: 1 }
  }
  const comments = [c('a', 2), c('b', 1), c('s', 3, 99), c('f', 4), c('d', 5), c('plain', 6)]

  it('keeps only comments with a graph or finding anchor', () => {
    expect(anchoredNotes(comments, anchors).map((n) => n.comment.id)).toEqual(['a', 'b', 's', 'f'])
  })

  it('counts unsent notes per node key', () => {
    expect(noteCountByNodeKey(anchoredNotes(comments, anchors))).toEqual({ orders: 2, fk: 1 })
  })

  it('groups by lens, oldest note first', () => {
    const groups = groupNotesByLens(anchoredNotes(comments, anchors))
    expect(groups.map((g) => g.lens)).toEqual(['erd', 'findings'])
    expect(groups[0].notes.map((n) => n.comment.id)).toEqual(['b', 'a', 's'])
  })
})
