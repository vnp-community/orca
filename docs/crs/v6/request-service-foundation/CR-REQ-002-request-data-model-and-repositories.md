# CR-REQ-002 — Mô hình dữ liệu, migration và repository của Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-002 |
| **Tên** | Schema, entity miền và repository (Postgres + MySQL) cho Request, lịch sử loại, Solution, liên kết, idempotency |
| **Loại** | Feature (nền dữ liệu) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-REQ-001 |
| **Mở khoá** | CR-REQ-003, 004, 005, 006, 007, 009 |
| **Tác động** | `backend-go/services/request-service/{internal/domain, internal/usecase/ports.go, internal/adapter/postgres, internal/adapter/mysql, migrations/postgres, migrations/mysql}` (mới). Không đụng service khác |

---

## 1. Bối cảnh và vấn đề

CR-REQ-001 dựng khung và hai bảng hạ tầng. Chưa có chỗ lưu Request. README v6 mục 3.5 định nghĩa các bảng chính nhưng chưa ghi kiểu cột, ràng buộc, chỉ mục, cách cấp số Request, và khác biệt Postgres/MySQL. Task hiện có mẫu tốt để theo: `task_sources` dùng khoá duy nhất có `site_id` và xử lý NULL khác nhau giữa hai dialect (migration `0012`, `0014` của `task-service`).

CR này chỉ sở hữu **schema và repository**. Quy tắc chuyển trạng thái (CR-REQ-003), logic phân loại (CR-REQ-005) và bảng `approvals` (CR-REQ-009) nằm ở CR khác.

## 2. Giải pháp đề xuất

### 2.1 Migration `0002_request_core` (up và down, cả hai dialect)

