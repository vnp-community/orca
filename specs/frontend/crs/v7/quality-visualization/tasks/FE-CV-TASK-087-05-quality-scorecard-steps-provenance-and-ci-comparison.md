# FE-CV-TASK-087-05: Scorecard cổng, lý do, bước chạy, dòng nguồn, so sánh CI, chip

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.5
**Priority:** P0
**Area:** frontend / components
**File:** `frontend/src/renderer/src/components/review-map/quality/QualityScorecard.tsx`, `QualityGateVerdictHeader.tsx`, `QualityGateReasonRow.tsx`, `QualityStepList.tsx`, `QualityProvenanceLine.tsx`, `QualityCiComparisonRow.tsx`, `QualityGateChip.tsx` (mới) và `*.test.tsx`
**Depends on:** FE-CV-TASK-088-03, 088-07; 087-03, 087-04
**Status:** [ ] TODO

## Context

- `ui/card`, `ui/collapsible`, `ui/tooltip` có sẵn; `GateVerdictBadge`, `StackedSeverityBar` từ 088.
- `QualityGate.reasons[]` có `code`, `params`, `category?`, `tool?`, `waivedCount?` (PQ-34); `QualityStep.status` gồm `failed|timeout|env_not_ready|skipped`; `QualityRun.dirty|workTreeChangedDuringRun|scopeWidened`; `CiComparison.relation` 9 giá trị (PQ-25).
- U9: chuỗi từ backend render văn bản thuần.

## Việc cần làm

1. `QualityScorecard` bố cục theo wireframe 2.5; `StackedSeverityBar` từ `summary` của run dựng cổng; nhiều `runIds` → nhiều dòng nguồn.
2. `QualityGateReasonRow`: nhãn từ `code`+`params` (bảng khoá i18n; mã lạ → `check`), `observed` so `threshold`, `result` biểu tượng+chữ, `waivedCount`; bấm → `setQualityUi({source:'quality', category|tool})` rồi mở dock (nếu không có `category`/`tool`: chỉ cuộn, ghi "Không lọc được").
3. `QualityStepList` (`Collapsible`): mỗi bước biểu tượng+chữ, `failureKind`, `envReason`, `durationMs`; `failed|timeout|env_not_ready` không hiển thị thành "0 phát hiện".
4. `QualityProvenanceLine`: Cục bộ/CI, `finishedAt`, HEAD, index; banner cũ + chữ "cũ" + nút "Chạy lại"; banner `dirty/workTreeChangedDuringRun/scopeWidened`; `outsideScopeCount`.
5. `QualityCiComparisonRow`: copy cho 9 `relation`; `local_pass_ci_fail` có `reasonsHint` và không dùng biểu tượng `pass`.
6. `QualityGateChip`: biểu tượng+chữ ngắn, `Tooltip` lý do đầu, bấm mở lens (deep link qua `openReviewFromEntryPoint` của CR-061, chưa có code).

## Kiểm thử

Ma trận verdict × reasons × stale × dirty; `failed` step; `unknown` không biểu tượng `pass`; chuỗi chứa HTML hiển thị như chữ; nhiều run nguồn; `local_pass_ci_fail`; bàn phím vào hàng lý do. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/QualityScorecard`.

## Tiêu chí hoàn thành

- [ ] Không dùng điểm số; không từ overclaim; không hex.
- [ ] Mọi trạng thái bằng biểu tượng + chữ.
- [ ] Chip chỉ hiện khi `enabled`.

## Rủi ro

- `observed/threshold` định dạng chưa nêu trong hợp đồng; render nguyên văn.
