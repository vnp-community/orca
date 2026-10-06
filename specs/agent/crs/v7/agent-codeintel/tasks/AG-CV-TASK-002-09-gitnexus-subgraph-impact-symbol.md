# AG-CV-TASK-002-09: Handler `subgraph`, `impact`, `symbol` và đọc mã nguồn symbol

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.6
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-gitnexus-subgraph.ts`, `-impact.ts`, `-symbol.ts`, `codeintel-symbol-source-reader.ts` (mới), `codeintel-method-table.ts` (sửa), `codeintel-symbol-source-reader.test.ts`, `codeintel-gitnexus-methods.test.ts` (sửa)
**Depends on:** [008](./AG-CV-TASK-002-08-gitnexus-overview-processes-process-routes.md)
**Status:** [ ] TODO

## Context
Contract §4.4–4.6. `impact`: `ambiguous` -> `AMBIGUOUS_SYMBOL` kèm `candidates` (+1 dòng), không tự chọn; `ImpactGraph` không có cạnh; `affectedModules` (PQ-19); `testsCovering` lọc theo mẫu tên khi `includeTests`, ngược lại `[]` + `tests_excluded`. `symbol`: không `--content`; mã nguồn tự đọc: `realpath` trong `workspaceRoot`, `git check-ignore -q -- <path>` (mã 0 -> `gitignored`), NUL trong 8 KiB đầu -> `binary`, ≤ 200 KiB cắt theo dòng, `source_may_not_match_index` khi stale; không cache khi có `source`.

## Việc cần làm
1. Trước khi sửa bảng method: không đổi code cũ; chỉ thêm dòng.
2. `subgraph`: duyệt theo mức (≤ 3 truy vấn, frontier ≤ 300, dừng nút ≥ `limit` hoặc cạnh ≥ 4000), loại `value`/`Section` trừ tâm, `center.symbol|file|cluster`, tham số `source` (codegraph -> rơi về GitNexus + `codegraph_source_unavailable` tới SOL-003).
3. `impact`: `target.uid|name|key` (key -> uid qua `SG_FILE_SYMBOLS`), `gitnexus impact`, `SYMBOL_FLOWS`, kind suy từ tiền tố id, `rawSummary`, cắt 300 nút theo mức.
4. `symbol`: `context -l`, `SYMBOL_FLOWS`; `codeintel-symbol-source-reader.ts` (dùng `runGit` cho `check-ignore`; mã thoát 1 = không ignore).

## Kiểm thử
Fixture + repo tạm: ambiguous 2 ứng viên; uid -> 1 nút depth 1; `limit 300`; `symbol` startLine/endLine +1, `source.text` đúng đoạn; tệp ignore, nhị phân, > 200 KiB, symlink thoát `workspaceRoot`; `subgraph` dừng đúng; test source reader không bao giờ đọc ngoài root.
Lệnh: `pnpm exec vitest run src/relay/codeintel-gitnexus-methods.test.ts src/relay/codeintel-symbol-source-reader.test.ts`

## Tiêu chí hoàn thành
- [ ] Các tiêu chí `subgraph/impact/symbol` ở solution mục 5.

## Rủi ro
- Chặn `.env*`/`*.pem` ở agent chưa chốt (mục 8 câu 2); backend che.
