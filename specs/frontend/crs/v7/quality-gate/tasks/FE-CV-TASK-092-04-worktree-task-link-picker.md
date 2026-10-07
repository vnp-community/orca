# FE-CV-TASK-092-04: Picker task bằng Command + Popover

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/requirements/WorktreeTaskLinkPicker.tsx` (mới) + test
**Depends on:** FE-CV-TASK-092-03; `hooks/useTasks.ts` (có sẵn)
**Status:** [x] DONE — `worktree-task-link-picker.tsx` chưa tồn tại. Rà soát 2026-10-07.

## Context

- Chưa có picker task trong repo; `ui/command.tsx` + `ui/popover.tsx` có sẵn; STYLEGUIDE: chọn từ danh sách có tìm kiếm dùng `Command` trong `Popover`.

## Việc cần làm

1. Tìm theo `#TG-n` và tiêu đề; chọn → `linkTask`; "Gỡ liên kết".
2. Focus mặc định ô tìm; Enter chọn; Esc đóng.

## Kiểm thử

- Lọc, chọn, gỡ, phím (`happy-dom`).
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không thêm thư viện.

## Rủi ro

- Danh sách task lớn.
