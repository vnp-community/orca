import { describe, it, expect, vi } from 'vitest'
import { bufLintParser, bufBreakingParser } from './quality-parser-buf'
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
    toolVersion: '1.30.0',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-buf', () => {
  it('parses buf lint valid json', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '{"path":"hello.proto","start_line":1,"start_column":1,"end_line":1,"end_column":10,"type":"FILE_LOWER_SNAKE_CASE","message":"Filename \\"hello.proto\\" should be lower_snake_case.proto."}'
    )
    const input = createMockInput('dummy.txt')
    const result = await bufLintParser.parse(input)
    
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toEqual({
      ruleId: 'buf/FILE_LOWER_SNAKE_CASE',
      message: 'Filename "hello.proto" should be lower_snake_case.proto.',
      file: 'hello.proto',
      line: 1,
      column: 1,
      endLine: 1,
      endColumn: 10,
      severity: 'warning'
    })
  })

  it('parses buf breaking valid json', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '{"path":"api/v1/user.proto","start_line":15,"start_column":3,"end_line":15,"end_column":20,"type":"FIELD_NO_DELETE","message":"Field \\"email\\" was deleted."}'
    )
    const input = createMockInput('dummy.txt')
    const result = await bufBreakingParser.parse(input)
    
    expect(result.failure).toBeNull()
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toEqual({
      ruleId: 'buf-breaking/FIELD_NO_DELETE',
      message: 'Field "email" was deleted.',
      file: 'api/v1/user.proto',
      line: 15,
      column: 3,
      endLine: 15,
      endColumn: 20,
      severity: 'error'
    })
  })

  it('detects format drift if missing required fields', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce('{"path":"hello.proto"}')
    const input = createMockInput('dummy.txt')
    const result = await bufLintParser.parse(input)
    expect(result.failure?.kind).toBe('format_drift')
  })
})
