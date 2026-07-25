# TASK-005-C — Tạo FleetHealthDashboard

**Task ID:** TASK-005-C  
**CR:** CR-005 — Fleet Health Monitoring  
**Solution Ref:** SOL-CR-005, Section 4.1  
**Dependencies:** TASK-005-A, TASK-005-B  
**Estimated:** 2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `FleetHealthDashboard` — main panel cho Fleet Health tab trong SSH settings. Gồm summary cards, alerts strip, và FleetHealthTable (TASK-005-D).

---

## Files cần tạo/sửa

| File | Action |
|------|--------|
| `src/renderer/src/components/settings/ssh/FleetHealthDashboard.tsx` | CREATE |
| `src/renderer/src/components/settings/ssh/FleetSummaryCard.tsx` | CREATE |
| `src/renderer/src/components/settings/ssh/SshSettingsPanel.tsx` | MODIFY — thêm Fleet Health tab |

---

## Bước 1: Tạo FleetSummaryCard.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetSummaryCard.tsx
import { cn } from '@/lib/utils'

type CardVariant = 'default' | 'success' | 'warning' | 'destructive'

type FleetSummaryCardProps = {
  label: string
  value: number
  variant?: CardVariant
}

const variantStyles: Record<CardVariant, string> = {
  default: 'bg-muted/50',
  success: 'bg-green-500/10 border-green-500/20',
  warning: 'bg-yellow-500/10 border-yellow-500/20',
  destructive: 'bg-destructive/10 border-destructive/20',
}

const valueStyles: Record<CardVariant, string> = {
  default: 'text-foreground',
  success: 'text-green-600 dark:text-green-400',
  warning: 'text-yellow-600 dark:text-yellow-400',
  destructive: 'text-destructive',
}

export function FleetSummaryCard({
  label,
  value,
  variant = 'default',
}: FleetSummaryCardProps) {
  return (
    <div
      className={cn(
        'rounded-lg border p-3 space-y-1',
        variantStyles[variant]
      )}
    >
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
        {label}
      </p>
      <p className={cn('text-2xl font-bold tabular-nums', valueStyles[variant])}>
        {value}
      </p>
    </div>
  )
}
```

## Bước 2: Tạo FleetHealthDashboard.tsx

```typescript
// src/renderer/src/components/settings/ssh/FleetHealthDashboard.tsx
import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useFleetHealthPolling } from '@/hooks/useFleetHealthPolling'
import { FleetSummaryCard } from './FleetSummaryCard'
import { FleetAlertStrip } from './FleetAlertStrip'
import { FleetHealthTable } from './FleetHealthTable'

// Relative time formatter
function formatRelativeTime(ts: number): string {
  const diffMs = Date.now() - ts
  const diffSec = Math.round(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.round(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  return `${Math.round(diffMin / 60)}h ago`
}

export function FleetHealthDashboard() {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const connectionStates = useAppStore((s) => s.sshConnectionStates)
  const healthMetrics = useAppStore((s) => s.serverHealthMetrics)
  const lastCheck = useAppStore((s) => s.lastFleetHealthCheck)
  const alerts = useAppStore(
    (s) => s.fleetAlerts.filter((a) => !a.dismissed)
  )

  // Enable polling when this component is mounted
  useFleetHealthPolling(true)

  // Summary stats
  const summary = useMemo(() => {
    let connected = 0
    let error = 0
    for (const target of sshTargets) {
      const status = connectionStates[target.id]?.status
      if (status === 'connected') connected++
      else if (status === 'error' || status === 'reconnection-failed') error++
    }
    return {
      total: sshTargets.length,
      connected,
      error,
      disconnected: sshTargets.length - connected - error,
    }
  }, [sshTargets, connectionStates])

  return (
    <div className="space-y-4">
      {/* Alerts strip */}
      {alerts.length > 0 && <FleetAlertStrip alerts={alerts} />}

      {/* Summary cards */}
      <div className="grid grid-cols-4 gap-3">
        <FleetSummaryCard
          label={translate('fleet.health.total', 'Total')}
          value={summary.total}
          variant="default"
        />
        <FleetSummaryCard
          label={translate('fleet.health.connected', 'Connected')}
          value={summary.connected}
          variant="success"
        />
        <FleetSummaryCard
          label={translate('fleet.health.disconnected', 'Offline')}
          value={summary.disconnected}
          variant="warning"
        />
        <FleetSummaryCard
          label={translate('fleet.health.error', 'Error')}
          value={summary.error}
          variant="destructive"
        />
      </div>

      {/* Last check info */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {lastCheck ? (
          <span>
            {translate('fleet.health.lastCheck', 'Last checked')}:{' '}
            {formatRelativeTime(lastCheck)}
          </span>
        ) : (
          <span>{translate('fleet.health.checking', 'Checking...')}</span>
        )}
        <Button
          variant="ghost"
          size="sm"
          className="h-5 px-2 text-xs"
          onClick={() => window.api.ssh.refreshFleetHealth?.()}
        >
          {translate('fleet.health.refresh', 'Refresh now')}
        </Button>
      </div>

      {/* Per-server table */}
      <FleetHealthTable
        targets={sshTargets}
        connectionStates={connectionStates}
        healthMetrics={healthMetrics}
      />
    </div>
  )
}
```

## Bước 3: Cập nhật SshSettingsPanel — thêm "Fleet Health" tab

```bash
# Tìm SshSettingsPanel hoặc tương đương:
grep -rn "SshSettings\|SshPanel\|SSH.*Panel" src/renderer/src/components/settings/ | head -10
```

Thêm tab:

```typescript
// Import:
import { FleetHealthDashboard } from './FleetHealthDashboard'
import { FleetHealthAlertBadge } from './FleetHealthAlertBadge'

// TabsList:
<TabsTrigger value="fleet-health">
  {translate('settings.ssh.fleetHealth', 'Fleet Health')}
  <FleetHealthAlertBadge />
</TabsTrigger>

// TabsContent:
<TabsContent value="fleet-health">
  <FleetHealthDashboard />
</TabsContent>
```

Tạo `FleetHealthAlertBadge` inline:

```typescript
function FleetHealthAlertBadge() {
  const count = useAppStore(
    (s) => s.fleetAlerts.filter((a) => !a.dismissed).length
  )
  if (count === 0) return null
  return (
    <span className="ml-1.5 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-destructive-foreground">
      {count > 9 ? '9+' : count}
    </span>
  )
}
```

---

## Acceptance Criteria

- [x] "Fleet Health" tab xuất hiện trong SSH Settings
- [x] Badge count hiển thị số alerts chưa dismiss
- [x] 4 summary cards: Total, Connected, Offline, Error
- [x] Last checked timestamp + Refresh button
- [x] `useFleetHealthPolling(true)` active khi tab visible
- [x] Alerts strip hiện khi có undismissed alerts
- [x] TypeScript compile clean

---

## Implementation Notes

> **Completed:** 2026-07-23 | `FleetHealthDashboard.tsx`: SshPane Fleet Health button with undismissed alert badge count, collapsible section, 4 summary cards (total/connected/offline/error), last-checked timestamp + Refresh button, useFleetHealthPolling(true) active when visible. TypeScript: ✅ 0 errors.
