# IMPLEMENTATION-NOTES: request-quality-rollout (CR-REQ-024, 025)

Hai phần: đợt J1 (issue-status-sync, dưới) và đợt rf/roll (request-service, ở cuối file).

Ngày: 2026-10-07. Phạm vi đợt J1: `issue-status-sync` và `backend-go/deploy/alerts`. Task xong: 024-03, 024-04 (024-01 xong từ trước). Một phần: 024-06, 07, 08. Chưa làm: 024-02, 024-05 và phần `request-service` của 07/08.

## Quyết định khi triển khai (lệch hoặc bổ sung so với task)

- `NewSyncIssueStatus` giữ chữ ký 5 tham số, thêm `opts ...Option` (`WithStatusNames`, `WithRequestSyncState`, `WithRequestLookup`, `WithObserver`, `WithIssueComments`): cộng thêm, không đổi người gọi.
- `TargetState` có slice nên không còn so sánh bằng `==`; hai test cũ đổi sang `reflect.DeepEqual`, một điều kiện trong `HandleWorktreeLifecycle` kiểm `TrackerState` và `GitHubLabelPatch` rỗng.
- Version 0 trong payload (publisher không gửi) bỏ qua `Advance`, chỉ dedupe theo `EventID`.
- Lỗi `Advance` và lỗi tra cứu Nak tối đa 3 lần (bộ đếm trong bộ nhớ, không chia sẻ giữa replica); sau đó `MarkSeen`, `failed`, không chạm Jira. Lỗi Jira sau `doWithRetry` giữ hợp đồng cũ (ghi nhận, `MarkSeen`).
- `skipped_transition_unavailable` nhận biết qua chuỗi "transition unavailable" (sentinel Go không qua được gRPC).
- Nhãn `event` của `orca_issuesync_request_events_total` ngoài `status_changed`/`completed` còn có `worktree`, `pr` (lỗi tra cứu sở hữu) và `comment` (bình luận lỗi); bảng CR chưa liệt kê. Cảnh báo `IssueSyncRequestFailures` tính cả chúng.
- Span link nằm ở adapter `eventbus` (nơi có otel), không ở `usecase`.
- Luật cảnh báo kiểm bằng test Go (YAML hợp lệ, 6 alert, series thuộc bảng 2.9) vì không có `promtool`.
- `deploy/dev/docker-compose.yml` (SOL-024 2.11) chưa sửa vì ngoài phạm vi được giao.
- Thêm `ISSUE_SYNC_ORCA_BASE_URL` (liên kết trong bình luận Jira).
- Postgres test tích hợp cho service là file mới; `go.mod` thêm trực tiếp prometheus, otel, otel/sdk, otel/trace, yaml.v3 (sửa tay vì `go mod tidy` không chạy được ngoại tuyến; `go build`/`vet`/`test` trong workspace đều xanh).

## Đã kiểm chứng thật

- `go build ./...`, `go vet ./...` (kể cả `-tags integration`), `go test ./... -count=1` trong `services/issue-status-sync`: PASS, 0 FAIL, `gofmt -l` sạch.
- Tích hợp Postgres (`postgres:16-alpine`) và MySQL (`mysql:8`) thật: `Advance` tuần tự và 8 goroutine đồng thời (đúng 1 winner), migration up/down/up/down -all/up.

## Chưa kiểm chứng

- Jira thật, NATS thật (chỉ kiểm hợp đồng handler và tên durable; tên stream `REQUEST` chưa khớp request-service), `promtool`, MySQL 8.0 khác bản `mysql:8`/TiDB.
- Chưa chạy `gitnexus_impact`/`detect_changes` (chỉ mục gitnexus của worktree không có); người gọi đã kiểm bằng đọc code.

## Câu hỏi mở / việc nối tiếp

1. Khi `LookupRequestBySource` có trong `request.proto` (024-05): thêm `adapter/grpcclient/request_client.go` và truyền `usecase.WithRequestLookup` trong `main.go` (hiện `REQUEST_SERVICE_ADDR` chỉ ghi cảnh báo).
2. request-service: phát `source_*`, `version`, `reporter_id`, `number`, `traceparent` trong `status_changed`/`completed`; 12 series `orca_request_*`; sau đó đánh DONE 024-07, 024-08.
3. Dọn dữ liệu `request_sync_state` và `processed_events` (chưa có retention).

---

# Đợt rf/roll (2026-10-08): request-service, cờ, e2e

Task xong: 024-02, 05, 06, 07, 08 (SOL-024 8/8) và 025-01, 02, 05, 06. Một phần: 025-03, 04, 07, 08. Số migration dùng: `0090` (`tenant_settings`), `0091` (chỉ mục tra cứu theo nguồn), `0092` (policy `relay_scan` cho Request + chỉ mục `stuck`).

