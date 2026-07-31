# TDD-FE-17: Remote File Explorer

**Document:** TDD-FE-17 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** File Explorer — remote directory tree, file viewer, search, context menu
**Feature:** F38
**ADR:** ADR-011
**HLD Ref:** C3.12
**Backend TDD:** TDD-19 (workspace.refreshFileTree)
**Source files (to create):**
- `src/renderer/src/components/workspace/ExplorerPanel.tsx`
- `src/renderer/src/components/workspace/FileTreeNode.tsx`
- `src/renderer/src/components/workspace/FileViewer.tsx`
- `src/renderer/src/components/workspace/FileSearchPanel.tsx`
- `src/renderer/src/hooks/useFileExplorer.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. ExplorerPanel Layout

```
┌─────────────────────────────────────────────────────┐
│ 📁 Explorer  [🔍] [↺]                               │
├─────────────────────────────────────────────────────┤
│ ▼ myapp-backend  (repo root)                        │
│   ▼ src/                                            │
│     ▶ auth/                                         │
│     ▶ db/                                           │
│     ▼ main/                                         │
│       📄 index.ts                                   │
│       📄 server-bootstrap.ts                        │
│     📄 package.json                                 │
│   ▶ deploy/                                         │
│   📄 tsconfig.json                                  │
├─────────────────────────────────────────────────────┤
│ [Search Files...]                                   │
└─────────────────────────────────────────────────────┘
```

---

## 2. ExplorerPanel Component

```typescript
// src/renderer/src/components/workspace/ExplorerPanel.tsx

export function ExplorerPanel() {
  const { project, fileTree, refreshFileTree, isOffline } = useWorkspace()
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set())
  const [searchOpen, setSearchOpen] = useState(false)
  const [viewingFile, setViewingFile] = useState<string | null>(null)

  // Auto-refresh on agent.complete or files.changed
  const { on } = useWorkspace()
  useEffect(() => {
    const unsubs = [
      on('agent.complete', () => refreshFileTree()),
      on('files.changed', ({ paths }) => {
        // Only refresh parent directories of changed files
        const parentDirs = [...new Set(paths.map(p => p.split('/').slice(0, -1).join('/')))]
        parentDirs.forEach(dir => refreshFileTree(dir))
      }),
    ]
    return () => unsubs.forEach(u => u())
  }, [on, refreshFileTree])

  const toggleDir = async (dirPath: string) => {
    if (expandedDirs.has(dirPath)) {
      setExpandedDirs(prev => { const s = new Set(prev); s.delete(dirPath); return s })
    } else {
      setExpandedDirs(prev => new Set([...prev, dirPath]))
      // Lazy load children if not already loaded
      await refreshFileTree(dirPath)
    }
  }

  if (!project) return <div className="p-3 text-sm text-muted-foreground">No project selected</div>

  return (
    <div className="explorer-panel flex flex-col h-full">
      <ExplorerHeader
        onRefresh={() => refreshFileTree()}
        onToggleSearch={() => setSearchOpen(v => !v)}
        isOffline={isOffline}
      />
      {searchOpen ? (
        <FileSearchPanel onSelect={path => setViewingFile(path)} />
      ) : (
        <div className="file-tree flex-1 overflow-y-auto py-1">
          {fileTree.map(node => (
            <FileTreeNode
              key={node.path}
              node={node}
              expandedDirs={expandedDirs}
              onToggleDir={toggleDir}
              onSelectFile={setViewingFile}
              selectedFile={viewingFile}
            />
          ))}
        </div>
      )}
      {viewingFile && (
        <FileViewer
          filePath={viewingFile}
          onClose={() => setViewingFile(null)}
        />
      )}
    </div>
  )
}
```

---

## 3. FileTreeNode Component

```typescript
// src/renderer/src/components/workspace/FileTreeNode.tsx

interface FileTreeNodeData {
  path: string
  name: string
  type: 'file' | 'directory'
  children?: FileTreeNodeData[]
  size?: number
  gitStatus?: 'M' | 'A' | 'D' | '?'  // from gitStatus overlay
}

