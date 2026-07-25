# SOL-CR-003 — Frontend Solution: Bulk Server Provisioning

**CR:** CR-003 — Bulk Server Provisioning from Fleet Inventory  
**Priority:** 🟠 High  
**TDD References:** TDD-FE-02 (State Management), TDD-FE-05 (UI Components), TDD-FE-07 (Hooks & IPC)  
**Depends on:** SOL-CR-001 (FleetConfig import), SOL-CR-002 (SshTarget types)  
**Estimated effort:** 3–5 ngày frontend  
**Implementation Status:** ✅ IMPLEMENTED — 2026-07-23  
**Tasks:** TASK-003-A (ProvisioningSlice), TASK-003-B (IPC/preload), TASK-003-C (FleetProvisionWizard), TASK-003-D (ProvisionProgressPanel)

---

## 1. Tổng quan giải pháp

CR-003 yêu cầu **Bulk Provisioning** — deploy orca-relay lên nhiều servers cùng lúc. Frontend cần:

1. Thêm **ProvisioningSlice** mới vào Zustand store để track tiến trình
2. Thêm **FleetProvisionWizard** component (chọn servers → provision → monitor)
3. Handle **streaming progress events** từ backend qua IPC
4. Hiển thị **per-server status** trong real-time

---

## 2. Zustand Slice mới — `provisioningSlice`

```typescript
// src/renderer/src/store/slices/provisioning.ts
// [NEW FILE]

// Types
export type ProvisioningServerStatus =
  | 'pending'
  | 'connecting'
  | 'deploying-relay'
  | 'done'
  | 'error'
  | 'skipped'

export type ProvisioningServerEntry = {
  serverId: string
  label: string
  host: string
  status: ProvisioningServerStatus
  error: string | null
  startedAt: number | null
  completedAt: number | null
  relayVersion: string | null
}

export type ProvisioningSession = {
  sessionId: string
  startedAt: number
  phase: 'selecting' | 'running' | 'done' | 'cancelled'
  servers: ProvisioningServerEntry[]
  concurrency: number             // parallel provision slots
}

export type ProvisioningSlice = {
  provisioningSession: ProvisioningSession | null

  // Actions
  startProvisioningSession: (serverIds: string[]) => void
  updateProvisioningServerStatus: (
    serverId: string,
    update: Partial<ProvisioningServerEntry>
  ) => void
  finishProvisioningSession: () => void
  cancelProvisioningSession: () => void
}

// Slice factory
export const createProvisioningSlice: StateCreator<
  AppState,
  [],
  [],
  ProvisioningSlice
> = (set) => ({
  provisioningSession: null,

  startProvisioningSession: (serverIds) => {
    const targets = useAppStore.getState().sshTargets ?? []
    set(s => {
      s.provisioningSession = {
        sessionId: crypto.randomUUID(),
        startedAt: Date.now(),
        phase: 'running',
        concurrency: 3,
        servers: serverIds.map(id => {
          const target = targets.find(t => t.id === id)
          return {
            serverId: id,
            label: target?.label ?? id,
            host: target?.host ?? '',
            status: 'pending',
            error: null,
            startedAt: null,
            completedAt: null,
            relayVersion: null,
          }
        }),
      }
    })
  },

  updateProvisioningServerStatus: (serverId, update) =>
    set(s => {
      const session = s.provisioningSession
      if (!session) return
      const entry = session.servers.find(e => e.serverId === serverId)
      if (entry) Object.assign(entry, update)
    }),

  finishProvisioningSession: () =>
    set(s => {
      if (s.provisioningSession) s.provisioningSession.phase = 'done'
    }),

  cancelProvisioningSession: () =>
    set(s => { s.provisioningSession = null }),
})
```

### Đăng ký slice vào AppState

```typescript
// src/renderer/src/store/types.ts
type AppState = /* existing */ & ProvisioningSlice

// src/renderer/src/store/index.ts
export const useAppStore = create<AppState>()((...a) => ({
  // ...existing slices...
  ...createProvisioningSlice(...a),    // [NEW]
}))
```

---

## 3. IPC Events cho Provisioning

