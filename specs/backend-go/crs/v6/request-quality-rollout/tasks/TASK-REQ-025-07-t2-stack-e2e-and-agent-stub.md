# TASK-REQ-025-07: T2 e2e trên stack dev (Python), stub dev server agent và workflow `request-e2e.yml`

**From Solution:** BE-REQ-SOL-025
**Priority:** P1
**Service:** `tests/request`, `backend-go/ci`, `.github`
**File:** `tests/request/check_request_flow_types.py` (mới), `tests/request/check_request_jira_sync.py` (mới), `tests/request/request_env_config.py` (mới), `tests/request/stubs/` (mới: stub dev server agent và Jira), `backend-go/ci/request-e2e/run-request-e2e.sh` (mới), `.github/workflows/request-e2e.yml` (mới)
**Depends on:** BE-REQ-SOL-016 (kênh WS), BE-REQ-SOL-024 (Jira sync), TASK-REQ-025-01; kịch bản MCP: TASK-REQ-017-05
**Status:** [ ] TODO (một phần, xem Tiến độ 2026-10-08)

---

## Context

- Hai khung Python có sẵn: `tests/mcp/mcp_check_framework.py` (PASS/FAIL/SKIP, đăng nhập, PAT tạm, WebSocket, dọn dẹp) và `tests/backend/` (`check_framework.py`, `env_config.py`, `orca_api_session.py`, `api_websocket_channels.py`). Dùng khung `tests/backend/` cho kịch bản `/ws`, `tests/mcp/` cho E17.
- Đường AI là relay `ai.complete` và `agent.execPrompt` của dev server agent qua `infra-fleet-service` (README v6 mục 8 dòng 1). **Không có stub trong repo** (SOL-025 mục 1 điểm 1). Cần quyết định: stub giao thức agent (WebSocket relay) hay chạy agent thật ở chế độ giả; đọc `agent/src` phần relay và `infra-fleet-service` `Relay` trước khi chọn.
- `deploy/dev/docker-compose.yml` dựng service thật; `MCP_ENABLED: ${MCP_ENABLED:-false}` là mẫu cờ; `.github/workflows/e2e.yml` hiện là Playwright cho frontend.
- Jira giả: `issue-tracking-service` trỏ vào stub HTTP (hành vi `GetIssue`, `ListTransitions`, `UpdateIssue`); đọc `issue-tracking-service/internal/adapter/jira/client.go` để biết phương thức cần giả.

## Việc cần làm

1. Chốt cách stub agent (ghi vào mô tả PR, cập nhật SOL-025 Q2); hiện thực tối thiểu trả JSON hợp lệ cho `ai.complete` theo loại prompt (phân loại, Solution, Plan) và cho `agent.execPrompt` (Chẩn đoán).
2. `run-request-e2e.sh`: `docker compose up` các service cần (`request-service`, `task-service`, `api-gateway`, `issue-tracking-service`, `issue-status-sync`, NATS, Postgres, Vault hoặc cách `deploy/dev` đang dùng), migrate (`deploy/dev/scripts/migrate.sh`), bật `REQUEST_FLOW_ENABLED=true`, chạy các script Python, dọn dẹp, thoát mã đúng.
3. `check_request_flow_types.py`: tạo Request cho 11 loại qua `/ws` (`request.create`, `request.confirmType`, `solution.*`, `approval.*`, `request.generatePlan`, `request.startPhase`), kiểm chuỗi trạng thái và Approval cho từng loại theo registry (đọc từ `request.flow` nếu CONTRACT chốt, nếu không thì hằng trong script có nguồn trích dẫn).
4. `check_request_jira_sync.py`: E18: Request nguồn Jira `executing` thì Jira giả nhận "In Progress"; PR merge giả trong lúc Request chưa xong không chuyển "Done"; `completed` thì "Done".
5. `.github/workflows/request-e2e.yml`: `on: schedule` (hằng đêm) và `workflow_dispatch`; không chặn PR; tải log khi lỗi; biến `ORCA_REQUEST_E2E_REAL_AI` (T3) mặc định `false`.

## Kiểm thử

- Chạy tay: `bash backend-go/ci/request-e2e/run-request-e2e.sh` trên máy có Docker; ghi kết quả vào PR (không có thì "chưa chạy").
- `workflow_dispatch` thử một lần trên nhánh.

## Tiêu chí hoàn thành

- [ ] Script chạy được cục bộ, thoát mã khác 0 khi sai.
- [ ] Workflow chạy qua `workflow_dispatch`.
- [ ] E18 xanh với Jira giả.

## Rủi ro và lưu ý

- Chưa kiểm chứng stack dev chạy trong runner CI (nhiều service, Vault, NATS); nếu không được, chỉ chạy cục bộ và ghi rõ.
- Task lớn: nếu stub agent quá tốn công, tách thành task riêng và để T2 chỉ chạy các luồng không cần AI.

## Tiến độ (2026-10-08)

Đã làm: stub agent là biên relay của infra-fleet (`e2e/stubs` + binary `e2e/cmd/agent-stub`, quyết định ghi ở IMPLEMENTATION-NOTES; có unit test, và đã được e2e T1 dùng chung code); `backend-go/ci/request-e2e/docker-compose.e2e.yml` (compose merge đã `config` thành công) và `run-request-e2e.sh` (`bash -n` sạch); `tests/request/request_env_config.py` và `check_request_flow_types.py` (biên dịch được); `.github/workflows/request-e2e.yml` (`actionlint` sạch, cron + `workflow_dispatch`, không chặn PR).
Còn thiếu: chưa chạy lần nào trên stack dev (cần shared Vault, nhiều service); `check_request_jira_sync.py` (E18) và stub Jira chưa viết; E17 thuộc TASK-REQ-017-05; phụ thuộc CR-REQ-007..013 cho các giai đoạn sau xác nhận loại. Không đánh DONE.