Kiểu: Postgres `UUID`, `TIMESTAMPTZ`, `JSONB`, `NUMERIC(4,3)`; MySQL `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `DECIMAL(4,3)`. Mọi id do ứng dụng sinh (`uuid.NewString()`), không `DEFAULT` sinh id ở DB. Postgres bật RLS và `CREATE POLICY tenant_isolation ... USING (tenant_id = current_setting('app.tenant_id', true)::uuid)` cho từng bảng, như `task.task_sources`.

**`requests`**

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `project_id` | uuid | NULL (Request chưa gắn project được phép), không FK |
| `number` | BIGINT | NOT NULL; hiển thị `REQ-<number>`; UNIQUE `(tenant_id, number)` |
| `title` | TEXT (MySQL `VARCHAR(500)`) | NOT NULL, không rỗng sau trim (kiểm ở domain) |
| `body` | TEXT | NOT NULL DEFAULT '' |
| `source_provider` | TEXT (MySQL `VARCHAR(20)`) | NOT NULL, CHECK IN (`jira`,`github`,`gitlab`,`linear`,`mcp`,`manual`,`webhook`) |
| `source_ref` | TEXT (MySQL `VARCHAR(255)`) | NOT NULL DEFAULT '' |
| `source_url` | TEXT | NOT NULL DEFAULT '' |
| `source_site` | TEXT (MySQL `VARCHAR(255)`) | NOT NULL DEFAULT '' ('' = không có hoặc không biết) |
| `type` | TEXT (MySQL `VARCHAR(20)`) | NULL cho tới khi có đề xuất đầu tiên; CHECK IN 11 loại README 3.2 |
| `type_source` | TEXT | NULL hoặc `ai`, `human` |
| `size` | TEXT | NULL hoặc `S`,`M`,`L` |
| `urgency` | TEXT | NOT NULL DEFAULT `normal`, CHECK IN (`normal`,`urgent`) |
| `confidence` | NUMERIC(4,3) | NULL hoặc 0 đến 1 (CHECK) |
| `classification_reason` | TEXT | NOT NULL DEFAULT '' |
| `status` | TEXT (MySQL `VARCHAR(40)`) | NOT NULL DEFAULT `new`, CHECK IN 11 giá trị README 3.3 |
| `returned_from_stage` | TEXT | NULL hoặc `classification`,`analysis`,`plan`,`phase`,`task` |
| `return_reason` | TEXT | NOT NULL DEFAULT '' |
| `plan_task_id` | uuid | NULL, không FK (id Task của `task-service`) |
| `reporter_id` | uuid | NOT NULL (user tạo hoặc user mà nguồn ánh xạ tới) |
| `created_at`, `updated_at` | timestamptz | NOT NULL, mặc định đồng hồ DB |
| `version` | BIGINT | NOT NULL DEFAULT 1 (khoá lạc quan) |

Ràng buộc chéo (CHECK, cả hai dialect): `(status = 'request_backlog') = (returned_from_stage IS NOT NULL)`. MySQL 8.0.16 trở lên mới thực thi CHECK; xem mục 6.

Chỉ mục: `(tenant_id, status, updated_at DESC)` cho view backlog và danh sách; `(tenant_id, project_id, status)`; `(tenant_id, plan_task_id)` (CR-REQ-013 tra ngược từ Task); `(tenant_id, source_provider, source_site, source_ref)` không duy nhất (khoá duy nhất nằm ở `request_idempotency`).

**`request_counters`** (bổ sung ngoài README): `tenant_id` PK, `next_number BIGINT NOT NULL`. Cấp số xem 2.4.

**`request_type_history`**: `id` PK, `tenant_id` NOT NULL, `request_id` NOT NULL, `from_type` NULL (lần đầu), `to_type` NOT NULL (CHECK 11 loại), `actor_id` NULL (NULL khi `actor_kind='ai'`), `actor_kind` CHECK (`ai`,`user`), `reason` NOT NULL DEFAULT '', `at` NOT NULL. Chỉ mục `(request_id, at)`. Bảng chỉ thêm, không sửa, không xoá.

**`solutions`**: `id` PK, `tenant_id`, `request_id`, `kind` CHECK (`solution`,`diagnosis`,`findings`,`answer`), `status` CHECK (`draft`,`proposed`,`approved`,`rejected`,`superseded`) DEFAULT `draft`, `options` JSON NOT NULL DEFAULT `'[]'` (MySQL: không cho DEFAULT literal trên JSON ở mọi bản, ứng dụng luôn ghi giá trị), `chosen_option` INT NULL (chỉ số trong `options`), `content_ref` TEXT NOT NULL DEFAULT '', `generation_run_id` TEXT NOT NULL DEFAULT '', `created_at`, `updated_at`, `version` BIGINT DEFAULT 1. Chỉ mục `(tenant_id, request_id, created_at)`. Thêm `tenant_id`, `updated_at`, `version` ngoài README để khớp quy ước "mọi bảng có `tenant_id`" và khoá lạc quan. CR-REQ-007 sở hữu nội dung `options`.

**`request_links`**: PK `(tenant_id, parent_request_id, child_request_id)`; `reason` CHECK (`spawned_by_spike`,`spawned_by_question`,`followup_hotfix`,`escalation`); `created_by` uuid NULL; `created_at`. CHECK `parent_request_id <> child_request_id`. Chỉ mục `(tenant_id, child_request_id)`. CR-REQ-006 sở hữu quy tắc nghiệp vụ.

**`request_idempotency`**: PK `(tenant_id, source_provider, source_site, source_ref)`, cột `request_id` NOT NULL. `source_site` NOT NULL DEFAULT '' để NULL không phá tính duy nhất (khác `task_sources` phải dùng `COALESCE` cho `project_id`). Dòng chỉ tồn tại khi `source_ref <> ''`. MySQL độ dài khoá: `CHAR(36)` + `VARCHAR(20)` + 2 × `VARCHAR(255)` bytes utf8mb4 khoảng 2264, nằm trong giới hạn 3072 của InnoDB.

Down: `DROP TABLE` ngược thứ tự phụ thuộc; không có FK nên thứ tự tuỳ ý, vẫn giữ ngược.

### 2.2 Domain (`internal/domain`, tất cả mới)

Tên file theo khái niệm: `request.go` (struct `Request`, constructor `NewRequest` kiểm title, tenant, reporter), `request_type.go` (kiểu `RequestType`, 11 hằng, `ParseRequestType`), `request_status.go` (kiểu `RequestStatus` và hằng; bảng chuyển trạng thái thuộc CR-REQ-003, ở đây chỉ khai báo giá trị và `IsTerminal`), `request_source.go` (`SourceProvider`, `SourceRef{Provider, Site, Ref, URL}`), `request_type_change.go`, `solution.go` (`Solution`, `SolutionKind`, `SolutionStatus`), `request_link.go`, `request_errors.go`.

Lỗi dùng `common/apperrors`:

| Mã | Kind | Khi |
|----|------|-----|
| `REQUEST_TENANT_REQUIRED` | InvalidArgument | không có tenant trong context |
| `REQUEST_INVALID_TYPE` / `REQUEST_INVALID_STATUS` / `REQUEST_INVALID_SIZE` | InvalidArgument | giá trị ngoài tập |
| `REQUEST_TITLE_REQUIRED` | InvalidArgument | title rỗng sau trim |
| `REQUEST_NOT_FOUND` | NotFound | không có id trong tenant (kể cả id của tenant khác) |
| `REQUEST_VERSION_CONFLICT` | FailedPrecondition | CAS `version` không khớp |
| `REQUEST_SOURCE_ALREADY_EXISTS` | AlreadyExists | trùng khoá `request_idempotency`; mang `request_id` hiện có |
| `REQUEST_LINK_SELF` | InvalidArgument | cha trùng con |
| `SOLUTION_NOT_FOUND` / `SOLUTION_VERSION_CONFLICT` | NotFound / FailedPrecondition | như trên |

### 2.3 Cổng repository (`internal/usecase/ports.go`)

| Cổng | Phương thức |
|------|-------------|
| `RequestRepository` | `Create(ctx, Request)`, `Get(ctx, id)`, `GetByNumber(ctx, number)`, `List(ctx, ListFilter)`, `Update(ctx, Request, expectedVersion)` (CAS), `NextNumber(ctx)` |
| `RequestTypeHistoryRepository` | `Append(ctx, Change)`, `List(ctx, requestID)` |
| `SolutionRepository` | `Insert`, `Get`, `ListByRequest`, `Update(ctx, Solution, expectedVersion)` |
| `RequestLinkRepository` | `Insert`, `ListChildren(parentID)`, `ListParents(childID)` |
| `RequestIdempotencyRepository` | `Claim(ctx, SourceRef, requestID) (existingRequestID string, claimed bool, err)` |
| `TxRunner` | `InTx(ctx, func(ctx) error) error`; mọi repo lấy executor từ context (mẫu `task-service`) |
| `OutboxWriter` | `InsertOutboxEvent(ctx, OutboxEvent)` (CR-REQ-001) |

`ListFilter`: `ProjectID`, `Statuses []`, `Types []`, `ReporterID`, `PageSize` (mặc định 50, tối đa 200), `PageToken`. Phân trang keyset theo `(created_at DESC, id DESC)`; `PageToken` là base64 của hai giá trị đó. Mọi truy vấn có `tenant_id = $tenant` ở `WHERE`, kể cả Postgres có RLS (phòng thủ chiều sâu, và MySQL không có RLS).

### 2.4 Hành vi theo dialect

- **Cấp số:** trong cùng transaction với `Create`. Postgres: `INSERT INTO request_counters (tenant_id,next_number) VALUES ($1,1) ON CONFLICT (tenant_id) DO UPDATE SET next_number = request_counters.next_number + 1 RETURNING next_number`. MySQL (không có `RETURNING`): `INSERT ... VALUES (?,1) ON DUPLICATE KEY UPDATE next_number = next_number + 1`, rồi `SELECT next_number ... WHERE tenant_id = ?` trong cùng transaction. Hàng bị khoá tới commit nên không trùng số, rollback không để lại lỗ hổng.
- **CAS:** `UPDATE ... SET ..., version = version + 1, updated_at = now() WHERE id = ? AND tenant_id = ? AND version = ?`; 0 hàng thì `SELECT` để phân biệt `REQUEST_NOT_FOUND` với `REQUEST_VERSION_CONFLICT`. MySQL dùng `clientFoundRows` hoặc kiểm `version` đọc lại, vì `RowsAffected` mặc định tính hàng thật sự đổi; giá trị `version` luôn đổi nên an toàn.
- **Claim idempotency:** Postgres `INSERT ... ON CONFLICT DO NOTHING`; MySQL `INSERT IGNORE`. Nếu không chèn được thì `SELECT request_id` trả về bản đã có. Thua race trong `Create` thì transaction của người thua rollback toàn bộ (kể cả số Request đã cấp).
- **JSON:** `options` đọc và ghi như `[]byte`; không dùng toán tử JSONB trong truy vấn, nên không có khác biệt hành vi.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| `type` nullable | Request mới chưa có loại; ép giá trị giả như `task` sẽ làm lệch thống kê và registry |
| Số Request theo tenant, bảng bộ đếm | `COUNT(*)+1` đua nhau; sequence DB khác nhau giữa hai dialect |
| Khoá idempotency tách bảng, `source_site` NOT NULL DEFAULT '' | Một khoá duy nhất giống nhau ở hai dialect, không cần `COALESCE`/generated column |
| Khoá duy nhất không có `project_id` | Theo README v6 (tenant, provider, site, ref). Xem Q2 |
| `request_type_history` chỉ thêm | Là bằng chứng kiểm toán đổi loại (CR-REQ-005) |
| Khoá lạc quan `version` thay `SELECT FOR UPDATE` | Cùng cách `approvals.version` ở README; chạy giống nhau hai dialect |
| Thêm `tenant_id` vào bảng con | Quy ước "mọi bảng có `tenant_id`"; README 3.5 thiếu ở `request_type_history`, `solutions`, `request_links` |

## 4. Tiêu chí chấp nhận

- [ ] `0002_request_core` up/down chạy sạch bằng golang-migrate trên Postgres và MySQL; up lại sau down không lỗi.
- [ ] Mỗi CHECK ở 2.1 từ chối giá trị sai (test chèn trực tiếp từng giá trị lỗi).
- [ ] 20 `Create` đồng thời trong một tenant cho 20 `number` liên tiếp, không trùng, không lỗ hổng (cả hai dialect).
- [ ] Hai `Claim` đồng thời cùng khoá: đúng một bên `claimed=true`, bên kia nhận `request_id` của bên thắng.
- [ ] `Update` với `version` cũ trả `REQUEST_VERSION_CONFLICT`; id không tồn tại trả `REQUEST_NOT_FOUND`.
- [ ] Tenant A không đọc, không cập nhật được dòng tenant B qua repository (cả hai dialect) và qua SQL trực tiếp trên Postgres (RLS).
- [ ] `List` phân trang keyset ổn định khi có hàng mới chèn giữa hai trang.
- [ ] `request_links` từ chối cha trùng con; `request_type_history` giữ đúng thứ tự theo `at`.
- [ ] `go vet`, `make lint` xanh; không có tên file `helpers`, `utils`, `common`, `misc`.

## 5. Kiểm thử

- **Unit (không DB):** `ParseRequestType`, `NewRequest` (title rỗng, thiếu tenant), mã lỗi map đúng gRPC status qua `apperrors.ToGRPCStatus`.
- **Integration, cả hai dialect (`-tags=integration`):** toàn bộ tiêu chí mục 4; mỗi test chạy một lần cho mỗi dialect trong matrix CI của CR-REQ-001. Thư mục test: `adapter/postgres/*_test.go` và `adapter/mysql/*_test.go`, dùng chung bộ kịch bản qua hàm kiểm thử nhận interface repository (đặt trong file `request_repository_contract_test.go`).
- **Hợp đồng schema:** test đọc `information_schema` ở cả hai DB, so tên bảng, tên cột, tính NULL với danh sách mong đợi trong test.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- CHECK trên MySQL cần 8.0.16 trở lên; bản cũ phân tích rồi bỏ qua CHECK. Cần xác định phiên bản MySQL/TiDB mục tiêu (CR-DB-002 chỉ nêu `SKIP LOCKED` cần 8.0.1). Chưa kiểm chứng TiDB.
- Hàng đếm `request_counters` tuần tự hoá việc tạo Request trong một tenant; đủ cho tải tạo tay và webhook mức vừa, chưa đo ở tải import hàng loạt.
- `request_idempotency` khoá bằng `source_site`: Jira Cloud site là base URL, cần chuẩn hoá (CR-REQ-004) để cùng issue không sinh hai khoá.
- `plan_task_id` không FK: Task bị xoá ở `task-service` để lại id mồ côi; kiểm ở tầng ứng dụng (CR-REQ-013).

## 7. Câu hỏi mở

- **Q1.** README v6 mục 3.5 thiếu `tenant_id` ở `request_type_history`, `solutions`, `request_links`, thiếu `request_counters`, `processed_events`, và ghi `outbox` thay vì `outbox_events`. CR này bổ sung; cần cập nhật README.
- **Q2.** Khoá duy nhất không gồm `project_id` nghĩa là một issue Jira chỉ tạo được một Request cho cả tenant, dù Orca có nhiều project ánh xạ cùng project Jira. CR-TG-008 dùng `(tenant, project, provider, site, ref)`. Cần chốt.
- **Q3.** `requests.type` nullable chưa có trong README (mục 3.5 không nói). Xác nhận.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.2, 3.3, 3.5
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql`, `0014_task_sources_site.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0005_outbox.up.sql`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/task_sources.go`, `adapter/mysql/task_sources.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/task_source_ports.go`, `ports.go` (mẫu `TxRunner`)
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go`, `common/tenant/tenant.go`, `common/dbcapability/capability.go`
- `/opt/repos/orca/docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md`
- `/opt/repos/orca/backend-go/services/request-service/` (mới, từ CR-REQ-001)
