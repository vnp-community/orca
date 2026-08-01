# BUG-TM-002 [BACKEND]: `WorkspaceService.initWorkspace` gọi `relay.call('git.exec', {...})` nhưng relay dispatch chỉ nhận `git.exec` với `args` array — payload mismatch

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TM-001  
**Note:** resolveTerminalStartupCwd(): relative paths resolved against worktreeRoot  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/main/workspace/WorkspaceService.ts:83`:
```typescript
relay.call('git.exec', { args: ['status', '--porcelain=v2', '--branch'] })
relay.call('git.exec', { args: ['worktree', 'list', '--porcelain'] })
```

`src/relay/agent-rpc-dispatch.ts:207`:
```typescript
case 'git.exec': {
  // Cần kiểm tra expected payload schema
}
```

`src/relay/git-exec-validator.ts` — có validator cho `git.exec`. Cần verify payload format match.

**Thêm**: `relay.call('fs.readDir', { path: '.', depth: 2 })` — path là `.` (relative). Dev Server cần biết `cwd` để resolve path tương đối → nhưng không có `cwd` trong payload.

## Chi tiết `fs.readDir` với relative path

`relay.call('fs.readDir', { path: '.', depth: 2 })` tại line 93:
- Dev Server nhận `path: '.'` → resolve thành gì? Working directory của relay process?
- Không có `cwd` context → không biết resolve về repo path
- Phải là `{ path: project.repoPath, depth: 2 }` để đúng

## Fix đề xuất

```typescript
// WorkspaceService.ts line 92-95:
// ❌ BAD:
relay.call('fs.readDir', { path: '.', depth: 2 })

// ✅ CORRECT:
const project = await this.router.getProject(projectId)
relay.call('fs.readDir', { path: project.repoPath, depth: 2 })
```

## Files liên quan

- `src/main/workspace/WorkspaceService.ts:93`: relative path issue
- `src/relay/agent-rpc-dispatch.ts:231`: fs.readDir handler
- `src/relay/git-exec-validator.ts`: git.exec validation
