# request-quality-rollout: tasks (backend-go, v6)

> 📋 Proposed. Mọi task `Status: [ ] TODO`; chưa task nào được làm hay chạy. Đã đối chiếu với file thật của `issue-status-sync`, `common`, `deploy`, `ci`, workflows; phần thuộc `request-service` dựa trên CR vì service chưa có.

## Solution → Task

| Solution | Task | Tên | Priority | Phụ thuộc |
|---|---|---|---|---|
| [BE-REQ-SOL-024](../solutions/BE-REQ-SOL-024-jira-status-sync-audit-observability.md) | [TASK-REQ-024-01](./TASK-REQ-024-01-auditclient-append-detailed.md) | `auditclient.AppendDetailed` | P1 | không |
| | [TASK-REQ-024-02](./TASK-REQ-024-02-request-service-audit-recorder.md) | `AuditRecorder` và audit mọi quyết định Request | P1 | 024-01, CR-004..010, 025-01 |
| | [TASK-REQ-024-03](./TASK-REQ-024-03-request-sync-state-migration-repository.md) | Migration `0002` và repository `request_sync_state` | P1 | không |
| | [TASK-REQ-024-04](./TASK-REQ-024-04-handle-request-status-usecase.md) | `HandleRequestStatus`, subscription `REQUEST` | P1 | 024-03, CR-003 |
| | [TASK-REQ-024-05](./TASK-REQ-024-05-lookup-request-by-source-rpc.md) | RPC `LookupRequestBySource` | P1 | CR-002, 004, 025-01 |
| | [TASK-REQ-024-06](./TASK-REQ-024-06-issue-sync-request-ownership-check.md) | Kiểm sở hữu ở handler worktree, PR | P1 | 024-04, 024-05 |
| | [TASK-REQ-024-07](./TASK-REQ-024-07-request-and-issuesync-metrics.md) | Metric và `/metrics` hai service | P1 | 024-04 |
| | [TASK-REQ-024-08](./TASK-REQ-024-08-alert-rules-trace-and-jira-comment.md) | Alert, `traceparent`, bình luận Jira | P2 | 024-04, 024-07 |
| [BE-REQ-SOL-025](../solutions/BE-REQ-SOL-025-e2e-feature-flag-rollout.md) | [TASK-REQ-025-01](./TASK-REQ-025-01-tenant-settings-and-flow-rpcs.md) | Bảng `tenant_settings`, RPC cờ | P0 | CR-001, 002 |
| | [TASK-REQ-025-02](./TASK-REQ-025-02-flow-gate-grpc-interceptor.md) | Interceptor `flow_gate` | P0 | 025-01 |
| | [TASK-REQ-025-03](./TASK-REQ-025-03-e2e-harness-fakes-and-e01.md) | Khung e2e T1, fake, E01 | P0 | CR-003..013, 025-01 |
| | [TASK-REQ-025-04](./TASK-REQ-025-04-e2e-scenarios-e02-e19.md) | Kịch bản E02 đến E19, ma trận loại | P0 | 025-03, CR-006, 010, 014 |
| | [TASK-REQ-025-05](./TASK-REQ-025-05-feature-flag-e2e-and-ci-matrix.md) | E20 và CI ma trận | P0 | 025-02, 025-04 |
| | [TASK-REQ-025-06](./TASK-REQ-025-06-wiring-check-script.md) | Script kiểm đăng ký | P0 | CR-001 |
| | [TASK-REQ-025-07](./TASK-REQ-025-07-t2-stack-e2e-and-agent-stub.md) | T2 trên stack dev, stub agent | P1 | BE-REQ-SOL-016, 024, 025-01 |
| | [TASK-REQ-025-08](./TASK-REQ-025-08-docs-runbook-and-rollback-drill.md) | Tài liệu, runbook, diễn tập rollback | P1 | 025-01..06, BE-REQ-SOL-024 |

## Thứ tự phụ thuộc

```
SOL-025 phần cờ:   025-01 ─▶ 025-02 ──────────────┐
                      │                            ▼
                      ├─▶ 024-05 ─┐          025-05 (E20 + CI)
SOL-024:   024-01 ─▶ 024-02       ├─▶ 024-06
           024-03 ─▶ 024-04 ──────┘      │
                        ├─▶ 024-07 ─▶ 024-08
                        ▼
SOL-025 e2e:   025-03 ─▶ 025-04 ─▶ 025-05
               025-06 (độc lập, sau CR-001)
               025-07 (sau BE-REQ-SOL-016, 024)  ─▶  025-08 (tài liệu, cuối cùng)
```

## Ghi chú

- Làm sớm: 025-01 và 025-02 (CR-016, 017 đã trỏ tới `request.flowStatus`), 025-06, 024-01, 024-03 (không cần `request-service`).
- Trước khi sửa `updateIssueStatus` hoặc `SyncIssueStatus` chạy `gitnexus_impact` (task 024-04, 024-06).
- Lệnh: `cd backend-go/services/issue-status-sync && go test ./... && go test -tags=integration ./internal/adapter/...`; `cd backend-go/services/request-service && go test ./... && go test -tags=e2e ./e2e/...` (cần Docker). Chưa chạy.
- Số ngày, số Request, ngưỡng alert ở rollout là đề xuất, chưa có số đo thực.
