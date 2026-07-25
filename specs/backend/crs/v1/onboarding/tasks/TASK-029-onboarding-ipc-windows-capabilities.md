# TASK-029: Sửa `src/main/ipc/onboarding-ipc.ts` — Thêm `detectWindowsCapabilities`

**Phase:** 3 — Windows Terminal  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §A.2  
**Depends on:** TASK-022, TASK-028  
**Blocks:** TASK-041

---

## Mục tiêu

Thêm IPC handler `onboarding.detectWindowsCapabilities` với platform guard (chỉ cho Windows servers) và cache TTL 60s.

---

## File cần sửa

**Path:** `src/main/ipc/onboarding-ipc.ts`

---

## Thay đổi cần thực hiện

```typescript
// Cache Windows capabilities per dev server
const windowsCapsCache = new Map<string, {
  result: WindowsTerminalCapabilities
  cachedAt: number
}>()

// Trong registerOnboardingIpcHandlers():
ipc.handle('onboarding.detectWindowsCapabilities', async (_, params: {
  devServerId: string
}): Promise<WindowsTerminalCapabilities> => {
  const devServer = devServerManager.get(params.devServerId)
  if (!devServer) throw new Error('Dev server not found')
  if (devServer.platform !== 'win32') {
    throw new Error(
      `Dev server ${params.devServerId} is not Windows (platform: ${devServer.platform})`
    )
  }

  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  // Cache check TTL 60s
  const cacheKey = `win-caps-${params.devServerId}`
  const cached = windowsCapsCache.get(cacheKey)
  if (cached && Date.now() - cached.cachedAt < 60_000) return cached.result

  const result = await relay.session!.call('preflight.detectWindowsTerminalCapabilities', {})
  windowsCapsCache.set(cacheKey, { result, cachedAt: Date.now() })
  return result
})
```

> `WindowsTerminalCapabilities` — tìm type trong codebase hoặc khai báo inline:
> `{ wslAvailable, wslDistros, pwshAvailable, pwshVersion?, gitBashAvailable, gitBashPath? }`

---

## Acceptance Criteria

- [x] Handler chỉ cho phép dev server `platform === 'win32'`
- [x] Non-Windows dev server → throw Error với message rõ platform
- [x] Dev server không tồn tại → throw 'not found'
- [x] Relay không connected → throw 'not connected'
- [x] Cache hit (< 60s) → không gọi relay
- [x] Cache miss → gọi relay, lưu cache
- [x] `pwshVersion` và `gitBashPath` có trong response
- [x] TypeScript compile thành công
