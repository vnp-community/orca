# FE-CV-TASK-095-05: Nối tracker vào commit, tạo review, gửi ghi chú, đánh dấu đã xem

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (sửa vài dòng: sau `handleCommit` trả true; trong/sau `handlePullRequestCreated`), điểm gọi ở 060 và 052
**Depends on:** FE-CV-TASK-095-04
**Status:** [~] PARTIAL — `send_to_agent` (ReviewNotesSendMenu), `mark_reviewed` (khi mọi bước Thứ tự đọc đã xem) và `create_review` từ ChecksPanel đã nối qua `lib/review-surface-decision.ts` (review-surface-decision.test, ReviewNotesSendMenu.test, ReviewWorkspace.companions.test PASS); `open_findings` vẫn tạm = số lý do của cổng (ở Review luôn 0, gate `unknown`/`none`); điểm gọi ChecksPanel chưa có test riêng (chỉ typecheck + ChecksPanel.* test cũ PASS)

## Context

- `handleCommit` :1792 trả boolean; `handlePullRequestCreated` :2488 được gọi ở 3078, 3116, 3295, 3318.

## Việc cần làm

1. Chạy GitNexus `impact` và báo blast radius (chưa chạy).
2. Mỗi điểm một dòng gọi `decide(...)`; không đổi hành vi.

## Kiểm thử

- Test hiện có `SourceControl.*.test` xanh; test mới giả `decide`.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Hành vi commit/PR không đổi.

## Rủi ro

- SourceControl.tsx rất lớn.

## Ghi chú triển khai (2026-10-07)

Nối `commit` (sau handleCommit thành công) và `create_review` (handlePullRequestCreated) trong SourceControl qua `useReviewDecisionTelemetry`; `noteReviewOpened` trong `ensureReviewTab`. THIẾU: `send_to_agent` (SOL-060), `mark_reviewed` (SOL-052), `create_review` từ ChecksPanel; `open_findings` tạm = số lý do của cổng. Impact handleCommit: UNKNOWN (index cũ), 5 phụ thuộc trực tiếp.

## Ghi chú tích hợp (W6, 2026-10-07)

Không dùng `useReviewDecisionTelemetry` ở Review vì hook đó đăng ký thêm một subscription `registerCompletion` (đăng ký hai lần sẽ báo lượt trước là `abandon`). `mark_reviewed` = chuyển trạng thái "đã xem hết" của Thứ tự đọc. ChecksPanel.tsx sửa tối thiểu 1 chỗ sau `result.ok`.
