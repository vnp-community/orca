# SOL-FE-V6-007: Remote File Explorer (TDD-FE-17)

**Solution ID:** SOL-FE-V6-007
**TDD Ref:** [TDD-FE-17](../../../../tdd/v5/17-file-explorer-ui.md)
**Feature:** F38 | **ADR:** ADR-011 | **HLD Ref:** C3.12, C3.12c
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/workspace/ExplorerPanel.tsx` | 2092 bytes | Co san — day du structure |
| `components/workspace/FileTreeNode.tsx` | 2698 bytes | Co san — day du |
| `components/workspace/FileViewer.tsx` | 3169 bytes | Co san — can kiem tra Monaco |
| `components/workspace/FileSearchPanel.tsx` | 2218 bytes | Co san — day du debounce |
| `components/workspace/FileContextMenu.tsx` | 2240 bytes | Co san — day du |
| `hooks/useFileExplorer.ts` | 2237 bytes | Co san — day du |

> **Dac biet:** TDD-FE-17 la module co nhieu code hien co NHAT. Chinh sach: CHI can verify va boi sung Monaco trong FileViewer.

### 1.2 Can kiem tra

| File | Van de |
|------|-------|
| `components/workspace/FileViewer.tsx` | Co the chua tich hop @monaco-editor/react |
| `components/workspace/ExplorerPanel.tsx` | Kiem tra event listeners cho agent.complete, files.changed |

---

## 2. Giai phap — FileViewer Monaco Integration

**KIEM TRA:** `FileViewer.tsx` (3169 bytes)

**Neu FileViewer.tsx chua su dung Monaco, bo sung:**

```typescript
// MODIFY: src/renderer/src/components/workspace/FileViewer.tsx
// Bo sung Monaco Editor (read-only) thay cho <pre> text display

import Editor from '@monaco-editor/react'

// Thay the phan hien thi content cu:
// Truoc (likely):
// <pre className="text-xs font-mono p-3 overflow-auto">{content}</pre>

// Sau (Monaco):
<Editor
  value={content}
  language={language}
  options={{
    readOnly: true,
    minimap: { enabled: false },
    fontSize: 12,
    scrollBeyondLastLine: false,
    wordWrap: 'on',
    lineNumbers: 'on',
    renderWhitespace: 'none',
    contextmenu: false,
  }}
  theme="vs-dark"
  height={300}
/>
```

**FileViewer da phai co:**

```typescript
// Lazy import de tranh load Monaco khi khong can:
const Editor = lazy(() => import('@monaco-editor/react').then(m => ({ default: m.Editor })))

// detectLanguage helper:
const LANG_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript',
  js: 'javascript', jsx: 'javascript',
  py: 'python', go: 'go', rs: 'rust',
  // ...
}
function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANG_MAP[ext] ?? 'plaintext'
}
```

---

## 3. Giai phap — ExplorerPanel Event Integration

**KIEM TRA:** `ExplorerPanel.tsx` (2092 bytes)

**TDD-FE-17 yeu cau auto-refresh khi:**
1. `agent.complete` event — refresh toan bo tree
2. `files.changed` event — refresh chi parent dirs cua changed files

**Kiem tra co trong ExplorerPanel.tsx chua:**

```typescript
// Nen co trong ExplorerPanel:
useEffect(() => {
  const unsubs = [
    on('agent.complete', () => refreshFileTree()),
    on('files.changed', ({ paths }: { paths: string[] }) => {
      const parentDirs = [...new Set(paths.map(p => p.split('/').slice(0, -1).join('/')))]
      parentDirs.forEach(dir => refreshFileTree(dir))
    }),
    on('git.commit', () => refreshFileTree()),      // Sau commit, refresh tree
    on('worktree.switched', () => refreshFileTree()), // Sau switch worktree
  ]
  return () => unsubs.forEach(u => u())
}, [on, refreshFileTree])
```

**Gap co the co:** `on()` type signature trong TDD dung `(event, handler)` voi payload la object. Kiem tra WorkspaceContext.tsx co matches khong.

---

## 4. Giai phap — FileContextMenu Verification

**KIEM TRA:** `FileContextMenu.tsx` (2240 bytes)

**TDD-FE-17 yeu cau context menu co:**

**File:**
- View File
- Copy Path
- Copy Relative Path
- Open in New Worktree → git worktree add
- Run Agent Here

**Folder:**
- Open in Terminal
- Copy Path
- Run Agent Here
- New File
- New Folder

**Neu chua day du, bo sung:**

```typescript
// Trong FileContextMenu, phan "Run Agent Here":
const runAgentHere = async (path: string) => {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  await callRuntimeRpc(target, 'tasks.runAgent', {
    taskId: undefined,  // no task — free-form agent
    worktreePath: path,
  })
  toast.success('Agent started in ' + path)
}

// "Open in New Worktree" → git worktree add:
const openInWorktree = async (path: string) => {
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  await callRuntimeRpc(target, 'git.worktree.add', {
    projectId: project!.id,
    basePath: path,
  })
}
```

---

## 5. Giai phap — FileSearch + Lazy Load

**KIEM TRA:** `FileSearchPanel.tsx` (2218 bytes)

**TDD-FE-17 yeu cau:**
- Debounce: 300ms
- Min query length: 2 chars
- Max results: 30 (fs.grep limit)
- Scope: current worktree path

**RPC method:** `fs.grep` (theo HLD C4.10)

```typescript
// Verify trong FileSearchPanel.tsx:
const result = await callRuntimeRpc<GrepResult[]>(target, 'fs.grep', {
  projectId: project.id,
  cwd: currentWorktree?.path ?? '.',  // QUAN TRONG: dung currentWorktree
  pattern: query,
  maxResults: 30,
  // include: '*.ts,*.tsx,*.js'  // optional filter by extension
})
```

**Gap co the co:** `currentWorktree` khong co trong WorkspaceContext hien tai (chi co `project`). Can kiem tra va bo sung neu chua co.

---

## 6. Lazy Load Strategy

**FileViewer + Monaco = heavy bundle.** Dam bao lazy loading:

```typescript
// Trong ExplorerPanel.tsx — lazy load FileViewer:
const FileViewer = lazy(() => 
  import('./FileViewer').then(m => ({ default: m.FileViewer }))
)

