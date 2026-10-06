# Tasks: request-frontend (frontend, v6)

> 📋 Proposed. Chưa triển khai. Mỗi task làm được trong 0,5 đến 2 ngày. Solution gốc ở [../solutions/README.md](../solutions/README.md). Lệnh test, typecheck, e2e chung: xem mục cuối README solutions (chưa kiểm chứng các lệnh gốc repo).

## Bảng Solution → Task

| Task | Mô tả | Priority | Phụ thuộc | Status |
|---|---|---|---|---|
| [FE-REQ-TASK-018-01](./FE-REQ-TASK-018-01-shared-request-types-registry-parsers.md) | Kiểu, registry luồng, hằng kênh, lỗi, parser (`shared/request-*.ts`) | P0 | không | [ ] TODO |
| [FE-REQ-TASK-018-02](./FE-REQ-TASK-018-02-request-rpc-client-and-event-bus.md) | `callRequestRpc`, `subscribeRequestEvents`, event bus | P0 | 018-01 | [ ] TODO |
| [FE-REQ-TASK-018-03](./FE-REQ-TASK-018-03-request-hooks.md) | 8 hook Request/Solution/Approval/Backlog/sự kiện | P0 | 018-01, 018-02, 018-04 | [ ] TODO |
| [FE-REQ-TASK-018-04](./FE-REQ-TASK-018-04-request-store-slice-and-routing.md) | Slice `request`, `TopLevelView` `requests`, `openRequestPage` | P0 | 018-01 | [ ] TODO |
| [FE-REQ-TASK-018-05](./FE-REQ-TASK-018-05-request-page-shell-sidebar-badges.md) | `RequestPage`, nút sidebar, badge, i18n nền, tài liệu trang | P0 | 018-03, 018-04 | [ ] TODO |
| [FE-REQ-TASK-018-06](./FE-REQ-TASK-018-06-remove-backlog-task-status.md) | Gỡ `backlog` khỏi `TaskStatus`, `normalizeTaskStatus`, `plan\|phase` | P0 | không | [ ] TODO |
| [FE-REQ-TASK-019-01](./FE-REQ-TASK-019-01-stage-timeline-model-and-child-rules.md) | Mô hình dòng thời gian, luật Request con, `isLowConfidence` | P0 | 018-01 | [ ] TODO |
| [FE-REQ-TASK-019-02](./FE-REQ-TASK-019-02-requests-tab-list-filters-keyboard.md) | `RequestsTab`: danh sách, lọc, trạng thái, phím điều hướng | P0 | 018-03, 018-05, 019-01 | [ ] TODO |
| [FE-REQ-TASK-019-03](./FE-REQ-TASK-019-03-request-detail-pane-header-actions.md) | `RequestDetailPane`, header, hủy, trả về backlog, mở lại | P0 | 018-03, 018-05, 019-01 | [ ] TODO |
| [FE-REQ-TASK-019-04](./FE-REQ-TASK-019-04-type-confirmation-card-and-history.md) | `TypeConfirmationCard`, đổi loại, lịch sử | P0 | 019-03 | [ ] TODO |
| [FE-REQ-TASK-019-05](./FE-REQ-TASK-019-05-related-requests-and-spawn-child.md) | `RequestRelatedTab`, tạo Request con | P1 | 019-03, 019-01 | [ ] TODO |
| [FE-REQ-TASK-019-06](./FE-REQ-TASK-019-06-create-request-dialog-and-tasks-page-buttons.md) | `CreateRequestDialog`, nút "Tạo Request" trên trang Tasks | P0 | 018-03, 018-05 | [ ] TODO |
| [FE-REQ-TASK-019-07](./FE-REQ-TASK-019-07-request-list-detail-i18n-and-e2e.md) | i18n, test phủ khoá, e2e danh sách/chi tiết | P1 | 019-02 đến 019-06 | [ ] TODO |
| [FE-REQ-TASK-020-01](./FE-REQ-TASK-020-01-solution-view-model.md) | Mô hình hiển thị Solution (hàm thuần) | P0 | 018-01 | [ ] TODO |
| [FE-REQ-TASK-020-02](./FE-REQ-TASK-020-02-solution-panel-body-views.md) | `SolutionPanel`, banner, Chẩn đoán/Findings/Answer, Markdown an toàn | P0 | 020-01, 019-03, 018-03 | [ ] TODO |
| [FE-REQ-TASK-020-03](./FE-REQ-TASK-020-03-solution-option-compare.md) | Thẻ phương án và bảng so sánh | P0 | 020-01, 020-02 | [ ] TODO |
| [FE-REQ-TASK-020-04](./FE-REQ-TASK-020-04-reject-dialog-decision-bar.md) | `RejectReasonDialog`, `SolutionDecisionBar`, chuỗi chọn rồi duyệt | P0 | 020-01 đến 020-03, 018-03 | [ ] TODO |
| [FE-REQ-TASK-020-05](./FE-REQ-TASK-020-05-solution-review-i18n-and-e2e.md) | i18n, test phủ khoá, e2e duyệt Solution | P1 | 020-02 đến 020-04 | [ ] TODO |
| [FE-REQ-TASK-021-01](./FE-REQ-TASK-021-01-planning-task-filter-and-effective-parent.md) | Lọc `plan`/`phase` khỏi Board/cây/DAG, `parentId` hiệu dụng | P0 | 018-06 | [ ] TODO |
| [FE-REQ-TASK-021-02](./FE-REQ-TASK-021-02-use-plan-tree-hook.md) | `usePlanTree`, `buildPlanSubtree`, ghép Approval | P0 | 021-01, 018-03 | [ ] TODO |
| [FE-REQ-TASK-021-03](./FE-REQ-TASK-021-03-plan-tree-components.md) | `PlanTree`, `PhaseNode`, `PlanTaskRow`, tiêu đề tổng | P0 | 021-02, 018-05 | [ ] TODO |
| [FE-REQ-TASK-021-04](./FE-REQ-TASK-021-04-plan-phase-approval-bars.md) | Duyệt Plan/Phase/`pre_deploy`, bắt đầu Phase | P0 | 021-02, 021-03, 020-04 | [ ] TODO |
| [FE-REQ-TASK-021-05](./FE-REQ-TASK-021-05-request-plan-tab-states-and-execution-gate.md) | `RequestPlanTab`, trạng thái, khoá Chạy theo Phase | P0 | 021-03, 021-04, 018-04 | [ ] TODO |
| [FE-REQ-TASK-021-06](./FE-REQ-TASK-021-06-plan-tree-i18n-and-e2e.md) | i18n, test phủ khoá, e2e cây Plan | P1 | 021-01 đến 021-05 | [ ] TODO |

