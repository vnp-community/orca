import { describe, it, expect, vi } from 'vitest'
import { oxlintParser } from './quality-parser-oxlint'
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
    toolVersion: '1.87.0',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-oxlint', () => {
  it('parses valid json fixture', async () => {
    const input = createMockInput(path.join(__dirname, '__fixtures__/quality/oxlint/Version__1.87.0__/output.txt'))
    const result = await oxlintParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings.length).toBeGreaterThan(0)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'eslint(no-unused-vars)',
      severity: 'warning',
      file: 'lỗi unicode 🚀.ts',
      line: 1,
      column: 7
    })
    expect(result.stats.scannedFiles).toBe(4)
  })

  it('handles github actions format', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('::warning title=oxlint,file=foo.ts,line=10,col=5::Something went wrong')
    const input = createMockInput('dummy.txt')
    const result = await oxlintParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toEqual({
      ruleId: 'oxlint',
      message: 'Something went wrong',
      file: 'foo.ts',
      line: 10,
      column: 5,
      severity: 'warning'
    })
  })

  it('detects invalid format drift (missing diagnostics)', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(JSON.stringify({ notDiagnostics: [] }))
    const input = createMockInput('dummy.txt')
    const result = await oxlintParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })

  it('detects unparseable json and no github format', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('{ truncated ')
    const input = createMockInput('dummy.txt')
    const result = await oxlintParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })

  it('returns empty when file is empty', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('   \n  ')
    const input = createMockInput('dummy.txt')
    const result = await oxlintParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(0)
  })
})
