# SOLUTION: Terminal Management (TM-002) — Fix workspace init relative path

**Domain:** terminal-management (folder riêng, không có dấu chấm)  
**TDD Reference:** TDD-07 (Runtime Service)  
**Files cần thay đổi:** `src/main/runtime/orca-runtime.ts`  
**Tổng số bugs:** 1 (TM-002)

---

## BUG-TM-002 — Fix workspace init relative path

**Mức độ:** 🟡 MEDIUM  
**Root cause:** `OrcaRuntime.initWorkspace()` dùng relative path → fails khi CWD thay đổi.

### Fix

```typescript
// src/main/runtime/orca-runtime.ts

import { resolve, isAbsolute } from 'node:path'
import { existsSync, mkdirSync } from 'node:fs'

export class OrcaRuntime {
  async initWorkspace(config: WorkspaceConfig): Promise<void> {
    // FIX TM-002: Luôn resolve sang absolute path
    const absoluteRoot = isAbsolute(config.rootPath)
      ? config.rootPath
      : resolve(process.cwd(), config.rootPath)

    if (!existsSync(absoluteRoot)) {
      throw new Error(`Workspace root does not exist: ${absoluteRoot}`)
    }

    // Data dir: .orca trong workspace root
    const absoluteDataPath = resolve(absoluteRoot, '.orca')
    mkdirSync(absoluteDataPath, { recursive: true })

    this.workspaceRoot     = absoluteRoot
    this.workspaceDataPath = absoluteDataPath

    this.log.info(`[Runtime] Workspace: ${absoluteRoot}`)
  }
}
```

> **Note:** Solution chi tiết hơn xem tại [SOLUTION-terminal-management.md trong terminal-management.](../terminal-management./solutions/SOLUTION-terminal-management.md)

---

## Tóm tắt

| File | Action |
|------|--------|
| `src/main/runtime/orca-runtime.ts` | Use `resolve()` for absolute path conversion |

## Verification

```bash
# Test relative path:
# 1. Start server from /tmp → call initWorkspace({ rootPath: '../../home/user/project' })
# 2. Verify absolute path /home/user/project used (not relative from /tmp)
pnpm vitest run src/main/runtime/__tests__/orca-runtime.test.ts
```
