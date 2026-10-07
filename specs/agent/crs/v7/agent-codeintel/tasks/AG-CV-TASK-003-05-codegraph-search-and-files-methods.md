# AG-CV-TASK-003-05: Method `codeintel.codegraphSearch` và `codeintel.files`

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-codegraph-search.ts`, `codeintel-codegraph-files.ts`, `codeintel-method-table.ts` (sửa), `codeintel-codegraph-methods.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-003-01-codegraph-cli-output-parsing.md), [003](./AG-CV-TASK-003-03-codeintel-symbol-ref-codegraph.md)
**Status:** [x] DONE

## Context
Contract §4.14. Backend phải chịu `-32601` (phát hành đợt 4).

## Việc cần làm
1. `codegraphSearch`: `search` 1..256, `limit` 1..50, `kind` trong tập hằng; `query -j`; bỏ `import`; phong bì với `sources[{tool:'codegraph',commit:null,lineBase:1}]`.
2. `files`: `filter` tương đối ≤ 512 không `-` đầu, không `..`; `limit` ≤ 5000; `truncated`.
3. Đăng ký 2 dòng (25 s).

## Kiểm thử
Binary giả fixture: 2 hàm `runToolCommand`; khoá lạ bị từ chối; `filter:'-x'`, `'../a'` bị từ chối trước spawn.
Lệnh: `pnpm exec vitest run src/relay/codeintel-codegraph-methods.test.ts`

## Tiêu chí hoàn thành
- [ ] Kết quả đúng hình dạng §4.14; không nút `import`.

## Rủi ro
- Hình dạng `query -j` chưa chạy lại.
