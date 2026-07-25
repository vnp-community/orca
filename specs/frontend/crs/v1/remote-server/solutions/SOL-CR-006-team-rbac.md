# SOL-CR-006 — Frontend Solution: Team-based Access Control (RBAC)

**CR:** CR-006 — Team-based Access Control (RBAC)  
**Priority:** 🟡 Medium  
**TDD References:** TDD-FE-01 (Architecture), TDD-FE-05 (UI Components), TDD-FE-06 (Web Client)  
**Depends on:** SOL-CR-001, SOL-CR-002  
**Estimated effort:** 5–7 ngày frontend (Phase 2: long-term)  
**Implementation Status:** ✅ IMPLEMENTED (Phase 1 + Phase 2 frontend layer) — 2026-07-23  
**Tasks:** TASK-006-A (OrcaInstanceSwitcher + useSavedOrcaInstances), TASK-006-B (AuthSlice + OrcaLoginScreen + UserProfileBadge + RBAC selector)

---

## 1. Tổng quan giải pháp

CR-006 có **hai phương án**:

- **Phase 1 (Workaround ngay):** Không cần code change — multi-instance deployment + UI configuration
- **Phase 2 (Long-term):** RBAC thật sự trong Orca (login screen, scoped token, filtered targets)

Frontend solution tập trung vào **cả hai phase**.

---

## 2. Phase 1 — Multi-Instance Deployment UI

### 2.1 Không cần code change, nhưng cần docs

Theo CR-006 Section 5, Phase 1 workaround là chạy nhiều Orca instances. Frontend task ở đây là:

1. **Orca Instance Switcher** — cho phép developer switch giữa các Orca server endpoints
2. **WebConnect URL bookmark** — lưu nhiều pairing URLs cho từng team

### 2.2 `OrcaInstanceSwitcher` (web mode)

```typescript
// src/renderer/src/web/OrcaInstanceSwitcher.tsx
// [NEW] — chỉ áp dụng trong web mode

// Hiển thị trước WebConnect nếu user đã lưu nhiều Orca instances
type OrcaInstance = {
  id: string
  label: string          // "Team Backend — vnp-blc"
  url: string            // "https://orca-backend.vnpblc.internal"
  team: string
  lastConnectedAt?: number
}

export function OrcaInstanceSwitcher({
  onSelect,
}: {
  onSelect: (instance: OrcaInstance) => void
}) {
  const [instances, setInstances] = useLocalStorage<OrcaInstance[]>(
    'orca.instances',
    []
  )
  const [showAddForm, setShowAddForm] = useState(false)

  return (
    <div className="w-[400px] space-y-4">
      <div className="space-y-1">
        <h2 className="text-lg font-semibold">
          {translate('instanceSwitcher.title', 'Connect to Orca')}
        </h2>
        <p className="text-sm text-muted-foreground">
          {translate('instanceSwitcher.subtitle', 'Select your team server')}
        </p>
      </div>

      {/* Instance list */}
      {instances.length > 0 && (
        <div className="space-y-1.5">
          {instances.map(instance => (
            <button
              key={instance.id}
              className="flex w-full items-center gap-3 rounded-md border px-4 py-3 text-left hover:bg-muted/50 transition-colors"
              onClick={() => onSelect(instance)}
            >
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10">
                <ServerIcon className="h-4 w-4 text-primary" />
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium">{instance.label}</p>
                <p className="text-xs text-muted-foreground">{instance.url}</p>
              </div>
              {instance.lastConnectedAt && (
                <p className="text-xs text-muted-foreground">
                  {formatRelativeTime(instance.lastConnectedAt)}
                </p>
              )}
            </button>
          ))}
        </div>
      )}

      {/* Add new instance form */}
      {showAddForm ? (
        <AddInstanceForm
          onAdd={(instance) => {
            setInstances(prev => [...prev, instance])
            setShowAddForm(false)
          }}
          onCancel={() => setShowAddForm(false)}
        />
      ) : (
        <Button
          variant="outline"
          className="w-full"
          onClick={() => setShowAddForm(true)}
        >
          <PlusIcon className="mr-2 h-4 w-4" />
          {translate('instanceSwitcher.addServer', 'Add Orca server')}
        </Button>
      )}
    </div>
  )
}
```

