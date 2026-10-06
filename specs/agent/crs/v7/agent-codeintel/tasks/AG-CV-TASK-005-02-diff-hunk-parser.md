# AG-CV-TASK-005-02: Parser `git diff` (`--raw -z`, `--numstat -z`, `-U0`)

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-diff-hunk-parser.ts`, `codeintel-diff-hunk-parser.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context
CR-005 1.8: `@@ -83 +83 @@` (số dòng 1 bị bỏ), xoá thuần `+c,0`; `--raw -z`: `:modes sha sha Status\0path\0[path2\0]`, sha mới `0000…` với working tree; nhị phân `numstat -\t-`.

## Việc cần làm
1. `parseRawZ`, `parseNumstatZ`, `parseUnifiedZero(text)` -> khối hunk theo thứ tự tệp; `d=0` -> `pureDeletion`.
2. `matchHunksToFiles(raw, hunkBlocks)` -> lệch số tệp -> cờ `hunk_file_order_mismatch`.
3. Tên tệp có ký tự điều khiển -> `unsafePath`.

## Kiểm thử
Các dạng `@@`, `R100`/`C`, tên có khoảng trắng/Unicode/dấu nháy, nhị phân, thứ tự lệch, `new file`/`deleted`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-diff-hunk-parser.test.ts`

## Tiêu chí hoàn thành
- [ ] Mọi ca xanh; hàm thuần, không I/O.

## Rủi ro
- Định dạng Git đa phiên bản: bổ sung chế độ integration với `git` thật trong ma trận.
