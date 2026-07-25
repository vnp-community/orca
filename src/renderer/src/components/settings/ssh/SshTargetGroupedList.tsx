// SshTargetGroupedList — filter bar + grouped collapsible SSH target list (CR-002)
import { useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import {
  selectUniqueProjects,
  selectUniqueTeams,
  selectFilteredSshTargets,
  type SshTargetFilter
} from '@/store/selectors'
import { FleetFilterBar } from './FleetFilterBar'
import { SshTargetGroup } from './SshTargetGroup'
import type { SshTarget, SshConnectionState } from '../../../../../shared/ssh-types'
import { statusColor } from '../SshTargetCard'

/** Compact read-only row for a single SSH target in the fleet grouped list. */
function FleetTargetRow({
  target,
  state
}: {
  target: SshTarget
  state: SshConnectionState | undefined
}): React.JSX.Element {
  const status = state?.status ?? 'disconnected'

  return (
    <div className="flex items-center gap-2 rounded px-2 py-1 text-xs hover:bg-muted/40">
      {/* Status dot */}
      <span className={cn('size-1.5 flex-shrink-0 rounded-full', statusColor(status))} />

      {/* Label */}
      <span className={cn('flex-1 truncate font-medium', status !== 'connected' && 'text-muted-foreground')}>
        {target.label || `${target.username}@${target.host}`}
      </span>

      {/* Team badge */}
      {target.team && (
        <Badge variant="outline" className="h-4 px-1.5 py-0 text-[10px]">
          {target.team}
        </Badge>
      )}

      {/* Environment badge — color coded */}
      {target.environment && (
        <Badge
          variant="outline"
          className={cn('h-4 px-1.5 py-0 text-[10px]', {
            'border-green-500/40 text-green-600 dark:text-green-400':
              target.environment === 'development',
            'border-yellow-500/40 text-yellow-600 dark:text-yellow-400':
              target.environment === 'staging',
            'border-red-500/40 text-red-600 dark:text-red-400':
              target.environment === 'production'
          })}
        >
          {target.environment}
        </Badge>
      )}
    </div>
  )
}

export function SshTargetGroupedList(): React.JSX.Element {
  const [filter, setFilter] = useState<SshTargetFilter>({})

  const sshTargets = useAppStore((s) => s.sshTargets)
  const connectionStates = useAppStore((s) => s.sshConnectionStates)
  const collapsedGroups = useAppStore((s) => s.collapsedSshGroups)
  const toggleGroup = useAppStore((s) => s.toggleSshGroupCollapsed)

  // Run selectors — use state snapshot to avoid extra store reads
  const projects = useMemo(() => selectUniqueProjects({ sshTargets }), [sshTargets])
  const teams = useMemo(() => selectUniqueTeams({ sshTargets }), [sshTargets])

  // Apply filter
  const filteredTargets = useMemo(
    () => selectFilteredSshTargets({ sshTargets }, filter),
    [sshTargets, filter]
  )

  // Group by project — named projects first, __unassigned__ last
  const groupedTargets = useMemo(() => {
    return filteredTargets.reduce<Record<string, SshTarget[]>>((acc, t) => {
      const key = t.project ?? '__unassigned__'
      if (!acc[key]) acc[key] = []
      acc[key].push(t)
      return acc
    }, {})
  }, [filteredTargets])

  const groupKeys = Object.keys(groupedTargets).sort((a, b) => {
    if (a === '__unassigned__') return 1
    if (b === '__unassigned__') return -1
    return a.localeCompare(b)
  })

  return (
    <div className="flex flex-col gap-1">
      <FleetFilterBar
        filter={filter}
        onFilterChange={setFilter}
        projects={projects}
        teams={teams}
      />

      <div className="mt-3 space-y-1">
        {groupKeys.map((groupKey) => (
          <SshTargetGroup
            key={groupKey}
            label={
              groupKey === '__unassigned__'
                ? translate('fleet.group.unassigned', 'Unassigned')
                : groupKey
            }
            targets={groupedTargets[groupKey]}
            connectionStates={connectionStates}
            isCollapsed={collapsedGroups[groupKey] ?? false}
            onToggleCollapse={() => toggleGroup(groupKey)}
            renderTarget={(target, state) => (
              <FleetTargetRow key={target.id} target={target} state={state} />
            )}
          />
        ))}

        {groupKeys.length === 0 && (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {translate('fleet.empty', 'No SSH hosts match your filter.')}
          </div>
        )}
      </div>
    </div>
  )
}
