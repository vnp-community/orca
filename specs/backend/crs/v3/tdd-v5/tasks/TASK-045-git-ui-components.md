# TASK-045: Git UI React Components

**Phase:** 7 — Workspace + Remote Git  
**Solution ref:** [SOL-V5-007](../solutions/SOL-V5-007-remote-git-ui.md) §4  
**Prerequisite:** TASK-042 (WorkspaceContext), TASK-044 (git-remote RPC)  
**Status:** ✅ DONE

---

## Files cần tạo

### `src/renderer/src/components/workspace/git/GitPanel.tsx`

Main git panel container:
- Uses `useWorkspace()` for gitStatus, currentWorktree
- 4 tabs: **Changes** | **History** | **Branches** | **Worktrees**
- Calls `workspace.refreshGitStatus` on tab switch

```tsx
export function GitPanel({ projectId }: { projectId: string }) { ... }
```

### `src/renderer/src/components/workspace/git/CommitForm.tsx`

Staged/unstaged changes + commit:
- File list with checkboxes (stage/unstage via `git.add`, `git.restore`)
- Staged diff preview
- AI commit message button → `rpc.call('git.generateCommitMessage', { projectId, worktreePath })`
- Commit button → `rpc.call('git.commit', { projectId, worktreePath, message })`
- After commit: `workspace.emit({ type: 'git.commit', ... })`

### `src/renderer/src/components/workspace/git/DiffViewer.tsx`

Unified diff viewer:
- Syntax-highlighted line-by-line diff
- Toggle: staged vs unstaged
- Calls: `rpc.call('git.diff', { projectId, worktreePath, file, staged })`

### `src/renderer/src/components/workspace/git/BranchManager.tsx`

Branch operations:
- List branches (local + remote)
- Create new branch
- Delete branch (with force option for unmerged)
- Checkout branch
- Push with streaming progress display
- Pull with streaming progress display

### `src/renderer/src/components/workspace/git/PullRequestForm.tsx`

PR creation:
- Title, body, base branch
- AI PR description → `rpc.call('git.generatePRDescription', ...)`
- Submit → `rpc.call('git.pr.create', { projectId, worktreePath, title, body, base, draft })`
- Show PR URL after creation

---

## Acceptance Criteria

- [x] `GitPanel.tsx` renders với 4 tabs
- [x] `CommitForm.tsx`: file list, stage/unstage, AI button, commit button
- [x] `DiffViewer.tsx`: staged/unstaged toggle, syntax highlighting
- [x] `BranchManager.tsx`: list + create + delete + checkout + push/pull
- [x] `PullRequestForm.tsx`: title + body + AI + submit
- [x] All components use `useWorkspace()` cho state
- [x] TypeScript types đúng
- [x] Không TypeScript errors
