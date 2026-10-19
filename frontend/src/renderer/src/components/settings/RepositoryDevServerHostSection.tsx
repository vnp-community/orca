import type { Repo } from '../../../../shared/types'
import { useAppStore } from '../../store'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { SearchableSetting } from './SearchableSetting'
import { translate } from '@/i18n/i18n'
import { getDevServerStatusLabel } from '@/lib/dev-server-status-label'
import { isUnboundWebRepo } from '@/lib/repo-host-binding'
import { isWebClientLocation } from '@/lib/web-client-location'

/**
 * Web only: the dev server a project's git runs on, with "Change dev server" (repo.rebindDevServer).
 * A repo saved without one shows an inline warning; it is never re-bound automatically.
 */
export function RepositoryDevServerHostSection({
  repo,
  forceVisible
}: {
  repo: Repo
  forceVisible: boolean
}): React.JSX.Element | null {
  const devServers = useAppStore((state) => state.devServers)
  const openModal = useAppStore((state) => state.openModal)
  if (!isWebClientLocation()) {
    return null
  }
  const devServerId = repo.devServerId?.trim() ?? ''
  const server = devServerId ? devServers?.find((entry) => entry.id === devServerId) : undefined
  const unbound = isUnboundWebRepo(repo)
  return (
    <SearchableSetting
      title={translate(
        'auto.components.settings.RepositoryDevServerHostSection.title',
        'Dev server'
      )}
      description={translate(
        'auto.components.settings.RepositoryDevServerHostSection.description',
        'The dev server this project lives on. Orca runs git and agents there.'
      )}
      keywords={[repo.displayName, 'host', 'dev server', 'rebind', 'change host']}
      className="space-y-2"
      forceVisible={forceVisible}
    >
      <Label className="text-sm font-semibold">
        {translate('auto.components.settings.RepositoryDevServerHostSection.title', 'Dev server')}
      </Label>
      <div className="flex flex-wrap items-center gap-3">
        {unbound ? (
          <p
            role="alert"
            data-testid="repo-host-unbound-warning"
            className="text-xs text-destructive"
          >
            {translate(
              'auto.components.settings.RepositoryDevServerHostSection.unbound',
              'No dev server is assigned, so Orca cannot run git for this project. Choose one.'
            )}
          </p>
        ) : (
          <p className="text-xs text-muted-foreground" data-testid="repo-host-summary">
            <span className="text-foreground">{server?.name || devServerId}</span>
            {server
              ? ` · ${getDevServerStatusLabel(server.status)}`
              : ` · ${translate('auto.components.settings.RepositoryDevServerHostSection.unknownStatus', 'status unknown')}`}
            {repo.devServerInherited
              ? ` · ${translate('auto.components.settings.RepositoryDevServerHostSection.fromProject', 'from the project')}`
              : ''}
          </p>
        )}
        <Button
          type="button"
          size="sm"
          variant="outline"
          data-testid="repo-host-change"
          onClick={() => openModal('repo-host-picker', { repoId: repo.id })}
        >
          {translate(
            'auto.components.settings.RepositoryDevServerHostSection.change',
            'Change dev server'
          )}
        </Button>
      </div>
    </SearchableSetting>
  )
}
