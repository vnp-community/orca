import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForRepo } from '@/lib/repo-runtime-owner'
import { getRuntimeRepoBaseRefDefault } from '@/runtime/runtime-repo-client'

/** `ready` with a null name means the repo has no default we can name; `failed` means the host could not be asked. */
export type RepoDefaultBaseRef =
  | { status: 'loading' }
  | { status: 'ready'; name: string | null }
  | { status: 'failed' }

export type RepoDefaultBaseRefLookup = RepoDefaultBaseRef & { retry: () => void }

/**
 * The name of a repo's default branch, for the "Default branch (main)" chip. Results are keyed by repo and runtime owner,
 * so switching project never shows the previous repo's default. Never invents a name.
 */
export function useRepoDefaultBaseRef(repoId: string): RepoDefaultBaseRefLookup {
  const runtimeEnvironmentId = useAppStore((state) => getRuntimeEnvironmentIdForRepo(state, repoId))
  const scope = `${runtimeEnvironmentId ?? ''}\u0000${repoId}`
  const [loaded, setLoaded] = useState<{ scope: string; value: RepoDefaultBaseRef } | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let stale = false
    getRuntimeRepoBaseRefDefault({ activeRuntimeEnvironmentId: runtimeEnvironmentId }, repoId)
      .then((result) => {
        if (!stale) {
          setLoaded({ scope, value: { status: 'ready', name: result.defaultBaseRef || null } })
        }
      })
      .catch((err: unknown) => {
        console.error('[StartFromField] getBaseRefDefault failed', err)
        if (!stale) {
          setLoaded({ scope, value: { status: 'failed' } })
        }
      })
    return () => {
      stale = true
    }
  }, [repoId, runtimeEnvironmentId, scope, attempt])

  const value: RepoDefaultBaseRef = loaded?.scope === scope ? loaded.value : { status: 'loading' }
  return { ...value, retry: () => setAttempt((n) => n + 1) }
}
