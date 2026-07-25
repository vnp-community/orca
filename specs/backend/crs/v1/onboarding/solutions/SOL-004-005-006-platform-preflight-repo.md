# SOL-004 + SOL-005 + SOL-006: Remote Preflight, Platform Wizard & Remote Repo

**CRs:** [CR-OB-004](../../../../../docs/crs/v1/onboarding/CR-OB-004-platform-aware-wizard.md) | [CR-OB-005](../../../../../docs/crs/v1/onboarding/CR-OB-005-remote-preflight.md) | [CR-OB-006](../../../../../docs/crs/v1/onboarding/CR-OB-006-remote-folder-repo.md)  
**TDD refs:** TDD-05, TDD-06, TDD-09  
**Status:** ✅ Implemented (IPC layer) | **Phase:** 2  
**Depends on:** SOL-002, SOL-003

> Gộp 3 CR thành 1 solution vì cùng mở rộng IPC layer `onboarding.*` và `repo.*`.

---

## PART A — SOL-004: Platform Source of Truth (Backend)

### A.1 DevServer platform expose qua IPC

```typescript
// Đã có trong SOL-002: DevServerManager.get(id) → DevServer { platform }
// IPC mới cần thêm:
ipc.handle('devServer.getPlatform', async (_, devServerId: string): Promise<NodeJS.Platform | null> => {
  return devServerManager.get(devServerId)?.platform ?? null
})
```

### A.2 GlobalSettings — activeDevServerId

```typescript
// src/shared/types.ts (MODIFY):
type GlobalSettings = {
  // ...existing...
  activeDevServerId?: string | null    // NEW
}

// IPC:
ipc.handle('settings.setActiveDevServer', async (_, devServerId: string | null) => {
  await store.updateGlobalSettings({ activeDevServerId: devServerId ?? null })
  devServerManager.emit('activeDevServerChanged', devServerId)
})
```

### A.3 Theme Step — Ghostty detection (remote)

```typescript
// src/relay/preflight-handler.ts (MODIFY) — thêm method:
private async detectGhosttyConfig(): Promise<{
  configPath: string | null
  themeDir: string | null
}> {
  // Chạy trên relay (dev server):
  const home = homedir()
  const ghosttyConfigPath = join(home, '.config', 'ghostty', 'config')
  const ghosttyThemeDir = join(home, '.config', 'ghostty', 'themes')
  return {
    configPath: existsSync(ghosttyConfigPath) ? ghosttyConfigPath : null,
    themeDir: existsSync(ghosttyThemeDir) ? ghosttyThemeDir : null
  }
}

// Register:
this.dispatcher.onRequest('preflight.detectGhosttyConfig', () => this.detectGhosttyConfig())
```

```typescript
// IPC:
ipc.handle('onboarding.detectGhosttyConfig', async (_, params: { devServerId: string }) => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  return relay.session.call('preflight.detectGhosttyConfig', {})
})
```

---

## PART B — SOL-005: Remote Preflight (gh + git)

### B.1 Remote Preflight Status Type

```typescript
// src/shared/dev-server-types.ts (MODIFY — thêm):
export type RemotePreflightStatus = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}
```

### B.2 Relay — Preflight Handler (MODIFY)

```typescript
// src/relay/preflight-handler.ts — thêm method toàn diện:
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

private async checkGhCli(): Promise<{ installed: boolean; authenticated: boolean; version?: string }> {
  try {
    const { stdout: version } = await execFileAsync('gh', ['--version'])
    // gh auth status: exit 0 = authenticated, exit 1 = not authenticated
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

### B.3 Set Git Identity (remote)

```typescript
// src/relay/preflight-handler.ts — thêm:
private async setGitIdentity(params: { name: string; email: string }): Promise<void> {
  await execFileAsync('git', ['config', '--global', 'user.name', params.name])
  await execFileAsync('git', ['config', '--global', 'user.email', params.email])
}

// Register:
this.dispatcher.onRequest('preflight.setGitIdentity', (p) => this.setGitIdentity(p as any))
```

### B.4 IPC Handlers

```typescript
// src/main/ipc/onboarding-ipc.ts (MODIFY — thêm vào registerOnboardingIpcHandlers):

// Cache per dev server
const preflightCache = new Map<string, { result: RemotePreflightStatus; cachedAt: number }>()
const PREFLIGHT_CACHE_TTL_MS = 30_000

