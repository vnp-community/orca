# FE-CV-TASK-087-12: Danh sách phát hiện kiểm tra: lọc, sắp, ảo hoá, phân trang

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.4
**Priority:** P0
**Area:** frontend / components
**File:** `frontend/src/renderer/src/components/review-map/quality/findings/QualityFindingsList.tsx`, `QualityFindingRow.tsx`, `QualityFindingsToolbar.tsx`, `quality-finding-filter.ts`, `quality-finding-sort.ts`; `hooks/useQualityFindings.ts` (mới) và `*.test.ts(x)`
**Depends on:** 087-02, 087-03, FE-CV-TASK-088-03
**Status:** [x] DONE

## Context

- `@tanstack/react-virtual` ^3.13.24 đã dùng (`CsvViewer`, `WorktreeList`, `SearchResultsPane`); hợp đồng: `limit≤500`, `pageToken`, `totalCount`, `truncated`, `outsideScopeCount`.
- `ruleId` mã lạ hiển thị nguyên văn (PQ-26); U9 văn bản thuần.

## Việc cần làm

1. `useQualityFindings(worktreeId, filters)`: `severities/categories/inScope/file` gửi server; nối trang bằng `pageToken`, trần 5 000, khử trùng `fingerprint`; trả `{items,total,truncated,outsideScopeCount,loadMore}`.
2. `quality-finding-filter.ts` (tìm chữ, hiện đã miễn trừ) và `quality-finding-sort.ts` (mức → category → file → line → fingerprint), thuần.
3. List ảo hoá > 50 hàng; `QualityFindingRow`: `SeverityBadge`, `ruleId`, `message` kẹp 2 dòng + Tooltip, `file:line[-endLine]`, `tool toolVersion`, `fixHint` thu gọn; dòng miễn trừ có nhãn người/hạn.
4. Dòng trạng thái "Đang hiển thị X/Y" và "N ngoài phạm vi (không tính vào cổng)"; trạng thái rỗng giải thích vì sao (chưa chạy / không phát hiện trong phạm vi các kiểm tra đã chạy).

## Kiểm thử

Filter/sort thuần; list: số hàng DOM < tổng, `getItemKey`, loadMore, trần 5 000; chuỗi có HTML hiển thị chữ; `ruleId` lạ. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/findings`.

## Tiêu chí hoàn thành

- [ ] Không cắt im lặng; sắp ổn định.
- [ ] Không hex; không toast cho lỗi tải (inline + thử lại).

## Rủi ro

- Hiệu năng 5 000 hàng chưa đo.
