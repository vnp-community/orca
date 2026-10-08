# FE-CV-TASK-090-03: Guard cho chuỗi Mermaid của backend

**From Solution:** [FE-CV-SOL-090-review-report-export](../solutions/FE-CV-SOL-090-review-report-export.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/report/review-report-diagram-guard.ts` (mới) + test
**Depends on:** FE-CV-TASK-090-01
**Status:** [x] DONE (verified 2026-10-07: 10 tests)

## Context

- Backend trả `mermaid` ≤ 6 KiB; frontend không tin tuyệt đối.

## Việc cần làm

1. `guardMermaidSource`: từ chối > 6 KiB, ``` hàng rào, `%%{init`, thẻ HTML, ký tự điều khiển/bidi.

## Kiểm thử

- Bảng ca xấu/tốt.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Mọi chuỗi xấu bị loại.

## Rủi ro

- Guard quá chặt làm mất sơ đồ hợp lệ.

## Ghi chú triển khai (2026-10-07)

`guardMermaidSource` (too_large/fence/init_directive/html/control_chars) theo spec.