// Trong WorkspaceLayout.tsx — ExplorerPanel da duoc lazy loaded:
const ExplorerPanel = lazy(() => import('./ExplorerPanel')...)
// => FileViewer se duoc lazy loaded theo cascade
```

---

## 7. Git Decoration Overlay

**TDD-FE-17 Section 3 (FileTreeNode) yeu cau:**
- `node.gitStatus` duoc overlay tu `WorkspaceContext.gitStatus`
- Parent folder hien badge neu ANY child modified

**Giai phap — overlay trong ExplorerPanel (KHONG fetch rieng):**

```typescript
// Trong ExplorerPanel, sau khi load fileTree:
const gitStatusMap = useMemo(() => {
  const map = new Map<string, 'M' | 'A' | 'D' | '?'>()
  if (!gitStatus) return map
  
  const entries = [
    ...gitStatus.staged.map(f => ({ path: f.path, status: f.status })),
    ...gitStatus.unstaged.map(f => ({ path: f.path, status: f.status })),
  ]
  
  for (const entry of entries) {
    map.set(entry.path, entry.status as any)
    // Mark parent directories
    const parts = entry.path.split('/')
    for (let i = 1; i < parts.length; i++) {
      const parentDir = parts.slice(0, i).join('/')
      if (!map.has(parentDir)) {
        map.set(parentDir, 'M')  // parent shows M if any child modified
      }
    }
  }
  return map
}, [gitStatus])

// Truyen gitStatusMap vao FileTreeNode de overlay
```

---

## 8. useFileExplorer Hook

**KIEM TRA:** `hooks/useFileExplorer.ts` (2237 bytes)

**TDD-FE-17 yeu cau:**
```typescript
// useFileExplorer nen tra ve:
{
  expandedDirs: Set<string>
  toggleDir: (path: string) => Promise<void>  // async vi can lazy load
  selectedFile: string | null
  setSelectedFile: (path: string | null) => void
}
```

**Gap co the co:** `toggleDir` chua goi `refreshFileTree(dirPath)` khi expand lan dau.

---

## 9. WorkspaceContext — fileTree Type Issue

**PHAT HIEN:** `WorkspaceContext.tsx` khai bao:
```typescript
fileTree: FileNode | null  // SINGLE node
```

Nhung `ExplorerPanel` dung:
```typescript
fileTree.map(node => ...)  // ARRAY
```

**Giai phap:**

**Option A (recommended):** Doi `fileTree` thanh `FileNode[]` trong WorkspaceContext:
```typescript
// Thay:
fileTree: FileNode | null
// Thanh:
fileTree: FileNode[]
```

**Option B:** `FileNode` co `children: FileNode[]` — `ExplorerPanel` render `fileTree?.children?.map(...)`:
```typescript
{(fileTree?.children ?? []).map(node => (...))}
```

**Lua chon Option A** de nhat quan voi TDD spec.

---

## 10. Test Plan

**Target:** >= 30 tests

```
src/renderer/src/components/workspace/__tests__/
├── ExplorerPanel.test.tsx           (7+ tests)
│   ├── renders file tree root nodes from fileTree
│   ├── toggleDir expands => calls refreshFileTree(path)
│   ├── toggleDir collapse => removes from expandedDirs
│   ├── selecting file opens FileViewer
│   ├── agent.complete event => refreshFileTree called
│   ├── files.changed event => refreshFileTree(parentDir) called
│   └── search toggle shows/hides FileSearchPanel
├── FileTreeNode.test.tsx            (7+ tests)
│   ├── directory: shows ChevronDown when expanded
│   ├── directory: shows ChevronRight when collapsed
│   ├── file: shows file icon, no chevron
│   ├── selected file => bg-accent class applied
│   ├── gitStatus 'M' => yellow M indicator
│   ├── gitStatus 'A' => green A indicator
│   ├── gitStatus 'D' => line-through + red D
│   ├── keyboard Enter => toggleDir or selectFile
│   └── children rendered when expanded
├── FileViewer.test.tsx              (5+ tests)
│   ├── fetches file content on mount via fs.readFile
│   ├── shows Skeleton while isLoading=true
│   ├── Monaco Editor rendered with content
│   ├── FILE_TOO_LARGE error => streaming fallback
│   └── close button calls onClose
└── FileSearchPanel.test.tsx         (5+ tests)
    ├── query < 2 chars => no search triggered
    ├── query >= 2 chars => debounced 300ms => fs.grep called
    ├── shows results with file path + line number
    ├── no results + searched => shows "No results" message
    └── selecting result calls onSelect with path
```

---

## 11. Phu thuoc va Thu tu

**Prerequisite:** `@monaco-editor/react` (chung voi SOL-FE-V6-006)

**Co the implement song song voi SOL-FE-V6-006** vi ca hai dung cung dependency.

**Sau khi implement SOL-FE-V6-007:**
- `WorkspaceLayout` (SOL-FE-V6-002) se render `ExplorerPanel` day du trong left panel
- `ExplorerPanel` tu dong refresh khi git operations thay doi files
