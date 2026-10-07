import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  normalizeAnchor,
  normalizeMessage,
  assignOccurrences,
  computeFingerprint,
  createSourceLineReader
} from './quality-finding-fingerprint'
import fs from 'fs/promises'
import path from 'path'
import os from 'os'

describe('quality-finding-fingerprint', () => {
  it('normalizeAnchor', () => {
    expect(normalizeAnchor(null)).toBe('')
    expect(normalizeAnchor('  hello   world  ')).toBe('hello world')
    // test truncation to 512
    const longString = 'a'.repeat(600)
    expect(normalizeAnchor(longString)).toHaveLength(512)
    // test NFC
    expect(normalizeAnchor('e\u0301')).toBe('\u00e9')
  })

  it('normalizeMessage', () => {
    const ctx = { repoRoot: '/repo', home: '/home/user', tmp: '/tmp' }
    
    // path replacements
    expect(normalizeMessage('/repo/src/main.ts failed', ctx)).toBe('<repo>/src/main.ts failed')
    expect(normalizeMessage('/home/user/cache/foo', ctx)).toBe('~/cache/foo')
    
    // line col replacements
    expect(normalizeMessage('error at file.go:10:5', ctx)).toBe('error at file.go:<loc>')
    expect(normalizeMessage('error at (10, 5)', ctx)).toBe('error at <loc>')
    expect(normalizeMessage('error on line 25', ctx)).toBe('error on line <loc>')
    
    // times and dates
    expect(normalizeMessage('took 1.5ms to run', ctx)).toBe('took <time> to run')
    expect(normalizeMessage('date 2026-10-06T15:00:00Z', ctx)).toBe('date <date>')
    
    // formatting
    expect(normalizeMessage('   too    many   spaces   ', ctx)).toBe('too many spaces')
  })

  it('assignOccurrences and computeFingerprint', () => {
    const baseItem = {
      tool: 'oxlint',
      ruleId: 'rule1',
      file: 'main.ts',
      anchor: 'console.log',
      normMessage: 'msg',
      severity: 'error' as const,
      category: 'lint' as const,
      printedPath: 'main.ts',
      endLine: 1,
      endColumn: 5
    }

    const items = [
      { ...baseItem, line: 10, column: 5 },
      { ...baseItem, line: 5, column: 1 },
      { ...baseItem, line: 5, column: 10 } // same line, different column
    ]

    const withOccurrences = assignOccurrences(items)
    
    // Sorted by line/col:
    // [0] line 5, col 1 -> occ 1
    // [1] line 5, col 10 -> occ 2
    // [2] line 10, col 5 -> occ 3
    expect(withOccurrences[0].occurrence).toBe(1)
    expect(withOccurrences[1].occurrence).toBe(2)
    expect(withOccurrences[2].occurrence).toBe(3)

    // Compute fingerprint
    const fp1 = computeFingerprint(withOccurrences[0], withOccurrences[0].occurrence)
    const fp2 = computeFingerprint(withOccurrences[1], withOccurrences[1].occurrence)
    
    expect(fp1.fingerprint).not.toBe(fp2.fingerprint)
    expect(fp1.fingerprint).toHaveLength(32)
    expect(fp1.fpVersion).toBe(1)
  })

  describe('sourceLineReader', () => {
    let tempDir: string

    beforeEach(async () => {
      tempDir = await fs.realpath(await fs.mkdtemp(path.join(os.tmpdir(), 'fingerprint-')))
    })

    afterEach(async () => {
      await fs.rm(tempDir, { recursive: true, force: true })
    })

    it('reads and caches lines correctly', async () => {
      await fs.writeFile(path.join(tempDir, 'test.txt'), `line1\nline2\nline3`)
      const reader = createSourceLineReader(tempDir)
      
      expect(await reader('test.txt', 2)).toBe('line2')
      expect(await reader('test.txt', 4)).toBeNull()
      expect(await reader('nonexistent.txt', 1)).toBeNull()
      
      // out of bounds
      expect(await reader('../outside.txt', 1)).toBeNull()
    })

    it('returns null for binary files', async () => {
      const buf = Buffer.from([0, 1, 2, 3])
      await fs.writeFile(path.join(tempDir, 'binary.bin'), buf)
      
      const reader = createSourceLineReader(tempDir)
      expect(await reader('binary.bin', 1)).toBeNull()
    })
  })
})
