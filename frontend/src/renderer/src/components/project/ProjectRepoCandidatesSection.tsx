// ProjectRepoCandidatesSection.tsx — Project Settings' "Repos" tab: pick an
// already-existing repo (living in some other project — the legacy
// sidebar's own repo catalog) and attach it here via
// repo.assignToProject (distinct from repo.add/create, which always
// creates a brand-new repo). Candidate pool is every repo in the tenant
// (repo.list with no projectId — ListReposForTenant, gated on tenant
// membership alone) minus repos already in THIS project, optionally
// narrowed by ProjectDevServerFilterSection's dev-server filter above.
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  RuntimeRpcCallError
} from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { Repo } from '../../types/workspace-types'
import { translate } from '@/i18n/i18n'

function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

type ProjectRepoCandidatesSectionProps = {
  projectId: string
  existingRepoIds: ReadonlySet<string>
  selectedDevServerIds: ReadonlySet<string>
  onAdded: () => void
}

export function ProjectRepoCandidatesSection({
  projectId,
  existingRepoIds,
  selectedDevServerIds,
  onAdded
}: ProjectRepoCandidatesSectionProps) {
  const [candidates, setCandidates] = useState<Repo[]>([])
  const [loading, setLoading] = useState(false)
  const [addingId, setAddingId] = useState<string | null>(null)

  const loadCandidates = (): void => {
    setLoading(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // No projectId → every repo in the caller's tenant (ListReposForTenant).
    callRuntimeRpc<{ repos: Repo[] }>(target, 'repo.list', {})
      .then((result) => setCandidates(Array.isArray(result?.repos) ? result.repos : []))
      .catch(() => setCandidates([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadCandidates()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const visibleCandidates = candidates.filter(
    (repo) =>
      !existingRepoIds.has(repo.id) &&
      (selectedDevServerIds.size === 0 || selectedDevServerIds.has(repo.devServerId ?? ''))
  )

  const handleAdd = async (repo: Repo): Promise<void> => {
    setAddingId(repo.id)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'repo.assignToProject', {
        repoId: repo.id,
        targetProjectId: projectId
      })
      toast.success(`${repo.displayName || repo.url} added to this project`)
      loadCandidates()
      onAdded()
    } catch (err) {
      toast.error(describeError(err, 'Failed to add this repo to the project.'))
    } finally {
      setAddingId(null)
    }
  }

  if (loading) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate('auto.components.project.ProjectRepoCandidatesSection.loading', 'Loading…')}
      </p>
    )
  }

  if (visibleCandidates.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          'auto.components.project.ProjectRepoCandidatesSection.empty',
          'No other repos available to add.'
        )}
      </p>
    )
  }

  return (
    <div className="space-y-1.5" data-testid="repo-candidate-list">
      {visibleCandidates.map((repo) => (
        <div
          key={repo.id}
          className="flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5"
          data-testid={`repo-candidate-${repo.id}`}
        >
          <span className="min-w-0 truncate text-xs">{repo.displayName || repo.url}</span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={addingId === repo.id}
            onClick={() => void handleAdd(repo)}
            data-testid={`repo-candidate-add-${repo.id}`}
          >
            {addingId === repo.id
              ? translate('auto.components.project.ProjectRepoCandidatesSection.adding', 'Adding…')
              : translate('auto.components.project.ProjectRepoCandidatesSection.add', 'Add')}
          </Button>
        </div>
      ))}
    </div>
  )
}
