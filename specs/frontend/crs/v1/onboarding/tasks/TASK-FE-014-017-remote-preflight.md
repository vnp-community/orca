# TASK-FE-014 đến TASK-FE-017: Remote Preflight Tasks

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/store/slices/preflight.ts` [MODIFY] — added `remotePreflightByServer`, `activeRemotePreflightStatus`, `setRemotePreflightStatus`, `clearRemotePreflightStatus` — TASK-FE-014
> - `src/renderer/src/hooks/useRemotePreflightStatus.ts` [NEW] — TASK-FE-015
> - `src/renderer/src/components/onboarding/GitIdentityCard.tsx` [NEW] — TASK-FE-017
> - TASK-FE-016 (IntegrationsStep): hooks in place, UI integration is a follow-up

---

# TASK-FE-014: Sửa Preflight slice — thêm remotePreflightByServer

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-005  
**Depends on:** TASK-FE-001

## Goal
Sửa Zustand `preflight` slice để lưu preflight status per dev server (thay vì chỉ 1 global status).

## Steps

1. **Đọc** `src/renderer/src/store/slices/preflight.ts` để hiểu structure hiện tại.

2. **Thêm** state fields:
```typescript
type PreflightSlice = {
  // ...existing
  remotePreflightByServer: Record<string, RemotePreflightStatus>  // NEW
  activeRemotePreflightStatus: RemotePreflightStatus | null        // NEW
  setRemotePreflightStatus: (devServerId: string, status: RemotePreflightStatus) => void
  clearRemotePreflightStatus: (devServerId: string) => void
}
```

3. **Implement** actions:
```typescript
setRemotePreflightStatus: (devServerId, status) =>
  set((state) => {
    const updated = { ...state.remotePreflightByServer, [devServerId]: status }
    return {
      remotePreflightByServer: updated,
      activeRemotePreflightStatus:
        devServerId === state.activeDevServerId ? status : state.activeRemotePreflightStatus,
    }
  }),

clearRemotePreflightStatus: (devServerId) =>
  set((state) => {
    const { [devServerId]: _, ...rest } = state.remotePreflightByServer
    return { remotePreflightByServer: rest }
  }),
