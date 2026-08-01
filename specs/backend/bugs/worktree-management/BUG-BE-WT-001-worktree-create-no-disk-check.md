# BUG-BE-WT-001: `worktree.create` không check disk space trước khi tạo — thiếu business rule BR-WT-01

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WT-001  
**Note:** WorkspaceService.ts: disk check before worktree creation  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-WT-01) mô tả bước validate khi tạo worktree:
```
[Main Process — WorktreeManager.create()]
    ├─ Validate path không xung đột (BR-WT-01, BR-WT-03)
    ├─ Check disk space > 100MB
    ├─ git worktree add <path> <baseRef>
```

Nhưng `worktree.create` handler trong `worktree.ts` gọi thẳng `runtime.createManagedWorktree(...)` **không có bất kỳ disk space check nào trước đó**:

```typescript
// worktree.ts:73-145
handler: async (params, { runtime }) => {
  const repo = await runtime.showRepo(params.repo)
  // ...
  const result = await runtime.createManagedWorktree({...})
  // ← THIẾU: check disk space > 100MB
  // ← THIẾU: explicit path conflict validation
}
```

## File liên quan

- [`src/main/runtime/rpc/methods/worktree.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/runtime/rpc/methods/worktree.ts) — Lines 70-145

## Ảnh hưởng

1. User có thể tạo worktree khi disk gần đầy → git worktree fail giữa chừng với cryptic error.
2. `BR-WT-01` (path conflict check) không được enforce ở layer RPC handler.
3. Khi disk đầy, partial git worktree state có thể bị left behind.

## Cách fix đề xuất

Thêm disk space check trước `createManagedWorktree`:
```typescript
// Trước khi gọi createManagedWorktree:
const repoPath = repo.path
const diskInfo = await getDiskSpaceBytes(repoPath)
if (diskInfo.available < 100 * 1024 * 1024) {  // 100MB
  throw new Error(`Insufficient disk space: ${Math.round(diskInfo.available / 1024 / 1024)}MB available, 100MB required`)
}
```

## Liên quan đến luồng

- **BL-WT-01**: Validate step — disk space check missing.
