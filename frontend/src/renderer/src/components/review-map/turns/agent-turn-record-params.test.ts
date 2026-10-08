import { describe, expect, it } from 'vitest'
import { buildAgentTurnRecordParams } from './agent-turn-record-params'
import type { AgentTurnRecordInput } from './agent-turn-record-params'

const CONTRACT_KEYS = new Set([
  'projectId', 'worktreeId', 'clientTurnId', 'agentType', 'endedAt', 'startedAt', 'interrupted',
  'endHeadCommit', 'treeDirtyEnd', 'filesChangedCount', 'filesDigest', 'promptDigest',
  'promptExcerpt', 'commandsSummary'
])

function input(overrides: Partial<AgentTurnRecordInput> = {}): AgentTurnRecordInput {
  return {
    projectId: 'p1',
    worktreeId: 'repo::wt',
    entry: {
      paneKey: 'tab:leaf',
      agentType: 'claude',
      prompt: 'SECRET prompt text',
      doneAt: Date.UTC(2026, 9, 7, 10, 0, 0),
      stateHistory: [
        { state: 'working', startedAt: Date.UTC(2026, 9, 7, 9, 50, 0) },
        { state: 'waiting', startedAt: Date.UTC(2026, 9, 7, 9, 55, 0) }
      ]
    },
    headOid: 'abc123',
    treeDirty: true,
    fileIdentities: ['b.ts', 'a.ts'],
    commands: null,
    storePromptExcerpt: false,
    ...overrides
  }
}

describe('buildAgentTurnRecordParams', () => {
  it('builds contract-shaped params with digests only', () => {
    const params = buildAgentTurnRecordParams(input())
    expect(params).toMatchObject({
      clientTurnId: `tab:leaf:${Date.UTC(2026, 9, 7, 10, 0, 0)}`,
      agentType: 'claude',
      endedAt: '2026-10-07T10:00:00.000Z',
      startedAt: '2026-10-07T09:50:00.000Z',
      endHeadCommit: 'abc123',
      treeDirtyEnd: true,
      filesChangedCount: 2
    })
    expect(params?.promptDigest).toMatch(/^[0-9a-f]{64}$/)
    expect(params?.filesDigest).toMatch(/^[0-9a-f]{64}$/)
  })

  it('emits only contract keys and never the raw prompt', () => {
    const params = buildAgentTurnRecordParams(input())!
    for (const key of Object.keys(params)) {
      expect(CONTRACT_KEYS.has(key)).toBe(true)
    }
    expect(JSON.stringify(params)).not.toContain('SECRET')
  })

  it.each([
    ['projectId', { projectId: null }],
    ['worktreeId', { worktreeId: undefined }],
    ['headOid', { headOid: null }],
    ['floating workspace', { worktreeId: '::workspace:folder:x' }],
    ['folder workspace', { worktreeId: 'folder:abc' }]
  ])('returns null when %s is unusable', (_name, override) => {
    expect(buildAgentTurnRecordParams(input(override as Partial<AgentTurnRecordInput>))).toBeNull()
  })

  it('omits startedAt without a working entry and sets interrupted only when true', () => {
    const base = input()
    const none = buildAgentTurnRecordParams({ ...base, entry: { ...base.entry, stateHistory: [] } })!
    expect(none).not.toHaveProperty('startedAt')
    expect(none).not.toHaveProperty('interrupted')
    const cut = buildAgentTurnRecordParams({ ...base, entry: { ...base.entry, interrupted: true } })!
    expect(cut.interrupted).toBe(true)
  })

  it('sends promptExcerpt only with the tenant flag AND a masker, capped at 160', () => {
    const long = 'x'.repeat(400)
    const base = input({ entry: { ...input().entry, prompt: long } })
    const mask = (t: string) => t.replace(/x/g, 'y')
    expect(buildAgentTurnRecordParams({ ...base, storePromptExcerpt: true })).not.toHaveProperty('promptExcerpt')
    expect(buildAgentTurnRecordParams({ ...base, maskSensitiveText: mask })).not.toHaveProperty('promptExcerpt')
    const withBoth = buildAgentTurnRecordParams({ ...base, storePromptExcerpt: true, maskSensitiveText: mask })!
    expect(withBoth.promptExcerpt).toBe('y'.repeat(160))
  })

  it('stays well under the 16 KiB argument limit', () => {
    const params = buildAgentTurnRecordParams(input())!
    expect(new TextEncoder().encode(JSON.stringify(params)).length).toBeLessThan(16 * 1024)
  })
})
