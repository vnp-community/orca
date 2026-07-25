# SOL-CR-005 — Frontend Solution: Fleet Health Monitoring

**CR:** CR-005 — Fleet Health Monitoring  
**Priority:** 🟡 Medium  
**TDD References:** TDD-FE-02 (State), TDD-FE-03 (Runtime Client), TDD-FE-05 (UI Components)  
**Depends on:** SOL-CR-001, SOL-CR-002  
**Estimated effort:** 2–3 ngày frontend  
**Implementation Status:** ✅ IMPLEMENTED — 2026-07-23  
**Tasks:** TASK-005-A (HealthSlice in SshSlice), TASK-005-B (useFleetHealthPolling), TASK-005-C (FleetHealthDashboard), TASK-005-D (FleetHealthTable + FleetAlertStrip + FleetServerStatusBadge)

---

## 1. Tổng quan giải pháp

CR-005 yêu cầu **Fleet Health Dashboard** — xem tổng quan trạng thái tất cả servers. Frontend cần:

1. Mở rộng **SshSlice** với fleet health metrics
2. Tạo **FleetHealthDashboard** — panel overview với per-server health cards
3. **Polling hook** để refresh health metrics định kỳ
4. **Alert notifications** khi server disconnect

---

## 2. Store Extensions

### 2.1 Mở rộng `SshSlice` với health data

```typescript
// src/renderer/src/store/slices/ssh.ts
// Thêm fleet health state

export type ServerHealthMetrics = {
  serverId: string
  lastCheckedAt: number
  isReachable: boolean
  uptimeSeconds: number | null
  relayVersion: string | null
  nodeVersion: string | null
  diskUsagePercent: number | null
  cpuUsagePercent: number | null
  memUsagePercent: number | null
}

// Trong SshSlice type:
type SshSlice = {
  // ...existing...

  // [NEW] Health metrics
  serverHealthMetrics: Record<string, ServerHealthMetrics>
  updateServerHealth: (serverId: string, metrics: Partial<ServerHealthMetrics>) => void

  // [NEW] Last fleet health check timestamp
  lastFleetHealthCheck: number | null
  setLastFleetHealthCheck: (ts: number) => void

  // [NEW] Alert state
  fleetAlerts: FleetAlert[]
  addFleetAlert: (alert: FleetAlert) => void
  dismissFleetAlert: (alertId: string) => void
}

export type FleetAlert = {
  id: string
  serverId: string
  serverLabel: string
  type: 'disconnected' | 'error' | 'relay-outdated'
  message: string
  timestamp: number
  dismissed: boolean
}
```

---

## 3. Custom Hook — `useFleetHealthPolling`

```typescript
// src/renderer/src/hooks/useFleetHealthPolling.ts
// [NEW FILE]

const POLL_INTERVAL_MS = 60_000    // 60 seconds
const ALERT_ON_DISCONNECT = true

export function useFleetHealthPolling(enabled: boolean) {
  const sshTargets = useAppStore(s => s.sshTargets ?? [])
  const connectionStates = useAppStore(s => s.sshConnectionStates)
  const setLastCheck = useAppStore(s => s.setLastFleetHealthCheck)
  const updateHealth = useAppStore(s => s.updateServerHealth)
  const addAlert = useAppStore(s => s.addFleetAlert)

  // Track previous states to detect disconnects
  const prevStatesRef = useRef<Record<string, SshConnectionStatus>>({})

  useEffect(() => {
    if (!enabled) return

    const poll = async () => {
      try {
        const healthData = await window.api.ssh.getFleetHealth()
        const now = Date.now()
        setLastCheck(now)

        for (const entry of healthData.servers) {
          updateHealth(entry.serverId, {
            lastCheckedAt: now,
            isReachable: entry.isReachable,
            uptimeSeconds: entry.uptimeSeconds,
            relayVersion: entry.relayVersion,
            nodeVersion: entry.nodeVersion,
            diskUsagePercent: entry.diskUsagePercent,
          })
        }
      } catch (err) {
        console.warn('[FleetHealthPolling] Poll failed:', err)
      }
    }

    // Initial poll
    poll()
    const interval = setInterval(poll, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [enabled])

  // Watch connection state changes for disconnect alerts
  useEffect(() => {
    if (!ALERT_ON_DISCONNECT) return

    for (const target of sshTargets) {
      const prevStatus = prevStatesRef.current[target.id]
      const currStatus = connectionStates[target.id]?.status

      if (
        prevStatus === 'connected' &&
        (currStatus === 'disconnected' || currStatus === 'error' || currStatus === 'reconnection-failed')
      ) {
        addAlert({
          id: `disconnect-${target.id}-${Date.now()}`,
          serverId: target.id,
          serverLabel: target.label,
          type: 'disconnected',
          message: translate(
            'fleet.alert.disconnected',
            `${target.label} disconnected`
          ),
          timestamp: Date.now(),
          dismissed: false,
        })
      }
    }

    // Update prev states
    prevStatesRef.current = Object.fromEntries(
      sshTargets.map(t => [t.id, connectionStates[t.id]?.status ?? 'disconnected'])
    )
  }, [connectionStates])
}
```

