# TASK-AT-001: Decouple AutomationService từ Electron dependency

**Priority:** 🔴 HIGH — Server mode crash vì import Electron APIs  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AT-001, BUG-BE-AT-001  
**Solution ref:** [SOLUTION-automation.md](../solutions/SOLUTION-automation.md)

## Bước 1 — Tìm Electron import trong AutomationService

```bash
grep -rn "electron\|ipcMain\|app\." src/main/automation/ 2>/dev/null | head -20
```

## Bước 2 — Thay thế Electron API bằng Node.js equivalents

Thay `ipcMain.handle()` bằng EventEmitter pattern:

```typescript
// TRƯỚC (Electron-dependent):
import { ipcMain } from 'electron'
ipcMain.handle('automation:run', (event, payload) => { ... })

// SAU (Node.js only):
import { EventEmitter } from 'node:events'
export class AutomationService extends EventEmitter {
  // Wire handlers qua event bus thay vì ipcMain
  on('automation:run', (payload) => { ... })
}
```

## Bước 3 — Verify server mode build

```bash
pnpm tsc --noEmit
# Check không còn electron imports:
grep -rn "from 'electron'" src/main/automation/ 2>/dev/null
# Expected: no results
```
