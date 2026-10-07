import fs from 'node:fs'
import type { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'

export interface GovulncheckFindingItem {
  osv: string
  fixedVersion?: string
  trace?: Array<{
    module?: string
    version?: string
    package?: string
    function?: string
    position?: { filename: string; line: number; column?: number }
  }>
}

export interface GovulncheckOSVItem {
  id: string
  details?: string
  aliases?: string[]
}

export function parseGovulncheckJson(jsonText: string): { findings: RawQualityFinding[]; formatDrift: boolean } {
  if (!jsonText || !jsonText.trim()) return { findings: [], formatDrift: false }

  const lines = jsonText.split(/\r?\n/).map(l => l.trim()).filter(Boolean)
  const osvMap = new Map<string, GovulncheckOSVItem>()
  const rawFindings: GovulncheckFindingItem[] = []
  let formatDrift = false

  for (const line of lines) {
    try {
      const obj = JSON.parse(line)
      if (obj.osv && obj.osv.id) {
        osvMap.set(obj.osv.id, obj.osv)
      } else if (obj.finding) {
        rawFindings.push(obj.finding)
      }
    } catch {
      // Maybe whole text was a JSON array or single object
      try {
        const parsed = JSON.parse(jsonText)
        if (Array.isArray(parsed)) {
          for (const item of parsed) {
            if (item.osv?.id) osvMap.set(item.osv.id, item.osv)
            if (item.finding) rawFindings.push(item.finding)
          }
          break
        }
      } catch {
        formatDrift = true
        break
      }
    }
  }

  // Group by (osvId, module)
  const grouped = new Map<string, {
    osvId: string
    module: string
    version: string
    fixedVersion?: string
    hasCallTrace: boolean
    file?: string
    line?: number
  }>()

  for (const item of rawFindings) {
    const osvId = item.osv
    if (!osvId) continue

    const trace = item.trace || []
    const hasCallTrace = trace.some(t => Boolean(t.function || t.position))

    const firstTrace = trace[0] || {}
    const mod = firstTrace.module || 'unknown-module'
    const version = firstTrace.version || ''
    const pos = trace.find(t => t.position)?.position

    const key = `${osvId}::${mod}`
    const existing = grouped.get(key)
    if (!existing) {
      grouped.set(key, {
        osvId,
        module: mod,
        version,
        fixedVersion: item.fixedVersion,
        hasCallTrace,
        file: pos?.filename,
        line: pos?.line
      })
    } else {
      if (hasCallTrace) existing.hasCallTrace = true
      if (pos && !existing.file) {
        existing.file = pos.filename
        existing.line = pos.line
      }
    }
  }

  const findings: RawQualityFinding[] = []
  for (const item of grouped.values()) {
    const osv = osvMap.get(item.osvId)
    const severity: 'error' | 'info' = item.hasCallTrace ? 'error' : 'info'
    const callStatus = item.hasCallTrace ? 'Vulnerable symbol called' : 'Module required but vulnerable symbol not called'

    let msg = `[${item.osvId}] ${item.module}@${item.version}: ${callStatus}`
    if (item.fixedVersion) msg += ` (fixed in ${item.fixedVersion})`
    if (osv?.details) msg += ` - ${osv.details}`

    findings.push({
      ruleId: `SEC-GOVULN/${item.osvId}`,
      file: item.file || 'backend-go/go.mod',
      line: item.line || 1,
      column: 1,
      severity,
      message: msg,
      evidence: {
        anchorOverride: `${item.module}@${item.osvId}`
      }
    })
  }

  return { findings, formatDrift }
}

export async function parseGovulncheck(input: QualityParserInput): Promise<QualityParserOutput> {
  let content = ''
  try {
    content = fs.readFileSync(input.stdoutPath, 'utf8')
  } catch {
    return { findings: [], formatDrift: true }
  }

  const { findings, formatDrift } = parseGovulncheckJson(content)
  return { findings, formatDrift }
}
