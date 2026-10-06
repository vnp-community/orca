# plan-phase-task: tasks backend (TASK-REQ-011-xx đến TASK-REQ-014-xx)

> **📋 Proposed.** Mọi task `[ ] TODO`, chưa triển khai. Tài liệu ngày 2026-10-06; đã đối chiếu code `task-service` thật; `request-service` chưa tồn tại nên các task của nó đọc lại code của CR-REQ-001, 002, 003, 006, 009 khi đã merge.

Mỗi task làm được trong 0,5 đến 2 ngày, có đường dẫn file, bước kiểm thử (tên test, lệnh `go test`) và checklist hoàn thành. Lệnh chạy từ `/opt/repos/orca/backend-go`; integration cần Docker (testcontainers, tag `integration`); chưa chạy lần nào. Quy ước: chạy `gitnexus_impact` trước khi sửa symbol có sẵn và `detect_changes` trước khi commit (CLAUDE.md); không `max-lines` disable; không đặt tên `helpers`/`utils`/`common`/`misc`.

## Bảng Solution → Task

| Solution | Task | Ghi chú |
|---|---|---|
| [BE-REQ-SOL-011](../solutions/BE-REQ-SOL-011-task-service-plan-phase-task-types.md) | 011-01 migration 0015/0016 · 011-02 domain + `request_id` + không số task · 011-03 `CreateTask` phân cấp · 011-04 `ListTasks` lọc · 011-05 `SyncContainerStatus` · 011-06 điểm gọi + đối soát · 011-07 chặn tác dụng phụ | Số migration thật 0015, 0016 (kiểm lại trước khi tạo). 03 và 04 song song sau 02 |
| [BE-REQ-SOL-012](../solutions/BE-REQ-SOL-012-plan-phase-task-generation-from-solution.md) | 012-01 `CreatePlanTree` usecase · 012-02 handler + integration · 012-03 `plan_shape` + validation · 012-04 proto + adapter AI/task · 012-05 `GeneratePlan`/`CommitPlan` · 012-06 `SubjectHandler` + single_task · 012-07 integration/e2e | 01, 02 ở task-service; 03 đến 07 ở request-service. 01 và 03 song song |
| [BE-REQ-SOL-013](../solutions/BE-REQ-SOL-013-phase-execution-and-feedback-loop.md) | 013-01 đổi cổng + outbox · 013-02 phát sự kiện `cause` · 013-03 migration + repo · 013-04 `StartPhase`/`AdvanceExecution` · 013-05 handler `phase` + guard + consumer · 013-06 `ReportTaskOutcome` · 013-07 đối soát + e2e | 01, 02 ở task-service (song song SOL-012); 03 đến 07 ở request-service |
| [BE-REQ-SOL-014](../solutions/BE-REQ-SOL-014-type-specific-execution-policies.md) | 014-01 `request_checks` · 014-02 RPC ghi/đọc · 014-03 `TypePolicy` + hook · 014-04 `pre_deploy` + hotfix/security · 014-05 performance/refactor · 014-06 ops_request · 014-07 follow-up hotfix + tích hợp | 05 và 06 song song sau 03 |

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