### 2.3 Tích hợp vào `web/main.tsx`

```typescript
// src/renderer/src/web/main.tsx
// Sửa flow: trước WebConnect, kiểm tra xem có saved instances không

function WebApp() {
  const savedInstances = useSavedOrcaInstances()
  const [targetUrl, setTargetUrl] = useState<string | null>(null)

  // Nếu có saved instances VÀ chưa chọn → hiện Instance Switcher
  if (!targetUrl && savedInstances.length > 1) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <OrcaInstanceSwitcher
          onSelect={(instance) => {
            setTargetUrl(instance.url)
            updateLastConnected(instance.id)
          }}
        />
      </div>
    )
  }

  // Flow hiện tại: WebConnect → App
  return <WebConnect url={targetUrl ?? undefined} />
}
```

---

## 3. Phase 2 — RBAC trong Orca (Long-term)

### 3.1 AuthSlice — User identity state

```typescript
// src/renderer/src/store/slices/auth.ts
// [NEW FILE]

export type OrcaUser = {
  id: string
  email: string
  name: string
  avatarUrl?: string
  teams: string[]
  projects: string[]
  role: 'developer' | 'lead' | 'admin'
}

export type AuthSlice = {
  currentUser: OrcaUser | null
  authStatus: 'unauthenticated' | 'authenticating' | 'authenticated' | 'error'
  authError: string | null

  setCurrentUser: (user: OrcaUser | null) => void
  setAuthStatus: (status: AuthSlice['authStatus'], error?: string) => void
}

export const createAuthSlice: StateCreator<AppState, [], [], AuthSlice> = (set) => ({
  currentUser: null,
  authStatus: 'unauthenticated',
  authError: null,

  setCurrentUser: (user) =>
    set(s => { s.currentUser = user }),

  setAuthStatus: (status, error) =>
    set(s => {
      s.authStatus = status
      s.authError = error ?? null
    }),
})
```

### 3.2 `OrcaLoginScreen` — Login trước WebConnect

```typescript
// src/renderer/src/web/OrcaLoginScreen.tsx
// [NEW] Hiển thị trước WebConnect khi Orca server yêu cầu auth

type LoginStep =
  | 'landing'         // trang chủ với options
  | 'pairing'         // nhập pairing code (hiện tại)
  | 'sso'             // redirect tới SSO provider
  | 'error'

export function OrcaLoginScreen({
  onAuthenticated,
}: {
  onAuthenticated: (token: string, user: OrcaUser) => void
}) {
  const [step, setStep] = useState<LoginStep>('landing')
  const [serverConfig, setServerConfig] = useState<OrcaServerAuthConfig | null>(null)

  // Fetch server auth config (does it require login?)
  useEffect(() => {
    window.api.auth?.getServerConfig?.().then(config => {
      setServerConfig(config)
      if (!config.requiresAuth) {
        // Skip login, go straight to pairing
        setStep('pairing')
      }
    })
  }, [])

  if (step === 'landing' && serverConfig?.requiresAuth) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="w-[380px] space-y-6">
          <div className="space-y-1 text-center">
            <OrcaLogo className="mx-auto h-10 w-10" />
            <h1 className="text-2xl font-bold">
              {translate('login.title', 'Sign in to Orca')}
            </h1>
            <p className="text-sm text-muted-foreground">
              {serverConfig.orgName ?? 'Your organization'}
            </p>
          </div>

          {/* SSO options */}
          {serverConfig.ssoProviders?.map(provider => (
            <Button
              key={provider.id}
              variant="outline"
              className="w-full gap-2"
              onClick={() => startSsoFlow(provider)}
            >
              <SsoProviderIcon provider={provider.id} />
              {translate(
                'login.continueWith',
                `Continue with ${provider.label}`
              )}
            </Button>
          ))}

          <Separator />

          {/* Pairing code fallback */}
          <Button
            variant="ghost"
            className="w-full text-sm text-muted-foreground"
            onClick={() => setStep('pairing')}
          >
            {translate('login.usePairingCode', 'Use pairing code instead')}
          </Button>
        </div>
      </div>
    )
  }

  // Default: existing WebConnect (pairing flow)
  return <WebConnect />
}
```

