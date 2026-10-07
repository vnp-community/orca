# FE-CV-TASK-059-02: Mô hình hiển thị `Finding`, lọc và sắp xếp (hàm thuần)

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.3
**Priority:** P1
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/findings/finding-view-model.ts`, `finding-filter.ts`, `finding-sort.ts` (mới) + `*.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `Finding`, `Owner`, `IndexFreshness`); FE-CV-TASK-057-01
**Status:** [x] DONE

## Context

- PQ-06: `severity ∈ error|warning|info`; `origin ∈ introduced|touched|preexisting|unknown`; `rule` là chuỗi mở; `kind` do backend điền; frontend dựng tiêu đề từ `titleKey`+`params`, vị trí từ `evidence`. `Finding` và `QualityFinding` tách nguồn (không trộn).

## Việc cần làm

1. `toFindingRow(finding, t)` (`t` = hàm dịch truyền vào để hàm không gọi `translate()` cấp module): `title` từ bảng `titleKey → khoá i18n`, thiếu ⇒ hiển thị `rule`; `location` từ `evidence[0]` (`path:line`, symbol); `ownerLabel`; `originLabelKey`; `isDismissed`; chuỗi `subject`/`params` qua `maskSensitiveText`.
2. `filterFindings(rows, {severities, origins, kinds, query, includeDismissed})` (client-side phần `kind`, `origin`, tìm kiếm trong title/file/symbol); phần `severities`, `pathPrefix`, `includeDismissed`, `scope` gửi server (hook 059-03).
3. `sortFindings(rows)`: `severity` (`error` > `warning` > `info`) rồi `origin` (`introduced` > `touched` > `preexisting` > `unknown`) rồi `findingKey` (tất định); nhóm theo `kind`.
4. `countByKindAndSeverity(rows)` cho chip.

## Kiểm thử

- `titleKey` lạ; `kind` lạ; sắp xếp tất định; lọc tổ hợp; tìm kiếm không dấu; `dismissed` ẩn mặc định.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/findings/finding-`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh; không gọi `translate()` ở cấp module (`i18n/no-top-level-translate.test.ts` xanh).
- [ ] Không trộn `QualityFinding` vào kết quả.

## Rủi ro

- Bảng `titleKey` đầy đủ chưa có (không đóng); `rule` lạ hiển thị nguyên văn.
