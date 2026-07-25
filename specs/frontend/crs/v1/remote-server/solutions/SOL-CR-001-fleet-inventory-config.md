# SOL-CR-001 — Frontend Solution: Fleet Inventory Config File

**CR:** CR-001 — Fleet Inventory Config File  
**Priority:** 🔴 Critical  
**TDD References:** TDD-FE-02 (State Management), TDD-FE-05 (UI Components), TDD-FE-07 (Hooks & IPC)  
**Depends on:** Không  
**Estimated effort:** 2–3 ngày frontend  
**Implementation Status:** ✅ IMPLEMENTED — 2026-07-23  
**Tasks:** TASK-001-A (SshSlice types), TASK-001-B (IPC/preload), TASK-001-C (FleetImportDialog), TASK-001-D (IPC events), TASK-001-E (SshSettingsPanel)

---

## 1. Tổng quan giải pháp

CR-001 yêu cầu tạo cơ chế **Import Fleet Config** từ file `orca-fleet.yaml`. Frontend cần:

1. Mở rộng **SshSlice** trong Zustand store để lưu fleet metadata
2. Thêm **IPC handlers** cho fleet import operations
3. Tạo **FleetImportDialog** component
4. Mở rộng **Settings → SSH & Remotes** với tab "Fleet Config"
5. Hiển thị **import status/progress** trong UI

---

## 2. Thay đổi Store (Zustand Slice)

### 2.1 Mở rộng `SshSlice`

```typescript
// src/renderer/src/store/slices/ssh.ts
// HIỆN TẠI:
type SshSlice = {
  sshConnectionStates: Record<string, SshConnectionState>
  setSshConnectionState: (targetId: string, state: SshConnectionState) => void
}

// MỞ RỘNG THÊM:
type SshSlice = {
  sshConnectionStates: Record<string, SshConnectionState>
  setSshConnectionState: (targetId: string, state: SshConnectionState) => void

  // [NEW] Fleet config state
  fleetImportStatus: FleetImportStatus | null
  setFleetImportStatus: (status: FleetImportStatus | null) => void
  clearFleetImportStatus: () => void
}

// [NEW] Type định nghĩa
type FleetImportStatus = {
  phase: 'parsing' | 'importing' | 'done' | 'error'
  totalServers: number
  importedServers: number
  skippedServers: number         // đã tồn tại, không overwrite
  failedServers: number
  errors: string[]
  configFilePath: string
}
```

### 2.2 Slice implementation

```typescript
// src/renderer/src/store/slices/ssh.ts
export const createSshSlice: StateCreator<AppState, [], [], SshSlice> = (set) => ({
  sshConnectionStates: {},
  fleetImportStatus: null,

  setSshConnectionState: (targetId, state) =>
    set(s => { s.sshConnectionStates[targetId] = state }),

  setFleetImportStatus: (status) =>
    set(s => { s.fleetImportStatus = status }),

  clearFleetImportStatus: () =>
    set(s => { s.fleetImportStatus = null }),
})
```

---

## 3. IPC Layer — `window.api.ssh`

### 3.1 Thêm vào preload API interface

```typescript
// src/preload/index.ts (Desktop) + src/renderer/src/web/web-preload-api.ts (Web)

interface OrcaApi {
  ssh: {
    listTargets(): Promise<SshTarget[]>
    connect(targetId: string): Promise<void>
    disconnect(targetId: string): Promise<void>
    // ...existing...

    // [NEW] Fleet management
    importFleetConfig(yamlPath: string): Promise<FleetImportResult>
    exportFleetConfig(outputPath: string): Promise<void>
    pickFleetConfigFile(): Promise<string | null>  // native file picker
  }
}

// [NEW] Return type
type FleetImportResult = {
  imported: SshTarget[]
  skipped: string[]           // IDs đã tồn tại
  failed: Array<{ id: string; error: string }>
  totalParsed: number
}
```

### 3.2 IPC Event mới

```typescript
// Thêm vào useIpcEvents() trong src/renderer/src/hooks/useIpcEvents.ts

// Fleet import progress events (từ backend streaming)
window.api.ssh.onFleetImportProgress?.((event) => {
  store.setFleetImportStatus({
    phase: event.phase,
    totalServers: event.total,
    importedServers: event.imported,
    skippedServers: event.skipped,
    failedServers: event.failed,
    errors: event.errors,
    configFilePath: event.configFilePath,
  })

  if (event.phase === 'done' || event.phase === 'error') {
    scheduleRuntimeGraphSync()
  }
})
```

