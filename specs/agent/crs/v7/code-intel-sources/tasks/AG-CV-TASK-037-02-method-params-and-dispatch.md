# AG-CV-TASK-037-02: Validate tham số, dòng method table (55 s), handler điều phối theo `kind`

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.1,5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-facts.ts` (mới), `.test.ts`; sửa `codeintel-method-table.ts` (AG-CV-SOL-001)
**Depends on:** AG-CV-SOL-001 (method table, errors, envelope, limits), AG-CV-TASK-037-01
**Status:** [ ] TODO

## Context

Hợp đồng §4.9 và §2.1: tham số lạ bị từ chối; chuỗi từ client 1..512 ký tự, không NUL/điều khiển, không bắt đầu `-` (kể cả U+FF0D, U+2212 sau NFKC); đường dẫn tương đối gốc, không `..`, không `\`.

## Việc cần làm

1. `validateStructuralFacts(params)`: `workspaceRoot`, `kind` (5 giá trị), `pair?` (4 giá trị, chỉ khi `kind=layerImports`), `pathPrefixes?` (≤ 20, mỗi cái hợp lệ; mặc định `["backend-go/services/"]`; phải kết thúc bằng `/`), `limit` (1..5000, mặc định 5000), `offset` (≥ 0), `_trace`.
2. Dòng `codeintel.structuralFacts` trong `CODEINTEL_METHODS`: `timeoutMs = 55_000` (env `ORCA_CODEINTEL_DETECT_TIMEOUT_MS` hạ về 25 000, dùng chung hằng với `detectChanges`).
3. `handleStructuralFacts`: phân giải repo (`resolveCodeIntelRepo`), kiểm GitNexus có chỉ mục (`INDEX_MISSING`/`TOOL_UNAVAILABLE`/`REINDEX_IN_PROGRESS`), gọi bộ xử lý theo `kind` (task 03-06), dựng phong bì qua `buildCodeIntelResult` (`sources`, `stale`, `truncated`, `totalCount`, `perf`).
4. Sắp xếp và cắt trang chung (task 07 hoàn thiện); không chạm CodeGraph.
5. Không có tham số ngoài danh sách; test phản chiếu schema khẳng định không có `command|argv|args|cypher|repo|env|cwd|timeout`.

## Kiểm thử

Bảng ca tham số (lạ, `pair` sai chỗ, `pathPrefixes` 21 phần tử, `..`, `-x`, `limit:5001`, `kind` thiếu); bộ xử lý giả trả hàng cố định → phong bì đúng; chưa có chỉ mục → `INDEX_MISSING`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-facts.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi tham số lạ bị từ chối; method ở bảng `CODEINTEL_METHODS`.

## Rủi ro

Chữ ký `CODEINTEL_METHODS`/`buildCodeIntelResult` do SOL-001 chốt: sửa lớp nối nếu khác.
