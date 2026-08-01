# SOL-FE-FLEET-001B: SSH Slice Extension, useFleetHealthPolling, FleetImportDialog

## Bug Reference
- **Bug:** BUG-FE-FLEET-001 (Supplement — thiếu Zustand slice code và Fleet hooks)
- **Depends on:** SOL-FE-FLEET-001 (FleetDashboard, OnboardingWizard, FleetHealthTable)
- **TDD Reference:** TDD-FE-10 §2 (Zustand Store Extensions), §3 (Hooks), §4 (Components)

---

## Lý do bổ sung

SOL-FE-FLEET-001 đề cập "Thêm Fleet state theo TDD-FE-10 §2" nhưng không có code cụ thể. Cần bổ sung:
1. SSH Slice extension — code đầy đủ theo TDD-FE-10 §2
2. `useFleetHealthPolling` hook implementation
3. `FleetImportDialog` component (CR-001 YAML import)
4. `FleetAlertStrip` component (referenced trong FleetDashboard nhưng chưa có code)

---

### Phần 1: SSH Slice Extension

**File:** `src/renderer/src/store/slices/ssh.ts` (MODIFY — thêm Fleet extensions)

Đây là code đầy đủ theo **TDD-FE-10 §2**:

```typescript
// src/renderer/src/store/slices/ssh.ts — Fleet extensions
// Thêm vào existing SshSlice (không xóa existing state)

// === Types ===
export type FleetImportStatus = {
  phase: 'parsing' | 'importing' | 'done' | 'error'
  totalServers: number
  importedServers: number
  skippedServers: number
  failedServers: number
  errors: string[]
  configFilePath: string
}

export type SshTargetGroup = {
  name: string
  targets: SshTarget[]
  tags: string[]
}

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

export type FleetAlert = {
  id: string
  serverId: string
  serverLabel: string
  type: 'disconnected' | 'error' | 'relay-outdated'
  message: string
  timestamp: number
  dismissed: boolean
}

export type BootstrapStatus = {
  serverId: string
  currentStep: number  // 0-6 (7 steps total)
  totalSteps: 7
  status: 'running' | 'done' | 'error'
  errorMessage?: string
  stepLogs: string[]
}

export type BulkProvisioningProgress = {
  total: number
  completed: number
  failed: number
  inProgress: string[]  // server IDs currently provisioning
}

// === Slice Extension (thêm vào createSshSlice) ===
// Note: Merge với existing sshConnectionStates, setSshConnectionState, etc.

const fleetExtensions = (set: any, get: any) => ({
  // CR-001: Fleet Import
  fleetImportStatus: null as FleetImportStatus | null,
  setFleetImportStatus: (status: FleetImportStatus | null) =>
    set({ fleetImportStatus: status }),
  clearFleetImportStatus: () =>
    set({ fleetImportStatus: null }),

  // CR-002: Server Grouping
  sshTargetGroups: [] as SshTargetGroup[],
  setSshTargetGroups: (groups: SshTargetGroup[]) =>
    set({ sshTargetGroups: groups }),
  activeGroupFilter: null as string | null,
  setActiveGroupFilter: (name: string | null) =>
    set({ activeGroupFilter: name }),

  // CR-003: Bulk Provisioning
  bulkProvisioningProgress: null as BulkProvisioningProgress | null,
  setBulkProvisioningProgress: (progress: BulkProvisioningProgress | null) =>
    set({ bulkProvisioningProgress: progress }),

  // CR-004: Bootstrap Automation
  bootstrapStatusByServer: {} as Record<string, BootstrapStatus>,
  setBootstrapStatus: (serverId: string, status: BootstrapStatus) =>
    set((s: any) => ({
      bootstrapStatusByServer: { ...s.bootstrapStatusByServer, [serverId]: status }
    })),

  // CR-005: Fleet Health Monitoring
  serverHealthMetrics: {} as Record<string, ServerHealthMetrics>,
  updateServerHealth: (serverId: string, metrics: Partial<ServerHealthMetrics>) =>
    set((s: any) => ({
      serverHealthMetrics: {
        ...s.serverHealthMetrics,
        [serverId]: {
          ...(s.serverHealthMetrics[serverId] ?? { serverId }),
          ...metrics,
          lastCheckedAt: Date.now(),
        },
      },
    })),
  lastFleetHealthCheck: null as number | null,
  setLastFleetHealthCheck: (ts: number) => set({ lastFleetHealthCheck: ts }),
  fleetAlerts: [] as FleetAlert[],
  addFleetAlert: (alert: FleetAlert) =>
    set((s: any) => ({ fleetAlerts: [alert, ...s.fleetAlerts] })),
  dismissFleetAlert: (alertId: string) =>
    set((s: any) => ({
      fleetAlerts: s.fleetAlerts.map((a: FleetAlert) =>
        a.id === alertId ? { ...a, dismissed: true } : a
      ),
    })),

  // CR-006: RBAC
  currentUser: null as OrcaUser | null,
  setCurrentUser: (user: OrcaUser | null) => set({ currentUser: user }),
  accessPolicy: null as OrcaAccessPolicy | null,
  setAccessPolicy: (policy: OrcaAccessPolicy | null) => set({ accessPolicy: policy }),
})
```

