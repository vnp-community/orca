import { RulePack } from './quality-rule-pack-schema'

export interface RuleResult {
  ruleId: string
  status: 'ran' | 'skipped_scope' | 'script_not_found' | 'env_not_ready' | 'disabled'
  durationMs?: number
}

export function buildRuleResults(pack: RulePack, outcomes: RuleResult[]): RuleResult[] {
  const map = new Map<string, RuleResult>()
  for (const o of outcomes) {
    map.set(o.ruleId, o)
  }

  const results: RuleResult[] = []
  for (const rule of pack.rules) {
    if (!rule.enabled) {
      results.push({ ruleId: rule.id, status: 'disabled' })
      continue
    }

    if (map.has(rule.id)) {
      results.push(map.get(rule.id)!)
    } else {
      results.push({ ruleId: rule.id, status: 'skipped_scope' })
    }
  }

  return results
}

export function evaluateStepStatus(
  ruleResults: RuleResult[],
  findingsCount: number
): { state: string; envMissing?: any } {
  const envNotReady = ruleResults.find(r => r.status === 'env_not_ready')
  const scriptNotFound = ruleResults.find(r => r.status === 'script_not_found')

  if (envNotReady || scriptNotFound) {
    return {
      state: 'env_not_ready',
      envMissing: {
        reason: 'rule_env_missing',
        hint: (envNotReady || scriptNotFound)!.ruleId
      }
    }
  }

  if (findingsCount > 0) {
    return { state: 'findings' }
  }

  return { state: 'success' }
}