CR-REQ-022, 023, 032, 036: xem [PARTIAL-INDEX-022-023.md](./PARTIAL-INDEX-022-023.md) và [PARTIAL-INDEX-032-036.md](./PARTIAL-INDEX-032-036.md) (bảng Solution → Task và thứ tự phụ thuộc).

## Thứ tự phụ thuộc

```
018-01 ─┬─▶ 018-02 ─▶ 018-03 ─▶ 018-05 ─┬─▶ 019-02 ─┐
        ├─▶ 018-04 ──────────▲          ├─▶ 019-03 ─┼─▶ 019-04, 019-05 ─▶ 019-07
        ├─▶ 019-01 ─────────────────────┤           │
        │                               └─▶ 019-06 ─┘
        └─▶ 020-01 ─▶ 020-02 ─▶ 020-03 ─▶ 020-04 ─▶ 020-05      (020-02 cần 019-03)
018-06 ─▶ 021-01 ─▶ 021-02 ─▶ 021-03 ─▶ 021-04 ─▶ 021-05 ─▶ 021-06   (021-04 cần 020-04)
```

Làm được ngay, không cần backend: 018-01, 018-04, 018-06, 019-01, 020-01, 021-01. Còn lại cần hoặc mock hợp đồng CR-REQ-016 hoặc backend thật.

## Ghi chú

- Mọi task nêu file cụ thể, kênh WS, trạng thái rỗng/lỗi/tải, phím tắt đa nền tảng, khoá i18n, test và lệnh `pnpm`.
- Đề xuất đợt PR: (1) 018-01..06; (2) 019-01..07; (3) 020-01..05; (4) 021-01..06. 018-06 và 021-01 chạm Board/cây Task hiện có nên tách PR riêng để dễ hoàn nguyên.
- Trước khi sửa symbol có sẵn (`useTasks`, `TaskTreeView`, `TaskDetail`, `TaskStatusBadge`, `openTaskPage`, `SidebarNav`, `JiraIssueRow`): chạy GitNexus `impact`, ghi kết quả vào PR (chưa chạy khi soạn).
- Khi `CONTRACT-request-ui-api.md` của backend ra: đối chiếu các chỗ "tạm" (schema `options`, `Approval`, `links`, `planTaskId`, `viewerCan`, `eventType`, nguồn dữ liệu cây Plan).

