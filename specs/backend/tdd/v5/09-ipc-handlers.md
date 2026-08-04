# TDD-09: IPC Handlers

**Document:** TDD-09  
**Domain:** Electron IPC — Filesystem, PTY, SSH, Worktrees, Settings  
**Source files:** `src/main/ipc/`  

---

## 1. IPC Architecture

Electron IPC là **kênh giao tiếp** giữa Main Process và Renderer Process (UI):

```
Renderer Process (React UI)
  │
  │ ipcRenderer.invoke('channel:method', params)  ← request
  │ ipcRenderer.on('channel:event', handler)       ← subscribe
  │
  │ (Electron IPC bridge qua contextBridge)
  ▼
Main Process
  │
  │ ipcMain.handle('channel:method', handler)      ← request handler
  │ webContents.send('channel:event', payload)     ← push event
  ▼
Services (Store, DaemonPtyAdapter, SshConnectionManager, ...)
```

**Khác với RPC Server:**
- IPC: Electron Desktop App ↔ Main Process (in-process, fast)
- RPC: External clients (Browser/Mobile/CLI) ↔ Main Process (network)

---

## 2. Filesystem Handlers (`ipc/filesystem.ts`) — ~78K

Module lớn nhất trong IPC layer.

### 2.1 File operations

```typescript
ipcMain.handle('filesystem:readFile', async (_, path: string) => {
  // Hỗ trợ: local và remote SSH path
  // Path format: /absolute/path (local) hoặc ssh://<targetId>/path
  return readFile(path)
})

ipcMain.handle('filesystem:writeFile', async (_, path: string, data: Uint8Array) => {
  return writeFile(path, data)
})

ipcMain.handle('filesystem:stat', async (_, path: string) => {
  return stat(path)    // { size, isDirectory, mtime, ... }
})

ipcMain.handle('filesystem:move', async (_, from: string, to: string) => {
  return rename(from, to)
})

ipcMain.handle('filesystem:delete', async (_, path: string, opts?: { recursive?: boolean }) => {
  return rm(path, opts)
})
```

### 2.2 Directory listing

```typescript
ipcMain.handle('filesystem:listDir', async (_, path: string, opts?: ListOptions) => {
  // opts.gitFallback: dùng git ls-files nếu thư mục quá lớn
  // opts.depth: max recursion depth
  // opts.respectGitignore: filter .gitignore files
  return listFiles(path, opts)
})
```

### 2.3 Search

```typescript
ipcMain.handle('filesystem:search', async (_, args: SearchArgs) => {
  // Dùng ripgrep (rg) cho text search
  // Fallback: git grep
  // SSH: forward search tới relay
  const { query, path, includeGlob, excludeGlob, maxResults } = args
  return searchFiles({ query, path, ... })
})
```

### 2.4 File watcher

```typescript
ipcMain.handle('filesystem:watch', async (_, path: string) => {
  // Parcel-watcher trong subprocess
  const watcher = await watchDirectory(path)
  return watcher.id
})

ipcMain.handle('filesystem:unwatch', async (_, watcherId: string) => {
  return stopWatcher(watcherId)
})

// Event push tới renderer:
// webContents.send('filesystem:change', { watcherId, events: WatchEvent[] })
```

---

## 3. PTY Handlers (`ipc/pty.ts`) — ~223K (largest file!)

```typescript
// Tạo PTY session
ipcMain.handle('pty:create', async (_, args: PtyCreateArgs) => {
  const { shell, cwd, env, cols, rows, worktreeId } = args
  const ptyId = await localPtyProvider.createPty({ shell, cwd, env, cols, rows })
  return { ptyId }
})

// Gửi input
ipcMain.on('pty:write', (_, ptyId: string, data: Uint8Array) => {
  localPtyProvider.writePty(ptyId, data)
})

// Resize terminal
ipcMain.handle('pty:resize', async (_, ptyId: string, cols: number, rows: number) => {
  return localPtyProvider.resizePty(ptyId, cols, rows)
})

// Kill terminal
ipcMain.handle('pty:kill', async (_, ptyId: string, signal?: string) => {
  return localPtyProvider.killPty(ptyId, signal)
})

// Subscribe output
ipcMain.on('pty:subscribe', (event, ptyId: string) => {
  const webContents = event.sender
  localPtyProvider.onPtyData(ptyId, (data) => {
    webContents.send('pty:data', { ptyId, data })
  })
})

// Unsubscribe
ipcMain.on('pty:unsubscribe', (_, ptyId: string) => {
  localPtyProvider.offPtyData(ptyId)
})
```

### IPC PTY flow

