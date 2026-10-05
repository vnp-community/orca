# CR-REQ-006 — Trả về Request backlog, mở lại, hủy, Request con

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-006 |
| **Tên** | `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest` và liên kết `request_links` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-005 (mở lại về `classifying`, Request con đi qua phân loại), CR-REQ-003, CR-REQ-004 |
| **Mở khoá** | CR-REQ-013, 014 (hotfix theo dõi), 015, 023 |
| **Tác động** | `request-service` (use case, migration `0004`, proto `request.proto`). Gọi CR-REQ-009 (huỷ Approval) qua cổng có bản no-op |

---

## 1. Bối cảnh và vấn đề

1. Nếu Request không thực thi được, hoặc Solution, Plan bị từ chối, README v6 yêu cầu về `request_backlog` kèm `returned_from_stage` và `return_reason` (mục 3.3). Chưa có lệnh nào để vào, ra khỏi trạng thái này.
2. Nghiên cứu mục 5 yêu cầu ghi lý do, người thực hiện, liên kết Request gốc, và nhóm backlog theo lý do (thiếu thông tin, không khả thi, bị chặn bởi phụ thuộc, bị từ chối). README v6 chỉ có hai cột trên `requests`, mỗi lần trả về ghi đè lần trước; không lưu được lịch sử nhiều lần trả về, cũng không có chỗ cho nhóm lý do.
3. `spike`, `question` kết thúc mà không đổi code; việc phát hiện ra phải thành Request con. `hotfix` luôn kéo theo Request theo dõi. Chưa có cách tạo và nối các Request này (`request_links`, README mục 3.5).

## 2. Giải pháp đề xuất

### 2.1 Migration `0004_request_return_history` (cả hai dialect)

Thêm cột `requests.returned_category` (TEXT, MySQL `VARCHAR(30)`, NULL): CHECK IN (`missing_info`, `infeasible`, `blocked_dependency`, `rejected`, `other`) và CHECK `(status = 'request_backlog') = (returned_category IS NOT NULL)`.

Bảng mới `request_return_history` (bổ sung ngoài README, cần cập nhật mục 3.5):

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK, ứng dụng sinh |
| `tenant_id` | uuid | NOT NULL |
| `request_id` | uuid | NOT NULL |
| `action` | text | CHECK IN (`returned`,`reopened`,`cancelled`) |
| `stage` | text | NULL hoặc `classification`,`analysis`,`plan`,`phase`,`task` |
| `category` | text | NULL hoặc như `returned_category` |
| `reason` | text | NOT NULL DEFAULT '' |
| `actor_id` | uuid | NULL khi hệ thống |
| `actor_kind` | text | CHECK IN (`ai`,`user`,`system`) |
| `at` | timestamptz | NOT NULL |

Chỉ mục `(tenant_id, request_id, at)`. Bảng chỉ thêm. Postgres có RLS `tenant_isolation`; MySQL lọc `tenant_id` ở mọi truy vấn.

### 2.2 Proto (thêm vào `request.proto`)

```
message ReturnToBacklogRequest { string request_id = 1; string stage = 2; string category = 3;
  string reason = 4; int64 expected_version = 5; }
message ReopenRequestRequest   { string request_id = 1; string note = 2; int64 expected_version = 3; }
message CancelRequestRequest   { string request_id = 1; string reason = 2; int64 expected_version = 3; }
message SpawnChildRequestRequest { string parent_request_id = 1; string link_reason = 2;
  string title = 3; string body = 4; string type_hint = 5; string client_request_id = 6; }
message SpawnChildRequestResponse { Request child = 1; bool created = 2; }
message ListRequestLinksRequest  { string request_id = 1; }          // RPC mới, xem Q2
message ListRequestLinksResponse { repeated RequestLink parents = 1; repeated RequestLink children = 2; }
```

Ba lệnh đầu trả `Request` đã cập nhật. `ListBacklog` (README mục 3.6) thuộc CR-REQ-015; ở CR này lọc `ListRequests(status=request_backlog)` đã đủ để kiểm thử.