```typescript
// src/renderer/src/hooks/useIpcEvents.ts
// Thêm handlers cho provisioning events

// Trong useIpcEvents() useEffect:

// [NEW] Fleet provisioning events
window.api.ssh.onProvisioningProgress?.((event) => {
  const store = useAppStore.getState()

  switch (event.type) {
    case 'server.started':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'connecting',
        startedAt: Date.now(),
      })
      break

    case 'server.relay-deploying':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'deploying-relay',
      })
      break

    case 'server.done':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'done',
        completedAt: Date.now(),
        relayVersion: event.relayVersion,
      })
      scheduleRuntimeGraphSync()
      break

    case 'server.error':
      store.updateProvisioningServerStatus(event.serverId, {
        status: 'error',
        error: event.error,
        completedAt: Date.now(),
      })
      break

    case 'session.done':
      store.finishProvisioningSession()
      // Toast summary
      const session = useAppStore.getState().provisioningSession
      if (session) {
        const done = session.servers.filter(s => s.status === 'done').length
        const failed = session.servers.filter(s => s.status === 'error').length
        toast.success(
          translate(
            'fleet.provision.done',
            `Provisioning complete: ${done} servers ready, ${failed} failed`
          )
        )
      }
      break
  }
})
```

---

## 4. UI Components

### 4.1 `FleetProvisionWizard` — Multi-step wizard

```typescript
// src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx
// Lazy loaded:
// const FleetProvisionWizard = lazyWithRetry(() =>
//   import('./FleetProvisionWizard'))

type WizardStep = 'select' | 'confirm' | 'provision' | 'done'

export function FleetProvisionWizard({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const [step, setStep] = useState<WizardStep>('select')
  const [selectedServerIds, setSelectedServerIds] = useState<Set<string>>(new Set())
  const session = useAppStore(s => s.provisioningSession)

  const handleStartProvision = async () => {
    const ids = Array.from(selectedServerIds)
    useAppStore.getState().startProvisioningSession(ids)
    setStep('provision')

    try {
      await window.api.ssh.provisionFleetServers({
        serverIds: ids,
        concurrency: 3,
      })
    } catch (err) {
      toast.error(translate('fleet.provision.error', 'Provisioning failed to start'))
      useAppStore.getState().cancelProvisioningSession()
      setStep('select')
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>
            {translate('fleet.provision.title', 'Provision Fleet Servers')}
          </DialogTitle>
        </DialogHeader>

        {/* Step indicator */}
        <ProvisionWizardSteps currentStep={step} />

        {/* Step content */}
        {step === 'select' && (
          <ProvisionServerSelector
            selectedIds={selectedServerIds}
            onSelectionChange={setSelectedServerIds}
          />
        )}

        {step === 'confirm' && (
          <ProvisionConfirmStep
            serverIds={Array.from(selectedServerIds)}
            onBack={() => setStep('select')}
            onConfirm={handleStartProvision}
          />
        )}

        {step === 'provision' && session && (
          <ProvisionProgressPanel session={session} />
        )}

        {step === 'done' && (
          <ProvisionDoneSummary
            session={session}
            onClose={onClose}
          />
        )}

        {/* Footer */}
        {step === 'select' && (
          <DialogFooter>
            <Button variant="ghost" onClick={onClose}>
              {translate('common.cancel', 'Cancel')}
            </Button>
            <Button
              onClick={() => setStep('confirm')}
              disabled={selectedServerIds.size === 0}
            >
              {translate('fleet.provision.next', 'Next')}
              <span className="ml-1 text-xs opacity-70">
                ({selectedServerIds.size} selected)
              </span>
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

### 4.2 `ProvisionServerSelector` — Chọn servers để provision

```typescript
// src/renderer/src/components/settings/ssh/ProvisionServerSelector.tsx

