# FE-CV-TASK-087-14: `QualityWaivePopover`: miễn trừ có hạn và bỏ miễn trừ

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.5
**Priority:** P1
**Area:** frontend / components + hook
**File:** `frontend/src/renderer/src/components/review-map/quality/findings/QualityWaivePopover.tsx`, `quality-waive-expiry-options.ts`; `hooks/useQualityWaive.ts` (mới) và `*.test.ts(x)`
**Depends on:** 087-12, 087-02
**Status:** [x] DONE (verified 2026-10-07: QualityWaivePopover.test.tsx + quality-waive-expiry-options.test.ts, 14/14 pass; oxlint + tsc sạch)

## Context

- Hợp đồng: `quality.waive {subjectKind:'finding', subjectKey, action, reason 1..1000, expiresAt ≤ 30 ngày}`; `CODEINTEL_WAIVER_EXPIRY_INVALID {maxDays}`; quyền `quality_waive`; `member` được miễn finding (không `check`).
- STYLEGUIDE: `Mod+Enter` qua `isScreenSubmitShortcut` (`lib/screen-submit-shortcut.ts`, đã dùng nhiều nơi), chip `ShortcutKeyCombo` (`components/ShortcutKeyCombo.tsx`); Cancel không destructive; khoá nút ngay (SSH).
- Miễn trừ không tự đổi cổng ở client.

## Việc cần làm

1. `quality-waive-expiry-options.ts`: 7/14/30 ngày + ngày cụ thể, kẹp ≤ 30 ngày (tính theo UTC, trả RFC 3339).
2. Popover: `reason` (đếm ký tự, bắt buộc), hạn bắt buộc; focus mặc định vào `reason`; Esc đóng; `Mod+Enter` gửi.
3. `useQualityWaive`: lạc quan (ẩn dòng/gắn `waiver`), hoàn nguyên khi lỗi (`forbidden`, `offline`, `conflict`, `validation`+`maxDays`), sau thành công `invalidateQuality` + `loadQualityGate`; toast chỉ xác nhận thoáng.
4. "Bỏ miễn trừ" (`action:'revoke'`) cho dòng đã miễn trừ khi bật "Hiện đã miễn trừ".

## Kiểm thử

Bắt buộc lý do/hạn; kẹp 30 ngày; lạc quan + hoàn nguyên; `Mod+Enter` theo `navigator.userAgent` giả (Mac/khác); revoke; chống bấm đúp. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/findings/QualityWaivePopover`.

## Tiêu chí hoàn thành

- [ ] Không có đường miễn trừ không lý do/hạn.
- [ ] Không hứa "Hoàn tác" ngoài revoke thật.

## Rủi ro

- `scope` mặc định của backend chưa rõ; không gửi.

## Ghi chú triển khai (2026-10-07)

- `useQualityWaive`: lạc quan + hoàn nguyên, khoá đồng bộ chống bấm đúp, revoke thật, không gửi `scope`. Toast chỉ xác nhận thoáng.
