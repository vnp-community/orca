# AG-CV-TASK-037-07: Phân trang `limit/offset`, cache ngắn hạn, lỗi từng phần, hợp đồng JSON

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.1,7
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-facts.ts` (hoàn thiện), `codeintel-structural-facts-contract.test.ts` (mới)
**Depends on:** AG-CV-TASK-037-03..06, AG-CV-SOL-002 (`codeintel-short-lived-cache.ts`)
**Status:** [x] DONE

## Context

Hợp đồng §2.2/§2.3: `truncated`, `totalCount`, cache 60 s khoá `(registryPath, indexedAt|lastCommit, method, hash(paramsChuẩnHoá))`, kết quả JSON ≤ 8 MiB (cắt dần + `truncated`, vẫn vượt → `OUTPUT_TOO_LARGE`).

## Việc cần làm

1. `paginate(rows, offset, limit) → { rows, totalCount, truncated }` dùng chung bốn `kind` (cycles theo vòng); `truncated = totalCount > offset + rows.length`.
2. Cache theo khoá hợp đồng (không cache lỗi); `codeintel.indexChanged`/reindex xong huỷ cache (cơ chế SOL-002).
3. Quy đổi lỗi: timeout CLI → `CODEINTEL_TIMEOUT data.tool`; stdout > 16 MiB → `OUTPUT_TOO_LARGE`; Cypher lỗi `{error}` exit 0 → `TOOL_FAILED reason="unknown_shape"|"unexpected_columns"`; dòng lệch cột bị bỏ + `warnings:["malformed_rows_dropped"]` (mã tạm của SOL-002).
4. Kiểm cỡ JSON cuối: > 8 MiB → cắt hàng cuối + `truncated`.
5. Test hợp đồng: `JSON.stringify(result)` của mỗi `kind` khớp ví dụ §4.9 (khoá, kiểu), đối chiếu fixture vàng (`structural-*.expected.json`).

## Kiểm thử

Kịch bản cache hit/miss/huỷ; phân trang 3 trang cuối; JSON quá lớn (hàng giả); tất cả mã lỗi ở bảng; so khớp fixture vàng cho 5 `kind`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-facts-contract.test.ts src/relay/codeintel-structural-facts.test.ts && pnpm test`.

## Tiêu chí hoàn thành

- [x] Mọi `kind` khớp ví dụ hợp đồng; thứ tự và `totalCount` xác định.
- [x] Không cache lỗi; huỷ cache khi `indexChanged`.

## Rủi ro

Mã `warnings` tạm (`malformed_rows_dropped`) chưa có trong hợp đồng: đồng bộ với chủ SOL-002.