```
Renderer: ipcRenderer.invoke('pty:create', { shell: '/bin/bash', cwd: '~/project', cols: 120, rows: 40 })
  → Main: createPty → DaemonPtyAdapter.createPty() → Daemon: node-pty spawn
  → returns ptyId

Renderer: ipcRenderer.send('pty:subscribe', ptyId)
  → Main: register listener on DaemonPtyAdapter
  → Daemon: onData → Main: webContents.send('pty:data', { ptyId, data })
  → Renderer: ipcRenderer.on('pty:data', render terminal output)
```

---

## 4. SSH Handlers (`ipc/ssh.ts`) — ~48K

```typescript
// List SSH targets
ipcMain.handle('ssh:listTargets', (_, options?: ListTargetOptions) => {
  return store.getSshTargets()
    .filter(t => !isRuntimeOwnedSshTarget(t))
    .map(t => sanitize(t))   // không expose identity files raw
})

// Add target
ipcMain.handle('ssh:addTarget', async (_, target: Omit<SshTarget, 'id'>) => {
  const added = sshConnectionStore.addTarget(target)
  return { target: added, repoReadoptions: sshConnectionStore.lastRepoReadoptions }
})

// Remove target
ipcMain.handle('ssh:removeTarget', async (_, id: string) => {
  sshConnectionStore.removeTarget(id)
  // Cleanup connections
  await sshConnectionManager.disconnect(id)
})

// Connect
ipcMain.handle('ssh:connect', async (_, targetId: string) => {
  // Tạo SshConnection → deploy relay → open session
  const session = await sshConnectionManager.connect(targetId)
  return { status: 'connected', targetId }
})

// Connection status
ipcMain.handle('ssh:getConnectionState', async (_, targetId: string) => {
  return sshConnectionManager.getConnectionState(targetId)
  // Returns: SshConnectionState { status, error, remotePlatform, ... }
})

// Import từ ~/.ssh/config
ipcMain.handle('ssh:importFromConfig', async (_, options?: { reAdopt?: boolean }) => {
  const imported = sshConnectionStore.importFromSshConfig(options)
  return { targets: imported }
})

// Port forwards
ipcMain.handle('ssh:listPortForwards', async (_, targetId: string) => {
  return sshPortForwardManager.list(targetId)
})

ipcMain.handle('ssh:addPortForward', async (_, args: AddPortForwardArgs) => {
  return sshPortForwardManager.add(args)
})
```

---

## 5. Worktree Handlers (`ipc/worktrees.ts`) — ~88K

```typescript
// Detect worktrees từ disk (git worktree list)
ipcMain.handle('worktrees:detect', async (_, repoId: string) => {
  return runtime.detectWorktrees(repoId)
})

// Create worktree
ipcMain.handle('worktrees:create', async (_, args: CreateWorktreeArgs) => {
  return runtime.createWorktree(args)
})

// Delete worktree
ipcMain.handle('worktrees:delete', async (_, args: DeleteWorktreeArgs) => {
  return runtime.deleteWorktree(args)
})

// Update metadata
ipcMain.handle('worktrees:update', async (_, id: string, updates: Partial<WorktreeMeta>) => {
  return store.updateWorktree(id, updates)
})

// Clone repo
ipcMain.handle('worktrees:cloneRepo', async (_, args: CloneRepoArgs) => {
  return runtime.cloneRepo(args)
})
```

---

## 6. Repo Handlers (`ipc/repos.ts`) — ~96K

```typescript
ipcMain.handle('repos:list', () => store.getRepos())
ipcMain.handle('repos:create', async (_, args) => runtime.createRepo(args))
ipcMain.handle('repos:update', async (_, id, updates) => store.updateRepo(id, updates))
ipcMain.handle('repos:delete', async (_, id) => runtime.deleteRepo(id))

// GitHub/GitLab integration:
ipcMain.handle('repos:fetchGithubRemote', async (_, repoId) => {
  // Lấy PR list, issue list từ GitHub API
  // Token từ AiVault (github token)
})
```

### 6.1 Addendum v5.0: `hostKind` cho Repo trên Dev Server — IMPLEMENTED ✅ (2026-08-02/03)

`addRemoteRepoFromPath` (internal helper trong `ipc/repos.ts`, dùng bởi cả `'projectHostSetups:setupExistingFolder'` và các entry point khác) có thêm param `hostKind?: 'ssh' | 'devServer'` (default `'ssh'`):

```typescript
async function addRemoteRepoFromPath(
  store: Store,
  args: {
    connectionId: string           // SSH targetId hoặc Dev Server id — cùng provider-registry key
    hostKind?: 'ssh' | 'devServer' // NEW — quyết định persist connectionId hay devServerId
    remotePath: string
    displayName?: string
    kind?: 'git' | 'folder'
  }
): Promise<{ repo: Repo; alreadyExisted: boolean } | { error: string }>
```

Handler `'projectHostSetups:setupExistingFolder'` branch theo `parseExecutionHostId(args.hostId)?.kind`:

