import { runGit } from './codeintel-git-exec'

let cache = new Map<string, { commit: string | null; timestamp: number }>()

export function invalidateHeadCommit(workspaceRoot?: string) {
  if (workspaceRoot) {
    cache.delete(workspaceRoot)
  } else {
    cache.clear()
  }
}

export async function getHeadCommit(workspaceRoot: string): Promise<string | null> {
  const cached = cache.get(workspaceRoot)
  if (cached && Date.now() - cached.timestamp < 5000) {
    return cached.commit
  }

  let commit: string | null = null
  try {
    const res = await runGit(['rev-parse', '--verify', '--quiet', 'HEAD'], workspaceRoot)
    const line = res.stdout.trim()
    if (line) {
      commit = line
    }
  } catch {
    // unborn or error
  }

  cache.set(workspaceRoot, { commit, timestamp: Date.now() })
  return commit
}
