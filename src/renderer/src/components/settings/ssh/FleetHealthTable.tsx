// FleetHealthTable — per-server health metrics table (CR-005, TASK-005-D)
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { FleetServerStatusBadge } from './FleetServerStatusBadge'
import type { SshTarget, SshConnectionState } from '../../../../../shared/ssh-types'
import type { ServerHealthMetrics } from '@/store/slices/ssh'

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

type FleetHealthTableProps = {
  targets: SshTarget[]
  /** Why: connectionStates is a Map<string, SshConnectionState> in the store. */
  connectionStates: Map<string, SshConnectionState>
  healthMetrics: Record<string, ServerHealthMetrics>
}

export function FleetHealthTable({
  targets,
  connectionStates,
  healthMetrics
}: FleetHealthTableProps): React.JSX.Element {
  if (targets.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-muted-foreground">
        {translate('fleet.health.empty', 'No SSH servers configured.')}
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-md border">
      {/* Table header */}
      <div className="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_1fr] border-b bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground">
        <span>{translate('fleet.health.server', 'Server')}</span>
        <span>{translate('fleet.health.project', 'Project')}</span>
        <span>{translate('fleet.health.status', 'Status')}</span>
        <span>{translate('fleet.health.uptime', 'Uptime')}</span>
        <span>{translate('fleet.health.relay', 'Relay')}</span>
        <span>{translate('fleet.health.disk', 'Disk')}</span>
      </div>

      {/* Table rows */}
      {targets.map((target) => {
        const connState = connectionStates.get(target.id)
        const health = healthMetrics[target.id]
        const status = connState?.status ?? 'disconnected'

        return (
          <div
            key={target.id}
            className="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_1fr] items-center border-b px-4 py-2.5 last:border-0 hover:bg-muted/20"
          >
            {/* Server name + host */}
            <div className="flex items-center gap-2 min-w-0">
              {/* Status dot */}
              <span
                className={cn('size-2 flex-shrink-0 rounded-full', {
                  'bg-green-500': status === 'connected',
                  'bg-blue-500 animate-pulse': status === 'connecting' || status === 'deploying-relay',
                  'bg-destructive': status === 'error' || status === 'reconnection-failed' || status === 'auth-failed',
                  'bg-muted-foreground/30': status === 'disconnected'
                })}
              />
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{target.label}</p>
                <p className="truncate text-xs text-muted-foreground">{target.host}</p>
              </div>
            </div>

            {/* Project badge */}
            <div>
              {target.project ? (
                <Badge variant="secondary" className="text-xs">
                  {target.project}
                </Badge>
              ) : (
                <span className="text-xs text-muted-foreground">—</span>
              )}
            </div>

            {/* Connection status badge */}
            <div>
              <FleetServerStatusBadge status={status} />
            </div>

            {/* Uptime */}
            <div>
              {health?.uptimeSeconds != null ? (
                <span className="text-sm tabular-nums">{formatUptime(health.uptimeSeconds)}</span>
              ) : (
                <span className="text-xs text-muted-foreground">—</span>
              )}
            </div>

            {/* Relay version */}
            <div>
              {health?.relayVersion ? (
                <span className="font-mono text-xs">v{health.relayVersion}</span>
              ) : (
                <span className="text-xs text-muted-foreground">—</span>
              )}
            </div>

            {/* Disk usage */}
            <div>
              {health?.diskUsagePercent != null ? (
                <div className="flex items-center gap-2">
                  <Progress
                    value={health.diskUsagePercent}
                    className={cn(
                      'h-1.5 w-14',
                      health.diskUsagePercent > 85 && '[&>div]:bg-destructive',
                      health.diskUsagePercent > 70 &&
                        health.diskUsagePercent <= 85 &&
                        '[&>div]:bg-yellow-500'
                    )}
                  />
                  <span className="text-xs tabular-nums text-muted-foreground">
                    {health.diskUsagePercent}%
                  </span>
                </div>
              ) : (
                <span className="text-xs text-muted-foreground">—</span>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
