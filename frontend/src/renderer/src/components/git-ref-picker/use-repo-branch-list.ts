import { useCallback, useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForRepo } from '@/lib/repo-runtime-owner'
import { listRuntimeRepoBranches } from '@/runtime/runtime-repo-client'
import { isRepoRefHostOffline } from './use-repo-ref-search'
import type { RepoBranchEntry } from '../../../../shared/types'

/**
 * loading: the host has not answered; ready: the list (possibly cut, see `truncated`); unsupported: this gateway has no
 * `repo.listBranches`, so the picker searches instead; error: the host answered with a failure or could not be reached.
 */
export type RepoBranchListState =
  | { status: 'loading' }
  | { status: 'ready'; branches: readonly RepoBranchEntry[]; truncated: boolean }
  | { status: 'unsupported' }
  | { status: 'error'; offline: boolean }

export type RepoBranchList = RepoBranchListState & { retry: () => void }

/**
 * The branches of one repo, loaded each time the component using it mounts (a popover mounts its content on open). The result
 * is tagged with the repo, runtime owner and attempt it was loaded for, so a repo switch or a retry never shows older data.
 */
export function useRepoBranchList(repoId: string): RepoBranchList {
  const runtimeEnvironmentId = useAppStore((state) => getRuntimeEnvironmentIdForRepo(state, repoId))
  const [attempt, setAttempt] = useState(0)
  const key = `${runtimeEnvironmentId ?? ''}\u0000${repoId}\u0000${attempt}`
  const [loaded, setLoaded] = useState<{ key: string; value: RepoBranchListState } | null>(null)

  useEffect(() => {
    // The IPC cannot be aborted: an answer for an older repo or attempt is dropped instead.
    let stale = false
    listRuntimeRepoBranches({ activeRuntimeEnvironmentId: runtimeEnvironmentId }, repoId)
      .then((outcome) => {
        if (stale) {
          return
        }
        setLoaded({
          key,
          value:
            outcome.status === 'ok'
              ? { status: 'ready', branches: outcome.branches, truncated: outcome.truncated }
              : { status: 'unsupported' }
        })
      })
      .catch((err: unknown) => {
        console.error('[RepoBranchPicker] listBranches failed', err)
        if (!stale) {
          setLoaded({ key, value: { status: 'error', offline: isRepoRefHostOffline(err) } })
        }
      })
    return () => {
      stale = true
    }
  }, [key, repoId, runtimeEnvironmentId])

  const retry = useCallback(() => setAttempt((n) => n + 1), [])
  const value: RepoBranchListState = loaded?.key === key ? loaded.value : { status: 'loading' }
  return { ...value, retry }
}
