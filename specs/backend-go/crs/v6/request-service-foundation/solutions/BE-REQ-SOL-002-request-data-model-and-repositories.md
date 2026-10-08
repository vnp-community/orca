# BE-REQ-SOL-002: Mô hình dữ liệu, migration hai dialect và repository của Request

> **✅ Đã triển khai (kiểm chứng 2026-10-07).** 7/7 task xong; test tích hợp chạy thật trên Postgres 16 và MySQL 8.0. Lệch so với solution (struct repository riêng, cổng `SolutionCoreRepository`, cột `reason`) ghi ở [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md). Phụ thuộc [BE-REQ-SOL-001](./BE-REQ-SOL-001-scaffold-request-service.md).

**CR:** [CR-REQ-002](../../../../../../docs/crs/v6/request-service-foundation/CR-REQ-002-request-data-model-and-repositories.md)
**Service:** `request-service` (`internal/domain`, `internal/usecase/ports.go`, `internal/adapter/{postgres,mysql,grpc}`, `migrations/*`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (khoá lạc quan, tenant, hai dialect), [`arch/04`](../../../../tdd/architecture/04-tech-stack.md) (multi-dialect)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `task-service/migrations/{postgres,mysql}/0012_task_sources.up.sql`, `0014_task_sources_site.up.sql`, `0001_init.up.sql`, `task-service/internal/adapter/{postgres,mysql}/task_sources.go`, `internal/usecase/task_source_ports.go`, `common/{apperrors,tenant,dbcapability}`. Thư mục `services/request-service/` chưa tồn tại (do SOL-001 tạo); số migration của `request-service` chưa có file nào, nên `0001` do SOL-001, `0002_request_core` do solution này.

### Correction relative to CR-REQ-002

| # | CR nói | Mã thật | Xử lý |
|---|--------|---------|-------|
| C1 | "repo lấy executor từ context (mẫu `task-service`)" | `task-service` dùng `RunInTx(fn(ctx, tasks, edges))` với Repository có phạm vi giao dịch, không lấy từ ctx | Dùng executor trong ctx như SOL-001 mục 2.D; không theo mẫu `task-service` ở điểm này |
| C2 | Policy `USING (tenant_id = current_setting('app.tenant_id', true)::uuid)` "như `task.task_sources`" | Policy đó inert, và nếu `app.tenant_id` còn `''` từ giao dịch trước trên kết nối pool thì ép kiểu `uuid` gây lỗi | Policy `NULLIF(current_setting('app.tenant_id', true), '')::uuid` + `FORCE`, như `mcp-service` |
| C3 | `task_sources` "phải dùng `COALESCE` cho `project_id`" | Đúng ở Postgres (`0012`); MySQL `0014` đã đổi sang cột `project_key`, `site_id VARCHAR(255) NOT NULL DEFAULT ''` | Giữ quyết định của CR: khoá idempotency không có `project_id`, `source_site NOT NULL DEFAULT ''` |
| C4 | `request_idempotency` có `source_ref` có thể rỗng | Khoá `manual`/`mcp` là `(tenant, provider, 'user:<reporter>', client_request_id)` (CR-REQ-004 mục 2.2): `source_site` mang giá trị `user:<uuid>` (41 ký tự), `source_ref` mang `client_request_id` | Cột `source_ref` của bảng này `VARCHAR(255) NOT NULL`, không cho rỗng; ràng buộc CHECK `source_ref <> ''` |
| C5 | Cột `source_hints` | Thuộc CR-REQ-004 (`0003`) | Không đưa vào `0002`; xem Q1 |

## 2. Giải pháp

### A. Migration `0002_request_core` (up/down, hai dialect)

Kiểu cột: Postgres `UUID`, `TIMESTAMPTZ`, `JSONB`, `NUMERIC(4,3)`, `TEXT`; MySQL `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `DECIMAL(4,3)`, `VARCHAR(n)` cho cột nằm trong khoá hoặc chỉ mục. Id do ứng dụng sinh (`uuid.NewString()`), không `DEFAULT gen_random_uuid()`.

Bảng (đầy đủ cột ở CR-REQ-002 mục 2.1; dưới đây là phần quyết định thêm):

```sql
-- Postgres, đầu file: tạo hàm kiểm tập giá trị bằng CHECK inline, không dùng ENUM
CREATE TABLE request.requests (
  id UUID PRIMARY KEY, tenant_id UUID NOT NULL, project_id UUID,
  number BIGINT NOT NULL,
  title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '',
  source_provider TEXT NOT NULL CHECK (source_provider IN ('jira','github','gitlab','linear','mcp','manual','webhook')),
  source_ref TEXT NOT NULL DEFAULT '', source_url TEXT NOT NULL DEFAULT '', source_site TEXT NOT NULL DEFAULT '',
  type TEXT CHECK (type IN ('change_request','bug','hotfix','task','spike','question','refactor','security','performance','docs','ops_request')),
  type_source TEXT CHECK (type_source IN ('ai','human')),
  size TEXT CHECK (size IN ('S','M','L')),
  urgency TEXT NOT NULL DEFAULT 'normal' CHECK (urgency IN ('normal','urgent')),
  confidence NUMERIC(4,3) CHECK (confidence >= 0 AND confidence <= 1),
  classification_reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','classifying','awaiting_type_confirmation','analyzing',
     'awaiting_analysis_approval','planning','awaiting_plan_approval','executing','completed','request_backlog','cancelled')),
  returned_from_stage TEXT CHECK (returned_from_stage IN ('classification','analysis','plan','phase','task')),
  return_reason TEXT NOT NULL DEFAULT '',
  plan_task_id UUID, reporter_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  version BIGINT NOT NULL DEFAULT 1,
  CONSTRAINT requests_backlog_stage CHECK ((status = 'request_backlog') = (returned_from_stage IS NOT NULL)),
  UNIQUE (tenant_id, number));
