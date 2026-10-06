# execution-contract: tasks backend (TASK-REQ-029-01 đến 08)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Mỗi task làm được trong 0,5 đến 2 ngày (task 06 và 08 gần 2 ngày).

Solution: [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md). Danh sách solution: [solutions/README](../solutions/README.md).

## Bảng Solution → Task

| Task | Tên | Priority | Service | Phụ thuộc |
|---|---|---|---|---|
| [TASK-REQ-029-01](./TASK-REQ-029-01-task-execution-records-migration-and-repository.md) | Migration `task_execution_records`, domain, repository hai dialect | P0 | `task-service` | TASK-REQ-011-01, migration `task_specs` (CR-REQ-027) để chốt số |
| [TASK-REQ-029-02](./TASK-REQ-029-02-execution-result-parser-proto-and-list-records.md) | Bộ phân tích `ExecutionResult`, proto `result_nonce = 4`, RPC `ListExecutionRecords` | P0 | `task-service`, `proto` | 01 |
| [TASK-REQ-029-03](./TASK-REQ-029-03-contract-executor-engine1-and-failure-events.md) | `ExecuteWithContract`, Engine 1 cho task có spec, `failure_class` vào sự kiện | P0 | `task-service` | 01, 02, TASK-REQ-011-02, TASK-REQ-013-01/02, SOL-033 (hợp đồng `execPrompt`) |
| [TASK-REQ-029-04](./TASK-REQ-029-04-execution-contract-migration-request-service.md) | Migration `execution_contract`, repository packet và báo cáo sẵn sàng | P0 | `request-service` (mới) | TASK-REQ-001-04, 013-03, 014-01 |
| [TASK-REQ-029-05](./TASK-REQ-029-05-task-spec-v2-and-execution-packet-domain.md) | `TaskSpecV2`, `ScopeMatcher`, `RenderExecutionPacket` (domain thuần) | P0 | `request-service` (mới) | TASK-REQ-001-01, CR-REQ-027 |
| [TASK-REQ-029-06](./TASK-REQ-029-06-readiness-gate-agent-relay-and-rpcs.md) | `ReadinessGate` ba tầng, `AgentRelay`, RPC `CheckReadiness`/`GetReadinessReport`/`ListReadiness` | P0 | `request-service` (mới), `proto` | 02, 04, 05, TASK-REQ-033-04, CR-REQ-028 |
| [TASK-REQ-029-07](./TASK-REQ-029-07-verify-execution-and-failure-classification.md) | `VerifyExecution` và `ClassifyFailure` | P0 | `request-service` (mới) | 02, 05, 06, CR-REQ-035, TASK-REQ-014-01/02 |
| [TASK-REQ-029-08](./TASK-REQ-029-08-advance-execution-wiring-flags-and-e2e.md) | Nối vào `AdvanceExecution`/`ReportTaskOutcome`, cờ, sự kiện, hợp đồng JSON, e2e | P0 | `request-service` (mới) | 03, 04 đến 07, TASK-REQ-013-04/05/06, TASK-REQ-025-07 |

## Sơ đồ thứ tự

```
task-service:      01 ──▶ 02 ──▶ 03 ──────────────────────────────────────────────┐
                                                                                     │
request-service:   04 ──────────────────────────┐                                    │
                   05 ──▶ 06 ──▶ 07 ───────────┴──▶ 08 (wiring + e2e) ◀────────────┘
                          ▲
                 TASK-REQ-033-04 (DevServerCapabilityReader)
```

01 đến 03 (task-service) và 04, 05 (request-service) chạy song song. 06 cần 05 và 033-04. 07 cần 06. 08 cuối.

## Ghi chú

- **Số migration:** cả hai service ghi `NNNN`, bắt buộc `ls migrations/{postgres,mysql}` lúc làm. `task-service` hiện dừng ở `0014`; CR-REQ-011 lấy `0015` và `0016`, CR-REQ-027 lấy số kế (`task_specs`); số của task 01 sau đó. `request-service` có xung đột số giữa các CR 001/002/004/006: lấy số lớn nhất hiện có cộng một và báo người điều phối khi có hai PR cùng số.
- **Cờ:** toàn bộ sau `REQUEST_EXECUTION_CONTRACT_ENABLED` (mặc định `false`). Task 01 đến 05 không đổi hành vi khi cờ tắt; chỉ task 03 có nhánh `selectEngine` mới, nhưng nhánh này chỉ bật khi có `TaskSpecLookup` và task có spec.
- **Hợp đồng liên service:** `ExecuteTaskInput.ResultNonce` ↔ `TaskServiceExecuteRequest.result_nonce = 4`; `statuschanged` thêm `failure_class`, `execution_record_id` (`omitempty`); `ListExecutionRecords` (task 02) là đường duy nhất `request-service` đọc kết quả có cấu trúc; tên tham số agent theo [SOL-033 mục 2.I](../../agent-capabilities/solutions/BE-REQ-SOL-033-dev-server-capability-profile-and-agent-client.md).
- **Các task do agent khác soạn** (013-xx, 014-xx, 027, 028, 035) có thể đổi tên hàm; đọc lại file thật lúc làm.
- **Chưa kiểm chứng:** agent trả khối kết quả ổn định hay không, chạy lại Check cho cùng kết quả hay không, độ trễ cổng, hành vi `claude --print` với các tham số mới. Mỗi task nêu ở mục Rủi ro.
- Không file nào tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable; Git theo `guides/reference/git-compatibility.md` (chỉ `rev-parse`, `status --porcelain=v1`, `diff --name-only -z`, `ls-files`).
