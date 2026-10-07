import { describe, it, expect, vi } from 'vitest'
import { tscParser } from './quality-parser-tsc'
import path from 'path'
import fs from 'fs/promises'
import { QualityParserInput } from './quality-parser-types'

function createMockInput(stdoutPath: string): QualityParserInput {
  return {
    stepId: 'step1',
    stdoutPath,
    stderrPath: '',
    exitCode: 1,
    timedOut: false,
    cancelled: false,
    cwd: '/Users/binhnt/Work/blockchain/vnp-blc/orca/agent/src/relay/__fixtures__/quality-mini-repo',
    repoRoot: '/Users/binhnt/Work/blockchain/vnp-blc/orca/agent/src/relay/__fixtures__/quality-mini-repo',
    platform: 'linux',
    toolVersion: '5.9.3',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-tsc', () => {
  it('parses valid text fixture', async () => {
    const input = createMockInput(path.join(__dirname, '__fixtures__/quality/tsc/Version_5.9.3_/output.txt'))
    const result = await tscParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings.length).toBeGreaterThan(0)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'TS2792',
      severity: 'error',
      file: 'broken-import.test.ts',
      line: 1,
      column: 29
    })
  })

  it('handles indented multiline messages', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('index.ts(1,1): error TS1234: some error\n  line2\n  line3')
    const input = createMockInput('dummy.txt')
    const result = await tscParser.parse(input)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].message).toBe('some error\nline2\nline3')
  })

  it('handles global errors without file', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('error TS5023: Unknown compiler option')
    const input = createMockInput('dummy.txt')
    const result = await tscParser.parse(input)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0].file).toBe('')
    expect(result.findings[0].line).toBe(1)
    expect(result.findings[0].column).toBe(1)
  })

  it('detects unparseable format drift', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('This is just some random text')
    const input = createMockInput('dummy.txt')
    const result = await tscParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })
})
