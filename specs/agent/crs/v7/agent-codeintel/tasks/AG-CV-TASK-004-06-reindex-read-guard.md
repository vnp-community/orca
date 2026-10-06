# AG-CV-TASK-004-06: Chặn đọc khi đang làm mới chỉ mục

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.4
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-read-guard.ts`, `codeintel-reindex-read-guard.test.ts` (mới); sửa nhẹ `gitnexus-cypher-runner.ts`, `codegraph-sqlite-reader.ts`
**Depends on:** [005](./AG-CV-TASK-004-05-reindex-runner-cancel-and-verify.md)
**Status:** [ ] TODO

## Context
Bảo thủ theo bằng chứng `lbug.wal.missing-shadow.*`; đọc đồng thời với analyze chưa kiểm chứng.

## Việc cần làm
1. `assertReadable(binding, tool)`: job GitNexus đang chạy -> `REINDEX_IN_PROGRESS {jobId,state,stage}`; CodeGraph đang chạy -> đóng SQLite, đọc CLI hoặc cùng lỗi.
2. Gọi ở runner Cypher và probe (probe trả `state:'building'` thay vì lỗi).

## Kiểm thử
Trong job GitNexus: `overview` lỗi, `codegraphSearch` chạy; trong job CodeGraph: SQLite đóng.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-read-guard.test.ts`

## Tiêu chí hoàn thành
- [ ] Không truy cập `lbug` khi có job GitNexus của repo.

## Rủi ro
- Không phát hiện được analyze do bên ngoài: chỉ cảnh báo `other_processes_may_hold_index`.
