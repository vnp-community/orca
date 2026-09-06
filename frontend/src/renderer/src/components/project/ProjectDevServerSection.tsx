// ProjectDevServerSection.tsx — General tab's per-repo dev-server rebind
// form. Phase 10 moved dev-server ownership onto Repo (project.repos.
// dev_server_id), not Project — so this only ever rebinds one specific
// repo at a time via repo.rebindDevServer, regardless of how many repos
// the project has. (The OLD single-project-wide rebind, via
// project.rebindDevServer, is gone: that column is a Phase 10 deprecation
// leftover nothing reads anymore — see ProjectDevServerFilterSection.tsx,
// which replaced it with a pure client-side filter for the Repos tab.)
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { Repo } from '../../types/workspace-types'
import { translate } from '@/i18n/i18n'

type DevServerOption = { id: string; name: string; status: string }

// Same FORBIDDEN/UNAUTHENTICATED-message pattern as MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function ProjectDevServerSection({ projectId }: { projectId: string }) {
  const [devServers, setDevServers] = useState<DevServerOption[]>([])
  const [devServersLoading, setDevServersLoading] = useState(false)

  const [repos, setRepos] = useState<Repo[]>([])
  const [repoSelections, setRepoSelections] = useState<Record<string, string>>({})
  const [repoSavingId, setRepoSavingId] = useState<string | null>(null)

  const fetchRepos = (): void => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<{ repos: Repo[] }>(target, 'repo.list', { projectId })
      .then((result) => setRepos(result?.repos ?? []))
      .catch(() => setRepos([]))
  }

  useEffect(() => {
    setDevServersLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<DevServerOption[]>(target, 'devServer.list', null)
      .then((list) => setDevServers(list ?? []))
      .catch(() => setDevServers([]))
      .finally(() => setDevServersLoading(false))
  }, [])

  useEffect(() => {
    fetchRepos()
    // fetchRepos reads projectId via closure — depending on it directly (not
    // the function identity, which is recreated every render) is enough.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId])

  // Why sync from `repos` instead of initializing state once: fetchRepos
  // (also called after a successful per-repo save, below) re-fetches the
  // repo list, and each picker should reflect whatever is actually
  // persisted — including on first mount.
  useEffect(() => {
    setRepoSelections(Object.fromEntries(repos.map((r) => [r.id, r.devServerId ?? ''])))
  }, [repos])

  const handleRepoSave = async (repo: Repo): Promise<void> => {
    const newDevServerId = repoSelections[repo.id]
    if (!newDevServerId || newDevServerId === (repo.devServerId ?? '')) {
      return
    }
    setRepoSavingId(repo.id)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'repo.rebindDevServer', {
        repoId: repo.id,
        newDevServerId
      })
      toast.success('Dev server updated')
      fetchRepos()
    } catch (err) {
      toast.error(describeError(err, 'Failed to update the dev server.'))
    } finally {
      setRepoSavingId(null)
    }
  }

  if (repos.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.components.project.ProjectDevServerSection.noRepos',
          'Add a repo from the Repos tab to set its dev server.'
        )}
      </p>
    )
  }

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>
          {translate(
            'auto.components.project.ProjectDevServerSection.title',
            repos.length > 1 ? 'Dev servers' : 'Dev server'
          )}
        </Label>
        <p className="text-xs text-muted-foreground">
          {translate(
            'auto.components.project.ProjectDevServerSection.description',
            'Each repo has its own dev server binding. Changing one is blocked while a workflow or task is actively running against that repo.'
          )}
        </p>
      </div>
      {repos.map((repo) => {
        const repoHasChange =
          (repoSelections[repo.id] ?? '') !== '' &&
          repoSelections[repo.id] !== (repo.devServerId ?? '')
        return (
          <div key={repo.id} className="flex items-end gap-2">
            <div className="flex-1 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground">
                {repo.displayName || repo.url}
              </Label>
              <Select
                value={repoSelections[repo.id] ?? ''}
                onValueChange={(value) =>
                  setRepoSelections((selections) => ({ ...selections, [repo.id]: value }))
                }
              >
                <SelectTrigger className="w-64" data-testid={`repo-dev-server-select-${repo.id}`}>
                  <SelectValue
                    placeholder={devServersLoading ? 'Loading…' : 'Select a dev server'}
                  />
                </SelectTrigger>
                <SelectContent>
                  {devServers.map((ds) => (
                    <SelectItem key={ds.id} value={ds.id}>
                      {ds.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              type="button"
              size="sm"
              disabled={!repoHasChange || repoSavingId === repo.id}
              onClick={() => void handleRepoSave(repo)}
              data-testid={`repo-dev-server-save-${repo.id}`}
            >
              {repoSavingId === repo.id ? 'Saving…' : 'Save'}
            </Button>
          </div>
        )
      })}
      {!devServersLoading && devServers.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No dev servers available yet — add one from Settings first.
        </p>
      ) : null}
    </div>
  )
}
