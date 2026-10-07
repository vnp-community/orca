# AG-CV-TASK-004-03: Nhật ký job reindex

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-journal.ts`, `codeintel-reindex-journal.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context
Để biết job `interrupted` sau khi agent khởi động lại; `~/.orca/codeintel/jobs/<jobId>.json`, thư mục `0700`, giữ 50, không ghi `message`.

## Việc cần làm
1. `writeJobStart/Finish`, `markRunningJobsInterrupted()` (gọi khi khởi tạo module), `pruneJournal(50)`; ghi nguyên tử (tệp tạm + rename); `HOME` không ghi được -> bỏ qua, không lỗi.

## Kiểm thử
Thư mục tạm làm HOME: ghi/đọc; job `running` -> `interrupted`; prune 50; mode `0700`; HOME chỉ-đọc không ném.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-journal.test.ts`

## Tiêu chí hoàn thành
- [ ] Không tệp nào chứa `message`/đường dẫn ngoài `workspaceRoot`.

## Rủi ro
- Windows chưa hỗ trợ (`0700`).
