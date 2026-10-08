# execution-contract: solutions backend (BE-REQ-SOL-029)

> **🚧 SOL-029: 3/8 task xong** (nửa `task-service`, 2026-10-08); nửa `request-service` chưa làm. Tài liệu ngày 2026-10-06; số dòng và đường dẫn đã đối chiếu với code `backend-go/services/task-service`, `infra-fleet-service` và `agent/src/relay` cùng ngày. `request-service` chưa có thư mục: mọi đường dẫn của nó là "(mới)".

Nguồn: [docs/crs/v6/execution-contract](../../../../../../docs/crs/v6/execution-contract/README.md). README v6 [mục 8](../../../../../../docs/crs/v6/README.md) thắng mục 3 khi mâu thuẫn. Tài liệu thiết kế tham chiếu: [`tdd/README.md`](../../../../tdd/README.md), `architecture/03, 05, 08, 09`, [`services/task-service.md`](../../../../tdd/services/task-service.md), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md), [`services/orchestration-service.md`](../../../../tdd/services/orchestration-service.md), [`services/project-service.md`](../../../../tdd/services/project-service.md).

## Bảng CR → Solution → Task

| CR | Solution | Service | Task (xem [tasks/README](../tasks/README.md)) |
|---|---|---|---|
| [CR-REQ-029](../../../../../../docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md) `TaskSpec` v2, `ExecutionPacket`, `ReadinessGate`, `ExecutionResult`, `VerifyExecution`, `Failure.class`, `task_execution_records`, `result_nonce = 4`, Engine 1 cho task có spec | [BE-REQ-SOL-029](./BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) | `task-service`, `request-service` (mới), `proto/orca/task/v1`, `proto/orca/request/v1` | TASK-REQ-029-01 đến 08 |

## Thứ tự phụ thuộc

```
SOL-011 (request_id, 0015/0016)  CR-REQ-027 (task_specs)  SOL-033 (hồ sơ năng lực)  SOL-013 (AdvanceExecution)
        │                              │                         │                          │
        ▼                              ▼                         ▼                          ▼
 task-service:  01 migration+repo ─▶ 02 parser+proto ─▶ 03 ExecuteWithContract, Engine 1
                                                              │ (sự kiện statuschanged có failure_class)
 request-service: 04 migration ─┐                              │
                  05 spec v2 + packet (domain thuần) ─┬─▶ 06 ReadinessGate + AgentRelay + RPC ─▶ 07 VerifyExecution + ClassifyFailure
                                                      │                                              │
                                                      └──────────────────────────────────────────────┴─▶ 08 wiring + cờ + e2e
```

Ba lớp giao hàng của CR (README feature): (1) `TaskSpec` v2 và packet, không đổi hành vi khi cờ tắt: task 05; (2) cổng, chế độ chạy khô qua `CheckReadiness`: task 04, 06; (3) kết quả và kiểm chứng: task 01 đến 03, 07, 08.

Mở khoá: SOL-030 (`files_changed` đã kiểm chứng, `base_sha`, `AgentRelay`, sự kiện `execution.verified`), CR-REQ-036 (badge sẵn sàng).

## Quyết định chung

| # | Quyết định | Lý do |
|---|---|---|
| E1 | `TaskSpec` v2 lưu ở `task_specs` của CR-REQ-027, không thêm bảng spec | Một nơi lưu; spec khoá sau duyệt Plan |
| E2 | Packet render ở `request-service` bằng hàm thuần, gửi qua `prompt` | Chỉ ở đó có Request, Solution, Plan; `buildExecutePrompt` không đổi |
| E3 | Port `ContractAgentExecutor` mới thay vì đổi chữ ký `SimpleExecutor` | Không phá fake và hai adapter; cộng thêm |
| E4 | Task có spec luôn Engine 1; nhánh trong `selectEngine` đặt sau kiểm con, trước `depends_on` | Ghi đè prompt chỉ chạy ở `direct_agent` (`execute_task.go:179`) |
| E5 | Khối kết quả `ORCA_RESULT_BEGIN/END <nonce>`; phân tích ở `task-service`, kiểm chứng ở `request-service` | Toàn bộ `stdout` chỉ có ở `task-service` |
| E6 | Cổng chỉ đọc; `needs_info`, `spec_defect`, `env_defect` không tính lần thử | Lỗi không phải của agent |
| E7 | Cổng tự khớp glob trên `git ls-files`, không dùng `fs.glob` | `fs.glob` chỉ khớp tên cuối và cần `find` |
| E8 | Lệnh qua `agent.exec` luôn `binary`+`args` tách; không nối chuỗi shell từ dữ liệu người dùng | `agent.exec` không có shell; chống chèn lệnh |
| E9 | Ranh giới `<untrusted-...>` suy ra từ nội dung | Giữ packet xác định |
| E10 | Mọi thứ sau `REQUEST_EXECUTION_CONTRACT_ENABLED` (mặc định tắt) | Bật dần cùng `request_flow_enabled` |

## Đã kiểm chứng và điểm khác CR gốc

- `fs.glob` của agent không hiểu `**` (chỉ `find -name <tên cuối>`); `git.exec` không có `ls-files` và cấm `\ ! < > | & ; $`; `agent.exec` không có shell (SOL-029 mục 1, điều 1 đến 3). `command -v` của CR phải bọc `sh -c`, Windows dùng `where.exe`.
- Q3 của CR trả lời được: `project-service.GetWorktree(worktree_id).path` có; vấn đề còn lại là task đầu của Plan chưa có worktree lúc cổng chạy (SOL-029 điều 4, Q1).
- CR-REQ-028 mục 2.6 giao việc đưa câu trả lời Clarification vào packet cho CR-REQ-029, nhưng CR-REQ-029 không liệt kê mục này (SOL-029 điều 5, Q2).
- `common/secretscan` chưa có trong repo (CR-REQ-035 tạo); mọi chỗ dùng đều qua cổng `SecretScanner`/`redact` để không bị chặn.
- `task_run_outcomes` do TASK-REQ-013-03 tạo; migration của feature này chỉ `ALTER` thêm ba cột, không sửa file đó.
- Số migration cả hai service ghi `NNNN` và bắt buộc `ls` lúc làm: `task-service` tiếp theo `0015` (CR-REQ-011, 027 lấy trước); `request-service` chồng số giữa CR 001/002/004/006.
- Chưa chạy bất kỳ test nào; mọi lệnh test là lệnh dự kiến.
