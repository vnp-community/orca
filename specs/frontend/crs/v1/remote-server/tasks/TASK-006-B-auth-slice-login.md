# TASK-006-B — Tạo AuthSlice + OrcaLoginScreen (Phase 2 RBAC)

**Task ID:** TASK-006-B  
**CR:** CR-006 — Team-based Access Control  
**Solution Ref:** SOL-CR-006, Section 3  
**Dependencies:** TASK-001-A, TASK-002-A  
**Estimated:** 5–7 giờ  
**Priority:** Phase 2 (Long-term — cần backend thay đổi trước)  
**Status:** ✅ DONE

---

## Mục tiêu

Implement RBAC thật sự:
1. `AuthSlice` — user identity + auth state trong Zustand
2. `OrcaLoginScreen` — login page với SSO options
3. SSH target filtering theo current user's teams/projects
4. `UserProfileBadge` — user info trong titlebar

---

## Files cần tạo

| File | Action |
|------|--------|
| `src/renderer/src/store/slices/auth.ts` | CREATE |
| `src/renderer/src/web/OrcaLoginScreen.tsx` | CREATE |
| `src/renderer/src/components/activity/UserProfileBadge.tsx` | CREATE |
| `src/renderer/src/store/selectors.ts` | MODIFY — thêm selectSshTargetsForCurrentUser |

---

## Bước 1: Tạo AuthSlice

```typescript
// src/renderer/src/store/slices/auth.ts
import type { StateCreator } from 'zustand'
import type { AppState } from '@/store/types'

export type OrcaUserRole = 'developer' | 'lead' | 'admin'

export type OrcaUser = {
  id: string
  email: string
  name: string
  avatarUrl?: string
  teams: string[]
  projects: string[]
  role: OrcaUserRole
}

export type AuthStatus =
  | 'unauthenticated'
  | 'authenticating'
  | 'authenticated'
  | 'error'

export type AuthSlice = {
  currentUser: OrcaUser | null
  authStatus: AuthStatus
  authError: string | null

  setCurrentUser: (user: OrcaUser | null) => void
  setAuthStatus: (status: AuthStatus, error?: string) => void
  clearAuth: () => void
}

export const createAuthSlice: StateCreator<AppState, [], [], AuthSlice> = (
  set
) => ({
  currentUser: null,
  authStatus: 'unauthenticated',
  authError: null,

  setCurrentUser: (user) =>
    set((s) => { s.currentUser = user }),

  setAuthStatus: (status, error) =>
    set((s) => {
      s.authStatus = status
      s.authError = error ?? null
    }),

  clearAuth: () =>
    set((s) => {
      s.currentUser = null
      s.authStatus = 'unauthenticated'
      s.authError = null
    }),
})
```

## Bước 2: Đăng ký AuthSlice vào AppState và store

```typescript
// store/types.ts: & AuthSlice
// store/index.ts: ...createAuthSlice(...a),
```

## Bước 3: Thêm selectSshTargetsForCurrentUser selector

```typescript
// store/selectors.ts — thêm vào cuối:

// Selector: Filter SSH targets theo current user's permissions
export function selectSshTargetsForCurrentUser(state: AppState): SshTarget[] {
  const user = state.currentUser
  const all = state.sshTargets ?? []

  // Nếu không có auth hoặc là admin → thấy tất cả
  if (!user || user.role === 'admin') return all

  return all.filter((target) => {
    // Nếu target thuộc project không phải của user → ẩn
    if (target.project && !user.projects.includes(target.project)) return false
    // Nếu target thuộc team không phải của user → ẩn
    if (target.team && !user.teams.includes(target.team)) return false
    return true
  })
}
```

## Bước 4: Cập nhật SshTargetGroupedList để dùng selector

```typescript
// Trong SshTargetGroupedList.tsx, thay:
// const sshTargets = useAppStore((s) => s.sshTargets ?? [])
// Bằng:
const sshTargets = useAppStore(selectSshTargetsForCurrentUser)
```

## Bước 5: Tạo OrcaLoginScreen.tsx

```typescript
// src/renderer/src/web/OrcaLoginScreen.tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { translate } from '@/i18n/i18n'

// SSO Provider type (from server config)
type SsoProvider = {
  id: string
  label: string
  iconUrl?: string
}

type OrcaServerAuthConfig = {
  requiresAuth: boolean
  orgName?: string
  ssoProviders?: SsoProvider[]
}

export function OrcaLoginScreen({
  serverConfig,
  onUsePairingCode,
}: {
  serverConfig: OrcaServerAuthConfig
  onUsePairingCode: () => void
}) {
  const [isLoading, setIsLoading] = useState(false)

  const handleSsoLogin = async (provider: SsoProvider) => {
    setIsLoading(true)
    try {
      // Redirect to SSO provider — handled by backend
      await window.api.auth?.startSsoFlow?.({ providerId: provider.id })
    } catch (err) {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-[380px] space-y-6">
        {/* Header */}
        <div className="space-y-2 text-center">
          <h1 className="text-2xl font-bold">
            {translate('login.title', 'Sign in to Orca')}
          </h1>
          {serverConfig.orgName && (
            <p className="text-sm text-muted-foreground">
              {serverConfig.orgName}
            </p>
          )}
        </div>

        {/* SSO buttons */}
        {serverConfig.ssoProviders && serverConfig.ssoProviders.length > 0 && (
          <div className="space-y-2">
            {serverConfig.ssoProviders.map((provider) => (
              <Button
                key={provider.id}
                variant="outline"
                className="w-full gap-2"
                disabled={isLoading}
                onClick={() => handleSsoLogin(provider)}
              >
                {provider.iconUrl && (
                  <img
                    src={provider.iconUrl}
                    alt=""
                    className="h-4 w-4 object-contain"
                  />
                )}
                {translate(
                  'login.continueWith',
                  `Continue with ${provider.label}`
                )}
              </Button>
            ))}

            <Separator className="my-4" />
          </div>
        )}

        {/* Pairing code fallback */}
        <Button
          variant="ghost"
          className="w-full text-sm text-muted-foreground"
          onClick={onUsePairingCode}
        >
          {translate('login.usePairingCode', 'Use pairing code instead')}
        </Button>
      </div>
    </div>
  )
}
```

