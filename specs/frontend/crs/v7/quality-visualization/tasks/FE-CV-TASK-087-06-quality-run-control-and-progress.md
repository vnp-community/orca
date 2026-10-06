# FE-CV-TASK-087-06: `QualityRunControl`, tiến độ, huỷ, lỗi chạy

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.6
**Priority:** P0
**Area:** frontend / components
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityRunControl.tsx`, `QualityRunProgress.tsx`, `QualityLensToolbar.tsx` (mới) và `*.test.tsx`
**Depends on:** 087-03, 087-04
**Status:** [ ] TODO

## Context

- STYLEGUIDE UX rule 1: khoá nút ngay, spinner trễ ~200 ms từ xa (100 ms cục bộ), nhãn giữ độ rộng cố định; UX rule 3: Cancel không destructive (ghost, không chip phím).
- `ui/progress.tsx` không hỗ trợ `null`/có transition cố định → spinner tĩnh `Loader2` với `motion-reduce:animate-none`.
- O11: chỉ tên profile (`RunnableProfile.id`), không ô lệnh. Không phím tắt cho chạy ở MVP.

## Việc cần làm

1. `Select` profile từ `runnable`; `ready:false` → khoá nút + liệt kê `missing[] {check,reason,hint?}`; `heavy` ghi "nặng"; `Select` phạm vi theo `scopes`.
2. Máy trạng thái hiển thị theo SOL-087 2.6 (starting, queued, running + `stage/message/stepIndex`, cancelling, finished: succeeded/failed|interrupted/cancelled).
3. Xử lý lỗi: `run-in-progress` gắn lại; `profile-unknown` làm mới; `env-not-ready` inline + "Kiểm tra lại"; `rate-limited` (`retryAfterSeconds`); `forbidden` khoá chạy, vẫn đọc; không toast.
4. Chi tiết lỗi chạy sao chép được.

## Kiểm thử

Khoá ngay khi bấm đúp; spinner trễ (fake timers, hai mức local/remote); `percent=null` vs số; huỷ không nói "đã huỷ" trước `finished`; `ready:false`; mỗi mã lỗi; `motion-reduce` class. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityRunControl`.

## Tiêu chí hoàn thành

- [ ] Không ô nhập lệnh; không chip phím giả.
- [ ] Cancel không tô destructive.
- [ ] Không toast lỗi chạy.

## Rủi ro

- Phân biệt "mục tiêu từ xa" cho độ trễ: dùng cùng hàm CR-050 (chưa có code).
