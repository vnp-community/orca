# FE-CV-TASK-060-08: `ReviewTurnSwitcher`, lớp phủ lượt, i18n và e2e

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.5, 2.6, 6
**Priority:** P1
**Area:** frontend / renderer components + i18n + tests
**File:** `frontend/src/renderer/src/components/review-map/turns/ReviewTurnSwitcher.tsx` (mới) + test; `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (thêm `KEYS`); `tests/e2e/code-intel-web/review-notes.web.e2e.ts`
**Depends on:** FE-CV-TASK-060-04, 060-06, 060-07; FE-CV-SOL-052/053 (lớp phủ `review-overlay-model.ts`, lọc Thứ tự đọc); 073-02, 073-03
**Status:** [x] DONE (verified 2026-10-08)

## Context

- Ba chế độ; chế độ 3 chỉ đọc, không dựng lại diff chưa commit.

## Việc cần làm

1. `ReviewTurnSwitcher` (`ui/toggle-group` + `ui/popover`): danh sách lượt (agent, giờ, số ghi chú đã gửi); vô hiệu hoá chế độ 2–3 khi chưa có marker trước ("Chưa có lượt trước để so sánh").
2. Chế độ 2: truyền nhãn `compareTurns` vào lớp phủ (làm mờ `unchanged_since`, lọc Thứ tự đọc); chú giải đồng bộ.
3. Dịch khoá `auto.components.reviewMap.ReviewNote*|ReviewTurn*|ReviewSent*` sang 4 locale; thêm `KEYS`.
4. e2e (fake backend): soạn ghi chú ở ERD, gửi lô cho agent giả, thấy "Đã gửi ở lượt trước"; phát push agent xong lần hai ⇒ chế độ 2 có nhãn.

## Kiểm thử

- Component: vô hiệu khi chưa có marker; chế độ 3 không có nút diff giả; phủ khoá i18n.
- `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-locale-coverage src/renderer/src/components/review-map/turns`; e2e **chưa chạy**.

## Tiêu chí hoàn thành

- [ ] Tiêu chí 7–9 của SOL-060 mục 5.
- [ ] e2e xanh trên fake backend (khi 073-03 sẵn sàng).

## Rủi ro

- Chế độ 3 chỉ ở mức tệp/symbol; mở diff theo commit khi hai `headOid` khác chưa kiểm chứng.

## Ghi chú triển khai (2026-10-07)

- Chế độ 2–3 bị vô hiệu hoá khi chưa có marker trước; nếu đang ở chế độ đó mà marker biến mất thì tự về "all". Chế độ 3 chỉ đọc, không có nút diff.

## Ghi chú tích hợp (W6, 2026-10-07)

Phạm vi "lớp phủ" hiện chỉ là bộ lọc Thứ tự đọc (giao với bộ lọc chip). Chế độ "Xem lượt trước" vẫn chỉ hiện thông báo chỉ đọc.

## Ghi chú hoàn thiện (2026-10-08, P4)

- Nhãn lượt vào lớp phủ: `review-overlay-model.ts` thêm `turnOverlayLabel` (symbol trước, rồi tệp) và `turnOverlayDimmed` (`unchanged_since` làm mờ, không ẩn).
- `turns/review-turn-overlay-store.ts`: workspace (`use-review-companions.ts`) công bố so sánh "Từ lượt trước" theo worktree, dọn khi đổi chế độ/đóng tab; `ImpactGraphCanvas` đọc và gắn nhãn (`turnChangeLabel`) + `data-turn-label` lên `ImpactSymbolNode`.
- Chú giải: nhãn dùng cùng chuỗi `turnChangeLabel` với bộ đếm trong `ReviewTurnSwitcher`.
- Còn lại: e2e trên fake backend.
