# TASK-005-A — Thêm Health Metrics vào SshSlice

**Task ID:** TASK-005-A  
**CR:** CR-005 — Fleet Health Monitoring  
**Solution Ref:** SOL-CR-005, Section 2  
**Dependencies:** TASK-002-A  
**Estimated:** 1 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Mở rộng `SshSlice` để lưu health metrics per-server và fleet alerts (disconnect notifications).

---

## Bước thực thi

### Bước 1: Thêm types vào store/types.ts

```typescript
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

export type FleetAlertType = 'disconnected' | 'error' | 'relay-outdated'

export type FleetAlert = {
  id: string
  serverId: string
  serverLabel: string
  type: FleetAlertType
  message: string
  timestamp: number
  dismissed: boolean
}
```

### Bước 2: Mở rộng SshSlice interface

```typescript
// Thêm vào SshSlice type:
serverHealthMetrics: Record<string, ServerHealthMetrics>
updateServerHealth: (
  serverId: string,
  metrics: Partial<ServerHealthMetrics>
) => void

lastFleetHealthCheck: number | null
setLastFleetHealthCheck: (ts: number) => void

fleetAlerts: FleetAlert[]
addFleetAlert: (alert: FleetAlert) => void
dismissFleetAlert: (alertId: string) => void
clearDismissedAlerts: () => void
```

### Bước 3: Implement trong createSshSlice

```typescript
serverHealthMetrics: {},
lastFleetHealthCheck: null,
fleetAlerts: [],

updateServerHealth: (serverId, metrics) =>
  set((s) => {
    const existing = s.serverHealthMetrics[serverId] ?? {
      serverId,
      lastCheckedAt: 0,
      isReachable: false,
      uptimeSeconds: null,
      relayVersion: null,
      nodeVersion: null,
      diskUsagePercent: null,
      cpuUsagePercent: null,
      memUsagePercent: null,
    }
    s.serverHealthMetrics[serverId] = { ...existing, ...metrics }
  }),

setLastFleetHealthCheck: (ts) =>
  set((s) => { s.lastFleetHealthCheck = ts }),

addFleetAlert: (alert) =>
  set((s) => { s.fleetAlerts.push(alert) }),

dismissFleetAlert: (alertId) =>
  set((s) => {
    const alert = s.fleetAlerts.find((a) => a.id === alertId)
    if (alert) alert.dismissed = true
  }),

clearDismissedAlerts: () =>
  set((s) => {
    s.fleetAlerts = s.fleetAlerts.filter((a) => !a.dismissed)
  }),
```

### Bước 4: Thêm API vào preload

```typescript
// Thêm vào window.api.ssh:
getFleetHealth: () => ipcRenderer.invoke('ssh:getFleetHealth'),
refreshFleetHealth: () => ipcRenderer.invoke('ssh:refreshFleetHealth'),
```

### Bước 5: Verify

```bash
npx tsc --noEmit 2>&1 | grep "health\|Health\|FleetAlert" | head -10
```

---

## Acceptance Criteria

- [x] `serverHealthMetrics: Record<string, ServerHealthMetrics>` trong SshSlice
- [x] `fleetAlerts: FleetAlert[]` trong SshSlice
- [x] `updateServerHealth()` merge partial metrics
- [x] `addFleetAlert()` thêm alert vào array
- [x] `dismissFleetAlert(id)` set dismissed = true (không xóa)
- [x] TypeScript compile clean

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/ssh.ts`: serverHealthMetrics: Record<string,ServerHealthMetrics>, fleetAlerts: FleetAlert[], updateServerHealth() merge partial, addFleetAlert(), dismissFleetAlert() soft-dismiss (dismissed=true), clearDismissedAlerts(). TypeScript: ✅ 0 errors.
