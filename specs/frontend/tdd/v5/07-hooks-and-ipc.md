# TDD-FE-07: Custom Hooks & IPC Events

**Document:** TDD-FE-07  
**Domain:** Custom Hooks — useIpcEvents, useComposerState, agent hooks  
**Source files:** `src/renderer/src/hooks/`

---

## 1. Tổng quan

`src/hooks/` là tầng **event processing** cho các IPC events từ backend:

```
IPC Events (Desktop) hoặc WebSocket Events (Web)
  ↓
useIpcEvents() — subscribe và route tất cả events
  ├─ → Zustand store updates
  ├─ → Toast notifications
  ├─ → scheduleRuntimeGraphSync()
  └─ → Các hook xử lý chuyên biệt
```

---

## 2. `useIpcEvents` (~140KB — lớn nhất trong hooks/)

```typescript
// src/renderer/src/hooks/useIpcEvents.ts

export function useIpcEvents(): void {
  const store = useAppStore()

  useEffect(() => {
    // Đăng ký TẤT CẢ IPC events:

    // PTY events
    window.api.pty.onData((event) => {
      dispatchPtyData(event.ptyId, event.data)
    })

    window.api.pty.onExit((event) => {
      store.markPtyExited(event.ptyId, event.exitCode)
    })

    // Filesystem watch events
    window.api.filesystem.onChange((event) => {
      handleFilesystemChange(event)
      scheduleRuntimeGraphSync()
    })

    // SSH events
    window.api.ssh.onConnectionStateChanged((event) => {
      store.setSshConnectionState(event.targetId, event.state)
      scheduleRuntimeGraphSync()
    })

    // Agent status events (từ terminal title parsing)
    window.api.onAgentStatusUpdate((event) => {
      store.updateAgentStatus(event)
    })

    // Notification events
    window.api.onNotification((event) => {
      handleNotification(event)
    })

    // Automation events
    window.api.onAutomationEvent((event) => {
      handleAutomationEvent(event)
    })

    // Runtime events (serve mode)
    window.api.onRuntimeEvent((event) => {
      handleRuntimeClientEvent(event)
    })

    // Workspace session events
    window.api.onWorkspaceSession((patch) => {
      store.applyWorkspaceSessionPatch(patch)
    })

    return () => {
      window.api.pty.offData(...)
      // Cleanup tất cả
    }
  }, [])
}
```

### isRemoteWorkspaceSnapshotApplyInProgress

```typescript
// Exported flag: ngăn concurrent sync trong khi snapshot apply đang chạy
export let isRemoteWorkspaceSnapshotApplyInProgress = false
```

---

## 3. `useComposerState` (~171KB — file hook lớn nhất)

```typescript
// src/renderer/src/hooks/useComposerState.ts
// Quản lý state của "New Workspace Composer" (create worktree flow)

// State machine phức tạp:
type ComposerState = {
  step: 'repo-select' | 'branch-select' | 'configure' | 'creating' | 'done' | 'error'
  selectedRepoId: string | null
  selectedBranch: string | null
  newBranchName: string | null
  baseBranch: string | null
  executionHostId: ExecutionHostId
  agentToLaunch: TuiAgent | null
  setupScript: SetupScriptState
  // ...
}

// Actions:
useComposerState() → {
  state,
  selectRepo,
  selectBranch,
  createNewBranch,
  confirmAndCreate,
  reset
}
```

---

## 4. `useAutomationDispatchEvents` (~21KB)

```typescript
// src/renderer/src/hooks/useAutomationDispatchEvents.ts
// Xử lý automation dispatch events

// Khi automation chạy, UI cần:
// 1. Show progress toast
// 2. Update automation run status trong store
// 3. Notify khi done (với link tới worktree)
// 4. Handle errors

export function useAutomationDispatchEvents(): void {
  useEffect(() => {
    const unsub = window.api.onAutomationEvent(event => {
      switch (event.type) {
        case 'run.started':
          showAutomationRunningToast(event.automationId)
          break
        case 'run.completed':
          showAutomationCompleteToast(event)
          useAppStore.getState().updateAutomationRun(event.runId, { status: 'success' })
          break
        case 'run.failed':
          showAutomationFailureToast(event)
          break
      }
    })
    return unsub
  }, [])
}
```

---

## 5. `useEditorExternalWatch` (~39KB)

```typescript
// src/renderer/src/hooks/useEditorExternalWatch.ts
// Watch files đang mở trong editor cho external changes

// Khi file thay đổi bên ngoài (git pull, terminal edit, etc.):
// 1. Detect change qua filesystem watch event
// 2. So sánh với editor nội dung hiện tại
// 3. Nếu no local changes → auto-reload
// 4. Nếu có local changes → show "file changed externally" banner
// 5. User chọn: reload (mất changes) hoặc keep (create conflict)
```

---

## 6. `useIpcTabSwitch` (trong hooks/)

```typescript
// src/renderer/src/hooks/ipc-tab-switch.ts
// Handle tab switch events từ IPC (ví dụ: click notification → switch tab)

// Events:
// 'focus-worktree'    → switch sang worktree + tab
// 'focus-terminal'    → activate specific terminal tab
// 'focus-editor-file' → open file in editor tab
// 'focus-pr'          → open PR review page
```

