# BE-CV-TASK-080-09: Kiểm thử tích hợp tự làm mới (NATS + hai dialect)

**From Solution:** BE-CV-SOL-080
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/eventbus/auto_refresh_integration_test.go` (mới, `-tags=integration`)
**Depends on:** BE-CV-TASK-080-04, 080-06, 080-07
**Status:** [x] DONE

## Context
Kịch bản chấp nhận SOL-080 mục 4. Ma trận CI `dialect: [postgres, mysql]` của workflow `backend-go-code-intel-service.yml` (SOL-010).

## Việc cần làm
1. NATS testcontainer phát `statusChanged`; agent giả trả `codeintel.status`/`reindex`.
2. Kịch bản: 10 sự kiện → 1 job; trùng event; worktree liên kết; tắt cờ; tenant A/B; hạn mức.
3. Kiểm `git status`-tương đương: không lệnh `analyze` thiếu `--index-only` (agent giả ghi argv gửi đi: backend không gửi argv, chỉ `tools/trigger`).

## Kiểm thử
- `go test -tags=integration ./internal/adapter/eventbus/... -v` mỗi dialect.

## Tiêu chí hoàn thành
- [x] Mọi mục SOL-080 §4 có test tương ứng.

## Rủi ro
Thời gian chờ debounce: dùng `QUIET` nhỏ trong test.
