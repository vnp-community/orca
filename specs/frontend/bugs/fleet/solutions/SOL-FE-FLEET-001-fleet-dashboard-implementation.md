# SOL-FE-FLEET-001: Implement Fleet Dashboard (Admin SPA)

## Bug Reference
- **Bug:** BUG-FE-FLEET-001
- **Mức độ:** 🔴 HIGH (Feature Missing)
- **TDD Reference:** TDD-FE-10 (Fleet Management UI — hoàn chỉnh 6 CRs)

---

## Root Cause

Fleet Dashboard (Admin SPA) chưa được implement. TDD-FE-10 đã định nghĩa đầy đủ:
- CR-001: Fleet Import từ YAML
- CR-002: Server Grouping
- CR-003: Bulk Provisioning
- CR-004: Bootstrap Automation
- CR-005: Fleet Health Dashboard
- CR-006: RBAC

---

## Giải pháp

Implement theo TDD-FE-10 — đây là bộ components lớn. Solution tập trung vào 3 components cốt lõi nhất (BL-FLEET-01, BL-FLEET-03, BL-FLEET-04).

---

### Component 1: `fleet-dashboard.tsx`

**File:** `src/renderer/src/components/admin/fleet/fleet-dashboard.tsx` (TẠO MỚI)

```typescript
// BL-FLEET-03: Fleet Health Dashboard
// Theo TDD-FE-10 §4 CR-005

import { useFleetHealthPolling } from '@/hooks/useFleetHealthPolling'
import { useAppStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { FleetHealthTable } from './fleet-health-table'
import { FleetAlertStrip } from './fleet-alert-strip'
import { FleetImportDialog } from './fleet-import-dialog'
import { Button } from '@/components/ui/button'
import { RefreshCw, Plus } from 'lucide-react'
import { useState } from 'react'
import { formatDistanceToNow } from 'date-fns'

export function FleetDashboard() {
  const { serverHealthMetrics, lastFleetHealthCheck, fleetAlerts } = useAppStore(
    useShallow(s => ({
      serverHealthMetrics: s.serverHealthMetrics,
      lastFleetHealthCheck: s.lastFleetHealthCheck,
      fleetAlerts: s.fleetAlerts,
    }))
  )

  const { isPolling, checkNow } = useFleetHealthPolling({
    intervalMs: 30_000,
    autoStart: true,  // web mode only, theo TDD-FE-07 §Hook Rules
  })

  const [showImportDialog, setShowImportDialog] = useState(false)

  const activeAlerts = fleetAlerts.filter(a => !a.dismissed)

  return (
    <div className="fleet-dashboard flex flex-col gap-4">
      {/* Alert banner */}
      {activeAlerts.length > 0 && <FleetAlertStrip alerts={activeAlerts} />}

      {/* Header toolbar */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Fleet Health</h2>
          {lastFleetHealthCheck && (
            <p className="text-xs text-muted-foreground">
              Last checked: {formatDistanceToNow(lastFleetHealthCheck, { addSuffix: true })}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={checkNow}
            disabled={isPolling}
            className="gap-2"
          >
            <RefreshCw size={14} className={isPolling ? 'animate-spin' : ''} />
            {isPolling ? 'Checking...' : 'Check All'}
          </Button>
          <Button size="sm" className="gap-2" onClick={() => setShowImportDialog(true)}>
            <Plus size={14} />
            Import Fleet Config
          </Button>
        </div>
      </div>

      {/* Server inventory table */}
      <FleetHealthTable metrics={serverHealthMetrics} />

      {/* Fleet import dialog */}
      <FleetImportDialog
        open={showImportDialog}
        onOpenChange={setShowImportDialog}
      />
    </div>
  )
}
```

---

### Component 2: `server-health-card.tsx`

**File:** `src/renderer/src/components/admin/fleet/server-health-card.tsx` (TẠO MỚI)

