import { describe, it, expect } from 'vitest'
import { parseGovulncheckJson } from './quality-parser-govulncheck'

describe('quality-parser-govulncheck', () => {
  it('parses called vs import-only vulnerabilities correctly', () => {
    const ndjson = [
      JSON.stringify({ osv: { id: 'GO-2023-0001', details: 'Null dereference' } }),
      JSON.stringify({
        finding: {
          osv: 'GO-2023-0001',
          fixedVersion: 'v1.2.0',
          trace: [
            { module: 'example.com/mod1', version: 'v1.0.0', function: 'VulnerableFn', position: { filename: 'mod1/main.go', line: 42 } }
          ]
        }
      }),
      JSON.stringify({ osv: { id: 'GO-2023-0002', details: 'Denial of service' } }),
      JSON.stringify({
        finding: {
          osv: 'GO-2023-0002',
          fixedVersion: 'v2.1.0',
          trace: [
            { module: 'example.com/mod2', version: 'v2.0.0' } // No function or position -> import-only
          ]
        }
      })
    ].join('\n')

    const result = parseGovulncheckJson(ndjson)
    expect(result.formatDrift).toBe(false)
    expect(result.findings).toHaveLength(2)

    const called = result.findings.find(f => f.ruleId === 'SEC-GOVULN/GO-2023-0001')
    expect(called).toBeDefined()
    expect(called?.severity).toBe('error')
    expect(called?.file).toBe('mod1/main.go')
    expect(called?.line).toBe(42)

    const uncalled = result.findings.find(f => f.ruleId === 'SEC-GOVULN/GO-2023-0002')
    expect(uncalled).toBeDefined()
    expect(uncalled?.severity).toBe('info')
  })

  it('handles invalid json by setting formatDrift flag', () => {
    const result = parseGovulncheckJson('not json at all')
    expect(result.formatDrift).toBe(true)
    expect(result.findings).toHaveLength(0)
  })
})
