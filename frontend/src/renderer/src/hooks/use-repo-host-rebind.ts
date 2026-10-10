import { useCallback, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import { getDevServerStatusLabel } from '@/lib/dev-server-status-label'
import { rebindRepoToDevServer, type RepoHostRebindResult } from '@/lib/repo-host-rebind'

export type RepoHostChoice = {
  id: string
  name: string
  // Human label of the live connection state (Connected, Disconnected, ...).
  statusLabel: string
  // Only a connected dev server can take over a repo: git-gateway checks it before every call.
  selectable: boolean
}

/** Dev servers a repo can be moved to, plus the action itself; shared by the host picker dialog and Settings. */
export function useRepoHostRebind(): {
  hostChoices: RepoHostChoice[]
  rebindingRepoId: string | null
  rebind: (repoId: string, newDevServerId: string) => Promise<RepoHostRebindResult>
} {
  const devServers = useAppStore((s) => s.devServers)
  const [rebindingRepoId, setRebindingRepoId] = useState<string | null>(null)
  const hostChoices = useMemo(
    () =>
      (devServers ?? []).map((server) => ({
        id: server.id,
        name: server.name || server.id,
        statusLabel: getDevServerStatusLabel(server.status),
        selectable: server.status === 'connected'
      })),
    [devServers]
  )
  const rebind = useCallback(async (repoId: string, newDevServerId: string) => {
    setRebindingRepoId(repoId)
    try {
      return await rebindRepoToDevServer(repoId, newDevServerId)
    } finally {
      setRebindingRepoId(null)
    }
  }, [])
  return { hostChoices, rebindingRepoId, rebind }
}
