# FE-CV-TASK-062-01: Parser thủ công `MobileReviewSummary` (mobile)

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.3
**Priority:** P2
**Area:** mobile / session (hàm thuần)
**File:** `mobile/src/session/mobile-review-summary-rpc.ts` (mới) + `mobile-review-summary-rpc.test.ts`
**Depends on:** không
**Status:** [x] DONE

## Context

- Hình dạng: UI-API §8 `MobileReviewSummary`. Mobile không dùng zod; mẫu `mobile-diff-review-rpc.ts` (`readString/readNumber/readBoolean`). Kiểu khai báo **cục bộ** (không `vendor-shared`).

## Việc cần làm

1. Khai báo `MobileReviewSummary` cục bộ (mirror §8, `bySeverity` chỉ `error|warning|info`).
2. `readMobileReviewSummaryResult(value)`: bỏ trường lạ; ép enum (`risk.level` lạ ⇒ `UNKNOWN`, `severity` lạ ⇒ `info`, `origin` lạ ⇒ `unknown`, `reason` lạ ⇒ bỏ); cắt chuỗi (title 200, summary 400), tối đa 50 mục, số đếm âm ⇒ 0; không ném.

## Kiểm thử

- Dữ liệu đúng; thiếu trường; enum lạ; mảng dài; chuỗi dài; mỗi `reason`; `null`/kiểu sai; `high|medium|low` bị bỏ qua.
- `pnpm --dir mobile test -- mobile-review-summary-rpc` (lệnh mobile chưa kiểm chứng; theo `mobile/vitest.config.ts`).

## Tiêu chí hoàn thành

- [ ] Test xanh; không phụ thuộc React Native.
- [ ] Không import `vendor-shared` mới.

## Rủi ro

- Lệch hình dạng với host cài sau; sửa ở một file.
