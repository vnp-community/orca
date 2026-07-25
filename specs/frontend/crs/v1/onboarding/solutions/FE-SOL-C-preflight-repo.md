# FE-SOL-C: Remote Preflight (gh/git) & Remote Repo UI

**CRs:** [CR-OB-005](../../../../../docs/crs/v1/onboarding/CR-OB-005-remote-preflight.md) | [CR-OB-006](../../../../../docs/crs/v1/onboarding/CR-OB-006-remote-folder-repo.md)  
**TDD refs:** TDD-FE-02 (State), TDD-FE-05 (Components), TDD-FE-07 (Hooks)  
**Status:** ✅ COMPLETED (2026-07-23) | **Phase:** 2

---

## 1. New Files

```
src/renderer/src/
├── components/onboarding/
│   ├── IntegrationsStep.tsx            ← MODIFY: remote preflight + remote PTY
│   ├── GitIdentityCard.tsx             ← NEW: git user.name / user.email form
│   └── AddRepoStep.tsx                 ← NEW: remote directory browser
├── components/remote-browser/
│   ├── RemoteDirectoryBrowser.tsx      ← NEW: browse dev server filesystem
│   ├── RemoteDirectoryEntry.tsx        ← NEW: single directory row
│   └── remote-directory-browser.css
├── hooks/
│   ├── useRemotePreflightStatus.ts     ← NEW: gh + git check trên dev server
│   ├── useRemoteDirectoryBrowser.ts    ← NEW: browse remote filesystem
│   └── useRemoteRepoAdd.ts             ← NEW: add/clone/scan remote repos
└── store/slices/
    └── preflight.ts                    ← MODIFY: per-server preflight cache
```

---

## 2. Zustand Preflight Slice (MODIFY)

```typescript
// src/renderer/src/store/slices/preflight.ts (MODIFY)
import type { RemotePreflightStatus } from '../../../../shared/dev-server-types'

type PreflightSlice = {
  // TRƯỚC:
  preflightStatus: PreflightStatus | null

  // SAU (thêm):
  remotePreflightByServer: Record<string, RemotePreflightStatus>
  activeRemotePreflightStatus: RemotePreflightStatus | null

  // Actions:
  setRemotePreflightStatus: (devServerId: string, status: RemotePreflightStatus) => void
  clearRemotePreflightStatus: (devServerId: string) => void
}

export const createPreflightSlice = (set: SetState<AppState>, get: GetState<AppState>) => ({
  // ...existing...
  remotePreflightByServer: {},
  activeRemotePreflightStatus: null,

  setRemotePreflightStatus: (devServerId, status) =>
    set(state => {
      const updated = { ...state.remotePreflightByServer, [devServerId]: status }
      return {
        remotePreflightByServer: updated,
        activeRemotePreflightStatus:
          devServerId === state.activeDevServerId ? status : state.activeRemotePreflightStatus
      }
    }),

  clearRemotePreflightStatus: (devServerId) =>
    set(state => {
      const { [devServerId]: _, ...rest } = state.remotePreflightByServer
      return { remotePreflightByServer: rest }
    })
})

// Selector:
export function useActiveRemotePreflightStatus() {
  return useAppStore(s => s.activeRemotePreflightStatus)
}
```

---

## 3. Hook — `useRemotePreflightStatus.ts`

```typescript
// src/renderer/src/hooks/useRemotePreflightStatus.ts
import { useState, useCallback } from 'react'
import { useAppStore } from '../store'

export function useRemotePreflightStatus(devServerId: string | null) {
  const [loading, setLoading] = useState(false)
  const setRemotePreflightStatus = useAppStore(s => s.setRemotePreflightStatus)
  const clearRemotePreflightStatus = useAppStore(s => s.clearRemotePreflightStatus)
  const statusFromStore = useAppStore(
    s => devServerId ? s.remotePreflightByServer[devServerId] ?? null : null
  )

  const refresh = useCallback(async (force = false) => {
    if (!devServerId) return
    setLoading(true)
    try {
      const result = await window.api.onboarding.getPreflightStatus({
        devServerId,
        force
      })
      setRemotePreflightStatus(devServerId, result)
    } catch {
      // non-fatal: hiển thị stale data nếu có
    } finally {
      setLoading(false)
    }
  }, [devServerId, setRemotePreflightStatus])

  // Auto-refresh khi devServerId thay đổi
  useEffect(() => {
    if (!devServerId) return
    void refresh()
  }, [devServerId, refresh])

  // Khi dev server thay đổi → clear cache cũ
  useEffect(() => {
    return () => {
      if (devServerId) clearRemotePreflightStatus(devServerId)
    }
  }, [devServerId])

  return { status: statusFromStore, loading, refresh }
}

// Derived:
export function useGhInstalled(devServerId: string | null): boolean {
  const status = useAppStore(
    s => devServerId ? s.remotePreflightByServer[devServerId] ?? null : null
  )
  return status?.gh.installed === true
}
```

