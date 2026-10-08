/**
 * agent-turn-privacy-guard.test.ts — FE-CV-TASK-089-07
 *
 * Static + behavioural guard: the turn recorder modules never import telemetry and
 * never put raw prompt / tool input in the outgoing payload; the verification
 * wording stays free of attribution language.
 */

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildAgentTurnRecordParams } from './agent-turn-record-params'
import { createAgentTurnToolCollector } from './agent-tool-use-command-summarizer'

const TURNS_DIR = import.meta.dirname

const RECORDER_MODULES = [
  'agent-turn-record-params.ts',
  'agent-turn-record-queue.ts',
  'agent-tool-use-command-summarizer.ts',
  'agent-turn-completion-detector.ts',
  'use-agent-turn-backend-recorder.ts'
]

const read = (file: string): string => readFileSync(join(TURNS_DIR, file), 'utf-8')

describe('recorder modules do not import telemetry', () => {
  for (const file of RECORDER_MODULES) {
    it(file, () => {
      const content = read(file)
      expect(content).not.toMatch(/from\s+['"][^'"]*telemetry[^'"]*['"]/i)
      expect(content).not.toMatch(/\btrackEvent\b|\bsendTelemetry\b/)
    })
  }
})

describe('outgoing payload carries no raw text', () => {
  const secret = 'sk-live-SECRET-token and my private prompt'

  it('omits the prompt, tool input and assistant text from params', () => {
    const collector = createAgentTurnToolCollector()
    collector.observe('p', { state: 'working', updatedAt: 1, toolName: 'Bash', toolInput: `echo ${secret}` })
    const params = buildAgentTurnRecordParams({
      projectId: 'p1',
      worktreeId: 'repo::wt',
      entry: { paneKey: 'p', prompt: secret, doneAt: 1, stateHistory: [] },
      headOid: 'abc',
      treeDirty: false,
      fileIdentities: [],
      commands: collector.take('p'),
      storePromptExcerpt: true // flag on, but no masker exists -> still no excerpt
    })
    const serialized = JSON.stringify(params)
    expect(serialized).not.toContain('SECRET')
    expect(serialized).not.toContain('private prompt')
    expect(params).not.toHaveProperty('promptExcerpt')
  })
})

describe('verification wording', () => {
  it('uses no attribution language in code', () => {
    const forbidden = ['lying', 'lied', 'deceiv', 'fake', 'faking', 'hallucin', 'dishonest']
    for (const file of ['agent-turn-verification-view-model.ts', 'AgentTurnVerificationLine.tsx']) {
      const content = read(file).toLowerCase()
      for (const word of forbidden) {
        expect(content, `${file} contains "${word}"`).not.toContain(word)
      }
    }
  })
})
