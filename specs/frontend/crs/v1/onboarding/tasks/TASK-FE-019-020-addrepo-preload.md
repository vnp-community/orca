# TASK-FE-019 + TASK-FE-020: AddRepoStep & Preload Bridge

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/components/onboarding/AddRepoStep.tsx` [NEW] — TASK-FE-019
> - `src/preload/api-types.ts` [MODIFY] — onboarding extended methods + `repos.listRemoteDirectory`, `repos.scanRemote` — TASK-FE-020

---

# TASK-FE-019: Tạo AddRepoStep component

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-006  
**Depends on:** TASK-FE-002, TASK-FE-018, TASK-FE-020

## Goal
Tạo `AddRepoStep.tsx` — bước onboarding thêm repository từ dev server, hỗ trợ 3 modes: Browse / Clone / Scan.

## Steps

**Tạo** `src/renderer/src/components/onboarding/AddRepoStep.tsx`:

```typescript
type Props = {
  activeDevServerId: string | null
  onRepoAdded: (repoId: string) => void
}

type Mode = 'browse' | 'clone' | 'scan'
```

### Browse mode
- Render `<RemoteDirectoryBrowser devServerId={activeDevServerId} onSelect={handleSelectPath} />`
- `handleSelectPath(path)` → `window.api.repo.addRemote({ devServerId, path })`

### Clone mode
- `<Input id="clone-url-input" placeholder="https://github.com/org/repo" />`
- `<Button id="clone-btn">Clone</Button>`
- Gọi `window.api.repo.cloneRemote({ devServerId, url })`
- Loading state khi cloning

### Scan mode
- Button "Scan for git repos" → `window.api.repo.scanRemote({ devServerId, rootPath: currentPath })`
- List repos tìm được → checkbox chọn → "Add selected repos"

### No dev server state
- Hiển thị "Connect a dev server first" với button quay lại

### Dev server selector
- Nếu có > 1 connected server → show `<Select>` chọn server

**Tests** (10 cases):
- no devServerId → hiển thị notice
- Browse tab → RemoteDirectoryBrowser + add khi select
- Clone tab → input + button + loading
- Clone thành công → gọi onRepoAdded với repoId
- Scan tab → scan + checkbox + add selected
- Error display khi add fail

## Output Files
- **[NEW]** `src/renderer/src/components/onboarding/AddRepoStep.tsx`
- **[NEW]** `src/renderer/src/components/onboarding/__tests__/AddRepoStep.test.tsx`

---

# TASK-FE-020: Extend window.api preload bridge — repo + onboarding namespaces

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-005, CR-OB-006  
**Depends on:** TASK-FE-007

## Goal
Thêm đầy đủ `window.api.repo.*` và `window.api.onboarding.*` methods vào cả Electron preload và Web preload bridge.

## Steps

1. **Đọc** file type declarations của `window.api` hiện tại để tìm interface definition.

2. **Thêm** vào `window.api.repo`:

```typescript
// Type declarations:
repo: {
  // ...existing methods
  listRemoteDirectory: (params: {
    devServerId: string
    path: string
    includeGitStatus?: boolean
  }) => Promise<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }>

  addRemote: (params: {
    devServerId: string
    path: string
    name?: string
  }) => Promise<Repo>

  cloneRemote: (params: {
    devServerId: string
    url: string
    targetDir?: string
  }) => Promise<{ repoId: string; path: string }>

  scanRemote: (params: {
    devServerId: string
    rootPath: string
    maxDepth?: number
  }) => Promise<{ path: string; name: string }[]>
}
```

3. **Thêm** vào `window.api.onboarding`:

```typescript
onboarding: {
  detectAgents: (params: { devServerId: string | null }) => Promise<{
    agents: string[]
    platform: NodeJS.Platform | null
    devServerId: string | null
  }>

  detectAgentsAllServers: () => Promise<Record<string, {
    agents: string[]
    platform: NodeJS.Platform | null
    error?: string
  }>>

  getPreflightStatus: (params: {
    devServerId: string
    force?: boolean
  }) => Promise<RemotePreflightStatus>

  setGitIdentity: (params: {
    devServerId: string
    name: string
    email: string
  }) => Promise<void>

  openGhAuthTerminal: (params: {
    devServerId: string
  }) => Promise<{ ptyId: string; devServerId: string }>

  detectGhosttyConfig: (params: {
    devServerId: string
  }) => Promise<{ configPath: string | null; themeDir: string | null }>

  detectWindowsCapabilities: (params: {
    devServerId: string
  }) => Promise<WindowsTerminalCapabilities>

  markChecklistItem: (params: {
    item: string
    devServerId?: string
    value?: boolean
  }) => Promise<void>
}
```

4. **Implement** từng method bằng `ipcRenderer.invoke(...)` (Electron) hoặc RPC call (Web).

## Output Files
- **[MODIFY]** `src/preload/index.ts`
- **[MODIFY]** `src/renderer/src/web/web-preload-api.ts`
- **[MODIFY]** Type declaration file (nếu tách riêng)
