import { describe, it, expect } from 'vitest'
import { classifyIndexBasis, IndexBasisInput } from './codeintel-index-basis'

describe('classifyIndexBasis', () => {
  const baseInput: IndexBasisInput = {
    tool: 'gitnexus',
    toolUsable: true,
    indexExists: true,
    rootMatches: true,
    indexedCommit: 'abc',
    indexedAtMs: 1000,
    headCommit: 'abc',
    headCommitTimeMs: 900,
    mergeBase: 'abc',
    mergeBaseCommitTimeMs: 900,
    dirtySinceIndex: false,
    pendingChanges: { added: 0, modified: 0, removed: 0 }
  }

  describe('GitNexus 6 rows', () => {
    it('row 1: !indexExists -> none/unknown', () => {
      const result = classifyIndexBasis({ ...baseInput, indexExists: false })
      expect(result).toEqual({ indexScope: 'none', freshness: 'unknown' })
    })

    it('row 2: !toolUsable -> none/unknown', () => {
      const result = classifyIndexBasis({ ...baseInput, toolUsable: false })
      expect(result).toEqual({ indexScope: 'none', freshness: 'unknown' })
    })

    it('row 3: rootMatches && indexedCommit == headCommit && !dirty -> exact/fresh', () => {
      const result = classifyIndexBasis({ ...baseInput })
      expect(result).toEqual({ indexScope: 'exact', freshness: 'fresh' })
    })

    it('row 4: rootMatches && dirtySinceIndex -> stale/stale', () => {
      const result = classifyIndexBasis({ ...baseInput, dirtySinceIndex: true })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })

    it('row 4: rootMatches && commit different -> stale/stale', () => {
      const result = classifyIndexBasis({ ...baseInput, headCommit: 'def' })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })

    it('row 5: !rootMatches && indexedCommit == mergeBase -> repo_root/fresh_base', () => {
      const result = classifyIndexBasis({ ...baseInput, rootMatches: false, headCommit: 'def' })
      expect(result).toEqual({ indexScope: 'repo_root', freshness: 'fresh_base' })
    })

    it('row 6: !rootMatches && commit different from mergeBase -> stale/stale', () => {
      const result = classifyIndexBasis({ ...baseInput, rootMatches: false, headCommit: 'def', mergeBase: 'xyz' })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })
    
    it('mergeBase:null ở hàng 5 -> repo_root/unknown', () => {
      const result = classifyIndexBasis({ ...baseInput, rootMatches: false, mergeBase: null })
      expect(result).toEqual({ indexScope: 'repo_root', freshness: 'unknown' })
    })

    it('headCommit:null -> stale/stale nếu rootMatches', () => {
      const result = classifyIndexBasis({ ...baseInput, headCommit: null })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })

    it('headCommit:null -> repo_root/unknown nếu !rootMatches', () => {
      const result = classifyIndexBasis({ ...baseInput, rootMatches: false, headCommit: null })
      expect(result).toEqual({ indexScope: 'repo_root', freshness: 'unknown' })
    })
  })

  describe('CodeGraph', () => {
    const cgBase: IndexBasisInput = {
      ...baseInput,
      tool: 'codegraph',
      indexedCommit: null,
      mergeBase: null
    }

    it('sạch + indexedAt >= headTime -> exact/fresh', () => {
      const result = classifyIndexBasis(cgBase)
      expect(result).toEqual({ indexScope: 'exact', freshness: 'fresh' })
    })

    it('có pending.modified=1 -> stale/stale', () => {
      const result = classifyIndexBasis({ ...cgBase, pendingChanges: { added: 0, modified: 1, removed: 0 } })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })

    it('indexedAt < headTime -> stale/stale', () => {
      const result = classifyIndexBasis({ ...cgBase, indexedAtMs: 800 })
      expect(result).toEqual({ indexScope: 'stale', freshness: 'stale' })
    })

    it('!rootMatches với pendingChanges toàn 0 -> repo_root/unknown', () => {
      const result = classifyIndexBasis({ ...cgBase, rootMatches: false })
      expect(result).toEqual({ indexScope: 'repo_root', freshness: 'unknown' })
    })
  })

  describe('Property: !rootMatches -> never exact', () => {
    it.each([
      ['gitnexus', { ...baseInput, rootMatches: false, tool: 'gitnexus' as const }],
      ['codegraph', { ...baseInput, rootMatches: false, tool: 'codegraph' as const, indexedCommit: null }],
      ['gitnexus dirty', { ...baseInput, rootMatches: false, tool: 'gitnexus' as const, dirtySinceIndex: true }],
    ])('should not be exact for %s', (_, input) => {
      const result = classifyIndexBasis(input)
      expect(result.indexScope).not.toBe('exact')
    })
  })
})