interface FileTreeNodeProps {
  node: FileTreeNodeData
  depth?: number
  expandedDirs: Set<string>
  onToggleDir: (path: string) => void
  onSelectFile: (path: string) => void
  selectedFile: string | null
}

export function FileTreeNode({ node, depth = 0, ...props }: FileTreeNodeProps) {
  const isDir = node.type === 'directory'
  const isExpanded = props.expandedDirs.has(node.path)
  const isSelected = props.selectedFile === node.path

  const handleClick = () => {
    if (isDir) props.onToggleDir(node.path)
    else props.onSelectFile(node.path)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') handleClick()
  }

  return (
    <>
      <div
        className={cn(
          'file-tree-node flex items-center gap-1 py-0.5 px-2 cursor-pointer text-sm',
          'hover:bg-accent rounded-sm',
          isSelected && 'bg-accent'
        )}
        style={{ paddingLeft: 8 + depth * 16 }}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role={isDir ? 'treeitem' : 'button'}
        aria-expanded={isDir ? isExpanded : undefined}
        aria-label={node.name}
        onContextMenu={e => showContextMenu(e, node)}
      >
        {isDir ? (
          isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />
        ) : (
          <span className="w-3" />
        )}
        <FileIcon filename={node.name} isDir={isDir} size={14} />
        <span className={cn(
          'flex-1 truncate',
          node.gitStatus === 'D' && 'line-through opacity-50'
        )}>
          {node.name}
        </span>
        {/* Git status overlay */}
        {node.gitStatus && (
          <span className={cn('text-xs font-medium ml-auto', {
            'text-yellow-500': node.gitStatus === 'M',
            'text-green-500': node.gitStatus === 'A',
            'text-red-500': node.gitStatus === 'D',
            'text-gray-400': node.gitStatus === '?',
          })}>
            {node.gitStatus}
          </span>
        )}
      </div>

      {/* Children (lazy: only render if expanded and children loaded) */}
      {isDir && isExpanded && node.children?.map(child => (
        <FileTreeNode key={child.path} node={child} depth={depth + 1} {...props} />
      ))}
    </>
  )
}
```

---

## 4. FileViewer — Monaco Read-only

```typescript
// src/renderer/src/components/workspace/FileViewer.tsx

export function FileViewer({ filePath, onClose }: { filePath: string; onClose: () => void }) {
  const { project } = useWorkspace()
  const [content, setContent] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isStreaming, setIsStreaming] = useState(false)

  useEffect(() => {
    if (!project || !filePath) return
    setIsLoading(true)
    setContent('')

    // Large files: stream; small files: direct read
    rpc.call('fs.readFile', {
      projectId: project.id,
      path: filePath,
      encoding: 'utf-8',
      maxSize: 1024 * 1024  // 1MB limit
    }).then(r => {
      setContent((r as any).content)
    }).catch(err => {
      if (err.code === 'FILE_TOO_LARGE') {
        setIsStreaming(true)
        streamLargeFile(filePath)
      }
    }).finally(() => setIsLoading(false))
  }, [filePath, project])

  const language = detectLanguage(filePath)

  return (
    <div className="file-viewer border-t">
      <div className="file-viewer-header flex items-center gap-2 px-3 py-1 bg-muted border-b text-xs">
        <FileCode size={12} />
        <span className="font-mono">{filePath}</span>
        <Button variant="ghost" size="icon" className="h-5 w-5 ml-auto" onClick={onClose}>
          <X size={12} />
        </Button>
      </div>
      {isLoading ? (
        <Skeleton className="h-48" />
      ) : (
        <MonacoEditor
          value={content}
          language={language}
          options={{
            readOnly: true,
            minimap: { enabled: false },
            fontSize: 12,
            scrollBeyondLastLine: false,
            wordWrap: 'on',
          }}
          theme="vs-dark"
          height={300}
        />
      )}
    </div>
  )
}
```

---

## 5. FileSearchPanel

```typescript
// src/renderer/src/components/workspace/FileSearchPanel.tsx

