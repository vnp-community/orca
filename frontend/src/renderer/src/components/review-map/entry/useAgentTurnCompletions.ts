import { useRef } from 'react'
import { useAppStore } from '@/store'
import { selectAgentTurnCompletions, type AgentTurnCompletion } from './agent-turn-completion'

const EMPTY: AgentTurnCompletion[] = []

function sameCompletions(a: AgentTurnCompletion[], b: AgentTurnCompletion[]): boolean {
  return a.length === b.length && a.every((item, i) => item.id === b[i].id)
}

/** Completions for one worktree; reference-stable while the id list is unchanged. */
export function useAgentTurnCompletions(worktreeId: string | null): AgentTurnCompletion[] {
  const ref = useRef<AgentTurnCompletion[]>(EMPTY)
  return useAppStore((state) => {
    if (!worktreeId) {
      return EMPTY
    }
    const next = selectAgentTurnCompletions(state, worktreeId)
    if (sameCompletions(ref.current, next)) {
      return ref.current
    }
    ref.current = next.length === 0 ? EMPTY : next
    return ref.current
  })
}