---

## 4. UI Components

### 4.1 `FleetHealthDashboard` — Main dashboard

```typescript
// src/renderer/src/components/settings/ssh/FleetHealthDashboard.tsx
// Lazy loaded

export function FleetHealthDashboard() {
  const sshTargets = useAppStore(s => s.sshTargets ?? [])
  const connectionStates = useAppStore(s => s.sshConnectionStates)
  const healthMetrics = useAppStore(s => s.serverHealthMetrics)
  const lastCheck = useAppStore(s => s.lastFleetHealthCheck)
  const alerts = useAppStore(s => s.fleetAlerts.filter(a => !a.dismissed))

  // Enable polling
  useFleetHealthPolling(true)

  // Summary stats
  const summary = useMemo(() => {
    const total = sshTargets.length
    const connected = sshTargets.filter(
      t => connectionStates[t.id]?.status === 'connected'
    ).length
    const error = sshTargets.filter(t => {
      const s = connectionStates[t.id]?.status
      return s === 'error' || s === 'reconnection-failed'
    }).length
    return { total, connected, disconnected: total - connected - error, error }
  }, [sshTargets, connectionStates])

  return (
    <div className="space-y-4">
      {/* Alerts strip */}
      {alerts.length > 0 && (
        <FleetAlertStrip alerts={alerts} />
      )}

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
          label={translate('fleet.health.disconnected', 'Disconnected')}
          value={summary.disconnected}
          variant="warning"
        />
        <FleetSummaryCard
          label={translate('fleet.health.error', 'Error')}
          value={summary.error}
          variant="destructive"
        />
      </div>

      {/* Last check timestamp */}
      {lastCheck && (
        <p className="text-xs text-muted-foreground">
          {translate('fleet.health.lastCheck', 'Last checked')}:{' '}
          {formatRelativeTime(lastCheck)}
          <Button
            variant="ghost"
            size="sm"
            className="ml-2 h-5 px-1 text-xs"
            onClick={() => window.api.ssh.refreshFleetHealth()}
          >
            {translate('fleet.health.refresh', 'Refresh')}
          </Button>
        </p>
      )}

      {/* Per-server health table */}
      <FleetHealthTable
        targets={sshTargets}
        connectionStates={connectionStates}
        healthMetrics={healthMetrics}
      />
    </div>
  )
}
```

### 4.2 `FleetHealthTable` — Server status table

