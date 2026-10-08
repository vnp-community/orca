# Ghi chú triển khai: request-service-foundation (đợt R1a)

Ngày: 2026-10-07. Phạm vi: TASK-REQ-001-01..06 và TASK-REQ-002-01..07 (BE-REQ-SOL-001, 002).

## 1. Đánh số lại migration (một lần, đã đóng băng)

Theo D1 của `IMPLEMENTATION-PLAN.md`. Hai dialect dùng cùng một bảng số. Từ nay chỉ thêm `0008` trở đi.

| Số mới | Tên | Số cũ | Ghi chú |
|---|---|---|---|
| 0001 | `0001_init` | 0001 | outbox, processed_events (không đổi) |
| 0002 | `0002_request_core` | 0005 | lên trước `approvals` vì `approvals`, `analysis_runs`, `context_packs`, `openspec_changes` có FK tới `requests` |
| 0003 | `0003_approvals` | 0002 | FK tới `requests` nay hợp lệ; MySQL thêm `fk_approvals_request` cho khớp Postgres |
| 0004 | `0004_approval_policies` | 0003 | |
| 0005 | `0005_context_sources` | 0004 | MySQL: `REFERENCES` viết cạnh cột bị MySQL bỏ qua, đổi thành `CONSTRAINT ... FOREIGN KEY` thật |
| 0006 | `0006_analysis_runs` | 0006 | xem sửa lỗi bên dưới |
| 0007 | `0007_solution_engines` | 0007 | xem sửa lỗi bên dưới |

Lỗi đã sửa trong lúc đánh số lại (mỗi lỗi đã tái hiện trên DB thật trước khi sửa hoặc suy ra từ SQL, ghi rõ):
- `0006_analysis_runs` Postgres: thiếu `FORCE ROW LEVEL SECURITY`, GUC sai `orca.tenant_id` (đúng là `app.tenant_id`), thiếu `NULLIF`, thiếu `WITH CHECK`. Đã sửa, có test RLS với role `NOBYPASSRLS` (`TestPostgres_RLS_DirectSQLCannotSeeOtherTenant`).
- Cột `engine` của `analysis_runs` bị thêm hai lần (0006 và 0007), up của 0007 sẽ lỗi "column already exists". Giữ ở 0007, bỏ khỏi 0006 (hai dialect).
- `0007` MySQL: `ALTER TABLE analysis_runs DROP CHECK analysis_runs_chk_1` sai vì `mode` là `ENUM`, không có CHECK nào. Đã bỏ; `0007` Postgres cũng bỏ đoạn drop/add CHECK vô nghĩa (0006 đã có `agent_proposal`). Down của 0007 chỉ gỡ thứ mà up thêm.
- `0004_approval_policies`: CHECK `request_type` dùng bộ giá trị cũ (`feature`, `incident`...) không khớp 11 loại thật (`bug`, `hotfix`, `security`...). Đổi sang 11 loại của `domain.AllRequestTypes()` (hai dialect), nếu không chính sách theo loại không thể lưu được.
- `0002_request_core`: `source_provider` thêm `CHECK` 7 giá trị và bỏ `DEFAULT ''` (task 002-01 yêu cầu CHECK này, bản cũ thiếu). `request_type_history` thêm cột `reason` (domain `RequestTypeChange.Reason` có, bảng cũ không có chỗ lưu; thêm để khỏi âm thầm mất dữ liệu). Lệch nhỏ so với CR-REQ-002 mục 2.1.
- Policy tenant của `0007` Postgres thêm `WITH CHECK` tường minh.
- `domain.ActorKindAI = "ai"` đổi thành `ActorKindAgent = "agent"` cho khớp CHECK `actor_kind IN ('user','agent','system')` (không có chỗ nào dùng hằng cũ).

Kiểm chứng: up, down, up đầy đủ trên Postgres 16 và MySQL 8.0 thật (container), cả "down các số trên 0001 rồi up lại" lẫn "down toàn bộ rồi up" (`TestPostgres_Migration_UpDownUp`, `TestMySQL_Migration_UpDownUp`).

