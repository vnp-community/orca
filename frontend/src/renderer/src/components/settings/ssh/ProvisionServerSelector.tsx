// ProvisionServerSelector — server checkbox list grouped by project (CR-003, TASK-003-C)
import { useMemo } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'
import type { SshTarget } from '../../../../../shared/ssh-types'

type ProvisionServerSelectorProps = {
  selectedIds: Set<string>
  onSelectionChange: (ids: Set<string>) => void
}

export function ProvisionServerSelector({
  selectedIds,
  onSelectionChange
}: ProvisionServerSelectorProps): React.JSX.Element {
  const sshTargets = useAppStore((s) => s.sshTargets)
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  // Group by project — same sort as SshTargetGroupedList
  const grouped = useMemo(() => {
    const result: Record<string, SshTarget[]> = {}
    for (const t of sshTargets) {
      const key = t.project ?? '__unassigned__'
      if (!result[key]) {result[key] = []}
      result[key].push(t)
    }
    return Object.entries(result).sort(([a], [b]) => {
      if (a === '__unassigned__') {return 1}
      if (b === '__unassigned__') {return -1}
      return a.localeCompare(b)
    })
  }, [sshTargets])

  const allSelected = selectedIds.size === sshTargets.length && sshTargets.length > 0
  const isIndeterminate = selectedIds.size > 0 && !allSelected

  const handleToggleAll = (): void => {
    if (allSelected) {
      onSelectionChange(new Set())
    } else {
      onSelectionChange(new Set(sshTargets.map((t) => t.id)))
    }
  }

  const handleToggle = (id: string, checked: boolean): void => {
    const next = new Set(selectedIds)
    if (checked) {
      next.add(id)
    } else {
      next.delete(id)
    }
    onSelectionChange(next)
  }

  if (sshTargets.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-muted-foreground">
        {translate(
          'fleet.provision.noTargets',
          'No SSH hosts configured. Import a fleet config first.'
        )}
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Select all / count header */}
      <div className="flex items-center gap-2 border-b pb-2">
        <Checkbox
          id="select-all-servers"
          checked={allSelected}
          data-state={isIndeterminate ? 'indeterminate' : undefined}
          onCheckedChange={() => handleToggleAll()}
        />
        <label
          htmlFor="select-all-servers"
          className="flex-1 cursor-pointer text-sm"
        >
          {translate('fleet.provision.selectAll', 'Select all servers')}
        </label>
        <span className="text-xs tabular-nums text-muted-foreground">
          {selectedIds.size}/{sshTargets.length}
        </span>
      </div>

      {/* Server list grouped by project */}
      <ScrollArea className="max-h-[320px] pr-2">
        <div className="space-y-3">
          {grouped.map(([project, targets]) => (
            <div key={project}>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {project === '__unassigned__'
                  ? translate('fleet.group.unassigned', 'Unassigned')
                  : project}
              </p>
              <div className="space-y-0.5">
                {targets.map((target) => {
                  const state = connectionStates.get(target.id)
                  const isConnected = state?.status === 'connected'
                  return (
                    <label
                      key={target.id}
                      className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/50"
                    >
                      <Checkbox
                        checked={selectedIds.has(target.id)}
                        onCheckedChange={(checked) =>
                          handleToggle(target.id, Boolean(checked))
                        }
                      />
                      {/* Status dot */}
                      <span
                        className={cn('size-2 flex-shrink-0 rounded-full', {
                          'bg-green-500': isConnected,
                          'bg-muted-foreground/30': !isConnected
                        })}
                      />
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{target.label}</p>
                        <p className="text-xs text-muted-foreground">{target.host}</p>
                      </div>
                      {isConnected && (
                        <Badge variant="outline" className="flex-shrink-0 text-xs text-green-600">
                          {translate('fleet.provision.relayActive', 'relay active')}
                        </Badge>
                      )}
                    </label>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}
