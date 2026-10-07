import { describe, it, expect } from 'vitest'
import {
  captureSnapshot,
  diffSnapshots,
  WorkspaceSnapshot,
  SnapshotEntry
} from './agent-worktree-change-snapshot'

describe('agent-worktree-change-snapshot', () => {
  describe('diffSnapshots (git mode)', () => {
    it('reports added, modified, deleted, untracked and renamed files', () => {
      const before: WorkspaceSnapshot = {
        kind: 'git',
        head: 'commit-1',
        entries: new Map<string, SnapshotEntry>([
          ['file1.ts', { status: ' M', size: 100, mtimeMs: 1000 }],
          ['deleted.ts', { status: '  ', size: 200, mtimeMs: 1000 }]
        ])
      }

      const after: WorkspaceSnapshot = {
        kind: 'git',
        head: 'commit-1',
        entries: new Map<string, SnapshotEntry>([
          // modified: signature changed
          ['file1.ts', { status: ' M', size: 150, mtimeMs: 2000 }],
          // deleted
          ['deleted.ts', { status: ' D', size: null, mtimeMs: null }],
          // added
          ['new.ts', { status: 'A ', size: 50, mtimeMs: 3000 }],
          // untracked
          ['untracked.ts', { status: '??', size: 80, mtimeMs: 3000 }],
          // renamed
          ['renamed.ts', { status: 'R ', size: 100, mtimeMs: 3000 }]
        ])
      }

      const report = diffSnapshots(before, after)
      expect(report.available).toBe(true)
      if (report.available) {
        expect(report.headMoved).toBe(false)
        expect(report.headBefore).toBe('commit-1')
        expect(report.headAfter).toBe('commit-1')

        const paths = report.changedFiles.map((c) => ({ path: c.path, change: c.change }))
        expect(paths).toContainEqual({ path: 'file1.ts', change: 'modified' })
        expect(paths).toContainEqual({ path: 'deleted.ts', change: 'deleted' })
        expect(paths).toContainEqual({ path: 'new.ts', change: 'added' })
        expect(paths).toContainEqual({ path: 'untracked.ts', change: 'untracked' })
        expect(paths).toContainEqual({ path: 'renamed.ts', change: 'renamed' })
      }
    })

    it('reports headMoved when head changes', () => {
      const before: WorkspaceSnapshot = {
        kind: 'git',
        head: 'commit-1',
        entries: new Map()
      }
      const after: WorkspaceSnapshot = {
        kind: 'git',
        head: 'commit-2',
        entries: new Map()
      }

      const report = diffSnapshots(before, after)
      expect(report.available).toBe(true)
      if (report.available) {
        expect(report.headMoved).toBe(true)
        expect(report.headBefore).toBe('commit-1')
        expect(report.headAfter).toBe('commit-2')
      }
    })

    it('returns available: false when a snapshot is unavailable', () => {
      const before: WorkspaceSnapshot = { kind: 'unavailable', reason: 'NOT_A_GIT_REPO' }
      const after: WorkspaceSnapshot = { kind: 'git', head: null, entries: new Map() }

      const report = diffSnapshots(before, after)
      expect(report.available).toBe(false)
      if (!report.available) {
        expect(report.reason).toBe('NOT_A_GIT_REPO')
      }
    })

    it('caps changedFiles at 2000 and sets truncated', () => {
      const before: WorkspaceSnapshot = { kind: 'git', head: 'c1', entries: new Map() }
      const afterEntries = new Map<string, SnapshotEntry>()
      for (let i = 0; i < 2500; i++) {
        afterEntries.set(`file-${i}.ts`, { status: '??', size: 10, mtimeMs: 100 })
      }
      const after: WorkspaceSnapshot = { kind: 'git', head: 'c1', entries: afterEntries }

      const report = diffSnapshots(before, after)
      expect(report.available).toBe(true)
      if (report.available) {
        expect(report.changedFiles.length).toBe(2000)
        expect(report.truncated).toBe(true)
      }
    })
  })

  describe('captureSnapshot with mock deps', () => {
    it('returns git snapshot with parsed head and entries', async () => {
      const mockRunGit = async (args: string[]) => {
        if (args.includes('--is-inside-work-tree')) {
          return { stdout: 'true\n', code: 0 }
        }
        if (args.includes('HEAD')) {
          return { stdout: 'commit-abc\n', code: 0 }
        }
        if (args.includes('status')) {
          // git status -z output: "XY path\0"
          return { stdout: ' M file1.ts\0?? file2.ts\0', code: 0 }
        }
        return { stdout: '', code: 0 }
      }

      const mockStat = async () => ({ size: 100, mtimeMs: 500 })

      const snap = await captureSnapshot('/mock/repo', 'git', {
        runGit: mockRunGit,
        statFile: mockStat
      })

      expect(snap.kind).toBe('git')
      if (snap.kind === 'git') {
        expect(snap.head).toBe('commit-abc')
        expect(snap.entries.has('file1.ts')).toBe(true)
        expect(snap.entries.has('file2.ts')).toBe(true)
      }
    })

    it('returns unavailable when not a git repo', async () => {
      const mockRunGit = async () => {
        return { stdout: 'fatal: not a git repository\n', code: 128 }
      }

      const snap = await captureSnapshot('/non/git', 'git', {
        runGit: mockRunGit
      })

      expect(snap.kind).toBe('unavailable')
      if (snap.kind === 'unavailable') {
        expect(snap.reason).toBe('NOT_A_GIT_REPO')
      }
    })
  })
})
