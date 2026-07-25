# TASK-022: Sửa `src/main/ipc/onboarding-ipc.ts` — Preflight Handlers + setGitIdentity + Ghostty + PTY

**Phase:** 2 — Remote Preflight  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §A.3, §B.4, §B.5  
**Depends on:** TASK-014, TASK-017, TASK-018, TASK-019, TASK-020, TASK-021  
**Blocks:** TASK-027

---

## Mục tiêu

Thêm các IPC handlers Phase 2 vào `onboarding-ipc.ts`:
1. `onboarding.getPreflightStatus` — với cache TTL 30s
2. `onboarding.setGitIdentity` — invalidate cache sau khi set
3. `onboarding.detectGhosttyConfig` — forward đến relay
4. `onboarding.openGhAuthTerminal` — tạo remote PTY cho `gh auth login`

---

## File cần sửa

**Path:** `src/main/ipc/onboarding-ipc.ts`

---

## Thay đổi cần thực hiện

Trong hàm `registerOnboardingIpcHandlers()`, thêm các handlers:

```typescript
import type { RemotePreflightStatus } from '../../shared/dev-server-types'

// Cache preflight per dev server
const preflightCache = new Map<string, { result: RemotePreflightStatus; cachedAt: number }>()
const PREFLIGHT_CACHE_TTL_MS = 30_000

// === getPreflightStatus ===
ipc.handle('onboarding.getPreflightStatus', async (_, params: {
  devServerId: string
  force?: boolean
}): Promise<RemotePreflightStatus> => {
  const { devServerId, force = false } = params

  if (!force) {
    const cached = preflightCache.get(devServerId)
    if (cached && Date.now() - cached.cachedAt < PREFLIGHT_CACHE_TTL_MS) {
      return cached.result
    }
  }

  const relay = devServerManager.getRelay(devServerId)
  if (!relay) throw new Error(`Dev server ${devServerId} not connected`)

  const raw = await relay.session!.call('preflight.check', {})
  const result: RemotePreflightStatus = {
    devServerId,
    platform: raw.platform,
    checkedAt: Date.now(),
    gh: raw.gh,
    git: raw.git
  }
  preflightCache.set(devServerId, { result, cachedAt: Date.now() })
  return result
})

// === setGitIdentity ===
ipc.handle('onboarding.setGitIdentity', async (_, params: {
  devServerId: string
  name: string
  email: string
}): Promise<void> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error(`Dev server ${params.devServerId} not connected`)
  await relay.session!.call('preflight.setGitIdentity', {
    name: params.name,
    email: params.email
  })
  // Invalidate preflight cache
  preflightCache.delete(params.devServerId)
})

// === detectGhosttyConfig ===
ipc.handle('onboarding.detectGhosttyConfig', async (_, params: {
  devServerId: string
}): Promise<{ configPath: string | null; themeDir: string | null }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  return relay.session!.call('preflight.detectGhosttyConfig', {})
})

// === openGhAuthTerminal ===
ipc.handle('onboarding.openGhAuthTerminal', async (_, params: {
  devServerId: string
}): Promise<{ ptyId: string; devServerId: string }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  const ptyId = await relay.session!.createPty({
    command: 'gh',
    args: ['auth', 'login'],
    env: {},
    cols: 120,
    rows: 30
  })
  return { ptyId, devServerId: params.devServerId }
})
```

---

## Acceptance Criteria

- [x] `onboarding.getPreflightStatus`: cache miss → call relay, lưu cache
- [x] `onboarding.getPreflightStatus`: cache hit (<30s) → không gọi relay
- [x] `onboarding.getPreflightStatus`: `force: true` → bypass cache
- [x] `onboarding.setGitIdentity`: gọi `preflight.setGitIdentity` trên relay
- [x] `onboarding.setGitIdentity`: invalidate preflight cache sau khi set
- [x] `onboarding.detectGhosttyConfig`: forward đến relay
- [x] `onboarding.openGhAuthTerminal`: tạo PTY với `gh auth login`
- [x] Tất cả handlers throw Error khi relay không connected
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. `relay.session!.createPty()` — kiểm tra xem `SshRelaySession` có method này chưa, nếu chưa có thì dùng method tương đương trong codebase
2. Có thể cần thêm `createPty()` method vào `DevServerRelayBridge` thay vì gọi trực tiếp `session`
