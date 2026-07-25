# TDD-FE-10: Fleet Management UI

**Document:** TDD-FE-10 (NEW — remote-server CRs)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Fleet Inventory, Server Grouping, Bulk Provisioning, Bootstrap Automation, Health Monitoring, RBAC  
**Source files:**
- `src/renderer/src/components/fleet/`
- `src/renderer/src/hooks/useFleetHealthPolling.ts`
- `src/renderer/src/store/slices/ssh.ts` (extended)

> **Status: ✅ IMPLEMENTED** — 6/6 CRs (Phase 1+2 done, Phase 3 OIDC deferred)

---

## 1. Mục tiêu

Fleet Management UI cho phép quản lý **nhiều remote dev servers** như một fleet:

```
TRƯỚC:
  Settings → SSH Targets (list)
  Thêm từng server thủ công

SAU (remote-server CRs):
  Settings → SSH & Remotes → Fleet Config
    ├── Import từ orca-fleet.yaml
    ├── Group theo project/team
    ├── Bulk provisioning
    ├── Bootstrap automation
    ├── Health monitoring dashboard
    └── RBAC (Phase 2)
```

---

## 2. Zustand Store Extensions (SshSlice)

```typescript
// src/renderer/src/store/slices/ssh.ts — EXTENDED

// CR-001: Fleet Import
type FleetImportStatus = {
  phase: 'parsing' | 'importing' | 'done' | 'error'
  totalServers: number
  importedServers: number
  skippedServers: number
  failedServers: number
  errors: string[]
  configFilePath: string
}

// CR-002: Server Groups
type SshTargetGroup = {
  name: string
  targets: SshTarget[]
  tags: string[]
}

// CR-005: Health Monitoring
type ServerHealthMetrics = {
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

type FleetAlert = {
  id: string
  serverId: string
  serverLabel: string
  type: 'disconnected' | 'error' | 'relay-outdated'
  message: string
  timestamp: number
  dismissed: boolean
}

// Extended SshSlice type:
type SshSlice = {
  // --- existing ---
  sshConnectionStates: Record<string, SshConnectionState>
  setSshConnectionState: (targetId: string, state: SshConnectionState) => void

  // --- CR-001: Fleet Import ---
  fleetImportStatus: FleetImportStatus | null
  setFleetImportStatus: (status: FleetImportStatus | null) => void
  clearFleetImportStatus: () => void

  // --- CR-002: Server Grouping ---
  sshTargetGroups: SshTargetGroup[]
  setSshTargetGroups: (groups: SshTargetGroup[]) => void
  activeGroupFilter: string | null
  setActiveGroupFilter: (name: string | null) => void

  // --- CR-003: Bulk Provisioning ---
  bulkProvisioningProgress: BulkProvisioningProgress | null
  setBulkProvisioningProgress: (progress: BulkProvisioningProgress | null) => void

  // --- CR-004: Bootstrap ---
  bootstrapStatusByServer: Record<string, BootstrapStatus>
  setBootstrapStatus: (serverId: string, status: BootstrapStatus) => void

  // --- CR-005: Health ---
  serverHealthMetrics: Record<string, ServerHealthMetrics>
  updateServerHealth: (serverId: string, metrics: Partial<ServerHealthMetrics>) => void
  lastFleetHealthCheck: number | null
  setLastFleetHealthCheck: (ts: number) => void
  fleetAlerts: FleetAlert[]
  addFleetAlert: (alert: FleetAlert) => void
  dismissFleetAlert: (alertId: string) => void

  // --- CR-006: RBAC ---
  currentUser: OrcaUser | null
  setCurrentUser: (user: OrcaUser | null) => void
  accessPolicy: OrcaAccessPolicy | null
  setAccessPolicy: (policy: OrcaAccessPolicy) => void
}
```

---

## 3. New Hooks

### CR-001: Fleet Import

```typescript
// hooks/useFleetImport.ts
export function useFleetImport(): {
  status: FleetImportStatus | null
  importFromFile: (filePath?: string) => Promise<void>  // open file picker
  importFromPath: (path: string) => Promise<void>
  clearStatus: () => void
}
// IPC: window.api.ssh.fleet.import({ filePath })
// IPC events: window.api.ssh.fleet.onImportProgress(cb)
```

### CR-002: Server Grouping

```typescript
// hooks/useServerGroups.ts
export function useServerGroups(): {
  groups: SshTargetGroup[]
  activeFilter: string | null
  setFilter: (group: string | null) => void
  filteredTargets: SshTarget[]
}
// IPC: window.api.ssh.fleet.listByGroup()
```

