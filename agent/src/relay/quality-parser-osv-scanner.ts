import fs from 'node:fs'
import path from 'node:path'
import type { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'

export interface OSVScanVulnerability {
  id: string
  summary?: string
  database_specific?: {
    severity?: string
    cvss?: { score?: number }
  }
}

export interface OSVScanPackage {
  package: {
    name: string
    version: string
    ecosystem?: string
  }
  vulnerabilities?: OSVScanVulnerability[]
}

export interface OSVScanResultItem {
  source?: {
    path?: string
    type?: string
  }
  packages?: OSVScanPackage[]
}

export function parseOsvScannerJson(
  jsonText: string,
  repoRoot: string = ''
): { findings: RawQualityFinding[]; formatDrift: boolean } {
  if (!jsonText || !jsonText.trim()) return { findings: [], formatDrift: false }

  let doc: any
  try {
    doc = JSON.parse(jsonText)
  } catch {
    return { findings: [], formatDrift: true }
  }

  if (!doc || typeof doc !== 'object' || !Array.isArray(doc.results)) {
    return { findings: [], formatDrift: true }
  }

  const findings: RawQualityFinding[] = []

  for (const res of doc.results as OSVScanResultItem[]) {
    let filePath = res.source?.path || 'pnpm-lock.yaml'
    if (repoRoot && path.isAbsolute(filePath)) {
      filePath = path.relative(repoRoot, filePath)
    }

    const packages = res.packages || []
    for (const pkg of packages) {
      const pkgName = pkg.package.name
      const pkgVer = pkg.package.version
      const vulns = pkg.vulnerabilities || []

      for (const vuln of vulns) {
        const vulnId = vuln.id
        const cvssScore = vuln.database_specific?.cvss?.score
        const severityStr = (vuln.database_specific?.severity || '').toUpperCase()

        let severity: 'error' | 'warning' | 'info' = 'info'
        if (typeof cvssScore === 'number') {
          if (cvssScore >= 7.0) severity = 'error'
          else if (cvssScore >= 4.0) severity = 'warning'
          else severity = 'info'
        } else if (severityStr === 'CRITICAL' || severityStr === 'HIGH') {
          severity = 'error'
        } else if (severityStr === 'MEDIUM' || severityStr === 'MODERATE') {
          severity = 'warning'
        }

        const summary = vuln.summary ? `: ${vuln.summary}` : ''
        const message = `[${vulnId}] ${pkgName}@${pkgVer}${summary}`

        findings.push({
          ruleId: `SEC-OSV/${vulnId}`,
          file: filePath,
          line: 1,
          column: 1,
          severity,
          message,
          evidence: {
            anchorOverride: `${pkgName}@${vulnId}`
          }
        })
      }
    }
  }

  return { findings, formatDrift: false }
}

export async function parseOsvScanner(input: QualityParserInput): Promise<QualityParserOutput> {
  let content = ''
  try {
    content = fs.readFileSync(input.stdoutPath, 'utf8')
  } catch {
    return { findings: [], formatDrift: true }
  }

  const { findings, formatDrift } = parseOsvScannerJson(content, input.repoRoot)
  return { findings, formatDrift }
}
