# TASK-FE-002: Fix WorkspaceContext fileTree Type

**Task ID:** TASK-FE-002
**Phase:** 0 — Prerequisites
**Priority:** P0
**Solution Ref:** SOL-FE-V6-002 (Section 5), SOL-FE-V6-007 (Section 9)
**Estimated effort:** 20 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Fix a type mismatch in `WorkspaceContext.tsx`: `fileTree` is declared as `FileNode | null` but `ExplorerPanel` uses it as an array (`fileTree.map(...)`). Also verify that `currentWorktree` field exists in the context.

---

## Execution Results

### Step 1: fileTree type analysis

Read `ExplorerPanel.tsx` — it renders:
```tsx
<FileTreeNode node={fileTree} depth={0} ... />
```
**→ `fileTree` is a single root `FileNode` with `children?: FileNode[]`, NOT an array.**

`FileNode` type has:
```typescript
children?: FileNode[]  // lazy loaded
```

**Conclusion: `fileTree: FileNode | null` is CORRECT. Option B applies — no change needed to fileTree type.**

### Step 2: Changes made

**`workspace-types.ts`** — Added `Worktree` type:
```typescript
export type Worktree = {
  id:         string
  path:       string    // absolute path on dev server
  branch:     string
  isMain:     boolean
  createdAt?: number
}
```

**`WorkspaceContext.tsx`** — Added:
1. Import `Worktree` from workspace-types
2. `currentWorktree: Worktree | null` to `WorkspaceContextValue` interface
3. `setCurrentWorktree: (worktree: Worktree | null) => void` action
4. `useState<Worktree | null>(null)` state in Provider
5. Both exposed in context value object

---

## Acceptance Criteria

- [x] `WorkspaceContext.tsx` compiles without TypeScript errors (related to this task)
- [x] `ExplorerPanel.tsx` can iterate `fileTree` without TS errors (uses single root node — correct)
- [x] `WorkspaceContextValue` contains `currentWorktree: Worktree | null`
- [x] `WorkspaceContextValue` contains `fileTree: FileNode | null` (correct — single root node)
- [x] `refreshFileTree()` correctly updates the fileTree state

---

## Output

```
fileTree type: FileNode | null (correct — ExplorerPanel renders single root node, not array)
Worktree type: ADDED to workspace-types.ts
currentWorktree: ADDED to WorkspaceContextValue
setCurrentWorktree action: ADDED
TypeScript errors (task scope): 0
```
