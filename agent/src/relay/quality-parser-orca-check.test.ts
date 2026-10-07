import { describe, it, expect, vi } from 'vitest'
import { maxLinesRatchetParser, styledScrollbarsParser, reliabilityGatesParser } from './quality-parser-orca-check'
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
    cwd: '/opt/repos/orca',
    repoRoot: '/opt/repos/orca',
    platform: 'linux',
    toolVersion: '1.0.0',
    scopeFiles: [],
    readSourceLine: async () => null
  }
}

describe('quality-parser-orca-check', () => {
  it('maxLinesRatchetParser parses new and stale bypasses', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      '::error::New max-lines bypass not allowed: inline path/to/file.ts\n' +
      '::error::Stale max-lines baseline entry (prune it): mobile-config'
    )
    const input = createMockInput('dummy.txt')
    const result = await maxLinesRatchetParser.parse(input)
    
    expect(result.findings).toHaveLength(2)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'orca-check/max-lines-ratchet/new-bypass',
      severity: 'error',
      file: 'path/to/file.ts',
      anchorOverride: 'inline path/to/file.ts'
    })
    expect(result.findings[1]).toMatchObject({
      ruleId: 'orca-check/max-lines-ratchet/stale-baseline',
      severity: 'warning',
      file: 'mobile/.oxlintrc.json',
      anchorOverride: 'mobile-config'
    })
  })

  it('styledScrollbarsParser parses findings', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      'Title\nMore title\nAnother\nHeader\n' +
      'src/components/foo.css:10:5 unstyled scrollbar detected\n' +
      'src/bar.ts:20:1 something else'
    )
    const input = createMockInput('dummy.txt')
    const result = await styledScrollbarsParser.parse(input)
    
    expect(result.findings).toHaveLength(2)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'orca-check/styled-scrollbars/unstyled',
      file: 'desktop/src/components/foo.css',
      line: 10,
      column: 5
    })
  })

  it('reliabilityGatesParser parses gates failures', async () => {
    vi.spyOn(fs, 'readFile').mockResolvedValueOnce(
      'Reliability gate manifest check failed with 1 issue(s):\n' +
      '- gate-123: is invalid'
    )
    const input = createMockInput('dummy.txt')
    const result = await reliabilityGatesParser.parse(input)
    
    expect(result.findings).toHaveLength(1)
    expect(result.findings[0]).toMatchObject({
      ruleId: 'orca-check/reliability-gates/invalid-manifest',
      file: 'desktop/config/reliability-gates.jsonc',
      message: 'is invalid',
      anchorOverride: 'gate-123'
    })
  })
})
