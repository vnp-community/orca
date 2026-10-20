import { describe, expect, it } from 'vitest'
import { PLAN_AUTO_FEATURE, withoutPlanAutoFeature } from './live-plan-auto-gateway-features'

const answer = (features: string[], extra: Record<string, unknown> = {}): string =>
  JSON.stringify({ id: 'a1', ok: true, result: { enabled: true, features, ...extra } })

describe('withoutPlanAutoFeature', () => {
  it('removes the planAuto feature from a flowStatus answer and keeps the others', () => {
    const out = JSON.parse(withoutPlanAutoFeature(answer([PLAN_AUTO_FEATURE, 'request.baseRef'])))
    expect(out.result).toEqual({ enabled: true, features: ['request.baseRef'] })
    expect(out.id).toBe('a1')
  })

  it('leaves frames without the feature exactly as they came', () => {
    const plain = JSON.stringify({ id: 'a2', ok: true, result: { requests: [] } })
    expect(withoutPlanAutoFeature(plain)).toBe(plain)
  })

  it('leaves another answer that merely mentions the feature name alone', () => {
    const other = JSON.stringify({ id: 'a3', ok: true, result: { note: PLAN_AUTO_FEATURE } })
    expect(withoutPlanAutoFeature(other)).toBe(other)
  })

  it('rewrites only the matching line of a multi-line message and keeps the newline', () => {
    const stream = JSON.stringify({ id: 's', streaming: true, result: { eventType: 'x' } })
    const out = withoutPlanAutoFeature(`${stream}\n${answer([PLAN_AUTO_FEATURE])}\n`)
    const [first, second, tail] = out.split('\n')
    expect(first).toBe(stream)
    expect(JSON.parse(second).result.features).toEqual([])
    expect(tail).toBe('')
  })

  it('does not throw on a line that is not JSON', () => {
    const broken = `{"result": ${PLAN_AUTO_FEATURE}`
    expect(withoutPlanAutoFeature(broken)).toBe(broken)
  })
})
