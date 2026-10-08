import { describe, expect, it } from 'vitest'
import { isAnnotationEligible } from './quality-annotation-eligibility'
import type { QualityAnnotationEligibilityInput } from './quality-annotation-eligibility'

const base: QualityAnnotationEligibilityInput = {
  diffSource: 'unstaged',
  runHeadCommit: 'abc',
  currentHead: 'abc',
  compareHeadOid: 'abc',
  hasWorktreeId: true
}

describe('isAnnotationEligible', () => {
  it('matrix of the 8 DiffSource values (+ worktree) with matching head', () => {
    const expected: Record<string, boolean> = {
      worktree: true,
      unstaged: true,
      staged: false,
      commit: false,
      'combined-commit': false,
      branch: true,
      'combined-branch': true,
      'combined-all': true,
      'combined-uncommitted': false
    }
    for (const [source, eligible] of Object.entries(expected)) {
      const result = isAnnotationEligible({ ...base, diffSource: source as never })
      expect(result.eligible, source).toBe(eligible)
    }
  })

  it('rejects worktree-side sources when HEAD moved or is unknown', () => {
    expect(isAnnotationEligible({ ...base, currentHead: 'zzz' })).toMatchObject({
      eligible: false,
      reason: 'head-mismatch'
    })
    expect(isAnnotationEligible({ ...base, currentHead: undefined }).eligible).toBe(false)
  })

  it('branch sources require compareHeadOid to equal the run head', () => {
    for (const diffSource of ['branch', 'combined-branch'] as const) {
      expect(isAnnotationEligible({ ...base, diffSource, compareHeadOid: 'other' })).toMatchObject({
        eligible: false,
        reason: 'compare-head-mismatch'
      })
      expect(
        isAnnotationEligible({ ...base, diffSource, compareHeadOid: undefined }).eligible
      ).toBe(false)
    }
  })

  it('combined sections are decided by area', () => {
    const all = { ...base, diffSource: 'combined-all' as const }
    expect(isAnnotationEligible({ ...all, sectionArea: 'unstaged' }).eligible).toBe(true)
    expect(isAnnotationEligible({ ...all, sectionArea: 'untracked' }).eligible).toBe(true)
    expect(isAnnotationEligible({ ...all, sectionArea: 'staged' })).toMatchObject({
      eligible: false,
      reason: 'index-side'
    })
    expect(isAnnotationEligible({ ...all, sectionArea: undefined }).eligible).toBe(true)
    expect(
      isAnnotationEligible({ ...all, sectionArea: undefined, compareHeadOid: 'x' }).eligible
    ).toBe(false)
    const unc = { ...base, diffSource: 'combined-uncommitted' as const }
    expect(isAnnotationEligible({ ...unc, sectionArea: 'unstaged' }).eligible).toBe(true)
    expect(isAnnotationEligible({ ...unc, sectionArea: undefined })).toMatchObject({
      eligible: false,
      reason: 'unknown-area'
    })
  })

  it('needs a worktree, a source and a run', () => {
    expect(isAnnotationEligible({ ...base, hasWorktreeId: false }).reason).toBe('no-worktree')
    expect(isAnnotationEligible({ ...base, diffSource: undefined }).reason).toBe('no-source')
    expect(isAnnotationEligible({ ...base, runHeadCommit: null }).reason).toBe('no-run')
  })

  it('reports soft warnings without blocking', () => {
    expect(isAnnotationEligible({ ...base, runDirty: true })).toMatchObject({
      eligible: true,
      softWarning: 'dirty'
    })
    expect(
      isAnnotationEligible({ ...base, runDirty: true, workTreeChangedDuringRun: true }).softWarning
    ).toBe('changed-during-run')
    expect(isAnnotationEligible(base).softWarning).toBeUndefined()
  })
})
