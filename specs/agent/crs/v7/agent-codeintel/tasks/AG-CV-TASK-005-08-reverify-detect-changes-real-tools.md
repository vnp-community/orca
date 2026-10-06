# AG-CV-TASK-005-08: Re-verify trên công cụ thật và ma trận Git

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 7
**Priority:** P1
**Area:** `agent/` (kiểm chứng; chỉ lệnh đọc)
**File:** fixture `agent/src/relay/codeintel/__fixtures__/detect-changes/*` (mới)
**Depends on:** [007](./AG-CV-TASK-005-07-detect-changes-method.md)
**Status:** [ ] TODO

## Context
Chưa chạy: `FILE_SYMBOLS_BATCH` với `MATCH (n)` + `IN`; hiệu năng diff 446 tệp/1 508 symbol; Git 2.25.

## Việc cần làm
1. Chạy `codeintel.detectChanges` thật trên `/opt/repos/orca` (`base` HEAD~30), ghi thời gian từng giai đoạn.
2. Chạy bộ test parser/merge-base với Git 2.25.x, 2.38.x, 2.49.x (hoặc xác nhận tên job CI hiện có).
3. Kiểm `unborn`, đổi tên, tệp tên đặc biệt trên repo tạm.
4. Cập nhật mục 7 của solution; nếu `IN` lỗi, đề xuất truy vấn thay.

## Kiểm thử
`pnpm exec vitest run src/relay/codeintel-merge-base-resolution.test.ts src/relay/codeintel-diff-hunk-parser.test.ts src/relay/codeintel-detect-changes.test.ts`

## Tiêu chí hoàn thành
- [ ] Thời gian thực đo < 55 s hoặc ghi rõ cần phân trang.

## Rủi ro
- Chạy trên repo thật chỉ đọc; không `analyze`.