```

Hằng số giá trị của CHECK phải trùng 1-1 với hằng Go trong `domain` (test hợp đồng ở TASK-REQ-002-06). MySQL: `status VARCHAR(40)`, `title VARCHAR(500)`, `source_provider VARCHAR(20)`, `source_ref VARCHAR(255)`, `source_site VARCHAR(255)`, các cột còn lại `VARCHAR(≤40)`; `CHECK` có hiệu lực từ MySQL 8.0.16 (xem rủi ro).

Các bảng còn lại: `request_counters(tenant_id PK, next_number BIGINT NOT NULL)`; `request_type_history`; `solutions` (kiểu `options JSONB/JSON NOT NULL`, `chosen_option INT NULL`, `version BIGINT DEFAULT 1`); `request_links` (PK `(tenant_id, parent_request_id, child_request_id)`, CHECK cha khác con); `request_idempotency` (PK `(tenant_id, source_provider, source_site, source_ref)`, `request_id NOT NULL`).

Chỉ mục `requests`: `(tenant_id, status, updated_at DESC)`, `(tenant_id, project_id, status)`, `(tenant_id, plan_task_id)`, `(tenant_id, source_provider, source_site, source_ref)`, và cho phân trang keyset `(tenant_id, created_at DESC, id DESC)`. MySQL không có `DESC` trong chỉ mục trước 8.0 (8.0 trở lên có); dùng cú pháp 8.0, ghi vào rủi ro.

Postgres: mỗi bảng `ENABLE` + `FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` (USING + WITH CHECK, `NULLIF`) qua khối `DO $$ ... FOREACH` như `mcp-service/0001`. `GRANT` không cần (cùng owner). Down: `DROP TABLE` ngược thứ tự (Postgres) và `DROP TABLE` (MySQL).

### B. Domain (`internal/domain`, mới; tên file theo khái niệm)

```
request.go                 # type Request struct; NewRequest(in NewRequestInput) (Request, error)
request_type.go            # type RequestType string; 11 hằng; ParseRequestType; AllRequestTypes()
request_status.go          # type RequestStatus string; 11 hằng; ParseRequestStatus; IsTerminal()
request_size_urgency.go    # RequestSize (S|M|L), Urgency (normal|urgent), ParseSize
request_source.go          # SourceProvider, SourceRef{Provider, Site, Ref, URL}
request_type_change.go     # type RequestTypeChange struct (from_type nullable, actor_kind)
solution.go                # Solution, SolutionKind, SolutionStatus
request_link.go            # RequestLink, LinkReason
request_errors.go          # constructor lỗi REQUEST_*, SOLUTION_*
```

`Request.Type` là `*RequestType` hoặc `RequestType` rỗng? Chọn **chuỗi rỗng nghĩa là chưa có loại** (`RequestType("")`), vì repository ánh xạ `NULL` sang rỗng và ngược lại; `ParseRequestType("")` trả lỗi, còn `Request.HasType()` kiểm rỗng. Tránh con trỏ để `switch` đơn giản.

Mã lỗi (`common/apperrors`, bảng ở CR-REQ-002 mục 2.2): `REQUEST_TENANT_REQUIRED`, `REQUEST_INVALID_TYPE|STATUS|SIZE`, `REQUEST_TITLE_REQUIRED`, `REQUEST_NOT_FOUND`, `REQUEST_VERSION_CONFLICT`, `REQUEST_SOURCE_ALREADY_EXISTS`, `REQUEST_LINK_SELF`, `SOLUTION_NOT_FOUND`, `SOLUTION_VERSION_CONFLICT`. Chỉ khai báo constructor ở `request_errors.go`; mã của CR-REQ-003 đến 006 do các CR đó thêm vào cùng file.

### C. Cổng repository (`internal/usecase/ports.go`, bổ sung)

```go
type RequestRepository interface {
    Create(ctx context.Context, r domain.Request) error          // number đã cấp bởi NextNumber
    Get(ctx context.Context, id string) (domain.Request, error)
    GetByNumber(ctx context.Context, number int64) (domain.Request, error)
    List(ctx context.Context, f ListFilter) (ListResult, error)
    Update(ctx context.Context, r domain.Request, expectedVersion int64) (domain.Request, error) // CAS
    NextNumber(ctx context.Context) (int64, error)
}
type ListFilter struct {
    ProjectID, ReporterID, SourceProvider, SourceSite, SourceRef string
    Statuses []domain.RequestStatus; Types []domain.RequestType
    PageSize int; PageToken string // PageSize mặc định 50, tối đa 200
}
type ListResult struct{ Requests []domain.Request; NextPageToken string }
type RequestTypeHistoryRepository interface { Append(ctx, domain.RequestTypeChange) error; List(ctx, requestID string) ([]domain.RequestTypeChange, error) }
type SolutionRepository interface { Insert(ctx, domain.Solution) error; Get(ctx, id string) (domain.Solution, error); ListByRequest(ctx, requestID string) ([]domain.Solution, error); Update(ctx, domain.Solution, expectedVersion int64) (domain.Solution, error) }
type RequestLinkRepository interface { Insert(ctx, domain.RequestLink) error; ListChildren(ctx, parentID string) ([]domain.RequestLink, error); ListParents(ctx, childID string) ([]domain.RequestLink, error) }
type RequestIdempotencyRepository interface {
    Claim(ctx context.Context, ref domain.SourceRef, requestID string) (existingRequestID string, claimed bool, err error)
    Find(ctx context.Context, ref domain.SourceRef) (requestID string, found bool, err error) // CR-REQ-004 cần
}
```

`ListFilter` có sẵn bốn trường nguồn để CR-REQ-004 không phải sửa lại cổng. `Find` được thêm trước vì CR-REQ-004 bước 2 cần tra khoá không chiếm. `PageToken` là base64 của `(created_at, id)`; token sai trả `REQUEST_INVALID_PAGE_TOKEN` (InvalidArgument).

### D. Hành vi theo dialect

| Hành vi | Postgres | MySQL |
|---------|----------|-------|
| `NextNumber` | `INSERT ... ON CONFLICT (tenant_id) DO UPDATE SET next_number = request_counters.next_number + 1 RETURNING next_number` | `INSERT ... ON DUPLICATE KEY UPDATE next_number = next_number + 1`, rồi `SELECT next_number ... WHERE tenant_id = ?` cùng giao dịch (hàng bị khoá tới commit) |
| `Update` CAS | `UPDATE ... SET ..., version = version + 1, updated_at = now() WHERE id=$1 AND tenant_id=$2 AND version=$3`; 0 hàng thì `SELECT` phân biệt `NOT_FOUND` / `VERSION_CONFLICT` | như trên với `NOW(6)`; `version` luôn đổi nên `RowsAffected` đáng tin kể cả không bật `clientFoundRows` |
| `Claim` | `INSERT ... ON CONFLICT DO NOTHING`, không chèn được thì `SELECT request_id` | `INSERT IGNORE`; kiểm `RowsAffected()==0` rồi `SELECT` |
| `List` keyset | `WHERE (created_at, id) < ($1,$2)` (so sánh hàng) | `WHERE (created_at < ? OR (created_at = ? AND id < ?))`; MySQL cũ không tối ưu so sánh hàng |
| JSON | `[]byte` vào `jsonb` | `[]byte` vào `JSON`; luôn ghi `[]` tường minh, MySQL không cho `DEFAULT` literal trên JSON ở mọi bản |
| Tenant | `set_config` trong `InTx` (SOL-001 2.D) + `tenant_id` trong mọi `WHERE` | chỉ `tenant_id` trong mọi `WHERE` |

Thua race `Claim` trong `Create`: giao dịch người thua rollback toàn bộ (kể cả số Request đã cấp), nên không lỗ hổng số.

### E. gRPC đọc

SOL-001 để `GetRequest`, `ListRequests` có message nhưng trả `Unimplemented`. Solution này nối chúng với `RequestRepository` (use case mỏng `GetRequest`, `ListRequests` trong `usecase/`): `GetRequest` id lạ hoặc tenant khác đều `REQUEST_NOT_FOUND` (không lộ tồn tại). Ánh xạ domain sang proto ở `adapter/grpc/request_mapper.go`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| `type` nullable ở DB, chuỗi rỗng ở Go | Request mới chưa có loại; không ép giá trị giả |
| Số theo tenant bằng bảng bộ đếm | `COUNT+1` đua nhau; sequence khác nhau giữa dialect |
| Khoá idempotency tách bảng, `source_site NOT NULL DEFAULT ''` | Một khoá giống nhau hai dialect, không cần `COALESCE` |
| Khoá không có `project_id` | Theo README v6 mục 3.5 (xem Q2) |
| CAS bằng `version`, không `SELECT FOR UPDATE` | Giống nhau hai dialect; khớp `approvals.version` |
| Một bộ test hợp đồng chạy cho cả hai adapter | Tránh lệch hành vi hai dialect |
| `Find` thêm vào cổng idempotency | CR-REQ-004 cần |

## 4. Phụ thuộc và thứ tự

Cần SOL-001 xong: module, `TxRunner`, `OutboxWriter`, migration `0001`. Mở khoá SOL-003 (state machine dùng `Update` CAS), 004 (idempotency), 005 (history), 006 (links), CR-REQ-007 (solutions), CR-REQ-009. Thứ tự task: 01 migration, 02 domain, 03 ports, rồi 04 (Postgres) và 05 (MySQL) song song, 06 bộ test hợp đồng, 07 gRPC đọc.

## 5. Kiểm thử

- **Unit:** `ParseRequestType`, `ParseRequestStatus`, `NewRequest` (title rỗng, thiếu tenant); ánh xạ lỗi sang gRPC qua `apperrors.ToGRPCStatus`; mã hoá và giải mã `PageToken`.
- **Integration, từng dialect, cùng một bộ kịch bản** (`request_repository_contract_test.go` nhận interface repo): CHECK từ chối từng giá trị sai; 20 `Create` đồng thời cho 20 số liên tiếp; hai `Claim` đồng thời; CAS cũ; tenant A không đọc, không cập nhật được dòng B; keyset ổn định khi chèn giữa hai trang; `request_links` từ chối cha trùng con; lịch sử đúng thứ tự `at`.
- **Hợp đồng schema:** đọc `information_schema` hai DB, so tên bảng, tên cột, tính NULL với danh sách trong test.
- **Postgres RLS:** role `NOSUPERUSER NOBYPASSRLS`, SQL trực tiếp cũng không thấy dòng tenant khác.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- CHECK trên MySQL chỉ có hiệu lực từ 8.0.16; bản cũ phân tích rồi bỏ qua. Phiên bản MySQL và TiDB mục tiêu chưa xác định (CR-DB-002 chỉ nêu `SKIP LOCKED` cần 8.0.1). TiDB chưa kiểm chứng.
- `request_counters` tuần tự hoá tạo Request trong tenant; chưa đo ở tải import hàng loạt.
- `source_site` Jira cần chuẩn hoá (CR-REQ-004) nếu không cùng issue sinh hai khoá.
- `plan_task_id` không FK: Task bị xoá để lại id mồ côi, kiểm ở tầng ứng dụng (CR-REQ-013).
- Chỉ mục `DESC` trong MySQL 8.0 chưa kiểm với TiDB.

## 7. Câu hỏi mở

- **Q1.** `source_hints` vào `0002` hay `0003` riêng như CR-REQ-004: vì service chưa phát hành, gộp vào `0002` bớt một migration; theo CR hiện tại giữ `0003` (xem SOL-004).
- **Q2.** Khoá không có `project_id` nghĩa là một issue Jira chỉ tạo một Request cho cả tenant, dù nhiều project Orca cùng ánh xạ một project Jira (`task_sources` dùng khoá theo project). Cần chốt trước khi `0002` merge, vì đổi khoá sau là migration phá.
- **Q3.** `requests.type` nullable chưa ghi trong README v6 mục 3.5 (mục 8 điểm 3 đã chấp nhận); xác nhận.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v6/README.md` mục 3.2, 3.3, 3.5, 8
- `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0012_task_sources.up.sql`, `0014_task_sources_site.up.sql` và bản `mysql`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/postgres/task_sources.go`, `adapter/mysql/task_sources.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/task_source_ports.go`
- `/opt/repos/orca/backend-go/services/mcp-service/migrations/postgres/0001_init.up.sql` (RLS)
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go`, `common/tenant/tenant.go`, `common/dbcapability/capability.go`
- `/opt/repos/orca/docs/crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md`
