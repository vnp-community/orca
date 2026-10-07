import { describe, it, expect } from 'vitest'
import { matchRule } from './quality-rule-diff-matcher'
import { Rule } from './quality-rule-pack-schema'
import { ChangedFileDiff } from './quality-diff-changed-lines'

describe('quality-rule-diff-matcher', () => {
  it('matches added line regex', () => {
    const rule: Rule = {
      id: 'TEST-001',
      title: 'Test',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/*'], exclude: ['**/*.test.*'], fileStatus: ['added', 'modified'] },
      match: { type: 'added-line-regex', pattern: 'bad_word', maxLineLength: 100 },
      message: 'Found bad word',
      fixHint: '',
      source: { doc: '' }
    }

    const files: ChangedFileDiff[] = [
      {
        path: 'src/good.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'good word' }, { line: 2, text: 'bad_word' }]
      },
      {
        path: 'src/bad.test.ts', // excluded
        status: 'A',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'bad_word' }]
      }
    ]

    const deps = {
      now: () => Date.now(),
      budgetMsPerLine: 5,
      readFile: () => ''
    }

    const res = matchRule(rule, files, deps)
    expect(res.findings).toHaveLength(1)
    expect(res.findings[0].file).toBe('src/good.ts')
    expect(res.findings[0].line).toBe(2)
  })

  it('matches added file name', () => {
    const rule: Rule = {
      id: 'TEST-002',
      title: 'Test',
      kind: 'diff',
      category: 'convention',
      severity: 'warning',
      enabled: true,
      scope: { include: ['**/*'], exclude: [], fileStatus: ['added'] },
      match: { type: 'added-file-name', pattern: '\\.d\\.ts$', maxLineLength: 100 },
      message: 'No new d.ts',
      fixHint: '',
      source: { doc: '' }
    }

    const files: ChangedFileDiff[] = [
      { path: 'types.d.ts', status: 'A', untracked: false, binary: false, addedLines: [] },
      { path: 'old.d.ts', status: 'M', untracked: false, binary: false, addedLines: [] } // modified, should be ignored
    ]

    const deps = { now: () => Date.now(), budgetMsPerLine: 5, readFile: () => '' }
    const res = matchRule(rule, files, deps)

    expect(res.findings).toHaveLength(1)
    expect(res.findings[0].file).toBe('types.d.ts')
    expect(res.findings[0].line).toBeUndefined()
  })

  it('skips long lines', () => {
    const rule: Rule = {
      id: 'TEST-003',
      title: 'Test',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/*'], exclude: [], fileStatus: ['modified'] },
      match: { type: 'added-line-regex', pattern: 'test', maxLineLength: 10 },
      message: 'test',
      fixHint: '',
      source: { doc: '' }
    }

    const files: ChangedFileDiff[] = [
      {
        path: 'test.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'this is a very long line with test inside' }]
      }
    ]
    const deps = { now: () => Date.now(), budgetMsPerLine: 5, readFile: () => '' }
    const res = matchRule(rule, files, deps)
    
    expect(res.findings).toHaveLength(0)
    expect(res.skippedLongLines).toBe(1)
  })

  it('truncates on regex timeout', () => {
    const rule: Rule = {
      id: 'TEST-004',
      title: 'Test',
      kind: 'diff',
      category: 'convention',
      severity: 'error',
      enabled: true,
      scope: { include: ['**/*'], exclude: [], fileStatus: ['modified'] },
      match: { type: 'added-line-regex', pattern: 'a', maxLineLength: 1000 },
      message: 'test',
      fixHint: '',
      source: { doc: '' }
    }

    const files: ChangedFileDiff[] = [
      {
        path: 'test.ts',
        status: 'M',
        untracked: false,
        binary: false,
        addedLines: [{ line: 1, text: 'a'.repeat(100) + 'c' }, { line: 2, text: 'ab' }] // will timeout on first
      }
    ]

    let time = 0
    const deps = { 
      now: () => { time += 10; return time }, // simulate 10ms passed
      budgetMsPerLine: 5, 
      readFile: () => '' 
    }
    const res = matchRule(rule, files, deps)
    
    expect(res.truncated).toBe(true)
    expect(res.findings).toHaveLength(0) // skips second line because it timed out
  })
})
