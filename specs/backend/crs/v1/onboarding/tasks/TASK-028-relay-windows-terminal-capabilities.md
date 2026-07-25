# TASK-028: Sửa `src/relay/preflight-handler.ts` — Thêm `pwshVersion` + `gitBashPath`

**Phase:** 3 — Windows Terminal  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §A.1  
**Depends on:** TASK-020 (cùng file)  
**Blocks:** TASK-029

---

## Mục tiêu

Mở rộng `detectWindowsTerminalCapabilities()` để thêm `pwshVersion` và `gitBashPath` vào response.

---

## File cần sửa

**Path:** `src/relay/preflight-handler.ts`

---

## Thay đổi cần thực hiện

```typescript
import { stat } from 'node:fs/promises'

// Mở rộng return type:
private async detectWindowsTerminalCapabilities(): Promise<{
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string        // NEW
  gitBashAvailable: boolean
  gitBashPath?: string        // NEW
}> {
  const [wslResult, pwshResult, gitBashResult] = await Promise.all([
    this.checkWsl(),          // existing
    this.checkPwsh(),         // MODIFY: thêm version
    this.checkGitBash()       // NEW
  ])
  return { ...wslResult, ...pwshResult, ...gitBashResult }
}

// MODIFY checkPwsh():
private async checkPwsh(): Promise<{ pwshAvailable: boolean; pwshVersion?: string }> {
  try {
    const { stdout } = await execFileAsync('pwsh', ['--version'])
    return { pwshAvailable: true, pwshVersion: stdout.trim() }
  } catch {
    return { pwshAvailable: false }
  }
}

// NEW checkGitBash():
private async checkGitBash(): Promise<{ gitBashAvailable: boolean; gitBashPath?: string }> {
  const candidates = [
    'C:\\Program Files\\Git\\bin\\bash.exe',
    'C:\\Program Files (x86)\\Git\\bin\\bash.exe'
  ]
  for (const candidate of candidates) {
    try {
      await stat(candidate)
      return { gitBashAvailable: true, gitBashPath: candidate }
    } catch { /* continue */ }
  }
  return { gitBashAvailable: false }
}
```

---

## Acceptance Criteria

- [x] `detectWindowsTerminalCapabilities` response có `pwshVersion` và `gitBashPath`
- [x] `pwshVersion` có giá trị khi pwsh available, `undefined` khi không
- [x] `gitBashPath` có đường dẫn khi Git Bash found, `undefined` khi không
- [x] Logic `wslAvailable` và `wslDistros` hiện có không bị thay đổi
- [x] TypeScript compile thành công