```typescript
// BL-FLEET-03: Per-server health metrics card
// Theo TDD-FE-10 §4 CR-005 FleetServerHealthCard

import { ServerHealthMetrics } from '@/store/slices/ssh'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Cpu, HardDrive, MemoryStick, Wifi, WifiOff } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ServerHealthCardProps {
  serverId: string
  metrics: ServerHealthMetrics
}

function HealthStatusBadge({ isReachable }: { isReachable: boolean }) {
  return (
    <Badge
      variant={isReachable ? 'default' : 'destructive'}
      className="gap-1"
    >
      {isReachable ? <Wifi size={10} /> : <WifiOff size={10} />}
      {isReachable ? 'healthy' : 'unreachable'}
    </Badge>
  )
}

function MetricBar({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: number | null
  icon: React.ElementType
}) {
  if (value == null) return null
  const color = value > 80 ? 'bg-destructive' : value > 60 ? 'bg-yellow-500' : 'bg-primary'

  return (
    <div className="flex items-center gap-2 text-xs">
      <Icon size={12} className="text-muted-foreground shrink-0" />
      <span className="w-6 text-muted-foreground">{label}</span>
      <Progress value={value} className="flex-1 h-1.5" indicatorClassName={color} />
      <span className="w-8 text-right font-mono">{value.toFixed(0)}%</span>
    </div>
  )
}

export function ServerHealthCard({ serverId, metrics }: ServerHealthCardProps) {
  const uptimeDays = metrics.uptimeSeconds
    ? Math.floor(metrics.uptimeSeconds / 86400)
    : null

  return (
    <div className={cn(
      'server-health-card border rounded p-3 space-y-2',
      !metrics.isReachable && 'border-destructive/50 bg-destructive/5'
    )}>
      <div className="flex items-center justify-between">
        <span className="font-medium text-sm font-mono">{serverId}</span>
        <HealthStatusBadge isReachable={metrics.isReachable} />
      </div>

      <div className="flex gap-3 text-xs text-muted-foreground">
        {uptimeDays != null && <span>up {uptimeDays}d</span>}
        {metrics.relayVersion && <span>relay v{metrics.relayVersion}</span>}
      </div>

      <div className="space-y-1">
        <MetricBar label="CPU" value={metrics.cpuUsagePercent ?? null} icon={Cpu} />
        <MetricBar label="RAM" value={metrics.memUsagePercent ?? null} icon={MemoryStick} />
        <MetricBar label="Disk" value={metrics.diskUsagePercent ?? null} icon={HardDrive} />
      </div>
    </div>
  )
}
```

---

### Component 3: `onboarding-wizard.tsx`

**File:** `src/renderer/src/components/admin/fleet/onboarding-wizard.tsx` (TẠO MỚI)

