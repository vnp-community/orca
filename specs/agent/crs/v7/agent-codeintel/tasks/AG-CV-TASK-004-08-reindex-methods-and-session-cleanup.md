# AG-CV-TASK-004-08: Method `reindex`, `reindexStatus`, `reindexCancel`, `watch` và nối `stop()`

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.1
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-methods.ts`, `codeintel-method-table.ts` (sửa), `agent-session.ts` (sửa), `codeintel-reindex-methods.test.ts` (mới), `__tests__/agent-session.test.ts` (sửa)
**Depends on:** [001](./AG-CV-TASK-004-01-codeintel-notification-sink.md), [005](./AG-CV-TASK-004-05-reindex-runner-cancel-and-verify.md), [006](./AG-CV-TASK-004-06-reindex-read-guard.md), [007](./AG-CV-TASK-004-07-index-watcher-polling.md)
**Status:** [ ] TODO

## Context
Contract §4.10–4.13, timeout agent 25 s, Go 30 s. Kết quả **không có phong bì**. `reindexStatus {jobId?}` -> `{job|null}`; `reindexCancel {jobId}` idempotent; `watch {enabled}` -> `{enabled,watching}`.

## Việc cần làm
1. **Impact trước khi sửa** `stop` (`gitnexus impact -r /opt/repos/orca stop`, chọn `agent-session.ts`); thêm đúng một dòng `cleanupCodeIntelWatchers()` cạnh `cleanupAgentWatches()` (`agent-session.ts:253`), không huỷ job.
2. Bốn method; `reindexProgress.errorCode`, `outcome` khi kết thúc.
3. Đăng ký bảng method (4 dòng).

## Kiểm thử
Hình dạng khớp ví dụ contract §7.2; `reindexStatus` không job -> `{job:null}`; `reindexCancel` job đã xong -> trạng thái cuối; `stop()` dọn watcher nhưng job còn; khoá lạ bị từ chối.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-methods.test.ts src/relay/__tests__/agent-session.test.ts`

## Tiêu chí hoàn thành
- [ ] Mất WS giữa chừng: job tiếp tục, `reindexStatus` đúng sau nối lại.

## Rủi ro
- `reindexCancel` không có kênh backend (O-17); chỉ dùng khi `CODEINTEL_AUTOANALYZE_CANCEL_ON_RESUME=true`.
