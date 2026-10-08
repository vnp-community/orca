# FE-CV-TASK-095-06: Nối các sự kiện còn lại ở bề mặt Review

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/**`, `components/right-sidebar/use-source-control-quality-gate.ts` (085), menu (090), thẻ (093)
**Depends on:** FE-CV-TASK-095-03; FE-CV-SOL-051, 059, 087, 085, 090, 093 (các bề mặt)
**Status:** [x] DONE (verified 2026-10-08: useQualityWaive.telemetry.test 2/2, open-review-entry.test 12/12, quality/findings PASS, use-review-surface-telemetry.test PASS)

## Context

- Mỗi bề mặt gọi hàm bọc, không `track()` trực tiếp.
- `quality_gate_viewed` khử trùng lặp (worktree, HEAD) trong phiên.
- `review_lens_viewed` chỉ khi ở lại ≥ 2 s, tối đa một lần/lens/lần mở.

## Việc cần làm

1. Gắn `review_opened`, `review_lens_viewed`, `quality_gate_viewed`, `quality_finding_triaged`, `review_findings_summary`, `review_report_exported`, `review_ai_summary`.
2. Ánh xạ lý do triage UI → enum; chuỗi tự do không đi vào.

## Kiểm thử

- Kịch bản tích hợp mock `telemetryTrack`: chuỗi sự kiện đúng thứ tự.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] ≤ 10 sự kiện/lượt.

## Rủi ro

- Bề mặt chưa tồn tại; làm khi từng solution xong.

## Ghi chú triển khai (2026-10-07)

Đã nối: `quality_gate_viewed` (hook cổng, khử trùng lặp theo worktree+HEAD), `review_report_exported` (menu + nút chèn), `review_ai_summary` (hook AI). THIẾU: `review_opened`, `review_lens_viewed`, `review_findings_summary`, `quality_finding_triaged` (bề mặt 051/059/087 chưa nối; wrapper đã sẵn).

## Ghi chú tích hợp (W6, 2026-10-07)

Điểm vào (agent row, Source Control, Cmd+K, right sidebar, notification) cần gọi `setReviewOpenSource(worktreeId, {source, afterAgentTurn})` ngay trước khi mở tab; chưa gọi thì sự kiện tính là `restore`.

## Ghi chú hoàn thiện (2026-10-08, P4)

- `quality_finding_triaged`: phát trong `hooks/useQualityWaive.ts` sau khi waive/revoke thành công ở dock "Kiểm tra" (`QualityWaivePopover`); `reason` luôn `other` (lý do waiver là chữ tự do, không gửi), `severity` lạ ⇒ `info`, `blocking` = `error` trong phạm vi; lỗi ⇒ không phát. Finding cấu trúc (`FindingsPanel`) không phát sự kiện này (khác nguồn, PQ-05/06).
- `setReviewOpenSource`: gọi trong `entry/open-review-entry.ts` (một điểm cho agent row, Source Control, Cmd+K, right sidebar, notification; `afterAgentTurn` = có lượt agent chưa xem; huỷ nếu tab không mở được) và nút lý do cổng ở Source Control (`use-source-control-quality-gate.ts`).