### 2.3 `ReturnToBacklog` (`return_request_to_backlog.go`, mới)

Chuyển qua `TransitionRequest(return_to_backlog)`. Trạng thái nguồn và `stage` hợp lệ:

| `status` hiện tại | `stage` cho phép |
|-------------------|------------------|
| `classifying`, `awaiting_type_confirmation` | `classification` |
| `analyzing`, `awaiting_analysis_approval` | `analysis` |
| `planning`, `awaiting_plan_approval` | `plan` |
| `executing` | `phase` (chỉ khi loại có Phase), `task` |

`reason` bắt buộc (`REQUEST_REASON_REQUIRED`); `category` thuộc năm giá trị (`REQUEST_RETURN_CATEGORY_INVALID`). Từ `executing` mà còn Task đang chạy thì `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION` (cổng `ExecutionGuard` của CR-REQ-005; `false` cho tới CR-REQ-011). Tx: CAS → cập nhật `status`, `returned_from_stage`, `returned_category`, `return_reason` → thêm `request_return_history` (`returned`) → huỷ Approval `pending` của Request, lý do `returned`, qua cổng `ApprovalCanceller` (no-op tới CR-REQ-009) → outbox `orca.request.request.returned` `{request_id, stage, category, reason, actor_id}` và `status_changed`.

Từ chối Solution hoặc Plan (CR-REQ-007, 009) đi trigger `analysis_rejected`, `plan_rejected` của CR-REQ-003 với `category=rejected`, cùng ghi `request_return_history`. Lỗi thực thi (CR-REQ-013) gọi use case này với `actor_kind=system`.

### 2.4 `ReopenRequest` (`reopen_request.go`, mới)

Chỉ từ `request_backlog` (`REQUEST_REOPEN_NOT_ALLOWED` nếu khác). Tx: CAS → `TransitionRequest(reopen)` đưa về `classifying`, xoá `returned_from_stage`, `returned_category`, `return_reason` → thêm `request_return_history` (`reopened`, `reason`=`note`) → outbox `status_changed` (không có sự kiện `request.reopened` riêng trong README mục 3.7). Phân loại chạy lại như CR-REQ-005; `type` cũ giữ làm gợi ý tới khi AI đề xuất. Solution, Plan, Task đã có không bị xoá, thành tham chiếu. `completed` và `cancelled` không mở lại được ở v6 (Q3).

### 2.5 `CancelRequest` (`cancel_request.go`, mới)

Từ mọi trạng thái trừ `completed`, `cancelled`, kể cả `request_backlog`. `reason` bắt buộc. Từ `executing` còn Task đang chạy thì `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION`. Tx: `TransitionRequest(cancel)` → thêm `request_return_history` (`cancelled`) → huỷ Approval `pending` (lý do `cancelled`) → outbox `status_changed`. Gọi lặp khi đã `cancelled`: thành công, `Applied=false`. Không lan sang Request con. Việc huỷ cây Plan trong `task-service` chưa thuộc CR này (Q4).

### 2.6 `SpawnChildRequest` (`spawn_child_request.go`, mới)

Quy tắc cha, con, theo README v6 mục 3.4:

| `link_reason` | Loại cha | Trạng thái cha | `type_hint` của con |
|---------------|----------|----------------|---------------------|
| `spawned_by_spike` | `spike` | `awaiting_analysis_approval`, `completed` | `change_request`, `task` |
| `spawned_by_question` | `question` | `awaiting_analysis_approval`, `completed` | `change_request`, `task` |
| `followup_hotfix` | `hotfix` | `executing`, `completed` | `bug`, `task` |
| `escalation` | bất kỳ | mọi trạng thái trừ `cancelled` | không ràng buộc |

`escalation` chưa được định nghĩa trong README; CR này hiểu là con do người tạo tay từ Request bất kỳ khi phát hiện việc riêng (Q1). Sai tổ hợp: `REQUEST_CHILD_NOT_ALLOWED`.