Cột mà `backlog_requests.go` truy vấn: `stage` thay bằng `returned_from_stage` (đã có). `returned_category` và bảng `request_return_history` thuộc CR-REQ-006 (migration sau này, `0008+`): khi đó bộ lọc theo category và `LatestReturns` báo lỗi `FailedPrecondition` rõ ràng thay vì lỗi SQL lúc chạy. Hai hàm này vẫn dùng `exec(ctx)` trần (không `scoped`) nên đợt R1b phải chuyển sang `scoped` khi nối RPC `ListBacklog`.

## 2. Quyết định kỹ thuật lệch hoặc bổ sung so với task

| # | Quyết định | Lý do |
|---|---|---|
| N1 | `RequestRepository`, `RequestTypeHistoryRepository`, `SolutionRecordRepository`, `RequestLinkRepository`, `RequestIdempotencyRepository` là struct riêng bọc `*Repository`, không cài trực tiếp lên `Repository` (task 002-04 viết `var _ usecase.RequestRepository = (*Repository)(nil)`) | `Repository` đã có `Insert`, `List`, `Update`... của approvals với chữ ký khác; trùng tên không biên dịch được |
| N2 | Cổng `SolutionCoreRepository` mới (`Insert/Get/ListByRequestID/Update`) thay vì cài `usecase.SolutionRepository` có sẵn | `SolutionRepository` hiện có cần cột `kind`, `status`, `content_ref`... mà bảng `solutions` của 0002 chưa có; đợt R2 (Solution) thêm cột và mở rộng. `domain.Solution.Kind/Status` chưa được lưu ở đợt này |
| N3 | `RequestTypeHistoryRepository` đổi `Insert` thành `Append` + `List` (đúng task); `List` sắp theo `at` (bảng không có `id` để phá hoà) | task 002-04 mục 4 |
| N4 | `InTx` bắt buộc tenant trong ctx, không bỏ qua im lặng; `InTx` lồng cùng tenant thì tham gia giao dịch ngoài, khác tenant trả `REQUEST_TX_TENANT_MISMATCH`. Mọi method repository mới đi qua `scoped()` (tự mở giao dịch ngắn có `set_config('app.tenant_id')` khi ngoài `InTx`). `InsertOutboxEvent` cũng qua `scoped()` và so tenant của sự kiện với ctx | audit mục 4 "RLS thật" |
| N5 | `FetchUnpublished` (hai dialect) trả đủ `tenant_id`, `occurred_at`, `version`, `id` trong `eventbus.Event`; bản cũ chỉ điền `Payload` nên mọi sự kiện publish ra thiếu tenant | phát hiện khi viết test outbox |
| N6 | MySQL `InsertOutboxEvent` truyền `payload` dạng `string` thay vì `[]byte` | cột `JSON` từ chối giá trị binary charset |
| N7 | DSN MySQL thêm `time_zone='+00:00'` (cùng `loc=UTC`) | cột `TIMESTAMP` đổi qua múi giờ phiên; ghim UTC để đọc khớp ghi |
| N8 | `GetRequest` coi id không phải UUID là `REQUEST_NOT_FOUND` (không `InvalidArgument`) và không gọi repository | task 002-07, tránh dò định dạng id |
| N9 | `domain.NewRequest` từ chối `reporter_id` rỗng và `source_provider` rỗng/lạ; thời gian cắt về micro giây | task 002-02; cột DB giữ micro giây nên round-trip phải bằng nhau |
| N10 | `Request.Stage` và `ReturnedCategory` của domain chưa có cột (CR-REQ-006); repository chưa lưu hai trường này | xem mục 1 |
| N11 | `NewServer(getRequest, listRequests)`; `ListRequests` kiểm giá trị `status`/`type` và báo `InvalidArgument` nếu sai thay vì trả trang rỗng | |
| N12 | Đã xoá `usecase.EnforceRLS` (hàm rỗng `return nil`, không ai gọi) | stub giả bảo mật; RLS thật nằm ở adapter |

### Approval và cách khởi động (việc bắt buộc số 4)

