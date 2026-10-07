# BE-REQ-SOL-025: Kiểm thử đầu cuối, cờ `request_flow_enabled`, rollout và tài liệu

> ✅ **Đã triển khai.** Toàn bộ code đã được implement và verify (xem task list).

**CR:** [CR-REQ-025](../../../../../../docs/crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md)
**Service:** `request-service` (mới: `e2e/`, cờ, interceptor, migration), `backend-go/ci/`, `.github/workflows/`, `deploy/dev/`, `tests/request/` (mới), `docs/guides/request/` (mới)
**TDD tham chiếu:** [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (môi trường, CI, rollout), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (alert, runbook), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (hai dialect), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (cổng, adapter)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/Makefile` (`SERVICES` dòng 6-12), `backend-go/go.work`, `backend-go/deploy/postgres-init-databases.sh` (`DATABASES` dòng 8), `deploy/dev/docker-compose.yml` (`mcp-service` dòng 408, `MCP_ENABLED: ${MCP_ENABLED:-false}` dòng 576, `migrate-mcp` dòng 741), `deploy/dev/scripts/migrate.sh` (`SERVICES` có `issuetracking`, `issuestatussync`, `mcp`), `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/backend-go-issue-status-sync.yml` (ma trận `dialect: [postgres, mysql]`, `-tags=integration`), `tests/mcp/mcp_check_framework.py`, danh sách `tests/backend/`, `tests/e2e/`, `backend-go/services/mcp-service/internal/domain/tenant_settings.go`, `backend-go/services/api-gateway/internal/config/config_mcp.go` (`MCP_ENABLED`, `MCP_TENANT_DEFAULT_ENABLED`).

Khác hoặc bổ sung so với CR-REQ-025:

1. **Correction relative to CR-REQ-025 Q2 (stub AI).** README v6 mục 8 dòng 1 chốt đường AI **không** qua `ai-provider-service`: là relay `ai.complete` và `agent.execPrompt` của dev server agent qua `infra-fleet-service`. Do đó T1 giả ở các cổng `RequestClassifier`, `SolutionGenerator`, `PlanGenerator`, `AgentReadonlyRunner` (tên theo CR-REQ-005, 007, 008, 012), và T2 cần stub của **dev server agent hoặc của relay**, không phải stub provider. Chưa có stub như vậy trong repo (grep `ai.complete` chỉ ra `git-gateway-service` và `task-service`); đây là công việc mới và lớn, tách khỏi T1 (task 07 ghi rõ).
2. **Cờ không ở `tenant-service`.** Mẫu có sẵn là `mcp.tenant_settings` và `MCP_ENABLED` ở gateway; CR chọn bảng `request.tenant_settings` ở `request-service`. Giữ.
3. **Thư mục kiểm thử Python có hai khung.** `tests/mcp/mcp_check_framework.py` (PAT, WebSocket) và `tests/backend/` (`check_framework.py`, `orca_api_session.py`, `api_websocket_channels.py`). T2 dùng khung `tests/backend/` cho kịch bản qua `/ws` và khung `tests/mcp/` cho E17. CR chỉ nhắc `tests/mcp`.
4. **Chưa có workflow Request.** `.github/workflows/` không có `backend-go-request-service.yml` (CR-REQ-001 sẽ tạo); `e2e.yml` hiện là Playwright cho frontend, không dùng được cho T2.
5. **Số liệu "sở hữu" của tên DB.** Tên DB ở hai nơi khác nhau: `postgres-init-databases.sh` dùng tên đầy đủ (`mcp`), `migrate.sh` dùng tên ngắn (`issuestatussync`). Script kiểm đăng ký phải kiểm cả hai theo quy ước từng file.

## 2. Giải pháp

### 2.1 Ba tầng kiểm thử

| Tầng | Vị trí | Phụ thuộc | Chặn PR |
|---|---|---|---|
| T1 | `request-service/e2e/` (mới), tag `e2e`, testcontainers | cổng AI giả xác định; `task-service` giả trong bộ nhớ; outbox thật; DB thật | Có, ma trận `postgres`, `mysql` |
| T2 | `tests/request/` (mới), `backend-go/ci/request-e2e/run-request-e2e.sh` (mới), `.github/workflows/request-e2e.yml` (mới) | `deploy/dev/docker-compose.yml` dựng service thật; Jira giả; dev server agent giả | Không (hằng đêm, `workflow_dispatch`) |
| T3 | cùng T2, `ORCA_REQUEST_E2E_REAL_AI=true` | AI thật, dogfood | Không, chỉ báo cáo |

Lý do: AI thật không ổn định; theo mẫu CR-MCP-015 (tách e2e xác định khỏi e2e LLM thật).

### 2.2 Cờ `request_flow_enabled`

```sql
-- migrations/postgres/00NN_tenant_settings.up.sql (số chốt khi triển khai), schema request
CREATE TABLE request.tenant_settings (
  tenant_id            TEXT PRIMARY KEY,
  request_flow_enabled BOOLEAN NOT NULL DEFAULT false,
  updated_by           TEXT NOT NULL DEFAULT '',
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Postgres: ENABLE + FORCE ROW LEVEL SECURITY, policy tenant_isolation theo app.tenant_id (như các bảng request khác)
-- MySQL: tenant_id VARCHAR(255) PRIMARY KEY, request_flow_enabled TINYINT(1) NOT NULL DEFAULT 0,
--        updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6); lọc tenant ở mọi truy vấn
```

`effective = REQUEST_FLOW_ENABLED && tenant_settings.request_flow_enabled`; thiếu dòng hoặc lỗi đọc là `false` (fail closed). `GetRequestFlowSettings`, `SetRequestFlowSettings` trên `RequestService`: `Set` chỉ cho `role=admin` (kiểm bằng `tenant.Role` từ ngữ cảnh), ghi audit `request.flow.set` (BE-REQ-SOL-024) và outbox nội bộ không bắt buộc.

Interceptor `internal/adapter/grpc/flow_gate.go` (mới), một bảng cố định theo CR mục 2.2:

```go
type flowClass int
const (
    classRead flowClass = iota  // GetRequest, ListRequests, ListSolutions, ListApprovals, ListPendingForUser, ListBacklog, GetRequestFlowSettings, ...
    classSafeExit                // CancelRequest, ReturnToBacklog, Reject, Cancel (Approval)
    classAdvance                 // CreateRequest, ClassifyRequest, ConfirmRequestType, ChangeRequestType, GenerateSolution, ChooseSolutionOption, GeneratePlan, StartPhase, Approve, SpawnChildRequest, ReopenRequest
    classInternal                // ReportTaskOutcome, LookupRequestBySource, consumer outbox, hết hạn Approval
)
var flowMethodClass = map[string]flowClass{ "/orca.request.v1.RequestService/CreateRequest": classAdvance, /* ... */ }
```

Phương thức không có trong bảng: **từ chối** (`REQUEST_FLOW_DISABLED`) khi cờ tắt, và test khẳng định mọi RPC trong proto đều có dòng (không để RPC mới lọt qua cổng). Lỗi: `REQUEST_FLOW_DISABLED` (`FailedPrecondition`). `task-service` không đọc cờ.

### 2.3 Bộ kịch bản e2e (T1)

E01 đến E20 theo CR mục 2.3, mỗi kịch bản khẳng định: chuỗi trạng thái Request, `subject_type` các Approval, `Solution.kind`, Plan/Phase/Task tạo ra, sự kiện outbox, dòng audit. Kịch bản đọc từ bảng dữ liệu (Go struct) sinh từ `FlowFor` (CR-REQ-003); `TestEveryRequestTypeHasScenario` đỏ khi có loại thứ 12 mà không có kịch bản. T1 dùng `e2e/fakes/`:

| Fake | Thay |
|---|---|
| `fakeClassifier`, `fakeSolutionGenerator`, `fakePlanGenerator`, `fakeAgentReadonlyRunner` | các cổng AI của CR-REQ-005, 007, 008, 012 (trả JSON hợp lệ theo kịch bản, có chế độ lỗi) |
| `fakeTaskService` (bộ nhớ) | `task-service`: `CreatePlanTree`, `CreateTask`, `ListTasks`, `ExecuteTask`, `ListExecutionStates`; cho phép kịch bản tiêm "task xong/lỗi" qua `ReportTaskOutcome` |
| `fakeNotifier` | không cần: kiểm outbox, không kiểm notification-service |

### 2.4 Hai DB

T1 chạy ma trận `dialect: [postgres, mysql]` trong `.github/workflows/backend-go-request-service.yml` (CR-REQ-001 tạo, sao `backend-go-task-service.yml`), thêm bước `go test -tags=e2e ./e2e/...`. Cùng bộ kịch bản, khác DSN. Kiểm riêng hai dialect: khoá lặp `request_idempotency`, `SELECT ... FOR UPDATE` của Approval, claim outbox `SKIP LOCKED` (MySQL >= 8.0.1, CR-DB-002), `JSON` so với `JSONB`, thứ tự phân trang.

### 2.5 Kiểm tra đăng ký đầy đủ

`backend-go/ci/check-request-service-wiring.sh` (mới), thất bại nếu thiếu `request` ở: `backend-go/go.work` (`./services/request-service`), `SERVICES` trong `backend-go/Makefile` (`request-service`), `DATABASES` trong `backend-go/deploy/postgres-init-databases.sh` (`request`), dịch vụ `request-service` và `migrate-request` trong `deploy/dev/docker-compose.yml`, `REQUEST_SERVICE_ADDR` ở `api-gateway` và `issue-status-sync` trong compose, `SERVICES` trong `deploy/dev/scripts/migrate.sh` (`request`), và workflow `backend-go-request-service.yml`. Mẫu: `check-opa-bundle-in-images.sh` (`set -euo pipefail`, thông báo `FAIL:`). Dùng `grep -q` có neo từ, không `sed` (đa nền tảng: chạy bằng bash trên runner Linux; Windows dev dùng WSL, ghi trong README script).

### 2.6 Rollout, rollback, tài liệu

Giữ nguyên CR mục 2.6 (5 giai đoạn), 2.7 (7 bước rollback, không chạy `down` migration của `task-service` khi còn dòng `plan|phase`; `down` của CR-REQ-011 phải từ chối), 2.8 (7 tài liệu trong `docs/guides/request/` và cập nhật README các service). Bổ sung điều kiện giai đoạn: "bật theo loại" chưa có cơ chế (CR Q1); giai đoạn 2, 3 thực hiện bằng hướng dẫn tenant chọn loại, không bằng cờ.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Cờ thi hành ở `request-service` bằng interceptor có bảng phân loại RPC | Một điểm; RPC mới không lọt khi quên phân loại |
| D2 | Tắt cờ vẫn cho đọc, thoát an toàn và xử lý callback | Không mất kết quả, không giam dữ liệu |
| D3 | Fail closed | Thiếu dòng, lỗi đọc là tắt |
| D4 | T1 chặn PR; T2, T3 không | AI thật và stack đầy đủ không ổn định |
| D5 | Kịch bản sinh từ `FlowFor` | Thêm loại mà quên e2e thì CI đỏ |
| D6 | Rollback không dùng `down` migration | Tránh mất dữ liệu `plan`, `phase` |
| D7 | Cờ theo tenant, không theo loại | Đơn giản; bật dần theo loại là quyết định riêng |
| D8 | T2 cần stub dev server agent, tách thành task riêng | Đường AI là relay, không phải provider (mục 1 điểm 1) |

## 4. Phụ thuộc và thứ tự

Phần cờ (task 01, 02) cần CR-REQ-001, 002 và nên vào sớm vì CR-REQ-016, 017 đã trỏ tới `request.flowStatus`, `request_flowStatus`. Phần e2e cần lõi CR-REQ-003 đến 013 chạy được trên fake; T2 cần CR-REQ-016, 024 và stub agent. Rollout cần BE-REQ-SOL-024 (alert). Task: [`../tasks/README.md`](../tasks/README.md).

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `e2e/request_flow_test.go` | E01 đến E16, E19 (bảng kịch bản) |
| `e2e/type_matrix_test.go` | `TestEveryRequestTypeHasScenario` |
| `e2e/feature_flag_test.go` | E20: bật, tắt, bật; cô lập tenant; bảng 2.2 |
| `internal/adapter/grpc/flow_gate_test.go` | mỗi RPC trong proto có phân loại; hành vi từng lớp khi cờ tắt |
| `tenant_settings` repo test | hai dialect: đọc, ghi, mặc định `false` |
| `tests/request/check_request_flow_types.py` | T2: 11 loại qua `/ws` |
| `tests/mcp/check_mcp_request_flow.py` (TASK-REQ-017-05) | E17 |
| `tests/request/check_request_jira_sync.py` | E18 với Jira giả |
| `ci/check-request-service-wiring.sh` | cố ý xoá `request` khỏi một nơi thì script đỏ (test của script: chạy trên bản sao thư mục) |

Lệnh: `cd backend-go/services/request-service && go test ./... && go test -tags=e2e ./e2e/...` (cần Docker); T2: `backend-go/ci/request-e2e/run-request-e2e.sh`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- T1 dùng `task-service` giả nên có thể che lỗi ở đường nối thật; T2 bù nhưng chưa chặn PR.
- Stack `deploy/dev` có chạy được trong runner CI hay không chưa kiểm chứng (nhiều service, Vault, NATS).
- Stub dev server agent chưa tồn tại; nếu không làm, T2 chỉ chạy các loại không cần AI hoặc cần dev server thật.
- Jira thật chưa từng chạy với luồng này; E18 ở T2 dùng Jira giả.
- `performance`, `ops_request`, `spike`, `question` cần năng lực agent chưa có; T1 chỉ kiểm khung luồng.
- Thứ tự giao hàng: nếu cờ vào muộn hơn CR-016, `request.flowStatus` phải tạm trả `{enabled:false}` cố định (ghi vào CONTRACT).
- SSH, remote: e2e không giả định thực thi cục bộ; kiểm thử SSH thật nằm ngoài CR.

## 7. Câu hỏi mở

1. Cờ theo loại (CR Q1): giữ cờ theo tenant; hướng dẫn rollout theo loại bằng quy trình.
2. Stub dev server agent cho T2 (khác với câu hỏi "stub ai-provider-service" của CR): ai làm, ở đâu (`tests/request/stubs/` hay trong `infra-fleet-service`)?
3. README v6 cần thêm bảng `tenant_settings` và hai RPC cờ vào mục 3.5, 3.6 (CR Q3; chỉ người điều phối sửa).
4. Mặc định cờ cho tenant mới ở GA (CR Q4).
5. Cờ vào `tenant-service` hay giữ ở `request-service` (CR Q5): giữ.

## 8. Tham chiếu

- `backend-go/Makefile`, `backend-go/go.work`, `backend-go/deploy/postgres-init-databases.sh`, `backend-go/docker-compose.yml`, `backend-go/ci/check-opa-bundle-in-images.sh`, `backend-go/ci/mcp-conformance/run-go-conformance.sh`
- `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `deploy/dev/scripts/ensure-mcp-env.sh`
- `.github/workflows/backend-go-task-service.yml`, `.github/workflows/backend-go-issue-status-sync.yml`, `.github/workflows/backend-go-mcp-conformance.yml`
- `tests/mcp/mcp_check_framework.py`, `tests/backend/check_framework.py`, `tests/backend/orca_api_session.py`, `tests/backend/api_websocket_channels.py`, `tests/e2e/`
- `backend-go/services/mcp-service/internal/domain/tenant_settings.go`, `backend-go/services/api-gateway/internal/config/config_mcp.go`
- `docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md`, [`specs/backend-go/crs/v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-conformance-e2e-observability-rollout.md`](../../../v5/mcp-quality-rollout/solutions/BE-MCP-SOL-015-conformance-e2e-observability-rollout.md)
- `docs/crs/v6/README.md` mục 6, 7, 8; `docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md`
