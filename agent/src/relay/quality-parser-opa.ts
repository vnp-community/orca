import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

export const opaParser = {
  key: 'opa-test',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    let raw = ''
    try {
      raw = await fs.readFile(input.stdoutPath, 'utf8')
    } catch (e: any) {
      return {
        findings: [],
        failure: { kind: 'env', envReason: 'STDOUT_UNREADABLE', detail: e.message },
        stats: { scannedFiles: 0 }
      }
    }

    if (!raw.trim()) {
      return { findings: [], failure: null, stats: { scannedFiles: 0 } }
    }

    let parsed: any
    try {
      parsed = JSON.parse(raw)
    } catch (e: any) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Invalid JSON' },
        stats: { scannedFiles: 0 }
      }
    }

    if (!Array.isArray(parsed)) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Expected JSON array' },
        stats: { scannedFiles: 0 }
      }
    }

    const findings: RawQualityFinding[] = []

    for (const item of parsed) {
      if (item.fail || item.error) {
        const pathStr = item.location?.file
        let file = ''
        if (pathStr) {
          const mapped = toRepoRelative(pathStr, input.cwd, input.repoRoot, input.platform)
          if (!mapped.outside) {
            file = mapped.file
          }
        }

        const isError = item.error !== undefined && item.error !== null && item.error !== false
        const ruleId = isError ? 'opa/compile-error' : 'opa/test-failed'
        
        let message = ''
        if (isError) {
          message = typeof item.error === 'string' ? item.error : (item.error?.message || 'Compile error')
        } else {
          message = `Test failed: ${item.package}.${item.name}`
        }

        findings.push({
          ruleId,
          message,
          file,
          line: item.location?.row || 1,
          column: item.location?.col || 1,
          severity: 'error'
        })
      }
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles: 0 }
    }
  }
}
