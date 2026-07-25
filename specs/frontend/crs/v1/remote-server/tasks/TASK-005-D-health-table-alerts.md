# TASK-005-D — Tạo FleetHealthTable + FleetAlertStrip

**Task ID:** TASK-005-D  
**CR:** CR-005 — Fleet Health Monitoring  
**Solution Ref:** SOL-CR-005, Section 4.2, 4.3  
**Dependencies:** TASK-005-A  
**Estimated:** 2–3 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo 2 components:
1. `FleetHealthTable` — bảng per-server với metrics (status, uptime, relay version, disk usage)
2. `FleetAlertStrip` — alert bar cho disconnect notifications

---

## Files cần tạo

| File |
|------|
| `src/renderer/src/components/settings/ssh/FleetHealthTable.tsx` |
| `src/renderer/src/components/settings/ssh/FleetAlertStrip.tsx` |
| `src/renderer/src/components/settings/ssh/FleetServerStatusBadge.tsx` |

---

## Bước 1: Tạo FleetServerStatusBadge.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetServerStatusBadge.tsx
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import type { SshConnectionStatus } from 'src/shared/ssh-types'

export function FleetServerStatusBadge({
  status,
}: {
  status: SshConnectionStatus
}) {
  const config: Record<SshConnectionStatus, { label: string; variant: string; className: string }> = {
    connected: {
      label: translate('ssh.status.connected', 'Connected'),
      variant: 'outline',
      className: 'border-green-500/40 text-green-600 dark:text-green-400',
    },
    disconnected: {
      label: translate('ssh.status.disconnected', 'Offline'),
      variant: 'outline',
      className: 'text-muted-foreground',
    },
    connecting: {
      label: translate('ssh.status.connecting', 'Connecting'),
      variant: 'outline',
      className: 'border-blue-500/40 text-blue-500',
    },
    error: {
      label: translate('ssh.status.error', 'Error'),
      variant: 'destructive',
      className: '',
    },
    'reconnection-failed': {
      label: translate('ssh.status.reconnFailed', 'Failed'),
      variant: 'destructive',
      className: '',
    },
  }

  const c = config[status] ?? config['disconnected']
  
  return (
    <Badge variant={c.variant as any} className={c.className}>
      {c.label}
    </Badge>
  )
}
```

## Bước 2: Helper functions

```typescript
// Inline trong FleetHealthTable.tsx:

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}
```

## Bước 3: Tạo FleetHealthTable.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetHealthTable.tsx
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import { FleetServerStatusBadge } from './FleetServerStatusBadge'
import { SshConnectionStatusDot } from './SshConnectionStatusDot'
import type { SshTarget, SshConnectionState } from 'src/shared/ssh-types'
import type { ServerHealthMetrics } from '@/store/types'

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
  connectionStates: Record<string, SshConnectionState>
  healthMetrics: Record<string, ServerHealthMetrics>
}

export function FleetHealthTable({
  targets,
  connectionStates,
  healthMetrics,
}: FleetHealthTableProps) {
  if (targets.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-muted-foreground">
        {translate('fleet.health.empty', 'No SSH servers configured.')}
      </div>
    )
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[220px]">
              {translate('fleet.health.server', 'Server')}
            </TableHead>
            <TableHead>{translate('fleet.health.project', 'Project')}</TableHead>
            <TableHead>{translate('fleet.health.status', 'Status')}</TableHead>
            <TableHead>{translate('fleet.health.uptime', 'Uptime')}</TableHead>
            <TableHead>{translate('fleet.health.relay', 'Relay')}</TableHead>
            <TableHead>{translate('fleet.health.disk', 'Disk')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {targets.map((target) => {
            const connState = connectionStates[target.id]
            const health = healthMetrics[target.id]

            return (
              <TableRow key={target.id}>
                {/* Server name + host */}
                <TableCell>
                  <div className="flex items-center gap-2">
                    <SshConnectionStatusDot
                      status={connState?.status ?? 'disconnected'}
                    />
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">
                        {target.label}
                      </p>
                      <p className="text-xs text-muted-foreground truncate">
                        {target.host}
                      </p>
                    </div>
                  </div>
                </TableCell>

                {/* Project */}
                <TableCell>
                  {target.project ? (
                    <Badge variant="secondary" className="text-xs">
                      {target.project}
                    </Badge>
                  ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                  )}
                </TableCell>

                {/* Connection status */}
                <TableCell>
                  <FleetServerStatusBadge
                    status={connState?.status ?? 'disconnected'}
                  />
                </TableCell>

                {/* Uptime */}
                <TableCell>
                  {health?.uptimeSeconds != null ? (
                    <span className="text-sm tabular-nums">
                      {formatUptime(health.uptimeSeconds)}
                    </span>
                  ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                  )}
                </TableCell>

                {/* Relay version */}
                <TableCell>
                  {health?.relayVersion ? (
                    <span className="font-mono text-xs">
                      v{health.relayVersion}
                    </span>
                  ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                  )}
                </TableCell>

                {/* Disk usage */}
                <TableCell>
                  {health?.diskUsagePercent != null ? (
                    <div className="flex items-center gap-2">
                      <Progress
                        value={health.diskUsagePercent}
                        className={cn(
                          'h-1.5 w-14',
                          health.diskUsagePercent > 85 &&
                            '[&>div]:bg-destructive',
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
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
```

