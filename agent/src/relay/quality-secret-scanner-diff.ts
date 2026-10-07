import type { ChangedFileDiff } from './quality-diff-changed-lines'
import type { RawQualityFinding } from './quality-rule-diff-matcher'

export interface SecretPatternDef {
  type: string
  regex: RegExp
  message: string
  fixHint: string
}

export const SECRET_PATTERNS: readonly SecretPatternDef[] = [
  {
    type: 'aws-access-key',
    regex: /(?:^|[^A-Z0-9])(AKIA[0-9A-Z]{16})(?:[^A-Z0-9]|$)/,
    message: 'Potential AWS Access Key ID detected',
    fixHint: 'Revoke key and store in secret manager or environment variable'
  },
  {
    type: 'private-key',
    regex: /-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----/,
    message: 'Private key block detected',
    fixHint: 'Remove private key file or credentials from repository'
  },
  {
    type: 'github-token',
    regex: /(?:^|[^A-Za-z0-9_])(gh[pousr]_[A-Za-z0-9_]{36,255})(?:[^A-Za-z0-9_]|$)/,
    message: 'Potential GitHub Personal Access Token detected',
    fixHint: 'Revoke token and configure GitHub repository secrets'
  },
  {
    type: 'slack-token',
    regex: /(?:^|[^a-zA-Z0-9-])(xox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*)/,
    message: 'Potential Slack token detected',
    fixHint: 'Revoke Slack token and manage via env variable'
  },
  {
    type: 'openai-api-key',
    regex: /(?:^|[^A-Za-z0-9_])(sk-[A-Za-z0-9]{20,})(?:[^A-Za-z0-9_]|$)/,
    message: 'Potential OpenAI API key detected',
    fixHint: 'Revoke API key and use secrets manager'
  },
  {
    type: 'jwt',
    regex: /(?:^|[^A-Za-z0-9-_])(eyJ[A-Za-z0-9-_=]{10,}\.eyJ[A-Za-z0-9-_=]{10,}\.[A-Za-z0-9-_.+/=]{10,})/,
    message: 'Potential JSON Web Token detected',
    fixHint: 'Do not commit signed JWTs with sensitive claims'
  }
]

export const SECRET_SCAN_EXCLUDE_PATHS = [
  /(?:^|\/)(?:pnpm-lock\.yaml|go\.sum|package-lock\.json|yarn\.lock)$/,
  /\.lock$/,
  /(?:^|\/)(?:node_modules|vendor|__fixtures__|fixtures|test|tests)\//
]

export interface ScanAddedLinesResult {
  findings: RawQualityFinding[]
  truncated: boolean
}

export interface ScanAddedLinesOpts {
  maxFiles?: number
  maxBytes?: number
}

export function isPathExcluded(filePath: string): boolean {
  return SECRET_SCAN_EXCLUDE_PATHS.some(rx => rx.test(filePath))
}

export function scanAddedLines(
  files: readonly ChangedFileDiff[],
  opts: ScanAddedLinesOpts = {}
): ScanAddedLinesResult {
  const maxFiles = opts.maxFiles ?? 5000
  const maxBytes = opts.maxBytes ?? 20 * 1024 * 1024

  const findings: RawQualityFinding[] = []
  let totalBytes = 0
  let truncated = false

  let fileCount = 0
  for (const file of files) {
    if (fileCount >= maxFiles) {
      truncated = true
      break
    }
    fileCount++

    if (file.binary || isPathExcluded(file.path)) {
      continue
    }

    for (const addedLine of file.addedLines) {
      const lineLen = Buffer.byteLength(addedLine.text, 'utf8')
      if (totalBytes + lineLen > maxBytes) {
        truncated = true
        break
      }
      totalBytes += lineLen

      for (const pattern of SECRET_PATTERNS) {
        if (pattern.regex.test(addedLine.text)) {
          // Never output matchedText or value details
          findings.push({
            ruleId: `SEC-SECRET/${pattern.type}`,
            file: file.path,
            line: addedLine.line,
            message: pattern.message,
            severity: 'error',
            anchorOverride: ''
          })
        }
      }
    }

    if (truncated) break
  }

  return {
    findings,
    truncated
  }
}
