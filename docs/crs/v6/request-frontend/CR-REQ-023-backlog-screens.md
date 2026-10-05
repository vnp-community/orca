# CR-REQ-023 — Màn hình Backlog: Request backlog, Task backlog, Execute backlog

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-023 |
| **Tên** | Màn hình Backlog với ba phân đoạn (Request, Task, Execute), mở lại và hủy Request, điều hướng tới Task |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-018 (`useBacklog`, `backlogView` trong store, gỡ `backlog` khỏi `TaskStatus`), CR-REQ-019 (chi tiết Request, mở lại, hủy), CR-REQ-021 (xem Plan/Phase); backend CR-REQ-006 (trả về, mở lại, hủy), CR-REQ-015 (API ba view), CR-REQ-013 (`execution_link`); kênh CR-REQ-016 |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/request/backlog/` (mới), `hooks/useBacklog.ts` (do CR-REQ-018 tạo), `i18n/locales/*.json` |

---

## 1. Bối cảnh và vấn đề

Quyết định D4: Backlog là view tính toán, không thêm status backend, và gỡ `backlog` khỏi `TaskStatus` frontend (CR-REQ-018). Cột Board `backlog` biến mất, nên cần một nơi rõ ràng để xem "việc chưa chạy". README 3.8 định nghĩa ba view:

| View | Điều kiện (README 3.8) |
|---|---|
| Request backlog | `requests.status = request_backlog` |
| Task backlog | Task làm việc dưới Plan chưa có Approval `approved`, hoặc Plan chưa chia Phase |
| Execute backlog | Task làm việc dưới Phase đã `approved`, status `open` hoặc `blocked`, hoặc `execution_link` gần nhất `failed` |

Hiện chưa có UI nào cho ba view này. Hiện trạng liên quan: `TaskBoardView` có cột `backlog` (sẽ gỡ), `TaskDetail` đã có nút chạy agent và `ExecutionEngineBadge`, `TaskDispatchStatusPanel` hiển thị lần chạy gần nhất.

## 2. Giải pháp đề xuất

### 2.1 Cây component (tab "Backlog" của `RequestPage`)

```
BacklogTab
├─ BacklogSegmentControl        (ui/toggle-group: Request | Task | Execute, kèm số mục mỗi phân đoạn)
├─ BacklogToolbar               (tìm kiếm, dự án, loại Request, nút Làm mới)
├─ RequestBacklogTable          view=request
│    └─ RequestBacklogRow
├─ TaskBacklogTable             view=task
│    └─ TaskBacklogRow
├─ ExecuteBacklogTable          view=execute
│    └─ ExecuteBacklogRow
├─ BacklogEmptyState / BacklogErrorState / BacklogSkeleton
└─ ReopenRequestDialog, CancelRequestDialog
```

Bố cục: thanh phân đoạn trên cùng; bảng (`ui/table.tsx`) chiếm phần còn lại, tiêu đề cột dính khi cuộn. Phân đoạn đang chọn lưu ở `requestPage.backlogView` (không persist qua khởi động lại).

### 2.2 Nguồn dữ liệu

- `useBacklog(view, filters)` gọi `backlog.list {view, projectId?, type?, q?, pageToken?}` (`ListBacklog`, README 3.6; CR-REQ-015). Frontend không tự tính các view từ `task.list`: điều kiện cần Approval và `execution_link` mà client không có đủ, và tính lại ở client dễ lệch backend. Câu hỏi D4 "hiển thị ở frontend" nêu ở mục 7.
- Số mục ở nhãn phân đoạn: cùng lời gọi nhưng `limit=1` và `total` (nếu backend trả; mục 7). Nếu không có `total`, hiện số của trang đã tải kèm `+` khi còn trang.
- Làm mới khi có `request.returned`, `request.status_changed`, `plan.generated`, `phase.started`, `phase.completed`, `approval.decided`; không có luồng sự kiện thì polling 30 giây khi tab hiển thị.

### 2.3 Cột hiển thị

**Request backlog** (`RequestBacklogRow`):

| Cột | Nội dung |
|---|---|
| Request | `#số` và tiêu đề; bấm mở chi tiết |
| Nguồn | `RequestSourceBadge`: `source_provider` + `source_ref`, liên kết `source_url` (chỉ http/https) |
| Loại | `RequestTypeBadge` |
| Giai đoạn bị trả | `returned_from_stage` ∈ `classification|analysis|plan|phase|task` (nhãn i18n) |
| Lý do | `return_reason`, cắt 2 dòng, đầy đủ trong tooltip |
| Người trả | người hoặc "AI/Hệ thống" |
| Thời điểm | thời gian tương đối, tooltip ngày giờ |
| Hành động | "Mở lại", "Hủy" |

**Task backlog** (`TaskBacklogRow`):

| Cột | Nội dung |
|---|---|
| Task | `#TG-N`, tiêu đề, `TaskStatusBadge`; bấm mở `TaskDetail` trong `Sheet` |
| Plan | tên Plan, liên kết sang Request tab Plan |
| Trạng thái duyệt Plan | `ApprovalStatusBadge` của Approval `plan`/`task_list`: `pending`, `rejected`, hoặc "Chưa chia Phase" khi Plan đã duyệt nhưng chưa có Phase |
| Estimate | `estimatedHours` (giờ), `-` nếu trống |
| Phụ thuộc | số và danh sách rút gọn `depends_on` (`useTaskDependencyEdges`), chip `destructive` nếu phụ thuộc chưa `done` |

**Execute backlog** (`ExecuteBacklogRow`):

| Cột | Nội dung |
|---|---|
| Task | như trên |
| Phase | tên Phase, trạng thái duyệt |
| Bị chặn bởi | danh sách task/phụ thuộc đang chặn (`blocked`), rỗng nếu chỉ `open` |
| Lý do lỗi gần nhất | từ `execution_link` gần nhất `failed`, cắt 2 dòng, đầy đủ trong tooltip |
| Số lần thử | số `execution_link` của task |
| Engine | `ExecutionEngineBadge` (3 engine; CR-TG-008) |
| Thời điểm lỗi/cập nhật | tương đối |

Tên trường wire của view Task và Execute chưa có trong README; `shared/request-types.ts` (CR-REQ-018) định nghĩa `TaskBacklogItem` và `ExecuteBacklogItem` theo cột trên, parser chịu thiếu trường.

### 2.4 Hành động và quyền

| Hành động | Hiện khi | RPC | Quy tắc |
|---|---|---|---|
| Mở lại Request | `request_backlog`, người xem có quyền | `request.reopen {requestId, version}` | Hộp `ReopenRequestDialog` hiện `returned_from_stage` và cho biết Request quay về bước đó (backend quyết định, CR-REQ-006); ghi chú tuỳ chọn |
| Hủy Request | `request_backlog` | `request.cancel` | `CancelRequestDialog` xác nhận (`variant=destructive`), lý do tuỳ chọn |
| Mở Task / Plan / Request | luôn | điều hướng | `openRequestPage` hoặc mở `TaskDetail` |

- Task backlog và Execute backlog chỉ đọc ở màn này: chạy lại task dùng nút sẵn có trong `TaskDetail`, duyệt Plan/Phase dùng tab Plan (CR-REQ-021). Lý do: không mở thêm đường chạy agent thứ hai, và quyền thực thi do `Grant` quyết định.
- Quyền: không có `viewerCan` thì hiện nút và xử lý `forbidden` bằng toast; chỉ đọc khi `unsupported`.
- Sau Mở lại, hàng biến mất khỏi view và toast có nút "Xem Request". Lỗi `invalid_state` (đã được mở lại bởi người khác): toast, hàng biến mất.
- Phím tắt: `j`/`k`, `Enter` mở hàng; `1`, `2`, `3` chuyển phân đoạn khi tiêu điểm ở thanh phân đoạn hoặc danh sách (bỏ qua trong ô nhập). Không phím sửa đổi nên không phụ thuộc Mac/Windows; chip hiển thị bằng `ShortcutKeyCombo` trong tooltip của phân đoạn.

### 2.5 Rỗng, tải, lỗi

| Tình huống | UI |
|---|---|
| Đang tải | `BacklogSkeleton` 8 hàng đúng số cột của phân đoạn |
| Request backlog rỗng | "Không có Request nào bị trả về" |
| Task backlog rỗng | "Mọi task đã có Plan được duyệt" |
| Execute backlog rỗng | "Không có task nào chờ chạy hoặc lỗi" |
| Bộ lọc không khớp | "Không có mục khớp" + "Xoá bộ lọc" |
| `network` | Banner + "Thử lại", giữ dữ liệu cũ mờ |
| `forbidden` | "Bạn không có quyền xem backlog của dự án này" |
| `unsupported` (runtime thiếu `backlog.list`) | Tab Backlog ẩn (CR-REQ-018); không hiện gì, như `task.getSource` |
| Chỉ một view lỗi | Phân đoạn đó báo lỗi, hai phân đoạn kia vẫn dùng được |

### 2.6 Liên hệ với việc gỡ cột `backlog`

Cột `backlog` của Board bị gỡ ở CR-REQ-018. Người dùng quen kéo task vào `backlog` sẽ không còn cột đó; thêm một dòng gợi ý ở Board: "Task chưa chạy xem ở Requests > Backlog" (khoá `auto.components.task.TaskBoardView.backlogMoved`, hiển thị khi có Request flow). Không tạo status mới.

### 2.7 i18n

Tiền tố `auto.components.request.backlog.`, đủ 5 locale: `BacklogSegmentControl.{request,task,execute}`, các tiêu đề cột `RequestBacklogTable.col.*`, `TaskBacklogTable.col.*`, `ExecuteBacklogTable.col.*`, `ReturnedFromStage.{classification,analysis,plan,phase,task}`, `ReopenRequestDialog.{title,body,confirm}`, `CancelRequestDialog.{title,body,confirm}`, `BacklogEmptyState.{request,task,execute,filtered}`, `BacklogErrorState.{network,forbidden}`, `TaskBacklogRow.planNotSplit`.

## 3. Quyết định thiết kế

- Ba phân đoạn trong một bảng đổi cột, không ba màn riêng: cùng bộ lọc và cùng mô hình điều hướng.
- Dữ liệu lấy từ `backlog.list`, không tính ở client, để một định nghĩa duy nhất (README 3.8) nằm ở backend.
- Chỉ Request backlog có hành động ghi; Task/Execute backlog là chỗ để nhìn và đi tới nơi xử lý.
- Hủy luôn qua hộp xác nhận; mở lại không cần lý do bắt buộc (khác từ chối ở cổng duyệt).

## 4. Tiêu chí chấp nhận

- [ ] Ba phân đoạn hiển thị đúng bộ cột ở 2.3; chuyển phân đoạn không gọi lại phân đoạn khác.
- [ ] Request backlog hiện `returned_from_stage`, `return_reason`, người trả, thời điểm, link gốc (chỉ http/https).
- [ ] Mở lại gọi `request.reopen`, hàng biến mất, toast có "Xem Request"; hủy cần xác nhận.
- [ ] Task backlog hiện Plan, trạng thái duyệt Plan, estimate, phụ thuộc; Execute backlog hiện Phase, bị chặn bởi, lý do lỗi gần nhất, số lần thử, engine.
- [ ] Task backlog và Execute backlog không có nút ghi trạng thái; không có cột `backlog` trên Board.
- [ ] Mỗi phân đoạn có UI riêng cho rỗng, đang tải, lỗi mạng, lỗi quyền; một phân đoạn lỗi không làm hỏng phân đoạn khác.
- [ ] Runtime không hỗ trợ `backlog.list`: tab ẩn, không toast.
- [ ] Hàng mở đúng đích: Request, Task (`TaskDetail`), Plan (tab Plan).
- [ ] Phím `1`/`2`/`3`, `j`/`k`, `Enter` hoạt động và bị bỏ qua ở ô nhập.
- [ ] Mọi chuỗi mới có 5 locale.

## 5. Kiểm thử

- Unit: parser ba loại mục (thiếu trường, enum lạ); sắp xếp mặc định; nhãn `ReturnedFromStage`; chọn cột theo phân đoạn.
- Component: ba bảng (rỗng, lỗi, dữ liệu), `ReopenRequestDialog`, `CancelRequestDialog`, `BacklogSegmentControl` (số mục, phím tắt), `TaskBacklogRow` (phụ thuộc chưa xong), `ExecuteBacklogRow` (lý do dài cắt gọn).
- Hook: `useBacklog` (phân trang, sự kiện, polling, lỗi riêng từng view).
- Cập nhật: `TaskBoardView.test.tsx` (không còn cột `backlog`, có dòng gợi ý khi có Request flow).
- E2E (cần CR-REQ-006/013/015): trả Request về backlog rồi mở lại; Task chưa có Plan duyệt vào Task backlog, sau khi duyệt Plan thì rời khỏi; task chạy lỗi vào Execute backlog.
- Chưa chạy; kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- Hình dạng dữ liệu của hai view Task và Execute chưa được CR-REQ-015 xác định; cột có thể phải đổi.
- Điều kiện "Plan chưa chia Phase" (README 3.8) mơ hồ cho loại không có Phase (`task`, `docs`, `bug` cỡ nhỏ): task của các loại đó có luôn nằm trong Task backlog cho tới khi duyệt không?
- Số mục trên phân đoạn đòi `total`; không có thì số chỉ gần đúng.
- Execute backlog có thể rất lớn khi dự án nhiều task lỗi; cần phân trang và lọc theo Phase, chưa thiết kế sâu.
- Gỡ cột Board có thể làm người dùng hiện có bối rối; dòng gợi ý chỉ giảm nhẹ.

## 7. Câu hỏi mở

1. README D4 ghi "Backlog là view tính toán hiển thị ở frontend", trong khi README 3.6 và CR-REQ-015 cung cấp `ListBacklog` ở backend. CR này giả định backend tính, frontend chỉ hiển thị; cần xác nhận.
2. Hình dạng phản hồi của `backlog.list` cho từng view, và có trả `total` không?
3. Execute backlog có cần nút "Chạy lại" ngay tại bảng (gọi `task.execute` sẵn có), hay chỉ điều hướng?
4. Mở lại Request về bước nào: luôn `returned_from_stage` hay cho chọn?
5. Task `blocked` do phụ thuộc: "bị chặn bởi" lấy từ `task_edge` (`depends_on`, `blocks`) hay từ `execution_link`?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` (D4, O3, 3.3, 3.6, 3.8)
- `/opt/repos/orca/frontend/src/renderer/src/components/task/{TaskBoardView,TaskDetail,TaskDispatchStatusPanel,ExecutionEngineBadge,TaskStatusBadge}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/hooks/useTaskDependencyEdges.ts`, `hooks/useTaskActivity.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/ui/{table,toggle-group,sheet,dialog,skeleton}.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/ShortcutKeyCombo.tsx`
- `/opt/repos/orca/docs/crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- Mới: `components/request/backlog/{BacklogTab,RequestBacklogTable,TaskBacklogTable,ExecuteBacklogTable,ReopenRequestDialog,CancelRequestDialog}.tsx`
