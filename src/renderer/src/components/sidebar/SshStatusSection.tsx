// SshStatusSection — compact per-project SSH connection summary in the left sidebar (CR-002)
import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'

type ProjectCounts = { total: number; connected: number }

export function SshStatusSection(): React.JSX.Element | null {
  const sshTargets = useAppStore((s) => s.sshTargets)
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  // Compute per-project summary
  const projectSummary = useMemo((): [string, ProjectCounts][] => {
    const counts: Record<string, ProjectCounts> = {}
    for (const t of sshTargets) {
      const key = t.project ?? '__unassigned__'
      if (!counts[key]) counts[key] = { total: 0, connected: 0 }
      counts[key].total++
      if (connectionStates.get(t.id)?.status === 'connected') {
        counts[key].connected++
      }
    }
    return Object.entries(counts).sort(([a], [b]) => {
      if (a === '__unassigned__') return 1
      if (b === '__unassigned__') return -1
      return a.localeCompare(b)
    })
  }, [sshTargets, connectionStates])

  // Hide when there are no SSH targets configured
  if (sshTargets.length === 0) return null

  return (
    <div className="px-2 py-1.5">
      <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground/70">
        {translate('sidebar.sshServers', 'SSH Servers')}
      </p>
      <div className="space-y-0.5">
        {projectSummary.map(([project, counts]) => {
          const allConnected = counts.connected === counts.total
          const someConnected = counts.connected > 0 && !allConnected
          // noneConnected = !allConnected && !someConnected

          return (
            <div key={project} className="flex items-center gap-1.5 text-xs">
              {/* Status dot */}
              <span
                className={cn('size-1.5 flex-shrink-0 rounded-full', {
                  'bg-green-500': allConnected,
                  'bg-yellow-500': someConnected,
                  'bg-muted-foreground/40': !allConnected && !someConnected
                })}
              />
              {/* Project label */}
              <span className="flex-1 truncate text-muted-foreground">
                {project === '__unassigned__'
                  ? translate('sidebar.sshUnassigned', 'Other')
                  : project}
              </span>
              {/* Connected/total count */}
              <span className="tabular-nums text-muted-foreground/60">
                {counts.connected}/{counts.total}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
