// ProjectDevServerSection.tsx — General tab's dev-server rebind form.
// project.rebindDevServer already existed server-side (with its
// active-execution guard) and was only ever called from CreateProjectDialog
// at creation time — a project created without picking one (or one whose
// dev server later needs to change) had no way to fix that afterward; the
// dialog's own error toast even said "Set it from Project Settings," a
// promise this component finally keeps.
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
import { useWorkspace } from '../../context/WorkspaceContext'
import type { Repo } from '../../types/workspace-types'

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
  const { project, switchProject } = useWorkspace()
  const [devServers, setDevServers] = useState<DevServerOption[]>([])
  const [devServersLoading, setDevServersLoading] = useState(false)
  const [selectedDevServerId, setSelectedDevServerId] = useState('')
  const [saving, setSaving] = useState(false)

  // Phase 10 (project.repos.dev_server_id): each repo now carries its own
  // binding instead of inheriting one implicit project-wide host. Fetch the
  // repo list here (same repo.list channel ProjectSettings' Repos tab uses)
  // purely to decide which UI below applies — a project with 0-1 repos still
  // has one unambiguous "the project's dev server" concept and keeps the
  // exact selector/flow this component always had; a project with 2+ repos
  // may genuinely span hosts, so those get their own per-repo selectors.
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

  // Why sync from `project.devServerId` instead of initializing state once:
  // switchProject (called after a successful save, below) re-fetches the
  // project, and the picker should reflect whatever is actually persisted —
  // including on first mount, and after another tab/session changes it.
  useEffect(() => {
    setSelectedDevServerId(project?.devServerId ?? '')
  }, [project?.devServerId])

  const hasChange =
    selectedDevServerId !== '' && selectedDevServerId !== (project?.devServerId ?? '')

  const handleSave = async (): Promise<void> => {
    if (!hasChange) {
      return
    }
    setSaving(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'project.rebindDevServer', {
        projectId,
        newDevServerId: selectedDevServerId
      })
      toast.success('Dev server updated')
      await switchProject(projectId)
    } catch (err) {
      toast.error(describeError(err, 'Failed to update the dev server.'))
    } finally {
      setSaving(false)
    }
  }

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

  const devServerSelectOptions = (
    <SelectContent>
      {devServers.map((ds) => (
        <SelectItem key={ds.id} value={ds.id}>
          {ds.name}
        </SelectItem>
      ))}
    </SelectContent>
  )

  if (repos.length > 1) {
    return (
      <div className="space-y-3">
        <div className="space-y-1.5">
          <Label>Dev servers</Label>
          <p className="text-xs text-muted-foreground">
            This project has multiple repos — each repo has its own dev server binding. Changing one
            is blocked while a workflow or task is actively running against that repo.
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
                  {devServerSelectOptions}
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

  return (
    <div className="space-y-3">
      <div className="space-y-1.5">
        <Label>Dev server</Label>
        <p className="text-xs text-muted-foreground">
          The dev server this project&apos;s repos and worktrees run on. Changing it is blocked
          while a workflow or task is actively running against this project.
        </p>
      </div>
      <div className="flex items-end gap-2">
        <Select value={selectedDevServerId} onValueChange={setSelectedDevServerId}>
          <SelectTrigger className="w-64" data-testid="project-dev-server-select">
            <SelectValue placeholder={devServersLoading ? 'Loading…' : 'Select a dev server'} />
          </SelectTrigger>
          {devServerSelectOptions}
        </Select>
        <Button
          type="button"
          size="sm"
          disabled={!hasChange || saving}
          onClick={() => void handleSave()}
          data-testid="project-dev-server-save"
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
      {!devServersLoading && devServers.length === 0 ? (
        <p className="text-xs text-muted-foreground">
          No dev servers available yet — add one from Settings first.
        </p>
      ) : null}
    </div>
  )
}