```typescript
const parsedHost = parseExecutionHostId(args.hostId)
// parsedHost.kind === 'ssh'       → addRemoteRepoFromPath({ hostKind: 'ssh', connectionId: parsedHost.targetId, ... })
// parsedHost.kind === 'devServer' → addRemoteRepoFromPath({ hostKind: 'devServer', connectionId: parsedHost.devServerId, ... }) // NEW
// parsedHost.kind === 'runtime'   → error: phải dùng runtime projectHostSetup RPC
```

**Clone repo mới lên Dev Server — chưa hỗ trợ.** `cloneRemoteRepo` (dùng bởi `repos:cloneRemote`) chỉ hoạt động với `IGitProvider.getHostPlatform()` — method này optional trên interface và `DevServerGitProvider` không implement (chưa có host-platform detection / remote home-path resolution / SSH-multiplexer progress-notify tương đương ở Dev Server). Gọi clone với một Dev Server connectionId sẽ throw thay vì âm thầm clone lên local filesystem của chính Orca server. Xem [TDD-13 §11.3](./13-dev-server-onboarding.md#11-provider-unification-with-ssh-registries-v50).

---

## 7. Settings Handlers (`ipc/settings.ts`) — ~9K

```typescript
ipcMain.handle('settings:getGlobal', () => store.getGlobalSettings())
ipcMain.handle('settings:updateGlobal', async (_, updates) => {
  return store.updateGlobalSettings(updates)
})
ipcMain.handle('settings:getNotifications', () => store.getNotificationSettings())
ipcMain.handle('settings:updateNotifications', async (_, updates) => {
  return store.updateNotificationSettings(updates)
})
```

---

## 8. Notifications (`ipc/notifications.ts`) — ~31K

```typescript
// Orca notification system qua OS native notifications
ipcMain.handle('notifications:send', async (_, opts: NotificationOptions) => {
  const notif = new Notification({
    title: opts.title,
    body: opts.body,
    silent: opts.silent ?? false
  })
  notif.on('click', () => {
    // Focus worktree/window liên quan
  })
  notif.show()
})

// macOS: check authorization status
ipcMain.handle('notifications:getAuthorizationStatus', () => {
  return getNotificationAuthorizationStatus()
})
```

---

## 9. GitHub/GitLab Integration (`ipc/github.ts`, `ipc/gitlab.ts`)

```typescript
// GitHub: ~37K
ipcMain.handle('github:listPRs', async (_, args) => {
  // GitHub REST API v3 / GraphQL
  // Token từ store.globalSettings.githubToken
  return fetchGithubPRs(args)
})

ipcMain.handle('github:createPR', async (_, args) => {
  return createGithubPR(args)
})

// Linear: ~17K
ipcMain.handle('linear:listIssues', ...)
ipcMain.handle('linear:createIssue', ...)

// Jira: ~8K
ipcMain.handle('jira:listIssues', ...)
ipcMain.handle('jira:createIssue', ...)
```

---

## 10. Speech Handler (`ipc/speech.ts`) — ~7K

```typescript
// Transcription qua sherpa-onnx (native addon)
// Model: whisper-tiny (bundled trong app)
ipcMain.handle('speech:transcribe', async (_, audioData: Float32Array) => {
  return sherpaOnnxTranscribe(audioData)
})

ipcMain.handle('speech:listModels', () => {
  return getAvailableSpeechModels()
})
```

---

## 11. Mobile Handler (`ipc/mobile.ts`) — ~8K

```typescript
// Mobile app pairing và communication
ipcMain.handle('mobile:getPairingUrl', async () => {
  // Generate QR code
  const pairingUrl = rpcServer.getPairingUrl()
  const qrCode = await QRCode.toDataURL(pairingUrl)
  return { pairingUrl, qrCode }
})

ipcMain.handle('mobile:listDevices', () => {
  return deviceRegistry.listDevices()
})

ipcMain.handle('mobile:disconnectDevice', async (_, deviceId: string) => {
  return deviceRegistry.disconnect(deviceId)
})
```

---

## 12. Preload Bridge (`src/preload/index.ts`)

```typescript
// contextBridge expose IPC ra renderer:
contextBridge.exposeInMainWorld('electronAPI', {
  // Filesystem
  readFile: (path) => ipcRenderer.invoke('filesystem:readFile', path),
  writeFile: (path, data) => ipcRenderer.invoke('filesystem:writeFile', path, data),

  // PTY
  createPty: (args) => ipcRenderer.invoke('pty:create', args),
  writePty: (ptyId, data) => ipcRenderer.send('pty:write', ptyId, data),
  onPtyData: (callback) => ipcRenderer.on('pty:data', (_, data) => callback(data)),

  // SSH
  listSshTargets: () => ipcRenderer.invoke('ssh:listTargets'),
  connectSsh: (targetId) => ipcRenderer.invoke('ssh:connect', targetId),

  // ... etc
})
```

---

## 13. Addendum v2.0: IPC trong Web Server Mode (restructure_v1) — IMPLEMENTED ✅

> **Date:** 2026-07-23

### Web IPC vs Electron IPC

```
Electron mode:
  Renderer → ipcRenderer.invoke('filesystem:readFile', path)
           → preload bridge → ipcMain.handle() → Node.js handler
           → response via IPC

Server mode (Web SPA):
  Browser → WebSocket { type: 'invoke', channel: 'filesystem:readFile', args: [path] }
          → WebIpcBridge.handleWebSocketMessage()
          → NodeIpcBridge.invoke('filesystem:readFile', windowId, path)
          → ipcMain.handle() handler (same handler code!)
          → { type: 'result', result: <file content> }
```

### Tái sử dụng handlers

Các IPC handler `src/main/ipc/` **không thay đổi** — chúng hoạt động với cả 2 mode:

```typescript
// src/main/ipc/filesystem.ts:
import { ipcMain } from 'electron'  // ← alias → NodeIpcBridge trong server mode!

ipcMain.handle('filesystem:readFile', async (event, path) => {
  return fs.readFile(path, 'utf-8')
})
// Hoạt động trong Electron mode và Server mode
```

### Server-side Event Push (push message)

```typescript
// Electron mode: webContents.send('pty:data', data)
// Server mode: webIpcBridge.pushToClients('pty:data', [data], broadcast)

// Trong NodeWindow.send():
window.send('pty:data', data)
// → tất cả onSend subscribers được notify
// → WebSocket: { type: 'push', channel: 'pty:data', args: [data] }
```

### mocks/electron.ts — Deprecated

```typescript
// src/main/mocks/electron.ts
/**
 * @deprecated Use src/platform/stubs/electron-node-wrapper.ts for server mode.
 * This file is kept for legacy Electron-mode testing and compatibility only.
 */
```

Server mode **không dùng** `mocks/electron.ts` nữa. Thay bằng `electron-node-wrapper.ts` qua vite alias.

### Tham khảo

- [TDD-10: Platform Layer](./10-platform-layer.md) — NodeIpcBridge, WebIpcBridge
- `src/platform/adapters/node/ipc.ts` — NodeIpcBridge
- `src/platform/adapters/node/web-ipc-bridge.ts` — WebIpcBridge

---

## 14. Addendum v3.0: Dev Server & Onboarding IPC (onboarding CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **TDD-13:** [13-dev-server-onboarding.md](./13-dev-server-onboarding.md)

### 3 Handler Groups mới (đăng ký trong server-bootstrap.ts)

```typescript
// src/main/ipc/dev-server-ipc.ts
registerDevServerIpcHandlers(devServerManager, store)
// Handlers:
// devServer.list, devServer.add, devServer.update, devServer.remove
// devServer.connect, devServer.disconnect, devServer.testConnection

// src/main/ipc/onboarding-ipc.ts
registerOnboardingIpcHandlers(devServerManager, store)
// Handlers:
// onboarding.detectAgents     → DevServerManager.detectAgents() → relay.session.call()
// onboarding.runPreflight     → relay.runPreflight()
// onboarding.detectWindowsCapabilities → cache 60s per devServerId
// onboarding.cloneRepo        → relay.exec(git clone)
// onboarding.openFolder       → create worktree

// src/main/ipc/repo-remote-ipc.ts
registerRepoRemoteIpcHandlers(devServerManager, store)
// Handlers:
// repoRemote.list, repoRemote.clone, repoRemote.syncStatus
```

### Call Flow: Agent Detection

```
Browser → WebSocket { type: 'invoke', channel: 'onboarding.detectAgents', args: [{ devServerId }] }
  → WebIpcBridge → NodeIpcBridge.invoke()
  → onboarding-ipc.ts handler
  → DevServerManager.getRelay(devServerId)
  → relay.session.call('preflight.detectAgents', { commands })
  → [SSH tunnel] → relay process on dev server
  → { agents: ['github-copilot', 'claude'], platform: 'linux' }
```

### Windows Capabilities Cache

```typescript
// Cache key: `win-caps-${devServerId}`, TTL: 60 seconds
const windowsCapsCache = new Map<string, {
  result: WindowsTerminalCapabilities
  cachedAt: number
}>()
```

### SSH Fleet Handlers (remote-server CRs, trong ssh.ts)

12+ new IPC handlers đã được thêm vào `src/main/ipc/ssh.ts`:
```
ssh:fleet:import, ssh:fleet:status, ssh:fleet:bootstrap
ssh:fleet:listByGroup, ssh:fleet:getHealthHistory, ...
```

Tham khảo: [TDD-13: Dev Server Onboarding](./13-dev-server-onboarding.md)
