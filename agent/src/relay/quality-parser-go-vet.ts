import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

export const goVetParser = {
  key: 'go-vet',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    let stdoutRaw = ''
    let stderrRaw = ''
    try {
      stdoutRaw = await fs.readFile(input.stdoutPath, 'utf8')
      if (input.stderrPath) {
        stderrRaw = await fs.readFile(input.stderrPath, 'utf8')
      }
    } catch (e: any) {
      return {
        findings: [],
        failure: { kind: 'env', envReason: 'OUTPUT_UNREADABLE', detail: e.message },
        stats: { scannedFiles: 0 }
      }
    }

    const raw = stdoutRaw + '\n' + stderrRaw
    if (!raw.trim()) {
      return { findings: [], failure: null, stats: { scannedFiles: 0 } }
    }

    const lines = raw.split(/\r?\n/)
    const findings: RawQualityFinding[] = []

    for (const line of lines) {
      const trimmed = line.trim()
      if (!trimmed) continue

      if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
        try {
          const parsed = JSON.parse(trimmed)
          for (const pkg of Object.keys(parsed)) {
            const analyzers = parsed[pkg]
            for (const analyzer of Object.keys(analyzers)) {
              const issues = analyzers[analyzer]
              if (Array.isArray(issues)) {
                for (const issue of issues) {
                  const { file, lineNum, colNum } = parsePosn(issue.posn, input)
                  if (!file) continue
                  findings.push({
                    ruleId: `go-vet/${analyzer}`,
                    message: issue.message || 'go vet issue',
                    file,
                    line: lineNum,
                    column: colNum,
                    severity: 'error'
                  })
                }
              }
            }
          }
          continue
        } catch (e) {
          // ignore invalid json line
        }
      }

      if (trimmed.startsWith('# ')) {
        continue // # package name output
      }

      // Check text fallback: file.go:line:col: message
      const textMatch = trimmed.match(/^([^:]+):(\d+):(?:(\d+):)?\s+(.*)$/)
      if (textMatch) {
        const [_, pathStr, lineStr, colStr, msg] = textMatch
        const { file, outside } = toRepoRelative(pathStr, input.cwd, input.repoRoot, input.platform)
        if (!outside) {
          findings.push({
            ruleId: 'go-vet/build-failed',
            message: msg,
            file,
            line: parseInt(lineStr, 10) || 1,
            column: colStr ? parseInt(colStr, 10) : 1,
            severity: 'error'
          })
        }
      }
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles: 0 }
    }
  }
}

function parsePosn(posn: string | undefined, input: QualityParserInput) {
  if (!posn) return { file: '', lineNum: 1, colNum: 1 }
  // posn is typically "file:line:col" or "file:line"
  const parts = posn.split(':')
  if (parts.length < 2) {
    const { file, outside } = toRepoRelative(posn, input.cwd, input.repoRoot, input.platform)
    return { file: outside ? '' : file, lineNum: 1, colNum: 1 }
  }

  let colNum = 1
  let lineNum = 1
  let fileStr = ''

  if (parts.length >= 3 && !isNaN(parseInt(parts[parts.length - 1], 10)) && !isNaN(parseInt(parts[parts.length - 2], 10))) {
    colNum = parseInt(parts.pop()!, 10)
    lineNum = parseInt(parts.pop()!, 10)
    fileStr = parts.join(':')
  } else if (parts.length >= 2 && !isNaN(parseInt(parts[parts.length - 1], 10))) {
    lineNum = parseInt(parts.pop()!, 10)
    fileStr = parts.join(':')
  } else {
    fileStr = posn
  }

  const { file, outside } = toRepoRelative(fileStr, input.cwd, input.repoRoot, input.platform)
  return { file: outside ? '' : file, lineNum, colNum }
}
