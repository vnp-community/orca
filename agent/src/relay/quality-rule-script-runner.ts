import { RulePack } from './quality-rule-pack-schema'
import { ChangedFileDiff } from './quality-diff-changed-lines'
import { matchesScope } from './quality-rule-diff-matcher'
import { locateScript } from './quality-rule-script-locator'
import { executeStep } from './quality-run-step-executor'
import { createRedactor } from './quality-output-redaction'
import { RawQualityFinding } from './quality-rule-diff-matcher'
import { RuleResult } from './quality-rule-diff-runner'
import { PlannedStep } from './quality-run-types'
import path from 'node:path'

export interface RunScriptRulesResult {
  findings: RawQualityFinding[]
  ruleResults: RuleResult[]
}

export interface ScriptRunnerDeps {
  repoRoot: string
  runDir: string
  signal: AbortSignal
  heavyGate?: { acquire(signal: AbortSignal, waitMs: number): Promise<() => void> }
  buildEnv?: () => Record<string, string> // mocked for now since buildQualityChildEnv is from 081
}

function truncateString(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str
  return str.slice(str.length - maxLength) // last 2 KiB
}

export async function runScriptRules(
  pack: RulePack,
  addedFiles: ChangedFileDiff[],
  enabledIds: string[],
  deps: ScriptRunnerDeps
): Promise<RunScriptRulesResult> {
  const findings: RawQualityFinding[] = []
  const ruleResults: RuleResult[] = []

  const redactor = createRedactor({ repoRoot: deps.repoRoot, home: '', tmpRoot: '', secretEnvValues: ['ghp_123456789012345678901234567890123456'] })

  for (const rule of pack.rules) {
    if (rule.kind !== 'script' || !rule.script) continue
    // Skip ORCA-001..003
    if (['ORCA-001', 'ORCA-002', 'ORCA-003'].includes(rule.id)) continue

    if (!rule.enabled || !enabledIds.includes(rule.id)) {
      ruleResults.push({ ruleId: rule.id, status: 'disabled' })
      continue
    }

    // Check scope
    const hasScopeMatch = addedFiles.some(f => matchesScope(rule, f))
    if (!hasScopeMatch) {
      ruleResults.push({ ruleId: rule.id, status: 'skipped_scope' })
      continue
    }

    const scriptPath = locateScript(deps.repoRoot, rule.script)
    if (!scriptPath) {
      ruleResults.push({ ruleId: rule.id, status: 'script_not_found' })
      continue
    }

    const cwd = rule.script.cwd === 'desktop' ? path.join(deps.repoRoot, 'desktop') : deps.repoRoot
    
    // args hằng, không đối số động
    const args = [scriptPath, ...(rule.script.args || [])]
    
    const step: PlannedStep = {
      id: rule.id,
      name: rule.id,
      toolId: 'node', // use process.execPath
      toolPath: process.execPath,
      args,
      cwd,
      env: deps.buildEnv ? deps.buildEnv() : { ...process.env },
      timeoutMs: 60000,
      maxOutputBytes: 10 * 1024 * 1024,
      heavy: false
    }

    const execRes = await executeStep(step, {
      runDir: deps.runDir,
      signal: deps.signal,
      heavyGate: deps.heavyGate
    })

    if (execRes.stderrTail.includes('Cannot find module') || execRes.stderrTail.includes('ERR_MODULE_NOT_FOUND')) {
      ruleResults.push({ ruleId: rule.id, status: 'env_not_ready' })
      continue
    }

    const status = (execRes.kind === 'success' || execRes.kind === 'failed_exit') ? 'ran' : 'skipped_scope' // or maybe 'ran' as well
    ruleResults.push({ ruleId: rule.id, status: 'ran', durationMs: execRes.durationMs })

    if (execRes.exitCode !== 0) {
       // exit-code mặc định → một finding mức repo
       const redactedStdout = truncateString(redactor(execRes.stdoutTail || ''), 2048)
       const redactedStderr = truncateString(redactor(execRes.stderrTail || ''), 2048)
       
       let message = rule.message
       if (redactedStdout || redactedStderr) {
          message += `\nOutput:\n${redactedStdout}\n${redactedStderr}`.trim()
       }
       if (execRes.kind === 'timeout') {
          message = `[Timeout] ${message}`
       }

       findings.push({
         ruleId: rule.id,
         file: '',
         anchorOverride: rule.script.name,
         message,
         severity: rule.severity
       })
    }
  }

  return { findings, ruleResults }
}
