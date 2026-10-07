import { describe, it, expect } from 'vitest'
import { parseOsvScannerJson } from './quality-parser-osv-scanner'

describe('quality-parser-osv-scanner', () => {
  it('parses osv-scanner json and assigns severity based on CVSS score', () => {
    const json = JSON.stringify({
      results: [
        {
          source: { path: '/repo/pnpm-lock.yaml', type: 'lockfile' },
          packages: [
            {
              package: { name: 'critical-pkg', version: '1.0.0' },
              vulnerabilities: [
                {
                  id: 'GHSA-crit',
                  summary: 'RCE bug',
                  database_specific: { cvss: { score: 9.8 } }
                }
              ]
            },
            {
              package: { name: 'med-pkg', version: '2.0.0' },
              vulnerabilities: [
                {
                  id: 'GHSA-med',
                  summary: 'Info leak',
                  database_specific: { cvss: { score: 5.5 } }
                }
              ]
            },
            {
              package: { name: 'low-pkg', version: '3.0.0' },
              vulnerabilities: [
                {
                  id: 'GHSA-low',
                  summary: 'Minor issue',
                  database_specific: { cvss: { score: 2.1 } }
                }
              ]
            }
          ]
        }
      ]
    })

    const result = parseOsvScannerJson(json, '/repo')
    expect(result.formatDrift).toBe(false)
    expect(result.findings).toHaveLength(3)

    const crit = result.findings.find(f => f.ruleId === 'SEC-OSV/GHSA-crit')
    expect(crit?.severity).toBe('error')
    expect(crit?.file).toBe('pnpm-lock.yaml')

    const med = result.findings.find(f => f.ruleId === 'SEC-OSV/GHSA-med')
    expect(med?.severity).toBe('warning')

    const low = result.findings.find(f => f.ruleId === 'SEC-OSV/GHSA-low')
    expect(low?.severity).toBe('info')
  })

  it('detects formatDrift on invalid JSON structure', () => {
    const result = parseOsvScannerJson('{"someOther": true}')
    expect(result.formatDrift).toBe(true)
    expect(result.findings).toHaveLength(0)
  })
})
