import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'
import path from 'path'

async function parseBufOutput(input: QualityParserInput, isBreaking: boolean): Promise<QualityParserOutput> {
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

  const lines = raw.split(/\r?\n/)
  const findings: RawQualityFinding[] = []

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) continue

    let parsed: any
    try {
      parsed = JSON.parse(trimmed)
    } catch (e) {
      // not json line, maybe a build error or env issue, skip for buf JSON output as buf outputs strictly JSON
      continue
    }

    if (!parsed.path || !parsed.type || !parsed.message) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Invalid buf json line format' },
        stats: { scannedFiles: 0 }
      }
    }

    // buf paths are relative to the buf.yaml location which is cwd
    // so we can just use it directly relative to cwd
    const { file, outside } = toRepoRelative(parsed.path, input.cwd, input.repoRoot, input.platform)
    if (outside) continue

    const prefix = isBreaking ? 'buf-breaking' : 'buf'
    const severity = isBreaking ? 'error' : 'warning'

    findings.push({
      ruleId: `${prefix}/${parsed.type}`,
      message: parsed.message,
      file,
      line: parsed.start_line || 1,
      column: parsed.start_column || 1,
      endLine: parsed.end_line,
      endColumn: parsed.end_column,
      severity
    })
  }

  return {
    findings,
    failure: null,
    stats: { scannedFiles: 0 }
  }
}

export const bufLintParser = {
  key: 'buf-lint',
  parse: (input: QualityParserInput) => parseBufOutput(input, false)
}

export const bufBreakingParser = {
  key: 'buf-breaking',
  parse: (input: QualityParserInput) => parseBufOutput(input, true)
}