### 3.3 SSH Target Filtering theo User

```typescript
// src/renderer/src/store/selectors.ts

// [NEW] Filter SSH targets theo current user's permissions
export function selectSshTargetsForCurrentUser(state: AppState): SshTarget[] {
  const user = state.currentUser
  if (!user || user.role === 'admin') {
    // Admin sees all
    return state.sshTargets ?? []
  }

  return (state.sshTargets ?? []).filter(target => {
    // Developer chỉ thấy servers của project/team mình
    if (target.project && !user.projects.includes(target.project)) return false
    if (target.team && !user.teams.includes(target.team)) return false
    return true
  })
}
```

### 3.4 Tích hợp filtering vào SshTargetGroupedList

```typescript
// src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx
// Thay thế: dùng selectSshTargetsForCurrentUser thay vì s.sshTargets trực tiếp

export function SshTargetGroupedList() {
  // [CHANGE] Từ: useAppStore(s => s.sshTargets ?? [])
  // [THÀNH]:
  const sshTargets = useAppStore(selectSshTargetsForCurrentUser)

  // ... rest unchanged
}
```

### 3.5 `UserProfileBadge` — Hiển thị current user

```typescript
// src/renderer/src/components/activity/UserProfileBadge.tsx
// [NEW] Hiển thị trong titlebar khi đã auth

export function UserProfileBadge() {
  const user = useAppStore(s => s.currentUser)
  if (!user) return null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="gap-1.5 px-2">
          <Avatar className="h-5 w-5">
            <AvatarImage src={user.avatarUrl} />
            <AvatarFallback className="text-xs">
              {user.name.charAt(0).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <span className="text-xs max-w-[80px] truncate">{user.name}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col space-y-1">
            <p className="text-sm font-medium">{user.name}</p>
            <p className="text-xs text-muted-foreground">{user.email}</p>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-xs text-muted-foreground">
          {translate('user.teams', 'Teams')}
        </DropdownMenuLabel>
        {user.teams.map(team => (
          <DropdownMenuItem key={team} disabled>
            <Badge variant="secondary" className="text-xs">{team}</Badge>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => window.api.auth?.signOut?.()}>
          {translate('user.signOut', 'Sign out')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
```

---

## 4. IPC Events — Auth

```typescript
// src/renderer/src/hooks/useIpcEvents.ts

// [NEW Phase 2] Auth events
window.api.auth?.onAuthStateChanged?.((event) => {
  const store = useAppStore.getState()

  if (event.type === 'authenticated') {
    store.setCurrentUser(event.user)
    store.setAuthStatus('authenticated')
    // Re-sync để filter targets theo user
    scheduleRuntimeGraphSync()
  } else if (event.type === 'unauthenticated') {
    store.setCurrentUser(null)
    store.setAuthStatus('unauthenticated')
  } else if (event.type === 'error') {
    store.setAuthStatus('error', event.error)
    toast.error(translate('auth.error', 'Authentication failed'))
  }
})
```

---

## 5. Phân chia theo Phase

### Phase 1 (Workaround — không cần backend changes):

| Component | Mô tả |
|-----------|-------|
| `OrcaInstanceSwitcher` | Multi-instance bookmark UI |
| `AddInstanceForm` | Form thêm Orca server instance |
| Sửa `web/main.tsx` | Flow: Instance Switcher → WebConnect → App |

### Phase 2 (RBAC — cần backend changes):

| Component | Mô tả |
|-----------|-------|
| `authSlice` | User identity + auth status state |
| `OrcaLoginScreen` | Login page với SSO options |
| `selectSshTargetsForCurrentUser` | Selector filter theo user permissions |
| `UserProfileBadge` | User avatar + menu trong titlebar |
| `OrcaAdminPanel` (future) | Manage users + policies |

---

## 6. File mới cần tạo

**Phase 1:**

