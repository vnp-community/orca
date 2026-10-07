import { describe, it, expect, vi } from 'vitest'
import { vitestParser } from './quality-parser-vitest'
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
    toolVersion: '1.6.1',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-vitest', () => {
  it('parses valid json fixture', async () => {
    // The fixture path contains vitest_1.6.1_darwin-arm64_node
    // But inside the JSON it uses absolute paths we need to handle or mock.
    // Let's just use the raw fixture, but since it has <cwd> injected maybe we should mock the file content.
    const fixtureContent = JSON.stringify({
      testResults: [
        {
          name: '/Users/binhnt/Work/blockchain/vnp-blc/orca/agent/src/relay/__fixtures__/quality-mini-repo/math.test.ts',
          status: 'failed',
          assertionResults: [
            {
              ancestorTitles: ['', 'math'],
              title: 'fails',
              status: 'failed',
              failureMessages: ['expected 1 to be 2'],
              location: { line: 3, column: 13 }
            },
            {
              ancestorTitles: ['', 'math'],
              title: 'skipped-test',
              status: 'skipped'
            }
          ]
        },
        {
          name: '/Users/binhnt/Work/blockchain/vnp-blc/orca/agent/src/relay/__fixtures__/quality-mini-repo/broken.test.ts',
          status: 'failed',
          message: 'SyntaxError: unexpected token',
          assertionResults: []
        }
      ]
    })
    
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(fixtureContent)
    const input = createMockInput('dummy.txt')
    const result = await vitestParser.parse(input)
    
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(2)
    
    expect(result.findings[0]).toMatchObject({
      ruleId: 'vitest/test-failed',
      file: 'math.test.ts',
      line: 3,
      column: 13,
      message: 'math › fails\nexpected 1 to be 2',
      severity: 'error'
    })
    
    expect(result.findings[1]).toMatchObject({
      ruleId: 'vitest/suite-failed',
      file: 'broken.test.ts',
      line: 1,
      column: 1,
      message: 'SyntaxError: unexpected token',
      severity: 'error'
    })
  })

  it('detects invalid format drift', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(JSON.stringify({ notTestResults: [] }))
    const input = createMockInput('dummy.txt')
    const result = await vitestParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })
})
