import { describe, it, expect } from 'vitest'
import { buildRuleResults, evaluateStepStatus, RuleResult } from './quality-rule-results'
import { RulePack } from './quality-rule-pack-schema'

describe('quality-rule-results', () => {
  it('buildRuleResults merges outcomes and fills missing with skipped_scope', () => {
    const pack: RulePack = {
      rules: [
        { id: 'R-1', enabled: true, kind: 'diff', category: 'convention', severity: 'error', scope: { include: [], exclude: [] }, message: '', fixHint: '', source: { doc: '' } },
        { id: 'R-2', enabled: false, kind: 'diff', category: 'convention', severity: 'error', scope: { include: [], exclude: [] }, message: '', fixHint: '', source: { doc: '' } },
        { id: 'R-3', enabled: true, kind: 'diff', category: 'convention', severity: 'error', scope: { include: [], exclude: [] }, message: '', fixHint: '', source: { doc: '' } }
      ]
    }

    const outcomes: RuleResult[] = [
      { ruleId: 'R-1', status: 'ran', durationMs: 10 }
    ]

    const results = buildRuleResults(pack, outcomes)
    
    expect(results).toHaveLength(3)
    expect(results.find(r => r.ruleId === 'R-1')?.status).toBe('ran')
    expect(results.find(r => r.ruleId === 'R-2')?.status).toBe('disabled')
    expect(results.find(r => r.ruleId === 'R-3')?.status).toBe('skipped_scope')
  })

  it('evaluateStepStatus determines env_not_ready', () => {
    const res = evaluateStepStatus([{ ruleId: 'R-1', status: 'script_not_found' }], 0)
    expect(res.state).toBe('env_not_ready')
    expect(res.envMissing?.hint).toBe('R-1')
  })

  it('evaluateStepStatus determines findings', () => {
    const res = evaluateStepStatus([{ ruleId: 'R-1', status: 'ran' }], 1)
    expect(res.state).toBe('findings')
  })

  it('evaluateStepStatus determines success', () => {
    const res = evaluateStepStatus([{ ruleId: 'R-1', status: 'ran' }], 0)
    expect(res.state).toBe('success')
  })
})
