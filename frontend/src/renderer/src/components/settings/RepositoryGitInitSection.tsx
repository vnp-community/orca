import { useEffect, useState } from 'react'
import { FolderGit2 } from 'lucide-react'
import type { Repo } from '../../../../shared/types'
import { useAppStore } from '../../store'
import { checkRepoIsNotAGitRepo } from '@/lib/repo-git-status-check'
import { Button } from '../ui/button'
import { Label } from '../ui/label'
import { SearchableSetting } from './SearchableSetting'
import { matchesSettingsSearch } from './settings-search'
import type { SettingsSearchEntry } from './settings-search'
import { translate } from '@/i18n/i18n'

type RepositoryGitInitSectionProps = {
  repo: Repo
  forceVisible: boolean
  searchQuery: string
  searchEntries: SettingsSearchEntry[]
}

/** Proactively detects (once per repo per Settings session, via
 *  repo-git-status-check.ts's read-only worktree.detectedList probe) a repo
 *  whose folder was added to Orca but never `git init`'d, and offers the
 *  same remedy WorktreeCreationPanel's reactive path already ships —
 *  without waiting for a worktree-create attempt to fail first. */
export function RepositoryGitInitSection({
  repo,
  forceVisible,
  searchQuery,
  searchEntries
}: RepositoryGitInitSectionProps): React.JSX.Element | null {
  const activeRuntimeEnvironmentId = useAppStore(
    (s) => s.settings?.activeRuntimeEnvironmentId ?? null
  )
  const openModal = useAppStore((s) => s.openModal)
  const [isNotAGitRepo, setIsNotAGitRepo] = useState<boolean | null>(null)

  useEffect(() => {
    let cancelled = false
    setIsNotAGitRepo(null)
    checkRepoIsNotAGitRepo(
      { id: repo.id, projectId: repo.projectId },
      { activeRuntimeEnvironmentId }
    ).then((result) => {
      if (!cancelled) {
        setIsNotAGitRepo(result)
      }
    })
    return () => {
      cancelled = true
    }
  }, [repo.id, repo.projectId, activeRuntimeEnvironmentId])

  if (!isNotAGitRepo || (!forceVisible && !matchesSettingsSearch(searchQuery, searchEntries))) {
    return null
  }

  return (
    <SearchableSetting
      title={translate(
        'auto.components.settings.RepositoryGitInitSection.title',
        'Initialize Git repo'
      )}
      description={translate(
        'auto.components.settings.RepositoryGitInitSection.description',
        'This folder isn’t a Git repository yet.'
      )}
      keywords={[repo.displayName, 'git init', 'git remote', 'not a git repository']}
      className="space-y-3"
      forceVisible={forceVisible}
    >
      <div className="flex items-start justify-between gap-3 rounded-md border border-border/70 bg-muted/35 px-3 py-2.5">
        <div className="space-y-0.5">
          <Label className="text-sm font-semibold">
            {translate(
              'auto.components.settings.RepositoryGitInitSection.title',
              'Initialize Git repo'
            )}
          </Label>
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.components.settings.RepositoryGitInitSection.description',
              'This folder isn’t a Git repository yet.'
            )}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5"
          onClick={() =>
            openModal('init-repo-as-git', {
              repoId: repo.id,
              folderPath: repo.path,
              onSuccess: () => setIsNotAGitRepo(false)
            })
          }
        >
          <FolderGit2 className="size-3.5" />
          {translate(
            'auto.components.settings.RepositoryGitInitSection.action',
            'Initialize as Git repo'
          )}
        </Button>
      </div>
    </SearchableSetting>
  )
}
