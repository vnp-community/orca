import { TriangleAlert } from 'lucide-react'
import type { Repo } from '../../../../shared/types'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { isUnboundWebRepo } from '@/lib/repo-host-binding'
import { isWebClientLocation } from '@/lib/web-client-location'

/**
 * Web only: which dev server a project lives on, or a "No dev server" action for a repo that was
 * saved without one (older projects are never migrated automatically).
 */
export function RepoHostBadge({ repo }: { repo: Repo }): React.JSX.Element | null {
  const devServers = useAppStore((s) => s.devServers)
  const openModal = useAppStore((s) => s.openModal)
  if (!isWebClientLocation()) {
    return null
  }
  if (isUnboundWebRepo(repo)) {
    return (
      <button
        type="button"
        data-testid="repo-host-unbound"
        onClick={(event) => {
          event.stopPropagation()
          openModal('repo-host-picker', { repoId: repo.id })
        }}
        className="inline-flex shrink-0 items-center gap-1 rounded text-[11px] font-normal text-destructive hover:underline focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        <TriangleAlert className="size-3" />
        {translate('auto.components.sidebar.RepoHostBadge.noHost', 'No dev server')}
      </button>
    )
  }
  const devServerId = repo.devServerId?.trim()
  if (!devServerId) {
    return null
  }
  const label = devServers?.find((server) => server.id === devServerId)?.name || devServerId
  return (
    <span
      data-testid="repo-host-label"
      data-host-inherited={repo.devServerInherited ? 'true' : undefined}
      title={
        repo.devServerInherited
          ? translate(
              'auto.components.sidebar.RepoHostBadge.inheritedTitle',
              '{{name}} (inherited from the project)',
              { name: label }
            )
          : undefined
      }
      className="min-w-0 max-w-[45%] shrink truncate text-[11px] font-normal text-muted-foreground"
    >
      {label}
    </span>
  )
}
