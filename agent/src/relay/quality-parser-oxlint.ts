import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

export const oxlintParser = {
  key: 'oxlint',
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
      // Oxlint may append \n at the very end outside the JSON object, or other trailing chars
      let toParse = raw.trim()
      if (toParse.endsWith('\\n')) {
        toParse = toParse.slice(0, -2).trim()
      }
      parsed = JSON.parse(toParse)
    } catch (e: any) {
      // Fallback: oxlint@github format (::error title=...,file=...,line=...,col=...::msg)
      return parseGithubFormat(raw, input)
    }

    if (!parsed || !Array.isArray(parsed.diagnostics)) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Missing diagnostics array' },
        stats: { scannedFiles: 0 }
      }
    }

    const findings: RawQualityFinding[] = []
    for (const diag of parsed.diagnostics) {
      if (!diag.filename || typeof diag.message !== 'string') {
        return {
          findings: [],
          failure: { kind: 'format_drift', detail: 'Invalid diagnostic shape' },
          stats: { scannedFiles: 0 }
        }
      }

      const { file, outside } = toRepoRelative(diag.filename, input.cwd, input.repoRoot, input.platform)
      if (outside) continue

      const firstLabel = Array.isArray(diag.labels) && diag.labels.length > 0 ? diag.labels[0] : null
      const line = firstLabel?.span?.line ?? 1
      const column = firstLabel?.span?.column ?? 1
      const endLine = firstLabel?.span?.endLine ?? undefined
      const endColumn = firstLabel?.span?.endColumn ?? undefined
      
      const severity = diag.severity === 'error' ? 'error' : diag.severity === 'warning' ? 'warning' : 'info'

      findings.push({
        ruleId: diag.code || 'oxlint',
        message: diag.message,
        file,
        line,
        column,
        endLine,
        endColumn,
        severity,
        evidence: diag.help ? { help: diag.help } : undefined
      })
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles: typeof parsed.number_of_files === 'number' ? parsed.number_of_files : 0 }
    }
  }
}

function parseGithubFormat(raw: string, input: QualityParserInput): QualityParserOutput {
  const lines = raw.split('\n')
  const findings: RawQualityFinding[] = []
  
  for (const line of lines) {
    if (!line.startsWith('::')) continue
    const match = line.match(/^::(error|warning)\s+(.*?)::(.*)$/)
    if (!match) continue
    const [_, sevStr, attrsStr, msg] = match
    const attrs: Record<string, string> = {}
    for (const attr of attrsStr.split(',')) {
      const parts = attr.split('=')
      if (parts.length >= 2) {
        attrs[parts[0].trim()] = parts.slice(1).join('=').trim()
      }
    }
    
    if (!attrs.file) continue

    const { file, outside } = toRepoRelative(attrs.file, input.cwd, input.repoRoot, input.platform)
    if (outside) continue

    const lineNum = parseInt(attrs.line || '1', 10)
    const colNum = parseInt(attrs.col || '1', 10)
    
    findings.push({
      ruleId: attrs.title || 'oxlint',
      message: msg,
      file,
      line: isNaN(lineNum) ? 1 : lineNum,
      column: isNaN(colNum) ? 1 : colNum,
      severity: sevStr === 'error' ? 'error' : 'warning'
    })
  }

  if (findings.length === 0 && raw.length > 0) {
    return {
      findings: [],
      failure: { kind: 'format_drift', detail: 'Unparseable JSON and no github actions annotations found' },
      stats: { scannedFiles: 0 }
    }
  }

  return { findings, failure: null, stats: { scannedFiles: 0 } }
}
