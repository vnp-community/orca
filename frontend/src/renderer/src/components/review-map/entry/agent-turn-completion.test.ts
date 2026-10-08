import { describe, expect, it } from 'vitest'
import {
  hasUnreviewedCompletion,
  selectAgentTurnCompletions,
  selectLatestCompletion,
  type AgentTurnCompletionInputs
} from './agent-turn-completion'

function entry(paneKey: string, state: string, at: number, extra: object = {}): never {
  return { paneKey, state, stateStartedAt: at, agentType: 'claude', prompt: '', ...extra } as never
}

const base = (): AgentTurnCompletionInputs => ({
  agentStatusByPaneKey: {},
  retainedAgentsByPaneKey: {},
  tabsByWorktree: { wt1: [{ id: 'tabA' }], wt2: [{ id: 'tabB' }] }
})

describe('selectAgentTurnCompletions', () => {
  it('returns live done entries of the worktree only', () => {
    const s = base()
    s.agentStatusByPaneKey = {
      'tabA:1': entry('tabA:1', 'done', 10),
      'tabA:2': entry('tabA:2', 'working', 20),
      'tabB:1': entry('tabB:1', 'done', 30)
    }
    const out = selectAgentTurnCompletions(s, 'wt1')
    expect(out.map((c) => c.id)).toEqual(['tabA:1:10'])
    expect(out[0].source).toBe('live')
  })

  it('includes retained done entries, sorted newest first, and flags interrupted', () => {
    const s = base()
    s.agentStatusByPaneKey = { 'tabA:1': entry('tabA:1', 'done', 10) }
    s.retainedAgentsByPaneKey = {
      'tabA:9': { worktreeId: 'wt1', entry: entry('tabA:9', 'done', 50, { interrupted: true }) } as never,
      'tabB:9': { worktreeId: 'wt2', entry: entry('tabB:9', 'done', 60) } as never
    }
    const out = selectAgentTurnCompletions(s, 'wt1')
    expect(out.map((c) => c.id)).toEqual(['tabA:9:50', 'tabA:1:10'])
    expect(out[0].interrupted).toBe(true)
    expect(selectLatestCompletion(out)?.id).toBe('tabA:9:50')
  })

  it('dedupes live and retained with the same id, preferring live', () => {
    const s = base()
    const e = entry('tabA:1', 'done', 10)
    s.agentStatusByPaneKey = { 'tabA:1': e }
    s.retainedAgentsByPaneKey = { 'tabA:1': { worktreeId: 'wt1', entry: e } as never }
    const out = selectAgentTurnCompletions(s, 'wt1')
    expect(out).toHaveLength(1)
    expect(out[0].source).toBe('live')
  })

  it('does not throw on odd entries', () => {
    const s = base()
    s.agentStatusByPaneKey = { x: null as never, 'tabA:3': { state: 'done' } as never }
    s.retainedAgentsByPaneKey = { y: undefined as never }
    expect(selectAgentTurnCompletions(s, 'wt1')).toEqual([])
  })
})

describe('hasUnreviewedCompletion', () => {
  it('is true when the latest completion differs from the reviewed turn', () => {
    const s = base()
    s.agentStatusByPaneKey = { 'tabA:1': entry('tabA:1', 'done', 10) }
    const out = selectAgentTurnCompletions(s, 'wt1')
    expect(hasUnreviewedCompletion(out, null)).toBe(true)
    expect(hasUnreviewedCompletion(out, 'tabA:1:10')).toBe(false)
    expect(hasUnreviewedCompletion([], null)).toBe(false)
  })
})
