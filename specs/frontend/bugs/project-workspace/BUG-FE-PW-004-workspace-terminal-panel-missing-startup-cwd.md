# BUG-FE-PW-004: `WorkspaceTerminalPanel` không truyền `startupCwd` — terminal mở trong Project Workspace (Beta) không tự `cd` vào folder worktree

## Mức độ: 🟡 MEDIUM

## Trạng thái: ✅ Fixed & deployed (2026-09-15).

## Tóm tắt

User report: sau khi tạo worktree thành công trong "Project Workspace (Beta)", mở tab Terminal của worktree đó thì terminal **không tự đứng ở đúng folder làm việc của worktree** (không cd vào working directory).

## Root Cause — CONFIRMED

`frontend/src/renderer/src/components/workspace/WorkspaceTerminalPanel.tsx`:

```tsx
useEffect(() => {
  if (tabIds.length === 0) {
    createNewTerminalTab(worktreeId)   // ← không có options, không có startupCwd
  }
}, [worktreeId, tabIds.length])
```

`createNewTerminalTab` (`components/terminal/terminal-tab-actions.ts`) có chữ ký:
```ts
export function createNewTerminalTab(
  activeWorktreeId: string | null,
  shellOverride?: string,
  options?: { startupCwd?: string }
): void
```

`options.startupCwd`, nếu có, được truyền xuống `state.createTab(...)`/`createWebRuntimeSessionTerminal(...)` để làm cwd khởi động cho PTY. `WorkspaceTerminalPanel.tsx` gọi hàm này với **đúng 1 tham số** (`worktreeId`) — `shellOverride`/`options` đều `undefined` → `startupCwd` luôn rỗng.

Grep toàn bộ codebase xác nhận: **đây là nơi DUY NHẤT** gọi `createNewTerminalTab` mà không truyền `startupCwd`. Nơi khác duy nhất còn lại:
```ts
// FileExplorer.tsx:588
createNewTerminalTab(activeWorktreeId, undefined, { startupCwd: node.path })
```
— luôn truyền path cụ thể.

`WorkspaceLayout.tsx` (component cha, render `WorkspaceTerminalPanel`) đã có sẵn `currentWorktree` (từ `useWorkspace()`) — kiểu `Worktree`, có field `.path` (xác nhận qua `WorkspaceContext.tsx:62`, `path: node.path,`) — nhưng **không truyền path này xuống** `WorkspaceTerminalPanel`, và `WorkspaceTerminalPanel` cũng không nhận prop nào cho việc đó (chỉ nhận `worktreeId`).

## Fix direction (chưa implement)

1. `WorkspaceTerminalPanel` nhận thêm prop `worktreePath: string` (hoặc đọc trực tiếp `currentWorktree` qua `useWorkspace()` thay vì chỉ nhận `worktreeId`).
2. Gọi `createNewTerminalTab(worktreeId, undefined, { startupCwd: worktreePath })`, đúng pattern `FileExplorer.tsx` đã dùng.
3. `WorkspaceLayout.tsx` (dòng gọi `<WorkspaceTerminalPanel worktreeId={currentWorktree.id} />`) cần truyền thêm `worktreePath={currentWorktree.path}`.

Cần chạy `gitnexus impact({target: "WorkspaceTerminalPanel"})` trước khi sửa (theo quy tắc bắt buộc của repo) — chưa chạy, để dành cho lượt fix thực tế.

## Liên quan

- Phát hiện cùng lúc với [BUG-011](../../../backend-go/bugs/missing-v2/BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md) khi test end-to-end luồng tạo worktree trong "Project Workspace (Beta)" cho repo "aiops-v3".
- Cùng file header comment ghi rõ: "bridges the Workspace terminal panel (F38) to the app's real terminal-pane/PTY infra, instead of a separate PTY stack" — tính năng còn mới (F38), nhiều khả năng đây là 1 chi tiết bị bỏ sót khi tích hợp nhanh, không phải regression từ code cũ.