## Danh sách task của CR-REQ-022, 023, 032, 036 (đầy đủ)

| Task | Tiêu đề |
|---|---|
| [FE-REQ-TASK-022-01-approval-inbox-rules](./FE-REQ-TASK-022-01-approval-inbox-rules.md) | Quy tắc thuần của hộp duyệt (sắp xếp, duyệt nhanh, nhóm lọc, đích mở) |
| [FE-REQ-TASK-022-02-use-approval-inbox-hook](./FE-REQ-TASK-022-02-use-approval-inbox-hook.md) | Hook `useApprovalInbox` và phân loại kết cục lỗi quyết định |
| [FE-REQ-TASK-022-03-request-summaries-minute-clock-relative-time](./FE-REQ-TASK-022-03-request-summaries-minute-clock-relative-time.md) | `useRequestSummaries`, `useMinuteClock` và định dạng thời gian tương đối |
| [FE-REQ-TASK-022-04-row-list-keyboard-navigation](./FE-REQ-TASK-022-04-row-list-keyboard-navigation.md) | Hook điều hướng bằng phím `j`/`k`/`Enter` cho danh sách hàng |
| [FE-REQ-TASK-022-05-approval-row-list-toolbar-states](./FE-REQ-TASK-022-05-approval-row-list-toolbar-states.md) | `ApprovalRow`, `ApprovalList`, thanh lọc và các trạng thái rỗng/tải/lỗi |
| [FE-REQ-TASK-022-06-approval-inbox-tab-wiring-and-count](./FE-REQ-TASK-022-06-approval-inbox-tab-wiring-and-count.md) | `ApprovalInboxTab`: lắp ghép, xác nhận, từ chối, điều hướng, chấm số |
| [FE-REQ-TASK-022-07-approval-inbox-i18n-and-docs](./FE-REQ-TASK-022-07-approval-inbox-i18n-and-docs.md) | i18n năm locale, test phủ khoá và tài liệu trang cho hộp duyệt |
| [FE-REQ-TASK-023-01-backlog-wire-types-and-parsers](./FE-REQ-TASK-023-01-backlog-wire-types-and-parsers.md) | Kiểu và parser cho ba view backlog |
| [FE-REQ-TASK-023-02-use-backlog-hook](./FE-REQ-TASK-023-02-use-backlog-hook.md) | Hook `useBacklog` ba view (phân trang, sự kiện, polling, lỗi riêng từng view) |
| [FE-REQ-TASK-023-03-reopen-and-cancel-request-dialogs](./FE-REQ-TASK-023-03-reopen-and-cancel-request-dialogs.md) | `ReopenRequestDialog` và `CancelRequestDialog` |
| [FE-REQ-TASK-023-04-backlog-tab-shell-segments-and-states](./FE-REQ-TASK-023-04-backlog-tab-shell-segments-and-states.md) | Khung `BacklogTab`: phân đoạn, thanh công cụ, phím `1/2/3`, trạng thái chung |
| [FE-REQ-TASK-023-05-request-backlog-table](./FE-REQ-TASK-023-05-request-backlog-table.md) | `RequestBacklogTable` và hàng Request bị trả về (Mở lại, Hủy, điều hướng) |
| [FE-REQ-TASK-023-06-task-and-execute-backlog-tables](./FE-REQ-TASK-023-06-task-and-execute-backlog-tables.md) | `TaskBacklogTable` và `ExecuteBacklogTable` (chỉ đọc, nhóm theo Plan/Phase) |
| [FE-REQ-TASK-023-07-board-hint-i18n-and-docs](./FE-REQ-TASK-023-07-board-hint-i18n-and-docs.md) | Gợi ý trên Board, i18n năm locale, test phủ khoá và tài liệu trang Backlog |
| [FE-REQ-TASK-032-01-risk-tokens-risk-presentation-risk-badge](./FE-REQ-TASK-032-01-risk-tokens-risk-presentation-risk-badge.md) | Token màu rủi ro, `riskPresentation` và `RiskBadge` |
| [FE-REQ-TASK-032-02-task-dag-view-token-migration](./FE-REQ-TASK-032-02-task-dag-view-token-migration.md) | Chuyển `TaskDAGView` sang token màu và icon trạng thái |
| [FE-REQ-TASK-032-03-graph-wire-types-parser-lens-registry-hook](./FE-REQ-TASK-032-03-graph-wire-types-parser-lens-registry-hook.md) | Kiểu `GraphPayload`, parser, registry lens và hook `useGraphLens` |
| [FE-REQ-TASK-032-04-graph-grouping-zoom-focus-and-client-lens-adapters](./FE-REQ-TASK-032-04-graph-grouping-zoom-focus-and-client-lens-adapters.md) | Hàm thuần gom nhóm, thu phóng, tập trung, trước/sau và adapter lens phía client |
| [FE-REQ-TASK-032-05-graph-canvas-nodes-edges-mini-layout-engine](./FE-REQ-TASK-032-05-graph-canvas-nodes-edges-mini-layout-engine.md) | `GraphCanvas`, node, nhóm, cạnh, `GraphMini` và `LayoutEngine` bố cục tầng |
| [FE-REQ-TASK-032-06-graph-panel-toolbar-list-search-sheet](./FE-REQ-TASK-032-06-graph-panel-toolbar-list-search-sheet.md) | `GraphPanel`, thanh công cụ, danh sách, tìm kiếm `cmdk`, `GraphNodeSheet` và các trạng thái |
| [FE-REQ-TASK-032-07-elk-layout-engine-gated](./FE-REQ-TASK-032-07-elk-layout-engine-gated.md) | Bố cục `elkjs` trong Worker (CHẶN bởi quyết định duyệt phụ thuộc) |
| [FE-REQ-TASK-032-08-graph-entry-points-i18n-e2e](./FE-REQ-TASK-032-08-graph-entry-points-i18n-e2e.md) | Điểm vào đồ thị, i18n 5 locale, e2e và tài liệu |
| [FE-REQ-TASK-036-01-request-artifact-types-parsers-rpc-constants](./FE-REQ-TASK-036-01-request-artifact-types-parsers-rpc-constants.md) | Kiểu và parser Clarification, Decision, Impact, Readiness, ExecutionResult; hằng kênh; `awaiting_information` |
| [FE-REQ-TASK-036-02-clarification-decision-impact-readiness-hooks](./FE-REQ-TASK-036-02-clarification-decision-impact-readiness-hooks.md) | Hook `useClarifications`, `useDecisions`, `useImpactAssessment`, `useTaskReadiness`, `useExecutionResult` |
| [FE-REQ-TASK-036-03-clarification-panel](./FE-REQ-TASK-036-03-clarification-panel.md) | `ClarificationPanel`, danh sách câu hỏi và kiểm hợp lệ trả lời |
| [FE-REQ-TASK-036-04-decision-bar-rationale-high-risk-confirm](./FE-REQ-TASK-036-04-decision-bar-rationale-high-risk-confirm.md) | Decision: lý do khi chọn khác đề xuất, xác nhận lần hai gõ tên, lịch sử chọn |
| [FE-REQ-TASK-036-05-risk-summary-dimension-table-findings](./FE-REQ-TASK-036-05-risk-summary-dimension-table-findings.md) | `RiskSummaryCard`, `SolutionDimensionTable`, `ImpactFindingList` và `ImpactEvidenceSheet` |
| [FE-REQ-TASK-036-06-risk-acceptance-and-approval-gating](./FE-REQ-TASK-036-06-risk-acceptance-and-approval-gating.md) | `RiskAcceptanceChecklist`, cổng duyệt theo mức rủi ro và ghi đè cổng |
| [FE-REQ-TASK-036-07-readiness-badge-report-plan-drift](./FE-REQ-TASK-036-07-readiness-badge-report-plan-drift.md) | `ReadinessBadge`, `ReadinessReportSheet`, `PlanDriftBanner` và `PlanDriftReviewSheet` |
| [FE-REQ-TASK-036-08-execution-result-panel-i18n-e2e](./FE-REQ-TASK-036-08-execution-result-panel-i18n-e2e.md) | `ExecutionResultPanel`, i18n 5 locale, e2e và tài liệu |
