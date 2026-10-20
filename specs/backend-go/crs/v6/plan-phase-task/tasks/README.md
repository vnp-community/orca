# plan-phase-task: tasks backend (TASK-REQ-011-xx đến TASK-REQ-014-xx)

> **🚧 Đang triển khai.** SOL-011 7/7 DONE; SOL-012 01, 02 DONE (task-service); SOL-013 7/7 DONE (kiểm chứng 2026-10-08); SOL-014 01, 02, 07 DONE, 03 đến 06 một phần (còn hook `PlanPreconditions` ở `GeneratePlan`/`CommitPlan`, CR-REQ-012). Chi tiết: [IMPLEMENTATION-NOTES](../IMPLEMENTATION-NOTES.md).

Mỗi task làm được trong 0,5 đến 2 ngày, có đường dẫn file, bước kiểm thử (tên test, lệnh `go test`) và checklist hoàn thành. Lệnh chạy từ `/opt/repos/orca/backend-go`; integration cần Docker (testcontainers, tag `integration`); chưa chạy lần nào. Quy ước: chạy `gitnexus_impact` trước khi sửa symbol có sẵn và `detect_changes` trước khi commit (CLAUDE.md); không `max-lines` disable; không đặt tên `helpers`/`utils`/`common`/`misc`.

## Bảng Solution → Task

| Solution | Task | Ghi chú |
|---|---|---|
| [BE-REQ-SOL-011](../solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md) ✅ 7/7 DONE | 011-01 migration 0015/0016 · 011-02 domain + `request_id` + không số task · 011-03 `CreateTask` phân cấp · 011-04 `ListTasks` lọc · 011-05 `SyncContainerStatus` · 011-06 điểm gọi + đối soát · 011-07 chặn tác dụng phụ | Số migration thật 0015, 0016 (kiểm lại trước khi tạo). 03 và 04 song song sau 02 |
| [BE-REQ-SOL-012](../solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md) 🚧 2/7 DONE | 012-01 `CreatePlanTree` usecase · 012-02 handler + integration · 012-03 `plan_shape` + validation · 012-04 proto + adapter AI/task · 012-05 `GeneratePlan`/`CommitPlan` · 012-06 `SubjectHandler` + single_task · 012-07 integration/e2e | 01, 02 ở task-service; 03 đến 07 ở request-service. 01 và 03 song song |
| [BE-REQ-SOL-013](../solutions/BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) ✅ 7/7 DONE | 013-01 đổi cổng + outbox · 013-02 phát sự kiện `cause` · 013-03 migration + repo · 013-04 `StartPhase`/`AdvanceExecution` · 013-05 handler `phase` + guard + consumer · 013-06 `ReportTaskOutcome` · 013-07 đối soát + e2e | 01, 02 ở task-service (song song SOL-012); 03 đến 07 ở request-service |
| [BE-REQ-SOL-014](../solutions/BE-REQ-SOL-014-type-specific-execution-policies.md) 🚧 3/7 DONE (03 đến 06 một phần) | 014-01 `request_checks` · 014-02 RPC ghi/đọc · 014-03 `TypePolicy` + hook · 014-04 `pre_deploy` + hotfix/security · 014-05 performance/refactor · 014-06 ops_request · 014-07 follow-up hotfix + tích hợp | 05 và 06 song song sau 03 |
| [BE-REQ-SOL-038](../solutions/BE-REQ-SOL-038-continuous-execution-activity-feed-and-execution-summary.md) ✅ 8/8 DONE (2026-10-16) | 038-01 migration + repository · 038-02 `ExecutionMode` + cài đặt dự án · 038-03 thực thi liên tục + kiểm quyền người duyệt · 038-04 `engine`/`execution_mode` trên Request + `open_spec_profile` · 038-05 `ListRequestActivity` · 038-06 `GetExecutionSummary` + nguyên nhân lỗi · 038-07 kênh gateway + MCP parity + CONTRACT · 038-08 tích hợp, hợp đồng hai dialect, e2e | 02 và 05 song song sau 01; 04 song song sau 02; 06 cần 04; 07 cần 05 và 06; 03 cần 01 và 02 |
| [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md) 📝 0/6 | 039-01 migration `0099` và repository · 039-02 miền, tạo/sửa, khoá · 039-03 worktree đề xuất và kiểm ref · 039-04 task-service và cổng thực thi · 039-05 gateway, MCP, CONTRACT, cờ tính năng · 039-06 tích hợp và e2e trên repo fixture | 03 và 04 song song sau 02; 05 cần 02; 06 cần 03, 04, 05 |
| [BE-REQ-SOL-043](../solutions/BE-REQ-SOL-043-approve-plan-and-run-all-steps.md) ✅ 6/6 | 043-01 Miền `ApprovalExecutionMode`, hai mã lỗi mới, trường proto `execution_mode` · 043-02 `DecideApproval` ghim chế độ trong cùng transaction, payload và audit · 043-03 `ExecutionStartPrecheck` kiểm quyền thực thi theo chế độ được yêu cầu · 043-04 Kênh `approval.approve`, route REST, CLI và cờ tính năng `approval.runAll` · 043-05 Test hợp đồng hai dialect và luồng nhiều Phase sau một lần duyệt · 043-06 Suite acceptance `ARA-xx`, mở rộng `claude` giả cho prompt thực thi, cập nhật tài liệu phát hành |
| [BE-REQ-SOL-044](../solutions/BE-REQ-SOL-044-automatic-plan-generation.md) ✅ 9/9 | 044-01 Miền `plan_auto`: trạng thái job, khoá idempotency, mã lỗi tạm thời, chính sách từ env và luật nhận job · 044-02 Migration `0101_plan_auto_generation`, repository `plan_auto_jobs` và cột `plan_auto_generate`, hai dialect · 044-03 `GeneratePlan.ExecuteForSystem` với khoá idempotency, `CommitPlan.ExecuteForSystem` và commit idempotent khi có cuộc đua · 044-04 Công tắc theo dự án `plan_auto_generate` (usecase, proto, gRPC) và `PlanAutoSwitch` · 044-05 `EnqueuePlanAuto` và consumer bền `request-service-plan-auto-generate` · 044-06 `PlanAutoWorker` (bắt đầu, quan sát, commit, thử lại có giới hạn) và wiring `wire_plan_auto.go` · 044-07 `RetryPlanGeneration`, khối `auto_plan` của `GetPlanProposal`, proto `plan.proto` và gRPC · 044-08 Gateway (`request.retryPlan`, `autoPlan`, `planAutoGenerate`, `request.planAuto`), loại trừ MCP, authz, cờ CLI và CONTRACT · 044-09 Tích hợp, hợp đồng và e2e (`APG-xx`) cho tự động sinh Plan |
| [BE-REQ-SOL-045](../solutions/BE-REQ-SOL-045-plan-documents-and-revision.md) ✅ 9/9 | 045-01 `AuthorizeRevisePlan`: approver của cổng Plan được chỉnh, và cửa trước OPA cho `GeneratePlan` · 045-02 Đường chỉnh Plan chắc hơn: khoá idempotency, lần chỉnh thứ hai, kế thừa phản hồi, hotfix, dev server của reporter, job commit `revision` · 045-03 Làm sạch bí mật của bộ tài liệu khi lưu và khi đọc; kích thước thật của file bị lược · 045-04 Bảng lịch sử phiên bản của OpenSpec change, ghi cùng transaction với lượt sinh · 045-05 `ListPlanRevisions` và đọc theo phiên bản (`GetPlanProposal.revision`, `GetPlanBundleFile.revision`) · 045-06 Prompt Plan: sửa tại chỗ (OpenSpec) và Plan hiện hành (native), kèm quy trình golden · 045-07 Activity `plan.revision_requested` và sửa nhánh `analysis.revision_requested` cho khỏi bắt nhầm lượt Plan · 045-08 Gateway, REST, CLI, loại trừ MCP và CONTRACT cho xem tài liệu và chỉnh Plan · 045-09 Tích hợp, hợp đồng và e2e (suite `PRV-xx`) với claude giả, dữ liệu seed và ca OpenSpec thật tuỳ chọn |

