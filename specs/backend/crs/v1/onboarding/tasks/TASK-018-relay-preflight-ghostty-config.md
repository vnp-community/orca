# TASK-018: Sửa `src/relay/preflight-handler.ts` — Thêm `detectGhosttyConfig`

**Phase:** 2 — Platform Wizard  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §A.3  
**Depends on:** (không — relay-side)  
**Blocks:** TASK-022

---

## Mục tiêu

Thêm method `detectGhosttyConfig()` vào `preflight-handler.ts` và đăng ký nó trên dispatcher để IPC handler có thể forward relay call.

---

## File cần sửa

**Path:** `src/relay/preflight-handler.ts`

---

## Thay đổi cần thực hiện

```typescript
import { homedir } from 'node:os'
import { join } from 'node:path'
import { existsSync } from 'node:fs'

// Trong class PreflightHandler:

private async detectGhosttyConfig(): Promise<{
  configPath: string | null
  themeDir: string | null
}> {
  const home = homedir()
  const ghosttyConfigPath = join(home, '.config', 'ghostty', 'config')
  const ghosttyThemeDir = join(home, '.config', 'ghostty', 'themes')
  return {
    configPath: existsSync(ghosttyConfigPath) ? ghosttyConfigPath : null,
    themeDir: existsSync(ghosttyThemeDir) ? ghosttyThemeDir : null
  }
}

// Trong constructor hoặc register method:
this.dispatcher.onRequest('preflight.detectGhosttyConfig', () => this.detectGhosttyConfig())
```

---

## Acceptance Criteria

- [x] `preflight.detectGhosttyConfig` được đăng ký trên dispatcher
- [x] `configPath` = đường dẫn đúng nếu file tồn tại, `null` nếu không
- [x] `themeDir` = đường dẫn đúng nếu thư mục tồn tại, `null` nếu không
- [x] Method chạy đúng trên cả macOS và Linux (homedir-relative paths)
- [x] TypeScript compile thành công
