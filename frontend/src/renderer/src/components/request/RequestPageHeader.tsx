/**
 * RequestPageHeader — CR-REQ-018-05
 *
 * Title, project scope selector and the "Create request" slot (CR-REQ-019-06).
 *
 * @module components/request/RequestPageHeader
 */

import React, { useMemo } from 'react'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'

const ALL_PROJECTS = '__all__'

type Props = {
  onClose: () => void
  /** When provided, shows the "Create request" button. */
  onCreate?: () => void
}

export function RequestPageHeader({ onClose, onCreate }: Props): React.JSX.Element {
  const repos = useAppStore((s) => s.repos)
  const projectId = useAppStore((s) => s.requestPage.listFilters.projectId)
  const setRequestPageData = useAppStore((s) => s.setRequestPageData)
  const listFilters = useAppStore((s) => s.requestPage.listFilters)

  // Why: request-service projects are the repos' OrcaProject; local repos have none.
  const projects = useMemo(() => {
    const seen = new Map<string, string>()
    for (const repo of repos) {
      if (repo.projectId && !seen.has(repo.projectId)) {seen.set(repo.projectId, repo.displayName)}
    }
    return [...seen].map(([id, name]) => ({ id, name }))
  }, [repos])

  return (
    <div className="flex items-center gap-2 px-4 py-2 border-b border-border shrink-0">
      <h1 className="text-sm font-semibold text-foreground">
        {translate('auto.components.request.RequestPage.title', 'Requests')}
      </h1>
      {projects.length > 0 && (
        <Select
          value={projectId ?? ALL_PROJECTS}
          onValueChange={(v) =>
            setRequestPageData({
              listFilters: { ...listFilters, projectId: v === ALL_PROJECTS ? undefined : v }
            })
          }
        >
          <SelectTrigger
            size="sm"
            className="h-7 w-48 text-xs"
            aria-label={translate('auto.components.request.RequestPageHeader.project', 'Project')}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_PROJECTS}>
              {translate('auto.components.request.RequestPageHeader.allProjects', 'All projects')}
            </SelectItem>
            {projects.map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      <div className="ml-auto flex items-center gap-1">
        {onCreate && (
          <Button size="xs" onClick={onCreate}>
            <Plus className="size-3.5" aria-hidden />
            {translate('auto.components.request.RequestPageHeader.createRequest', 'Create request')}
          </Button>
        )}
        <button
          type="button"
          onClick={onClose}
          aria-label={translate('auto.components.request.RequestPage.close', 'Close requests page')}
          className="rounded p-1 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>
    </div>
  )
}
