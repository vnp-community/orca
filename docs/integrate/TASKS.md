# TASKS — Tích hợp luồng agent kiểu tdd-agent vào Workflow Orca

Từ [PLAN.md](./PLAN.md) và [TDD-WF-AGENT-001](./TDD-WF-AGENT-001-workflow-agent-pipeline.md). Trạng thái: `[ ]` chưa làm, `[~]` đang làm hoặc làm một phần, `[x]` xong (ghi ngày). Effort: S ≤ 0,5 ngày, M ≤ 2 ngày, L ≤ 4 ngày.

Viết tắt đường dẫn: `WS` = `backend-go/services/workflow-service`, `IF` = `backend-go/services/infra-fleet-service`, `GWY` = `backend-go/services/api-gateway`, `TDA` = repo `tdd-agentic-ai`, `E2E` = `tests/stages/e2e`.

## Tổng quan

| ID | Task | Repo | Effort | Phụ thuộc | Status |
|---|---|---|---|---|---|
| T0.1 | Kiểm Q9: `run_list` lọc theo run cha, `approval_submit` khi gate đã duyệt | TDA | S | — | `[x]` 2026-10-14 |
| T0.2 | Chốt Q1: nguồn `projectDir` | orca | S | — | `[x]` 2026-10-14 |
| T0.3 | Hợp đồng JSON `mcp.callTool` và output handler, cờ tính năng | orca | S | T0.1, T0.2 | `[x]` 2026-10-14 |
| T1.1 | Tool MCP `run_dispatch` | TDA | M | T0.1 | `[x]` 2026-10-14 |
| T1.2 | `run_start` nhận `sources[].path` | TDA | S | — | `[x]` 2026-10-14 |
| T1.3 | `origin.external` và tìm run theo execution | TDA | S | — | `[x]` 2026-10-14 |
| T1.4 | `pid` + `heartbeat_at` của tiến trình advance | TDA | M | T1.1 | `[x]` 2026-10-14 |
| T1.5 | Client MCP `orca-workflow` được duyệt | TDA | S | — | `[x]` 2026-10-14 |
| TE.1 | Overlay control plane, project `orca-e2e`, mạng `orca-e2e-wan` | orca | S | — | `[~]` |
| TE.2 | Image dev server + entrypoint | orca | M | — | `[~]` |
| TE.3 | Compose dev server, `split-up.sh` / `split-down.sh` | orca | M | TE.1, TE.2 | `[~]` |
| TE.4 | `verify_topology.py` (TOP-01..07) | orca | M | TE.3 | `[~]` |
| TE.5 | README, workflow CI chạy tay | orca | S | TE.4 | `[~]` |
| T2.1 | Cầu MCP stdio trên agent (pool tiến trình) | orca | M | T0.3 | `[x]` 2026-10-14 |
| T2.2 | RPC `mcp.callTool`: allowlist, kiểm `projectDir`, feature | orca | M | T2.1 | `[x]` 2026-10-14 |
| T2.3 | Cấu hình `mcpBridge.servers` của agent | orca | S | T2.1 | `[x]` 2026-10-14 |
| T2.4 | Timeout `mcp.callTool` ở infra-fleet | orca | S | — | `[x]` 2026-10-14 |
| T3.1 | `tddagentclient.CallTool` | orca | M | T0.3, T2.4 | `[x]` 2026-10-14 |
| T3.2 | Vòng poll dùng chung | orca | M | T3.1 | `[x]` 2026-10-14 |
| T3.3 | Handler `tdd_agent.start` | orca | S | T3.1, T1.3 | `[x]` 2026-10-14 |
| T3.4 | Handler `tdd_agent.advance`, `tdd_agent.wait_child` | orca | M | T3.2, T1.1 | `[x]` 2026-10-14 |
| T3.5 | Đăng ký handler trong `main.go`, cờ tính năng | orca | S | T3.3, T3.4 | `[x]` 2026-10-14 |
| T3.6 | Template TDD generation, Test specs + suite `TWA-01..06` | orca | M | T3.5, TE.4, T2.2 | `[~]` |
| T4.1 | Domain + migration `step_approvals` | orca | S | T0.3 | `[x]` 2026-10-14 |
| T4.2 | Repository Postgres/MySQL `step_approvals` | orca | M | T4.1 | `[~]` |
| T4.3 | Usecase list/get/resolve + phân quyền `roleMap` | orca | M | T4.2 | `[x]` 2026-10-14 |
| T4.4 | Proto, gRPC server, kênh wscompat `workflow.stepApproval.*` | orca | M | T4.3 | `[x]` 2026-10-14 |
| T4.5 | Handler `tdd_agent.approval` | orca | M | T4.3, T3.2, T1.5 | `[x]` 2026-10-14 |
| T4.6 | Handler `tdd_agent.wait_evidence` + template Change request | orca | M | T4.5 | `[~]` |
| T4.7 | Frontend: bảng gate, hộp duyệt, danh sách chờ duyệt | orca | L | T4.4 | `[~]` |
| T4.8 | Suite `TWA-07..12` + Playwright CR | orca | M | T4.6, T4.7 | `[~]` |
| T5.1 | Tool MCP `run_cancel` | TDA | M | T1.4 | `[x]` 2026-10-14 |
| T5.2 | Huỷ execution gọi `run_cancel` | orca | S | T5.1, T3.4 | `[x]` 2026-10-14 |
| T5.3 | Dispatch lại khi heartbeat cũ | orca | S | T1.4, T3.4 | `[x]` 2026-10-14 |
| T5.4 | Metrics và log chuyển trạng thái gate | orca | S | T3.4 | `[~]` |
| T5.5 | Suite khôi phục `TWA-13..17` | orca | M | T5.2, T5.3 | `[~]` |
| T6.1 | Chạy thật MiniPay với claude-cli trên topology tách | cả hai | M | T5.5 | `[ ]` |
| T6.2 | Báo cáo nghiệm thu, TDD v0.2, backlog | orca | S | T6.1 | `[ ]` |

