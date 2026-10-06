# FE-CV-TASK-095-06: Nối các sự kiện còn lại ở bề mặt Review

**From Solution:** [FE-CV-SOL-095-review-telemetry](../solutions/FE-CV-SOL-095-review-telemetry.md) mục 2.4
**Priority:** P2
**Area:** frontend
**File:** `frontend/src/renderer/src/components/review-map/**`, `components/right-sidebar/use-source-control-quality-gate.ts` (085), menu (090), thẻ (093)
**Depends on:** FE-CV-TASK-095-03; FE-CV-SOL-051, 059, 087, 085, 090, 093 (các bề mặt)
**Status:** [ ] TODO

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
