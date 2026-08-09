// FleetFilterBar — search + project/environment dropdown filters for SSH fleet view (CR-002)
import { Search, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { translate } from '@/i18n/i18n'
import type { SshTargetFilter } from '@/store/selectors'

type FleetFilterBarProps = {
  filter: SshTargetFilter
  onFilterChange: (filter: SshTargetFilter) => void
  /** Unique project names to display in the project dropdown. */
  projects: string[]
  /** Unique team names (reserved for Phase 2 — currently unused in UI). */
  teams?: string[]
}

export function FleetFilterBar({
  filter,
  onFilterChange,
  projects
}: FleetFilterBarProps): React.JSX.Element {
  const hasActiveFilter =
    Boolean(filter.project) ||
    Boolean(filter.team) ||
    Boolean(filter.environment) ||
    Boolean(filter.search)

  return (
    <div className="flex flex-wrap items-center gap-2 border-b pb-3">
      {/* Search input */}
      <div className="relative min-w-[180px] flex-1">
        <Search className="pointer-events-none absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder={translate('fleet.filter.search', 'Search hosts...')}
          value={filter.search ?? ''}
          onChange={(e) =>
            onFilterChange({ ...filter, search: e.target.value || undefined })
          }
          className="h-9 pl-8"
        />
      </div>

      {/* Project filter — only shown when at least one project exists */}
      {projects.length > 0 && (
        <Select
          value={filter.project ?? 'all'}
          onValueChange={(v) =>
            onFilterChange({
              ...filter,
              project: v === 'all' ? undefined : v
            })
          }
        >
          <SelectTrigger className="h-9 w-[160px]">
            <SelectValue
              placeholder={translate('fleet.filter.allProjects', 'All projects')}
            />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              {translate('fleet.filter.allProjects', 'All projects')}
            </SelectItem>
            {projects.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {/* Environment filter */}
      <Select
        value={filter.environment ?? 'all'}
        onValueChange={(v) =>
          onFilterChange({
            ...filter,
            environment:
              v === 'all' ? undefined : (v as 'development' | 'staging' | 'production')
          })
        }
      >
        <SelectTrigger className="h-9 w-[140px]">
          <SelectValue placeholder={translate('fleet.filter.allEnvs', 'All envs')} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">
            {translate('fleet.filter.allEnvs', 'All envs')}
          </SelectItem>
          <SelectItem value="development">Development</SelectItem>
          <SelectItem value="staging">Staging</SelectItem>
          <SelectItem value="production">Production</SelectItem>
        </SelectContent>
      </Select>

      {/* Clear all filters — only visible when any filter is active */}
      {hasActiveFilter && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onFilterChange({})}
          className="h-9 gap-1 text-muted-foreground hover:text-foreground"
        >
          <X className="size-3.5" />
          {translate('fleet.filter.clear', 'Clear')}
        </Button>
      )}
    </div>
  )
}