Bước: `client_request_id` bắt buộc (`REQUEST_CLIENT_REQUEST_ID_REQUIRED`); đọc cha (`REQUEST_PARENT_NOT_FOUND`); kiểm quy tắc; kiểm giới hạn 50 con mỗi cha (`REQUEST_CHILD_LIMIT`) và chuỗi tổ tiên tối đa 5 cấp (`REQUEST_CHILD_DEPTH_EXCEEDED`); rồi trong một transaction dùng lại phần lõi của `CreateRequest` (CR-REQ-004): cùng `project_id` với cha, `source_provider=manual` (hoặc `mcp` khi gọi từ MCP), `source_ref` rỗng, khoá idempotency `(tenant, provider, 'user:<actor>', client_request_id)`, `source_hints.type_hint = type_hint`, vào `classifying`; chèn `request_links(parent, child, reason, created_by)`; outbox `request.created` của con có thêm `parent_request_id`, `link_reason` (trường cộng thêm). Con vẫn đi đủ phân loại và xác nhận (CR-REQ-005 dùng `type_hint` như gợi ý mạnh). Gọi lặp cùng `client_request_id`: trả con cũ, `created=false`.

`hotfix` kết thúc thì Request theo dõi do CR-REQ-014 gọi use case này; CR này chỉ cung cấp lệnh.

### 2.7 Mã lỗi

| Mã | Kind | Khi |
|----|------|-----|
| `REQUEST_REASON_REQUIRED` | InvalidArgument | thiếu `reason` |
| `REQUEST_RETURN_STAGE_INVALID`, `REQUEST_RETURN_CATEGORY_INVALID` | InvalidArgument | `stage` không khớp trạng thái, `category` lạ |
| `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`, `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION` | FailedPrecondition | còn Task đang chạy |
| `REQUEST_REOPEN_NOT_ALLOWED` | FailedPrecondition | không ở `request_backlog` |
| `REQUEST_CANCEL_NOT_ALLOWED` | FailedPrecondition | đã `completed` |
| `REQUEST_PARENT_NOT_FOUND` | NotFound | cha không có |
| `REQUEST_CHILD_NOT_ALLOWED`, `REQUEST_CHILD_LIMIT`, `REQUEST_CHILD_DEPTH_EXCEEDED` | FailedPrecondition | quy tắc 2.6 |
| `REQUEST_CLIENT_REQUEST_ID_REQUIRED` | InvalidArgument | thiếu |
| `REQUEST_VERSION_CONFLICT`, `REQUEST_TRANSITION_NOT_ALLOWED` | FailedPrecondition | CR-REQ-002, 003 |

Quyền: ghi trên project (gateway). Quyền hủy, mở lại riêng theo vai trò thuộc CR-REQ-010.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Bảng `request_return_history` | Hai cột trên `requests` chỉ giữ lần cuối; nghiên cứu yêu cầu ghi người thực hiện và nhiều lần |
| `returned_category` tách khỏi `return_reason` | Nhóm backlog theo lý do cần giá trị có kiểu, lý do tự do không nhóm được |
| Mở lại về `classifying` | Nghiên cứu mục 3 (mở lại, phân loại lại); loại luôn đi qua phân loại |
| Con tạo bằng lõi `CreateRequest`, không tạo đường riêng | Một đường vào, một bộ kiểm tra |
| `client_request_id` bắt buộc | Con không có khoá nguồn; thiếu khoá thì bấm hai lần sinh hai con |
| Hủy không lan sang con | Con có vòng đời độc lập; huỷ cả cây dễ mất việc |
| Không có `request.reopened`, `request.cancelled` | README v6 mục 3.7 không liệt kê; dùng `status_changed` kèm `trigger` |

## 4. Tiêu chí chấp nhận

