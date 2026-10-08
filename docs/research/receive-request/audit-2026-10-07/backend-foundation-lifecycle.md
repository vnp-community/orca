# Kiểm toán thực thi: request-service-foundation và request-lifecycle (backend-go)

Ngày: 2026-10-07. Phạm vi: TASK-REQ-001-* và 002-* (13 task), TASK-REQ-003-* đến 006-* (28 task), tổng 41 task. Code: `backend-go/services/request-service`, proto `backend-go/proto/orca/request/v1`, gen `proto/gen/go/orca/request/v1`, wiring `cmd/server/main.go`.

## 1. Lệnh đã chạy và kết quả thật

Trong `backend-go/services/request-service`:

| Lệnh | Kết quả |
|---|---|
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test ./... -count=1` | Pass, 8 package có test, 57 test PASS, 0 SKIP, 0 FAIL |
| `go vet -tags integration ./...` và `go test -tags integration ./...` | **FAIL build** ở `internal/adapter/mysql` và `internal/adapter/postgres`: `outbox_test.go` import thừa (`context`, `time`, `uuid`, `domain`) |

Ghi chú về test:
- Không có test bị skip. Test tích hợp nằm sau build tag `integration` (7 file, đều về approvals và outbox), nên `go test` thường không chạy chúng. Với tag `integration` thì không biên dịch được, và cũng cần DB thật nên không chạy được ở đây. Test tích hợp cho request/transition/create/classification/lifecycle: không có file nào.
- `cmd/server/mysql_dsn_test.go` là test rỗng (`// Stub test`), pass giả.
- `internal/adapter/grpc/server_test.go` chỉ khẳng định `GetRequest` trả `Unimplemented`.
- Không có test cho domain `Request`, `RequestStatus`, repository request, usecase request. 57 test thuộc config, approvals, openspec, page token, opaclient.

## 2. Tóm tắt theo solution

| Solution | Số task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| SOL-001 scaffold (001-01..06) | 6 | 3 | 3 | 0 | 0 | 50% |
| SOL-002 data model, repo (002-01..07) | 7 | 1 | 2 | 4 | 0 | 14% |
| SOL-003 state machine, flow registry (003-01..06) | 6 | 0 | 1 | 5 | 0 | 0% |
| SOL-004 intake (004-01..08) | 8 | 0 | 0 | 8 | 0 | 0% |
| SOL-005 classification (005-01..08) | 8 | 0 | 0 | 8 | 0 | 0% |
| SOL-006 backlog, reopen, cancel, child (006-01..06) | 6 | 0 | 0 | 6 | 0 | 0% |
| **Tổng** | **41** | **4** | **6** | **31** | **0** | **10%** |

Cả 41 task đang ghi `[x]` (mọi tiêu chí). 37 task có trạng thái sai (verdict khác "Đủ").

## 3. Bảng từng task

Cột trạng thái: `[x]` là mọi tiêu chí đã tick.