```typescript
// BL-FLEET-04: Onboarding Wizard (Add Server flow — 6 steps)
// Theo TDD-FE-10 §4 CR-004 BootstrapStatusPanel + TDD-FE-09

import { useState } from 'react'
import { useBootstrapAutomation } from '@/hooks/useBootstrapAutomation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { CheckCircle, Circle, Loader2, XCircle } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

// 7 bootstrap steps theo TDD-FE-10 §4 CR-004
const BOOTSTRAP_STEPS = [
  'SSH connectivity test',
  'Remote platform detection',
  'Node.js version check / install',
  'Git version check / install',
  'Relay binary deploy',
  'Relay process start',
  'Agent detection + handshake',
] as const

type StepStatus = 'pending' | 'running' | 'done' | 'error'

interface OnboardingWizardProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onComplete: (serverId: string) => void
}

export function OnboardingWizard({
  open,
  onOpenChange,
  onComplete,
}: OnboardingWizardProps) {
  const [step, setStep] = useState<'configure' | 'bootstrap'>('configure')
  const [sshHost, setSshHost] = useState('')
  const [sshPort, setSshPort] = useState('22')
  const [sshUser, setSshUser] = useState('')
  const [serverId, setServerId] = useState<string | null>(null)

  const bootstrap = useBootstrapAutomation(serverId ?? '')

  const startOnboarding = async () => {
    // Step 1: Add SSH target → get serverId
    try {
      const result = await window.api.ssh.addTarget({
        host: sshHost,
        port: parseInt(sshPort),
        user: sshUser,
      }) as { id: string }
      setServerId(result.id)
      setStep('bootstrap')
      // Begin bootstrap automation (TDD-FE-10 CR-004)
      await bootstrap.startBootstrap()
    } catch (err: any) {
      console.error('Onboarding failed:', err)
    }
  }

  const getStepIcon = (idx: number) => {
    if (idx < bootstrap.currentStep) return <CheckCircle size={16} className="text-green-500" />
    if (idx === bootstrap.currentStep) {
      if (bootstrap.status?.status === 'error') {
        return <XCircle size={16} className="text-destructive" />
      }
      return <Loader2 size={16} className="text-primary animate-spin" />
    }
    return <Circle size={16} className="text-muted-foreground" />
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Add Server to Fleet</DialogTitle>
        </DialogHeader>

        {step === 'configure' && (
          <div className="space-y-4">
            <div className="space-y-1">
              <Label className="text-xs">SSH Host</Label>
              <Input
                value={sshHost}
                onChange={e => setSshHost(e.target.value)}
                placeholder="dev1.internal or 192.168.1.10"
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Label className="text-xs">Port</Label>
                <Input
                  value={sshPort}
                  onChange={e => setSshPort(e.target.value)}
                  type="number"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">SSH User</Label>
                <Input
                  value={sshUser}
                  onChange={e => setSshUser(e.target.value)}
                  placeholder="ubuntu"
                />
              </div>
            </div>
            <Button
              className="w-full"
              onClick={startOnboarding}
              disabled={!sshHost || !sshUser}
            >
              Start Onboarding
            </Button>
          </div>
        )}

        {step === 'bootstrap' && (
          <div className="space-y-4">
            <Progress
              value={(bootstrap.currentStep / bootstrap.totalSteps) * 100}
              className="h-2"
            />
            <div className="space-y-2">
              {BOOTSTRAP_STEPS.map((label, idx) => (
                <div key={idx} className="flex items-center gap-2 text-sm">
                  {getStepIcon(idx)}
                  <span className={idx < bootstrap.currentStep ? 'text-muted-foreground line-through' : ''}>
                    {label}
                  </span>
                </div>
              ))}
            </div>
            {bootstrap.status?.status === 'done' && (
              <Button
                className="w-full"
                onClick={() => {
                  onComplete(serverId!)
                  onOpenChange(false)
                }}
              >
                Finish — Server Added
              </Button>
            )}
            {bootstrap.status?.status === 'error' && (
              <p className="text-xs text-destructive">{bootstrap.status.errorMessage}</p>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

---

### Component 4: `fleet-health-table.tsx`

**File:** `src/renderer/src/components/admin/fleet/fleet-health-table.tsx` (TẠO MỚI)

```typescript
// BL-FLEET-01: Fleet inventory table với color-coded health status
// Theo TDD-FE-10 §4 CR-005 FleetHealthTable

import { ServerHealthMetrics } from '@/store/slices/ssh'
import { cn } from '@/lib/utils'
import { Cpu, HardDrive, MemoryStick } from 'lucide-react'

interface FleetHealthTableProps {
  metrics: Record<string, ServerHealthMetrics>
}

function StatusDot({ isReachable, cpuPct }: { isReachable: boolean; cpuPct?: number | null }) {
  if (!isReachable) return <span className="w-2 h-2 rounded-full bg-destructive inline-block" title="unreachable" />
  if (cpuPct && cpuPct > 80) return <span className="w-2 h-2 rounded-full bg-yellow-500 inline-block" title="degraded" />
  return <span className="w-2 h-2 rounded-full bg-green-500 inline-block" title="healthy" />
}

