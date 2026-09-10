# Tasks — CR-TG-007 (frontend task-graph v4)

Task thực thi cho
[FE-SOL-001](../solutions/FE-SOL-001-task-crud-board-grant-ui.md) (CR-TG-007), chia theo đúng 5
mảng solution đã thiết kế. FE-SOL-001 tự đính chính lại CR-TG-007 §C4 (Run-Agent prompt) đã được
sửa từ trước (đọc `TaskPromptEditor.tsx` thật xác nhận `prompt` đã gửi đúng qua `task.execute`) —
mục đó **không có task tương ứng ở đây**, đúng ý solution.

| Task ID | Mô tả | Phụ thuộc | Trạng thái cuối |
|---------|-------|-----------|-----------------|
| [FE-TASK-001](./FE-TASK-001-new-task-creation-dialog.md) | `TaskCreateDialog` + nút "+ New Task" trên `TaskGraph` toolbar, gọi `task.create` (RPC đã tồn tại thật) | Không có hard blocker; khuyến nghị backend-go vá song song `createArgs`/`CreateTaskRequest` thiếu `ProjectId` | ✅ DONE — UI + test đầy đủ; end-to-end "task thấy ngay sau khi tạo" chờ backend vá `projectId` (task riêng, đang chạy song song) |
| [FE-TASK-002](./FE-TASK-002-task-dag-view-real-dependencies.md) | `TaskDAGView` đổi nguồn cạnh từ `(task as any).dependsOn` (luôn `[]`) sang `task.getDependencies` thật + UI "+ Add dependency" gọi `task.addEdge` | Không có (cả 2 RPC đã tồn tại thật) | ✅ DONE |
| [FE-TASK-003](./FE-TASK-003-task-board-kanban-view.md) | `TaskBoardView` — Board/Kanban view mới, 7 cột theo `OrcaTask['status']`, kéo-thả gọi `task.update` | Không có (RPC đã tồn tại); phải vá `TaskStatusBadge`'s `STATUS_CONFIG` thiếu 3/7 status trước | ✅ DONE |
| [FE-TASK-004](./FE-TASK-004-task-grant-modal-access-tab.md) | `TaskGrantModal` + tab Access + permission badge, dùng đúng thang `GrantLevel` thật (`owner/admin/user/team/company`) | `task.grant`/`task.resolvePermission` đã tồn tại (Add Grant + badge dùng ngay); list/revoke/share-link chờ BE-SOL-003's RPC mới | ✅ DONE — phần list/revoke/share-link mock/no-op chờ BE-SOL-003 |
| [FE-TASK-005](./FE-TASK-005-use-task-activity-polling-fallback.md) | `useTaskActivity` — áp dụng nguyên thiết kế polling fallback đã có ở flow-task series (không thiết kế lại) | Không có | ✅ DONE |

**🔴 P0: FE-TASK-003** (Board/Kanban view) — gap chính ma trận hoàn thành CR-TG-007 nêu, ưu tiên làm
trước nếu phải chọn 1 task duy nhất trong đợt này.

## Dependency order

Cả 5 task **gần như độc lập với nhau về chức năng** — không có chuỗi cứng nào giữa chúng (khác với
flow-task series trước, nơi FE-TASK-001→002 là chuỗi cứng qua field mới). Thứ tự dưới đây chỉ để
giảm xung đột merge (nhiều task cùng chạm `TaskGraph.tsx`/`TaskDetail.tsx`):

```
FE-TASK-001 (TaskGraph.tsx toolbar: + New Task)
FE-TASK-002 (TaskDAGView.tsx: real deps + add-edge)     ─┐ độc lập nhau, có thể làm song song
FE-TASK-003 (TaskGraph.tsx toolbar: + Board view)       ─┘ (001/003 cùng chạm TaskGraph.tsx toolbar
                                                             — làm nối tiếp trong 1 PR hoặc 2 PR liền
                                                             để tránh conflict merge)
FE-TASK-004 (TaskDetail.tsx: tab Access + badge)        ─┐ độc lập nhau về chức năng, cùng chạm
FE-TASK-005 (TaskDetail.tsx: useTaskActivity)           ─┘ TaskDetail.tsx — khuyến nghị 004 trước 005
```

- FE-TASK-001 và FE-TASK-003 cùng sửa `TaskGraph.tsx`'s toolbar (nút mới cạnh nhau) — không phụ
  thuộc chức năng, chỉ khuyến nghị làm nối tiếp cùng 1 PR hoặc 2 PR liền để tránh conflict merge trên
  cùng vùng JSX.
- FE-TASK-004 và FE-TASK-005 cùng sửa `TaskDetail.tsx` — tương tự, không phụ thuộc chức năng,
  khuyến nghị 004 làm trước (badge permission ảnh hưởng layout Action Buttons mà 005 cũng chạm).
- FE-TASK-002 hoàn toàn độc lập (chỉ chạm `TaskDAGView.tsx`), có thể làm song song với bất kỳ task
  nào khác.

## Phụ thuộc backend — tổng hợp theo mức độ chặn

| Task | RPC dùng | Trạng thái RPC hôm nay | Có chặn việc build UI không? |
|------|----------|------------------------|-------------------------------|
| FE-TASK-001 | `task.create` | ✅ Tồn tại, nhưng `projectId` bị bỏ qua âm thầm (gap wscompat, không phải thiếu RPC) | Không — build/test được ngay, chỉ "xong end-to-end" cần thêm 1 vá nhỏ backend |
| FE-TASK-002 | `task.getDependencies`, `task.addEdge` | ✅ Tồn tại đầy đủ | Không |
| FE-TASK-003 | `task.update` | ✅ Tồn tại đầy đủ | Không |
| FE-TASK-004 | `task.grant`, `task.resolvePermission` | ✅ Tồn tại đầy đủ | Không, cho phần Add Grant + badge |
| FE-TASK-004 | `task.listGrants`/`task.revokeGrant`/`task.generateShareLink` | ❌ Chưa tồn tại (BE-SOL-003 📋 Proposed) | **Có, cho riêng phần xem/revoke/share-link** — nhưng không chặn build phần còn lại; UI mock/no-op có thông báo rõ ràng cho tới khi backend merge |
| FE-TASK-005 | `task.get` | ✅ Tồn tại đầy đủ | Không |

Không có task nào trong đợt này bị **chặn hoàn toàn** bởi backend chưa xong — khác biệt rõ với
flow-task series trước (nơi FE-TASK-002/003 gốc có hard blocker thật). FE-TASK-004 là task duy nhất
có 1 phần con (list/revoke/share) phải tạm mock — không lùi lịch cả task vì việc này.

## Không thuộc phạm vi các task ở đây

Xem mục "Not in scope" của
[FE-SOL-001](../solutions/FE-SOL-001-task-crud-board-grant-ui.md) — cụ thể: không implement lại §C4
(đã sửa từ trước), không xây Comments tab (chờ BE-SOL-001's `AddComment`/`ListComments`), không xây
Dashboard Task Execute (Engine 2) thay `OrchestrationPage.tsx`.