| Task | Trạng thái | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 001-01 module, config, workspace | [x] | Đủ | `go.work:22`, `Makefile:10`, `deploy/postgres-init-databases.sh:8`, `internal/config/config.go`, `config_test.go` (Defaults, Overrides, InvalidBool) | README bảng real vs stub còn cũ (chỉ ghi `config`) |
| 001-02 proto request và approval | [x] | Đủ | `proto/orca/request/v1/{request,approval,request_backlog}.proto`, `proto/gen/go/.../*.pb.go` | Chưa chạy buf lint; request.proto chỉ có 3 RPC (GetRequest, ListRequests, ListBacklog) |
| 001-03 migration 0001 hai dialect | [x] | Đủ | `migrations/{postgres,mysql}/0001_init.{up,down}.sql`; pg: ENABLE+FORCE RLS, policy `app.tenant_id`, relay policy | Chưa áp dụng lên DB thật (không dựng được) |
| 001-04 tx runner, outbox adapter | [x] | Một phần | `postgres/tx.go:15-39` (`set_config('app.tenant_id',$1,true)`), `postgres/outbox.go`, `mysql/tx.go`, `mysql/outbox.go` | Code thật nhưng test `outbox_test.go` (cả hai) không biên dịch với tag `integration`; không có test chạy được |
| 001-05 main, gRPC server, health | [x] | Một phần | `cmd/server/main.go:104-172` | `RequestService` là stub `Unimplemented` (`grpc/server.go:19-24`). Mọi handler approval là Noop và main trả lỗi khi `AllowNoopApprovalHandlers=false` (mặc định, `main.go:150-161`): service **không khởi động được với cấu hình mặc định**. `approvalRepo` chỉ gán rồi `_ =` (`main.go:166`) |
| 001-06 deploy dev và CI | [x] | Một phần | `deploy/Dockerfile`, `ci/check-opa-bundle-in-images.sh:6` | Không có `.github/workflows/backend-go-request-service.yml` (các service khác có), không có mục trong `docker-compose.yml` (grep `request-service`) |
| 002-01 migration request core | [x] | Một phần | `migrations/{postgres,mysql}/0005_request_core.up.sql` đủ 6 bảng, CHECK, RLS pg | Số là 0005 chứ không phải 0002. **Thứ tự migration vỡ**: `0002_approvals.up.sql:3` có `REFERENCES request.requests(id)` trong khi `requests` chỉ tạo ở 0005, migrate Postgres sẽ lỗi ở 0002. Thiếu cột mà `backlog_requests.go` truy vấn |
| 002-02 domain entities, errors | [x] | Một phần | `domain/request.go`, `request_status.go`, `request_type.go`, `request_errors.go`, `request_link.go` | Không có test domain nào. `NewRequest` không kiểm `reporter_id` rỗng (khối `if` rỗng, `request.go:59-62`) và từ chối `source_provider` rỗng; có comment nháp "Wait..." (`request.go:63-70`) |
| 002-03 repository ports, list filter | [x] | Đủ | `usecase/ports.go:133-175` (`ListFilter`, `Normalize`, `RequestRepository`, link, idempotency ports), `page_token_test.go:69` | Chỉ là interface; không có implementation |
| 002-04 postgres repositories | [x] | Chưa làm | `postgres/request_repository.go` là `type RequestRepository struct {}` (3 dòng) | Không có Create/Get/List/Update/NextNumber/link/idempotency |
| 002-05 mysql repositories | [x] | Chưa làm | `mysql/request_repository.go` 3 dòng, struct rỗng | Như trên |
| 002-06 repository contract suite | [x] | Chưa làm | `find . -name '*contract*'` chỉ có `approval_subject_handler_contract_test.go` | Không có suite repo request |
| 002-07 wire GetRequest, ListRequests | [x] | Chưa làm | `grpc/server.go:19-24` trả `codes.Unimplemented`; `usecase/get_list_requests.go` trả `nil, nil` | Không nối repo vào server |
| 003-01 flow registry | [x] | Một phần | `domain/backlog_gate.go:84-101` `FlowFor` ghi chú "Temporary stubs", test `openspec_profile_test.go:31` | Không có registry, không có file flow riêng; hằng cứng 2 nhánh |
| 003-02 triggers, transition table | [x] | Chưa làm | grep `trigger|transition` trong domain: không có bảng | Không có bảng chuyển trạng thái |
| 003-03 TransitionRequest usecase | [x] | Chưa làm | `usecase/transition_request.go` 9 dòng, `Execute` `return nil` | No-op |
| 003-04 transition integration tests | [x] | Chưa làm | Không có test chạm transition | Không có |
| 003-05 GetRequestFlow RPC | [x] | Chưa làm | `request.proto` không có `GetRequestFlow` | Không có |
| 003-06 status write guard, contract tests | [x] | Chưa làm | Không có script/test chốt chặn. grep `UPDATE requests` ngoài approvals: không có | Xem mục 4 |
| 004-01 migration 0003 source hints | [x] | Chưa làm | `ls migrations/*` không có migration source hints (0003 là approval_policies) | Không có |
| 004-02 source normalization, idempotency key | [x] | Chưa làm | `domain/source_hints.go` struct 2 trường; `source_catalog.go` là catalog context source, không liên quan | Không có normalize, key |
| 004-03 proto CreateRequest, source filters | [x] | Chưa làm | `request.proto:59-62` không có CreateRequest; `ListRequestsRequest` không có source filter | `// 6 to 8 reserved` chỉ là chú thích |
| 004-04 CreateRequest usecase | [x] | Chưa làm | `usecase/create_request.go` trả `"", nil` | No-op |
| 004-05 issue tracking client | [x] | Chưa làm | `grpcclient/` chỉ có dev_server, infra, proposal, task, team (stub) | Không có client issue-tracking |
| 004-06 gRPC CreateRequest, filters | [x] | Chưa làm | `grpc/server.go` | Không có |
| 004-07 CreateRequest integration tests | [x] | Chưa làm | Không có | Không có |
| 004-08 gateway webhook route | [x] | Chưa làm | `api-gateway/internal/adapter/httpgateway` không có route request webhook (chỉ `scm_webhook_routes.go`); `cmd/server/request_wiring.go` chỉ dial | Không có |
| 005-01 domain classification rules | [x] | Chưa làm | `domain/classification.go` 6 dòng, struct 2 trường | Không có luật |
| 005-02 classifier port, prompt, relay client | [x] | Chưa làm | grep `classif` chỉ ra `classification.go`, `propose_request_classification.go` | Không có |
| 005-03 processed_events, status-changed consumer | [x] | Chưa làm | Bảng `processed_events` có ở 0001, nhưng grep `ProcessedEvent` trong Go: không có; `handle_request_status.go` `return nil` | Không có consumer |
| 005-04 ProposeRequestClassification | [x] | Chưa làm | `usecase/propose_request_classification.go` 9 dòng no-op | No-op |
| 005-05 ConfirmRequestType | [x] | Chưa làm | grep `ConfirmRequestType`: không có | Không có |
| 005-06 ChangeRequestType | [x] | Chưa làm | `usecase/change_request_type.go` 9 dòng no-op | No-op |
| 005-07 proto, gRPC handlers classification | [x] | Chưa làm | `request.proto` không có RPC | Không có |
| 005-08 classification integration tests | [x] | Chưa làm | Không có | Không có |
| 006-01 migration return history, repos | [x] | Chưa làm | Không có migration; `request_links` có ở 0005 nhưng chưa có repo | Không có |
| 006-02 ReturnToBacklog | [x] | Chưa làm | `usecase/return_to_backlog.go` no-op | No-op |
| 006-03 Reopen, Cancel | [x] | Chưa làm | `usecase/reopen_request.go` hai hàm `return nil` | No-op |
| 006-04 SpawnChildRequest | [x] | Chưa làm | `usecase/spawn_child_request.go` trả `"", nil` | No-op |
| 006-05 proto, gRPC handlers, list links | [x] | Chưa làm | `request.proto` không có | Không có |
| 006-06 lifecycle exit integration tests | [x] | Chưa làm | Không có | Không có |

