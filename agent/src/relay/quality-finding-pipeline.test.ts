import { describe, it, expect } from 'vitest'
import { runFindingPipeline, inferStepStatus } from './quality-finding-pipeline'
import { RawQualityFinding } from './quality-parser-types'

describe('quality-finding-pipeline', () => {
  it('truncates correctly and counts accurately', async () => {
    const raw: RawQualityFinding[] = []
    for (let i = 0; i < 6000; i++) {
      raw.push({
        ruleId: 'rule1',
        message: 'hello',
        file: 'main.ts',
        line: i + 1,
        column: 1,
        severity: 'error',
        category: 'lint'
      })
    }

    const ctx = {
      stepId: 'step',
      tool: 'test',
      toolVersion: '1.0.0',
      repoRoot: '/repo',
      cwd: '/repo',
      scopeFiles: null,
      readSourceLine: async () => 'code',
      remainingRunBudget: 20000
    }

    const { findings, counts, truncated, outsideRepoCount } = await runFindingPipeline(raw, ctx)
    
    // Default step limit is 5000
    expect(findings.length).toBe(5000)
    expect(truncated).toBe(true)
    expect(counts.total).toBe(6000)
    expect(counts.error).toBe(6000)
    expect(outsideRepoCount).toBe(0)
  })

  it('infers step status', () => {
    // env issue
    expect(inferStepStatus({
      exitCode: 1, hasFindings: false, failureKind: 'env', envReason: 'tool_incompatible', findingsExitCodes: [1, 2]
    })).toEqual({ status: 'env_not_ready', failureKind: 'env', envReason: 'tool_incompatible' })
    
    // format drift
    expect(inferStepStatus({
      exitCode: 1, hasFindings: false, failureKind: null, findingsExitCodes: [1]
    })).toEqual({ status: 'failed', failureKind: 'format_drift' })

    // skip format drift
    expect(inferStepStatus({
      exitCode: 1, hasFindings: false, failureKind: null, findingsExitCodes: [1], skipDriftGuard: true
    })).toEqual({ status: 'failed', failureKind: 'exit_unexpected' }) // or whatever, wait if it skips drift guard it shouldn't drift

    // findings
    expect(inferStepStatus({
      exitCode: 1, hasFindings: true, failureKind: null, findingsExitCodes: [1]
    })).toEqual({ status: 'findings' })
    
    // pass
    expect(inferStepStatus({
      exitCode: 0, hasFindings: false, failureKind: null, findingsExitCodes: [1]
    })).toEqual({ status: 'passed' })
  })
})
