# Quyết định — đã chốt (2026-08-13)

**Cập nhật:** 2026-08-13 (8/8 quyết định đã chốt — giữ file này làm hồ sơ tra cứu bối cảnh/lý do)

> Nguồn: [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md). Giải pháp cho
> các bug không cần quyết định xem [fix-proposals-per-issue.md](./fix-proposals-per-issue.md).
> Kế hoạch thực thi cụ thể theo đúng các quyết định dưới đây xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## 1. F38 Workspace ✅ QUYẾT ĐỊNH: hoàn thiện (release)

Sửa bug RPC-contract, nối tab Agent/terminal, mount `ProjectSwitcher`/`WorkspaceLayout` vào
layout thật. Xem kế hoạch chi tiết ở
[project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) mục 4 và roadmap
mục Nhóm 3.

## 2. `WorkspaceContextV6` ✅ QUYẾT ĐỊNH: có kế hoạch nâng cấp, nhưng KHÔNG động tới trong đợt này

Giữ nguyên `WorkspaceContextV6.tsx`/`WorkspaceContextBridge.ts` — không xoá. Có ý định hoàn
thiện V6 làm bản nâng cấp sau này, nhưng **chưa phải bây giờ**. Hệ quả cho việc hoàn thiện F38
(quyết định #1): tiếp tục dùng V5 (`WorkspaceContext.tsx`, đang được `main.tsx` mount) làm nền
cho mọi việc nối tab/mount layout — không chuyển sang V6 trong đợt này, không xoá V6.

## 3. Rule merge multi-team ✅ QUYẾT ĐỊNH: theo khuyến nghị — phương án (a)

`priority: number` trên `TeamMember`, số cao thắng khi 2 Team user thuộc về xung đột cấu hình.
`_sources` ghi rõ `team:<teamId>` nào đã thắng để audit được. Áp dụng khi xây `Team`/`TeamMember`
ở [user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục 5.2.

## 4. `TaskGrantService` vs `ProjectMember` ✅ QUYẾT ĐỊNH: giữ nguyên 2 hệ tách biệt

Không hợp nhất. `TaskGrantService` (task-level) và `ProjectMember` (project-level) tiếp tục là
2 hệ RBAC độc lập, không chia sẻ code — coi đây là chủ ý kiến trúc, không phải nợ kỹ thuật cần
dọn. Khi xây RBAC cho `OrcaProject` sharing layer (quyết định #7), không cố gắng hợp nhất với
`TaskGrantService`.

## 5. `OrcaTask.execute` ✅ QUYẾT ĐỊNH: hybrid — (a) cho task đơn giản, (b) cho task phức tạp

Chọn tự động **bên trong `TaskAgentExecutor.executeTask()`**, dựa vào dữ liệu đã có sẵn trên
task (không cần field/flag mới do người dùng chọn tay):
- Task **không có** subtask con và **không có** dependency edge nào → đơn giản → (a) chạy đơn
  qua `agentSpawner.spawn()`, như hiện tại.
- Task **có** ít nhất 1 subtask hoặc 1 dependency → phức tạp → (b) seed `orchestration.run` từ
  subtree của task, giao `coordinator.ts` điều phối lead/worker.

Thiết kế đầy đủ (bao gồm field schema mới cần thêm, code rẽ nhánh cụ thể) xem
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2.

## 6. Field Profile UI ✅ QUYẾT ĐỊNH (một phần): `security.require2FA` theo phương án (a) — thêm vào backend

`require2FA` sẽ được thêm vào `backend/src/main/profile/OrcaProfile.ts`'s
`SecurityProfileSection` thật (schema + `ProfileResolver.merge()` phải duyệt qua field này để
không bị "lưu được nhưng chết vĩnh viễn" như hiện tại).

**Còn mở, chưa quyết định**: `integrations.*` (githubOrg/linearWorkspace/prTemplate) và
`fleet.*` (allowedServerTags/defaultConnectionType) — quyết định trên chỉ nêu rõ `require2FA`,
2 section còn lại vẫn cần câu trả lời (thêm vào backend hay bỏ khỏi frontend) trước khi động
vào.

## 7. `OrcaProject` cross-user sharing ✅ QUYẾT ĐỊNH: tiến hành theo thiết kế đã đề xuất (Nhóm 3.2 trong roadmap)

Tiến hành xây theo đề xuất "OrcaProject là lớp SỞ HỮU + CHIA SẺ" ở
[terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md).
**Lưu ý quan trọng vẫn giữ nguyên từ đề xuất gốc**: đây là phần đụng ranh giới cô lập bảo mật
giữa các user — bước đọc-chéo-user (bước 5 trong luồng đã thiết kế) cần review kỹ, viết test xác
nhận rõ ràng cả 2 chiều (thấy đúng phần được share, KHÔNG thấy phần chưa share) trước khi coi là
xong, không chỉ dựa vào code review thông thường.

## 8. Nguồn `currentWorktree` cho Workspace ✅ QUYẾT ĐỊNH: theo khuyến nghị — phương án (a)

Tái dùng cơ chế chọn worktree đã có ở sidebar chính (`WorktreeList.tsx`) — không xây bộ chọn
worktree riêng cho Workspace. Cần thiết kế cách 2 UI (sidebar + Workspace) đồng bộ lựa chọn khi
triển khai (xem [project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md)).
