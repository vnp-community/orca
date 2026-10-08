import { describe, expect, it } from 'vitest'
import { buildAgentTurnVerificationViewModel } from './agent-turn-verification-view-model'

const item = (agreement: string | undefined, extra: Record<string, unknown> = {}) => ({
  kind: 'tests_pass',
  basis: 'ran_command',
  agreement,
  ...extra
})

describe('buildAgentTurnVerificationViewModel', () => {
  it('maps the four agreements and keeps unverified distinct from consistent', () => {
    const vm = buildAgentTurnVerificationViewModel({
      claims: { items: [item('consistent'), item('contradicted'), item('unverified'), item('not_claimed')] }
    })
    expect(vm.checks.map((c) => c.agreement)).toEqual(['contradicted', 'unverified', 'consistent'])
    expect(vm.hasContradiction).toBe(true)
    const unverified = vm.checks.find((c) => c.agreement === 'unverified')!
    expect(unverified.labelKey).toMatch(/\.agreement\.unverified$/)
  })

  it('treats missing or unrecognized agreement as unknown, never as a pass', () => {
    const vm = buildAgentTurnVerificationViewModel({
      claims: { items: [item(undefined), item('verified-ish')] }
    })
    expect(vm.checks.every((c) => c.agreement === 'unknown')).toBe(true)
    expect(vm.hasContradiction).toBe(false)
  })

  it('labels stated claims as inferred and exposes known reasons only', () => {
    const vm = buildAgentTurnVerificationViewModel({
      claims: {
        items: [
          item('unverified', { basis: 'stated', agreementReason: 'tree_may_differ' }),
          item('unverified', { agreementReason: 'made_up' })
        ]
      }
    })
    expect(vm.checks[0].basisKey).toMatch(/basis\.stated$/)
    expect(vm.checks[0].reasonKey).toMatch(/reason\.tree_may_differ$/)
    expect(vm.checks[1].reasonKey).toBeNull()
  })

  it('builds the ran line from command categories', () => {
    const vm = buildAgentTurnVerificationViewModel({
      commandsSummary: {
        v: 1,
        totalToolUses: 5,
        commands: [
          { name: 'pnpm', sub: 'test', category: 'test', count: 3 },
          { name: 'pnpm', sub: 'lint', category: 'lint', count: 1 }
        ],
        toolCounts: { Bash: 4 },
        truncated: false
      }
    })
    expect(vm.ranLine?.parts).toEqual([
      { categoryKey: expect.stringMatching(/category\.test$/), count: 3 },
      { categoryKey: expect.stringMatching(/category\.lint$/), count: 1 }
    ])
  })

  it('returns an empty view model without input', () => {
    expect(buildAgentTurnVerificationViewModel({})).toEqual({ ranLine: null, checks: [], hasContradiction: false })
  })
})
