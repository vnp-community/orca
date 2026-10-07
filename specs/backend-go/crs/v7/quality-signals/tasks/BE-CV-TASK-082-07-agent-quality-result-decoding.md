# BE-CV-TASK-082-07: Giải mã kết quả `quality.*` từ agent và làm sạch

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/agentquality/decode.go`, `client.go`, `backend-go/services/code-intel-service/internal/usecase/quality_run_ports.go` (`AgentQualityClient`, `FindingSanitizer`), test (mới)
**Depends on:** BE-CV-SOL-021-agent-collector, BE-CV-SOL-023-infra-fleet-codeintel-transport, BE-CV-TASK-082-03
**Status:** [x] DONE

## Context
C-AG §5.3, §5.5, §6.4. Golden `testdata/agent-results/*.json` (cổng G1). Lỗi giải mã → `CODEINTEL_RESULT_INVALID` (mã Go sinh, C-AG §3.3). Owner gói che secret chưa chốt (O-16).

## Việc cần làm
1. `RunStatus`, `Results(view=findings|steps)`, `Cancel`: tham số chỉ `workspaceRoot`, `runId`, `offset`, `limit ≤ 500`, `view`, `stepId` (không `command/args/env`).
2. Giải mã có kiểu; khoá bắt buộc thiếu, số âm, `nextOffset` không tiến, trang > 1 MiB → lỗi.
3. `FindingSanitizer`: chặn đường dẫn tuyệt đối/`..`/`\`, che token/DSN/đường dẫn tuyệt đối; **không** lưu dòng nguồn.
4. Ánh xạ `AgentRPCError` (SOL-023) sang mã `CODEINTEL_*`.

## Kiểm thử
- Golden G1; khoá lạ vẫn nhận; canary secret trong `message` bị che.
- Test phản chiếu: không có tham số `command|argv|args|env|cwd|timeout` (§8.3 mục 5).

## Tiêu chí hoàn thành
- [x] Không lưu/ghi log nội dung thô của agent.

## Rủi ro
Hình dạng JSON chưa chạy thật.
