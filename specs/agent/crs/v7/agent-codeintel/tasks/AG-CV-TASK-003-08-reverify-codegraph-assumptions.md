# AG-CV-TASK-003-08: Re-verify giả định CodeGraph trên máy thật

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 7
**Priority:** P1
**Area:** `agent/` (kiểm chứng; chỉ lệnh đọc)
**File:** `agent/src/relay/codeintel/__fixtures__/codegraph-1.4.1/*.json` (mới)
**Depends on:** [007](./AG-CV-TASK-003-07-codegraph-enrichment-of-symbol-subgraph-impact.md)
**Status:** [x] DONE

## Context
Chưa chạy lại: hình dạng `-j`, `files` phẳng, `affected --stdin`, `EXPLAIN QUERY PLAN`, `node:sqlite` trên Node đích, `codegraph telemetry status`, `ExperimentalWarning`.

## Việc cần làm
1. Chụp `status -j`, `query -j`, `files -j`, `affected -j` làm fixture (rút gọn).
2. `EXPLAIN QUERY PLAN` cho từng SQL hằng; ghi vào PR.
3. Kiểm `node --version` và nạp `node:sqlite` trên dev server; ghi cờ cần.
4. `codegraph telemetry status`: nếu bật, đề xuất cách tắt (tên biến chưa kiểm chứng).

## Kiểm thử
Chạy lại các test task 01–07 trên fixture mới: `pnpm exec vitest run src/relay/codegraph-cli-output.test.ts src/relay/codeintel-codegraph-methods.test.ts`

## Tiêu chí hoàn thành
- [x] Mục "chưa kiểm chứng" ở solution 7 được cập nhật.

## Rủi ro
- Không chạy lệnh ghi (`index`, `sync`, `unlock`).