---

## 7. Agent Hook Notifications

```typescript
// src/renderer/src/hooks/agent-hook-completion-notifications.ts
// ~13KB — Hiện thông báo khi agent hook hoàn thành

// Agent hooks (từ orca.yaml):
// hooks:
//   session:start: [./setup.sh]
//   session:end: [./cleanup.sh]

// Khi hook complete:
// - Nếu success + agent done → "✓ Setup script finished" toast
// - Nếu failure → error dialog với output
// - Nếu timeout → warning toast

// Notification routing:
// hook event → check if worktree visible → toast hoặc OS notification
```

---

## 8. `useIssueMetadata` (~13KB)

```typescript
// src/renderer/src/hooks/useIssueMetadata.ts
// Fetch metadata cho GitHub/Linear/Jira issues liên kết với worktree

// Pattern: lazy fetch, deduplicated, cached
// - GitHub: fetch issue title, labels, assignee
// - Linear: fetch issue detail từ Linear API
// - Jira: fetch issue detail từ Jira API

// Sử dụng trong: TaskPage, WorktreeJumpPalette, TerminalPane header
```

---

## 9. `useInstalledAgentSkills` (~11KB)

```typescript
// src/renderer/src/hooks/useInstalledAgentSkills.ts
// Đọc .claude/commands/ và tương tự từ worktree

// Detect "skills" (custom Claude commands):
// - Scan .claude/commands/*.md trong worktree
// - Parse frontmatter: name, description, trigger words
// - Surface trong UI (QuickOpen, context menu)

// Skills types:
// - User-level: ~/.claude/commands/
// - Repo-level: <repo>/.claude/commands/
// - Both combined
```

---

## 10. `useSidebarResize` (~6KB)

```typescript
// src/renderer/src/hooks/useSidebarResize.ts
// Handle sidebar drag resize

// Left sidebar: drag right edge → resize leftSidebarWidth
// Right sidebar: drag left edge → resize rightSidebarWidth

// Constraints:
// - Min: 180px
// - Max: 50% viewport width
// - Snap to integer pixels
// - Save to store immediately (no debounce)
// - Persist via workspace session
```

---

## 11. `useVirtualizedScrollAnchor` (~11KB)

```typescript
// src/renderer/src/hooks/useVirtualizedScrollAnchor.ts
// Virtualized scrolling anchor management

// Problem: khi list thay đổi (GitHub PR file list, issue list)
// scroll position bị mất nếu list re-render
// Solution: anchor scroll tới một specific element
// Sau khi re-render, restore scroll tới anchor

// Dùng trong: PullRequestPage file list, TaskPage issue list
```

---

## 12. `useGlobalFileDrop` (~8KB)

```typescript
// src/renderer/src/hooks/useGlobalFileDrop.ts
// Handle drag-and-drop files vào app

// Drop targets:
// 1. Terminal pane → paste file path hoặc upload
// 2. Editor → open file
// 3. Composer → attach file

// File types:
// - Images → upload + paste path
// - Text files → open in editor
// - Archives → extract + open folder

// Terminal drop pipeline:
// terminal-drop-handler.ts → terminal-drop-path-writer.ts
// → writePty with escaped path
```

---

## 13. `useModalReturnFocus` (~7KB)

```typescript
// src/renderer/src/hooks/useModalReturnFocus.ts
// Sau khi dialog/modal đóng → return focus về đúng element

// Problem: modal trap focus, khi đóng focus về body (bad UX)
// Solution: track focused element trước khi modal open
// → restore focus sau khi modal close

// Works với:
// - Confirmation dialogs
// - Settings panel
// - GitHub/Linear item dialogs
// - Quick Open
```

---

## 14. `useAutoAckViewedAgent` (~13KB)

```typescript
// src/renderer/src/hooks/useAutoAckViewedAgent.ts
// Auto-acknowledge agent completion khi user đang xem

// Logic:
// 1. Agent status → 'completed' hoặc 'error'
// 2. Nếu worktree đang active (visible) → ack ngay
// 3. Nếu worktree ẩn → đợi user navigate to it → ack
// 4. Sau ack → clear notification badge

// Prevents "ghost" notifications cho tabs user đã seen
```

---

## 15. Key Hook Patterns

```typescript
// Pattern 1: Event subscription với cleanup
useEffect(() => {
  const unsub = window.api.onSomeEvent(handler)
  return unsub   // cleanup on unmount
}, [])

// Pattern 2: Store action + sync
const handleEvent = useCallback((event) => {
  useAppStore.getState().updateSomething(event.data)
  scheduleRuntimeGraphSync()
}, [])

// Pattern 3: Debounced refresh
const debouncedRefresh = useMemo(
  () => debounce(() => refreshGitStatus(), 500),
  []
)

// Pattern 4: Shallow store read
const { x, y } = useAppStore(useShallow(s => ({ x: s.x, y: s.y })))
```

---

