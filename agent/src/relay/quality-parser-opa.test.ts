import { describe, it, expect, vi } from 'vitest'
import { opaParser } from './quality-parser-opa'
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
    cwd: '/opt/repos/backend',
    repoRoot: '/opt/repos/backend',
    platform: 'linux',
    toolVersion: '0.61.0',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-opa', () => {
  it('parses opa test json', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(JSON.stringify([
      {
        location: { file: 'policy_test.rego', row: 10, col: 5 },
        package: 'data.authz',
        name: 'test_deny',
        fail: true
      },
      {
        location: { file: 'policy_test.rego', row: 20, col: 1 },
        package: 'data.authz',
        name: 'test_allow',
        fail: false
      },
      {
        location: { file: 'policy.rego', row: 5, col: 1 },
        package: 'data.authz',
        name: 'compile',
        error: { message: 'Syntax error' }
      }
    ]))
    const input = createMockInput('dummy.txt')
    const result = await opaParser.parse(input)
    
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(2)
    
    expect(result.findings[0]).toEqual({
      ruleId: 'opa/test-failed',
      message: 'Test failed: data.authz.test_deny',
      file: 'policy_test.rego',
      line: 10,
      column: 5,
      severity: 'error'
    })
    
    expect(result.findings[1]).toEqual({
      ruleId: 'opa/compile-error',
      message: 'Syntax error',
      file: 'policy.rego',
      line: 5,
      column: 1,
      severity: 'error'
    })
  })

  it('detects invalid format drift', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(JSON.stringify({ notAnArray: true }))
    const input = createMockInput('dummy.txt')
    const result = await opaParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })
})
