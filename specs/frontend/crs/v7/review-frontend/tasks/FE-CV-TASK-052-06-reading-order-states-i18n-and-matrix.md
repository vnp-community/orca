# FE-CV-TASK-052-06: Trạng thái rỗng/lỗi, i18n, ma trận lưu trữ

**From Solution:** [FE-CV-SOL-052-reading-order-and-progress](../solutions/FE-CV-SOL-052-reading-order-and-progress.md) mục 8
**Priority:** P1
**Area:** frontend / review-map + i18n + docs
**File:** `ReadingOrderList.tsx` (trạng thái), `i18n/locales/*.json`, `code-intel-locale-coverage.test.ts`, `specs/frontend/storage/feature-persistence-matrix.md`
**Depends on:** FE-CV-TASK-052-05
**Status:** [x] DONE

## Context

- Trạng thái: tải (Skeleton 8 hàng theo thang thời lượng), rỗng, `truncated`, lỗi tải `reviewState`, lỗi lưu.

## Việc cần làm

1. Rỗng: "Không có thay đổi trong phạm vi này"; nếu `changedFiles>0` mà `steps` rỗng: nêu điều quan sát được ("Không dựng được thứ tự đọc từ index"), không đoán nguyên nhân.
2. Lỗi tải tiến độ: dòng nhỏ + Thử lại, danh sách vẫn dùng được.
3. Khoá i18n (nhãn `ReasonCode` đủ 9 mã, tiến độ, phím) đủ 5 locale.
4. Thêm dòng `review-progress.ts` vào `feature-persistence-matrix.md` (bền qua `codeIntel.reviewState.*`, không local).

## Kiểm thử

- Coverage i18n; test trạng thái rỗng/lỗi.

## Tiêu chí hoàn thành

- [ ] Copy không overclaim; test xanh.

## Rủi ro

- Nhãn lý do dịch sát nghĩa cần người rà.
