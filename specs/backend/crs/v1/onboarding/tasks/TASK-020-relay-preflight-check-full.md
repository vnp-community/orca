# TASK-020: Sửa `src/relay/preflight-handler.ts` — Thêm `checkFullPreflight` (gh + git)

**Phase:** 2 — Remote Preflight  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §B.2  
**Depends on:** TASK-018 (cùng file, thứ tự thêm methods)  
**Blocks:** TASK-022

---

## Mục tiêu

Thêm `checkFullPreflight()`, `checkGhCli()`, `checkGitCli()` vào relay preflight handler và đăng ký `preflight.check` trên dispatcher.

---

## File cần sửa

**Path:** `src/relay/preflight-handler.ts`

---

## Thay đổi cần thực hiện

```typescript
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)

// Trong class PreflightHandler:

private async checkFullPreflight(): Promise<{
  platform: NodeJS.Platform
  gh: { installed: boolean; authenticated: boolean; version?: string }
  git: { installed: boolean; version?: string; hasUserName: boolean; hasUserEmail: boolean }
}> {
  const [ghResult, gitResult] = await Promise.all([
    this.checkGhCli(),
    this.checkGitCli()
  ])
  return {
    platform: process.platform,
    gh: ghResult,
    git: gitResult
  }
}

private async checkGhCli(): Promise<{
  installed: boolean
  authenticated: boolean
  version?: string
}> {
  try {
    const { stdout: version } = await execFileAsync('gh', ['--version'])
    try {
      await execFileAsync('gh', ['auth', 'status'])
      return { installed: true, authenticated: true, version: version.trim() }
    } catch {
      return { installed: true, authenticated: false, version: version.trim() }
    }
  } catch {
    return { installed: false, authenticated: false }
  }
}

private async checkGitCli(): Promise<{
  installed: boolean
  version?: string
  hasUserName: boolean
  hasUserEmail: boolean
}> {
  try {
    const { stdout: version } = await execFileAsync('git', ['--version'])
    const [nameResult, emailResult] = await Promise.allSettled([
      execFileAsync('git', ['config', '--global', 'user.name']),
      execFileAsync('git', ['config', '--global', 'user.email'])
    ])
    return {
      installed: true,
      version: version.trim(),
      hasUserName: nameResult.status === 'fulfilled' && nameResult.value.stdout.trim() !== '',
      hasUserEmail: emailResult.status === 'fulfilled' && emailResult.value.stdout.trim() !== ''
    }
  } catch {
    return { installed: false, hasUserName: false, hasUserEmail: false }
  }
}

// Register:
this.dispatcher.onRequest('preflight.check', () => this.checkFullPreflight())
```

---

## Acceptance Criteria

- [x] `preflight.check` được đăng ký trên dispatcher
- [x] `checkGhCli()`: gh cài và auth → `{ installed: true, authenticated: true, version: ... }`
- [x] `checkGhCli()`: gh cài nhưng chưa auth → `{ installed: true, authenticated: false }`
- [x] `checkGhCli()`: gh không cài → `{ installed: false, authenticated: false }`
- [x] `checkGitCli()`: git cài và có identity → `{ installed: true, hasUserName: true, hasUserEmail: true }`
- [x] `checkGitCli()`: git không cài → `{ installed: false }`
- [x] `checkFullPreflight()` trả về đúng `platform`
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

`execFileAsync` có thể đã được khai báo trong file. Nếu đã có, không khai báo lại.
