import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

export const vitestParser = {
  key: 'vitest',
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

    if (!parsed || !Array.isArray(parsed.testResults)) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Missing testResults array' },
        stats: { scannedFiles: 0 }
      }
    }

    const findings: RawQualityFinding[] = []
    let scannedFiles = 0

    for (const suite of parsed.testResults) {
      scannedFiles++
      
      const { file, outside } = toRepoRelative(suite.name, input.cwd, input.repoRoot, input.platform)
      if (outside) continue

      let suiteHasAssertions = false

      if (Array.isArray(suite.assertionResults)) {
        for (const assertion of suite.assertionResults) {
          if (assertion.status === 'passed' || assertion.status === 'skipped' || assertion.status === 'todo') {
            suiteHasAssertions = true
            continue
          }

          if (assertion.status === 'failed') {
            suiteHasAssertions = true
            const ancestors = Array.isArray(assertion.ancestorTitles) ? assertion.ancestorTitles.filter(Boolean).join(' › ') : ''
            const testAnchor = ancestors ? `${ancestors} › ${assertion.title}` : assertion.title

            let msg = testAnchor
            if (Array.isArray(assertion.failureMessages) && assertion.failureMessages.length > 0) {
              msg += '\n' + assertion.failureMessages[0]
            }

            const line = assertion.location?.line ?? 1
            const column = assertion.location?.column ?? 1

            findings.push({
              ruleId: 'vitest/test-failed',
              message: msg,
              file,
              line,
              column,
              severity: 'error'
            })
          }
        }
      }

      if (suite.status === 'failed' && !suiteHasAssertions) {
        findings.push({
          ruleId: 'vitest/suite-failed',
          message: suite.message || 'Test suite failed',
          file,
          line: 1,
          column: 1,
          severity: 'error'
        })
      }
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles }
    }
  }
}
