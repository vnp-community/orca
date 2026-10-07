# AG-CV-TASK-002-06: Cache ngắn hạn và singleflight

**From Solution:** [AG-CV-SOL-002-gitnexus-extraction](../solutions/AG-CV-SOL-002-gitnexus-extraction.md) mục 2.7
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-short-lived-cache.ts`, `codeintel-short-lived-cache.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context
Contract §2.3: TTL 60 s, ≤ 64 mục, ≤ 32 MiB (LRU), singleflight; khoá `(registryPath, indexedAt|lastCommit, method, hash(paramsChuẩnHoá))`; không cache lỗi và `symbol` có `source`. Không thay cache bền (CR-022).

## Việc cần làm
1. `getOrCompute(key, compute, {cacheable})`; hash bằng JSON khoá sắp xếp ổn định (crypto sha256).
2. Ước lượng kích thước bằng `Buffer.byteLength(JSON.stringify)`; loại LRU khi vượt.
3. `invalidateShortLivedCache(registryPath?)`.
4. Giá trị lưu là `data`+`totalCount`+`truncated`+`warnings`; `stale`, `headCommit`, `perf` do caller tính lại.

## Kiểm thử
Đồng hồ giả: TTL; hai lần đồng thời một compute; lỗi không cache; vượt 64 mục/32 MiB loại LRU; invalidate; `cacheable:false`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-short-lived-cache.test.ts`

## Tiêu chí hoàn thành
- [x] Spawn một lần trong 60 s cho cùng khoá.

## Rủi ro
- Tỷ lệ trúng chưa đo; 60 s là giả định.
