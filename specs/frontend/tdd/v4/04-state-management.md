# TDD-FE-04: State Management

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/store/`

---

## 1. Zustand Store Architecture

```typescript
// src/renderer/src/store/index.ts

export const useAppStore = create<AppStore>()(
  devtools(
    immer((...a) => ({
      ...createRepoSlice(...a),
      ...createWorktreeSlice(...a),
      ...createTerminalSlice(...a),
      ...createTabSlice(...a),
      ...createUiSlice(...a),
      ...createSshSlice(...a),
      ...createDevServerSlice(...a),   // v3.0
      ...createOnboardingSlice(...a),  // v3.0
      ...createPreflightSlice(...a),   // v3.0
      ...createAuthSlice(...a),        // v4.0 NEW
      // ... 40+ more slices
    }))
  )
)
```

---

## 2. AuthSlice (NEW v4.0)

```typescript
// src/renderer/src/store/slices/auth.ts

type AuthSliceState = {
  auth: {
    user:   AuthUser | null
    status: 'loading' | 'authenticated' | 'unauthenticated' | 'error'
    error?: string
  }
}

type AuthSliceActions = {
  setAuth(user: AuthUser): void
  clearAuth(): void
  checkSession(): Promise<void>
}

export type AuthSlice = AuthSliceState & AuthSliceActions
```

**Selectors:**
```typescript
const user     = useAppStore(s => s.auth.user)
const isAuth   = useAppStore(s => s.auth.status === 'authenticated')
const authRole = useAppStore(s => s.auth.user?.role)
```

---

## 3. DevServerSlice (v3.0)

```typescript
// src/renderer/src/store/slices/dev-servers.ts

type DevServerSliceState = {
  devServers: {
    items:   DevServer[]
    loading: boolean
    error:   string | null
  }
}

type DevServerSliceActions = {
  setDevServers(items: DevServer[]): void
  updateDevServerStatus(id: string, status: DevServerStatus, error?: string): void
  addDevServer(ds: DevServer): void
  removeDevServer(id: string): void
}
```

**IPC event binding:**
```typescript
// In App.tsx/runtime layer:
ipc.on('devServer:added',         (id) => store.addDevServer(...))
ipc.on('devServer:removed',       (id) => store.removeDevServer(id))
ipc.on('devServer:statusChanged', (id, status, err) => store.updateDevServerStatus(id, status, err))
```

---

## 4. SSH Slice Extensions (v4.0)

```typescript
// src/renderer/src/store/slices/ssh.ts — EXTENDED

// Existing: connections, known hosts, etc.

// NEW v4.0:
type SshUserAccount = {
  linuxUsername: string   // toLinuxUsername(email, userId)
  provisioned:   boolean
  provisionedAt: number | null
  error:         string | null
}

type ProvisioningStatus = {
  serverId: string
  step:     number         // 0-4 (steps: detect, create user, deploy relay, authorize key, verify)
  total:    number         // always 4
  message:  string
  done:     boolean
  error:    string | null
}

// Store extension:
sshUserAccounts: Map<string, SshUserAccount>   // key: serverId
provisioningStatus: Map<string, ProvisioningStatus>

setSshUserAccount(serverId: string, account: SshUserAccount): void
setProvisioningStatus(serverId: string, status: ProvisioningStatus): void
```

---

## 5. Onboarding Slice Extensions (v3.0)

```typescript
type OnboardingSlice = {
  agentDetectionByServer: Map<string, AgentDetectionResult>  // 60s TTL
  setAgentDetection(serverId: string, result: AgentDetectionResult): void
}
```

---

## 6. Preflight Slice Extensions (v3.0)

```typescript
type PreflightSlice = {
  remotePreflightByServer: Map<string, PreflightResult>
  setRemotePreflight(serverId: string, result: PreflightResult): void
}
```

---

## 7. Sync Graph (Backend → Frontend)

```typescript
// src/renderer/src/runtime/sync-runtime-graph.ts

// Periodic sync từ backend state vào Zustand
// (Electron IPC hoặc WebSocket JSON-RPC)

scheduleRuntimeGraphSync() → {
  // Sync repos, worktrees, terminals, tabs, agents, devServers, etc.
  // Interval: 1s (debounced)
}
```

---

## 8. Selectors

```typescript
// src/renderer/src/store/selectors.ts

// Derived state (không store trực tiếp)
selectActiveWorktree(state): Worktree | null
selectTabsByWorktree(state, worktreeId): Tab[]
selectDevServerById(state, id): DevServer | undefined
selectConnectedDevServers(state): DevServer[]

// Project-related
selectProjectHostSetup(state, worktreeId): ProjectHostSetup
```
