# FE-CV-TASK-092-03: Hành động xác nhận/bỏ gợi ý và liên kết task

**From Solution:** [FE-CV-SOL-092-requirement-trace-view](../solutions/FE-CV-SOL-092-requirement-trace-view.md) mục 2.3
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/requirements/use-requirement-trace.ts` (mở rộng), test
**Depends on:** FE-CV-TASK-092-02
**Status:** [x] DONE — `requirement-evidence-actions.ts` chưa tồn tại. Rà soát 2026-10-07.

## Context

- `quality.trace.confirm`/`link` ≤ 8 KiB, trả `{trace}`; `scope` chưa liệt kê trong hợp đồng.

## Việc cần làm

1. `confirm`, `reject`, `linkTask`, `unlinkTask` thay trace bằng kết quả; khoá ngay; xử lý forbidden/conflict/validation.
2. `scope` mặc định `worktree` (ghi giả định).

## Kiểm thử

- Mỗi hành động; lỗi; hai lần bấm liên tiếp chỉ một lời gọi.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Không lạc quan: chỉ cập nhật khi thành công.

## Rủi ro

- `scope` chưa chốt.