---

### Phần 2: `useFleetHealthPolling` Hook

**File:** `src/renderer/src/hooks/useFleetHealthPolling.ts` (TẠO MỚI)

Theo **TDD-FE-10 §3** — 30s polling với IPC events:

```typescript
// src/renderer/src/hooks/useFleetHealthPolling.ts
// Fleet health polling — 30s interval (TDD-FE-10 §3 CR-005)

import { useState, useEffect, useCallback, useRef } from 'react'
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { rpc } from '@/platform/rpc-client-interface'

const POLL_INTERVAL_MS = 30_000

export function useFleetHealthPolling(opts: {
  intervalMs?: number
  autoStart?: boolean
}) {
  const { intervalMs = POLL_INTERVAL_MS, autoStart = true } = opts

  const { updateServerHealth, setLastFleetHealthCheck, addFleetAlert, sshTargets } = useAppStore(
    useShallow(s => ({
      updateServerHealth: s.updateServerHealth,
      setLastFleetHealthCheck: s.setLastFleetHealthCheck,
      addFleetAlert: s.addFleetAlert,
      sshTargets: s.sshTargets as SshTarget[],  // existing SSH targets list
    }))
  )

  const [isPolling, setIsPolling] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const checkNow = useCallback(async () => {
    if (isPolling) return
    setIsPolling(true)
    try {
      // Batch health check for all known SSH targets
      const results = await rpc.call('fleet.health.checkAll', {
        serverIds: sshTargets.map(t => t.id),
      }) as ServerHealthMetrics[]

      for (const metrics of results) {
        updateServerHealth(metrics.serverId, metrics)

        // Auto-alert if server became unreachable
        if (!metrics.isReachable) {
          const target = sshTargets.find(t => t.id === metrics.serverId)
          addFleetAlert({
            id: `${metrics.serverId}-${Date.now()}`,
            serverId: metrics.serverId,
            serverLabel: target?.label ?? metrics.serverId,
            type: 'disconnected',
            message: `Server ${target?.label ?? metrics.serverId} is unreachable`,
            timestamp: Date.now(),
            dismissed: false,
          })
        }

        // Alert if relay version is outdated
        if (metrics.relayVersion && isRelayOutdated(metrics.relayVersion)) {
          addFleetAlert({
            id: `${metrics.serverId}-relay-${Date.now()}`,
            serverId: metrics.serverId,
            serverLabel: target?.label ?? metrics.serverId,
            type: 'relay-outdated',
            message: `Relay v${metrics.relayVersion} is outdated. Please update.`,
            timestamp: Date.now(),
            dismissed: false,
          })
        }
      }

      setLastFleetHealthCheck(Date.now())
    } catch (err) {
      console.error('Fleet health check failed:', err)
    } finally {
      setIsPolling(false)
    }
  }, [sshTargets, updateServerHealth, setLastFleetHealthCheck, addFleetAlert, isPolling])

  // Auto-polling
  useEffect(() => {
    if (!autoStart) return
    checkNow()  // Initial check
    intervalRef.current = setInterval(checkNow, intervalMs)
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [autoStart, intervalMs])

  return { isPolling, checkNow }
}

function isRelayOutdated(version: string): boolean {
  // Compare with minimum required relay version
  const MIN_RELAY_VERSION = '1.5.0'
  return compareVersions(version, MIN_RELAY_VERSION) < 0
}

function compareVersions(a: string, b: string): number {
  const aParts = a.split('.').map(Number)
  const bParts = b.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    const diff = (aParts[i] ?? 0) - (bParts[i] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}
```

---

### Phần 3: `FleetImportDialog` Component

**File:** `src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx` (TẠO MỚI)

Theo **TDD-FE-10 §4 CR-001** — Import từ orca-fleet.yaml:

```typescript
// BL-FLEET-01: Import Fleet Config (CR-001)
// YAML file picker + import progress

import { useState, useRef } from 'react'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Upload, FileText, CheckCircle, XCircle } from 'lucide-react'
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { rpc } from '@/platform/rpc-client-interface'
import { toast } from 'sonner'

interface FleetImportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function FleetImportDialog({ open, onOpenChange }: FleetImportDialogProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [yamlContent, setYamlContent] = useState<string | null>(null)
  const [isImporting, setIsImporting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { fleetImportStatus, setFleetImportStatus, clearFleetImportStatus } = useAppStore(
    useShallow(s => ({
      fleetImportStatus: s.fleetImportStatus,
      setFleetImportStatus: s.setFleetImportStatus,
      clearFleetImportStatus: s.clearFleetImportStatus,
    }))
  )

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setSelectedFile(file)
    const content = await file.text()
    setYamlContent(content)
  }

  const startImport = async () => {
    if (!yamlContent) return
    setIsImporting(true)
    setFleetImportStatus({
      phase: 'parsing',
      totalServers: 0,
      importedServers: 0,
      skippedServers: 0,
      failedServers: 0,
      errors: [],
      configFilePath: selectedFile?.name ?? 'orca-fleet.yaml',
    })

    try {
      // Phân tích YAML trên backend
      const result = await rpc.call('fleet.import', {
        yamlContent,
        configFilePath: selectedFile?.name ?? 'orca-fleet.yaml',
      }) as FleetImportResult

      setFleetImportStatus({
        phase: 'done',
        totalServers: result.total,
        importedServers: result.imported,
        skippedServers: result.skipped,
        failedServers: result.failed,
        errors: result.errors,
        configFilePath: selectedFile?.name ?? 'orca-fleet.yaml',
      })
      toast.success(`Fleet imported: ${result.imported} servers added`)
    } catch (err: any) {
      setFleetImportStatus(prev => prev ? {
        ...prev,
        phase: 'error',
        errors: [err.message],
      } : null)
      toast.error('Fleet import failed')
    } finally {
      setIsImporting(false)
    }
  }

  const reset = () => {
    setSelectedFile(null)
    setYamlContent(null)
    clearFleetImportStatus()
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const isDone = fleetImportStatus?.phase === 'done'
  const isError = fleetImportStatus?.phase === 'error'
  const progress = fleetImportStatus
    ? Math.floor((fleetImportStatus.importedServers / Math.max(fleetImportStatus.totalServers, 1)) * 100)
    : 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Import Fleet Configuration</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* File picker */}
          {!fleetImportStatus && (
            <div
              className="border-2 border-dashed rounded-lg p-8 text-center cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => fileInputRef.current?.click()}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".yaml,.yml"
                className="hidden"
                onChange={handleFileSelect}
              />
              {selectedFile ? (
                <div className="flex items-center justify-center gap-2">
                  <FileText size={20} className="text-primary" />
                  <span className="text-sm font-medium">{selectedFile.name}</span>
                </div>
              ) : (
                <>
                  <Upload size={32} className="mx-auto mb-2 text-muted-foreground" />
                  <p className="text-sm text-muted-foreground">
                    Click to select <code className="bg-muted px-1 rounded">orca-fleet.yaml</code>
                  </p>
                </>
              )}
            </div>
          )}

          {/* Import progress */}
          {fleetImportStatus && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                {isDone ? (
                  <CheckCircle size={16} className="text-green-500" />
                ) : isError ? (
                  <XCircle size={16} className="text-destructive" />
                ) : (
                  <div className="h-4 w-4 rounded-full border-2 border-primary border-t-transparent animate-spin" />
                )}
                <span className="text-sm font-medium capitalize">
                  {fleetImportStatus.phase === 'parsing' ? 'Parsing YAML...'
                    : fleetImportStatus.phase === 'importing' ? 'Importing servers...'
                    : fleetImportStatus.phase === 'done' ? 'Import complete'
                    : 'Import failed'}
                </span>
              </div>

              {fleetImportStatus.totalServers > 0 && (
                <>
                  <Progress value={progress} className="h-2" />
                  <div className="grid grid-cols-3 gap-2 text-xs text-center">
                    <div>
                      <p className="font-bold text-green-500">{fleetImportStatus.importedServers}</p>
                      <p className="text-muted-foreground">Added</p>
                    </div>
                    <div>
                      <p className="font-bold text-yellow-500">{fleetImportStatus.skippedServers}</p>
                      <p className="text-muted-foreground">Skipped</p>
                    </div>
                    <div>
                      <p className="font-bold text-destructive">{fleetImportStatus.failedServers}</p>
                      <p className="text-muted-foreground">Failed</p>
                    </div>
                  </div>
                </>
              )}

              {fleetImportStatus.errors.length > 0 && (
                <div className="text-xs text-destructive space-y-1 max-h-20 overflow-y-auto">
                  {fleetImportStatus.errors.map((err, i) => (
                    <p key={i}>• {err}</p>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          {fleetImportStatus ? (
            <Button onClick={() => { reset(); onOpenChange(false) }}>
              {isDone ? 'Done' : 'Close'}
            </Button>
          ) : (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
              <Button
                onClick={startImport}
                disabled={!yamlContent || isImporting}
              >
                Import
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

type FleetImportResult = {
  total: number
  imported: number
  skipped: number
  failed: number
  errors: string[]
}
```