## Bước 4: Tạo FleetAlertStrip.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetAlertStrip.tsx
import { AlertTriangleIcon, XIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import type { FleetAlert } from '@/store/types'

function formatRelativeTime(ts: number): string {
  const diff = Math.round((Date.now() - ts) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`
  return `${Math.round(diff / 3600)}h ago`
}

export function FleetAlertStrip({ alerts }: { alerts: FleetAlert[] }) {
  const dismissAlert = useAppStore((s) => s.dismissFleetAlert)

  // Show max 3 alerts, with a "+N more" indicator
  const visible = alerts.slice(0, 3)
  const overflow = alerts.length - visible.length

  return (
    <div className="space-y-1.5">
      {visible.map((alert) => (
        <div
          key={alert.id}
          className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"
        >
          <AlertTriangleIcon className="h-4 w-4 flex-shrink-0 text-destructive" />
          <p className="flex-1 text-sm text-destructive">{alert.message}</p>
          <span className="flex-shrink-0 text-xs text-muted-foreground">
            {formatRelativeTime(alert.timestamp)}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 flex-shrink-0 text-muted-foreground hover:text-foreground"
            onClick={() => dismissAlert(alert.id)}
          >
            <XIcon className="h-3 w-3" />
            <span className="sr-only">
              {translate('fleet.alert.dismiss', 'Dismiss alert')}
            </span>
          </Button>
        </div>
      ))}
      {overflow > 0 && (
        <p className="text-center text-xs text-muted-foreground">
          {translate('fleet.alert.moreAlerts', `+${overflow} more alerts`)}
        </p>
      )}
    </div>
  )
}
```

---

## Acceptance Criteria

**FleetHealthTable:**
- [x] Hiển thị tất cả servers
- [x] Columns: Server name+host, Project badge, Status badge, Uptime, Relay version, Disk %
- [x] Disk > 85% → progress bar đỏ; > 70% → vàng
- [x] Null/missing metrics → hiện "—"
- [x] Empty state khi không có servers

**FleetAlertStrip:**
- [x] Max 3 alerts hiển thị, overflow = "+N more"
- [x] X button dismiss từng alert
- [x] Timestamp relative (30s ago, 5m ago)
- [x] Không hiện khi `alerts.length === 0`

---

## Implementation Notes

> **Completed:** 2026-07-23 | `FleetHealthTable.tsx`: CSS grid (no table.tsx UI kit), columns server+host/project/status/uptime/relay/disk, disk>85%=red/>70%=yellow, null metrics show '—', empty state. `FleetAlertStrip.tsx`: max 3 + '+N more', X dismiss each, relative timestamp, hidden when no alerts. TypeScript: ✅ 0 errors.