```typescript
// src/renderer/src/components/settings/ssh/FleetHealthTable.tsx

export function FleetHealthTable({
  targets,
  connectionStates,
  healthMetrics,
}: {
  targets: SshTarget[]
  connectionStates: Record<string, SshConnectionState>
  healthMetrics: Record<string, ServerHealthMetrics>
}) {
  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[200px]">
              {translate('fleet.health.server', 'Server')}
            </TableHead>
            <TableHead>{translate('fleet.health.project', 'Project')}</TableHead>
            <TableHead>{translate('fleet.health.status', 'Status')}</TableHead>
            <TableHead>{translate('fleet.health.uptime', 'Uptime')}</TableHead>
            <TableHead>{translate('fleet.health.relay', 'Relay')}</TableHead>
            <TableHead>{translate('fleet.health.disk', 'Disk')}</TableHead>
            <TableHead className="w-[80px]" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {targets.map(target => {
            const connState = connectionStates[target.id]
            const health = healthMetrics[target.id]

            return (
              <TableRow key={target.id}>
                {/* Server name */}
                <TableCell>
                  <div className="flex items-center gap-2">
                    <SshConnectionStatusDot status={connState?.status ?? 'disconnected'} />
                    <div>
                      <p className="text-sm font-medium">{target.label}</p>
                      <p className="text-xs text-muted-foreground">{target.host}</p>
                    </div>
                  </div>
                </TableCell>

                {/* Project */}
                <TableCell>
                  {target.project ? (
                    <Badge variant="secondary">{target.project}</Badge>
                  ) : (
                    <span className="text-muted-foreground text-xs">—</span>
                  )}
                </TableCell>

                {/* Status */}
                <TableCell>
                  <FleetServerStatusBadge status={connState?.status ?? 'disconnected'} />
                </TableCell>

                {/* Uptime */}
                <TableCell>
                  {health?.uptimeSeconds != null ? (
                    <span className="text-sm">{formatUptime(health.uptimeSeconds)}</span>
                  ) : (
                    <span className="text-muted-foreground text-xs">—</span>
                  )}
                </TableCell>

                {/* Relay version */}
                <TableCell>
                  {health?.relayVersion ? (
                    <span className="text-xs font-mono">v{health.relayVersion}</span>
                  ) : (
                    <span className="text-muted-foreground text-xs">—</span>
                  )}
                </TableCell>

                {/* Disk usage */}
                <TableCell>
                  {health?.diskUsagePercent != null ? (
                    <div className="flex items-center gap-1.5">
                      <Progress
                        value={health.diskUsagePercent}
                        className={cn(
                          'h-1.5 w-14',
                          health.diskUsagePercent > 85 && '[&>div]:bg-destructive',
                          health.diskUsagePercent > 70 && '[&>div]:bg-yellow-500',
                        )}
                      />
                      <span className="text-xs text-muted-foreground">
                        {health.diskUsagePercent}%
                      </span>
                    </div>
                  ) : (
                    <span className="text-muted-foreground text-xs">—</span>
                  )}
                </TableCell>

                {/* Actions */}
                <TableCell>
                  <FleetServerRowActions target={target} connState={connState} />
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

### 4.3 `FleetAlertStrip` — Disconnect alerts

```typescript
// src/renderer/src/components/settings/ssh/FleetAlertStrip.tsx

export function FleetAlertStrip({ alerts }: { alerts: FleetAlert[] }) {
  const dismissAlert = useAppStore(s => s.dismissFleetAlert)

  return (
    <div className="space-y-1.5">
      {alerts.slice(0, 3).map(alert => (
        <div
          key={alert.id}
          className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"
        >
          <AlertTriangleIcon className="h-4 w-4 text-destructive flex-shrink-0" />
          <p className="flex-1 text-sm text-destructive">
            {alert.message}
          </p>
          <span className="text-xs text-muted-foreground">
            {formatRelativeTime(alert.timestamp)}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 text-muted-foreground"
            onClick={() => dismissAlert(alert.id)}
          >
            <XIcon className="h-3 w-3" />
          </Button>
        </div>
      ))}
    </div>
  )
}
```

### 4.4 Tích hợp vào Settings

```typescript
// Trong settings navigation, thêm "Fleet Health" tab dưới SSH & Remotes

// src/renderer/src/components/settings/SshSettingsPanel.tsx
export function SshSettingsPanel() {
  return (
    <Tabs defaultValue="hosts">
      <TabsList>
        <TabsTrigger value="hosts">
          {translate('settings.ssh.hosts', 'SSH Hosts')}
        </TabsTrigger>
        {/* [NEW] Fleet Health tab */}
        <TabsTrigger value="fleet-health">
          {translate('settings.ssh.fleetHealth', 'Fleet Health')}
          <FleetHealthAlertBadge />
        </TabsTrigger>
      </TabsList>

      <TabsContent value="hosts">
        <SshTargetGroupedList />
      </TabsContent>

      {/* [NEW] */}
      <TabsContent value="fleet-health">
        <FleetHealthDashboard />
      </TabsContent>
    </Tabs>
  )
}