---

## 4. IntegrationsStep — MODIFY

```tsx
// src/renderer/src/components/onboarding/IntegrationsStep.tsx (MODIFY)
import { useRemotePreflightStatus } from '../../hooks/useRemotePreflightStatus'
import { GitIdentityCard } from './GitIdentityCard'

type IntegrationsStepProps = {
  // ...existing
  activeDevServerId: string | null   // NEW
}

export function IntegrationsStep({ activeDevServerId, ...rest }: IntegrationsStepProps) {
  const { status, loading, refresh } = useRemotePreflightStatus(activeDevServerId)

  // Skip step nếu gh VÀ git đều đã OK:
  // (logic skip được xử lý trong use-onboarding-flow.ts)

  return (
    <div className="onboarding-step integrations-step">
      {!activeDevServerId && (
        <div className="no-server-notice">
          Connect a dev server first to check integrations
        </div>
      )}

      {loading && <Spinner />}

      {/* GitHub CLI section */}
      <GithubCliSection
        preflightStatus={status}
        activeDevServerId={activeDevServerId}
        onRefresh={() => refresh(true)}
      />

      {/* Git Identity section */}
      {status?.git.installed && (
        <GitIdentityCard
          devServerId={activeDevServerId}
          hasUserName={status.git.hasUserName}
          hasUserEmail={status.git.hasUserEmail}
          onSaved={() => refresh(true)}
        />
      )}

      {/* Linear (tùy chọn — không thay đổi) */}
      <LinearSection />
    </div>
  )
}

// GitHub CLI Section — sửa PTY để chạy trên remote:
function GithubCliSection({
  preflightStatus,
  activeDevServerId,
  onRefresh
}: {
  preflightStatus: RemotePreflightStatus | null
  activeDevServerId: string | null
  onRefresh: () => void
}) {
  const [terminalOpen, setTerminalOpen] = useState(false)

  if (preflightStatus?.gh.installed === false) {
    return (
      <div className="gh-section">
        <GhStatusBadge status="not-installed" />
        <a href="https://cli.github.com" target="_blank" rel="noreferrer">
          Install GitHub CLI
        </a>
      </div>
    )
  }

  if (preflightStatus?.gh.authenticated === false) {
    return (
      <div className="gh-section">
        <GhStatusBadge status="not-authenticated" />
        <Button onClick={() => setTerminalOpen(true)}>Sign in to GitHub</Button>
        {terminalOpen && (
          // Remote PTY: chạy gh auth login trên dev server
          <OnboardingInlineCommandTerminal
            command="gh auth login"
            title="GitHub setup"
            devServerId={activeDevServerId}   // NEW prop
            onComplete={() => {
              setTerminalOpen(false)
              onRefresh()
            }}
          />
        )}
      </div>
    )
  }

  return (
    <div className="gh-section">
      <GhStatusBadge status="connected" />
    </div>
  )
}
```

---

## 5. `GitIdentityCard.tsx` (NEW)