---

## P0 — Chuẩn bị

### T0.1 — Kiểm Q9 trong tdd-agent
**Repo:** TDA · **Effort:** S · **Phụ thuộc:** — · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/mcp/tools.py`, `src/tdd_agent/orchestrator/approvals.py`, `src/tdd_agent/store/*`

- [ ] `run_list` có lọc theo `parent_run_id` không; nếu không, ghi thành việc của T1.3.
- [ ] `run_get` có trả danh sách run con không (nếu không, `wait_child` dùng `run_list`).
- [ ] `approval_submit` trả gì khi gate đã `approved` (mã lỗi, `accepted`); ghi hành vi để T4.5 coi là thành công.
- [ ] Cập nhật TDD §12 Q9 bằng kết quả, có dẫn file:dòng.

**Kiểm chứng:** đọc code + một ca pytest nhỏ cho từng điểm.

### T0.2 — Chốt nguồn `projectDir` (Q1)
**Repo:** orca · **Effort:** S · **Status:** `[x]` 2026-10-14
**Files:** `backend-go/services/project-service` (GetProject/GetProjectContext), `WS/internal/adapter/infrafleetclient/project_context_resolver.go`

- [ ] Xác nhận project có `repoPath` (được kiểm qua agent lúc `project.create`) và workflow-service đọc được nó.
- [ ] Quyết định: `projectDir` mặc định = `repoPath` của project; inputs được ghi đè nhưng phải nằm trong `repoPath`.
- [ ] Ghi vào TDD §12 Q1 và §7.3.

### T0.3 — Hợp đồng JSON và cờ tính năng
**Repo:** orca · **Effort:** S · **Phụ thuộc:** T0.1, T0.2 · **Status:** `[x]` 2026-10-14
**Files (mới):** `docs/integrate/contracts/mcp-call-tool.request.schema.json`, `mcp-call-tool.response.schema.json`, `tdd-agent-step-output.schema.json`

- [ ] Schema request/response `mcp.callTool` theo TDD §7.1 (mã lỗi: `MCP_SERVER_NOT_ALLOWED`, `MCP_TOOL_NOT_ALLOWED`, `MCP_PROJECT_DIR_OUTSIDE_WORKSPACE`, `MCP_SERVER_NOT_INSTALLED`, `MCP_TIMEOUT`).
- [ ] Schema output chung của các handler `tdd_agent.*` (TDD §7.3).
- [ ] Cờ: `WORKFLOW_TDD_AGENT_ENABLED` (workflow-service, mặc định `false`), feature agent `mcp.callTool`.

---

## P1 — tdd-agent (repo `tdd-agentic-ai`)

### T1.1 — Tool MCP `run_dispatch`
**Effort:** M · **Phụ thuộc:** T0.1 · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/mcp/tools.py`, `src/tdd_agent/mcp/server.py`, `src/tdd_agent/agents/testspec/gate.py` (tái dùng `launch_detached`), `tests/test_mcp_run_dispatch.py` (mới)

- [ ] Đưa `launch_detached` thành tiện ích chung (không còn riêng testspec), giữ hook `LAUNCH` cho test.
- [ ] `run_dispatch(run_id)`: run `running` → `{dispatched:false, reason:"RUN_ALREADY_RUNNING"}`; trạng thái kết thúc → `{dispatched:false, reason:"RUN_TERMINAL"}`; còn lại → chạy nền, trả `{dispatched:true, pid}`.
- [ ] Thêm vào `WRITE_TOOLS`, có `_audit_write`.
- [ ] Test: idempotent khi gọi hai lần liền; không dispatch run đã `completed`.

### T1.2 — `run_start` nhận `sources[].path`
**Effort:** S · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/mcp/tools.py` (`_source_doc`), test

- [ ] Chấp nhận `{id, revision, role, path}`; đọc file tương đối với thư mục project; từ chối đường dẫn thoát ra ngoài (`..`, symlink).
- [ ] Giữ nguyên dạng `text` hiện có.

### T1.3 — `origin.external` và tìm run theo execution
**Effort:** S · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/mcp/tools.py` (`run_start`, `run_list`)

- [ ] `request.origin.external = {system, executionId, stepId, userId}` được lưu nguyên vẹn.
- [ ] `run_list` lọc được theo `external_execution_id`, `external_step_id` và (nếu T0.1 thấy thiếu) `parent_run_id`.

### T1.4 — `pid` và `heartbeat_at` của tiến trình advance
**Effort:** M · **Phụ thuộc:** T1.1 · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/orchestrator/engine.py`, `src/tdd_agent/cli.py` (`run advance`)

- [ ] Ghi `pid` lúc dispatch; tiến trình advance cập nhật `heartbeat_at` mỗi 15 s trong lúc gate chạy (luồng phụ).
- [ ] `run_get` trả `pid`, `heartbeat_at`, `heartbeat_interval_s`.

### T1.5 — Client MCP `orca-workflow` được duyệt
**Effort:** S · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/project.py` (`mcp enable`), `docs/CLI_Project_Usage.md`

- [ ] `tdd-agent mcp enable --client orca-workflow --approvals --roles BA,SA,SA/Dev,BA/owner` thêm actor vào allowlist approver và bật `approval_submit` cho client đó.
- [ ] Ghi tài liệu: rationale phải chứa `orca:user=<id>`.

---

## PE — Môi trường e2e tách máy (TDD §11.1)

Thư mục: `E2E/split-topology/`.

### TE.1 — Overlay control plane
**Effort:** S · **Status:** `[~]`
**Files (mới):** `control-plane.override.yml`

- [ ] Thêm `api-gateway` vào mạng `orca-e2e-wan` (tên cố định, alias `orca-gateway`); không service nào khác vào mạng này.
- [ ] Kiểm `docker compose -p orca-e2e ... config` gộp đúng mạng (base dùng dạng list, overlay dùng dạng map).

### TE.2 — Image dev server
**Effort:** M · **Status:** `[~]`
**Files (mới):** `devserver/Dockerfile`, `devserver/entrypoint.sh`, `devserver/wheels/.gitkeep`, `.gitignore` cho `*.whl`

- [ ] `node:22-bookworm-slim` + git, python3 (≥ 3.11 cho tdd-agent), ripgrep; user `node`.
- [ ] Build arg `INSTALL_CLAUDE_CLI`; wheel tdd-agent tuỳ chọn cài vào `/opt/tdd-agent`.
- [ ] Entrypoint: seed repo từ `/fixtures/*`, `AI_MODE=stub|real|none`, gỡ khoá API, `MODE=direct-websocket`.

### TE.3 — Compose dev server và script dựng
**Effort:** M · **Phụ thuộc:** TE.1, TE.2 · **Status:** `[~]`
**Files (mới):** `docker-compose.devserver.yml`, `split-up.sh`, `split-down.sh`, `devserver_ready.py`

- [ ] Project `orca-e2e-devservers`, mạng `orca-e2e-wan` dạng `external`; `devserver-a`, `devserver-b` (profile `multi`); không `restart`.
- [ ] `split-up.sh`: từ chối khi có container `orca-go-*` của project khác; `--build`; Vault local, bind `127.0.0.1`, không sửa `deploy/dev/.env`; migrate bằng chính lệnh compose của `orca-e2e` (không gọi `migrate.sh` vì nó dùng project mặc định); cấp token phía host; chờ kết nối và duyệt.
- [ ] `split-down.sh [--volumes]` không đụng project `orca-go`.

### TE.4 — `verify_topology.py`
**Effort:** M · **Phụ thuộc:** TE.3 · **Status:** `[~]`
**Files (mới):** `verify_topology.py` (dùng `orca-flows/acceptance_recorder.py` để có báo cáo cùng định dạng)

- [ ] TOP-01..07 đúng bảng TDD §11.1; TOP-07 `BLOCKED` khi không có `--multi`.
- [ ] Báo cáo `reports/split-topology.{json,md}`.

### TE.5 — README và CI
**Effort:** S · **Phụ thuộc:** TE.4 · **Status:** `[~]`
**Files (mới):** `E2E/split-topology/README.md`, `.github/workflows/orca-split-e2e.yml` (`workflow_dispatch`)

- [ ] README: điều kiện, lệnh, giới hạn (TDD §11.1).
- [ ] CI: build, `split-up.sh --multi`, `verify_topology.py`, thu log container khi lỗi, `split-down.sh --volumes`.

---

## P2 — Cầu MCP trên Dev Server Agent

### T2.1 — Cầu MCP stdio
**Effort:** M · **Phụ thuộc:** T0.3 · **Status:** `[x]` 2026-10-14
**Files (mới):** `agent/src/relay/mcp-stdio-bridge.ts`, `agent/src/relay/mcp-stdio-bridge.test.ts`

- [ ] Một tiến trình MCP stdio cho mỗi `(server, projectDir)`, tái dùng, đóng sau 10 phút rảnh, khởi lại khi chết.
- [ ] Hàng đợi lời gọi theo tiến trình; timeout từng lời gọi (mặc định 60 s, trần 120 s).
- [ ] Tắt sạch các tiến trình khi agent dừng.

### T2.2 — RPC `mcp.callTool`
**Effort:** M · **Phụ thuộc:** T2.1 · **Status:** `[x]` 2026-10-14
**Files:** `agent/src/relay/agent-rpc-dispatch-mcp.ts` (mới), nơi đăng ký dispatch của relay, `agent/src/relay/agent-protocol-features.ts`

- [ ] Kiểm allowlist server và tool; `projectDir` qua cùng kiểm tra workspace mà `agent.execPrompt` dùng.
- [ ] Phân biệt `isError` của tool với lỗi vận chuyển (TDD §7.1); log span không lộ rationale dài.
- [ ] Quảng bá feature `mcp.callTool` trong handshake.
- [ ] Test: server/tool ngoài allowlist, `projectDir` ngoài workspace, `MCP_SERVER_NOT_INSTALLED`.

### T2.3 — Cấu hình `mcpBridge.servers`
**Effort:** S · **Phụ thuộc:** T2.1 · **Status:** `[x]` 2026-10-14
**Files:** cấu hình agent (env `ORCA_MCP_BRIDGE_CONFIG` trỏ tới file JSON), `deploy/agent/agent-runtime.env.example`, `deploy/agent/README.md`

- [ ] Mặc định có mục `tdd-agent` (`tdd-agent -C ${projectDir} mcp serve --transport stdio`); không nhận lệnh từ request.

### T2.4 — Timeout ở infra-fleet
**Effort:** S · **Status:** `[x]` 2026-10-14
**Files:** `IF/internal/adapter/devserveragent/exec_timeouts.go` (+ test)

- [ ] `"mcp.callTool": 150 * time.Second` (lớn hơn trần 120 s phía agent).
- [ ] Chạy `impact` GitNexus cho `execTimeoutForMethod` trước khi sửa.

---

## P3 — Handler workflow

### T3.1 — `tddagentclient.CallTool`
**Effort:** M · **Phụ thuộc:** T0.3, T2.4 · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/adapter/tddagentclient/client.go`, `client_test.go`

- [ ] Resolve `project:<id>` qua `serverresolver`, gọi `Relay(method="mcp.callTool")`, tái dùng `withTenantMetadata`.
- [ ] Phân loại lỗi: vận chuyển (đếm để thử lại), tool `isError` (mang `code` của tdd-agent), hợp đồng sai.

### T3.2 — Vòng poll dùng chung
**Effort:** M · **Phụ thuộc:** T3.1 · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/adapter/tddagentclient/poll.go`, `poll_test.go`

- [ ] `pollIntervalSec`, `deadlineMin`, dừng khi ctx huỷ, 5 lỗi vận chuyển liên tiếp → `TDD_AGENT_UNREACHABLE`, quá hạn → `TDD_AGENT_DEADLINE`.
- [ ] Test bằng đồng hồ giả.

### T3.3 — Handler `tdd_agent.start`
**Effort:** S · **Phụ thuộc:** T3.1, T1.3 · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/adapter/tddagentclient/action_start.go` (+ test)

- [ ] Tìm run theo `(executionId, stepId)` trước khi `run_start`; output `{runId, flow, status}`.

### T3.4 — Handler `advance` và `wait_child`
**Effort:** M · **Phụ thuộc:** T3.2, T1.1 · **Status:** `[x]` 2026-10-14
**Files (mới):** `action_advance.go`, `action_wait_child.go` (+ test)

- [ ] `advance`: `run_get` trước; chỉ `run_dispatch` khi không `running`; bảng kết quả TDD §7.3 (`expectGate`, `TDD_AGENT_UNEXPECTED_GATE`, `blocked` mang `blockers[]`).
- [ ] `wait_child`: tìm run con theo T0.1, poll tới kết thúc, output đường dẫn test specs.

### T3.5 — Đăng ký handler
**Effort:** S · **Phụ thuộc:** T3.3, T3.4 · **Status:** `[x]` 2026-10-14
**Files:** `WS/cmd/server/main.go` (dòng đăng ký `StepTypeAction`), `WS/internal/config`

- [ ] Giữ `ActionExecutor` vào biến, `Register("tdd_agent.*", ...)` khi `WORKFLOW_TDD_AGENT_ENABLED=true`.
- [ ] `impact` GitNexus cho composition root trước khi sửa; `detect_changes` trước khi commit.

### T3.6 — Template + suite `TWA-01..06`
**Effort:** M · **Phụ thuộc:** T3.5, TE.4, T2.2 · **Status:** `[~]`
**Files (mới):** `docs/integrate/templates/{tdd-generation,testspec}.json`, `E2E/orca-flows/suite_workflow_tdd_agent.py`

- [ ] Hai template TDD §7.4 (bảng "Hai template khác").
- [ ] Suite chạy trên topology PE, tdd-agent `llm=fake`.

---

## P4 — Duyệt gate

### T4.1 — Domain + migration `step_approvals`
**Effort:** S · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/domain/step_approval.go` (+ test), `WS/migrations/{postgres,mysql}/0014_step_approvals.{up,down}.sql`

- [ ] Cột theo TDD §7.5; khoá duy nhất `(execution_id, step_id)`; trạng thái `pending/approved/rejected/superseded`.

### T4.2 — Repository
**Effort:** M · **Phụ thuộc:** T4.1 · **Status:** `[~]`
**Files:** `WS/internal/adapter/{postgres,mysql}/step_approval_repository.go` (mới), test tích hợp

- [ ] Upsert theo hash (hash khác → bản cũ `superseded`), not-found theo quy ước nhánh này.

### T4.3 — Usecase + phân quyền
**Effort:** M · **Phụ thuộc:** T4.2 · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/usecase/{list,get,resolve}_step_approval.go` (+ test)

- [ ] `resolve` kiểm `decisionHash` (`STEP_APPROVAL_STALE`), quyền ghi project và `tddAgent.roleMap` (mặc định chỉ owner).

### T4.4 — Proto, gRPC, wscompat
**Effort:** M · **Phụ thuộc:** T4.3 · **Status:** `[x]` 2026-10-14
**Files:** `backend-go/proto/orca/workflow/v1/workflow.proto`, `WS/internal/adapter/grpc/*`, `GWY/internal/adapter/wscompat/channels_workflow.go` (+ test)

- [ ] Kênh `workflow.stepApproval.list|get|resolve`; sinh lại code proto.

### T4.5 — Handler `tdd_agent.approval`
**Effort:** M · **Phụ thuộc:** T4.3, T3.2, T1.5 · **Status:** `[x]` 2026-10-14
**Files (mới):** `WS/internal/adapter/tddagentclient/action_approval.go` (+ test)

- [ ] Luồng TDD §7.5: `run_get` (ghi receipt) → upsert chờ duyệt → poll → `approval_submit` với rationale `orca:user=<id> exec=<id>: ...`.
- [ ] Ca: từ chối, hash đổi, duyệt ngoài Orca (`approvedBy: "external"`), đã duyệt sẵn (theo T0.1), tdd-agent từ chối duyệt.

### T4.6 — `wait_evidence` + template Change request
**Effort:** M · **Phụ thuộc:** T4.5 · **Status:** `[~]`
**Files (mới):** `action_wait_evidence.go` (+ test), `docs/integrate/templates/change-request.json`

- [ ] Template đúng TDD §7.4 (14 bước, fan-out `testspecs` / `approve_g5`).

### T4.7 — Frontend
**Effort:** L · **Phụ thuộc:** T4.4 · **Status:** `[~]`
**Files:** `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx` và component mới (`TddAgentGateTable.tsx`, `StepApprovalDialog.tsx`, `PendingStepApprovals.tsx`), test

- [ ] Bảng gate và blockers từ output bước `tdd_agent.*`; hộp xem quyết định + duyệt/từ chối (gửi đúng hash); danh sách chờ tôi duyệt.
- [ ] Theo `docs/STYLEGUIDE.md`; phím tắt và nhãn theo nền tảng.

### T4.8 — Suite `TWA-07..12` + Playwright
**Effort:** M · **Phụ thuộc:** T4.6, T4.7 · **Status:** `[~]`
**Files:** `E2E/orca-flows/suite_workflow_tdd_agent.py`, `E2E/orca-flows/ui-live/tdd-agent-cr.web.e2e.ts` (mới)

---

## P5 — Huỷ, khôi phục, vận hành

### T5.1 — Tool MCP `run_cancel`
**Repo:** TDA · **Effort:** M · **Phụ thuộc:** T1.4 · **Status:** `[x]` 2026-10-14
**Files:** `src/tdd_agent/mcp/tools.py`, `src/tdd_agent/orchestrator/engine.py`, test

- [ ] Run → `failed` với `CANCELLED_BY_CLIENT`, SIGTERM tiến trình nền theo `pid`, huỷ cả run con.

### T5.2 — Huỷ execution gọi `run_cancel`
**Effort:** S · **Phụ thuộc:** T5.1, T3.4 · **Status:** `[x]` 2026-10-14
**Files:** `action_advance.go`, `action_wait_child.go`, `action_approval.go`

- [ ] Khi ctx bị huỷ do `CancelExecution` (không phải do restart service) thì gọi `run_cancel`; phân biệt hai trường hợp.

### T5.3 — Dispatch lại khi heartbeat cũ
**Effort:** S · **Phụ thuộc:** T1.4, T3.4 · **Status:** `[x]` 2026-10-14

- [ ] `running` mà `heartbeat_at` cũ hơn 3 × chu kỳ → `run_dispatch` lại, log cảnh báo.

### T5.4 — Metrics và log
**Effort:** S · **Phụ thuộc:** T3.4 · **Status:** `[~]`

- [ ] `workflow_tdd_agent_calls_total{tool,result}`, `workflow_tdd_agent_poll_duration_seconds{action}`, `workflow_tdd_agent_approval_wait_seconds{gate}`; log mỗi lần gate đổi trạng thái.

### T5.5 — Suite khôi phục `TWA-13..17`
**Effort:** M · **Phụ thuộc:** T5.2, T5.3 · **Status:** `[~]`

---

## P6 — Nghiệm thu thật

### T6.1 — MiniPay với claude-cli
**Effort:** M · **Phụ thuộc:** T5.5 · **Status:** `[ ]`

- [ ] Topology PE với `AI_MODE=real`, tdd-agent `llm=claude-cli`; chạy ba template trên MiniPay.
- [ ] Đo thời gian từng gate, số lần poll, so với lần chạy trực tiếp qua MCP (testspec ≈ 178 s).

### T6.2 — Báo cáo
**Effort:** S · **Phụ thuộc:** T6.1 · **Status:** `[ ]`

- [ ] `docs/integrate/ACCEPTANCE-REPORT.md`; nâng TDD lên v0.2 với các điểm đã kiểm chứng; mở backlog cho phần còn lại (`on_behalf_of`, CR-WFD-004/005/002).

---

## Đánh giá 2026-10-14

`[x]` = code xong và có test tự động đạt; `[~]` = code đã viết nhưng chưa chạy/kiểm đủ; `[ ]` = chưa làm.

**Kết quả test đã chạy**

| Phạm vi | Lệnh | Kết quả |
|---|---|---|
| tdd-agentic-ai (P1, T5.1) | `pytest` toàn bộ | 706 passed, 2 skipped (10 test mới `test_mcp_run_dispatch.py`, 4 test `test_mcp_remote_flows.py`) |
| tdd-agent thật | `run advance --heartbeat` trên `tmp/minipay-run` | ghi `.proc.json` có `pid`, `heartbeat_at`, `finished_at` |
| agent (P2) | `vitest agent-rpc-dispatch-mcp.test.ts`, `agent-protocol-features.test.ts` | 14 passed; oxlint sạch; `tsc` không lỗi ở file mới |
| agent ↔ tdd-agent thật | `mcp.callTool` qua stdio vào `tdd-agent mcp serve` (MiniPay) | `run_list` 629 ms (khởi tiến trình), `run_get` 6 ms, lỗi `RUN_NOT_FOUND` đi qua đúng |
| workflow-service (P3, P4, P5) | `go vet ./... && go test ./...` | đạt (`tddagentclient`: 20+ test handler/client, `usecase`: step approvals) |
| api-gateway | `go vet ./... && go test ./...` | đạt sau khi cập nhật golden kênh, fake client, loại trừ MCP `workflow.stepApproval.*` |
| infra-fleet | `go test ./internal/adapter/devserveragent/...` | đạt |
| frontend (T4.7) | `vitest StepApprovalPanel.test.tsx ExecutionMonitor.test.tsx` | 18 passed |

**Chưa kiểm chứng / còn thiếu**

| Task | Còn lại |
|---|---|
| TE.1–TE.3 | Stack đã lên (migration 0014 chạy, `WORKFLOW_TDD_AGENT_ENABLED` nạp, agent bắt tay qua `orca-gateway` thành công) nhưng chưa có lần `split-up.sh` trọn vẹn tới bước dev server được duyệt. Đã sửa 3 lỗi tìm ra: token Vault dùng chung không hợp lệ cho dev-mode (`SPLIT_VAULT_TOKEN`), tenant của agent (`ORCA_AGENT_DEFAULT_TENANT_ID`), token chờ kết nối hết hạn sau 60 s (build image trước khi cấp token). Seed `tdd-project` chưa kiểm. |
| TE.4, T3.6, T4.8, T5.5 | `verify_topology.py` (TOP-01..07) và `suite_workflow_tdd_agent.py` (TWA-*) đã viết, **chưa chạy lần nào**. Playwright CR (T4.8) chưa viết. |
| TE.5 | README xong; `.github/workflows/orca-split-e2e.yml` chưa chạy trên runner. |
| T4.2 | Repository Postgres/MySQL `step_approvals` chưa có test tích hợp với DB thật (chỉ migration đã chạy được trên Postgres). |
| T4.6 | Handler `wait_evidence` có unit test; template `change-request.json` chưa chạy (cần LLM ở G0/G2: `ORCA_E2E_REAL_AI=1`). |
| T4.7 | Có panel duyệt; **chưa có bảng gate** vì frontend không đọc được output từng bước (TDD §14, Q10). Danh sách "chờ tôi duyệt" toàn project chưa có (hook đã hỗ trợ lọc theo project). |
| T5.4 | Metrics và log chuyển trạng thái đã có, `/metrics` đã mount; chưa gọi `/metrics` thật, chưa có test. |
| T6.1, T6.2 | Chưa làm. |

**Lần chạy sau:** `TDD_AGENT_SRC=/opt/repos/tdd-agentic-ai bash tests/stages/e2e/split-topology/split-up.sh --build --multi`, rồi
`verify_topology.py --multi` và `suite_workflow_tdd_agent.py`. Không `docker stop/start` stack (Vault dev-mode); hỏng thì `split-down.sh --volumes`.

## Phụ lục — Ca kiểm `TWA-*`

| ID | Ca kiểm | Task |
|---|---|---|
| TWA-01 | Template TDD generation hoàn tất; file TDD có trên dev server, không có trên máy Orca | T3.6 |
| TWA-02 | Template Test specs hoàn tất; đủ 3 tài liệu | T3.6 |
| TWA-03 | Chạy lại bước `start` trả cùng `runId` | T3.6 |
| TWA-04 | Gate `blocked` → bước `failed`, output có `blockers[]` | T3.6 |
| TWA-05 | Gate dừng khác `expectGate` → `TDD_AGENT_UNEXPECTED_GATE` | T3.6 |
| TWA-06 | `projectDir` ngoài repo của project bị từ chối | T3.6 |
| TWA-07 | CR dừng ở G1, mục chờ duyệt xuất hiện với `decision` | T4.8 |
| TWA-08 | Duyệt đúng hash → run qua G1 | T4.8 |
| TWA-09 | Duyệt bằng hash cũ → `STEP_APPROVAL_STALE` | T4.8 |
| TWA-10 | Từ chối → bước `failed` `TDD_AGENT_APPROVAL_REJECTED` | T4.8 |
| TWA-11 | Duyệt bằng CLI trên dev server → bước `completed`, `approvedBy: external` | T4.8 |
| TWA-12 | Nhánh test specs chạy song song với duyệt G5; `finish` chờ cả hai | T4.8 |
| TWA-13 | Restart workflow-service trong `advance` → hoàn tất, không dispatch lần hai | T5.5 |
| TWA-14 | Restart workflow-service trong `approval` → hoàn tất | T5.5 |
| TWA-15 | Dừng `devserver-a` lâu → `TDD_AGENT_UNREACHABLE`; dừng ngắn → chạy tiếp | T5.5 |
| TWA-16 | Huỷ execution → run và run con tdd-agent `CANCELLED_BY_CLIENT` | T5.5 |
| TWA-17 | Giết tiến trình advance → heartbeat cũ → dispatch lại → hoàn tất | T5.5 |
