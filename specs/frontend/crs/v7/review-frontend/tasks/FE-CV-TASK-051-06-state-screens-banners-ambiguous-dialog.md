# FE-CV-TASK-051-06: Màn trạng thái, banner và `AmbiguousSymbolDialog`

**From Solution:** [FE-CV-SOL-051-review-workspace-shell](../solutions/FE-CV-SOL-051-review-workspace-shell.md) mục 4.4
**Priority:** P0
**Area:** frontend / review-map
**File:** `ReviewViewStateScreen.tsx`, `ReviewStateBanners.tsx`, `AmbiguousSymbolDialog.tsx`, tests
**Depends on:** FE-CV-TASK-051-02, 051-03
**Status:** [x] DONE (verified 2026-10-07: ReviewViewStateScreen.test 14 screens + banners, AmbiguousSymbolDialog.test, ReviewWorkspace.test no-binding/offline cases)

## Context

- STYLEGUIDE "Empty and error states": persistent inline, có hành động; không toast; copy không overclaim.
- Lỗi theo `kind`; `ambiguous.candidates[≤10] {key?, uid, name, kind, filePath, line, score?, impactedCount?, risk?}`; `retryAfterSeconds` ở `rate-limited`.

## Việc cần làm

1. `ReviewViewStateScreen` cho từng màn của bảng 13 dòng (nút: "Lập chỉ mục", "Thử gắn lại" gọi `bindRepo` rồi `status`, "Thử lại", "Đóng tab", mở bộ chọn phạm vi); chi tiết lỗi sao chép được; `path-not-allowed` không hiện đường dẫn.
2. Banner: `stale` (từ `IndexFreshness`/`overall`), `truncated` `{shown}/{totalCount}`, `offline` có cache + "Thử lại" (tự thử lại khi kết nối `established`), "Có dữ liệu mới" (`applyNow`).
3. `AmbiguousSymbolDialog` (`ui/dialog` + `ui/command`): tìm kiếm, Enter chọn, chọn xong gọi lại với `key` hoặc `{name,file}`.

## Kiểm thử

- Mỗi màn/banner; không `toast`; tiêu điểm vào ô tìm; `Esc`; tự thử lại khi `established`.

## Tiêu chí hoàn thành

- [ ] Mọi lỗi có thân hoặc banner đúng, không toast.

## Rủi ro

- Liên kết hướng dẫn cài công cụ chưa có URL xác nhận: không bịa, chỉ nêu văn bản.
