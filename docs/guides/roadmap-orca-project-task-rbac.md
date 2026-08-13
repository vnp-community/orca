# Roadmap: sửa bug + hoàn thiện Project/Task/RBAC/Orchestration

**Cập nhật:** 2026-08-13 (viết lại toàn bộ sau audit `backend/`+`agent/` —  xem
[audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md))

> Bản gốc của roadmap này (cùng ngày, trước audit) giả định phần lớn backend "chưa xây" — sai.
> Bức tranh thật: **hầu hết backend đã xây xong, đang chạy** — vấn đề là các bug wiring/RPC-name
> cụ thể, và vài cụm UI mồ côi. Điều này làm scope NHỎ HƠN nhiều so với roadmap gốc, nhưng đổi
> hình dạng: ít "xây mới", nhiều "sửa đúng chỗ".

## Cấu trúc roadmap mới

```
Nhóm 1 — SỬA NGAY (bug cơ học, không cần quyết định, rủi ro thấp)
   ↓ độc lập với nhau, có thể làm song song, làm trước tất cả
Nhóm 2 — QUYẾT ĐỊNH (xem decisions-needed.md — không tự triển khai)
   ↓
Nhóm 3 — XÂY MỚI (chỉ làm sau khi Nhóm 2 có câu trả lời)
```

## Nhóm 1 — Sửa ngay (an toàn, cơ học, không cần quyết định)

Toàn bộ đây là sửa lỗi cụ thể đã audit ra — không phải thiết kế mới, rủi ro thấp, có thể làm
độc lập từng cái. Xem chi tiết từng case ở
[fix-proposals-per-issue.md](./fix-proposals-per-issue.md).

| # | Việc | Nơi sửa | File chính |
|---|---|---|---|
| 1.1 | `AutomationService` chưa từng khởi tạo trong backend — scheduler không chạy | Backend | `server-bootstrap.ts` (thêm ~5 dòng theo pattern `desktop/src/main/index.ts:1810`) |
| 1.2 | `project.agentSpawn` luôn lỗi (thiếu tham số đăng ký) | Backend | `server-bootstrap.ts` (đổi thứ tự khởi tạo) |
| 1.3 | `profile.getUser` sai tên (đúng là `profile.getUserProfile`) | Frontend | `useProfile.ts` |
| 1.4 | `profile.listDepts` không tồn tại | Backend (thêm method) | `profile-rpc-handler.ts` |
| 1.5 | Route `/admin` bị chặn, admin SPA không serve được | Backend | `http-server.ts` |
| 1.6 | `DeptProfileAdmin.tsx` không có trong router Admin | Frontend | `AdminApp.tsx` |
| 1.7 | 7 chỗ UI Task gọi sai tên RPC (`tasks.*` → `task.*`) | Frontend | `TaskPromptEditor.tsx`, `TaskDetail.tsx`, `useTask.ts` |
| 1.8 | `WorkspaceContext` gọi `git.status`/`workspace.listFiles` sai contract | Frontend | `WorkspaceContext.tsx` (`workspace.listFiles`→`workspace.refreshFileTree` đổi được ngay; `git.status` cần `worktreeId` thật, phụ thuộc mục 2.4) |
| 1.9 | Xoá 3/5 bản `OrcaProfile` chết (0 importer xác nhận) | Frontend | `frontend/src/main/profile/OrcaProfile.ts`, `agent/src/main/profile/OrcaProfile.ts`, `frontend/src/shared/profile-types.ts` |
| 1.10 | 2 bản `OrcaTask`/`task-types.ts` lệch dùng lẫn trong 1 cụm | Frontend | migrate `TaskDetail.tsx`/`TaskPromptEditor.tsx`/`TaskAIDecompose.tsx`/`TaskDAGView.tsx` sang `@shared/task-types` |

**Điều kiện hoàn thành Nhóm 1**: mỗi mục có 1 test xác nhận cụ thể — ví dụ 1.1: tạo 1 automation
với `rrule` gần (vài phút tới), xác nhận nó tự chạy đúng giờ trên server thật; 1.2: gọi
`project.agentSpawn` xác nhận không còn throw; 1.7: gọi từng RPC method Task qua script, xác
nhận không còn "method not found".

## Nhóm 2 — Quyết định trước khi làm tiếp

Không tự triển khai — xem đầy đủ bối cảnh/phương án ở
[decisions-needed.md](./decisions-needed.md):

1. F38 Workspace: hoàn thiện hay dọn dẹp cụm ~32 file mồ côi?
2. `WorkspaceContextV6`: có kế hoạch dùng, hay xoá?
3. Rule merge multi-team cho profile cascade.
4. `TaskGrantService` vs `ProjectMember`: giữ tách biệt hay hợp nhất?
5. `OrcaTask.execute`: luôn chạy đơn, hay có thể chạy qua Orchestration coordinator?
6. Field `integrations`/`fleet`/`security.require2FA`: thêm vào backend hay bỏ khỏi frontend?
7. `OrcaProject` cross-user sharing: cần review bảo mật riêng trước khi thiết kế.
8. Nguồn `currentWorktree` cho Workspace (chỉ liên quan nếu mục 1 = "hoàn thiện F38").

## Nhóm 3 — Xây mới (sau khi Nhóm 2 có câu trả lời)

### 3.1 Team entity (phụ thuộc quyết định #3)

Bảng metadata `Team` (tái dùng `orca_team_members` đã có), RPC `team.create/addMember/
removeMember/list`, `departmentId` trên `OrcaUser` (tận dụng cột `department_id` đã có sẵn trên
`orca_users`). Chi tiết:
[user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục 5.2.

### 3.2 `OrcaProject` sharing layer (phụ thuộc quyết định #7 — review bảo mật trước)

Bảng `OrcaProjectSourceProject`, API đọc-chéo-user có kiểm tra quyền. Chi tiết:
[terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
mục "Đề xuất: OrcaProject là lớp SỞ HỮU + CHIA SẺ".

### 3.3 Pipeline Source→Plan→Execute cho Task (phụ thuộc quyết định #5)

Thêm field `worktreeId`/`agentSessionId`/`workflowExecutionId` vào `OrcaTask`, nối
`task.execute`/`orchestration.run` với `OrcaTask`, ghi kết quả ngược. Chi tiết:
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2/9.4.

### 3.4 F38 Workspace hoàn thiện (phụ thuộc quyết định #1, #2, #8)

Nối tab Agent/terminal, ghép `ServerStatusBar`, mount `ProjectSwitcher`/`WorkspaceLayout` vào
layout thật — chỉ sau khi Nhóm 1 (đặc biệt 1.8) đã xong. Chi tiết:
[project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) mục 4.

### 3.5 UI cho Task graph (Tree/Board/Graph view F37)

Chỉ làm sau 3.3, dùng đúng RPC method đã sửa ở Nhóm 1 — tránh lặp lỗi "tab bấm vào trống" như
đã thấy với F38's Agent tab và `TaskGraphPanel`'s stub.

## Việc có thể bắt đầu ngay hôm nay — không cần chờ gì

Toàn bộ Nhóm 1 (10 mục) — tất cả đều là sửa lỗi cụ thể, không phụ thuộc quyết định nào, có thể
làm song song và độc lập từng mục.
