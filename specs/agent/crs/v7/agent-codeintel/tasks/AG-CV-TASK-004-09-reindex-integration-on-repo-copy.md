# AG-CV-TASK-004-09: Kiểm chứng thủ công `analyze`/`index` trên bản sao nhỏ

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 7
**Priority:** P1
**Area:** `agent/` (kiểm chứng; **ghi** vào bản sao, không vào Orca)
**File:** cập nhật `codeintel-reindex-progress.ts`, fixture `agent/src/relay/codeintel/__fixtures__/reindex/*.txt` (mới)
**Depends on:** [008](./AG-CV-TASK-004-08-reindex-methods-and-session-cleanup.md)
**Status:** [x] DONE

## Context
Chưa ai chạy `analyze`. Cần chốt: định dạng tiến độ/`%`, thời gian, RAM, `git status` sau analyze, hiệu ứng huỷ giữa chừng, đọc đồng thời.

## Việc cần làm
1. Tạo repo fixture vài chục tệp trong thư mục tạm; chạy qua `codeintel.reindex` thật (`gitnexus analyze --index-only`, `codegraph index`).
2. Ghi đầu ra tiến độ, thời gian, `git status --porcelain` (phải sạch), `indexedAt` đổi.
3. Huỷ giữa chừng: có `lbug.wal.missing-shadow.*`? `status` sau đó?
4. Thử đọc `overview` trong lúc analyze để cân nhắc nới chặn (Q3).
5. Cập nhật mẫu `percent`, mục "chưa kiểm chứng" của solution.

## Kiểm thử
Test progress với fixture thật: `pnpm exec vitest run src/relay/codeintel-reindex-progress.test.ts`

## Tiêu chí hoàn thành
- [x] Kết quả ghi vào PR; mẫu `percent` chốt hoặc ghi "luôn null".

## Rủi ro
- Không chạy trên `/opt/repos/orca` (làm bẩn chỉ mục dùng chung).
