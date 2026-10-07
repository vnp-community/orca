import { ChangedFileDiff } from './quality-diff-changed-lines'
import { Rule } from './quality-rule-pack-schema'
import fs from 'node:fs'
import path from 'node:path'

export interface RawQualityFinding {
  ruleId: string
  file: string
  line?: number
  message: string
  severity: 'error' | 'warning' | 'info'
  anchorOverride?: string
}

export interface MatchResult {
  findings: RawQualityFinding[]
  skippedLongLines: number
  truncated: boolean
}

export interface MatchDeps {
  now: () => number
  budgetMsPerLine: number
  readFile: (p: string) => string
}

// Simple glob matching
// Supports **, *, {a,b}
function matchGlob(pattern: string, filePath: string): boolean {
  let p = pattern.replace(/\\/g, '\\\\').replace(/\./g, '\\.')
  
  // **/ at the start
  p = p.replace(/^\*\*\//, '(.*\\/)?')
  // /**/ in the middle
  p = p.replace(/\/\*\*\//g, '\\/(.*\\/)?')
  // /** at the end
  p = p.replace(/\/\*\*$/, '(\\/.*)?')
  // ** anywhere else (should just be .*)
  p = p.replace(/\*\*/g, '.*')
  // * (single directory level)
  p = p.replace(/\*/g, '[^/]*')
  // {a,b}
  p = p.replace(/\{([^}]+)\}/g, (m, p1) => `(${p1.split(',').join('|')})`)
  
  let regexStr = `^${p}$`
  
  try {
    const re = new RegExp(regexStr)
    return re.test(filePath)
  } catch (e) {
    return false
  }
}

export function matchesScope(rule: Rule, file: ChangedFileDiff): boolean {
  if (rule.scope.fileStatus && rule.scope.fileStatus.length > 0) {
    const statusMap = { A: 'added', M: 'modified', R: 'renamed', C: 'added', T: 'modified' }
    const st = statusMap[file.status] || 'modified'
    if (!rule.scope.fileStatus.includes(st as any)) {
      return false
    }
  }

  let included = false
  for (const inc of rule.scope.include) {
    if (matchGlob(inc, file.path)) {
      included = true
      break
    }
  }
  if (!included && rule.scope.include.length > 0) return false

  for (const exc of rule.scope.exclude) {
    if (matchGlob(exc, file.path)) {
      return false
    }
  }

  return true
}

export function matchRule(
  rule: Rule,
  addedFiles: ChangedFileDiff[],
  deps: MatchDeps
): MatchResult {
  const result: MatchResult = { findings: [], skippedLongLines: 0, truncated: false }
  
  if (rule.kind !== 'diff' || !rule.match) {
    return result
  }

  let re: RegExp
  try {
    re = new RegExp(rule.match.pattern)
  } catch {
    return result
  }

  const isAddedFileRule = rule.match.type === 'added-file-name'
  const isContentRule = rule.match.type === 'file-content-regex'
  const isLineRule = rule.match.type === 'added-line-regex'

  for (const file of addedFiles) {
    if (!matchesScope(rule, file)) continue
    
    if (isAddedFileRule) {
      if (file.status === 'A' || file.status === 'R' || file.status === 'C') {
        if (re.test(file.path)) {
          result.findings.push({
            ruleId: rule.id,
            file: file.path,
            message: rule.message,
            severity: rule.severity
          })
        }
      }
      continue
    }

    if (isContentRule) {
      if (file.status === 'A' || file.status === 'R' || file.status === 'C') {
        try {
          const content = deps.readFile(file.path)
          // limit to 1MB
          if (Buffer.byteLength(content) <= 1024 * 1024) {
            const start = deps.now()
            const match = re.test(content)
            const duration = deps.now() - start
            if (duration > deps.budgetMsPerLine * 100) { // allow a bit more for whole file
               result.truncated = true
            } else if (match) {
               result.findings.push({
                  ruleId: rule.id,
                  file: file.path,
                  message: rule.message,
                  severity: rule.severity
               })
            }
          }
        } catch (e) {
          // ignore read error
        }
      }
      continue
    }

    if (isLineRule) {
      let skipRestOfFile = false
      for (const added of file.addedLines) {
        if (skipRestOfFile) break

        if (added.text.length > rule.match.maxLineLength) {
          result.skippedLongLines++
          continue
        }

        const start = deps.now()
        const match = re.exec(added.text)
        const duration = deps.now() - start

        if (duration > deps.budgetMsPerLine) {
          result.truncated = true
          skipRestOfFile = true
          continue
        }

        if (match) {
          result.findings.push({
            ruleId: rule.id,
            file: file.path,
            line: added.line,
            message: rule.message,
            severity: rule.severity
          })
        }
      }
    }
  }

  return result
}
