# FE-CV-TASK-090-02: Bộ dựng Markdown cho mô tả PR/MR

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/review-report-markdown.ts` (mới) + test
**Depends on:** FE-CV-TASK-090-01, 090-03
**Status:** [x] DONE

## Context

- Ngân sách 20 000 ký tự; thứ tự ưu tiên giữ; cặp dấu `orca-review`; chữ không overclaim; "PR/MR" qua `localizedHostedReviewCopy`.
- `translate()` chỉ gọi trong hàm (không top-level).

## Việc cần làm

1. Dựng các mục (nguồn, tóm tắt, rủi ro, cổng, phát hiện, hợp đồng, thứ tự đọc, sơ đồ, chân văn bản).
2. Cắt theo ưu tiên, không cắt giữa bảng/khối mã, thêm dòng rút gọn.
3. Thoát ký tự trong ô bảng.
4. Tham số `extraSections` (cho 093).

## Kiểm thử

- Golden theo provider; ngưỡng cắt; XSS dạng Markdown; `unknown`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] ≤ maxChars.
- [ ] Một cặp dấu.
- [ ] Không từ cấm.

## Rủi ro

- Render Mermaid ở nhà cung cấp chưa kiểm chứng.
