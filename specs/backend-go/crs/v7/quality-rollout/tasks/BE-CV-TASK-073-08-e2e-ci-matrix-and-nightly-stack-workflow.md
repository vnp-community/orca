# BE-CV-TASK-073-08: CI e2e hai dialect và workflow stack hằng đêm (T3, T4)

**From Solution:** BE-CV-SOL-073
**Priority:** P1
**Service:** `.github/workflows`, `backend-go/ci`, `tests/code-intel`
**File:** `.github/workflows/backend-go-code-intel-service.yml` (thêm bước e2e), `.github/workflows/code-intel-e2e.yml` (mới), `backend-go/ci/code-intel-e2e/run-code-intel-e2e.sh` (mới), `tests/code-intel/check_code_intel_flow.py`, `tests/code-intel/check_code_intel_flag.py` (mới)
**Depends on:** BE-CV-TASK-073-05..07, BE-CV-SOL-010 (workflow service), FE `FE-CV-SOL-073` (T2 chặn PR phía frontend, ngoài task này)
**Status:** `[ ] TODO`

---

## Context

- Mẫu ma trận: `backend-go-issue-status-sync.yml` (`dialect: [postgres, mysql]`, `-tags=integration`); Python: `tests/backend/check_framework.py`, `tests/mcp/mcp_check_framework.py`.
- T3: `deploy/dev/docker-compose.yml` dựng service thật; agent + công cụ thật; repo mẫu của SOL-070. Chạy được trong runner CI hay không **chưa kiểm chứng** ⇒ không chặn.
- Go CI 1.25 so với `go.work` 1.26.

## Việc cần làm

1. Workflow service: bước `go test -tags=e2e ./e2e/...` trong ma trận `dialect` (đặt `E2E_DIALECT`); chặn PR.
2. `code-intel-e2e.yml`: `schedule` + `workflow_dispatch`, `continue-on-error`; gọi `run-code-intel-e2e.sh` (dựng stack, bật `CODEINTEL_ENABLED=true`, đăng nhập, chạy `tests/code-intel/*.py`, dọn).
3. `check_code_intel_flow.py`: E01–E05, E07 qua `/ws` (khung `tests/backend/`); `check_code_intel_flag.py`: bật/tắt qua `codeIntel.settings.*`.
4. Báo cáo artifact; T4 (Orca cỡ thật) dùng chung runner benchmark (SOL-071, chưa có): job `if: vars.CODEINTEL_BENCH_RUNNER != ''`.

## Kiểm thử

- PR thử: bước e2e chạy đúng đường dẫn; `workflow_dispatch` T3 một lần (kết quả có thể đỏ do công cụ trôi; không chặn). Chưa chạy.

## Tiêu chí hoàn thành

- [ ] T1 chặn PR hai dialect; T3 chạy qua `workflow_dispatch`.

## Rủi ro và lưu ý

- Dữ liệu T4 có thể chứa mã nguồn thật; không lưu vào git.
