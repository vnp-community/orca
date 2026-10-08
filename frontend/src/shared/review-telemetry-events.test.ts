import { describe, expect, it } from 'vitest'
import { reviewEventSchemas } from './review-telemetry-events'
import { eventSchemas } from './telemetry-events'

const valid = {
  review_opened: { source: 'agent_row', scope: 'merge_base', lens: 'impact', index: 'fresh', after_agent_turn: true },
  review_lens_viewed: { lens: 'requirements', dwell: '<1m' },
  quality_gate_viewed: { verdict: 'unknown', reasons: '2-3', stale: false, surface: 'review', source: 'mixed' },
  review_decision_made: { decision: 'create_review', latency: '<5m', used_review: true, gate: 'none', open_findings: '0' },
  review_findings_summary: { shown: '4-10', dismissed: '0', waived: '1', resolved: '31+' },
  quality_finding_triaged: { action: 'restore', reason: 'false_positive', severity: 'warning', tool: 'eslint', blocking: false },
  review_report_exported: { format: 'markdown_pr_insert', provider: 'azure-devops', truncated: true },
  review_ai_summary: { outcome: 'bad_output', level: 'diff', cache_hit: true, feedback: 'wrong' }
} as const

const FORBIDDEN_KEYS = [
  'repo_id', 'worktree_id', 'commit', 'path', 'rule_id', 'message', 'task_id', 'tenant_id', 'user', 'email', 'count'
]

describe('review telemetry schemas', () => {
  it('defines exactly the eight events and registers them in eventSchemas', () => {
    expect(Object.keys(reviewEventSchemas).sort()).toEqual(Object.keys(valid).sort())
    for (const name of Object.keys(valid)) {
      expect(eventSchemas).toHaveProperty(name)
    }
  })

  for (const [name, payload] of Object.entries(valid)) {
    const schema = reviewEventSchemas[name as keyof typeof reviewEventSchemas]

    it(`${name}: accepts a valid payload`, () => {
      expect(schema.safeParse(payload).success).toBe(true)
    })

    it(`${name}: rejects every forbidden identifying key`, () => {
      for (const key of FORBIDDEN_KEYS) {
        expect(schema.safeParse({ ...payload, [key]: 'x' }).success, key).toBe(false)
      }
    })

    it(`${name}: rejects a missing field and an out-of-enum value for each field`, () => {
      for (const field of Object.keys(payload)) {
        const { [field as keyof typeof payload]: _omitted, ...rest } = payload as Record<string, unknown>
        expect(schema.safeParse(rest).success, `missing ${field}`).toBe(false)
        const value = (payload as Record<string, unknown>)[field]
        const wrong = typeof value === 'boolean' ? 'yes' : 'not-an-enum-value'
        expect(schema.safeParse({ ...payload, [field]: wrong }).success, `bad ${field}`).toBe(false)
      }
    })
  }

  it('rejects raw numbers where a bucket is required', () => {
    expect(reviewEventSchemas.quality_gate_viewed.safeParse({ ...valid.quality_gate_viewed, reasons: 3 }).success).toBe(false)
    expect(reviewEventSchemas.review_decision_made.safeParse({ ...valid.review_decision_made, open_findings: 12 }).success).toBe(false)
  })

  it('rejects free-form tool names and unknown lenses', () => {
    expect(reviewEventSchemas.quality_finding_triaged.safeParse({ ...valid.quality_finding_triaged, tool: 'my-tool' }).success).toBe(false)
    expect(reviewEventSchemas.review_lens_viewed.safeParse({ lens: 'custom-lens', dwell: '<1m' }).success).toBe(false)
  })
})
