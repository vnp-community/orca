# FE-CV-TASK-095-05: Nối tracker vào commit, tạo review, gửi ghi chú, đánh dấu đã xem

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P1
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (sửa vài dòng: sau `handleCommit` trả true; trong/sau `handlePullRequestCreated`), điểm gọi ở 060 và 052
**Depends on:** FE-CV-TASK-095-04
**Status:** [x] DONE (verified 2026-10-08: checks-panel-review-decision.test 3/3, review-surface-decision.test PASS, use-source-control-quality-gate.test + ChecksPanel.* PASS)

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

## Ghi chú hoàn thiện (2026-10-08, P4)

- `open_findings` chính thức: số lý do cổng đang `fail|warn` (`countOpenGateFindings` trong `lib/review-surface-decision.ts`); Source Control dùng `qualityGate.openFindingCount` (mới trong `use-source-control-quality-gate.ts`), bề mặt Review/Checks đọc cổng đã cache trong store (`codeIntelQualityByWorktree[w].gate`, đọc `verdict` theo hợp đồng, fallback `result`) — không thêm RPC.
- Điểm gọi ChecksPanel tách ra `components/right-sidebar/checks-panel-review-decision.ts` (`recordChecksPanelReviewCreated`, chỉ ghi khi `result.ok` và có worktree) để test riêng không cần render cả panel.
- Impact (best effort): `gitnexus impact` báo symbol không có trong index (index cũ); người gọi trực tiếp: SourceControl (1), ChecksPanel (1).