## 4. Stub và vấn đề chất lượng

Use case request là vỏ rỗng, không nối wiring và không được gọi từ đâu:
- `usecase/transition_request.go:5-8`, `create_request.go:11-13`, `propose_request_classification.go`, `change_request_type.go`, `return_to_backlog.go`, `reopen_request.go:5-11`, `spawn_child_request.go`, `get_list_requests.go:5-10`, `handle_request_status.go`, `lookup_request_by_source.go`: đều `return nil` hoặc `return "", nil`. Chữ ký dùng `string`/`interface{}` thay cho kiểu domain.
- `adapter/postgres/request_repository.go`, `adapter/mysql/request_repository.go`: struct rỗng.
- `adapter/grpc/server.go:19-24`: `GetRequest`, `ListRequests` trả `Unimplemented`. `adapter/grpc/approval_server.go:23-47`: 7 RPC `Unimplemented`.
- `cmd/server/main.go:150-161`: đăng ký `NoopSubjectHandler{Reason:"Not implemented"}` cho mọi loại subject, rồi từ chối khởi động nếu `REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS` không bật. Kết quả: mặc định service thoát ngay khi chạy.

Vấn đề nghiêm trọng khác:
- Migration Postgres không chạy được theo thứ tự: `0002_approvals.up.sql` có FK tới `request.requests` (tạo ở `0005_request_core.up.sql`). MySQL không FK nên không lỗi, hai dialect lệch nhau ở điểm này.
- `postgres/backlog_requests.go:24,51` và `mysql/backlog_requests.go:23` truy vấn cột `stage`, `returned_category` không có trong bảng `requests` của 0005 (và không migration nào thêm): lỗi SQL lúc chạy.
- `go test -tags integration` không biên dịch được (`postgres/outbox_test.go`, `mysql/outbox_test.go` import thừa), nghĩa là toàn bộ test tích hợp (approvals, outbox) chưa từng chạy được ở trạng thái hiện tại.
- Pass giả: `cmd/server/mysql_dsn_test.go` rỗng.
- `internal/usecase/rls_tenant_isolation.go` (7 dòng), `request_audit_recorder.go` (7 dòng), `type_policy.go` (9 dòng) cũng là vỏ mỏng.
- Stub rải trong approvals (ngoài phạm vi, nhưng nằm cùng service): `postgres|mysql/approval_policy_repository.go` `return nil // Stub`, `resolve_approver_policy.go:25-26` size/urgency cứng, `authorize_approval_decision.go:29`, `grpcclient/*` trả `nil, nil`.

### Kiểm tra riêng theo yêu cầu

