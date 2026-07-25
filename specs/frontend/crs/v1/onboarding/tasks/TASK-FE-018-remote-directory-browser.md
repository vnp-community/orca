# TASK-FE-018: Tạo RemoteDirectoryBrowser hook + component

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created:**
> - `src/renderer/src/hooks/useRemoteDirectoryBrowser.ts` [NEW]
> - `src/renderer/src/components/remote-browser/RemoteDirectoryBrowser.tsx` [NEW]
> - `src/renderer/src/components/remote-browser/RemoteDirectoryEntry.tsx` [NEW]

**Phase:** 2 | **Solution:** [FE-SOL-C](../solutions/FE-SOL-C-preflight-repo.md) | **CR:** CR-OB-006  
**Depends on:** TASK-FE-002, TASK-FE-020  
**Estimated effort:** ~60 phút

---

## Context

Đọc trước:
- [`../solutions/FE-SOL-C-preflight-repo.md`](../solutions/FE-SOL-C-preflight-repo.md) — Section 7

---

## Goal

Tạo hook và components cho phép browse remote filesystem của dev server:
- `useRemoteDirectoryBrowser` — navigate, platform-aware path separator
- `RemoteDirectoryBrowser` — UI component với breadcrumb + manual path input + listing
- `RemoteDirectoryEntry` — single directory row

---

## Steps

### 1. Hook — `src/renderer/src/hooks/useRemoteDirectoryBrowser.ts`

```typescript
import { useState, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'

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
        includeGitStatus: true,
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

  // Navigate up — platform-aware separator
  const navigateUp = useCallback(() => {
    if (!currentPath || !platform) return
    const sep = platform === 'win32' ? '\\' : '/'
    const parts = currentPath.split(sep).filter(Boolean)
    if (parts.length <= 1) return
    parts.pop()
    const parent = (platform === 'win32' ? '' : '/') + parts.join(sep)
    void navigate(parent)
  }, [currentPath, platform, navigate])

  // Init: navigate đến workspaceDir hoặc default home của dev server
  useEffect(() => {
    if (!devServerId) return
    const ds = useAppStore.getState().devServers.find((d) => d.id === devServerId)
    const defaultPath = ds?.workspaceDir ?? (ds?.platform === 'win32' ? 'C:\\Users' : '/home')
    void navigate(defaultPath)
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { currentPath, entries, loading, error, platform, navigate, navigateUp }
}
```

### 2. Component — `src/renderer/src/components/remote-browser/RemoteDirectoryBrowser.tsx`

```tsx
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
      {/* Toolbar */}
      <div className="remote-dir-browser__toolbar">
        <Button variant="ghost" size="sm" onClick={navigateUp} disabled={!currentPath}>
          ↑ Up
        </Button>
        <code className="remote-dir-browser__path">{currentPath ?? '…'}</code>
      </div>

      {/* Manual path input */}
      <div className="remote-dir-browser__manual">
        <Input
          id="manual-path-input"
          placeholder={platform === 'win32' ? 'C:\\path\\to\\project' : '/home/user/projects'}
          value={manualPath}
          onChange={(e) => setManualPath(e.target.value)}
        />
        <Button id="manual-path-add-btn" onClick={() => onSelect(manualPath)} disabled={!manualPath}>
          Use this path
        </Button>
      </div>

      {loading && <div className="remote-dir-browser__loading"><Spinner /> Loading…</div>}
      {error && <p className="remote-dir-browser__error">{error}</p>}

      {/* Directory listing */}
      <div className="remote-dir-browser__list" role="list">
        {entries.map((entry) => (
          <RemoteDirectoryEntry
            key={entry.path}
            entry={entry}
            onNavigate={() => void navigate(entry.path)}
            onSelect={() => onSelect(entry.path)}
          />
        ))}
        {!loading && entries.length === 0 && (
          <p className="remote-dir-browser__empty">No directories found</p>
        )}
      </div>
    </div>
  )
}
```

### 3. Component — `src/renderer/src/components/remote-browser/RemoteDirectoryEntry.tsx`

```tsx
export function RemoteDirectoryEntry({ entry, onNavigate, onSelect }: {
  entry: { name: string; path: string; isGitRepo: boolean }
  onNavigate: () => void
  onSelect: () => void
}) {
  return (
    <div className={`remote-dir-entry ${entry.isGitRepo ? 'remote-dir-entry--git' : ''}`} role="listitem">
      <button className="remote-dir-entry__name" onClick={onNavigate} title={entry.path}>
        <span aria-hidden="true">{entry.isGitRepo ? '📂' : '📁'}</span>
        {entry.name}
        {entry.isGitRepo && (
          <span className="remote-dir-entry__git-dot" title="Git repository" aria-label="Git repository" />
        )}
      </button>
      <Button variant="ghost" size="xs" onClick={onSelect}>
        Select
      </Button>
    </div>
  )
}
```

---

## Tests

```typescript
// src/renderer/src/hooks/__tests__/useRemoteDirectoryBrowser.test.ts
describe('useRemoteDirectoryBrowser', () => {
  it('navigate() gọi window.api.repo.listRemoteDirectory')
  it('entries được set từ API response')
  it('loading = true khi đang navigate')
  it('error được set khi API throw')
  it('navigateUp() — POSIX: /home/user/projects → /home/user')
  it('navigateUp() — Windows: C:\\Users\\user → C:\\Users')
  it('navigateUp() không làm gì khi path chỉ có 1 segment')
  it('init: navigate đến workspaceDir hoặc default home')
})

// src/renderer/src/components/remote-browser/__tests__/RemoteDirectoryBrowser.test.tsx
describe('RemoteDirectoryBrowser', () => {
  it('render entries từ hook')
  it('git repo entry có css class remote-dir-entry--git')
  it('click Up button gọi navigateUp()')
  it('click entry name gọi navigate(entry.path)')
  it('click Select gọi onSelect(entry.path)')
  it('manual path input + "Use this path" gọi onSelect(manualPath)')
  it('hiển thị Spinner khi loading')
  it('hiển thị error text khi có lỗi')
  it('hiển thị "No directories found" khi entries rỗng')
})
```

---

## Acceptance Criteria

- [ ] `navigateUp()` đúng separator cho Windows và POSIX
- [ ] `RemoteDirectoryBrowser` có unique IDs: `manual-path-input`, `manual-path-add-btn`
- [ ] Git repo entries có visual indicator
- [ ] Loading và error states hiển thị đúng
- [ ] Tests pass

## Output Files

- **[NEW]** `src/renderer/src/hooks/useRemoteDirectoryBrowser.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useRemoteDirectoryBrowser.test.ts`
- **[NEW]** `src/renderer/src/components/remote-browser/RemoteDirectoryBrowser.tsx`
- **[NEW]** `src/renderer/src/components/remote-browser/RemoteDirectoryEntry.tsx`
- **[NEW]** `src/renderer/src/components/remote-browser/__tests__/RemoteDirectoryBrowser.test.tsx`