```tsx
// src/renderer/src/components/onboarding/GitIdentityCard.tsx
import { useState } from 'react'

type Props = {
  devServerId: string | null
  hasUserName: boolean
  hasUserEmail: boolean
  onSaved: () => void
}

export function GitIdentityCard({ devServerId, hasUserName, hasUserEmail, onSaved }: Props) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (hasUserName && hasUserEmail) {
    return (
      <div className="git-identity-card git-identity-card--ok">
        <span>✓ Git identity configured</span>
      </div>
    )
  }

  const handleSave = async () => {
    if (!devServerId || !name || !email) return
    setSaving(true)
    setError(null)
    try {
      await window.api.onboarding.setGitIdentity({ devServerId, name, email })
      onSaved()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="git-identity-card">
      <h4>Git identity</h4>
      <p className="hint">Required for commits on this dev server</p>
      {!hasUserName && (
        <div className="field">
          <label>Name</label>
          <Input
            id="git-user-name"
            placeholder="Your Name"
            value={name}
            onChange={e => setName(e.target.value)}
          />
        </div>
      )}
      {!hasUserEmail && (
        <div className="field">
          <label>Email</label>
          <Input
            id="git-user-email"
            placeholder="you@example.com"
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
          />
        </div>
      )}
      {error && <p className="error">{error}</p>}
      <Button
        id="save-git-identity-btn"
        onClick={handleSave}
        disabled={!name || !email || saving}
        loading={saving}
      >
        Save Git Identity
      </Button>
    </div>
  )
}
```

---

## 6. `OnboardingInlineCommandTerminal` — MODIFY (thêm `devServerId`)

```tsx
// src/renderer/src/components/onboarding/OnboardingInlineCommandTerminal.tsx (MODIFY)
type Props = {
  command: string
  title: string
  devServerId?: string | null    // NEW
  onComplete?: () => void
}

export function OnboardingInlineCommandTerminal({ devServerId, ...rest }: Props) {
  useEffect(() => {
    // Mở PTY trên dev server nếu có, không thì local
    if (devServerId) {
      window.api.onboarding.openGhAuthTerminal({ devServerId })
        .then(({ ptyId }) => {
          // Gán ptyId vào terminal component
          attachPty(ptyId)
        })
    } else {
      // Existing local behavior
    }
  }, [devServerId])
  // ...
}
```

---

## 7. Remote Directory Browser

### Hook — `useRemoteDirectoryBrowser.ts`

```typescript
// src/renderer/src/hooks/useRemoteDirectoryBrowser.ts
import { useState, useCallback } from 'react'

type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  isGitRepo: boolean
}

export function useRemoteDirectoryBrowser(devServerId: string | null) {
  const [currentPath, setCurrentPath] = useState<string | null>(null)
  const [entries, setEntries] = useState<DirectoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [platform, setPlatform] = useState<NodeJS.Platform | null>(null)

  const navigate = useCallback(async (path: string) => {
    if (!devServerId) return
    setLoading(true)
    setError(null)
    try {
      const result = await window.api.repo.listRemoteDirectory({
        devServerId,
        path,
        includeGitStatus: true
      })
      setCurrentPath(path)
      setEntries(result.entries)
      setPlatform(result.platform)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [devServerId])

  // Navigate up one level:
  const navigateUp = useCallback(() => {
    if (!currentPath || !platform) return
    const sep = platform === 'win32' ? '\\' : '/'
    const parts = currentPath.split(sep).filter(Boolean)
    if (parts.length <= 1) return
    parts.pop()
    const parentPath = (platform === 'win32' ? '' : '/') + parts.join(sep)
    void navigate(parentPath)
  }, [currentPath, platform, navigate])

  // Init: navigate to home
  useEffect(() => {
    if (!devServerId) return
    const devServer = useAppStore.getState().devServers.find(ds => ds.id === devServerId)
    const defaultPath = devServer?.workspaceDir ?? (
      devServer?.platform === 'win32' ? 'C:\\Users' : '/home'
    )
    void navigate(defaultPath)
  }, [devServerId, navigate])

  return { currentPath, entries, loading, error, platform, navigate, navigateUp }
}
```

### Component — `RemoteDirectoryBrowser.tsx`