- **Hai dialect, khớp nhau**: có đủ `migrations/postgres` và `migrations/mysql` cho 0001..0007, cặp up/down đầy đủ. 0001 và 0005 khớp về cột, kiểu tương đương, CHECK, UNIQUE, index. Khác biệt có chủ ý: MySQL không schema, không RLS. Không có 0003 source hints, không có migration return history ở cả hai dialect. Lỗi thứ tự 0002 nêu ở trên chỉ ở Postgres.
- **RLS thật**: có. `0001_init` và `0005_request_core` (pg) chạy `ENABLE` và `FORCE ROW LEVEL SECURITY`, policy `tenant_isolation` dùng `current_setting('app.tenant_id', true)` cho USING và WITH CHECK; outbox có thêm policy `relay_read`, `relay_mark_published` theo `app.relay`. `InTx` gọi `set_config('app.tenant_id', $1, true)` (`postgres/tx.go:26`) nhưng **bỏ qua im lặng** nếu context không có tenant (`if ... err == nil && tenantID != ""`), và `exec()` dùng pool trần khi ngoài tx, nên truy vấn ngoài `InTx` không có tenant (FORCE RLS sẽ trả 0 dòng hoặc lỗi, khó chẩn đoán). Chưa có test RLS nào chạy (cần DB). Bảng approvals (0002) cũng `FORCE`, các migration còn lại chưa rà từng dòng.
- **CHECK**: có đủ ở 0005 cả hai dialect: type (11 giá trị), type_source, size, urgency, confidence 0..1, status (11 giá trị khớp `domain.AllRequestStatuses`), returned_from_stage, `requests_backlog_stage`, actor_kind, link reason, `parent <> child`, `source_ref <> ''`. MySQL: README trong migration tự ghi CHECK bị bỏ qua trước 8.0.16; không có test kiểm chứng.
- **Chốt chặn ghi `status` ngoài `transition_request.go`**: **không có**. Không có script lint, test kiến trúc, hay kiểm tra README. Hiện không có đường ghi `requests.status` nào trong code (không có `UPDATE requests`), nên chưa vi phạm, nhưng chỉ vì repository chưa tồn tại. `transition_request.go` là no-op.

## 5. Lệch giữa tài liệu và code

- Task 002-01 nói migration `0002`; code là `0005_request_core` vì 0002..0004 đã bị approvals và context sources chiếm. Task 004-01 (0003) và 006-01 sẽ lệch số tương tự.
- `FlowFor` và `PhasesFor` nằm trong `domain/backlog_gate.go` với ghi chú tạm, không phải flow registry như task 003-01.
- `domain.Request` có thêm `Stage`, `ReturnedCategory`, `SolutionEngine` mà bảng `requests` (0005) không có cột tương ứng (`solution_engines` ở 0007 chưa kiểm).
- `README.md` của service vẫn chỉ ghi `config` là thật, không phản ánh các phần đã có (approvals, outbox) hay stub.
- Cả 41 task và các solution đều ghi hoàn thành. Thực tế phần lifecycle (003-006) gần như chưa bắt đầu.

## 6. Việc còn lại (theo ưu tiên)

1. Sửa thứ tự migration (FK `approvals` -> `requests`): chuyển tạo `requests` lên trước hoặc bỏ FK, thêm cột còn thiếu cho `backlog_requests`.
2. Cho service khởi động được: hoặc thay Noop handler bằng handler thật, hoặc bật cờ mặc định ở dev; sửa `main.go` nối `RequestRepository` vào gRPC server.
3. Viết repository Postgres và MySQL cho request (002-04, 002-05) cùng contract suite (002-06), và nối `GetRequest`, `ListRequests` (002-07).
4. Viết `TransitionRequest` thật với bảng chuyển trạng thái và flow registry (003-01..03); thêm chốt chặn ghi `status` (003-06).
5. Sửa test tích hợp để biên dịch được, thêm CI workflow và docker-compose cho request-service (001-06), rồi viết test tích hợp hai dialect.
6. Làm intake (004), classification (005), backlog/reopen/cancel/child (006), kể cả proto, gateway webhook route.
7. Đổi trạng thái 37 task đang tick sai về chưa xong; xóa `mysql_dsn_test.go` rỗng hoặc viết test thật.

## 7. Không kiểm chứng được

- Mọi test tích hợp (cần Postgres/MySQL/NATS): không chạy được ở đây, và còn không biên dịch được với tag `integration`.
- Áp dụng migration lên DB thật, hành vi RLS và CHECK MySQL: chưa chạy, chỉ đọc SQL. Lỗi FK 0002 suy ra từ đọc SQL, chưa chạy `migrate`.
- Buf lint và tương thích proto: chưa chạy.
