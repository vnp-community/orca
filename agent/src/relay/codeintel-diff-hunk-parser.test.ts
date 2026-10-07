import { describe, it, expect } from 'vitest'
import {
  parseRawZ,
  parseNumstatZ,
  parseUnifiedZero,
  matchHunksToFiles
} from './codeintel-diff-hunk-parser'

describe('codeintel-diff-hunk-parser', () => {
  describe('parseRawZ', () => {
    it('parses standard modified files', () => {
      const raw = ':100644 100644 e69de29 d95f3ad M\0src/foo.ts\0'
      const entries = parseRawZ(raw)
      expect(entries).toHaveLength(1)
      expect(entries[0]).toEqual({
        oldMode: '100644',
        newMode: '100644',
        oldSha: 'e69de29',
        newSha: 'd95f3ad',
        status: 'M',
        score: undefined,
        path: 'src/foo.ts',
        oldPath: undefined,
        unsafePath: undefined
      })
    })

    it('parses renamed and copied files with score', () => {
      const raw = ':100644 100644 e69de29 d95f3ad R100\0src/old.ts\0src/new.ts\0'
      const entries = parseRawZ(raw)
      expect(entries).toHaveLength(1)
      expect(entries[0]).toEqual({
        oldMode: '100644',
        newMode: '100644',
        oldSha: 'e69de29',
        newSha: 'd95f3ad',
        status: 'R',
        score: 100,
        oldPath: 'src/old.ts',
        path: 'src/new.ts',
        unsafePath: undefined
      })
    })

    it('detects control characters as unsafePath', () => {
      const raw = ':100644 100644 e69de29 d95f3ad M\0src/\x07bell.ts\0'
      const entries = parseRawZ(raw)
      expect(entries).toHaveLength(1)
      expect(entries[0].unsafePath).toBe(true)
    })

    it('handles unicode and spaces without corrupting path', () => {
      const raw = ':100644 100644 e69de29 d95f3ad A\0src/tài liệu test.ts\0'
      const entries = parseRawZ(raw)
      expect(entries).toHaveLength(1)
      expect(entries[0].path).toBe('src/tài liệu test.ts')
      expect(entries[0].status).toBe('A')
      expect(entries[0].unsafePath).toBeUndefined()
    })
  })

  describe('parseNumstatZ', () => {
    it('parses normal additions and deletions', () => {
      const numstat = '12\t5\tsrc/foo.ts\0'
      const entries = parseNumstatZ(numstat)
      expect(entries).toHaveLength(1)
      expect(entries[0]).toEqual({
        additions: 12,
        deletions: 5,
        binary: false,
        path: 'src/foo.ts',
        oldPath: undefined
      })
    })

    it('parses binary files represented by - -', () => {
      const numstat = '-\t-\timage.png\0'
      const entries = parseNumstatZ(numstat)
      expect(entries).toHaveLength(1)
      expect(entries[0]).toEqual({
        additions: 0,
        deletions: 0,
        binary: true,
        path: 'image.png',
        oldPath: undefined
      })
    })

    it('parses renamed entries in numstat -z', () => {
      const numstat = '3\t2\t\0old-file.ts\0new-file.ts\0'
      const entries = parseNumstatZ(numstat)
      expect(entries).toHaveLength(1)
      expect(entries[0]).toEqual({
        additions: 3,
        deletions: 2,
        binary: false,
        path: 'new-file.ts',
        oldPath: 'old-file.ts'
      })
    })
  })

  describe('parseUnifiedZero', () => {
    it('parses hunks with single line shorthand @@ -83 +83 @@', () => {
      const diff = `diff --git a/src/foo.ts b/src/foo.ts
--- a/src/foo.ts
+++ b/src/foo.ts
@@ -83 +83 @@
-const x = 1
+const x = 2
`
      const blocks = parseUnifiedZero(diff)
      expect(blocks).toHaveLength(1)
      expect(blocks[0].newPath).toBe('src/foo.ts')
      expect(blocks[0].hunks).toHaveLength(1)
      expect(blocks[0].hunks[0]).toEqual({
        oldStart: 83,
        oldLines: 1,
        newStart: 83,
        newLines: 1,
        startLine: 83,
        endLine: 83,
        pureDeletion: false,
        header: '@@ -83 +83 @@'
      })
    })

    it('handles pure deletion where +c,0', () => {
      const diff = `diff --git a/src/foo.ts b/src/foo.ts
--- a/src/foo.ts
+++ b/src/foo.ts
@@ -10,5 +10,0 @@
-line1
-line2
-line3
-line4
-line5
`
      const blocks = parseUnifiedZero(diff)
      expect(blocks[0].hunks[0]).toEqual({
        oldStart: 10,
        oldLines: 5,
        newStart: 10,
        newLines: 0,
        startLine: 10,
        endLine: 10,
        pureDeletion: true,
        header: '@@ -10,5 +10,0 @@'
      })
    })

    it('handles c=0 pure deletion gracefully by setting startLine=1', () => {
      const diff = `diff --git a/src/foo.ts b/src/foo.ts
--- a/src/foo.ts
+++ b/src/foo.ts
@@ -1,5 +0,0 @@
`
      const blocks = parseUnifiedZero(diff)
      expect(blocks[0].hunks[0].pureDeletion).toBe(true)
      expect(blocks[0].hunks[0].startLine).toBe(1)
      expect(blocks[0].hunks[0].endLine).toBe(1)
    })
  })

  describe('matchHunksToFiles', () => {
    it('matches hunks to corresponding files correctly', () => {
      const raw = parseRawZ(':100644 100644 a b M\0file1.ts\0:100644 100644 a b M\0file2.ts\0')
      const blocks = parseUnifiedZero(`diff --git a/file1.ts b/file1.ts
--- a/file1.ts
+++ b/file1.ts
@@ -1,2 +1,2 @@
diff --git a/file2.ts b/file2.ts
--- a/file2.ts
+++ b/file2.ts
@@ -10 +10 @@
`)
      const res = matchHunksToFiles(raw, blocks)
      expect(res.warnings).toHaveLength(0)
      expect(res.fileHunks.get('file1.ts')).toHaveLength(1)
      expect(res.fileHunks.get('file2.ts')).toHaveLength(1)
    })

    it('detects hunk_file_order_mismatch when hunk order differs from raw entries', () => {
      const raw = parseRawZ(':100644 100644 a b M\0file1.ts\0:100644 100644 a b M\0file2.ts\0')
      const blocks = parseUnifiedZero(`diff --git a/file2.ts b/file2.ts
--- a/file2.ts
+++ b/file2.ts
@@ -10 +10 @@
diff --git a/file1.ts b/file1.ts
--- a/file1.ts
+++ b/file1.ts
@@ -1,2 +1,2 @@
`)
      const res = matchHunksToFiles(raw, blocks)
      expect(res.warnings).toContain('hunk_file_order_mismatch')
      expect(res.fileHunks.get('file1.ts')).toHaveLength(1)
      expect(res.fileHunks.get('file2.ts')).toHaveLength(1)
    })
  })
})
