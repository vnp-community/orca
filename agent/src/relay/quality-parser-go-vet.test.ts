import { describe, it, expect, vi } from 'vitest'
import { goVetParser } from './quality-parser-go-vet'
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
    toolVersion: '1.22.0',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-go-vet', () => {
  it('parses valid json format', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '{"main": {"printf": [{"posn": "main.go:10:2", "message": "Printf format %s reads arg of wrong type int"}]}}\n# main'
    )
    const input = createMockInput('dummy.txt')
    const result = await goVetParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toEqual({
      ruleId: 'go-vet/printf',
      message: 'Printf format %s reads arg of wrong type int',
      file: 'main.go',
      line: 10,
      column: 2,
      severity: 'error'
    })
  })

  it('parses text fallback for build errors', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('main.go:5:1: expected declaration, found string')
    const input = createMockInput('dummy.txt')
    const result = await goVetParser.parse(input)
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'go-vet/build-failed',
      message: 'expected declaration, found string',
      file: 'main.go',
      line: 5,
      column: 1,
      severity: 'error'
    })
  })

  it('handles posn without column', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '{"main": {"test": [{"posn": "main.go:15", "message": "Something"}]}}'
    )
    const input = createMockInput('dummy.txt')
    const result = await goVetParser.parse(input)
    expect(result.findings[0].line).toBe(15)
    expect(result.findings[0].column).toBe(1)
  })

  it('ignores paths outside repo', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '{"main": {"test": [{"posn": "/tmp/outside.go:15:2", "message": "Something"}]}}'
    )
    const input = createMockInput('dummy.txt')
    const result = await goVetParser.parse(input)
    expect(result.findings).toHaveLength(0)
  })
})
