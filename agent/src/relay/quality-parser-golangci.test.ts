import { describe, it, expect, vi } from 'vitest'
import { golangciParser } from './quality-parser-golangci'
import { getGolangciSeverity } from './quality-parser-golangci-severity'
import fs from 'fs/promises'
import path from 'path'
import { QualityParserInput } from './quality-parser-types'

function createMockInput(stdoutPath: string, version: string = '1.55.2'): QualityParserInput {
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
    toolVersion: version,
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-golangci', () => {
  it('rejects unsupported major version', async () => {
    const input = createMockInput('dummy.txt', '2.0.0')
    const result = await golangciParser.parse(input)
    expect(result.failure?.kind).toBe('env')
    expect(result.failure?.envReason).toBe('tool_incompatible')
  })

  it('parses valid json format', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(JSON.stringify({
      Issues: [
        {
          FromLinter: 'typecheck',
          Text: 'some error',
          Pos: { Filename: 'main.go', Line: 10, Column: 5 }
        },
        {
          FromLinter: 'ineffassign',
          Text: 'ineffective assignment',
          Pos: { Filename: 'main.go', Line: 15, Column: 2 },
          Replacement: 'foo'
        }
      ]
    }))
    const input = createMockInput('dummy.txt')
    const result = await golangciParser.parse(input)
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(2)
    
    expect(result.findings[0].ruleId).toBe('golangci-lint/typecheck')
    expect(result.findings[0].severity).toBe('error')
    expect(result.findings[0].evidence).toBeUndefined()
    
    expect(result.findings[1].ruleId).toBe('golangci-lint/ineffassign')
    expect(result.findings[1].severity).toBe('warning')
    expect(result.findings[1].evidence).toEqual({ fixHint: 'foo' })
  })

  it('detects invalid format drift', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('invalid json')
    const input = createMockInput('dummy.txt')
    const result = await golangciParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })

  it('verifies that configured linters are mapped to severity', async () => {
    // Read the actual backend-go/.golangci.yml
    const yamlContent = await fs.readFile(path.join(__dirname, '../../../backend-go/.golangci.yml'), 'utf8')
    const linters = yamlContent.split('\\n')
      .map(line => line.trim())
      .filter(line => line.startsWith('- '))
      .map(line => line.substring(2))

    // Just verify the parser doesn't crash on them, and they return some severity
    for (const linter of linters) {
      if (!linter || linter.startsWith('#')) continue
      const severity = getGolangciSeverity(linter)
      expect(['error', 'warning', 'info']).toContain(severity)
    }
  })
})
