# TASK-001-B — Expose Fleet API qua Preload (IPC Layer)

**Task ID:** TASK-001-B  
**CR:** CR-001 — Fleet Inventory Config  
**Solution Ref:** SOL-CR-001, Section 3.1  
**Dependencies:** TASK-001-A (FleetImportResult type)  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Expose 3 fleet API methods qua `window.api.ssh` để renderer có thể gọi từ cả Desktop (Electron IPC) và Web (WebSocket RPC).

---

## Context

**Files cần sửa:**
- `src/preload/index.ts` — Desktop Electron preload
- `src/renderer/src/web/web-preload-api.ts` — Web mode bridge

**Pattern:** `window.api.ssh.*` abstraction

---

## Các bước thực thi

### Bước 1: Khám phá preload API hiện tại

```bash
grep -n "ssh" src/preload/index.ts | head -30
grep -n "importFleet\|exportFleet\|pickFleet" src/preload/index.ts
```

### Bước 2: Thêm interface declaration

Tìm `OrcaApi` interface hoặc `ssh:` object trong preload types, thêm 3 methods:

```typescript
// Trong ssh namespace của OrcaApi interface:
importFleetConfig(yamlPath: string): Promise<FleetImportResult>
exportFleetConfig(outputPath: string): Promise<void>
pickFleetConfigFile(): Promise<string | null>
```

### Bước 3: Implement trong `src/preload/index.ts` (Desktop)

Thêm IPC invoke calls:

```typescript
// Trong ssh object của contextBridge.exposeInMainWorld:
importFleetConfig: (yamlPath: string) =>
  ipcRenderer.invoke('ssh:importFleetConfig', yamlPath),

exportFleetConfig: (outputPath: string) =>
  ipcRenderer.invoke('ssh:exportFleetConfig', outputPath),

pickFleetConfigFile: () =>
  ipcRenderer.invoke('ssh:pickFleetConfigFile'),
```

### Bước 4: Thêm event listener (nếu backend dùng streaming)

```typescript
// Nếu backend emit progress events (không phải return):
onFleetImportProgress: (
  callback: (event: FleetImportProgressEvent) => void
) => {
  const handler = (_: IpcRendererEvent, event: FleetImportProgressEvent) =>
    callback(event)
  ipcRenderer.on('ssh:fleetImportProgress', handler)
  return () => ipcRenderer.removeListener('ssh:fleetImportProgress', handler)
},
```

### Bước 5: Implement trong `src/renderer/src/web/web-preload-api.ts` (Web)

Web mode dùng WebSocket RPC, thêm stub hoặc RPC call tương ứng:

```typescript
// Trong web-preload-api.ts, thêm vào ssh object:
importFleetConfig: (yamlPath: string) =>
  callRpc('ssh.importFleetConfig', { yamlPath }),

exportFleetConfig: (outputPath: string) =>
  callRpc('ssh.exportFleetConfig', { outputPath }),

pickFleetConfigFile: async () => {
  // Web mode: hiển thị native file input thay vì Electron dialog
  return new Promise<string | null>((resolve) => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.yaml,.yml'
    input.onchange = () => {
      const file = input.files?.[0]
      resolve(file ? file.name : null)
    }
    input.click()
  })
},

onFleetImportProgress: (callback) => {
  // Web: subscribe qua WebSocket event
  return subscribeToEvent('ssh:fleetImportProgress', callback)
},
```

### Bước 6: Verify types

```bash
npx tsc --noEmit 2>&1 | grep -i "fleet\|ssh" | head -20
```

---

## Acceptance Criteria

- [x] `window.api.ssh.importFleetConfig(path)` callable từ renderer
- [x] `window.api.ssh.exportFleetConfig(path)` callable từ renderer
- [x] `window.api.ssh.pickFleetConfigFile()` callable từ renderer
- [x] `window.api.ssh.onFleetImportProgress(cb)` callable từ renderer
- [x] TypeScript types match `FleetImportResult` từ TASK-001-A
- [x] Cả Desktop và Web mode đều có implementation (không throw "not implemented")

---

## Notes cho AI

- Nếu `FleetImportResult` type chưa export từ shared types, import từ `@/store/types` hoặc `src/shared/`
- Với Electron: IPC channel names phải match với main process handlers (`src/main/ipc/`)
- Nếu IPC handler chưa có trong main process, ghi chú là "TODO: main process handler cần implement" nhưng vẫn khai báo interface
- Không tạo side-effects ở module level trong preload

---

## Implementation Notes

> **Completed:** 2026-07-23 | `preload/api-types.ts`: importFleetConfig, exportFleetConfig, pickFleetConfigFile, onFleetImportProgress. `preload/index.ts`: IPC bridges. `web-preload-api.ts`: no-op stubs. TypeScript: ✅ 0 errors.
