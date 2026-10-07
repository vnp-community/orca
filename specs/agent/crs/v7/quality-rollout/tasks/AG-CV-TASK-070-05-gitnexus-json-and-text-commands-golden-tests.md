# AG-CV-TASK-070-05: Test vàng `context`, `impact`, `query`, `detect-changes`, `status` của GitNexus

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/gitnexus-json-commands.golden.test.ts`, `agent/src/relay/codeintel/gitnexus-text-commands.golden.test.ts` (mới); fixture `agent/src/relay/codeintel/__fixtures__/gitnexus/1.6.9/{context-*,impact-*,query-found,detect-changes-unstaged.txt,status-stale.txt,list.txt,meta.json,no-repo-flag.stderr.txt}`
**Depends on:** 070-02, 070-03; AG-CV-SOL-002, AG-CV-SOL-005
**Status:** [x] DONE

## Context

Hợp đồng: agent-rpc §§3.2, §4.5, §4.6, §4.8. Đã chạy golden tests cho cả JSON và văn bản.
`impact` không tìm thấy trả `{error:"Target … not found", impactedCount:0, risk:"UNKNOWN"}` exit 0; `context` có `ambiguous`+`candidates` (≤ 10, hợp đồng §3.2); nhóm cạnh snake_case động.
`detect-changes` và `status/list` là **văn bản người đọc** (không có JSON); trần 15 symbol, 10 luồng theo README v7 điểm 8 (chưa kiểm).

## Việc cần làm

1. Test JSON: `found`, `ambiguous` → `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates`; `not found` → `SYMBOL_NOT_FOUND`; thiếu khoá bắt buộc → `format_drift`.
2. Test văn bản: `detect-changes` → danh sách symbol, số tệp, mức rủi ro; biến thể CRLF; số 0; dòng lạ → `format_drift` (không bỏ qua im lặng).
3. `meta.json` (đã loại `fileHashes`, `cacheKeys`): đọc `schemaVersion`, `lastCommit`, `indexedAt` (dùng cho task 07).
4. `no-repo-flag.stderr.txt`: kết quả `REPO_NOT_REGISTERED`/`TOOL_FAILED`, thông điệp **không** chứa tên repo khác hay đường dẫn nội bộ công cụ.

## Kiểm thử

- Hai tệp test, golden byte-exact.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/gitnexus-json-commands.golden.test.ts src/relay/codeintel/gitnexus-text-commands.golden.test.ts`.

## Tiêu chí hoàn thành

- [x] Phủ tất cả hình dạng liệt kê; `format_drift` chứ không đoán.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Nếu AG-CV-SOL-005 chuyển `detect-changes` sang Cypher thì fixture văn bản còn cần không (câu hỏi mở 6 của solution).
