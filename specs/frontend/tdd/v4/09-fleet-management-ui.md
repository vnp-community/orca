# TDD-FE-09: Fleet Management UI

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/components/fleet/`, `src/renderer/src/hooks/`

---

## 1. Fleet Components

```
components/fleet/
├─ FleetImportDialog.tsx       ← Import fleet từ YAML config
├─ FleetHealthDashboard.tsx    ← Health metrics tất cả servers
├─ BulkProvisioningWizard.tsx  ← Provision nhiều servers cùng lúc
├─ BootstrapStatusPanel.tsx    ← 7-step bootstrap progress
├─ UserProfileBadge.tsx        ← User info + role indicator
└─ ServerGroupFilter.tsx       ← Filter theo group/tag
```

---

## 2. FleetHealthDashboard

```tsx
function FleetHealthDashboard() {
  const { metrics, loading } = useFleetHealthPolling()

  return (
    <div className="fleet-dashboard">
      <SummaryBar
        total={metrics.total}
        healthy={metrics.healthy}
        degraded={metrics.degraded}
        offline={metrics.offline}
      />

      <ServerGrid>
        {metrics.servers.map(server => (
          <ServerHealthCard
            key={server.id}
            server={server}
            metrics={server.metrics}
          />
        ))}
      </ServerGrid>

      <FleetAlertsPanel alerts={metrics.alerts} />
    </div>
  )
}
```

---

## 3. FleetImportDialog

```tsx
function FleetImportDialog({ onClose }) {
  // Input: YAML text area (fleet config format)
  // Parse: Zod schema validation
  // Preview: list of servers to import
  // Confirm → window.api.importFleetConfig(yaml)
  // Progress: show import status per server
}
```

---

## 4. BootstrapStatusPanel

```tsx
function BootstrapStatusPanel({ serverId }) {
  const { steps, status } = useBootstrapAutomation(serverId)

  // 7-step bootstrap:
  // 1. SSH connect
  // 2. Detect platform/arch
  // 3. Upload relay binary
  // 4. Set permissions
  // 5. Install systemd service
  // 6. Start service
  // 7. Health check

  return (
    <StepList steps={steps} currentStep={status.currentStep} />
  )
}
```

---

## 5. Fleet Hooks

### useFleetHealthPolling

```typescript
function useFleetHealthPolling(interval = 30000): {
  metrics:  FleetHealthMetrics
  loading:  boolean
  error:    string | null
  refresh:  () => void
}

// Polls: window.api.getFleetHealth() mỗi 30 giây
// Also subscribes to IPC: 'fleet:statusChanged'
```

### useFleetImport

```typescript
function useFleetImport(): {
  importing:  boolean
  progress:   FleetImportProgress | null
  importYaml: (yaml: string) => Promise<FleetImportResult>
}
```

### useBootstrapAutomation

```typescript
function useBootstrapAutomation(serverId: string): {
  steps:         BootstrapStep[]
  status:        BootstrapStatus
  startBootstrap: () => Promise<void>
  cancelBootstrap: () => void
}
```

### useServerGroups

```typescript
function useServerGroups(): {
  groups:        ServerGroup[]
  filter:        string | null
  setFilter:     (groupId: string | null) => void
  filteredServers: DevServer[]
}
```

### useBulkProvisioning

```typescript
function useBulkProvisioning(): {
  provisioning:   boolean
  results:        Map<string, ProvisionResult>
  provisionAll:   (serverIds: string[]) => Promise<void>
  provisionBatch: (serverIds: string[], concurrency?: number) => Promise<void>
}
// Default concurrency: 3 servers at a time
```

---

## 6. Store Extensions (SSH Slice)

```typescript
// Thêm vào store/slices/ssh.ts:

// Fleet health
fleetHealth:         FleetHealthMetrics | null
serverHealthMetrics: Map<string, ServerHealthMetrics>
fleetAlerts:         FleetAlert[]

// Bootstrap
bootstrapStatus:     Map<string, BootstrapStatus>   // key: serverId

// RBAC
userRoles:           Map<string, UserRole>           // key: userId

// Groups
serverGroups:        ServerGroup[]
fleetImportStatus:   FleetImportStatus | null
```

---

## 7. RBAC UI

```tsx
// UserProfileBadge — hiển thị user role in fleet context
function UserProfileBadge({ userId }) {
  const { role } = useUserRole(userId)
  return <RolePill role={role} />  // admin | user | viewer
}
```

**Role capabilities in fleet UI:**
| Role | View health | Bootstrap | Import YAML | Bulk provision |
|------|------------|-----------|-------------|----------------|
| admin | ✅ | ✅ | ✅ | ✅ |
| user | ✅ | ❌ | ❌ | ❌ |
| viewer | ✅ | ❌ | ❌ | ❌ |
