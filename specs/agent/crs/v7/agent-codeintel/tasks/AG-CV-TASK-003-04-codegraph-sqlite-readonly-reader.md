# AG-CV-TASK-003-04: Đọc SQLite CodeGraph chỉ-đọc (tuỳ chọn)

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.2
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codegraph-sqlite-reader.ts`, `codegraph-sqlite-reader.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-06](./AG-CV-TASK-001-06-gitnexus-registry-and-repo-resolution.md)
**Status:** [ ] TODO

## Context
Tiền lệ: `external-automations-handler.ts:497-520` nạp `node:sqlite` qua `requireOptional`, truyền `readOnly`. Không thêm dependency. Schema nội bộ (`nodes`, `edges`, `files`, `schema_versions`, `project_metadata`).

## Việc cần làm
1. `openCodeGraphDb(binding)`: nạp tuỳ chọn; thiếu -> `null` (không lỗi); mở `readOnly:true, timeout:2000` đúng `<projectPath>/.codegraph/codegraph.db` (realpath trong `projectPath`).
2. Kiểm schema/extractionVersion; `ORCA_CODEINTEL_SQLITE=off`.
3. Hàm `findNodes`, `callersById`, `calleesById` với SQL hằng (`SELECT` theo CR 2.4, `LIMIT ≤ 1500`); kết nối đóng nhàn rỗi 60 s và qua `closeCodeGraphDb(projectPath)` (SOL-004 gọi khi reindex).
4. `sqliteReadAvailable` = nạp được ∧ mở thử được ∧ schema trong dải; `status` đọc giá trị này.

## Kiểm thử
DB tạm bằng `node:sqlite` schema thu gọn; ghi vào DB thất bại; schema 9 -> unavailable; mock thiếu module; `mtime` không đổi; `describe.skipIf(!hasNodeSqlite)`.
Lệnh: `pnpm exec vitest run src/relay/codegraph-sqlite-reader.test.ts`

## Tiêu chí hoàn thành
- [ ] Chỉ `SELECT` hằng; không SQL từ ngoài (test quét nguồn).

## Rủi ro
- EXPLAIN QUERY PLAN chưa chạy (task 08); Node cũ không có `node:sqlite`.
