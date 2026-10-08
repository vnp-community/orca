import { describe, expect, it } from 'vitest'
import { detectAgentTurnCompletions } from './agent-turn-completion-detector'
import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'

function entry(state: AgentStatusEntry['state'], stateStartedAt: number, worktreeId = 'w'): AgentStatusEntry {
  return { state, prompt: '', updatedAt: stateStartedAt, stateStartedAt, paneKey: 'p', stateHistory: [], worktreeId }
}

describe('detectAgentTurnCompletions', () => {
  it('detects working -> done', () => {
    const result = detectAgentTurnCompletions({ p: entry('working', 1) }, { p: entry('done', 2) })
    expect(result.map((c) => c.paneKey)).toEqual(['p'])
  })

  it('ignores a ping inside the same done state', () => {
    const done = entry('done', 2)
    expect(detectAgentTurnCompletions({ p: done }, { p: { ...done, updatedAt: 9 } })).toEqual([])
  })

  it('ignores first-seen done entries (restore) and entries without a worktree', () => {
    expect(detectAgentTurnCompletions({}, { p: entry('done', 2) })).toEqual([])
    const noWt = { ...entry('done', 2), worktreeId: undefined }
    expect(detectAgentTurnCompletions({ p: entry('working', 1) }, { p: noWt })).toEqual([])
  })

  it('detects a new turn that finishes after a previous done', () => {
    expect(detectAgentTurnCompletions({ p: entry('done', 2) }, { p: entry('done', 5) })).toHaveLength(1)
  })
})
