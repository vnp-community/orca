import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'
import { getGolangciSeverity } from './quality-parser-golangci-severity'

export const golangciParser = {
  key: 'golangci-lint',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    // Check version
    if (input.toolVersion && !input.toolVersion.startsWith('1.')) {
      return {
        findings: [],
        failure: { kind: 'env', envReason: 'tool_incompatible', detail: `Unsupported major version: ${input.toolVersion}` },
        stats: { scannedFiles: 0 }
      }
    }

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

    if (!parsed || (!Array.isArray(parsed.Issues) && parsed.Issues !== null && parsed.Issues !== undefined)) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Missing or invalid Issues array' },
        stats: { scannedFiles: 0 }
      }
    }

    const findings: RawQualityFinding[] = []
    
    if (Array.isArray(parsed.Issues)) {
      for (const issue of parsed.Issues) {
        const pathStr = issue.Pos?.Filename
        if (!pathStr) continue

        const { file, outside } = toRepoRelative(pathStr, input.cwd, input.repoRoot, input.platform)
        if (outside) continue

        const linter = issue.FromLinter || 'unknown'
        const severity = getGolangciSeverity(linter)

        const finding: RawQualityFinding = {
          ruleId: `golangci-lint/${linter}`,
          message: issue.Text || '',
          file,
          line: issue.Pos?.Line || 1,
          column: issue.Pos?.Column || 1,
          severity
        }

        if (issue.Replacement) {
          finding.evidence = { fixHint: issue.Replacement }
        }

        findings.push(finding)
      }
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles: 0 }
    }
  }
}
