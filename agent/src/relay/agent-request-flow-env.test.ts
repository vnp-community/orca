import { describe, expect, it } from 'vitest'
import { buildRequestFlowEnv, isWithinRequestFlowEnvAllowlist } from './agent-request-flow-env'

describe('buildRequestFlowEnv', () => {
  it('contains only the two allowed keys', () => {
    const env = buildRequestFlowEnv('r1', 'p1')
    expect(env).toEqual({ ORCA_REQUEST_ID: 'r1', ORCA_PROJECT_ID: 'p1' })
    expect(isWithinRequestFlowEnvAllowlist(env)).toBe(true)
  })
  it('rejects provider keys and tokens', () => {
    expect(isWithinRequestFlowEnvAllowlist({ ANTHROPIC_API_KEY: 'x' })).toBe(false)
    expect(isWithinRequestFlowEnvAllowlist({ ORCA_TOKEN: 'x' })).toBe(false)
  })
})
