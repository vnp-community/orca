# AG-CV-TASK-070-06: Test vàng parser CodeGraph (JSON, ANSI "not found", SQLite schema)

**From Solution:** [AG-CV-SOL-070-golden-fixtures-and-parsers](../solutions/AG-CV-SOL-070-golden-fixtures-and-parsers.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/codegraph-output.golden.test.ts` (mới); fixture `agent/src/relay/codeintel/__fixtures__/codegraph/1.4.1/*` (mới)
**Depends on:** 070-02, 070-03; AG-CV-SOL-003
**Status:** [x] DONE

## Context

Hợp đồng: agent-rpc §§2.2, §4.14. Đã chạy golden tests cho CodeGraph.
CR-070 §1: `status --json`, `query --json` (`[{node,score}]`; rỗng `[]`), `callers|callees` `{symbol,callers|callees:[...]}`; **không tìm thấy** → văn bản có ANSI dù có `--json`; `files --json` rất lớn (cắt 20 mục). `commit` của CodeGraph luôn `null` (§2.2).
SQLite chỉ đọc (`schema_versions`, `project_metadata`) nếu CR-003 dùng; `node:sqlite` cần Node ≥ 22.5 (§10).

## Việc cần làm

1. Test từng lệnh; `[]` và ANSI "not found" → "không có kết quả", không phải `format_drift`; JSON hỏng thật → `format_drift`.
2. `sqlite-schema.json` → `extractionVersion` 24, `schema_versions` max 8 (dùng cho task 07).
3. Chụp ANSI trong đúng điều kiện `spawn` của agent (`stdio` pipe, `NO_COLOR=1`); ghi vào `MANIFEST.captureEnv`.
4. Lỗi quy trình chung: exit 0 + `error`; stderr không chuyển nguyên.

## Kiểm thử

- Golden byte-exact; test CRLF/ANSI.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/codeintel/codegraph-output.golden.test.ts`.

## Tiêu chí hoàn thành

- [x] Phủ đủ danh sách; ANSI không bị coi là lỗi cú pháp.
- [x] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- ANSI có thể không xuất hiện khi không TTY: fixture phải chụp đúng điều kiện (chưa kiểm).
