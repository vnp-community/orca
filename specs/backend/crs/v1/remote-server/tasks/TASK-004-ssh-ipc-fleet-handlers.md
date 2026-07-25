# TASK-004: Thêm IPC Handlers — importFleetConfig, exportFleetConfig, watchFleetConfig

**Source:** SOL-001  
**Phase:** 1 | **Effort:** S (30–90 min)  
**Depends on:** TASK-003

---

## Objective

Thêm 3 IPC handlers vào SSH IPC module để expose fleet config operations cho renderer/CLI:
- `ssh:importFleetConfig` — import từ YAML file
- `ssh:exportFleetConfig` — export hiện tại ra FleetConfig object
- `ssh:watchFleetConfig` — watch YAML file và auto re-import khi thay đổi

---

## File to modify

**`src/main/ipc/ssh.ts`** (hoặc file IPC tương đương — tìm file có `ipcMain.handle('ssh:`)

---

## Implementation

### Step 1: Add imports at top

```typescript
import * as fs from 'node:fs'
import type { BrowserWindow } from 'electron'
```

### Step 2: Thêm handlers (trong function đăng ký IPC, cùng scope với các handler khác)

```typescript
  // ── Fleet Config IPC Handlers (NEW) ─────────────────────────

  ipcMain.handle('ssh:importFleetConfig', async (_event, { filePath }: { filePath: string }) => {
    try {
      const result = await sshConnectionStore.importFromFleetConfig(filePath)
      // Trigger runtime graph sync to push new targets to renderer
      scheduleRuntimeGraphSync()
      return { ok: true, result }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      return { ok: false, error: message }
    }
  })

  ipcMain.handle('ssh:exportFleetConfig', async () => {
    try {
      const config = sshConnectionStore.exportToFleetConfig()
      return { ok: true, config }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      return { ok: false, error: message }
    }
  })

  ipcMain.handle('ssh:watchFleetConfig', async (_event, { filePath }: { filePath: string }) => {
    try {
      // Stop existing watcher for this path (if any)
      existingFleetWatchers.get(filePath)?.close()

      const watcher = fs.watch(filePath, { persistent: false }, async () => {
        try {
          await sshConnectionStore.importFromFleetConfig(filePath)
          scheduleRuntimeGraphSync()
          // Notify all renderer windows
          for (const win of BrowserWindow.getAllWindows()) {
            if (!win.isDestroyed()) {
              win.webContents.send('ssh:fleetConfigChanged', { filePath })
            }
          }
        } catch (watchErr) {
          // Log parse error but don't crash the watcher
          console.error('[fleet-watch] Error re-importing fleet config:', watchErr)
        }
      })

      existingFleetWatchers.set(filePath, watcher)
      return { ok: true }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      return { ok: false, error: message }
    }
  })

  ipcMain.handle('ssh:unwatchFleetConfig', (_event, { filePath }: { filePath: string }) => {
    existingFleetWatchers.get(filePath)?.close()
    existingFleetWatchers.delete(filePath)
    return { ok: true }
  })
```

### Step 3: Add watcher registry (module-level, outside handler registration function)

```typescript
// Module-level registry to manage fs.watch instances
const existingFleetWatchers = new Map<string, fs.FSWatcher>()
```

---

## Notes for AI

- `sshConnectionStore` — use the same reference already used in other handlers in this file
- `scheduleRuntimeGraphSync()` — import từ runtime module (check existing imports in file)
- `BrowserWindow` — should already be imported in Electron main process IPC files

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep "ipc/ssh" | head -20
```

Test flow manually:
1. Create `test-fleet.yaml` với 1 server
2. Call `ssh:importFleetConfig` với path tới file
3. Call `ssh:listTargets` — verify new target appears

---

## Done criteria

- [x] `ipcMain.handle('ssh:importFleetConfig', ...)` registered
- [x] `ipcMain.handle('ssh:exportFleetConfig', ...)` registered
- [x] `ipcMain.handle('ssh:watchFleetConfig', ...)` registered
- [x] `ipcMain.handle('ssh:unwatchFleetConfig', ...)` registered
- [x] Watcher registry prevents duplicate watchers for same path
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — 4 fleet IPC handlers added to `src/main/ipc/ssh.ts`. `fleetConfigWatchers` Map prevents duplicate `FSWatcher` instances. Channels added to `SSH_IPC_CHANNELS` for cleanup on re-register.