export function ProvisionServerSelector({
  selectedIds,
  onSelectionChange,
}: {
  selectedIds: Set<string>
  onSelectionChange: (ids: Set<string>) => void
}) {
  const sshTargets = useAppStore(s => s.sshTargets ?? [])
  const connectionStates = useAppStore(s => s.sshConnectionStates)

  // Group by project
  const grouped = useMemo(() => {
    return sshTargets.reduce<Record<string, SshTarget[]>>((acc, t) => {
      const key = t.project ?? '__unassigned__'
      if (!acc[key]) acc[key] = []
      acc[key].push(t)
      return acc
    }, {})
  }, [sshTargets])

  const toggleAll = () => {
    if (selectedIds.size === sshTargets.length) {
      onSelectionChange(new Set())
    } else {
      onSelectionChange(new Set(sshTargets.map(t => t.id)))
    }
  }

  return (
    <div className="space-y-3">
      {/* Select all */}
      <div className="flex items-center gap-2 pb-2 border-b">
        <Checkbox
          id="select-all"
          checked={selectedIds.size === sshTargets.length && sshTargets.length > 0}
          onCheckedChange={toggleAll}
        />
        <label htmlFor="select-all" className="text-sm cursor-pointer">
          {translate('fleet.provision.selectAll', 'Select all servers')}
        </label>
        <span className="ml-auto text-xs text-muted-foreground">
          {selectedIds.size}/{sshTargets.length} selected
        </span>
      </div>

      {/* Grouped server list */}
      <ScrollArea className="max-h-[320px]">
        <div className="space-y-3">
          {Object.entries(grouped).map(([project, targets]) => (
            <div key={project}>
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">
                {project === '__unassigned__'
                  ? translate('fleet.group.unassigned', 'Unassigned')
                  : project}
              </p>
              <div className="space-y-0.5">
                {targets.map(target => {
                  const connState = connectionStates[target.id]
                  const isConnected = connState?.status === 'connected'

                  return (
                    <label
                      key={target.id}
                      className="flex items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/50 cursor-pointer"
                    >
                      <Checkbox
                        checked={selectedIds.has(target.id)}
                        onCheckedChange={(checked) => {
                          const next = new Set(selectedIds)
                          checked ? next.add(target.id) : next.delete(target.id)
                          onSelectionChange(next)
                        }}
                      />
                      <SshConnectionStatusDot
                        status={connState?.status ?? 'disconnected'}
                      />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm truncate">{target.label}</p>
                        <p className="text-xs text-muted-foreground">
                          {target.host}
                        </p>
                      </div>
                      {isConnected && (
                        <Badge variant="outline" className="text-xs text-green-600">
                          {translate('fleet.provision.relayReady', 'relay active')}
                        </Badge>
                      )}
                    </label>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}
```

### 4.3 `ProvisionProgressPanel` — Real-time progress

```typescript
// src/renderer/src/components/settings/ssh/ProvisionProgressPanel.tsx

export function ProvisionProgressPanel({
  session,
}: {
  session: ProvisioningSession
}) {
  const doneCount = session.servers.filter(s =>
    s.status === 'done' || s.status === 'error' || s.status === 'skipped'
  ).length
  const successCount = session.servers.filter(s => s.status === 'done').length
  const failCount = session.servers.filter(s => s.status === 'error').length
  const totalCount = session.servers.length
  const progress = totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0

  return (
    <div className="space-y-4">
      {/* Overall progress */}
      <div className="space-y-2">
        <div className="flex justify-between text-sm">
          <span className="text-muted-foreground">
            {translate('fleet.provision.progress', 'Provisioning in progress...')}
          </span>
          <span className="font-medium">{progress}%</span>
        </div>
        <Progress value={progress} className="h-2" />
        <div className="flex gap-4 text-xs text-muted-foreground">
          <span>✓ {successCount} done</span>
          {failCount > 0 && <span className="text-destructive">✗ {failCount} failed</span>}
          <span>{totalCount - doneCount} remaining</span>
        </div>
      </div>

      {/* Per-server status list */}
      <ScrollArea className="max-h-[280px]">
        <div className="space-y-1">
          {session.servers.map(server => (
            <ProvisionServerStatusRow key={server.serverId} entry={server} />
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}

function ProvisionServerStatusRow({ entry }: { entry: ProvisioningServerEntry }) {
  const statusConfig: Record<ProvisioningServerStatus, {
    icon: React.ReactNode
    label: string
    className: string
  }> = {
    pending:         { icon: <ClockIcon />, label: 'Waiting...', className: 'text-muted-foreground' },
    connecting:      { icon: <LoaderIcon className="animate-spin" />, label: 'Connecting...', className: 'text-blue-500' },
    'deploying-relay': { icon: <UploadIcon />, label: 'Deploying relay...', className: 'text-yellow-500' },
    done:            { icon: <CheckCircleIcon />, label: 'Ready', className: 'text-green-500' },
    error:           { icon: <XCircleIcon />, label: entry.error ?? 'Error', className: 'text-destructive' },
    skipped:         { icon: <MinusIcon />, label: 'Skipped', className: 'text-muted-foreground' },
  }

  const config = statusConfig[entry.status]

  return (
    <div className="flex items-center gap-2 px-1 py-1.5">
      <span className={cn('flex-shrink-0', config.className)}>
        {config.icon}
      </span>
      <div className="flex-1 min-w-0">
        <p className="text-sm truncate">{entry.label}</p>
        <p className={cn('text-xs truncate', config.className)}>
          {config.label}
        </p>
      </div>
      {entry.status === 'done' && entry.relayVersion && (
        <Badge variant="secondary" className="text-xs">
          v{entry.relayVersion}
        </Badge>
      )}
    </div>
  )
}
```

---

## 5. File mới cần tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/provisioning.ts` | [NEW] | Provisioning session state |
| `src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx` | [NEW] | Provision wizard shell |
| `src/renderer/src/components/settings/ssh/ProvisionServerSelector.tsx` | [NEW] | Server selection step |
| `src/renderer/src/components/settings/ssh/ProvisionProgressPanel.tsx` | [NEW] | Real-time progress |
| `src/renderer/src/components/settings/ssh/ProvisionConfirmStep.tsx` | [NEW] | Confirmation step |
| `src/renderer/src/components/settings/ssh/ProvisionDoneSummary.tsx` | [NEW] | Done summary |

## 6. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/types.ts` | Thêm `ProvisioningSlice` |
| `src/renderer/src/store/index.ts` | Register `createProvisioningSlice` |
| `src/renderer/src/hooks/useIpcEvents.ts` | Thêm `onProvisioningProgress` handlers |
| `src/renderer/src/components/settings/ssh/SshSettingsPanel.tsx` | Thêm "Provision Fleet" button |
| `src/preload/index.ts` | Expose `ssh.provisionFleetServers`, `ssh.cancelProvisioning` |

---

## 7. Acceptance Criteria (Frontend)

- [x] Button "Provision Fleet" mở wizard từ Settings → SSH & Remotes
- [x] Step 1: Chọn servers (grouped by project, checkbox, select-all)
- [x] Step 2: Confirm — hiển thị danh sách servers sẽ được provision
- [x] Step 3: Progress — per-server status cập nhật real-time
- [x] Mỗi server row hiển thị: pending → connecting → deploying-relay → done/error
- [x] Overall progress bar với %, done count, fail count
- [x] Provision chạy song song (3 concurrent slots)
- [x] Cancel button dừng provisioning
- [x] Khi done: toast summary + dialog chuyển sang "Done" step
- [x] Error servers: hiển thị error message, có thể retry

## 8. Implementation Notes

> **Implemented 2026-07-23**
>
> - `src/renderer/src/store/slices/provisioning.ts`: [NEW] `ProvisioningSlice` — `ProvisioningSession`, `ProvisioningServerStatus`, init/update/finish/cancel actions.
> - `src/renderer/src/store/slices/provisioning-events.ts`: [NEW] `ProvisioningProgressEvent` discriminated union (7 event types).
> - `src/preload/api-types.ts`: Added `provisionFleetServers`, `cancelProvisioning`, `onProvisioningProgress` to `ssh` namespace.
> - `src/preload/index.ts`: IPC bridges for provisioning methods.
> - `src/renderer/src/web/web-preload-api.ts`: No-op stubs.
> - `src/renderer/src/hooks/useIpcEvents.ts`: `onProvisioningProgress` handler with all 7 event cases.
> - `src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx`: [NEW] 3-step modal wizard.
> - `src/renderer/src/components/settings/ssh/ProvisionServerSelector.tsx`: [NEW] Step 1 server selection.
> - `src/renderer/src/components/settings/ssh/ProvisionConfirmStep.tsx`: [NEW] Step 2 confirmation.
> - `src/renderer/src/components/settings/ssh/ProvisionProgressPanel.tsx`: [NEW] Step 3 real-time progress.
> - `src/renderer/src/components/settings/ssh/ProvisionDoneSummary.tsx`: [NEW] Completion summary.
> - `src/renderer/src/components/settings/SshPane.tsx`: Integrated "Provision Fleet" button + wizard.
> - **TypeScript:** ✅ 0 new errors.