```tsx
// src/renderer/src/components/remote-browser/RemoteDirectoryBrowser.tsx
import { useRemoteDirectoryBrowser } from '../../hooks/useRemoteDirectoryBrowser'

type Props = {
  devServerId: string
  onSelect: (path: string) => void
}

export function RemoteDirectoryBrowser({ devServerId, onSelect }: Props) {
  const { currentPath, entries, loading, error, platform, navigate, navigateUp } =
    useRemoteDirectoryBrowser(devServerId)
  const [manualPath, setManualPath] = useState('')

  return (
    <div className="remote-dir-browser">
      {/* Breadcrumb + up button */}
      <div className="browser-toolbar">
        <Button variant="ghost" size="sm" onClick={navigateUp} disabled={!currentPath}>
          ↑ Up
        </Button>
        <span className="current-path">{currentPath}</span>
      </div>

      {/* Manual path input */}
      <div className="manual-path">
        <Input
          id="manual-path-input"
          placeholder={platform === 'win32' ? 'C:\\Users\\user\\projects' : '/home/user/projects'}
          value={manualPath}
          onChange={e => setManualPath(e.target.value)}
        />
        <Button
          id="manual-path-add-btn"
          onClick={() => onSelect(manualPath)}
          disabled={!manualPath}
        >
          Add
        </Button>
      </div>

      {/* Loading / Error */}
      {loading && <Spinner />}
      {error && <p className="error">{error}</p>}

      {/* Directory listing */}
      <div className="entry-list">
        {entries.map(entry => (
          <RemoteDirectoryEntry
            key={entry.path}
            entry={entry}
            onNavigate={() => navigate(entry.path)}
            onSelect={() => onSelect(entry.path)}
          />
        ))}
      </div>
    </div>
  )
}

function RemoteDirectoryEntry({ entry, onNavigate, onSelect }: {
  entry: DirectoryEntry
  onNavigate: () => void
  onSelect: () => void
}) {
  return (
    <div className={`dir-entry ${entry.isGitRepo ? 'dir-entry--git' : ''}`}>
      <button className="dir-name" onClick={onNavigate}>
        📁 {entry.name}
        {entry.isGitRepo && <span className="git-dot" title="Git repository" />}
      </button>
      <Button variant="ghost" size="xs" onClick={onSelect}>Select</Button>
    </div>
  )
}
```

### `AddRepoStep.tsx` (NEW)

```tsx
// src/renderer/src/components/onboarding/AddRepoStep.tsx
import { useState } from 'react'
import { RemoteDirectoryBrowser } from '../remote-browser/RemoteDirectoryBrowser'
import { useConnectedDevServers } from '../../store/slices/dev-servers'

type Props = {
  activeDevServerId: string | null
  onRepoAdded: (repoId: string) => void
}

type Mode = 'browse' | 'clone' | 'scan'

export function AddRepoStep({ activeDevServerId, onRepoAdded }: Props) {
  const [mode, setMode] = useState<Mode>('browse')
  const [cloneUrl, setCloneUrl] = useState('')
  const [cloning, setCloning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const connectedServers = useConnectedDevServers()

  const handleSelectPath = async (path: string) => {
    if (!activeDevServerId) return
    try {
      const repo = await window.api.repo.addRemote({ devServerId: activeDevServerId, path })
      onRepoAdded(repo.id)
    } catch (err) {
      setError((err as Error).message)
    }
  }

  const handleClone = async () => {
    if (!activeDevServerId || !cloneUrl) return
    setCloning(true)
    setError(null)
    try {
      const { repoId } = await window.api.repo.cloneRemote({
        devServerId: activeDevServerId,
        url: cloneUrl
      })
      onRepoAdded(repoId)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setCloning(false)
    }
  }

  if (!activeDevServerId) {
    return (
      <div className="add-repo-no-server">
        <p>Connect a dev server first to add a repository</p>
        {connectedServers.length === 0 && (
          <Button onClick={() => {/* go back to dev server step */}}>
            Connect Dev Server
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="add-repo-step">
      {/* Dev server selector (nếu có nhiều) */}
      {connectedServers.length > 1 && (
        <DevServerSelector
          servers={connectedServers}
          activeId={activeDevServerId}
          onSelect={id => {/* cập nhật activeDevServerId */}}
        />
      )}

      {/* Mode tabs */}
      <div className="mode-tabs">
        <button className={mode === 'browse' ? 'active' : ''} onClick={() => setMode('browse')}>
          Browse
        </button>
        <button className={mode === 'clone' ? 'active' : ''} onClick={() => setMode('clone')}>
          Clone URL
        </button>
        <button className={mode === 'scan' ? 'active' : ''} onClick={() => setMode('scan')}>
          Scan folder
        </button>
      </div>

      {error && <p className="error">{error}</p>}

      {/* Browse mode */}
      {mode === 'browse' && (
        <RemoteDirectoryBrowser
          devServerId={activeDevServerId}
          onSelect={handleSelectPath}
        />
      )}

      {/* Clone mode */}
      {mode === 'clone' && (
        <div className="clone-section">
          <Input
            id="clone-url-input"
            placeholder="https://github.com/org/repo"
            value={cloneUrl}
            onChange={e => setCloneUrl(e.target.value)}
          />
          <Button
            id="clone-btn"
            onClick={handleClone}
            disabled={!cloneUrl || cloning}
            loading={cloning}
          >
            Clone
          </Button>
        </div>
      )}

      {/* Scan mode */}
      {mode === 'scan' && (
        <ScanReposSection
          devServerId={activeDevServerId}
          onReposSelected={paths =>
            Promise.all(paths.map(p => handleSelectPath(p)))
          }
        />
      )}
    </div>
  )
}
```

