import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs/promises'
import { toRepoRelative } from './quality-repo-path-mapping'

export const tscParser = {
  key: 'tsc',
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

    const lines = raw.split(/\r?\n/)
    const findings: RawQualityFinding[] = []
    
    let currentFinding: RawQualityFinding | null = null
    let currentMsgLines: string[] = []
    let currentMsgBytes = 0

    const flushFinding = () => {
      if (currentFinding) {
        currentFinding.message += currentMsgLines.length > 0 ? '\n' + currentMsgLines.join('\n') : ''
        findings.push(currentFinding)
      }
      currentFinding = null
      currentMsgLines = []
      currentMsgBytes = 0
    }

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i]
      if (!line.trim()) continue

      // Regex to match "path/to/file(line,col): error TSXXXX: Message"
      // or "error TSXXXX: Global message" (no file)
      const match = line.match(/^(?:(.*)\((\d+),(\d+)\):\s+)?(error|warning|info)\s+(TS\d+):\s+(.*)$/)
      
      if (match) {
        flushFinding()
        
        const [_, filePath, lineStr, colStr, sevStr, ruleId, msg] = match
        const isGlobal = !filePath
        
        let file = ''
        if (!isGlobal) {
          const mapped = toRepoRelative(filePath, input.cwd, input.repoRoot, input.platform)
          if (mapped.outside) continue
          file = mapped.file
        }

        const lineNum = isGlobal ? 1 : parseInt(lineStr, 10)
        const colNum = isGlobal ? 1 : parseInt(colStr, 10)
        
        currentFinding = {
          ruleId,
          message: msg,
          file,
          line: isNaN(lineNum) ? 1 : lineNum,
          column: isNaN(colNum) ? 1 : colNum,
          severity: sevStr === 'error' ? 'error' : sevStr === 'warning' ? 'warning' : 'info'
        }
      } else if (currentFinding && line.startsWith(' ')) {
        // Indented continuation line
        if (currentMsgLines.length < 10 && currentMsgBytes < 2048) {
          currentMsgLines.push(line.substring(2)) // usually indented by 2 spaces
          currentMsgBytes += Buffer.byteLength(line)
        }
      } else if (currentFinding) {
        // Not indented and didn't match a new error - probably some other garbage or tsc format drift?
        // Let's just ignore it or flush.
        flushFinding()
      }
    }

    flushFinding()

    if (findings.length === 0 && raw.trim().length > 0) {
      return {
        findings: [],
        failure: { kind: 'format_drift', detail: 'Could not parse any TSC errors from non-empty output' },
        stats: { scannedFiles: 0 }
      }
    }

    return {
      findings,
      failure: null,
      stats: { scannedFiles: 0 }
    }
  }
}