export function FileSearchPanel({ onSelect }: { onSelect: (path: string) => void }) {
  const { project, currentWorktree } = useWorkspace()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<FileSearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)

  // Debounced search — 300ms
  useEffect(() => {
    if (!query.trim() || query.length < 2) { setResults([]); return }
    const timer = setTimeout(async () => {
      if (!project || !currentWorktree) return
      setIsSearching(true)
      try {
        const result = await rpc.call('fs.grep', {
          projectId: project.id,
          root: currentWorktree.path,
          pattern: query,
          maxResults: 50,
          includeContent: true,
        }) as FileSearchResult[]
        setResults(result)
      } finally {
        setIsSearching(false)
      }
    }, 300)
    return () => clearTimeout(timer)
  }, [query, project, currentWorktree])

  return (
    <div className="file-search p-2 flex-1 overflow-y-auto">
      <div className="relative">
        <Search size={14} className="absolute left-2 top-2.5 text-muted-foreground" />
        <Input
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search in files..."
          className="pl-7 text-sm"
          autoFocus
        />
        {isSearching && <Loader2 className="absolute right-2 top-2.5 animate-spin" size={14} />}
      </div>
      <div className="results mt-2 space-y-1">
        {results.map(r => (
          <SearchResultItem key={`${r.file}:${r.line}`} result={r} onSelect={onSelect} />
        ))}
        {results.length === 0 && query.length >= 2 && !isSearching && (
          <p className="text-xs text-muted-foreground text-center py-4">No results for "{query}"</p>
        )}
      </div>
    </div>
  )
}
```

---

## 6. Context Menu

```typescript
// Right-click on file/folder shows context menu:

function showContextMenu(e: React.MouseEvent, node: FileTreeNodeData) {
  e.preventDefault()
  // Context menu items:
  if (node.type === 'file') {
    // [View File]
    // [Copy Path]
    // [Copy Relative Path]
    // [Open in New Worktree] → git worktree add
    // ─────────────────────
    // [Run Agent Here] → spawn agent with worktreePath = parent dir
  } else {
    // [Open in Terminal]
    // [Copy Path]
    // [Run Agent Here] → spawn agent with worktreePath = dir
    // ─────────────────────
    // [New File]
    // [New Folder]
  }
}
```

---

## 7. Test Coverage

```
src/renderer/src/components/workspace/__tests__/
├── ExplorerPanel.test.tsx
│   ├── renders file tree from fileTree context
│   ├── toggleDir expands directory + calls refreshFileTree(path)
│   ├── selecting file opens FileViewer
│   ├── agent.complete event → refreshFileTree() called
│   ├── files.changed event → refreshFileTree(parentDir) called
│   └── search toggle shows FileSearchPanel
├── FileTreeNode.test.tsx
│   ├── directory: shows expand/collapse chevron
│   ├── file: shows file icon, no chevron
│   ├── selected file → bg-accent class
│   ├── gitStatus 'M' → yellow M indicator
│   ├── gitStatus 'D' → line-through + red D
│   ├── keyboard Enter → toggles expand or selects
│   └── children rendered when expanded
├── FileViewer.test.tsx
│   ├── fetches file content on mount
│   ├── shows skeleton while loading
│   ├── FILE_TOO_LARGE → streaming mode
│   └── close button calls onClose
└── FileSearchPanel.test.tsx
    ├── debounces search 300ms
    ├── query < 2 chars → no search
    ├── shows results with file path + line
    └── selecting result calls onSelect
```

**Target:** ≥ 30 tests

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.12, C3.12c](../../../docs/hld/v1/C3-components.md), [HLD C4.10](../../../docs/hld/v1/C4-code.md), [web-server-architecture.md §10.8](../../../docs/hld/web-server-architecture.md)

### fs.* — Exact Backend Methods (từ HLD C4.10)

```typescript
'fs.readDir'  params: { path, depth: 1 }
              → relay: fs.readdir(path) + fs.stat(entry) per file
              → return: FileEntry[]

