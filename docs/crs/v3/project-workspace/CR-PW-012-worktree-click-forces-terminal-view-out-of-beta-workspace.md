# CR-PW-012 — Chọn worktree trong sidebar khi đang ở "Project Workspace (Beta)" bị đẩy thẳng ra Terminal view

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-012 |
| **Tên** | `activateAndRevealWorktree`'s step 2 luôn force `activeView` về `'terminal'` — không biết tới view `'workspace'` (Beta) mới thêm sau này |
| **Loại** | Bug Fix |
| **Priority** | 🔴 P0 — vô hiệu hoá hoàn toàn tương tác tự nhiên với Project Workspace: bất kỳ click chọn worktree nào (kể cả để xem tab Git vừa fix xong ở CR-PW-010/011) đều đẩy user ra khỏi trang |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | ✅ Root cause CONFIRMED, ✅ Đã fix + test (3 test mới, 34/34 pass, toàn bộ `lib/` 2509 test pass), ✅ **deployed** (`2026.09.15-worktree-workspace-view-fix`) |
| **Tác giả** | Điều tra theo yêu cầu "workspace project select worktree lại ra terminal, xem lại toàn bộ phần workspace" |
| **Tác động HLD** | `frontend/src/renderer/src/lib/worktree-activation.ts` (`activateAndRevealWorktree`) |
| **Tác động Features** | Project Workspace (Beta) — toàn bộ luồng chọn worktree để xem Git/Tasks/Workflows/Agent tab |

---

## Bối cảnh & Vấn đề gốc

**Chuỗi lỗi** (đọc trực tiếp source, có dẫn chứng dòng):

1. `WorktreeCard.tsx:878-887`'s `handleClick` → `activateWorktreeFromSidebar(worktree.id)`
2. `sidebar-worktree-activation.ts:50` → `activateAndRevealWorktree(worktreeId, { revealInSidebar: false })`
3. `worktree-activation.ts:321-324`:
   ```js
   // 2. Switch any non-terminal view back to terminal
   if (state.activeView !== 'terminal') {
     state.setActiveView('terminal')
   }
   ```
   Comment ở dòng 352 xác nhận đây là "invariant" có chủ đích: *"activateAndRevealWorktree always ends in 'terminal' view"*.

Sidebar (`<Sidebar>`/`WorktreeList`) render **không điều kiện** trong `App.tsx` (~dòng 2385/2406), bất kể `activeView` đang là gì — nên nó vẫn hiển thị và bấm được khi user đang ở trang "Project Workspace (Beta)" (`App.tsx:323-330`, `activeView === 'workspace'`, vào qua nút nav ở `SidebarNav.tsx:172-190`).

**Kết quả**: user mở "Project Workspace (Beta)" → bấm 1 worktree trong sidebar để xem Git status → bị kéo thẳng ra Terminal view cổ điển — khớp chính xác báo cáo "select worktree lại ra terminal".

Điều này **mâu thuẫn trực tiếp với thiết kế đã ghi trong chính code**: `WorktreeList.tsx:5102-5109` và `GitPanel.tsx:50-52` đều ghi rõ *"Workspace has no picker of its own, it reuses this one [sidebar]"* / *"Workspace intentionally reuses the sidebar's selection as its only picker"* — thiết kế này giả định bấm worktree khi đang ở `'workspace'` view chỉ cập nhật `currentWorktree` tại chỗ, không điều hướng đi đâu cả. **Không ai cập nhật lại step 2 của `activateAndRevealWorktree` khi thêm entry point Beta Workspace.**

Đây KHÔNG liên quan tới các thay đổi backend-go của phiên này (CR-PW-010/011, SOL-013/014) — hoàn toàn là bug state/routing phía frontend, đã tồn tại từ trước, chỉ giờ mới bị lộ rõ vì tab Git giờ đã hoạt động thật (trước đây user có bấm vào cũng không thấy gì khác biệt để nhận ra bug này).

## Giải pháp đề xuất

Sửa hẹp tại `activateAndRevealWorktree`'s step 2: bỏ qua việc force `activeView` về `'terminal'` CHỈ khi đang ở `activeView === 'workspace'` VÀ đây là 1 lần chọn "thường" (không có `startup`/`setup`/`defaultTabs`/`issueCommand` — tức không có việc gì cần mở giao diện terminal thật sự, ví dụ mở worktree mới kèm agent tự khởi động). Các luồng khác (keyboard cycling, tạo worktree từ GitHub, resource-session nav...) đều đi qua CÙNG hàm này nhưng hầu hết đều truyền activation work hoặc chỉ chạy khi đang ở `'terminal'` view sẵn — không bị ảnh hưởng.

## Không thuộc phạm vi CR này

- **BUG-FE-PW-005** (Agent tab trong Beta Workspace hoàn toàn không hoạt động trên web — `window.api.agentOrchestration` không tồn tại) — xác nhận LÀ bug thật (không còn hedge), nhưng là CR riêng, ưu tiên thứ 2 sau CR này.
- Rò rỉ state nhỏ: `WorkspaceTerminalPanel.tsx` mở terminal panel gọi `setActiveTabType('terminal')` lên global store — ảnh hưởng thấp, không sửa trong CR này.

## Liên quan

- [CR-PW-010](./CR-PW-010-git-status-worktree-id-resolver-broken.md) / [CR-PW-011](./CR-PW-011-dispatch-executor-never-relays-to-dev-server.md) — lý do bug này giờ mới đáng chú ý (tab Git giờ đã hoạt động thật)
- BUG-FE-PW-005 — Agent tab chết trên web, phát hiện lại/xác nhận trong cùng đợt điều tra này
