import { describe, it, expect, vi } from 'vitest'
import { mapHunksToSymbols } from './codeintel-hunk-symbol-mapping'
import { ChangedFileEntry } from './codeintel-diff-collection'
import { DiffHunk } from './codeintel-diff-hunk-parser'

describe('codeintel-hunk-symbol-mapping', () => {
  const fakeCypher = vi.fn()
  const fakeGit = vi.fn()

  it('maps hunks to innermost method in class and records containers', async () => {
    // Fake commit exists
    fakeGit.mockResolvedValue({ stdout: '', stderr: '' })

    fakeCypher.mockResolvedValue({
      rows: [
        { 'n.filePath': 'src/user.ts', 'n.id': 'Class:src/user.ts:UserService', 'label(n)': 'Class', 'n.name': 'UserService', 'n.startLine': 9, 'n.endLine': 50 },
        { 'n.filePath': 'src/user.ts', 'n.id': 'Method:src/user.ts:UserService.findUser', 'label(n)': 'Method', 'n.name': 'findUser', 'n.startLine': 19, 'n.endLine': 30 }
      ]
    })

    const hunk: DiffHunk = {
      oldStart: 25,
      oldLines: 2,
      newStart: 25,
      newLines: 3,
      startLine: 25,
      endLine: 27,
      pureDeletion: false,
      header: '@@ -25,2 +25,3 @@'
    }

    const changedFiles: ChangedFileEntry[] = [
      {
        path: 'src/user.ts',
        status: 'M',
        additions: 3,
        deletions: 2,
        hunks: [hunk]
      }
    ]

    const res = await mapHunksToSymbols(changedFiles, {
      workspaceRoot: '/test',
      indexedCommit: 'c1',
      headOid: 'c1',
      dirtyFiles: new Set(),
      runCypher: fakeCypher as any,
      execGit: fakeGit as any
    })

    expect(res.mappedSymbols).toHaveLength(1)
    const entry = res.mappedSymbols[0]
    expect(entry.symbol.name).toBe('findUser')
    expect(entry.symbol.kind).toBe('method')
    expect(entry.containers).toContain('UserService')
    expect(entry.confidence).toBe('exact')
  })

  it('prefers function over top-level constant when ranges overlap', async () => {
    fakeGit.mockResolvedValue({ stdout: '', stderr: '' })

    fakeCypher.mockResolvedValue({
      rows: [
        { 'n.filePath': 'src/helper.ts', 'n.id': 'Const:src/helper.ts:MY_CONST', 'label(n)': 'Const', 'n.name': 'MY_CONST', 'n.startLine': 0, 'n.endLine': 99 },
        { 'n.filePath': 'src/helper.ts', 'n.id': 'Function:src/helper.ts:doWork', 'label(n)': 'Function', 'n.name': 'doWork', 'n.startLine': 10, 'n.endLine': 20 }
      ]
    })

    const hunk: DiffHunk = {
      oldStart: 15,
      oldLines: 1,
      newStart: 15,
      newLines: 1,
      startLine: 15,
      endLine: 15,
      pureDeletion: false,
      header: '@@ -15 +15 @@'
    }

    const res = await mapHunksToSymbols([
      { path: 'src/helper.ts', status: 'M', additions: 1, deletions: 1, hunks: [hunk] }
    ], {
      workspaceRoot: '/test',
      indexedCommit: 'c1',
      headOid: 'c1',
      dirtyFiles: new Set(),
      runCypher: fakeCypher as any,
      execGit: fakeGit as any
    })

    expect(res.mappedSymbols).toHaveLength(1)
    expect(res.mappedSymbols[0].symbol.name).toBe('doWork')
  })

  it('marks confidence as approximate when file is dirty or commit is unreachable', async () => {
    // git cat-file fails -> commit unreachable
    fakeGit.mockRejectedValue(new Error('fatal: Not a valid object name'))

    fakeCypher.mockResolvedValue({
      rows: [
        { 'n.filePath': 'src/dirty.ts', 'n.id': 'Function:src/dirty.ts:fn', 'label(n)': 'Function', 'n.name': 'fn', 'n.startLine': 0, 'n.endLine': 10 }
      ]
    })

    const hunk: DiffHunk = {
      oldStart: 5,
      oldLines: 1,
      newStart: 5,
      newLines: 1,
      startLine: 5,
      endLine: 5,
      pureDeletion: false,
      header: '@@ -5 +5 @@'
    }

    const res = await mapHunksToSymbols([
      { path: 'src/dirty.ts', status: 'M', additions: 1, deletions: 1, hunks: [hunk] }
    ], {
      workspaceRoot: '/test',
      indexedCommit: 'old_unreachable',
      headOid: 'head1',
      dirtyFiles: new Set(['src/dirty.ts']),
      runCypher: fakeCypher as any,
      execGit: fakeGit as any
    })

    expect(res.warnings).toContain('index_commit_unreachable')
    expect(res.mappedSymbols[0].confidence).toBe('approximate')
    expect(res.overallConfidence).toBe('approximate')
    expect(res.driftedFileCount).toBe(1)
  })

  it('handles deleted files by marking symbols as deleted with approximate confidence', async () => {
    fakeGit.mockResolvedValue({ stdout: '', stderr: '' })

    fakeCypher.mockResolvedValue({
      rows: [
        { 'n.filePath': 'src/deleted.ts', 'n.id': 'Function:src/deleted.ts:oldFn', 'label(n)': 'Function', 'n.name': 'oldFn', 'n.startLine': 0, 'n.endLine': 10 }
      ]
    })

    const res = await mapHunksToSymbols([
      { path: 'src/deleted.ts', status: 'D', additions: 0, deletions: 10, hunks: [] }
    ], {
      workspaceRoot: '/test',
      indexedCommit: 'c1',
      headOid: 'c1',
      dirtyFiles: new Set(),
      runCypher: fakeCypher as any,
      execGit: fakeGit as any
    })

    expect(res.mappedSymbols).toHaveLength(1)
    expect(res.mappedSymbols[0].status).toBe('deleted')
    expect(res.mappedSymbols[0].confidence).toBe('approximate')
  })

  it('does not invent symbols for unindexed code or files without symbols', async () => {
    fakeGit.mockResolvedValue({ stdout: '', stderr: '' })

    // Return empty for unindexed.ts
    fakeCypher.mockResolvedValue({ rows: [] })

    const hunk: DiffHunk = {
      oldStart: 1,
      oldLines: 1,
      newStart: 1,
      newLines: 1,
      startLine: 1,
      endLine: 1,
      pureDeletion: false,
      header: '@@ -1 +1 @@'
    }

    const res = await mapHunksToSymbols([
      { path: 'src/unindexed.ts', status: 'A', additions: 1, deletions: 0, hunks: [hunk] }
    ], {
      workspaceRoot: '/test',
      indexedCommit: 'c1',
      headOid: 'c1',
      dirtyFiles: new Set(),
      runCypher: fakeCypher as any,
      execGit: fakeGit as any
    })

    expect(res.mappedSymbols).toHaveLength(0)
    expect(res.unmapped.filesNotIndexed).toContain('src/unindexed.ts')
  })
})
