import { beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({
  s: {} as Record<string, unknown>
}))
vi.mock('@/store', () => ({ useAppStore: { getState: () => state.s } }))
vi.mock('@/store/slices/worktree-helpers', () => ({
  findWorktreeById: (_m: unknown, id: string) => (id === 'wt' ? { path: '/repo/wt' } : null)
}))
vi.mock('@/lib/worktree-activation', () => ({ activateAndRevealWorktree: vi.fn() }))
vi.mock('@/lib/language-detect', () => ({ detectLanguage: () => 'typescript' }))

import { openReviewDiffAtSymbol, openReviewFileInEditor } from './review-diff-navigation'

const summary = { status: 'ready', baseRef: 'main', baseOid: 'b', compareRef: 'h', headOid: 'h', mergeBase: 'm' }
let frames: (() => void)[]

function setup(over: Record<string, unknown> = {}): void {
  state.s = {
    worktreesByRepo: {},
    gitBranchChangesByWorktree: {},
    gitBranchCompareSummaryByWorktree: { wt: summary },
    gitStatusByWorktree: {},
    activeFileId: 'file-1',
    openBranchDiff: vi.fn(),
    openDiff: vi.fn(),
    openFile: vi.fn(),
    setPendingDiffReveal: vi.fn(),
    setPendingEditorReveal: vi.fn(),
    ...over
  }
}
const flushFrames = (): void => {
  while (frames.length) {
    frames.shift()!()
  }
}
beforeEach(() => {
  frames = []
  vi.stubGlobal('requestAnimationFrame', (cb: () => void) => frames.push(cb))
  vi.stubGlobal('cancelAnimationFrame', () => {})
  setup()
})
const s = () => state.s as Record<string, ReturnType<typeof vi.fn>>
const branchScope = { kind: 'branch', baseRef: 'main', includeUncommitted: true } as const

describe('openReviewDiffAtSymbol', () => {
  it('uses the branch diff when the file is in the branch changes, revealing after two frames', () => {
    setup({ gitBranchChangesByWorktree: { wt: [{ path: 'src/a.ts', status: 'modified' }] } })
    const r = openReviewDiffAtSymbol('wt', { filePath: 'src/a.ts', startLine: 12 }, branchScope)
    expect(r).toEqual({ ok: true, opened: 'branch-diff', notInChanges: false })
    expect(s().openBranchDiff).toHaveBeenCalled()
    expect(s().setPendingDiffReveal).not.toHaveBeenCalled()
    frames.shift()!()
    expect(s().setPendingDiffReveal).not.toHaveBeenCalled()
    flushFrames()
    expect(s().setPendingDiffReveal).toHaveBeenCalledWith(
      expect.objectContaining({ fileId: 'file-1', line: 12, side: 'modified' })
    )
  })
  it('uses the working diff for uncommitted-only files', () => {
    setup({ gitStatusByWorktree: { wt: [{ path: 'src/a.ts', status: 'modified' }] } })
    const r = openReviewDiffAtSymbol('wt', { filePath: 'src/a.ts' }, branchScope, { line: 3 })
    expect(r).toMatchObject({ ok: true, opened: 'working-diff' })
    expect(s().openDiff).toHaveBeenCalledWith('wt', '/repo/wt/src/a.ts', 'src/a.ts', 'typescript', false)
    flushFrames()
    expect(s().setPendingDiffReveal).toHaveBeenCalledWith(expect.objectContaining({ line: 3 }))
  })
  it('falls back to the editor with a note when the file is not in the changes', () => {
    const r = openReviewDiffAtSymbol('wt', { filePath: 'src/a.ts', startLine: 5 }, branchScope)
    expect(r).toEqual({ ok: true, opened: 'file', notInChanges: true })
    flushFrames()
    expect(s().setPendingEditorReveal).toHaveBeenCalledWith(expect.objectContaining({ line: 5 }))
  })
  it('range scope without any change entry is reported, not guessed', () => {
    const r = openReviewDiffAtSymbol(
      'wt',
      { filePath: 'src/a.ts' },
      { kind: 'range', baseCommit: 'a', headCommit: 'b' }
    )
    expect(r).toEqual({ ok: false, reason: 'scope-unsupported' })
  })
  it.each(['../x.ts', '/etc/passwd', 'C:\\x.ts', 'a/../../x.ts'])(
    'blocks escaping path %s',
    (p) => {
      expect(openReviewDiffAtSymbol('wt', { filePath: p }, branchScope)).toEqual({
        ok: false,
        reason: 'path-not-allowed'
      })
      expect(s().openFile).not.toHaveBeenCalled()
    }
  )
  it('unknown worktree', () => {
    expect(openReviewDiffAtSymbol('nope', { filePath: 'a.ts' }, null)).toEqual({
      ok: false,
      reason: 'worktree-not-found'
    })
  })
})

describe('openReviewFileInEditor', () => {
  it('opens in edit mode and reveals after two frames; blocks escapes', () => {
    expect(openReviewFileInEditor('wt', { filePath: 'src/a.ts' }, { line: 9 })).toMatchObject({ ok: true })
    expect(s().openFile).toHaveBeenCalled()
    flushFrames()
    expect(s().setPendingEditorReveal).toHaveBeenLastCalledWith(expect.objectContaining({ line: 9 }))
    expect(openReviewFileInEditor('wt', { filePath: '../x' })).toEqual({
      ok: false,
      reason: 'path-not-allowed'
    })
  })
})