## Addendum v3.0: New Hooks (onboarding + remote-server CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **TDD-FE-09/10:** [09-onboarding-devserver.md](./09-onboarding-devserver.md) | [10-fleet-management.md](./10-fleet-management.md)

### Phase 1 — Dev Server (OB-002, OB-003)

#### `useDevServers.ts`
```typescript
// Load devServers from backend on mount, subscribe to status changes
export function useDevServers(): {
  devServers: DevServer[]
  isLoading: boolean
  refetch: () => Promise<void>
}
// IPC: window.api.devServer.list()
// IPC events: window.api.devServer.onStatusChanged(cb) → cleanup: offStatusChanged
```

#### `useRemoteAgentDetection.ts`
```typescript
// Per-server module-level cache (Map), 60s TTL
// Survives unmount/remount — avoid re-detection on tab switch
const detectionCache = new Map<string, DetectionState>()

export function useRemoteAgentDetection(devServerId: string | null): DetectionState & {
  redetect: () => Promise<void>
}
// IPC: window.api.onboarding.detectAgents({ devServerId })
```

### Phase 2 — Preflight & Repo (OB-005, OB-006)

#### `useRemotePreflightStatus.ts`
```typescript
export function useRemotePreflightStatus(devServerId: string | null): {
  status: RemotePreflightStatus | null
  loading: boolean
  recheck: () => Promise<void>
}
// IPC: window.api.onboarding.runPreflight({ devServerId })
```

#### `useRemoteDirectoryBrowser.ts`
```typescript
export function useRemoteDirectoryBrowser(devServerId: string, rootPath?: string): {
  entries: RemoteDirEntry[]
  currentPath: string
  navigate: (path: string) => void
  goUp: () => void
  loading: boolean
  error: string | null
}
// IPC: window.api.repoRemote.listDir({ devServerId, path })
```

### Phase 3 — Windows & Notifications (OB-007, OB-008)

#### `useRemoteWindowsTerminalCapabilities.ts`
```typescript
const capsCache = new Map<string, { caps: RemoteWindowsCapabilities; cachedAt: number }>()
const CACHE_TTL = 60_000

export function useRemoteWindowsTerminalCapabilities(
  devServerId: string | null,
  enabled: boolean   // only fetch when user is on Windows step
): RemoteWindowsCapabilities & { retry: () => void }
// IPC: window.api.onboarding.detectWindowsCapabilities({ devServerId })
```

#### `useWebPushSubscription.ts`
```typescript
export function useWebPushSubscription(): PushSubscriptionState & {
  subscribe: () => Promise<void>   // GET /push/vapid-key → PushManager.subscribe()
  unsubscribe: () => Promise<void> // DELETE /push/subscribe
}
// Fetches VAPID key from server: GET /push/vapid-key
// Web Push API only — no IPC (direct HTTP)
```

#### `useBrowserNotificationPermission.ts`
```typescript
export function useBrowserNotificationPermission(): {
  permission: NotificationPermission  // 'default' | 'granted' | 'denied'
  isSupported: boolean
  requestPermission: () => Promise<NotificationPermission>
}
```

### Fleet Hooks (remote-server CRs)

| Hook | Key IPC / API | Cleanup |
|------|---------------|---------|
| `useFleetImport` | `ssh:fleet:import` + `ssh:fleet:onImportProgress` | `offImportProgress` |
| `useServerGroups` | `ssh:fleet:listByGroup` | n/a |
| `useBulkProvisioning` | `ssh:fleet:bulkProvision` + `onProvisionProgress` | `offProvisionProgress` |
| `useBootstrapAutomation` | `ssh:fleet:bootstrap` + `onBootstrapProgress` | `offBootstrapProgress` |
| `useFleetHealthPolling` | `ssh:fleet:status` | `clearInterval` + `stopPolling` |
| `useCurrentUser` | `ssh:fleet:currentUser` | n/a |

#### `useFleetHealthPolling.ts`
```typescript
// Polling interval: 30s default
// IPC event: window.api.ssh.fleet.onHealthUpdate(cb) → real-time alerts
export function useFleetHealthPolling(options?: {
  intervalMs?: number  // default 30_000
  autoStart?: boolean  // default true in web mode
}): {
  isPolling: boolean
  lastCheckedAt: number | null
  start: () => void
  stop: () => void
  checkNow: () => Promise<void>
}

// cleanup:
useEffect(() => {
  window.api.ssh.fleet.onHealthUpdate(handleUpdate)
  return () => { window.api.ssh.fleet.offHealthUpdate(handleUpdate) }
}, [])
```

### Hook Rules (v3.0 additions)

- **60s module cache**: `useRemoteAgentDetection`, `useRemoteWindowsTerminalCapabilities` — dùng module-level Map, không phải Zustand
- **Web Push = direct HTTP**: Không qua IPC — dùng `fetch('/push/vapid-key')`, `fetch('/push/subscribe', { method: 'POST' })`
- **Service Worker registration** trong `bootstrapWebApp()` không phải hook — only once at startup
- **Fleet polling** chỉ `autoStart = true` trong web mode (không áp dụng Electron — server-side fleet)