## Bước 6: Tạo UserProfileBadge.tsx

```typescript
// src/renderer/src/components/activity/UserProfileBadge.tsx
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'

export function UserProfileBadge() {
  const user = useAppStore((s) => s.currentUser)
  if (!user) return null

  const initials = user.name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 gap-1.5 px-2 text-xs"
        >
          <Avatar className="h-5 w-5">
            <AvatarImage src={user.avatarUrl} alt={user.name} />
            <AvatarFallback className="text-[10px]">{initials}</AvatarFallback>
          </Avatar>
          <span className="max-w-[80px] truncate">{user.name}</span>
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-[200px]">
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col space-y-0.5">
            <p className="text-sm font-medium">{user.name}</p>
            <p className="text-xs text-muted-foreground">{user.email}</p>
          </div>
        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        {/* Teams */}
        {user.teams.length > 0 && (
          <>
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              {translate('user.teams', 'Teams')}
            </DropdownMenuLabel>
            <div className="flex flex-wrap gap-1 px-2 pb-1">
              {user.teams.map((team) => (
                <Badge key={team} variant="secondary" className="text-xs">
                  {team}
                </Badge>
              ))}
            </div>
            <DropdownMenuSeparator />
          </>
        )}

        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => window.api.auth?.signOut?.()}
        >
          {translate('user.signOut', 'Sign out')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
```

## Bước 7: Auth IPC events

```typescript
// useIpcEvents.ts — thêm auth event handler:
const unsubAuth = window.api.auth?.onAuthStateChanged?.((event) => {
  const store = useAppStore.getState()
  
  if (event.type === 'authenticated') {
    store.setCurrentUser(event.user)
    store.setAuthStatus('authenticated')
    scheduleRuntimeGraphSync()
  } else if (event.type === 'unauthenticated') {
    store.clearAuth()
  } else if (event.type === 'error') {
    store.setAuthStatus('error', event.error)
  }
})

// Cleanup:
unsubAuth?.()
```

## Bước 8: Preload API cho auth

```typescript
// Thêm vào preload/index.ts:
auth: {
  getServerConfig: () => ipcRenderer.invoke('auth:getServerConfig'),
  startSsoFlow: (args: { providerId: string }) =>
    ipcRenderer.invoke('auth:startSsoFlow', args),
  signOut: () => ipcRenderer.invoke('auth:signOut'),
  onAuthStateChanged: (callback) => {
    const handler = (_: IpcRendererEvent, event: any) => callback(event)
    ipcRenderer.on('auth:stateChanged', handler)
    return () => ipcRenderer.removeListener('auth:stateChanged', handler)
  },
},
```

---

## Acceptance Criteria

**AuthSlice:**
- [x] `currentUser: OrcaUser | null` trong store
- [x] `authStatus` state cycle đúng
- [x] `clearAuth()` reset tất cả auth state

**SSH Target Filtering:**
- [x] Admin → thấy tất cả servers
- [x] Developer → chỉ thấy servers thuộc project/team của mình
- [x] Servers không có project/team → visible với tất cả users

**UI:**
- [x] `UserProfileBadge` hiển thị trong titlebar khi authenticated
- [x] Click → dropdown với name, email, teams, sign out
- [x] `OrcaLoginScreen` render đúng với SSO buttons

> [!NOTE]
> Phase 2 cần backend hỗ trợ auth endpoints (`auth:getServerConfig`, SSO flow, `auth:stateChanged` events). Frontend task này có thể implement trước và test với mock config.

---

## Implementation Notes

> **Completed:** 2026-07-23 | `store/slices/auth.ts`: currentUser OrcaUser|null, authStatus cycle, clearAuth() resets. `selectors.ts`: selectSshTargetsForCurrentUser (admin=all, developer=filter project/team, no project=visible to all). `web/OrcaLoginScreen.tsx`: SSO buttons per provider. `UserProfileBadge.tsx`: initials avatar, dropdown teams+sign-out. `useIpcEvents.ts`: auth event handler all cases + default:break. TypeScript: ✅ 0 errors.
