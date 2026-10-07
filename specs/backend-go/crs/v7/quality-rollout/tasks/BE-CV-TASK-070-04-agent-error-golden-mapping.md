# BE-CV-TASK-070-04: Tệp vàng lỗi agent và ánh xạ `data.code` → `apperrors.Kind` → tiền tố message

**From Solution:** BE-CV-SOL-070
**Priority:** P0
**Service:** `code-intel-service` (test collector), `infra-fleet-service` (test `AgentRPCError`)
**File:** `backend-go/services/code-intel-service/testdata/agent-results/errors/*.json` (mới, do agent sinh), `.../internal/usecase/agent_error_golden_test.go` (mới), `backend-go/services/infra-fleet-service/internal/usecase/relay_agent_error_golden_test.go` (mới)
**Depends on:** BE-CV-TASK-070-01, BE-CV-SOL-023 (`domain.AgentRPCError`, trailer), BE-CV-SOL-021
**Status:** `[x] DONE`

---

## Context

- PQ-02 chặng 2: infra-fleet đổi `AgentRPCError` thành `apperrors.New(kind, "CODEINTEL_X", message≤300, err)` (message gRPC `CODEINTEL_X: message`) và đặt `error.data` vào trailer `x-orca-agent-error-data-bin` (JSON ≤ 4 KiB, hợp lệ hoá bằng bỏ phần tử, không cắt giữa chuỗi). Hiện `relay_by_dev_server.go` bọc mọi lỗi `Exec` thành `INFRA_AGENT_EXEC_FAILED`: đây là hành vi **cũ** mà CR-023 thay cho `codeintel.*`/`quality.*`.
- Bảng ánh xạ chuẩn: agent-rpc §3.4. Mã Go-sinh (`CODEINTEL_AGENT_UNSUPPORTED`, `DEV_SERVER_OFFLINE`, `RESULT_INVALID`) ở §3.3 không có tệp vàng agent.

## Việc cần làm

1. Tệp lỗi theo SOL-070 mục 5.1: phong bì `{ "code": <số>, "message": "...", "data": { "code": "CODEINTEL_X", ... } }` cho: `AMBIGUOUS_SYMBOL` (có `candidates[]` ≤ 10), `TOOL_UNAVAILABLE` (`reason:"schema_version_unsupported"`), `TOOL_FAILED` (`reason:"format_drift"`, `stderrTail` đã che), `INDEX_MISSING`, `REINDEX_IN_PROGRESS` (`jobId`), `TIMEOUT` (`reason:"queue_wait"`), `OUTPUT_TOO_LARGE`, `PATH_NOT_ALLOWED`, `REPO_NOT_REGISTERED`, `SYMBOL_NOT_FOUND`.
2. Test phía infra-fleet: bảng `data.code` → `Kind` (`FailedPrecondition`, `PermissionDenied`, `InvalidArgument`, `NotFound`, `DeadlineExceeded`, `Internal`) khớp §3.4; `status.Message` bắt đầu `^CODEINTEL_[A-Z0-9_]+: `; mã **ngoài** danh sách cho phép rơi về `INFRA_AGENT_EXEC_FAILED`; trailer ≤ 4 KiB và vẫn là JSON hợp lệ khi `candidates` quá lớn.
3. Test phía collector: từ message + trailer dựng lại `{code, data}`; không đưa `stderrTail` ra ngoài tầng service; `CODEINTEL_TIMEOUT` từ agent so với `DeadlineExceeded` do Go cắt đều ra cùng mã.
4. Khẳng định không đường dẫn tuyệt đối trong message (≥ 3 phân đoạn → `<path>`, ui-api §2.3).

## Kiểm thử

- Hai bộ test ở mục 2, 3; `go test ./services/infra-fleet-service/... ./services/code-intel-service/... -run ErrorGolden`.
- Không DB.

## Tiêu chí hoàn thành

- [x] Mỗi `data.code` §3.2 có đúng một ánh xạ `Kind` được test.
- [x] Trailer luôn JSON hợp lệ ≤ 4 KiB.
- [x] Mã lạ không làm rò `error.data` thô.

## Rủi ro và lưu ý

- Hai module (infra-fleet và code-intel) cùng sửa: PR có thể tách theo module, nhưng tệp vàng chỉ ở code-intel; infra-fleet đọc bằng đường dẫn tương đối từ gốc repo hoặc có bản sao có kiểm băm (chọn một, ghi vào PR).
- `data` của `TOOL_FAILED` không có trường `command`, nên không thể gắn nhãn `command` cho bộ đếm trôi định dạng ở service (xem BE-CV-SOL-071 Lệch L6).