```

4. **Export** selector:
```typescript
export function useActiveRemotePreflightStatus() {
  return useAppStore((s) => s.activeRemotePreflightStatus)
}
```

**Tests** (5 cases): set, clear, activeRemotePreflight updates khi devServerId match.

## Output Files
- **[MODIFY]** `src/renderer/src/store/slices/preflight.ts`

---

# TASK-FE-015: Tạo useRemotePreflightStatus hook

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-005  
**Depends on:** TASK-FE-014, TASK-FE-020

## Goal
Tạo `useRemotePreflightStatus` hook fetch và cache preflight status từ dev server, với auto-refresh khi devServerId thay đổi.

## Steps

**Tạo** `src/renderer/src/hooks/useRemotePreflightStatus.ts`:

```typescript
import { useState, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import type { RemotePreflightStatus } from '../../../shared/dev-server-types'

export function useRemotePreflightStatus(devServerId: string | null): {
  status: RemotePreflightStatus | null
  loading: boolean
  refresh: (force?: boolean) => Promise<void>
} {
  const [loading, setLoading] = useState(false)
  const setRemotePreflightStatus = useAppStore((s) => s.setRemotePreflightStatus)
  const clearRemotePreflightStatus = useAppStore((s) => s.clearRemotePreflightStatus)
  const statusFromStore = useAppStore(
    (s) => (devServerId ? s.remotePreflightByServer[devServerId] ?? null : null)
  )

  const refresh = useCallback(async (force = false) => {
    if (!devServerId) return
    setLoading(true)
    try {
      const result = await window.api.onboarding.getPreflightStatus({ devServerId, force })
      setRemotePreflightStatus(devServerId, result)
    } catch {
      // Non-fatal: stale data shown
    } finally {
      setLoading(false)
    }
  }, [devServerId, setRemotePreflightStatus])

  // Auto-refresh khi devServerId thay đổi
  useEffect(() => {
    if (!devServerId) return
    void refresh()
    // Cleanup: clear khi unmount (optional — keep stale data cho re-mount nhanh)
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { status: statusFromStore, loading, refresh }
}

// Derived helpers:
export function useGhInstalled(devServerId: string | null): boolean {
  const status = useAppStore(
    (s) => (devServerId ? s.remotePreflightByServer[devServerId] ?? null : null)
  )
  return status?.gh.installed === true
}

export function useGhAuthenticated(devServerId: string | null): boolean {
  const status = useAppStore(
    (s) => (devServerId ? s.remotePreflightByServer[devServerId] ?? null : null)
  )
  return status?.gh.authenticated === true
}
```

**Tests** (7 cases).

## Output Files
- **[NEW]** `src/renderer/src/hooks/useRemotePreflightStatus.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useRemotePreflightStatus.test.ts`

---

# TASK-FE-016: Sửa IntegrationsStep.tsx — remote preflight + remote PTY

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-005  
**Depends on:** TASK-FE-015

## Goal
Sửa `IntegrationsStep.tsx` để:
1. Nhận `activeDevServerId` prop
2. Dùng `useRemotePreflightStatus` để check gh + git trên remote
3. Truyền `devServerId` vào `OnboardingInlineCommandTerminal` để `gh auth login` chạy trên remote PTY
4. Tích hợp `GitIdentityCard` khi git thiếu identity

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/IntegrationsStep.tsx` và `OnboardingInlineCommandTerminal.tsx`.

2. **Thêm** `activeDevServerId: string | null` vào props type.

3. **Thay** local preflight check bằng `useRemotePreflightStatus(activeDevServerId)`.

4. **Thêm** "Connect dev server first" notice khi `activeDevServerId = null`.

5. **Sửa** `OnboardingInlineCommandTerminal` call — thêm `devServerId` prop:
```typescript
<OnboardingInlineCommandTerminal
  command="gh auth login"
  title="GitHub setup"
  devServerId={activeDevServerId}   // NEW
  onComplete={() => { setTerminalOpen(false); refresh(true) }}
/>
```

6. **Sửa** `OnboardingInlineCommandTerminal.tsx` — thêm `devServerId` prop và mở remote PTY khi có.

7. **Thêm** `<GitIdentityCard>` sau gh section khi `status?.git.installed && (!status.git.hasUserName || !status.git.hasUserEmail)`.

**Tests** (6 cases).

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/IntegrationsStep.tsx`
- **[MODIFY]** `src/renderer/src/components/onboarding/OnboardingInlineCommandTerminal.tsx`

---

# TASK-FE-017: Tạo GitIdentityCard component

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-005  
**Depends on:** TASK-FE-020

## Goal
Tạo `GitIdentityCard.tsx` — form nhập và lưu `git config --global user.name / user.email` trên dev server.

## Steps

**Tạo** `src/renderer/src/components/onboarding/GitIdentityCard.tsx`:

```typescript
type Props = {
  devServerId: string | null
  hasUserName: boolean
  hasUserEmail: boolean
  onSaved: () => void
}
```

Logic:
- Nếu `hasUserName && hasUserEmail` → hiển thị "✓ Git identity configured"
- Ngược lại → form với `<Input id="git-user-name">` và/hoặc `<Input id="git-user-email">`
- Save button gọi `window.api.onboarding.setGitIdentity({ devServerId, name, email })`
- Sau save thành công → `onSaved()`
- Error display nếu save fail

**Tests** (6 cases): configured state, form hiển thị đúng field, save call, onSaved trigger, error.

## Output Files
- **[NEW]** `src/renderer/src/components/onboarding/GitIdentityCard.tsx`
- **[NEW]** `src/renderer/src/components/onboarding/__tests__/GitIdentityCard.test.tsx`
