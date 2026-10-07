#!/usr/bin/env node
import fs from 'node:fs/promises'
import path from 'node:path'
import crypto from 'node:crypto'

const FIXTURES_DIR = path.resolve(import.meta.dirname, '../src/relay/__fixtures__/quality-security')

const SAMPLE_GOVULNCHECK = JSON.stringify({
  config: { protocolVersion: 'v1' },
  osv: { id: 'GO-2023-1234', details: 'Vulnerability in sample go module' },
  finding: {
    osv: 'GO-2023-1234',
    fixedVersion: 'v1.2.3',
    trace: [{ module: 'example.com/mod', version: 'v1.0.0', function: 'VulnerableFunc' }]
  }
}, null, 2)

const SAMPLE_OSV_SCANNER = JSON.stringify({
  results: [
    {
      source: { path: 'pnpm-lock.yaml', type: 'lockfile' },
      packages: [
        {
          package: { name: 'sample-pkg', version: '1.0.0', ecosystem: 'npm' },
          vulnerabilities: [
            {
              id: 'GHSA-xxxx-yyyy-zzzz',
              database_specific: { severity: 'HIGH', cvss: { score: 7.5 } },
              summary: 'Sample vulnerability'
            }
          ]
        }
      ]
    }
  ]
}, null, 2)

const SAMPLE_GITLEAKS = JSON.stringify([
  {
    RuleID: 'aws-access-token',
    Description: 'AWS Access Key',
    StartLine: 12,
    EndLine: 12,
    StartColumn: 5,
    EndColumn: 25,
    File: 'src/config.ts',
    Secret: 'REDACTED',
    Match: 'REDACTED',
    Entropy: 0
  }
], null, 2)

async function main() {
  await fs.mkdir(FIXTURES_DIR, { recursive: true })

  await fs.writeFile(path.join(FIXTURES_DIR, 'sample-govulncheck.json'), SAMPLE_GOVULNCHECK, 'utf8')
  await fs.writeFile(path.join(FIXTURES_DIR, 'sample-osv-scanner.json'), SAMPLE_OSV_SCANNER, 'utf8')
  await fs.writeFile(path.join(FIXTURES_DIR, 'sample-gitleaks.json'), SAMPLE_GITLEAKS, 'utf8')

  const manifest = {
    capturedAt: new Date().toISOString(),
    tools: {
      govulncheck: { status: 'BLOCKED', reason: 'tool_not_installed_pending_approval' },
      'osv-scanner': { status: 'BLOCKED', reason: 'tool_not_installed_pending_approval' },
      gitleaks: { status: 'BLOCKED', reason: 'tool_not_installed_pending_approval' }
    },
    fixtures: {
      'sample-govulncheck.json': {
        sha256: crypto.createHash('sha256').update(SAMPLE_GOVULNCHECK).digest('hex'),
        sizeBytes: Buffer.byteLength(SAMPLE_GOVULNCHECK)
      },
      'sample-osv-scanner.json': {
        sha256: crypto.createHash('sha256').update(SAMPLE_OSV_SCANNER).digest('hex'),
        sizeBytes: Buffer.byteLength(SAMPLE_OSV_SCANNER)
      },
      'sample-gitleaks.json': {
        sha256: crypto.createHash('sha256').update(SAMPLE_GITLEAKS).digest('hex'),
        sizeBytes: Buffer.byteLength(SAMPLE_GITLEAKS)
      }
    }
  }

  await fs.writeFile(path.join(FIXTURES_DIR, 'MANIFEST.json'), JSON.stringify(manifest, null, 2) + '\n', 'utf8')
  console.log('Security fixtures and MANIFEST.json generated successfully.')
}

main().catch(err => {
  console.error(err)
  process.exit(1)
})
