export type RawQualityFinding = {
  ruleId: string
  message: string
  file: string
  line: number
  column: number
  endLine?: number
  endColumn?: number
  severity: 'error' | 'warning' | 'info'
  evidence?: any
}

export type QualityParserInput = {
  stepId: string
  stdoutPath: string
  stderrPath: string
  extraPath?: string
  exitCode: number
  timedOut: boolean
  cancelled: boolean
  cwd: string
  repoRoot: string
  platform: NodeJS.Platform
  toolVersion: string | null
  scopeFiles: string[]
  readSourceLine: (file: string, line: number) => Promise<string | null>
}

export type QualityParserOutput = {
  findings: RawQualityFinding[]
  failure: null | {
    kind: 'format_drift' | 'parser_error' | 'env'
    envReason?: string
    detail: string
  }
  stats: {
    scannedFiles: number
    durationMs?: number
  }
}

export interface QualityParser {
  key: string
  parse: (input: QualityParserInput) => Promise<QualityParserOutput>
}