Sơ đồ phụ thuộc SOL-038: `038-01 ─▶ 038-02 ─┬─▶ 038-03 ─┐` · `038-02 ─▶ 038-04 ─▶ 038-06` · `038-01 ─▶ 038-05` · `038-05, 038-06 ─▶ 038-07` · `038-03, 05, 06, 07 ─▶ 038-08`. 038-03 phải chạy `gitnexus_impact` cho `StartExecution.Execute`, `EvaluateExecution.completePhases`, `StartPhase.Execute` (blast radius cao).

## Sơ đồ phụ thuộc

```
011-01 ─▶ 011-02 ─┬─▶ 011-03 ─────────────────────────────┐
                  ├─▶ 011-04                               │
                  └─▶ 011-05 ─▶ 011-06                     ▼
                         └──────────▶ 011-07 ◀───────── 011-03
                                         │
        ┌────────────────────────────────┘ (011-01,03,07)
        ▼
   012-01 ─▶ 012-02                       012-03 ─▶ 012-04 ─▶ 012-05 ─▶ 012-06 ─▶ 012-07
        └──────────────(proto CreatePlanTree)──▶ 012-04

   011-05,06 ─▶ 013-01 ─▶ 013-02                (nửa task-service, song song 012)
   012-05 + 013-03 ─▶ 013-04 ─┬─▶ 013-05
                              └─▶ 013-06 ─▶ 013-07   (013-02 cung cấp payload thật cho 013-06)

   012-05, 013-04, 013-06 ─▶ 014-03 ◀─ 014-01 ─▶ 014-02
   014-03 ─┬─▶ 014-04 ─┐
           ├─▶ 014-05  ├─▶ 014-07
           └─▶ 014-06 ─┘
```

## Quyết định chung

- Task migration (011-01, 013-03, 014-01) ghi số thật khi tạo file; `task-service` là 0015/0016, `request-service` theo quy tắc "lớn nhất cộng 1", hai dialect cùng số.
- Task đổi chữ ký cổng (011-04, 013-01) phải cập nhật mọi fake và hai adapter trong cùng PR để biên dịch.
- Task `request-service` dùng tên cổng theo CR-REQ-001 đến 009; khi chữ ký thật khác, đổi theo code đã merge và ghi vào PR.

## Điểm cần người điều phối chú ý

- Xung đột số migration trong `request-service` giữa các CR khác (nhiều CR cùng dùng `0002`, `0005`).
- Giả định lớn nhất chưa kiểm chứng: một worktree dùng chung cho mọi task của Plan (013-04).
- Quyền ghi và xem ở mức Request chưa chốt (012-05, 014-02, 015-06).