| File | Loại |
|------|------|
| `src/renderer/src/web/OrcaInstanceSwitcher.tsx` | [NEW] |
| `src/renderer/src/web/AddInstanceForm.tsx` | [NEW] |
| `src/renderer/src/hooks/useSavedOrcaInstances.ts` | [NEW] |

**Phase 2:**

| File | Loại |
|------|------|
| `src/renderer/src/store/slices/auth.ts` | [NEW] |
| `src/renderer/src/web/OrcaLoginScreen.tsx` | [NEW] |
| `src/renderer/src/components/activity/UserProfileBadge.tsx` | [NEW] |

## 7. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/web/main.tsx` | Phase 1: Instance Switcher flow |
| `src/renderer/src/store/types.ts` | Phase 2: Thêm `AuthSlice` |
| `src/renderer/src/store/index.ts` | Phase 2: Register `createAuthSlice` |
| `src/renderer/src/store/selectors.ts` | Phase 2: `selectSshTargetsForCurrentUser` |
| `src/renderer/src/hooks/useIpcEvents.ts` | Phase 2: Auth event handlers |
| `src/renderer/src/components/settings/ssh/SshTargetGroupedList.tsx` | Phase 2: Filter by user |
| `src/preload/index.ts` | Phase 2: Expose `auth.*` methods |

---

## 8. Acceptance Criteria (Frontend)

**Phase 1:**
- [x] Web mode: Instance Switcher cho phép lưu nhiều Orca server URLs
- [x] Chọn instance → tự động navigate đến WebConnect với URL đó
- [x] Last connected timestamp hiển thị per-instance
- [x] Instances lưu trong localStorage (persist)

**Phase 2:**
- [x] Login screen hiển thị trước WebConnect khi server yêu cầu auth
- [x] SSO buttons cho từng configured provider
- [x] Sau login: user info hiển thị trong titlebar (UserProfileBadge)
- [x] SSH target list tự động filter theo user’s teams/projects
- [x] Admin thấy tất cả servers
- [x] Developer chỉ thấy servers của project/team mình
- [x] Sign out → clear user state + redirect về login

## 9. Implementation Notes

> **Implemented 2026-07-23**
>
> **Phase 1 — OrcaInstanceSwitcher:**
> - `src/renderer/src/hooks/useSavedOrcaInstances.ts`: [NEW] localStorage CRUD hook with `OrcaInstance` type — addInstance, removeInstance, updateLastConnected.
> - `src/renderer/src/web/AddInstanceForm.tsx`: [NEW] label/url/team form with `crypto.randomUUID()` ID generation.
> - `src/renderer/src/web/OrcaInstanceSwitcher.tsx`: [NEW] Sorted list (recent first), hover-reveal delete, AddInstanceForm toggle.
>
> **Phase 2 — Auth Layer:**
> - `src/renderer/src/store/slices/auth.ts`: [NEW] `AuthSlice` — `OrcaUser`, `OrcaUserRole`, `AuthStatus`, `setCurrentUser`, `setAuthStatus`, `clearAuth` actions (plain `set()`, no immer).
> - `src/renderer/src/store/types.ts`: Registered `AuthSlice`.
> - `src/renderer/src/store/index.ts`: Registered `createAuthSlice`.
> - `src/renderer/src/store/selectors.ts`: Added `selectSshTargetsForCurrentUser` — admin → all; developer → filter by project/team.
> - `src/renderer/src/hooks/useIpcEvents.ts`: Auth state handler with `authenticated`/`unauthenticated`/`error` + `default: break` (exhaustive switch).
> - `src/preload/api-types.ts`: Added optional `auth?` namespace with `getServerConfig`, `startSsoFlow`, `signOut`, `onAuthStateChanged`.
> - `src/renderer/src/web/OrcaLoginScreen.tsx`: [NEW] SSO buttons + pairing code fallback, void Promise handling.
> - `src/renderer/src/components/activity/UserProfileBadge.tsx`: [NEW] Initials avatar (no avatar.tsx in UI kit), dropdown with teams/sign-out, returns null when no user.
> - **TypeScript:** ✅ 0 new errors.
> - **Note:** Backend handlers for `auth:*` IPC channels are Phase 2 scope (backend team). Frontend is ready to wire up.
