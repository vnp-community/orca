# AG-CV-TASK-003-01: Parse đầu ra CLI CodeGraph và chặn text lỗi

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.2
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codegraph-cli-output.ts`, `codegraph-cli-output.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md)
**Status:** [x] DONE

## Context
CR-003 1.2–1.3: `status/query/callers/callees/impact/files/affected` có `-j`; lỗi là text ANSI exit 0 (`ℹ Symbol "X" not found`, `✗ CodeGraph not initialized in /tmp`); `files -j` luôn mảng phẳng `{path,language,nodeCount,size}`; `affected -j` -> `{changedFiles, affectedTests[]}`.

## Việc cần làm
1. `parseCodeGraphQuery/Callers/Callees/Files/Affected(stdout)` sau `classifyCodeGraphStdout`; lỗi hình dạng -> `TOOL_FAILED reason='unknown_shape'`.
2. Cắt `files` theo `limit` -> `truncated`.

## Kiểm thử
Fixture thật: mảng rỗng, text ANSI, `initialized:false`, `affected` nhiều test.
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codegraph-cli-output.test.ts`

## Tiêu chí hoàn thành
- [ ] Ca text lỗi quy đúng `SYMBOL_NOT_FOUND`/`INDEX_MISSING`.

## Rủi ro
- Hình dạng JSON nội bộ 1.4.1, chưa chạy lại.
