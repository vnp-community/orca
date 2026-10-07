/**
 * agent-turn-privacy-guard.test.ts — FE-CV-TASK-089-07
 *
 * Privacy guard: verifies that agent turn recording modules
 * do NOT contain raw prompts, telemetry imports, or sensitive field names.
 */

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { join } from 'path'

const TURNS_DIR = join(
  import.meta.dirname,
  // Relative from this test file to the turns directory
  '.'
)

const MODULE_FILES = [
  'agent-turn-record-params.ts',
  'agent-turn-record-queue.ts',
  'agent-tool-use-command-summarizer.ts',
  'agent-turn-verification-view-model.ts'
]

describe('Privacy guard: no raw prompt/sensitive fields in output', () => {
  it('buildAgentTurnRecordParams does not include raw prompt without masking', async () => {
    const { buildAgentTurnRecordParams } = await import('./agent-turn-record-params')
    const result = await buildAgentTurnRecordParams({
      paneKey: 'pane-1',
      projectId: 'proj-1',
      worktreeId: 'repo-1::main',
      headOid: 'abc123',
      stateStartedAt: '2024-01-01T00:00:00Z',
      endedAt: '2024-01-01T01:00:00Z',
      stateHistory: [],
      fileIdentifiers: [],
      // No promptExcerptEnabled, no masking function, raw prompt present
      rawPrompt: 'secret system prompt with credentials',
    })
    expect(result).not.toBeNull()
    if (result) {
      const serialized = JSON.stringify(result)
      expect(serialized).not.toContain('secret system prompt')
      expect(serialized).not.toContain('credentials')
      expect(result).not.toHaveProperty('promptExcerpt')
    }
  })

  it('promptExcerpt only present when flag AND masking function are both provided', async () => {
    const { buildAgentTurnRecordParams } = await import('./agent-turn-record-params')
    const maskFn = (text: string) => text.replace(/secret/gi, '[REDACTED]')
    const result = await buildAgentTurnRecordParams({
      paneKey: 'pane-1',
      projectId: 'proj-1',
      worktreeId: 'repo-1::main',
      headOid: 'abc123',
      stateStartedAt: '2024-01-01T00:00:00Z',
      endedAt: '2024-01-01T01:00:00Z',
      stateHistory: [],
      fileIdentifiers: [],
      promptExcerptEnabled: true,
      maskSensitiveText: maskFn,
      rawPrompt: 'secret prompt text'
    })
    expect(result?.promptExcerpt).toBeDefined()
    expect(result?.promptExcerpt).not.toContain('secret')
    expect(result?.promptExcerpt).toContain('[REDACTED]')
  })
})

describe('Privacy guard: no telemetry imports in recorder modules', () => {
  for (const file of MODULE_FILES) {
    it(`${file} does not import telemetry/track`, () => {
      const content = readFileSync(join(TURNS_DIR, file), 'utf-8')
      expect(content).not.toMatch(/import.*\btrack\b/i)
      expect(content).not.toMatch(/import.*\btelemetry\b/i)
      expect(content).not.toMatch(/\bsendTelemetry\b/)
      expect(content).not.toMatch(/\btrackEvent\b/)
    })
  }
})

describe('Privacy guard: verification view model uses no forbidden attribution language', () => {
  it('view model file does not contain forbidden words', () => {
    const content = readFileSync(
      join(TURNS_DIR, 'agent-turn-verification-view-model.ts'),
      'utf-8'
    )
    const forbidden = ['lying', 'lied', 'deceiving', 'faking', 'fake', 'hallucin']
    for (const word of forbidden) {
      expect(content.toLowerCase()).not.toContain(word)
    }
  })

  it('unverified agreement does not map to consistent', async () => {
    const { buildAgentTurnVerificationViewModel } = await import('./agent-turn-verification-view-model')
    const vm = buildAgentTurnVerificationViewModel({
      claims: [{ id: 'c1', text: 'test passed', agreement: 'unverified' }],
      commandsSummary: null
    })
    const unverifiedClaim = vm.claims[0]
    expect(unverifiedClaim.labelKey).not.toContain('consistent')
    expect(unverifiedClaim.agreement).toBe('unverified')
  })
})