## Quyết định lệch task

- **Audit và metric suy từ sự kiện outbox đã commit**, không gọi trong từng use case: `stores.tx/txScope/outboxWriter` được bọc trong `wire_rollout.go` bằng `usecase.CommitHooks` (hàng đợi `AfterCommit`) và `TappedOutbox` (`AuditTap`, `metrics.Set`). Lý do: các use case của rf-sol/rf-exec/rf-sec đang đổi, và audit/metric không được mô tả giao dịch bị rollback (test hợp đồng hai dialect). Ngoại lệ ở RPC: `AuditRPC` ghi `denied` khi Approve/Reject/Cancel bị từ chối và `solution.choose`.
- Thêm `stage` và `wait_seconds` (omitempty) vào `domain.ApprovalDecidedPayload` (file của rf-appr) để audit có `stage` và histogram `approval_wait_seconds`; golden cũ vẫn khớp.
- Payload `status_changed`/`completed` thêm `source_provider/site/ref`, `reporter_id`, `traceparent` (và `version`, `actor_*` cho `completed`); `TransitionRequest.once` mở span `request.Transition`. Test cũ `TestTransition_HappyPath_WritesStatusChangedEvent` sửa kỳ vọng.
- `tenant_settings.tenant_id` là UUID/CHAR(36) (theo schema hiện có), không TEXT. `GetRequestFlowSettings` và `SetRequestFlowSettings` trả giá trị HIỆU LỰC (công tắc tổng AND tenant); `Get` trả lỗi khi không đọc được dòng.
- Interceptor cờ chỉ áp cho gói `/orca.request.v1.` (health/reflection đi qua; lỗi này bắt được khi chạy test cmd/server) và có lớp thứ năm `classAdmin` (cấu hình, tuân thủ, chính cờ). Bảng ở `adapter/grpc/flow_gate.go`, KHÔNG sửa `domain/rpc_catalog.go`.
- `LookupRequestBySource`: guard `internalcaller.Guard(SERVICE_INTERNAL_TOKEN)`; phía `issue-status-sync` biến là `REQUEST_SERVICE_INTERNAL_TOKEN` (cùng giá trị). Cờ tắt thì `found=false`.
- e2e T1 chạy **binary thật** và chỉ stub biên gRPC (relay `ai.complete` của infra-fleet, project `ListRepos`, auth `AppendAuditEntry`/`ListUsers`) thay vì fake cổng trong tiến trình; stub agent chính là biên relay này (`e2e/stubs`, `e2e/cmd/agent-stub`) và dùng chung cho T2. Stage phụ thuộc RPC của task khác khai `Needs` và tự SKIP (có tên task chặn) khi RPC còn `Unimplemented`, tự chạy khi có.
- `go.mod` request-service thêm trực tiếp prometheus, otel, otel/sdk, otel/trace, protobuf (sửa tay).
- Mã `DevServerExecutor` (stub trả "OK") chưa ai gọi và thuộc 033-05 (rf-exec): không đụng; stub e2e không trả lời `agent.execPrompt` mặc định (không có hợp đồng prompt), chỉ có chỗ gắn `AddHandler`.

## Cần hợp nhất

- `cmd/server/main.go`: 1 lời gọi `wireRollout` ngay sau `openStores`, `rollout.AttachLookup(intake)`, `rollout.StartSampler`, `rollout.MetricsHandler`, `rollout.WithRPCs`, và `grpc.NewServer(append(..., rollout.ServerOptions()...))`. rf-sec cần xếp `Guard` của `LookupRequestBySource` và `AuditRPC`/`FlowGate` vào chuỗi chung của họ.
- `store_wiring.go` (3 trường + `observer`), `wire_classification.go` (gắn observer), `config.go` (`StuckThresholds`, `MetricsSampleInterval`), `deploy/dev/docker-compose.yml`.
- Sau khi rf-sol/rf-exec hợp nhất: chạy lại `E2E_DIALECT=... go test -tags e2e ./e2e/...`; các stage SKIP sẽ báo "RPC is implemented now; extend this stage" và cần viết bước thật (TASK-REQ-025-03/04). Nối `ObserveTaskOutcome` (ReportTaskOutcome) và `ObserveAIGeneration` (Solution, Plan).
- `cmd/server/server` là binary bị theo dõi trong git; đừng `go build ./` trong thư mục đó.

## Đã kiểm chứng thật (2026-10-08)

