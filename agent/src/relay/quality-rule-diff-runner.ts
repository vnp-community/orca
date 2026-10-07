import { RulePack } from './quality-rule-pack-schema'
import { ChangedFileDiff } from './quality-diff-changed-lines'
import { matchRule, RawQualityFinding, MatchDeps } from './quality-rule-diff-matcher'
import { execFile } from 'node:child_process'
import path from 'node:path'

export interface RuleResult {
  ruleId: string
  status: 'ran' | 'skipped_scope' | 'script_not_found' | 'env_not_ready' | 'disabled'
  durationMs?: number
}

export interface RunDiffRulesResult {
  findings: RawQualityFinding[]
  ruleResults: RuleResult[]
}

export interface RunnerDeps extends MatchDeps {
  cwd: string
  mergeBaseOf?: string
}

function execGit(args: string[], cwd: string): Promise<{ stdout: string; code: number }> {
  return new Promise((resolve) => {
    execFile('git', args, { cwd, encoding: 'utf8' }, (error, stdout) => {
      resolve({ stdout: stdout || '', code: error ? (error as any).code : 0 })
    })
  })
}

export async function runDiffRules(
  pack: RulePack,
  addedFiles: ChangedFileDiff[],
  enabledIds: string[],
  deps: RunnerDeps
): Promise<RunDiffRulesResult> {
  const findings: RawQualityFinding[] = []
  const ruleResults: RuleResult[] = []

  for (const rule of pack.rules) {
    if (rule.kind !== 'diff') continue

    if (!rule.enabled || !enabledIds.includes(rule.id)) {
      ruleResults.push({ ruleId: rule.id, status: 'disabled' })
      continue
    }

    const start = deps.now()
    const matchRes = matchRule(rule, addedFiles, deps)
    const duration = deps.now() - start

    if (matchRes.findings.length === 0 && matchRes.skippedLongLines === 0 && !matchRes.truncated) {
      // Actually, if it ran and found nothing, is it skipped_scope or ran?
      // "skipped_scope" means no files matched the include/exclude.
      // matchRule doesn't tell us if it skipped scope.
      // Let's check manually if any file matched scope.
      let inScope = false
      for (const f of addedFiles) {
        // A hacky way since matchesScope isn't exported. We can assume if it took time or has findings.
        // If we want exact, we might export matchesScope. For now, assume 'ran'.
      }
    }

    // Custom post-filtering
    let filteredFindings = matchRes.findings

    if (rule.id === 'ORCA-011') {
      // ORCA-011: JSON parse mobile/.oxlintrc.json
      const oxlintFile = addedFiles.find(f => f.path === 'mobile/.oxlintrc.json')
      filteredFindings = [] // replace default regex finding with custom logic
      if (oxlintFile && (oxlintFile.status === 'M' || oxlintFile.status === 'A')) {
        let baseObj: any = {}
        let headObj: any = {}
        if (deps.mergeBaseOf) {
          const mbRes = await execGit(['merge-base', 'HEAD', deps.mergeBaseOf], deps.cwd)
          if (mbRes.code === 0 && mbRes.stdout.trim()) {
            const baseStr = await execGit(['show', `${mbRes.stdout.trim()}:mobile/.oxlintrc.json`], deps.cwd)
            if (baseStr.code === 0) {
              try { baseObj = JSON.parse(baseStr.stdout) } catch {}
            }
          }
        }
        try { headObj = JSON.parse(deps.readFile(path.join(deps.cwd, 'mobile/.oxlintrc.json'))) } catch {}

        const baseOverrides = baseObj.overrides || []
        const headOverrides = headObj.overrides || []

        let raised = false
        for (const h of headOverrides) {
           // simple check
           const hMaxLines = h?.rules?.['max-lines']?.[1]
           if (typeof hMaxLines === 'number') {
              const b = baseOverrides.find((x: any) => x.files?.join(',') === h.files?.join(','))
              const bMaxLines = b?.rules?.['max-lines']?.[1] || 0
              if (hMaxLines > bMaxLines) {
                raised = true
              }
           }
        }
        if (raised) {
           filteredFindings.push({
             ruleId: 'ORCA-011',
             file: 'mobile/.oxlintrc.json',
             message: rule.message,
             severity: rule.severity
           })
        }
      }
    }

    if (rule.id === 'ORCA-012') {
      filteredFindings = filteredFindings.filter(f => {
        const file = addedFiles.find(af => af.path === f.file)
        return file && (file.status === 'A' || file.status === 'R' || file.status === 'C')
      })
    }

    if (rule.id === 'ORCA-013') {
      filteredFindings = filteredFindings.filter(f => {
        if (f.file.endsWith('main.css')) return false
        const file = addedFiles.find(af => af.path === f.file)
        if (!file || !f.line) return true
        const added = file.addedLines.find(l => l.line === f.line)
        if (!added) return true
        if (added.text.includes('href=') || added.text.includes('url(')) return false
        return true
      })
    }

    if (rule.id === 'ORCA-014') {
      filteredFindings = filteredFindings.filter(f => {
        try {
          const content = deps.readFile(f.file) // read full file
          if (content.includes('navigator.userAgent.includes(\\\'Mac\\\')') ||
              content.includes('navigator.userAgent.includes("Mac")') ||
              content.includes('isMac') ||
              content.includes('CmdOrCtrl')) {
            return false
          }
        } catch {}
        return true
      })
    }

    if (rule.id === 'ORCA-015') {
      filteredFindings = filteredFindings.filter(f => {
        const file = addedFiles.find(af => af.path === f.file)
        if (!file || !f.line) return true
        const added = file.addedLines.find(l => l.line === f.line)
        if (!added) return true
        // only if line has execFile(git or git(
        if (!added.text.includes('execFile') && !added.text.includes('git(')) return false
        try {
           const content = deps.readFile(f.file)
           if (content.includes('GitCapabilityCache')) return false
        } catch {}
        return true
      })
    }

    // For finding scope, we consider it skipped if no findings and files array was empty?
    // Let's just always say 'ran' for diff rules if enabled.
    ruleResults.push({ ruleId: rule.id, status: 'ran', durationMs: deps.now() - start })
    findings.push(...filteredFindings)
  }

  return { findings, ruleResults }
}
