# Ghi chú triển khai: request-lifecycle

Mỗi agent ghi một phần riêng để hợp nhất không xung đột. Phần dưới là **life-a: CR-REQ-003 (máy trạng thái) và CR-REQ-006 (trả backlog, mở lại, hủy, Request con)**, ngày 2026-10-08. Phần life-b (CR-REQ-004, 005) do agent kia thêm bên dưới.

## A1. Cổng ổn định cho life-b và các feature khác

File `request-service/internal/usecase/ports_lifecycle.go`:

```go
type RequestTransitioner interface {
    Execute(ctx context.Context, in TransitionInput) (TransitionResult, error)
}
type TransitionInput struct {
    RequestID    string
    Trigger      domain.Trigger          // 16 hằng domain.Trigger*
    ExpectedFrom *domain.RequestStatus   // nil: không kiểm giao lặp
    ActorID      string
    ActorKind    domain.ActorKind        // user | agent | system
    Stage        domain.ReturnStage      // chỉ return_to_backlog (analysis_rejected/plan_rejected tự đặt)
    Category     domain.ReturnCategory   // bắt buộc khi đích là request_backlog
    Reason       string                  // bắt buộc cho analysis_rejected, plan_rejected, return_to_backlog, cancel
}
type TransitionResult struct { Request domain.Request; Applied bool }
```

- Dựng bằng `usecase.NewTransitionRequest(requests, txScope, outbox)`; bản dựng sẵn ở `cmd/server/wire_request_lifecycle.go` (`requestLifecycle.Transition`).
- `ExpectedFrom` = trạng thái người gọi đã thấy; gọi lặp cùng cặp (`ExpectedFrom`, `Trigger`) trả `Applied=false`, không ghi gì. Lệch và không phải giao lặp: `REQUEST_STATE_STALE`.
- Ngoài giao dịch: use case tự mở giao dịch và thử lại tối đa 3 lần khi `REQUEST_VERSION_CONFLICT`. Trong giao dịch của người gọi (`CreateRequest`, `ClassifyRequest`...): không thử lại, trả `REQUEST_VERSION_CONFLICT`.
- Đây là nơi duy nhất được gán `Request.Status`, `ReturnedFromStage`, `ReturnReason`, `ReturnedCategory` (test `TestOnlyTransitionRequestWritesStatus`). life-b: dùng cổng này cho `start_classification`, `proposal_ready`, `type_confirmed`, `type_change`; không gán `Status` ở `usecase`.
- Sự kiện: `orca.request.request.status_changed` (payload `usecase.StatusChangedPayload`, có `version`, `number`, `category`), thêm `orca.request.request.completed` khi đích là `completed`. Consumer phân loại (CR-REQ-005) lọc `to=classifying`, `trigger` là `start_classification` hoặc `reopen`.
- `TxScope` (TxRunner + `InTransaction(ctx) bool`) là interface riêng, không sửa `TxRunner`; cả hai `Repository` đã cài.

Cổng khác life-b cần biết: `ReturnHistoryRepository`, `ApprovalCanceller`, `ExecutionGuard`, `ClassificationAttemptsResetter`, `ChildRequestCreator`.

## A2. Quyết định lệch so với task

