# SOL-FE-V5-07: Remote File Explorer

**TDD Ref:** [TDD-FE-17](../../../tdd/17-file-explorer-ui.md)  
**Feature:** F38 | **ADR:** ADR-011 | **HLD:** C3.12  
**Status:** ✅ DONE — Implemented via TASK-V5-09, TASK-V5-10  
**Dependency:** WorkspaceContext (SOL-FE-V5-02)

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/components/workspace/ExplorerPanel.tsx` | Component | Main file tree panel |
| `src/renderer/src/components/workspace/FileTreeNode.tsx` | Component | Single tree node (dir/file) |
| `src/renderer/src/components/workspace/FileViewer.tsx` | Component | Read-only file content viewer |
| `src/renderer/src/components/workspace/FileSearchPanel.tsx` | Component | Fuzzy file search |
| `src/renderer/src/components/workspace/FileContextMenu.tsx` | Component | Right-click context menu |
| `src/renderer/src/hooks/useFileExplorer.ts` | Hook | Tree state, expand/collapse, lazy load |

---

## 2. ExplorerPanel — Thiết kế

```typescript
// src/renderer/src/components/workspace/ExplorerPanel.tsx
// Xem TDD-FE-17 section 2 cho layout

// Lazy load children:
// - Đầu tiên: chỉ load root directory
// - Khi user expand dir → load children của dir đó
// - Cache: đã load rồi thì không load lại (trừ khi refresh)

// Event subscriptions từ WorkspaceContext:
// 'agent.complete' → refreshFileTree() (agent có thể tạo/xóa files)
// 'files.changed' → refresh only parent dirs của changed files

// Context menu (right-click):
// File:      [Copy Path] [View] [Rename] [Delete]
// Directory: [Copy Path] [Refresh] [New File] [New Folder]

// Offline mode: tree hiển thị stale data với banner
// "File operations unavailable — dev server offline"
```

---

## 3. FileTreeNode Component

```typescript
// src/renderer/src/components/workspace/FileTreeNode.tsx

export type FileNode = {
  name:       string
  path:       string     // relative to project root
  type:       'file' | 'directory'
  size?:      number     // bytes (files only)
  children?:  FileNode[] // loaded lazily
  isLoading?: boolean    // lazy loading in progress
}

export function FileTreeNode({
  node, depth, isExpanded, onToggle, onSelect, onContextMenu
}: FileTreeNodeProps) {
  const indent = depth * 16  // px per level

  return (
    <div
      className={cn('file-tree-node', 'hover:bg-accent cursor-pointer', selected && 'bg-accent')}
      style={{ paddingLeft: indent }}
      onClick={() => node.type === 'directory' ? onToggle(node.path) : onSelect(node.path)}
      onContextMenu={e => onContextMenu(e, node)}
    >
      {node.type === 'directory' ? (
        <>
          {node.isLoading ? <Spinner size={12} /> : (
            isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />
          )}
          <FolderIcon size={14} className="text-yellow-500 ml-1" />
        </>
      ) : (
        <FileIcon size={14} className="ml-5 text-gray-400" />
      )}
      <span className="text-sm ml-1.5">{node.name}</span>
      {node.size !== undefined && (
        <span className="ml-auto text-xs text-muted-foreground">{formatBytes(node.size)}</span>
      )}
    </div>
  )
}
```

---

## 4. useFileExplorer Hook

```typescript
// src/renderer/src/hooks/useFileExplorer.ts

export function useFileExplorer() {
  const { project, fileTree, refreshFileTree, isOffline, on } = useWorkspace()
  const [expandedDirs, setExpandedDirs] = useState(new Set<string>())
  const [selectedPath, setSelectedPath]   = useState<string | null>(null)
  const [viewingFile, setViewingFile]     = useState<string | null>(null)
  const [searchQuery, setSearchQuery]     = useState('')

  // Subscribe to workspace events for auto-refresh
  useEffect(() => {
    const unsubs = [
      on('agent.complete', () => refreshFileTree()),
      on('files.changed', ({ paths }: { paths: string[] }) => {
        const parentDirs = [...new Set(paths.map(p => p.split('/').slice(0, -1).join('/')))]
        parentDirs.forEach(dir => refreshFileTree(dir))
      }),
      on('git.committed', () => refreshFileTree()),   // new files post-commit
    ]
    return () => unsubs.forEach(u => u())
  }, [on, refreshFileTree])

  const toggleDir = useCallback(async (dirPath: string) => {
    if (expandedDirs.has(dirPath)) {
      setExpandedDirs(prev => { const s = new Set(prev); s.delete(dirPath); return s })
    } else {
      setExpandedDirs(prev => new Set([...prev, dirPath]))
      await refreshFileTree(dirPath)  // lazy load children
    }
  }, [expandedDirs, refreshFileTree])

  const openFile = useCallback(async (filePath: string) => {
    setSelectedPath(filePath)
    setViewingFile(filePath)
  }, [])

  return {
    fileTree, expandedDirs, selectedPath, viewingFile, isOffline,
    toggleDir, openFile, searchQuery, setSearchQuery,
    refresh: refreshFileTree,
  }
}
```

---

## 5. FileViewer Component

```typescript
// src/renderer/src/components/workspace/FileViewer.tsx
// Read-only syntax-highlighted file content

