# FE-CV-TASK-092-06: Đăng ký lens `requirements`

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.1
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/` (registry lens của FE-CV-SOL-051, sửa nhỏ)
**Depends on:** FE-CV-TASK-092-05; FE-CV-SOL-051-review-workspace-shell
**Status:** [ ] TODO

## Context

- `ReviewLensId` ở CR-051 gốc không có `requirements`; hợp đồng 4.6 và CR-095 có.

## Việc cần làm

1. Chạy GitNexus `impact` (chưa chạy); thêm `requirements` vào `ReviewLensId` và registry; lazy; chỉ khi `flags.quality`.

## Kiểm thử

- Không tab chết khi cờ tắt.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Test registry xanh.

## Rủi ro

- Sửa file thuộc 051.
