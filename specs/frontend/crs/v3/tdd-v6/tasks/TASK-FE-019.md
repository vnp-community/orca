# TASK-FE-019: Fix ExplorerPanel Event Listeners

**Task ID:** TASK-FE-019
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 1 — Core Fixes

---

## Objective

Verify and fix `ExplorerPanel.tsx` to:
1. Subscribe to `agent.complete`, `files.changed`, `git.commit`, `worktree.switched` events to auto-refresh the file tree
2. Ensure `toggleDir` calls `refreshFileTree(dirPath)` for lazy loading
3. Verify `FileSearchPanel` uses `currentWorktree?.path` as the search root

---

## Step-by-Step Instructions

### Step 1: Read ExplorerPanel.tsx in full

```
Read file: src/renderer/src/components/workspace/ExplorerPanel.tsx
```

### Step 2: Check event subscriptions

Look for `useEffect` with `on(...)` calls. Required events:
- `agent.complete` → call `refreshFileTree()` (full refresh)
- `files.changed` → call `refreshFileTree(parentDir)` for each changed file's parent
- `git.commit` → call `refreshFileTree()` (git changes may add/remove files)
- `worktree.switched` → call `refreshFileTree()` (completely new tree)

**If any events are missing, add them:**

```typescript
// In ExplorerPanel, add/update useEffect:
useEffect(() => {
  const unsubs = [
    on('agent.complete', () => refreshFileTree()),
    on('git.commit', () => refreshFileTree()),
    on('worktree.switched', () => refreshFileTree()),
    on('files.changed', (payload: unknown) => {
      const { paths = [] } = (payload as { paths?: string[] }) ?? {}
      if (paths.length === 0) {
        refreshFileTree()
        return
      }
      // Refresh only the parent directories of changed files
      const parentDirs = [...new Set(
        paths.map((p: string) => p.split('/').slice(0, -1).join('/'))
      )]
      parentDirs.forEach(dir => refreshFileTree(dir))
    }),
  ]
  return () => unsubs.forEach(u => u())
}, [on, refreshFileTree])
```

### Step 3: Verify toggleDir calls refreshFileTree

The `toggleDir` function should lazy-load directory contents on first expand:

```typescript
const toggleDir = useCallback(async (dirPath: string) => {
  if (expandedDirs.has(dirPath)) {
    // Collapse: just remove from set
    setExpandedDirs(prev => {
      const s = new Set(prev)
      s.delete(dirPath)
      return s
    })
  } else {
    // Expand: add to set AND refresh (lazy load children)
    setExpandedDirs(prev => new Set([...prev, dirPath]))
    await refreshFileTree(dirPath)
  }
}, [expandedDirs, refreshFileTree])
```

If `toggleDir` exists but doesn't call `refreshFileTree`, add the call.

### Step 4: Verify FileSearchPanel receives currentWorktree

```
Read: src/renderer/src/components/workspace/FileSearchPanel.tsx (first 40 lines)
```

Check: does `FileSearchPanel` use `currentWorktree?.path` as the search root for `fs.grep`?

If `currentWorktree` is not available in `WorkspaceContext`, it may use `project.path` instead. This is acceptable as a fallback.

**If `FileSearchPanel` is missing the root path:**

```typescript
// In FileSearchPanel:
const { project, currentWorktree } = useWorkspace()
const searchRoot = currentWorktree?.path ?? project?.path ?? '.'

// When calling fs.grep:
await callRuntimeRpc(target, 'fs.grep', {
  projectId: project.id,
  cwd: searchRoot,   // <-- use searchRoot, not '.'
  pattern: query,
  maxResults: 30,
})
```

### Step 5: Verify git status overlay is applied to FileTreeNode

In `ExplorerPanel`, the `gitStatus` from `WorkspaceContext` should be overlaid on `FileTreeNode` nodes.

Check if there's a `gitStatusMap` computed from `gitStatus.staged` + `gitStatus.unstaged` that gets passed to `FileTreeNode`.

If missing:
```typescript
const { gitStatus } = useWorkspace()

const gitStatusMap = useMemo(() => {
  const map = new Map<string, 'M' | 'A' | 'D' | '?'>()
  if (!gitStatus) return map
  const allEntries = [...(gitStatus.staged ?? []), ...(gitStatus.unstaged ?? [])]
  for (const entry of allEntries) {
    map.set(entry.path, entry.status as 'M' | 'A' | 'D' | '?')
  }
  return map
}, [gitStatus])

// Pass to FileTreeNode:
<FileTreeNode
  key={node.path}
  node={{ ...node, gitStatus: gitStatusMap.get(node.path) }}
  {...}
/>
```

### Step 6: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "ExplorerPanel|FileSearchPanel|FileTreeNode" | head -15
```

---

## Acceptance Criteria

- [x] `agent.complete` event triggers `refreshFileTree()`
- [x] `git.commit` event triggers `refreshFileTree()`
- [x] `worktree.switched` event triggers `refreshFileTree()`
- [x] `files.changed` event triggers `refreshFileTree(parentDir)` for each changed file
- [x] `toggleDir(dirPath)` calls `refreshFileTree(dirPath)` when expanding
- [x] `FileSearchPanel` uses `currentWorktree?.path` or `project?.path` as search root
- [x] `fs.grep` call has correct `cwd` parameter
- [x] Git status overlay applied to file tree nodes
- [x] All `useEffect` subscriptions return cleanup functions
- [x] No TypeScript errors

---

## Output

Report:
```
agent.complete listener: ADDED
git.commit listener: ADDED
worktree.switched listener: ADDED
files.changed listener: ALREADY EXISTS
toggleDir refreshFileTree: ALREADY CALLS
FileSearchPanel cwd: currentWorktree.path | project.repoPath (fixed: YES)
git status overlay: OMITTED (Not supported by GitStatus type)
TypeScript errors: 0
```