| # | Quyết định | Lý do |
|---|---|---|
| L1 | Migration `0020_request_return_history` (cột `returned_category`, CHECK ghép, bảng lịch sử) và `0021_request_links_child_reasons` (mở CHECK `reason` thêm 4 lý do con, thêm `created_by`, `created_at`) thay cho `0005` trong task | dải số cấp cho life-a là `0020`..`0024` |
| L2 | Category bắt buộc ngay từ `TransitionRequest` (task 003-03 chưa có, 006-02 thêm) | một migration gộp CHECK ghép; hai task không thể merge riêng |
| L3 | `TxScope` thay cho thêm `InTransaction` vào `TxRunner` | không đổi interface đang dùng; các fake khác không vỡ |
| L4 | Postgres 0020 tạm `NO FORCE ROW LEVEL SECURITY` để backfill rồi `FORCE` lại | chủ bảng cũng chịu FORCE RLS nên `UPDATE` không thấy dòng nào |
| L5 | MySQL 0021 tìm tên CHECK inline không tên của `reason` qua `information_schema` rồi `DROP CHECK` bằng câu lệnh chuẩn bị | tên `request_links_chk_N` do MySQL sinh, không cố định |
| L6 | `ChildRequestCreator` là cổng; cài sẵn `IdempotentChildCreator` (claim idempotency trước, rồi `NextNumber`, `Create`, outbox `created` kèm `parent_request_id`, `link_reason`, `type_hint`) | `CreateRequest.CreateWithinTx` (CR-REQ-004) chưa có ở nhánh này. Khoá: nguồn `manual\|mcp`, site `user:<actor>`, ref `<parent_id>:<client_request_id>` |
| L7 | `type_hint` của Request con chỉ nằm trong payload `created`, chưa lưu cột | cột hint thuộc CR-REQ-004. life-b: khi thay `IdempotentChildCreator` bằng `CreateWithinTx`, lưu hint và giữ chữ ký cổng |
| L8 | `ReopenRequest` gọi `ClassificationAttemptsResetter` (nil nếu chưa nối) thay vì ghi `ClassificationAttempts=0` trong `Update` | cột `classification_attempts` thuộc CR-REQ-005. life-b: cài cổng này trên repository và truyền vào `NewReopenRequest` ở `wire_request_lifecycle.go` |
| L9 | `ExecutionGuard` mặc định `NoActiveExecutionGuard` (không bao giờ chặn) | task-service chưa có RPC "Task đang chạy của Request" (CR-REQ-011/013). Chặn đã kiểm bằng guard giả |
| L10 | `ApprovalCanceller` thật = `PendingApprovalCanceller` bọc `CancelPendingApprovalsForRequest`; sửa use case này chịu `Registry == nil` (approval tắt mặc định) | tránh panic khi `REQUEST_APPROVAL_ENABLED=false` |
| L11 | `domain.SubjectRequestStatusChanged` đổi từ `...statuschanged` thành `...status_changed` | khớp consumer `issue-status-sync` và mọi CR; hằng cũ chưa có người dùng |
| L12 | `domain.Request.ReturnedCategory` đổi từ `string` sang `ReturnCategory`; `Request.Stage` giữ nguyên | chưa ai dùng; kiểu mạnh hơn |
| L13 | `FlowFor` trả `(FlowDefinition, error)` và `PhasesFor` thành method; `ResolveTaskGate` dùng registry thật | thay stub `backlog_gate.go`. Hành vi đổi: trước đây `PhasesFor` là `size == "L"` cho mọi loại, nay theo luật của loại (`change_request` luôn có Phase). `FlowDefinition` giữ thêm `OpenSpecProfile` (cộng thêm) |
| L14 | Backlog reader: lọc `returned_category` và `LatestReturns` chạy thật (qua `scoped`); `ListReturnedRequests`, `ParentRequestIDs` vẫn `exec(ctx)` trần | đổi sang `scoped` thuộc CR-REQ-015 khi nối `ListBacklog` |
| L15 | Contract test cũ tạo dòng `request_backlog` bổ sung `ReturnedCategory` | CHECK ghép mới buộc |
| L16 | `ReturnRequestToBacklog` coi `ActorKind` rỗng là `user` | RPC luôn là người dùng |

## A3. Việc còn lại (cần proto hợp nhất)

- Handler gRPC: `GetRequestFlow` (003-05), `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks` (006-05). Use case, wiring (`requestLifecycle`) và test use case đã xong; chỉ thiếu handler, `request_mapper` (`returned_category`), `buf lint/breaking`. `SpawnChildRequest` đặt `Provider=manual` (chưa có đường `mcp`, CR-REQ-017).
- Hợp nhất với life-b: `request_scan.go`/`request_repository.go` hai dialect (cột `returned_category` thêm cuối `requestColumns`/`requestArgs`), `contracttest/expected_columns.go`, `cmd/server/store_wiring.go` (thêm `txScope`, `approvals`, `returns`, `outboxWriter`), `main.go` (một dòng `wireRequestLifecycle`).

## A4. Chưa kiểm chứng

- Hotfix và security: cách đọc `pre_deploy` chiếm `awaiting_plan_approval` (Q1 của SOL-003) là suy luận.
- `execution_finished` chưa có điều kiện theo loại (CR-REQ-013).
- Chưa có handler nên chưa gọi qua gRPC thật; chưa chạy cùng NATS thật cho các sự kiện mới.
- Giới hạn 50 con và 5 cấp là mềm khi đồng thời; chưa có giới hạn số lần mở lại (Q4).
- Huỷ Request chưa huỷ cây Plan/Task trong task-service (Q5).

## A5. Câu hỏi mở

- Q1, Q2, Q3 của SOL-003 và Q1 đến Q5 của SOL-006 giữ nguyên.
- Khi `CreateWithinTx` có, `IdempotentChildCreator` nên bị xoá hay giữ làm đường riêng cho con?

---

## Phần B: tiếp nhận (CR-REQ-004) và phân loại (CR-REQ-005)

Ngày: 2026-10-08. Nhánh `rf/life-b`. Sau hợp nhất mọi task 004 và 005 đã DONE (004-03, 004-06, 005-07 xong khi nối proto của rf/proto). Phần A (máy trạng thái, trả backlog, reopen, cancel, child) do agent lifecycle-a ghi ở mục riêng.

