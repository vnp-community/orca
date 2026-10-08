# FE-CV-TASK-095-06: Nối các sự kiện còn lại ở bề mặt Review

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/**`, `components/right-sidebar/use-source-control-quality-gate.ts` (085), menu (090), thẻ (093)
**Depends on:** FE-CV-TASK-095-03; FE-CV-SOL-051, 059, 087, 085, 090, 093 (các bề mặt)
**Status:** [~] PARTIAL — `review_opened` (nguồn qua `lib/review-open-source.ts`, mặc định `restore`), `review_lens_viewed` (≥ 2 s, một lần/lens/lần mở) và `review_findings_summary` đã nối (use-review-surface-telemetry.test, FindingsPanel.test, ReviewWorkspace.companions.test PASS); thiếu: `quality_finding_triaged` (chưa có UI triage cho QualityFinding — lens quality của W5-A) và các điểm vào chưa gọi `setReviewOpenSource` (W5-B)

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