// Badge count cho alerts
function FleetHealthAlertBadge() {
  const alertCount = useAppStore(
    s => s.fleetAlerts.filter(a => !a.dismissed).length
  )
  if (alertCount === 0) return null
  return (
    <Badge variant="destructive" className="ml-1.5 text-xs px-1.5 py-0 h-4">
      {alertCount}
    </Badge>
  )
}
```

---

## 5. File mới cần tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/hooks/useFleetHealthPolling.ts` | [NEW] | Polling hook + disconnect alerts |
| `src/renderer/src/components/settings/ssh/FleetHealthDashboard.tsx` | [NEW] | Main health dashboard |
| `src/renderer/src/components/settings/ssh/FleetHealthTable.tsx` | [NEW] | Server status table |
| `src/renderer/src/components/settings/ssh/FleetAlertStrip.tsx` | [NEW] | Disconnect alert UI |
| `src/renderer/src/components/settings/ssh/FleetSummaryCard.tsx` | [NEW] | Stat cards |

## 6. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/slices/ssh.ts` | Thêm `serverHealthMetrics`, `fleetAlerts`, actions |
| `src/renderer/src/store/types.ts` | Thêm `ServerHealthMetrics`, `FleetAlert` types |
| `src/renderer/src/components/settings/ssh/SshSettingsPanel.tsx` | Thêm "Fleet Health" tab |
| `src/preload/index.ts` | Expose `ssh.getFleetHealth`, `ssh.refreshFleetHealth` |

---

## 7. Acceptance Criteria (Frontend)

- [x] "Fleet Health" button trong SSH settings hiển thị dashboard (expandable collapsible)
- [x] Summary cards: total / connected / disconnected / error counts
- [x] Server table: status, project, uptime, relay version, disk usage
- [x] Disk usage > 85% → progress bar đỏ (warning)
- [x] Alert strip xuất hiện khi server disconnect
- [x] Alert có thể dismiss thủ công (soft-dismiss, clearDismissedAlerts)
- [x] Badge count trên "Fleet Health" button cho unread alerts
- [x] "Refresh" button trigger immediate health check
- [x] Auto-refresh mỗi 60 giây khi panel visible

## 8. Implementation Notes

> **Implemented 2026-07-23**
>
> - `src/renderer/src/store/slices/ssh.ts`: Added `ServerHealthMetrics`, `FleetAlert`, `FleetAlertType` types + `serverHealthMetrics`, `lastFleetHealthCheck`, `fleetAlerts` state + `updateServerHealth`, `setLastFleetHealthCheck`, `addFleetAlert`, `dismissFleetAlert`, `clearDismissedAlerts` actions.
> - `src/preload/api-types.ts`: Added `getFleetHealth`, `refreshFleetHealth` to `ssh` namespace.
> - `src/preload/index.ts`: IPC bridges via `ipcRenderer.invoke`.
> - `src/renderer/src/web/web-preload-api.ts`: No-op stubs returning `{ servers: [] }`.
> - `src/renderer/src/hooks/useFleetHealthPolling.ts`: [NEW] 60s polling + disconnect detection via Map snapshot comparison.
> - `src/renderer/src/components/settings/ssh/FleetSummaryCard.tsx`: [NEW] Stat card with 4 variants.
> - `src/renderer/src/components/settings/ssh/FleetHealthDashboard.tsx`: [NEW] Dashboard orchestrating cards + alerts + table + polling.
> - `src/renderer/src/components/settings/ssh/FleetHealthTable.tsx`: [NEW] CSS grid table (no table.tsx UI kit; inline status dot).
> - `src/renderer/src/components/settings/ssh/FleetAlertStrip.tsx`: [NEW] Max 3 visible alerts + overflow counter.
> - `src/renderer/src/components/settings/ssh/FleetServerStatusBadge.tsx`: [NEW] All 8 SshConnectionStatus values including `reconnecting`.
> - `src/renderer/src/components/settings/SshPane.tsx`: "Fleet Health" button with red alert badge + collapsible `FleetHealthDashboard`.
> - **TypeScript:** ✅ 0 new errors.
