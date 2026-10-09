# TASK-FE-E2E-007: Cây task: phân biệt rỗng, lỗi và không có quyền

**From Solution:** SOL-FE-E2E-003 · **Resolves:** FE-E2E-E, BUG-E2E-006
**Priority:** P2 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/task/ (cây con)
- frontend/src/renderer/src/hooks/useTask*.ts
**Depends on:** TASK-E2E-011, 012
**Status:** `[ ]` TODO

---

## Context

`task.getSubtree` từng trả `{}` cho cả chủ task; UI không có cách phân biệt.

## Việc cần làm

- [ ] Xác minh cách UI dùng `getSubtree`/`list` để dựng cây (đọc mã trước).
- [ ] Trạng thái rỗng ("chưa có task con") khác trạng thái lỗi ("không tải được", có thử lại).
- [ ] Xoá task: `TASK_NOT_FOUND` coi là thành công.

## Tiêu chí hoàn thành

- [ ] Test render ba trạng thái; thử với task cũ sau backfill.

## Kiểm thử

Vitest.