'fs.readFile' params: { path, encoding: 'utf-8' }
              → relay: fs.readFile(path, 'utf-8')
              → LIMIT: max 5MB, từ chối nếu vượt
              → return: { content: string, size: number }

'fs.stat'     params: { path }
              → relay: fs.stat(path)
              → return: { size, mtime, isDir, isFile }

'fs.glob'     params: { pattern, cwd, ignore?: string[] }
              → relay: glob(pattern, { cwd, ignore })
              → return: string[]  (relative paths)

'fs.grep'     params: { pattern, cwd, include?: string, maxResults?: number }
              → relay: grep -rn --include=<ext> <pattern> <cwd>
              → LIMIT: tối đa 30 kết quả (configurable)
              → return: GrepResult[] { file, line, content, match }
```

### FileEntry — Data Shape

```typescript
interface FileEntry {
  name: string
  path: string      // absolute path on dev server
  isDir: boolean
  isFile: boolean
  size?: number     // bytes (only for files)
  mtime?: Date
  gitStatus?: 'M' | 'A' | 'D' | 'R' | 'C' | '?'  // overlaid from WorkspaceContext.gitStatus
}
```

### Lazy Expand — Exact Data Flow (từ HLD C3.12c)

```
User expand 📁 src/
    │
    ├── Không cached → RPC: fs.readDir({ path: '/srv/vnp/src', depth: 1 })
    │       Backend → relay → Dev Server: fs.readdir('/srv/vnp/src')
    │       Return: FileEntry[]
    │
    ├── Overlay git status decorations:
    │       gitStatusMap = WorkspaceContext.gitStatus.staged + unstaged
    │       { 'src/auth/auth-manager.ts': 'M', 'src/auth/bcrypt.ts': 'A' }
    │       NOTE: parent folder shows badge if ANY child modified
    │
    ├── Cache expanded state trong expandedDirs (Set<string>)
    │       → Tab switch KHÔNG unmount → state preserved
    │
    └── Render:
          📁 src/
          ├── 📁 auth/       [M]  ← parent decorated
          │   ├── 📄 auth-manager.ts  [M]
          │   └── 📄 bcrypt-utils.ts  [A]
          └── 📄 index.ts
```

### File Viewer — Monaco Read-only (từ HLD)

```typescript
// Click file → FileViewer tab
// 1. RPC: fs.readFile({ path, encoding: 'utf-8' })
// 2. Detect language from extension:
//    .ts/.tsx → typescript | .py → python | .go → go | ...
// 3. Monaco Editor (read-only):
//    editor.updateOptions({ readOnly: true })
//    monaco.editor.setModelLanguage(model, language)
// 4. Tab title: basename(path)
// 5. Multiple tabs: dedup by path (focus existing if already open)

// Max file size: 5MB
// Encoding fallback: nếu binary → hiển thị "Binary file, cannot display"
```

### Git Decoration Update Strategy (từ HLD C3.12b)

```typescript
// Git decorations được overlay từ WorkspaceContext.gitStatus
// KHÔNG fetch riêng trong Explorer

// Refresh triggers:
// 1. switchProject() → initial fetch
// 2. git status poll every 5s → WorkspaceContext.gitStatus updated
// 3. Event: 'agent.complete' → immediate refreshGitStatus()
// 4. Event: 'git.commit' → immediate refreshGitStatus()
// 5. Event: 'worktree.switched' → reload tree + status

// Parent folder decoration rule:
// folder shows badge if ANY descendant has git status
// (traverse up from changed file → mark each parent)
```

### FileSearchPanel — Search Constraints (từ HLD)

```typescript
// Debounce: 300ms
// Min query length: 2 chars
// Max results: 30 (fs.grep limit)
// Search scope: current worktree path (cwd từ WorkspaceContext)

// grep result format:
interface GrepResult {
  file: string      // relative path
  line: number      // 1-indexed
  content: string   // full line content
  match: string     // matched substring (for highlight)
}

// Click result → open FileViewer tab + scroll to line
// Monaco: editor.revealLineInCenter(lineNumber)
//         editor.setPosition({ lineNumber, column: matchColumn })
```
