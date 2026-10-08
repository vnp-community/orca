import { describe, expect, it, vi } from 'vitest'
import { jumpToNoteAnchor } from './review-note-navigation'

const actions = () => ({
  setReviewLens: vi.fn(),
  selectReviewSymbol: vi.fn(),
  selectErdTable: vi.fn(),
  selectStorageNode: vi.fn(),
  setReviewDataFlowId: vi.fn(),
  openDiff: vi.fn()
})

describe('jumpToNoteAnchor', () => {
  it('erd: switches lens, selects the table, opens the file-level diff without a line', () => {
    const a = actions()
    jumpToNoteAnchor('wt', { kind: 'graph-node', lens: 'erd', nodeKey: 'orders', filePath: 'db/1.sql', label: 'orders' }, a)
    expect(a.setReviewLens).toHaveBeenCalledWith('wt', 'erd')
    expect(a.selectErdTable).toHaveBeenCalledWith('wt', 'orders')
    expect(a.openDiff).toHaveBeenCalledWith('db/1.sql', undefined)
  })

  it('impact: selects the symbol and opens the diff at the end line', () => {
    const a = actions()
    jumpToNoteAnchor('wt', { kind: 'graph-node', lens: 'impact', nodeKey: 'k', filePath: 'a.ts', startLine: 3, endLine: 9, label: 'f' }, a)
    expect(a.selectReviewSymbol).toHaveBeenCalledWith('wt', 'k')
    expect(a.openDiff).toHaveBeenCalledWith('a.ts', 9)
  })

  it('finding anchors only open the diff; contract only switches lens', () => {
    const a = actions()
    jumpToNoteAnchor('wt', { kind: 'finding', findingKey: 'f', filePath: 'x.go', startLine: 4, label: 'l' }, a)
    expect(a.setReviewLens).not.toHaveBeenCalled()
    expect(a.openDiff).toHaveBeenCalledWith('x.go', 4)
    const b = actions()
    jumpToNoteAnchor('wt', { kind: 'graph-node', lens: 'contract', nodeKey: 'c', filePath: 'p.proto', label: 'c' }, b)
    expect(b.setReviewLens).toHaveBeenCalledWith('wt', 'contract')
    expect(b.selectReviewSymbol).not.toHaveBeenCalled()
  })
})