### CR-003: Bulk Provisioning

```typescript
// hooks/useBulkProvisioning.ts
export function useBulkProvisioning(): {
  progress: BulkProvisioningProgress | null
  provisionServers: (serverIds: string[], config: ProvisionConfig) => Promise<void>
  cancelProvisioning: () => void
}
// IPC: window.api.ssh.fleet.bulkProvision({ serverIds, config })
// IPC events: window.api.ssh.fleet.onProvisionProgress(cb)
```

### CR-004: Bootstrap Automation

```typescript
// hooks/useBootstrapAutomation.ts
export function useBootstrapAutomation(serverId: string): {
  status: BootstrapStatus | null
  startBootstrap: () => Promise<void>
  currentStep: number
  totalSteps: 7
  stepLabel: string
}
// IPC: window.api.ssh.fleet.bootstrap({ serverId })
// IPC events: window.api.ssh.fleet.onBootstrapProgress(cb)
```

### CR-005: Fleet Health Monitoring

```typescript
// hooks/useFleetHealthPolling.ts
export function useFleetHealthPolling(options?: {
  intervalMs?: number  // default: 30_000
  autoStart?: boolean  // default: true in web mode
}): {
  isPolling: boolean
  lastCheckedAt: number | null
  start: () => void
  stop: () => void
  checkNow: () => Promise<void>
}
// IPC: window.api.ssh.fleet.status()
// Polling interval: 30 seconds default
```

### CR-006: RBAC

```typescript
// hooks/useCurrentUser.ts
export function useCurrentUser(): OrcaUser | null

// hooks/useAccessPolicy.ts
export function useAccessPolicy(): {
  policy: OrcaAccessPolicy | null
  canAccess: (serverId: string) => boolean
  isAdmin: boolean
}
```

---

## 4. New Components

### CR-001: Fleet Inventory

```
src/renderer/src/components/fleet/
├── FleetImportDialog.tsx         ← Dialog import YAML + progress
├── FleetImportProgress.tsx       ← Progress bar + error list
├── FleetImportSummary.tsx        ← Summary table: imported/skipped/failed
└── fleet-import-dialog.css
```

**Settings integration:**
```
Settings → SSH & Remotes (tab)
  ├── [Existing] SSH Targets list
  └── [NEW] Fleet Config section
        ├── "Import Fleet Config..." button → FleetImportDialog
        ├── Last import: "Imported 5/7 servers from ~/orca-fleet.yaml"
        └── "Clear Fleet Config" button
```

### CR-002: Server Grouping

```
├── GroupFilterPanel.tsx          ← Sidebar panel: filter by group
├── GroupBadge.tsx                ← Tag-style badge per group
├── ServerGroupList.tsx           ← List grouped by name
└── TagFilterBar.tsx              ← Multi-select tag filter
```

### CR-003: Bulk Provisioning

```
├── BulkProvisioningWizard.tsx    ← Multi-step: select → config → progress → results
├── BulkProvisioningProgress.tsx  ← Per-server progress bars
├── ProvisionConfigStep.tsx       ← Config: relay version, node version
└── ProvisionResultsTable.tsx     ← Success/fail per server
```

### CR-004: Bootstrap Automation

```
├── BootstrapStatusPanel.tsx      ← Inline in SSH target details
├── BootstrapStepList.tsx         ← 7 steps với status icons
└── BootstrapStepItem.tsx         ← Single step: pending/running/done/error
```

**Bootstrap step labels:**
```
1. SSH connectivity test
2. Remote platform detection
3. Node.js version check / install
4. Git version check / install
5. Relay binary deploy
6. Relay process start
7. Agent detection + handshake
```

### CR-005: Fleet Health Dashboard

```
├── FleetHealthDashboard.tsx      ← Overview panel: all servers status
├── FleetHealthTable.tsx          ← Table: server | status | latency | uptime | disk/cpu/mem
├── FleetServerHealthCard.tsx     ← Card: per-server metrics
├── FleetServerStatusBadge.tsx    ← Badge: reachable/unreachable/checking
├── FleetAlertStrip.tsx           ← Fixed-position alert banner for disconnects
└── FleetAlertItem.tsx            ← Single alert row
```

**Dashboard layout:**
```
Fleet Health Dashboard
  ┌─────────────────────────────────────────────────────┐
  │ [Refresh] [Check All]          Last checked: 30s ago │
  ├─────────────────────────────────────────────────────┤
  │ dev1.internal  ● healthy  48ms  up 7d  CPU 12%  💾 45% │
  │ dev2.internal  ● healthy  52ms  up 3d  CPU 8%   💾 67% │
  │ dev3.win32     ⚠ degraded 280ms up 1d  CPU 45%  💾 80% │
  │ dev4.offline   ✕ error    —     —      —               │
  └─────────────────────────────────────────────────────┘
```