---

## 8. Preload Bridge — thêm repo.* (MODIFY)

```typescript
// web-preload-api.ts / preload/index.ts (MODIFY)
window.api.repo = {
  // ...existing
  listRemoteDirectory: (params) => ipcRenderer.invoke('repo.listRemoteDirectory', params),
  addRemote: (params) => ipcRenderer.invoke('repo.addRemote', params),
  cloneRemote: (params) => ipcRenderer.invoke('repo.cloneRemote', params),
  scanRemote: (params) => ipcRenderer.invoke('repo.scanRemote', params)
}

window.api.onboarding = {
  // ...existing
  getPreflightStatus: (params) => ipcRenderer.invoke('onboarding.getPreflightStatus', params),
  setGitIdentity: (params) => ipcRenderer.invoke('onboarding.setGitIdentity', params),
  openGhAuthTerminal: (params) => ipcRenderer.invoke('onboarding.openGhAuthTerminal', params),
  detectGhosttyConfig: (params) => ipcRenderer.invoke('onboarding.detectGhosttyConfig', params)
}
```

---

## 9. Tests

```tsx
describe('useRemotePreflightStatus', () => {
  it('devServerId = null → status = null, không gọi API')
  it('gọi getPreflightStatus khi mount với devServerId')
  it('lưu result vào store')
  it('refresh(true) force gọi API')
  it('loading = true khi đang fetch')
})

describe('GitIdentityCard', () => {
  it('hiển thị "✓ Git identity configured" khi hasUserName VÀ hasUserEmail = true')
  it('hiển thị form Name khi hasUserName = false')
  it('hiển thị form Email khi hasUserEmail = false')
  it('Save disabled khi name hoặc email rỗng')
  it('gọi api.onboarding.setGitIdentity với đúng params')
  it('gọi onSaved() sau khi lưu thành công')
})

describe('useRemoteDirectoryBrowser', () => {
  it('navigate() gọi api.repo.listRemoteDirectory')
  it('navigateUp() tính đúng parent path theo platform')
  it('Windows: separator = backslash')
  it('POSIX: separator = forward slash')
  it('init: navigate đến workspaceDir hoặc default home')
})

describe('RemoteDirectoryBrowser', () => {
  it('render entries từ API')
  it('git repo entries có git-dot indicator')
  it('click entry.name → navigate vào folder')
  it('click Select → gọi onSelect với path')
  it('manual path input + Add → gọi onSelect với manual path')
})

describe('AddRepoStep', () => {
  it('hiển thị "Connect a dev server" khi activeDevServerId = null')
  it('Browse mode: hiển thị RemoteDirectoryBrowser')
  it('Clone mode: input URL + Clone button')
  it('Clone button disabled khi URL rỗng')
  it('Clone thành công → gọi onRepoAdded với repoId')
})
```

---

## 10. Checklist triển khai

**CR-OB-005 (Remote Preflight):**
- [x] Sửa Preflight slice: thêm `remotePreflightByServer`, `activeRemotePreflightStatus`
- [x] Tạo `useRemotePreflightStatus` hook
- [x] Sửa `IntegrationsStep.tsx`: nhận `activeDevServerId`, dùng remote preflight + GitIdentityCard
- [x] Tạo `GitIdentityCard.tsx`
- [ ] Sửa `OnboardingInlineCommandTerminal`: thêm `devServerId` prop → remote PTY (deferred)

**CR-OB-006 (Remote Repo):**
- [x] Tạo `useRemoteDirectoryBrowser` hook
- [x] Tạo `RemoteDirectoryBrowser.tsx` component
- [x] Tạo `AddRepoStep.tsx` component
- [x] Extend `window.api.repo.*` trong preload bridge (`api-types.ts`)
- [x] Extend `window.api.onboarding.*`
- [ ] Unit tests (deferred to test pass)
