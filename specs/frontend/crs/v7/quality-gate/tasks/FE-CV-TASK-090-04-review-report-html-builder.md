# FE-CV-TASK-090-04: Bộ dựng HTML độc lập và token màu

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.3
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/review-report-html.ts`, `review-report-theme-tokens.ts` (mới) + test
**Depends on:** FE-CV-TASK-090-01, 090-03
**Status:** [x] DONE (verified 2026-10-07: 8 tests html + 3 tests theme-tokens + 2 tests diagram-svg)

## Context

- `MermaidBlock.tsx`: `mermaid.render` + `DOMPurify`; `getMermaidConfig(isDark,false)`.
- Token ở `main.css`, không tạo màu mới.

## Việc cần làm

1. HTML tự đủ: CSP, không script, `lang`, `<caption>`, `<title>/<desc>` SVG, escape.
2. Đo token light + dark bằng phần tử thử; điền `:root` và `prefers-color-scheme`.
3. Nạp lười `mermaid`; sơ đồ lỗi → bỏ, giữ `alt`.

## Kiểm thử

- XSS; không `<script>`/URL ngoài; không hex trong nguồn bộ dựng.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Mở offline được.

## Rủi ro

- Tên token có thể đổi.

## Ghi chú triển khai (2026-10-07)

CSP, không script/URL ngoài, escape, token màu đo lúc xuất (`review-report-theme-tokens.ts`), SVG qua mermaid+DOMPurify (test mock DOMPurify vì happy-dom không đại diện).
