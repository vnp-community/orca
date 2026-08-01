# BUG-WT-001 [BACKEND]: `WorktreeManager` gọi `git.worktree.add` qua relay nhưng relay dispatch handler thiếu `repoPath` parameter

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WT-001  
**Note:** WorkspaceService.ts: git.exec worktree → git.worktree.list API  

## Mức độ: 🔴 HIGH

## Tóm tắt

Flow BL-WT-01 gọi `relay.call('git.worktree.add', { repoPath, branch, worktreePath })`.

Kiểm tra `src/relay/agent-rpc-dispatch.ts:341`:
```typescript
case 'git.worktree.add': {
  // Handler có tồn tại ✅
  // Cần kiểm tra payload schema và cwd handling
}
```

Tuy nhiên, `WorkspaceService.ts` gọi relay.call với `{ args: ['worktree', 'list', '--porcelain'] }` (line 88) — dùng format `git.exec` với args, không phải `git.worktree.list` method.

**Mismatch**: Một số code dùng `relay.call('git.worktree.list', {...})` và một số khác dùng `relay.call('git.exec', { args: ['worktree', 'list'] })`.

`WorkspaceService.ts:88`:
```typescript
relay.call('git.exec', { args: ['worktree', 'list', '--porcelain'] })
// → dùng git.exec variant
```

Nhưng relay dispatch có cả:
- `case 'git.exec'` (line 207) — generic git command
- `case 'git.worktree.list'` (line 330) — specific handler
- `case 'git.worktree.add'` (line 341) — specific handler

**API không nhất quán** giữa WorkspaceService và các flow code.

## Fix đề xuất

Thống nhất sử dụng specific methods hoặc `git.exec`:

Option A — Dùng specific methods (recommended):
```typescript
relay.call('git.worktree.list', { repoPath: project.repoPath })
relay.call('git.worktree.add', { repoPath, branch, worktreePath })
```

Option B — Dùng git.exec với cwd:
```typescript
relay.call('git.exec', { cwd: project.repoPath, args: ['worktree', 'list', '--porcelain'] })
```

## Files liên quan

- `src/main/workspace/WorkspaceService.ts:88`: git.exec vs git.worktree.list
- `src/relay/agent-rpc-dispatch.ts:330,341`: hai handlers khác nhau