- request-service: `go build`, `go vet` (cả `-tags integration` và `-tags e2e`), `gofmt -l` sạch; `go test ./... -count=1`: 512 PASS, 0 FAIL. issue-status-sync: 186 PASS.
- Tích hợp Postgres (`postgres:16-alpine`, role NOSUPERUSER NOBYPASSRLS): 262 PASS, gồm migration up/down/up của `0090..0092`, RLS `tenant_settings`, relay chỉ đọc trên `requests`, hợp đồng rollout, `cmd/server` khởi động. MySQL (`mysql:8.0`): toàn bộ package adapter PASS.
- e2e `E2E_DIALECT=postgres` và `mysql`: mỗi dialect 74 PASS, 49 SKIP (stage chờ RPC), 0 FAIL.
- `check-request-service-wiring.sh` và `_test.sh` (11 ca) PASS; `actionlint` sạch cho hai workflow; `docker compose config` hợp lệ với override.
- Container dọn xong: chỉ container do chính testcontainers/nhãn `rf-owner=rf-roll-e2e` tạo; không xoá hàng loạt. `/dev/shm/gocache` của máy đầy nên dùng `GOCACHE` riêng.

## Chưa kiểm chứng

- GitHub Actions thật; T2 (`run-request-e2e.sh`, `tests/request`) chưa chạy trên stack dev (cần shared Vault); E18 (Jira) chưa viết; Jira thật; `promtool`; `buf breaking` (không sửa proto); `gitnexus_impact`/`detect_changes` (chỉ mục không có trong worktree; người gọi đã kiểm bằng đọc code: `statusChangedPayload`/`completedPayload` chỉ do `TransitionRequest` gọi).
- `solution.choose` end-to-end (RPC chưa có); diễn tập rollback ở dev.

## Câu hỏi mở

1. `LookupRequestBySource` khi cờ tắt trả `found=false` kể cả khi Request tồn tại: chấp nhận cho worktree/PR sync cũ hoạt động lại?
2. Ai gọi `ObserveTaskOutcome`/`ObserveAIGeneration` cho solution, plan, diagnosis (rf-exec, rf-sol)?
3. `REQUEST_APPROVAL_ENABLED` mặc định `false` trong compose dev nhưng `true` trong code: thống nhất?
4. Retention của `request_sync_state`, `processed_events` vẫn chưa làm.

## Merge rf/roll vào feat/request-flow-backend (2026-10-08)

- Thứ tự interceptor (đã hợp nhất với rf/sec, 2026-10-08), sau `grpcmw.ChainUnary` (recovery, tenant, logging): Guard gateway/dịch vụ (nhận diện) → actor type → `FlowGate` → authz (`RequestAccessInterceptor`) → `AuditRPC` → rate limit/trần đồng thời. Gate đứng trước authz để lời gọi bị chặn không bị coi là quyết định authz; `AuditRPC` chỉ thấy lời gọi đã được phép (lời gọi bị từ chối đã có audit `request.access.denied`). Nối bằng `rolloutWiring.SecurityHooks()` (`AfterGuard=[FlowGate]`, `AfterAuthz=[AuditRPC]`) truyền vào `wireSecurity`; `rolloutWiring.ServerOptions()` chỉ còn `StreamFlowGate`. Guard riêng của `LookupRequestBySource` đã bỏ: RPC thuộc nhóm `internal` của catalog sec. Test: `TestChain_HookPointsRunInTheDocumentedOrder` và `TestChain_RealFlowGateRunsBeforeAuthzAndAuditRPCAfterIt` (FlowGate/AuditRPC thật) trong `adapter/grpc/security_chain_test.go`.
- Hệ quả hợp nhất: `usecase.AuditEvent/AuditRecorder/NoopAuditRecorder` của rf/roll đổi tên `RPCAuditEvent/RPCAuditRecorder/NoopRPCAuditRecorder` vì trùng tên với bộ ghi audit của rf/sec. Hai đường audit cùng tồn tại (roll: quyết định qua outbox tap và AuditRPC, ghi thẳng auth-service; sec: audit outbox bền cho erase/export/denied); chưa gộp. Quyền đọc `requests` dưới cờ `app.relay` (migration 0092 của roll) làm yếu bất biến "audit relay không đọc requests" của sec: đã đổi test thành "đọc được, không ghi được".
- Test migration của artifact (`artifact_integration_test.go`, hai dialect) định vị 0060–0062 theo vị trí cuối danh sách; thêm 0090–0092 nên offset đổi (`len-6`…) và `downs[4:5]` cho 0061 down.
- `runArtifactAndClarificationFlow` phải bật cờ tenant bằng `SetRequestFlowSettings` (admin) vì cờ theo tenant mặc định tắt.
