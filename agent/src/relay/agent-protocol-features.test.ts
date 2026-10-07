import { describe, it, expect } from 'vitest'
import {
  AGENT_PROTOCOL_VERSION,
  AGENT_FEATURES
} from './agent-protocol-features'

describe('agent-protocol-features', () => {
  it('protocolVersion is 2', () => {
    expect(AGENT_PROTOCOL_VERSION).toBe(2)
  })

  it('has no duplicate feature names and all match pattern', () => {
    const featureSet = new Set(AGENT_FEATURES)
    expect(featureSet.size).toBe(AGENT_FEATURES.length)

    const pattern = /^[a-z][A-Za-z0-9.]*$/
    for (const f of AGENT_FEATURES) {
      expect(f).toMatch(pattern)
    }
  })

  it('declares all expected CR-REQ-033 capabilities', () => {
    expect(AGENT_FEATURES).toContain('agent.execPrompt')
    expect(AGENT_FEATURES).toContain('agent.execPrompt.readonly')
    expect(AGENT_FEATURES).toContain('agent.execPrompt.workspaceKind')
    expect(AGENT_FEATURES).toContain('agent.execPrompt.changes')
    expect(AGENT_FEATURES).toContain('agent.execPrompt.resultBlock')
    expect(AGENT_FEATURES).toContain('agent.capabilities')
    expect(AGENT_FEATURES).toContain('ai.complete')
    expect(AGENT_FEATURES).toContain('ai.complete.usage')
  })
})