### B1. Hợp nhất với life-a và rf/proto (2026-10-08, đã làm)

- `TransitionRequest` thật của life-a đã thay các giữ chỗ: xoá `usecase/transition_contract_pending.go` và `cmd/server/wire_transitioner_pending.go`, giữ `domain/request_trigger.go` của life-a (16 trigger); `TransitionInput` thật có thêm `Category`, các trường còn lại trùng với bản tôi giả định. Cổng `RequestTransitioner` và `ApprovalCanceller`, `ExecutionGuard` dùng bản trong `ports_lifecycle.go` (tôi bỏ bản trùng). `wire_intake.go`/`wire_classification.go` nhận `lifecycle.Transition`.
- Cột: thứ tự `... solution_engine, returned_category, source_hints, classification_attempts`; Postgres `$22 solution_engine, $23 returned_category, $24::jsonb source_hints, $25 classification_attempts, $26..$28 created_at/updated_at/version`. `stores.outboxWriter` của life-a thay `stores.events` của tôi.
- Hai điểm nối giữa hai feature (việc `ClassificationAttemptsResetter` và `CreateWithinTx` mà Phần A để mở, xem câu hỏi mở A5):
  - Reopen: `ClassificationAttemptsReset` (usecase) đặt `classification_attempts = 0` trong giao dịch reopen; `ReopenRequest` trả lại hàng đã đọc lại để phản hồi thấy số 0 (sửa nhỏ ở `reopen_request.go`).
  - Spawn child: `IntakeChildCreator` (`child_request_via_intake.go`) thay `IdempotentChildCreator` mặc định, đi qua `CreateRequest.CreateWithinTx`: con vào `classifying`, `type_hint` lưu trong `source_hints`, sự kiện `created` có `parent_request_id`/`link_reason`, khoá idempotency vẫn `user:<reporter>` + `<parent>:<client_request_id>`. `IdempotentChildCreator` giữ lại (không còn được lắp trong `main`).
- Handler gRPC đã nối vào `Server` bằng `WithIntake`, `WithClassification`, `WithLifecycle` (`adapter/grpc/server_{intake,classification,lifecycle}.go`): `CreateRequest`, `LookupRequestBySource`, `ClassifyRequest` (trả `run_id`), `ConfirmRequestType`, `ChangeRequestType`, `ListRequestTypeHistory`, `GetRequestFlow`, `ReturnToBacklog`, `ReopenRequest`, `CancelRequest`, `SpawnChildRequest`, `ListRequestLinks`; mapper thêm `returned_category`, `source_hints` (kể cả `type_hint` trong phản hồi), `classification_attempts`, bộ lọc nguồn; `actor_kind` `agent` hiển thị `ai`. Actor luôn từ metadata, thiếu thì `REQUEST_REPORTER_REQUIRED`.
- `ListBacklog` không thuộc phạm vi tôi, vẫn theo trạng thái của life-a.
- Migration của tôi `0025`..`0027`; của life-a `0020`, `0021`; không trùng.

### B2. Quyết định lệch so với task

| # | Quyết định | Lý do |
|---|---|---|
| B-D1 | Số migration `0025`..`0027` thay cho `0003`/`0004` | dải số đợt 2 |
| B-D2 | `created` ghi trước khi gọi transition `start_classification` | tiêu chí của 004-04 đòi thứ tự `created`, `status_changed` theo `seq`; thứ tự trong bước 3(f) của task sẽ cho kết quả ngược |
| B-D3 | Phân loại bất đồng bộ theo run có lease (D3 của kế hoạch): `ClassificationRunner.Enqueue` trả `run_id` ngay; bảng `classification_runs` (một run sống mỗi request nhờ cột `active`, khử trùng theo `source_event_id`), heartbeat gia hạn lease, quét phục hồi `RecoverLoop` (xuyên tenant qua GUC `app.relay`), tối đa 3 lần nhận lại rồi đánh `failed`. Consumer chỉ tạo run rồi ack. Kết quả phân loại, `MarkProcessed`, `classification_attempts` và kết thúc run cùng một giao dịch | task 005-04 viết `ClassifyNow` đồng bộ; D3 và brief yêu cầu không giữ RPC chờ AI (timeout 25 giây của `wscompat`) và phải phục hồi khi chết giữa chừng |
| B-D4 | Webhook ở `request-service`, không ở `api-gateway`; bí mật/reporter từ `REQUEST_WEBHOOK_SOURCES` | brief; phương án thay thế mà 004-08 mục 5 nêu |
| B-D5 | Lỗi lạ của classifier (không phải DB) đi nhánh thất bại "classifier error" thay vì trả lỗi giao lại | tránh vòng lặp vô hạn không tăng `attempts`; chỉ lỗi DB làm run ở lại `running` để phục hồi |
| B-D6 | `ValidateConfirmation` dùng `typeRequiresSize` cố định (`bug`, `refactor`) thay cho `FlowFor().PhaseRule` | registry thuộc lifecycle-a chưa hợp nhất; thay khi có |
| B-D7 | Hành động AI ghi `actor_id` = UUID zero vào `request_type_history` | cột `actor_id UUID NOT NULL`, AI không có danh tính |
| B-D8 | "Ghi revision" trong `CreateRequest`: chỉ có điểm móc `RequestCreationRecorder` (no-op) | bảng `request_revisions` thuộc CR-REQ-027 (đợt R4); 027-05 yêu cầu `CreateWithinTx` gọi `AppendWithinTx` |
| B-D9 | `Start` của `ClassificationRunRepository` trả chính run khi thành công | tiện cho người gọi; `existing` chỉ có nghĩa khi `started=false` |
| B-D10 | Cột `active` (1 hoặc NULL) làm khoá duy nhất một-run-sống thay cho chỉ mục từng phần | MySQL không có chỉ mục từng phần; một thiết kế chung cho hai dialect, `ExpectedColumns` khớp nhau |
| B-D11 | `Prune` `processed_events` xuyên tenant bằng policy `relay_prune_scan`/`relay_prune` (Postgres) | RLS tenant chỉ cho xoá trong một tenant |

