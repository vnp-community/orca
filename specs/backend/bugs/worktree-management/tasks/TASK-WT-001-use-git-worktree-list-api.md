# TASK-WT-001: Fix WorkspaceService dùng git.worktree.list thay vì git.exec

**Priority:** 🔴 HIGH — API inconsistency, git.exec worktree không reliable  
**Effort:** ~15 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-WT-001  
**Solution ref:** [SOLUTION-worktree-exact.md](../solutions/SOLUTION-worktree-exact.md)

---

## Mục tiêu

Thay thế `relay.call('git.exec', { args: ['worktree', 'list', '--porcelain'] })` bằng `relay.call('git.worktree.list', ...)` đã có sẵn trong relay dispatch.

## Bước 1 — Tìm vị trí

```bash
grep -rn "git.exec.*worktree\|worktree.*git.exec" src/main/ --include="*.ts" | head -10
grep -rn "git.worktree.list\|worktree.list" src/main/ --include="*.ts" | head -10
```

## Bước 2 — File cần sửa

Sửa file tìm được ở Bước 1 (thường là `WorkspaceService.ts`):

```typescript
// TRƯỚC (dùng git.exec):
const raw = await relay.call('git.exec', {
  cwd:  project.repoPath,
  args: ['worktree', 'list', '--porcelain'],
})
// Sau đó parse raw porcelain output...

// SAU (dùng git.worktree.list — relay dispatch case đã có ở line 330):
const result = await relay.call('git.worktree.list', {
  repoPath: project.repoPath,
}) as { worktrees: Array<{ path: string; branch: string; head: string; bare?: boolean }> }

const worktrees = result.worktrees
```

## Verification

```bash
# Verify git.worktree.list exists in relay:
grep -n "git.worktree.list" src/relay/agent-rpc-dispatch.ts
# Expected: case 'git.worktree.list':

pnpm tsc --noEmit
```