### CR-006: RBAC UI (Phase 1+2)

```
├── UserProfileBadge.tsx          ← Display current user + role
├── AccessDeniedOverlay.tsx       ← Overlay khi không có permission
└── TeamMemberList.tsx            ← Admin: list users + roles
```

---

## 5. IPC API Surface (window.api.ssh.fleet extensions)

```typescript
window.api.ssh.fleet = {
  // CR-001
  import: (opts?: { filePath?: string }) => invoke('ssh:fleet:import', opts),
  onImportProgress: (cb) => on('ssh:fleet:importProgress', cb),
  offImportProgress: (cb) => off('ssh:fleet:importProgress', cb),

  // CR-002
  listByGroup: () => invoke('ssh:fleet:listByGroup'),

  // CR-003
  bulkProvision: (opts) => invoke('ssh:fleet:bulkProvision', opts),
  onProvisionProgress: (cb) => on('ssh:fleet:provisionProgress', cb),
  offProvisionProgress: (cb) => off('ssh:fleet:provisionProgress', cb),
  cancelProvisioning: () => invoke('ssh:fleet:cancelProvision'),

  // CR-004
  bootstrap: (opts) => invoke('ssh:fleet:bootstrap', opts),
  onBootstrapProgress: (cb) => on('ssh:fleet:bootstrapProgress', cb),
  offBootstrapProgress: (cb) => off('ssh:fleet:bootstrapProgress', cb),

  // CR-005
  status: () => invoke('ssh:fleet:status'),
  getHealthHistory: (serverId) => invoke('ssh:fleet:getHealthHistory', serverId),
  onHealthUpdate: (cb) => on('ssh:fleet:healthUpdate', cb),
  offHealthUpdate: (cb) => off('ssh:fleet:healthUpdate', cb),
}
```

---

## 6. Settings Panel Integration

```
Settings (sidebar):
  ├── General
  ├── Terminal
  ├── Editors
  ├── SSH & Remotes               ← TAB: SSH Targets
  │     ├── SSH Targets list (existing)
  │     ├── [NEW] Fleet Config section (CR-001)
  │     │     ├── FleetImportDialog
  │     │     └── Last import summary
  │     └── [NEW] Fleet Health tab (CR-005)
  │           └── FleetHealthDashboard
  ├── AI Agents
  └── [NEW] Team & Access (CR-006, web mode only)
        ├── UserProfileBadge
        └── TeamMemberList (admin only)
```

---

## 7. Design Patterns

### Polling pattern (CR-005)

```typescript
// useFleetHealthPolling.ts
useEffect(() => {
  if (!autoStart) return
  const id = setInterval(async () => {
    const report = await window.api.ssh.fleet.status()
    report.servers.forEach(s => {
      updateServerHealth(s.id, mapToMetrics(s))
    })
    setLastFleetHealthCheck(Date.now())
  }, intervalMs)
  return () => clearInterval(id)
}, [autoStart, intervalMs])
```

### Progress tracking pattern (CR-003, CR-004)

```typescript
// Sử dụng IPC event subscription:
useEffect(() => {
  window.api.ssh.fleet.onBootstrapProgress(handleProgress)
  return () => { window.api.ssh.fleet.offBootstrapProgress(handleProgress) }
}, [])
```

### Permission guard pattern (CR-006)

```tsx
// AccessDeniedOverlay — wrap any sensitive component:
<AccessGuard serverId={target.id} requiredRole="developer">
  <ServerDetailsPanel target={target} />
</AccessGuard>
```

---

## 8. Implementation Status

| CR | Solution | Components | Hooks | Tests |
|----|----------|-----------|-------|-------|
| CR-001 | SOL-CR-001 | FleetImportDialog + 3 | useFleetImport | n/a |
| CR-002 | SOL-CR-002 | GroupFilterPanel + 3 | useServerGroups | n/a |
| CR-003 | SOL-CR-003 | BulkProvisioningWizard + 3 | useBulkProvisioning | n/a |
| CR-004 | SOL-CR-004 | BootstrapStatusPanel + 2 | useBootstrapAutomation | n/a |
| CR-005 | SOL-CR-005 | FleetHealthDashboard + 5 | useFleetHealthPolling | n/a |
| CR-006 | SOL-CR-006 | UserProfileBadge + 2 | useCurrentUser, useAccessPolicy | n/a |