export function FleetHealthTable({ metrics }: FleetHealthTableProps) {
  const servers = Object.entries(metrics)

  if (servers.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground text-sm">
        No servers in fleet. Click "Import Fleet Config" to add servers.
      </div>
    )
  }

  return (
    <div className="fleet-health-table overflow-auto rounded border">
      <table className="w-full text-sm">
        <thead className="bg-muted text-xs">
          <tr>
            <th className="text-left px-3 py-2">Server</th>
            <th className="text-left px-3 py-2">Status</th>
            <th className="text-left px-3 py-2">Uptime</th>
            <th className="text-left px-3 py-2 gap-1"><Cpu size={10} className="inline" /> CPU</th>
            <th className="text-left px-3 py-2 gap-1"><MemoryStick size={10} className="inline" /> RAM</th>
            <th className="text-left px-3 py-2 gap-1"><HardDrive size={10} className="inline" /> Disk</th>
          </tr>
        </thead>
        <tbody>
          {servers.map(([serverId, m]) => {
            const uptimeDays = m.uptimeSeconds ? Math.floor(m.uptimeSeconds / 86400) : null
            return (
              <tr
                key={serverId}
                className={cn(
                  'border-t hover:bg-muted/50 transition-colors',
                  !m.isReachable && 'opacity-60'
                )}
              >
                <td className="px-3 py-2 font-mono text-xs">{serverId}</td>
                <td className="px-3 py-2">
                  <div className="flex items-center gap-2">
                    <StatusDot isReachable={m.isReachable} cpuPct={m.cpuUsagePercent} />
                    <span className="text-xs">
                      {m.isReachable ? 'healthy' : 'unreachable'}
                    </span>
                  </div>
                </td>
                <td className="px-3 py-2 text-xs text-muted-foreground">
                  {uptimeDays != null ? `${uptimeDays}d` : '—'}
                </td>
                <td className="px-3 py-2 text-xs font-mono">
                  {m.cpuUsagePercent != null ? `${m.cpuUsagePercent.toFixed(0)}%` : '—'}
                </td>
                <td className="px-3 py-2 text-xs font-mono">
                  {m.memUsagePercent != null ? `${m.memUsagePercent.toFixed(0)}%` : '—'}
                </td>
                <td className="px-3 py-2 text-xs font-mono">
                  {m.diskUsagePercent != null ? `${m.diskUsagePercent.toFixed(0)}%` : '—'}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
```

---

### Page: `src/renderer/src/pages/admin/fleet.tsx`

**File:** `src/renderer/src/pages/admin/fleet.tsx` (TẠO MỚI)

```typescript
// Admin Fleet page — entry point cho Admin SPA
import { FleetDashboard } from '@/components/admin/fleet/fleet-dashboard'

export default function FleetPage() {
  return (
    <div className="fleet-page p-6">
      <h1 className="text-2xl font-bold mb-6">Fleet Management</h1>
      <FleetDashboard />
    </div>
  )
}
```

---

### Zustand Store Extensions

**File:** `src/renderer/src/store/slices/ssh.ts` — MODIFY

```typescript
// Thêm Fleet state theo TDD-FE-10 §2
// serverHealthMetrics, lastFleetHealthCheck, fleetAlerts đã được spec
// Thêm actions: updateServerHealth, setLastFleetHealthCheck, addFleetAlert, dismissFleetAlert
```

---

## Files cần tạo/sửa

| File | Action | BL |
|------|--------|-----|
| `src/renderer/src/components/admin/fleet/fleet-dashboard.tsx` | CREATE | BL-FLEET-03 |
| `src/renderer/src/components/admin/fleet/fleet-health-table.tsx` | CREATE | BL-FLEET-01 |
| `src/renderer/src/components/admin/fleet/server-health-card.tsx` | CREATE | BL-FLEET-03 |
| `src/renderer/src/components/admin/fleet/onboarding-wizard.tsx` | CREATE | BL-FLEET-04 |
| `src/renderer/src/components/admin/fleet/fleet-import-dialog.tsx` | CREATE | BL-FLEET-01 |
| `src/renderer/src/components/admin/fleet/fleet-alert-strip.tsx` | CREATE | BL-FLEET-03 |
| `src/renderer/src/pages/admin/fleet.tsx` | CREATE | BL-FLEET-01 |
| `src/renderer/src/store/slices/ssh.ts` | MODIFY | CR-005 |
| `src/renderer/src/hooks/useFleetHealthPolling.ts` | CREATE | CR-005 |
| `src/renderer/src/hooks/useBootstrapAutomation.ts` | CREATE | CR-004 |

---

## Liên quan

- **BL-FLEET-01**: Fleet inventory table ✅ implemented (`FleetHealthTable`)
- **BL-FLEET-03**: Real-time health dashboard ✅ implemented (`FleetDashboard` + `useFleetHealthPolling`)
- **BL-FLEET-04**: Onboarding Wizard ✅ implemented (`OnboardingWizard`)
- **TDD-FE-10**: §2 Store, §3 Hooks, §4 Components, §5 IPC API
- **TDD-FE-07**: §Addendum Fleet Hooks — `useFleetHealthPolling`, `useBootstrapAutomation`