- [ ] Mỗi trạng thái nguồn trả về backlog với `stage` hợp lệ; `stage` sai trả `REQUEST_RETURN_STAGE_INVALID`; `executing` không có Phase mà `stage=phase` bị từ chối.
- [ ] Sau trả về: `status=request_backlog`, `returned_from_stage`, `returned_category`, `return_reason` đặt; một dòng `request_return_history`; sự kiện `returned` và `status_changed`.
- [ ] Trả về hai lần cùng `expected_version`: lần hai không thêm dòng lịch sử, không thêm sự kiện.
- [ ] Mở lại: về `classifying`, ba cột trả về NULL hoặc rỗng, lịch sử `reopened`, Solution và Task cũ còn nguyên, phân loại AI chạy lại.
- [ ] Mở lại từ trạng thái khác `request_backlog` bị từ chối.
- [ ] Hủy từ `request_backlog` và từ `analyzing` thành công; hủy `completed` bị từ chối; hủy hai lần thành công.
- [ ] Đang có Task chạy (cổng giả trả `true`) thì trả về, hủy bị chặn.
- [ ] Con từ `spike` với `type_hint=bug` bị từ chối; con từ `hotfix` với `bug` được; cha `cancelled` không sinh `escalation`.
- [ ] 12 `SpawnChildRequest` đồng thời cùng `client_request_id` ra đúng một con và một `request_links`.
- [ ] Cha có 50 con rồi: con thứ 51 trả `REQUEST_CHILD_LIMIT`; chuỗi 6 cấp trả `REQUEST_CHILD_DEPTH_EXCEEDED`.
- [ ] Mọi chuyển giữ CHECK `(status='request_backlog') = (returned_category IS NOT NULL)` trên cả hai DB.
- [ ] Tenant B không đọc, không ghi `request_links` hay `request_return_history` của tenant A.

## 5. Kiểm thử

- **Unit:** bảng `stage` theo trạng thái; bảng cha, con, `type_hint`; giới hạn độ sâu và số con (repo giả).
- **Integration, Postgres và MySQL:** migration `0004` up/down; các tiêu chí đồng thời, CHECK, rollback giữa chừng (không để `request_links` mồ côi); mọi lệnh đi qua `TransitionRequest`.
- **Hợp đồng:** `buf breaking`; test sự kiện `returned` có đủ trường cho `notification-service` (CR-REQ-010) và `issue-status-sync` (CR-REQ-024).
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- `ExecutionGuard` giả tới CR-REQ-011: chặn trả về, hủy khi có Task chạy chưa kiểm chứng được trên hệ thống thật.
- Cạnh tranh giữa người trả về và consumer outbox (từ chối Approval) cùng lúc: CAS `version` giải quyết, bên thua nhận `REQUEST_VERSION_CONFLICT`, UI phải tải lại.
- Giới hạn 50 con và 5 cấp là số ước lượng, không có dữ liệu thực.
- Mở lại nhiều lần có thể vòng lặp AI tốn chi phí; chưa đặt giới hạn số lần mở lại.

## 7. Câu hỏi mở

- **Q1.** README v6 mục 3.5 liệt kê `escalation` mà không định nghĩa; CR này giả định "con tạo tay từ Request bất kỳ". Cần xác nhận hoặc bỏ.
- **Q2.** `ListRequestLinks` chưa có trong README v6 mục 3.6; frontend (CR-REQ-019) cần cha và con. Đề nghị thêm.
- **Q3.** Có cho mở lại `completed`, `cancelled` không; mặc định không.
- **Q4.** Hủy Request có huỷ cây Plan, Phase, Task trong `task-service` không (cần RPC ở CR-REQ-011 hoặc 013).
- **Q5.** Cột `returned_category` và bảng `request_return_history` chưa có trong README mục 3.5; cần cập nhật.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.3, 3.4, 3.5, 3.7
- `/opt/repos/orca/docs/research/receive-request/request-classification-and-flows.md` mục 4, 5
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/report_execution_result.go` (mẫu thao tác idempotent)
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql` (mẫu RLS, khoá duy nhất)
- `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `common/apperrors/apperrors.go`
- `/opt/repos/orca/backend-go/services/request-service/internal/usecase/{return_request_to_backlog,reopen_request,cancel_request,spawn_child_request}.go`, `migrations/{postgres,mysql}/0004_request_return_history.*.sql` (mới)