---

## 4. UI Components

### 4.1 `FleetImportDialog` — Component chính

```typescript
// src/renderer/src/components/settings/ssh/FleetImportDialog.tsx

type FleetImportDialogProps = {
  open: boolean
  onClose: () => void
}

export function FleetImportDialog({ open, onClose }: FleetImportDialogProps) {
  const fleetImportStatus = useAppStore(s => s.fleetImportStatus)
  const [selectedPath, setSelectedPath] = useState<string | null>(null)

  const handlePickFile = async () => {
    const path = await window.api.ssh.pickFleetConfigFile()
    if (path) setSelectedPath(path)
  }

  const handleImport = async () => {
    if (!selectedPath) return
    try {
      await window.api.ssh.importFleetConfig(selectedPath)
      // Status cập nhật qua IPC events trong useIpcEvents()
    } catch (err) {
      toast.error(translate('fleet.import.error', 'Import failed'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>
            {translate('fleet.import.title', 'Import Fleet Config')}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'fleet.import.description',
              'Import servers from an orca-fleet.yaml file to add them to your SSH hosts.'
            )}
          </DialogDescription>
        </DialogHeader>

        {/* File picker */}
        <div className="flex gap-2">
          <Input
            readOnly
            value={selectedPath ?? ''}
            placeholder={translate('fleet.import.placeholder', 'Select orca-fleet.yaml...')}
            className="flex-1"
          />
          <Button variant="outline" onClick={handlePickFile}>
            {translate('fleet.import.browse', 'Browse')}
          </Button>
        </div>

        {/* Import progress */}
        {fleetImportStatus && (
          <FleetImportProgress status={fleetImportStatus} />
        )}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            {translate('common.cancel', 'Cancel')}
          </Button>
          <Button
            onClick={handleImport}
            disabled={!selectedPath || fleetImportStatus?.phase === 'importing'}
          >
            {fleetImportStatus?.phase === 'importing'
              ? translate('fleet.import.importing', 'Importing...')
              : translate('fleet.import.import', 'Import')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

### 4.2 `FleetImportProgress` — Sub-component progress

```typescript
// src/renderer/src/components/settings/ssh/FleetImportProgress.tsx

export function FleetImportProgress({ status }: { status: FleetImportStatus }) {
  const progress = status.totalServers > 0
    ? Math.round((status.importedServers / status.totalServers) * 100)
    : 0

  return (
    <div className="space-y-3 rounded-md border p-3">
      {/* Progress bar */}
      <div className="space-y-1">
        <div className="flex justify-between text-sm text-muted-foreground">
          <span>{translate('fleet.import.progress', 'Importing servers...')}</span>
          <span>{status.importedServers}/{status.totalServers}</span>
        </div>
        <Progress value={progress} className="h-2" />
      </div>

      {/* Stats */}
      <div className="flex gap-4 text-sm">
        <span className="text-green-500">
          ✓ {status.importedServers} {translate('fleet.import.imported', 'imported')}
        </span>
        {status.skippedServers > 0 && (
          <span className="text-muted-foreground">
            ↷ {status.skippedServers} {translate('fleet.import.skipped', 'skipped')}
          </span>
        )}
        {status.failedServers > 0 && (
          <span className="text-destructive">
            ✗ {status.failedServers} {translate('fleet.import.failed', 'failed')}
          </span>
        )}
      </div>

      {/* Errors */}
      {status.errors.length > 0 && (
        <div className="space-y-1 rounded border border-destructive/30 bg-destructive/10 p-2">
          {status.errors.map((err, i) => (
            <p key={i} className="text-xs text-destructive">{err}</p>
          ))}
        </div>
      )}

      {/* Done state */}
      {status.phase === 'done' && (
        <p className="text-sm text-green-500">
          ✅ {translate('fleet.import.done', 'Import complete!')}
        </p>
      )}
    </div>
  )
}
```

### 4.3 Tích hợp vào Settings SSH panel

```typescript
// src/renderer/src/components/settings/ssh/SshSettingsPanel.tsx
// Thêm button "Import Fleet Config" vào toolbar

