import fs from 'node:fs'
import path from 'node:path'
import { assertNoSecretFields } from './quality-secret-redaction'
import type { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'

export interface GitleaksRawItem {
  RuleID: string
  Description?: string
  StartLine?: number
  EndLine?: number
  StartColumn?: number
  EndColumn?: number
  File?: string
  Secret?: string
  Match?: string
  Entropy?: number
  Fingerprint?: string
}

export function parseGitleaksJson(
  jsonText: string,
  repoRoot: string = ''
): { findings: RawQualityFinding[]; formatDrift: boolean } {
  if (!jsonText || !jsonText.trim()) return { findings: [], formatDrift: false }

  let rawList: any
  try {
    rawList = JSON.parse(jsonText)
  } catch {
    return { findings: [], formatDrift: true }
  }

  if (!Array.isArray(rawList)) {
    return { findings: [], formatDrift: true }
  }

  const findings: RawQualityFinding[] = []

  for (const item of rawList as GitleaksRawItem[]) {
    // Rejects and throws if item contains unredacted raw secret fields!
    assertNoSecretFields(item as any)

    let file = item.File || ''
    if (repoRoot && path.isAbsolute(file)) {
      file = path.relative(repoRoot, file)
    }

    const ruleId = item.RuleID || 'unknown-secret'
    const desc = item.Description || 'Potential secret detected'

    findings.push({
      ruleId: `SEC-SECRET/${ruleId}`,
      file,
      line: item.StartLine || 1,
      column: item.StartColumn || 1,
      severity: 'error',
      message: desc,
      evidence: {
        anchorOverride: ''
      }
    })
  }

  return { findings, formatDrift: false }
}

export async function parseGitleaks(input: QualityParserInput): Promise<QualityParserOutput> {
  let content = ''
  try {
    content = fs.readFileSync(input.stdoutPath, 'utf8')
  } catch {
    return { findings: [], formatDrift: true }
  }

  try {
    const { findings, formatDrift } = parseGitleaksJson(content, input.repoRoot)
    return { findings, formatDrift }
  } catch (err: any) {
    // Unredacted secret or assert failure -> return formatDrift / error
    return {
      findings: [],
      formatDrift: true
    }
  }
}

/**
 * Creates temporary diff file with mode 0600 (read/write only by owner).
 */
export function createTempDiffFile(dir: string, content: string): string {
  const filePath = path.join(dir, `diff-${Date.now()}-${Math.random().toString(36).substring(2)}.diff`)
  fs.writeFileSync(filePath, content, { mode: 0o600, encoding: 'utf8' })
  return filePath
}

/**
 * Safely unlinks temporary diff file.
 */
export function cleanupTempDiffFile(filePath: string): void {
  try {
    if (fs.existsSync(filePath)) {
      fs.unlinkSync(filePath)
    }
  } catch {
    // Ignore cleanup error
  }
}