ipc.handle('onboarding.getPreflightStatus', async (_, params: {
  devServerId: string
  force?: boolean
}): Promise<RemotePreflightStatus> => {
  const { devServerId, force = false } = params

  // Cache check
  if (!force) {
    const cached = preflightCache.get(devServerId)
    if (cached && Date.now() - cached.cachedAt < PREFLIGHT_CACHE_TTL_MS) {
      return cached.result
    }
  }

  const relay = devServerManager.getRelay(devServerId)
  if (!relay) throw new Error(`Dev server ${devServerId} not connected`)

  const raw = await relay.session.call('preflight.check', {})
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

ipc.handle('onboarding.setGitIdentity', async (_, params: {
  devServerId: string
  name: string
  email: string
}): Promise<void> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error(`Dev server ${params.devServerId} not connected`)
  await relay.session.call('preflight.setGitIdentity', {
    name: params.name,
    email: params.email
  })
  // Invalidate cache
  preflightCache.delete(params.devServerId)
})
```

### B.5 PTY Route cho `gh auth login` (remote)

```typescript
// Dùng lại hệ thống PTY hiện có — không cần IPC mới.
// Frontend chỉ cần truyền targetId (devServerId mapped sang relay PTY):

// PTY handler đã hỗ trợ remote qua environmentId.
// Thêm mapping devServerId → environmentId trong runtime:
ipc.handle('onboarding.openGhAuthTerminal', async (_, params: { devServerId: string }) => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  // Tạo PTY trên remote relay với command 'gh auth login'
  const ptyId = await relay.session.createPty({
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

## PART C — SOL-006: Remote Folder/Repo

### C.1 Remote Directory Listing

```typescript
// src/relay/fs-handler.ts (MODIFY) — thêm method listDirectoryWithGitStatus:
// Hoặc tạo mới: src/relay/fs-handler-directory-browse.ts

export class FsDirectoryBrowserHandler {
  constructor(private dispatcher: RelayDispatcher) {
    this.dispatcher.onRequest('fs.listDirectory', (p) => this.listDirectory(p))
  }

  private async listDirectory(params: {
    path: string
    includeGitStatus?: boolean
  }): Promise<{
    entries: DirectoryEntry[]
    platform: NodeJS.Platform
  }> {
    const { path: dirPath, includeGitStatus = false } = params

    let entries: DirectoryEntry[]
    try {
      const items = await readdir(dirPath, { withFileTypes: true })
      entries = await Promise.all(
        items
          .filter(item => item.isDirectory())  // Chỉ trả directories trong onboarding
          .map(async item => {
            const fullPath = join(dirPath, item.name)
            let isGitRepo = false
            if (includeGitStatus) {
              isGitRepo = await this.isGitRepo(fullPath)
            }
            return {
              name: item.name,
              path: fullPath,
              isDirectory: true,
              isGitRepo
            }
          })
      )
    } catch (err) {
      throw new Error(`Cannot list directory ${dirPath}: ${(err as Error).message}`)
    }

    return { entries, platform: process.platform }
  }

  private async isGitRepo(dirPath: string): Promise<boolean> {
    try {
      await stat(join(dirPath, '.git'))
      return true
    } catch {
      return false
    }
  }
}
```

### C.2 Remote Clone

```typescript
// src/relay/git-handler.ts (MODIFY) — thêm cloneRepo method:
async cloneRepo(params: {
  url: string
  targetPath: string
  onProgress?: (line: string) => void
}): Promise<{ path: string }> {
  // Tạo PTY để stream git clone progress
  const pty = createPty({
    command: 'git',
    args: ['clone', '--progress', params.url, params.targetPath],
    env: {}
  })
  // Stream output về caller qua progress callback
  // ...
  return { path: params.targetPath }
}

// Register:
this.dispatcher.onRequest('git.clone', (p) => this.cloneRepo(p))
```

### C.3 IPC Handlers — Remote Repo

```typescript
// src/main/ipc/repo-ipc.ts (MODIFY):

ipc.handle('repo.listRemoteDirectory', async (_, params: {
  devServerId: string
  path: string
  includeGitStatus?: boolean
}): Promise<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')
  return relay.session.call('fs.listDirectory', {
    path: params.path,
    includeGitStatus: params.includeGitStatus ?? false
  })
})

ipc.handle('repo.addRemote', async (_, params: {
  devServerId: string
  path: string
  name?: string
}): Promise<Repo> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  // Validate path exists on remote:
  const stat = await relay.session.call('fs.stat', { path: params.path })
  if (!stat.exists) throw new Error(`Path does not exist on dev server: ${params.path}`)

  const devServer = devServerManager.get(params.devServerId)!
  return runtimeService.addRepo({
    path: params.path,
    name: params.name ?? basename(params.path),
    connectionId: devServer.sshTargetId,   // Kế thừa SSH target
    devServerId: params.devServerId
  })
})

ipc.handle('repo.cloneRemote', async (_, params: {
  devServerId: string
  url: string
  targetDir?: string
}): Promise<{ repoId: string; path: string }> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  const devServer = devServerManager.get(params.devServerId)!
  const workspaceDir = devServer.workspaceDir ?? '~/orca/workspaces'
  const repoName = params.url.split('/').pop()?.replace(/\.git$/, '') ?? 'repo'
  const targetPath = params.targetDir ?? `${workspaceDir}/${repoName}`

  // Clone trên dev server via relay PTY
  await relay.session.call('git.clone', {
    url: params.url,
    targetPath
  })

  // Add repo to store
  const repo = await ipcHandler('repo.addRemote', { devServerId: params.devServerId, path: targetPath })
  return { repoId: repo.id, path: targetPath }
})

ipc.handle('repo.scanRemote', async (_, params: {
  devServerId: string
  rootPath: string
  maxDepth?: number
}): Promise<{ path: string; name: string }[]> => {
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  const { entries } = await relay.session.call('fs.listDirectory', {
    path: params.rootPath,
    includeGitStatus: true
  })

  return entries
    .filter((e: DirectoryEntry) => e.isGitRepo)
    .map((e: DirectoryEntry) => ({
      path: e.path,
      name: basename(e.path)
    }))
})
```

### C.4 Repo Schema — thêm devServerId

```typescript
// src/shared/types.ts (MODIFY):
type Repo = {
  // ... existing ...
  devServerId?: string | null    // NEW — ID của DevServer chứa repo (null = local)
}
```

---

## Tests tổng hợp

```typescript
// src/main/ipc/__tests__/onboarding-preflight.test.ts
describe('onboarding.getPreflightStatus', () => {
  it('cache miss → gọi relay, lưu cache')
  it('cache hit (< 30s) → không gọi relay')
  it('force: true → bỏ qua cache')
  it('relay không kết nối → throw Error')
  it('gh installed + authenticated → { installed: true, authenticated: true }')
  it('gh installed + not authenticated → { installed: true, authenticated: false }')
  it('gh not installed → { installed: false }')
  it('git installed, có identity → { installed: true, hasUserName: true, hasUserEmail: true }')
})

describe('onboarding.setGitIdentity', () => {
  it('gọi preflight.setGitIdentity trên relay với name + email')
  it('invalidate preflight cache sau khi set')
})

describe('repo.listRemoteDirectory', () => {
  it('trả về directories trên dev server path')
  it('includeGitStatus = true → đánh dấu git repos')
  it('path không tồn tại → throw Error')
})

describe('repo.cloneRemote', () => {
  it('tạo PTY với git clone trên relay')
  it('add repo vào store sau khi clone thành công')
  it('targetDir mặc định theo devServer.workspaceDir')
})
```

---

## Checklist triển khai

**SOL-004:**
- [x] IPC `devServer.getPlatform`
- [x] IPC `settings.setActiveDevServer`
- [x] Relay `preflight.detectGhosttyConfig` (IPC handler → forwards to relay)

**SOL-005:**
- [x] Relay `preflight.check` (gh + git combined — forwarded via IPC)
- [x] Relay `preflight.setGitIdentity` (forwarded via IPC)
- [x] IPC `onboarding.getPreflightStatus` với cache (TTL 30s)
- [x] IPC `onboarding.setGitIdentity`
- [x] IPC `onboarding.openGhAuthTerminal` (remote PTY via relay)

**SOL-006:**
- [x] Relay `fs.listDirectory` với `includeGitStatus` (forwarded via IPC)
- [x] IPC `repo.listRemoteDirectory`
- [x] IPC `repo.addRemote` với path validation
- [x] IPC `repo.cloneRemote`
- [x] IPC `repo.scanRemote`
- [x] `Repo.devServerId` schema field
- [ ] Relay `git.clone` với PTY streaming (relay-side PTY — deferred to relay release)