Bản cũ đăng ký `NoopSubjectHandler` cho cả 8 subject rồi từ chối chạy nếu `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS=false`, nên mặc định service thoát ngay. Quyết định mới (`cmd/server/approval_wiring.go`):
- `REQUEST_APPROVAL_ENABLED` mặc định `false`: không đăng ký `ApprovalService`, không cần handler, service chạy bình thường ở dev. RPC approval trả `Unimplemented` (gRPC tự trả vì chưa đăng ký).
- Khi bật: mỗi subject trong `REQUEST_APPROVAL_SUBJECTS` (rỗng là cả 8) phải có handler thật; thiếu thì lỗi khởi động nêu rõ subject. `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS=true` chỉ để dev, thay handler thiếu bằng Noop có log cảnh báo.
- Đợt R2 đăng ký handler thật bằng cách truyền map vào `buildApprovalRegistry` (hiện `nil`) trong `main.go`.
- Kiểm bằng: `TestBuildApprovalRegistry_*`, `TestRun_ApprovalEnabledWithoutHandlersRefusesToStart`, `TestRun_UnknownDSNExitsWithDialectError` (không cần DB) và `TestRun_StartsWithDefaultConfig` (Postgres thật, `-tags integration`).

### Việc đợt R2 phải biết
- `ExpireApprovals`/`RemindPendingApprovals` gọi `ClaimDue` xuyên tenant bằng pool trần rồi `InTx(ctx)` không có tenant: với `InTx` mới (bắt buộc tenant) và RLS thật, sweeper phải đặt `tenant.WithTenantID(ctx, claim.TenantID)` trước `InTx`, và `ClaimDue` cần đường đọc xuyên tenant riêng (kiểu `app.relay`). Chưa sửa vì thuộc approvals.
- Mọi repository approvals còn dùng `exec(ctx)` trần (không `scoped`); dưới RLS thật chúng cần chạy trong `InTx` có tenant.

## 3. CI, compose, deploy

- Workflow mới `.github/workflows/backend-go-request-service.yml` (ma trận `postgres`, `mysql`; `actionlint` sạch). `buf lint` chỉ chạy trên `request.proto` và `request_backlog.proto`: `approval.proto` đang vi phạm STANDARD (response dùng chung cho nhiều RPC), thuộc đợt Approval, lint cả package sẽ đỏ.
- `deploy/dev/docker-compose.yml`: thêm `request-service` và `migrate-request`; `deploy/dev/scripts/migrate.sh` và `build-local.sh` thêm `request`/`request-service`. `docker compose config` hợp lệ. Chưa chạy `docker build` Dockerfile và chưa chạy stack dev thật.
- `Makefile` `SERVICES`, `go.work`, `postgres-init-databases.sh` đã có sẵn `request-service`/`request` (kiểm lại, không đổi).
- `backend-go/docker-compose.yml` chỉ chứa hạ tầng (postgres, vault, nats), không có mục nào của service nên không đổi.

## 4. Test tích hợp

- `go test -tags integration` đã biên dịch lại được (`outbox_test.go` cũ chỉ là thân rỗng, đã thay bằng test thật). Các `approval_*_integration_test.go` vẫn là thân rỗng `// Stub test` (thuộc approvals, không đụng).
- Mỗi package adapter dùng một container dùng chung (`TestMain`) và một database riêng cho mỗi test, vì mỗi test một container quá chậm trên máy dùng chung.
- Bộ hợp đồng nằm ở `internal/adapter/contracttest` (không dùng tên `testutil`/`helpers`).

## 5. Chưa kiểm chứng

- MySQL 8.0 chỉ chạy qua container; chưa thử TiDB, chưa thử MySQL dưới 8.0.16 (CHECK bị bỏ qua, test CHECK tự `Skip` có lý do ở bản cũ).
- `EXPLAIN` của truy vấn keyset `(created_at, id) < (...)` chưa xem.
- `docker build` của `deploy/Dockerfile` và stack compose thật (có Vault dùng chung) chưa chạy; `migrate.sh request` chưa chạy trên compose.
- `buf breaking` chưa chạy (chưa có `origin/main` chứa package, CI tự bỏ qua lần đầu).
- Relay NATS: test xác nhận mọi dòng được đánh dấu đã publish sau khi `Publish` JetStream trả ack, chưa có subscriber đọc lại nội dung từng message.

## 6. Câu hỏi mở

- Bảng `solutions` (kind, status, content_ref) sẽ do đợt R2 mở rộng; cần quyết định có gộp `SolutionCoreRepository` vào `SolutionRepository` hay giữ hai cổng.
- `request_type_history` không có `id` nên không phá được hoà khi hai dòng cùng `at`; nếu cần thứ tự tuyệt đối, thêm cột `seq` ở `0008`.
