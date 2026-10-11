import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'

export type RequestProjectRepo = {
  id: string
  url: string
  displayName: string
  devServerId: string
}

export type RequestProjectRepos =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ready'; repos: RequestProjectRepo[] }
  | { status: 'failed' }

/**
 * Repos of the project a Request is being created in. The backend runs the Request on the first
 * one, so the dialog shows it instead of leaving users to guess.
 */
export function useRequestProjectRepos(projectId: string): RequestProjectRepos {
  const [state, setState] = useState<{ projectId: string; result: RequestProjectRepos } | null>(
    null
  )

  useEffect(() => {
    if (!projectId) {
      return
    }
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    void callRuntimeRpc<{ repos?: RequestProjectRepo[] }>(target, 'repo.list', { projectId })
      .then((list) => {
        if (!cancelled) {
          const repos = Array.isArray(list?.repos) ? list.repos : []
          setState({ projectId, result: { status: 'ready', repos } })
        }
      })
      .catch(() => {
        if (!cancelled) {
          setState({ projectId, result: { status: 'failed' } })
        }
      })
    return () => {
      cancelled = true
    }
  }, [projectId])

  if (!projectId) {
    return { status: 'idle' }
  }
  // Why project-keyed: a result for the previous project must never describe the new selection.
  return state?.projectId === projectId ? state.result : { status: 'loading' }
}
