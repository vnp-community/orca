export type IndexScope = 'exact' | 'repo_root' | 'stale' | 'none'
export type IndexFreshness = 'fresh' | 'fresh_base' | 'stale' | 'unknown'

export type IndexBasisInput = {
  tool: 'gitnexus' | 'codegraph'
  toolUsable: boolean
  indexExists: boolean
  rootMatches: boolean
  indexedCommit: string | null
  indexedAtMs: number | null
  headCommit: string | null
  headCommitTimeMs: number | null
  mergeBase: string | null
  mergeBaseCommitTimeMs: number | null
  dirtySinceIndex: boolean
  pendingChanges: { added: number; modified: number; removed: number } | null
}

export type IndexBasisResult = { indexScope: IndexScope; freshness: IndexFreshness }

export function classifyIndexBasis(input: IndexBasisInput): IndexBasisResult {
  if (!input.indexExists) return { indexScope: 'none', freshness: 'unknown' }
  if (!input.toolUsable) return { indexScope: 'none', freshness: 'unknown' }

  if (input.tool === 'codegraph') {
    if (!input.rootMatches) {
      return { indexScope: 'repo_root', freshness: 'unknown' }
    }
    
    const hasPending = input.pendingChanges 
      ? (input.pendingChanges.added > 0 || input.pendingChanges.modified > 0 || input.pendingChanges.removed > 0)
      : false
      
    if (!hasPending && !input.dirtySinceIndex && input.indexedAtMs != null && input.headCommitTimeMs != null && input.indexedAtMs >= input.headCommitTimeMs) {
      return { indexScope: 'exact', freshness: 'fresh' }
    }
    
    return { indexScope: 'stale', freshness: 'stale' }
  }

  // GitNexus
  if (!input.headCommit || !input.indexedCommit) {
    return input.rootMatches 
      ? { indexScope: 'stale', freshness: 'stale' } 
      : { indexScope: 'repo_root', freshness: 'unknown' }
  }

  if (input.rootMatches) {
    if (input.indexedCommit === input.headCommit && !input.dirtySinceIndex) {
      return { indexScope: 'exact', freshness: 'fresh' }
    }
    return { indexScope: 'stale', freshness: 'stale' }
  }

  if (!input.rootMatches) {
    if (input.mergeBase === null) {
      return { indexScope: 'repo_root', freshness: 'unknown' }
    }
    if (input.indexedCommit === input.mergeBase) {
      return { indexScope: 'repo_root', freshness: 'fresh_base' }
    }
    return { indexScope: 'stale', freshness: 'stale' }
  }

  return { indexScope: 'stale', freshness: 'stale' }
}