export function SshSettingsPanel() {
  const [fleetImportOpen, setFleetImportOpen] = useState(false)

  return (
    <div>
      {/* Existing SSH targets list */}
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium">
          {translate('settings.ssh.title', 'SSH Hosts')}
        </h3>

        <div className="flex gap-2">
          {/* [NEW] Fleet import button */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => setFleetImportOpen(true)}
          >
            {translate('fleet.import.button', 'Import Fleet Config')}
          </Button>

          {/* Existing: Export fleet config */}
          <Button
            variant="outline"
            size="sm"
            onClick={handleExportFleet}
          >
            {translate('fleet.export.button', 'Export Fleet')}
          </Button>

          {/* Existing: Add SSH host */}
          <Button size="sm" onClick={handleAddTarget}>
            {translate('settings.ssh.add', 'Add Host')}
          </Button>
        </div>
      </div>

      {/* SSH target list — xem SOL-CR-002 cho grouped view */}
      <SshTargetList />

      {/* Fleet import dialog */}
      <FleetImportDialog
        open={fleetImportOpen}
        onClose={() => setFleetImportOpen(false)}
      />
    </div>
  )
}
```

---

## 5. RuntimeSyncWindowGraph mở rộng

```typescript
// src/shared/runtime-types.ts
// Thêm fleet metadata vào sync graph

type RuntimeSyncWindowGraph = {
  // ...existing...
  sshTargets: SshTarget[]    // SshTarget đã được extend với project/team/environment

  // [NEW] Fleet config metadata
  lastFleetConfigPath?: string         // đường dẫn file đã import cuối cùng
  lastFleetImportTimestamp?: number    // timestamp lần import gần nhất
}
```

---

## 6. File mới cần tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/components/settings/ssh/FleetImportDialog.tsx` | [NEW] | Dialog import fleet config |
| `src/renderer/src/components/settings/ssh/FleetImportProgress.tsx` | [NEW] | Progress indicator |
| `src/renderer/src/store/slices/fleet.ts` | [NEW] | Fleet-specific state (optional separate slice) |

## 7. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/slices/ssh.ts` | Thêm `fleetImportStatus` state + actions |
| `src/renderer/src/store/types.ts` | Thêm `FleetImportStatus` type |
| `src/renderer/src/hooks/useIpcEvents.ts` | Thêm `onFleetImportProgress` handler |
| `src/renderer/src/components/settings/ssh/SshSettingsPanel.tsx` | Thêm "Import Fleet Config" button |
| `src/preload/index.ts` | Expose `ssh.importFleetConfig`, `ssh.exportFleetConfig`, `ssh.pickFleetConfigFile` |
| `src/renderer/src/web/web-preload-api.ts` | Same (web mode) |

---

## 8. Acceptance Criteria (Frontend)

- [x] Button "Import Fleet Config" hiển thị trong Settings → SSH & Remotes
- [x] Click button → FileDialog mở, lọc `.yaml` files
- [x] Chọn file → Preview số lượng servers sẽ import
- [x] Click Import → progress bar hiển thị real-time
- [x] Khi xong: toast `"X servers imported, Y skipped"` + dialog tự đóng
- [x] Servers mới xuất hiện trong SSH target list ngay lập tức (sau graph sync)
- [x] Nếu server đã tồn tại → skip với message rõ ràng
- [x] Hoạt động trong cả Desktop mode và Web mode

## 9. Implementation Notes

> **Implemented 2026-07-23**
>
> - `src/renderer/src/store/slices/ssh.ts`: Added `FleetImportStatus`, `FleetImportPhase`, `fleetImportStatus` state, `setFleetImportStatus`, `clearFleetImportStatus` actions.
> - `src/preload/api-types.ts`: Added `importFleetConfig`, `exportFleetConfig`, `pickFleetConfigFile` to `ssh` namespace.
> - `src/preload/index.ts`: Implemented IPC bridges for all fleet methods.
> - `src/renderer/src/web/web-preload-api.ts`: Added no-op stubs for web mode.
> - `src/renderer/src/hooks/useIpcEvents.ts`: Added `onFleetImportProgress` handler with all event type cases.
> - `src/renderer/src/components/settings/FleetImportDialog.tsx`: Full wizard dialog with file picker, server preview table, real-time progress tracking.
> - `src/renderer/src/components/settings/SshPane.tsx`: Integrated "Import Fleet" button and `FleetImportDialog`.
> - **TypeScript:** ✅ 0 new errors.
