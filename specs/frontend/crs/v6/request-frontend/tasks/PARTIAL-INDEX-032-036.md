# PARTIAL-INDEX: Tasks của CR-REQ-032 và CR-REQ-036 (frontend)

Phần này để người điều phối gộp vào `tasks/README.md`. Không phải README.

## Bảng Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| FE-REQ-SOL-032 | [FE-REQ-TASK-032-01](./FE-REQ-TASK-032-01-risk-tokens-risk-presentation-risk-badge.md) | Token rủi ro, `riskPresentation`, `RiskBadge`, STYLEGUIDE | P0 | không |
| | [FE-REQ-TASK-032-02](./FE-REQ-TASK-032-02-task-dag-view-token-migration.md) | `TaskDAGView` sang token, icon trạng thái, `colorMode` | P1 | 032-01, 018-06 |
| | [FE-REQ-TASK-032-03](./FE-REQ-TASK-032-03-graph-wire-types-parser-lens-registry-hook.md) | `GraphPayload`, parser, registry lens, `useGraphLens` | P0 | 018-01, 018-02, 018-03 |
| | [FE-REQ-TASK-032-04](./FE-REQ-TASK-032-04-graph-grouping-zoom-focus-and-client-lens-adapters.md) | Gom nhóm, thu phóng, tập trung, trước/sau, adapter lens client | P0 | 032-03, 021-02 |
| | [FE-REQ-TASK-032-05](./FE-REQ-TASK-032-05-graph-canvas-nodes-edges-mini-layout-engine.md) | `GraphCanvas`, node, nhóm, cạnh, `GraphMini`, `LayoutEngine` tầng | P0 | 032-01, 032-03, 032-04 |
| | [FE-REQ-TASK-032-06](./FE-REQ-TASK-032-06-graph-panel-toolbar-list-search-sheet.md) | `GraphPanel`, danh sách, tìm `cmdk`, sheet, trạng thái | P0 | 032-03 đến 032-05 |
| | [FE-REQ-TASK-032-07](./FE-REQ-TASK-032-07-elk-layout-engine-gated.md) | `elkjs` trong Worker (CHẶN bởi duyệt phụ thuộc, tuỳ chọn) | P2 | 032-05; quyết định duyệt |
| | [FE-REQ-TASK-032-08](./FE-REQ-TASK-032-08-graph-entry-points-i18n-e2e.md) | Điểm vào, i18n, e2e, tài liệu | P1 | 032-01, 032-05, 032-06, 019-03, 021-03 |
| FE-REQ-SOL-036 | [FE-REQ-TASK-036-01](./FE-REQ-TASK-036-01-request-artifact-types-parsers-rpc-constants.md) | Kiểu, parser, hằng kênh, `awaiting_information` | P0 | 018-01, 018-02 |
| | [FE-REQ-TASK-036-02](./FE-REQ-TASK-036-02-clarification-decision-impact-readiness-hooks.md) | Năm hook dữ liệu | P0 | 036-01, 018-03 |
| | [FE-REQ-TASK-036-03](./FE-REQ-TASK-036-03-clarification-panel.md) | `ClarificationPanel` | P0 | 036-01, 036-02, 019-01, 019-03 |
| | [FE-REQ-TASK-036-04](./FE-REQ-TASK-036-04-decision-bar-rationale-high-risk-confirm.md) | Lý do, xác nhận lần hai, lịch sử Decision | P0 | 036-01, 036-02, 020-04, 021-04 |
| | [FE-REQ-TASK-036-05](./FE-REQ-TASK-036-05-risk-summary-dimension-table-findings.md) | `RiskSummaryCard`, bảng chiều, phát hiện | P1 | 032-01, 032-05, 032-06, 036-02, 020-02, 020-03 |
| | [FE-REQ-TASK-036-06](./FE-REQ-TASK-036-06-risk-acceptance-and-approval-gating.md) | `RiskAcceptanceChecklist`, cổng duyệt theo mức, ghi đè | P1 | 036-04, 036-05 |
| | [FE-REQ-TASK-036-07](./FE-REQ-TASK-036-07-readiness-badge-report-plan-drift.md) | Readiness, báo cáo, dải lệch kế hoạch | P1 | 036-01 đến 036-03, 032-06, 021-03, 021-05 |
| | [FE-REQ-TASK-036-08](./FE-REQ-TASK-036-08-execution-result-panel-i18n-e2e.md) | Kết quả thực thi, i18n, e2e, tài liệu | P1 | 036-02 đến 036-07 |

## Sơ đồ thứ tự

```
018 ─▶ 032-03 ─▶ 032-04 ─┐
032-01 ──┬─▶ 032-02      ├─▶ 032-05 ─▶ 032-06 ─▶ 032-08
         └──────────────┘        └▶ (032-07, nếu được duyệt)
018 ─▶ 036-01 ─▶ 036-02 ─┬─▶ 036-03
                          ├─▶ 036-04 ─▶ 036-06
                          ├─▶ 036-05 ─▶ 036-06   (cần 032-01, 032-05, 032-06)
                          ├─▶ 036-07             (cần 032-06)
                          └─▶ 036-08 (sau 036-03 đến 036-07)
```

## Ghi chú

- Làm ngay không cần backend: 032-01, 032-02, 032-03, 032-04, 036-01 (các hàm thuần và token).
- 032-07 chặn bởi quyết định duyệt phụ thuộc (README v7 O5 cấm `elkjs`/`dagre` ở MVP); nếu không duyệt, đóng task bằng ghi chú, `package.json` không đổi.
- 036-02 đến 036-08 dùng kênh "(tạm)" của `clarification.*`, `decision.*`, `impact.*`, `readiness.*`, `execution.get`; đối chiếu `CONTRACT-request-ui-api.md` khi CR-REQ-028, 029, 030 có solution backend.
- 036-01 sửa `request-types.ts`, `request-flow-registry.ts`, `request-errors.ts` của FE-REQ-TASK-018-01/018-02: nếu 018 chưa merge, gộp vào cùng PR.
- 036-04, 036-06 sửa `SolutionDecisionBar`, `useSolutionDecision`, `PlanApprovalBar`, `PhaseApprovalBar` (020-04, 021-04); 036-07 sửa `PlanTaskRow`, `TaskDetail`: chỉ thêm điểm cắm.
- Mỗi task ghi lệnh `pnpm --filter orca-frontend test <đường dẫn>`; `frontend/package.json` chỉ có `build`, `dev`, `test`, `test:watch`.