### B3. Đã kiểm chứng (2026-10-08, máy dev, Docker thật)

- `go build ./...`, `go vet ./...`, `go vet -tags integration ./...`, `gofmt -l .` sạch ở `backend-go/services/request-service`.
- `go test -race -count=3 ./...` PASS toàn bộ.
- `go test -tags integration -race ./internal/adapter/postgres -run "Intake|Classification|Migration|Schema"` PASS (Postgres 16, role `NOBYPASSRLS`); toàn bộ `./internal/adapter/postgres` và `./cmd/server` với tag `integration` PASS.
- `go test -tags integration -race ./internal/adapter/mysql -run "Intake|Classification|Migration|Schema|RequestRepositoryContract"` PASS (MySQL 8.0); migration up/down/up cả hai dialect.
- `go test -tags integration ./internal/adapter/eventbus` PASS với `nats:2.10-alpine`: giao lặp cùng `id` tới trigger với cùng event id, sự kiện `to` khác bị bỏ qua.
- Không đổi module dùng chung (`common/*`, `proto/*`, service khác): chỉ `backend-go/services/request-service` và tài liệu.

- Sau hợp nhất (2026-10-08): `gofmt -l`, `go build`, `go vet` (cả `-tags integration`) sạch ở request-service; `go test -race ./...` PASS; `-tags integration -race` PASS trọn bộ `cmd/server`, `internal/adapter/postgres` (Postgres 16), `internal/adapter/mysql` (MySQL 8.0), `internal/adapter/eventbus` (NATS 2.10). `RunRequestRPCContract` chạy 13 kịch bản gRPC với máy trạng thái thật và DB thật cả hai dialect. `proto`, `api-gateway`, `mcp-service`: build, vet, test PASS.

### B4. Chưa kiểm chứng

- Dev server agent thật: `ai.complete` qua Relay, `RelayByDevServer`, định dạng JSON trả về, chất lượng phân loại.
- Jira/Linear thật qua `issue-tracking-service` (`workspace_id = site` chưa kiểm).
- Webhook qua `api-gateway`/mạng thật; ký thật từ hệ thống bên ngoài.
- `EXPLAIN` của truy vấn quét `classification_runs`; hành vi với MySQL dưới 8.0.16 (CHECK bị bỏ qua) và TiDB (`SKIP LOCKED`, cột `active`).
- Hai instance cùng quét phục hồi (Postgres dùng `FOR UPDATE SKIP LOCKED`, đã chạy một instance trong test).
- Hành vi của các RPC mới qua gateway/WS (chỉ kiểm qua gRPC in-process).

### B5. Câu hỏi mở

- Bí mật webhook về lâu dài lấy từ `credential-broker-service` hay `tenant_settings` (CR-REQ-025)? Hiện là env/tệp.
- `request_type_history` vẫn không có khoá phá hoà `at`; AI và người sửa ghi ở hai giao dịch khác nhau nên hiếm trùng, nếu cần thêm cột `seq`.
- Sau `ChangeRequestType`, có tự chạy lại AI không? Hiện không (người xác nhận lại bằng `ConfirmRequestType`).
- `LookupRequestBySource` chỉ tìm qua bảng idempotency: Request tạo trước khi có khoá (không có) thì không thấy; hiện không có dữ liệu cũ nên chấp nhận.