---

### Phần 4: `FleetAlertStrip` Component

**File:** `src/renderer/src/components/admin/fleet/fleet-alert-strip.tsx` (TẠO MỚI)

Referenced trong `FleetDashboard` của SOL-FE-FLEET-001:

```typescript
// Fleet alert strip — hiển thị unreachable/outdated alerts
import { AlertTriangle, X } from 'lucide-react'
import { useAppStore } from '@/store'
import { FleetAlert } from '@/store/slices/ssh'
import { Button } from '@/components/ui/button'

interface FleetAlertStripProps {
  alerts: FleetAlert[]
}

export function FleetAlertStrip({ alerts }: FleetAlertStripProps) {
  const dismissFleetAlert = useAppStore(s => s.dismissFleetAlert)

  if (alerts.length === 0) return null

  return (
    <div className="fleet-alert-strip space-y-1">
      {alerts.slice(0, 3).map(alert => (
        <div
          key={alert.id}
          className="flex items-center gap-2 px-3 py-2 bg-yellow-500/10 border border-yellow-500/30 rounded text-sm"
        >
          <AlertTriangle size={14} className="text-yellow-500 shrink-0" />
          <span className="flex-1 text-yellow-700 dark:text-yellow-400">
            <strong>{alert.serverLabel}</strong>: {alert.message}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-5 w-5 shrink-0"
            onClick={() => dismissFleetAlert(alert.id)}
          >
            <X size={10} />
          </Button>
        </div>
      ))}
      {alerts.length > 3 && (
        <p className="text-xs text-muted-foreground px-3">
          +{alerts.length - 3} more alerts
        </p>
      )}
    </div>
  )
}
```

---

## Files cần tạo (BỔ SUNG vào SOL-FE-FLEET-001)

| File | Action | CR |
|------|--------|-----|
| `src/renderer/src/store/slices/ssh.ts` | MODIFY — Fleet extensions | CR-001~006 |
| `src/renderer/src/hooks/useFleetHealthPolling.ts` | CREATE | CR-005 |
| `src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx` | CREATE | CR-001 |
| `src/renderer/src/components/admin/fleet/fleet-alert-strip.tsx` | CREATE | CR-005 |
| `src/renderer/src/hooks/useBootstrapAutomation.ts` | CREATE | CR-004 |

---

## useBootstrapAutomation hook (CR-004)

**File:** `src/renderer/src/hooks/useBootstrapAutomation.ts` (TẠO MỚI)

```typescript
// src/renderer/src/hooks/useBootstrapAutomation.ts
// 7-step bootstrap tracking (TDD-FE-10 §3 CR-004)

export function useBootstrapAutomation(serverId: string) {
  const { bootstrapStatusByServer, setBootstrapStatus } = useAppStore(
    useShallow(s => ({
      bootstrapStatusByServer: s.bootstrapStatusByServer,
      setBootstrapStatus: s.setBootstrapStatus,
    }))
  )

  const status = bootstrapStatusByServer[serverId]

  const startBootstrap = useCallback(async () => {
    if (!serverId) return

    // Initialize
    setBootstrapStatus(serverId, {
      serverId,
      currentStep: 0,
      totalSteps: 7,
      status: 'running',
      stepLogs: [],
    })

    // Subscribe to bootstrap progress events from server
    // Backend emits 'fleet.bootstrap.progress' events via IPC/SSE
    try {
      await rpc.call('fleet.bootstrap.start', { serverId })
      // Events will update store via useIpcEvents subscription
    } catch (err: any) {
      setBootstrapStatus(serverId, {
        serverId,
        currentStep: status?.currentStep ?? 0,
        totalSteps: 7,
        status: 'error',
        errorMessage: err.message,
        stepLogs: status?.stepLogs ?? [],
      })
    }
  }, [serverId, setBootstrapStatus])

  return {
    status,
    currentStep: status?.currentStep ?? 0,
    totalSteps: 7,
    startBootstrap,
  }
}
```

---

## Liên quan

- **BL-FLEET-01**: Fleet import UI ✅ implemented (`FleetImportDialog`)
- **BL-FLEET-03**: Real-time health + alerts ✅ implemented (`useFleetHealthPolling`, `FleetAlertStrip`)
- **BL-FLEET-04**: Onboarding Wizard bootstrap ✅ implemented (`useBootstrapAutomation`)
- **TDD-FE-10**: §2 SshSlice (ALL 6 CRs), §3 Hooks, §4 Components