// Lazy-loaded: heavy (syntax highlighter)
const FileViewerLazy = lazy(() => import('./FileViewer'))

// Implementation:
// - Fetch file content: rpc('workspace.readFile', { projectId, path })
// - Language detection: extension → highlight.js language
// - Supported: .ts, .tsx, .js, .jsx, .py, .go, .rs, .yaml, .json, .md, etc.
// - Max size: 500KB — show warning for larger files
// - Copy to clipboard button
// - Line numbers

export function FileViewer({ filePath }: { filePath: string }) {
  const { project } = useWorkspace()
  const [content, setContent] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!project || !filePath) return
    setIsLoading(true)
    rpc('workspace.readFile', { projectId: project.id, path: filePath })
      .then(c => setContent(c as string))
      .catch(err => setError(err.message))
      .finally(() => setIsLoading(false))
  }, [project, filePath])

  // Render with syntax highlight (lazy)
  // Max 500KB check
  // Copy button
}
```

---

## 6. FileSearchPanel Component

```typescript
// src/renderer/src/components/workspace/FileSearchPanel.tsx
// Fuzzy file search (by name, not content)

export function FileSearchPanel({ onSelect }: { onSelect: (path: string) => void }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<string[]>([])

  useEffect(() => {
    if (!query || query.length < 2) { setResults([]); return }
    const timer = setTimeout(async () => {
      const { project } = useWorkspace()
      if (!project) return
      const found = await rpc('workspace.searchFiles', {
        projectId: project.id,
        query,
        limit: 20
      }) as string[]
      setResults(found)
    }, 200)  // 200ms debounce
    return () => clearTimeout(timer)
  }, [query])

  return (
    <div className="file-search-panel">
      <Input
        value={query}
        onChange={e => setQuery(e.target.value)}
        placeholder="Search files..."
        autoFocus
      />
      {results.map(path => (
        <div key={path} className="result-item" onClick={() => onSelect(path)}>
          <FileIcon size={12} />
          <span className="text-sm">{path}</span>
        </div>
      ))}
    </div>
  )
}
```

---

## 7. FileContextMenu

```typescript
// src/renderer/src/components/workspace/FileContextMenu.tsx
// Radix UI ContextMenu

export function FileContextMenu({ node, children }) {
  const { project } = useWorkspace()

  const copyPath = () => navigator.clipboard.writeText(node.path)
  const deleteFile = async () => {
    if (!confirm(`Delete ${node.name}?`)) return
    await rpc('workspace.deleteFile', { projectId: project.id, path: node.path })
    refreshFileTree(node.path.split('/').slice(0, -1).join('/'))
  }

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onSelect={copyPath}>Copy Path</ContextMenuItem>
        {node.type === 'file' && (
          <>
            <ContextMenuItem onSelect={() => openFile(node.path)}>View</ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem className="text-red-600" onSelect={deleteFile}>Delete</ContextMenuItem>
          </>
        )}
        {node.type === 'directory' && (
          <>
            <ContextMenuItem onSelect={() => refreshFileTree(node.path)}>Refresh</ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onSelect={() => createFileInDir(node.path)}>New File</ContextMenuItem>
            <ContextMenuItem onSelect={() => createFolderInDir(node.path)}>New Folder</ContextMenuItem>
          </>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}
```

---

## 8. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/components/workspace/WorkspaceLayout.tsx` | Mount `<ExplorerPanel />` trong left panel |

---

## 9. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `workspace.refreshFileTree` | `{ projectId, dirPath? }` | `FileNode` |
| `workspace.readFile` | `{ projectId, path }` | `string` |
| `workspace.searchFiles` | `{ projectId, query, limit? }` | `string[]` |
| `workspace.deleteFile` | `{ projectId, path }` | `void` |
| `workspace.createFile` | `{ projectId, path, content? }` | `void` |
| `workspace.createDir` | `{ projectId, path }` | `void` |
| `workspace.renameFile` | `{ projectId, oldPath, newPath }` | `void` |

---

## 10. Test Plan

```
src/renderer/src/components/workspace/__tests__/
├── ExplorerPanel.test.tsx         (6 tests)
│   ├── renders root directory name
│   ├── expands directory on click
│   ├── lazy loads children when dir expanded
│   ├── collapses dir on second click
│   ├── shows offline message when isOffline
│   └── refresh button calls refreshFileTree
├── FileTreeNode.test.tsx          (5 tests)
│   ├── directory: shows ChevronRight when collapsed
│   ├── directory: shows ChevronDown when expanded
│   ├── file: shows file icon (no chevron)
│   ├── file: shows size when available
│   └── context menu shows Copy Path
├── FileViewer.test.tsx            (4 tests)
│   ├── fetches file content on mount
│   ├── shows loading state while fetching
│   ├── shows error message on fetch failure
│   └── copy button copies content to clipboard
└── hooks/__tests__/useFileExplorer.test.ts  (5 tests)
    ├── toggleDir: expand → loads children
    ├── toggleDir: collapse → removes from expanded set
    ├── agent.complete event → refreshFileTree called
    ├── files.changed event → refreshes parent dirs only
    └── searchQuery change triggers file search debounced
```

**Target:** ≥ 20 tests
