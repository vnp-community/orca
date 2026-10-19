import { describe, expect, it } from 'vitest'
import { BASE_REF_FEATURE, withoutBaseRefFeature } from './live-base-branch-gateway-features'

const answer = (features: string[], extra: Record<string, unknown> = {}): string =>
  JSON.stringify({ id: 'a1', ok: true, result: { enabled: true, features, ...extra } })

describe('withoutBaseRefFeature', () => {
  it('removes the starting-branch feature from a flowStatus answer and keeps the others', () => {
    const out = JSON.parse(withoutBaseRefFeature(answer([BASE_REF_FEATURE, 'request.other'])))
    expect(out.result).toEqual({ enabled: true, features: ['request.other'] })
    expect(out.id).toBe('a1')
  })

  it('leaves frames without the feature exactly as they came', () => {
    const plain = JSON.stringify({ id: 'a2', ok: true, result: { requests: [] } })
    expect(withoutBaseRefFeature(plain)).toBe(plain)
  })

  it('leaves another answer that merely mentions the feature name alone', () => {
    const other = JSON.stringify({ id: 'a3', ok: true, result: { note: BASE_REF_FEATURE } })
    expect(withoutBaseRefFeature(other)).toBe(other)
  })

  it('rewrites only the matching line of a multi-line message and keeps the newline', () => {
    const stream = JSON.stringify({ id: 's', streaming: true, result: { eventType: 'x' } })
    const out = withoutBaseRefFeature(`${stream}\n${answer([BASE_REF_FEATURE])}\n`)
    const [first, second, tail] = out.split('\n')
    expect(first).toBe(stream)
    expect(JSON.parse(second).result.features).toEqual([])
    expect(tail).toBe('')
  })

  it('does not throw on a line that is not JSON', () => {
    const broken = `{"result": ${BASE_REF_FEATURE}`
    expect(withoutBaseRefFeature(broken)).toBe(broken)
  })
})
