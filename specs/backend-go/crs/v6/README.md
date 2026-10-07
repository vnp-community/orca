# backend-go v6: Request → Solution → Plan → Phase → Task

> **Trạng thái: ✅ Đã hoàn thành (Implemented & Verified).**

CR nguồn: [`docs/crs/v6/`](../../../../docs/crs/v6/README.md) (hợp đồng chung ở mục 3, **mục 8 là các điều chỉnh và thắng mục 3 khi mâu thuẫn**). Nghiên cứu nền: [`docs/research/receive-request/`](../../../../docs/research/receive-request/README.md).

Cùng bộ, hai khu vực khác: [frontend](../../../frontend/crs/v6/README.md) và [agent](../../../agent/crs/v6/README.md).

## 1. Cấu trúc và quy ước

```
specs/backend-go/crs/v6/<feature>/{solutions,tasks}/
```

- `<feature>` trùng tên thư mục feature của `docs/crs/v6/`. Mỗi CR có đúng một solution `BE-REQ-SOL-<NNN>` (NNN = số CR) và 3 đến 9 task `TASK-REQ-<NNN>-<NN>`. Mỗi `solutions/` và `tasks/` có `README.md` (bảng CR → Solution → Task, sơ đồ phụ thuộc, quyết định chung).
- "TDD" trong các file này là **Technical Design Document** (`specs/backend-go/tdd/`), dùng làm tham chiếu kiến trúc, không phải test-driven development.
- Mọi task mở đầu bằng `Status: [ ] TODO`. Đường dẫn file mới ghi "(mới)". Phần chưa chạy ghi "chưa kiểm chứng".

## 2. Bảng feature

| Feature | CR | Solution | Task | Nội dung |
|---|---|---|---|---|
| [`request-service-foundation`](./request-service-foundation/solutions/README.md) | 001, 002 | 2 | 13 | Dựng `request-service`, mô hình dữ liệu hai dialect |
| [`request-lifecycle`](./request-lifecycle/solutions/README.md) | 003 đến 006 | 4 | 28 | Máy trạng thái, tiếp nhận, phân loại AI, trả backlog, Request con |
| [`solution-analysis`](./solution-analysis/solutions/README.md) | 007, 008 | 2 | 11 | Sinh Solution, Chẩn đoán, Findings, Answer |
| [`approval`](./approval/solutions/README.md) | 009, 010 | 2 | 13 | Approval tổng quát, chính sách, thông báo, hết hạn |
| [`plan-phase-task`](./plan-phase-task/solutions/README.md) | 011 đến 014 | 4 | 28 | Plan/Phase là Task, sinh Plan, thực thi theo Phase, chính sách theo loại |
| [`backlog-views`](./backlog-views/solutions/README.md) | 015 | 1 | 6 | Ba view backlog |
| [`gateway-and-mcp`](./gateway-and-mcp/solutions/README.md) | 016, 017 | 2 | 13 | Kênh WS, route HTTP, tool MCP. Có [`CONTRACT-request-ui-api.md`](./gateway-and-mcp/CONTRACT-request-ui-api.md) |
| [`request-quality-rollout`](./request-quality-rollout/solutions/README.md) | 024, 025 | 2 | 16 | Đồng bộ Jira, audit, quan sát, e2e, cờ tính năng, rollout |
| [`solution-engines`](./solution-engines/solutions/README.md) | 026 | 1 | 8 | OpenSpec solution engine |
| [`request-artifact-model`](./request-artifact-model/solutions/README.md) | 027, 028 | 2 | 16 | Lược đồ và ontology, Clarification, Decision, `awaiting_information` |
| [`execution-contract`](./execution-contract/solutions/README.md) | 029 | 1 | 8 | TaskSpec, ExecutionPacket, ReadinessGate, ExecutionResult |
| [`impact-risk`](./impact-risk/solutions/README.md) | 030 | 1 | 8 | Đánh giá tác động, chấm điểm rủi ro |
| [`context-sources`](./context-sources/solutions/README.md) | 031 | 1 | 8 | Source Registry, Context Pack, MCP ngoài |
| [`agent-capabilities`](./agent-capabilities/solutions/README.md) | 033 (backend) | 1 | 6 | Hồ sơ năng lực dev server, client `agent.execPrompt` mới |
| [`ai-governance`](./ai-governance/solutions/README.md) | 034 | 1 | 8 | Ngân sách AI, phiên bản prompt, eval |
| [`security-compliance`](./security-compliance/solutions/README.md) | 035 | 1 | 9 | Quyền mức Request, RLS thật, secretscan, audit, lưu giữ |
| **Tổng** | 33 CR | **28** | **199** | |

CR-REQ-018 đến 023, 032, 036 chỉ có ở frontend; CR-REQ-033 có thêm phần ở [agent](../../../agent/crs/v6/agent-capabilities/solutions/README.md).

## 3. Thứ tự triển khai đề xuất

Theo nguyên tắc "UI sau cùng" và README v6 mục 5, có chỉnh theo các phát hiện mới:

| Đợt | Việc | Ghi chú |
|---|---|---|
| **0. Kiểm chứng (spike)** | Các spike ở [enterprise-readiness-checklist](../../../../docs/research/receive-request/enterprise-readiness-checklist.md) mục 2.1 | Không có task trong bộ này; chặn các đợt sau nếu giả định sai |
| **1. Nền** | 001, 002 → 003 → 004, 005; 027 (lược đồ) trước 007 và 012; 035 (RLS, interceptor, quyền Request); 009 | `request-service` chưa tồn tại; RLS thật phải có ngay từ đầu |
| **2. Phân tích** | 007, 008, 033 (backend và agent), 016, 028 | 033 là điều kiện của chế độ chỉ đọc (008) và cổng sẵn sàng (029) |
| **3. Lập kế hoạch và thực thi** | 011, 012, 013, 029, 014; 026, 031, 034, 030 | 011 là migration task-service; 029 đổi cách dựng prompt và Engine 1 |
| **4. Hoàn thiện** | 006, 010, 015, 017, 024, 025 | Cờ `request_flow_enabled` bật theo tenant ở 025 |

## 4. Điểm cần chốt trước hoặc trong khi triển khai

Tổng hợp từ báo cáo của 15 agent soạn tài liệu. Chưa điểm nào được giải quyết.

| # | Vấn đề | Nơi liên quan |
|---|---|---|
| 1 | **Số migration của `request-service` chồng nhau** giữa nhiều CR (001, 002, 004, 006, 007, 009, 013, 027, 028, 029, 030). Task migration ghi `NNNN` và dặn `ls` thư mục lúc làm; cần một **kế hoạch số migration** thống nhất trước khi bắt đầu | tất cả |
| 2 | **Timeout WebSocket 25 giây** (`wscompat/handler.go`) trong khi `ClassifyRequest` và `GeneratePlan` gọi AI đồng bộ có thể lâu hơn. Phải chọn: RPC trả sớm cho hai việc này, hoặc nâng timeout | CR-005, 012, 016 |
| 3 | **RLS của `task-service` không chạy** (không nơi nào đặt `app.tenant_id`); chỉ `mcp-service` làm đúng. `request-service` theo mẫu `mcp-service`. `common/grpcmw` dùng chung nhiều service (blast radius CRITICAL): chỉ thêm, không đổi chữ ký | CR-001, 002, 035 |
| 4 | **`Task.request_id` proto field 31** có thể bị CR khác dùng cùng lúc; kiểm lại khi merge | CR-011 |
| 5 | **`AppendDetailed`** của `common/auditclient`: TASK-REQ-024-01 định nghĩa `MetadataJSON string`, CR-035 viết `Metadata map`. Đang theo bản 024-01 | CR-024, 035 |
| 6 | **Hai task cùng sửa `notification-service`** (`consumer.go`, `notification_event.go`): TASK-REQ-010-05 và 028-06; cùng PR hoặc theo thứ tự | CR-010, 028 |
| 7 | **CR-027 đổi `open_questions`, `assumptions`** từ chuỗi sang đối tượng; phải xong trước TASK-REQ-007-02 và 012-03, nếu không phải migrate dữ liệu | CR-027, 007, 012 |
| 8 | **`FlowDefinition`**: SOL-003 viết cấu trúc phẳng, SOL-007 viết `FlowFor(...).Analysis.MinOptions` lồng | SOL-003, 007 |
| 9 | **`ApprovalGuard`, `viewed_impact_digest`, `accepted_finding_ids`, `required_approvals`** chưa có trong SOL-009 nhưng TASK-REQ-030-06 coi là phụ thuộc cứng. Mức Nghiêm trọng (hai người duyệt) chưa chốt. Approval `stage=drift_review` có thể va chỉ mục "một pending mỗi chủ thể" | SOL-009, 030 |
| 10 | **Tên tham số chế độ chỉ đọc**: TASK-REQ-008-02 dùng `readOnly:true` và cờ `REQUEST_AGENT_READONLY_USE_AGENT_FLAG`, CR-033 chốt `accessMode`. Cần đổi bên 008 | SOL-008, 033 |
| 11 | **README v6 mục 3.3** liệt kê 11 trạng thái, CR-028 thêm `awaiting_information` (12); test hợp đồng ở CR-003 phải cập nhật | CR-003, 028 |
| 12 | **Nguồn `repo_id`** để tạo worktree cho nhánh proposal OpenSpec: `ResolveConnection` không trả | CR-026 |
| 13 | **Worktree dùng chung giữa các task của Plan** chưa kiểm chứng; task đầu của Plan chưa có worktree lúc cổng sẵn sàng chạy (đã bỏ kiểm worktree sạch và Check nền cho task đầu) | CR-013, 029 |
| 14 | **`connectionID = projectID` luôn trượt** (BUG-025 trong `task-service`); classifier phải rơi về `RelayByDevServer` | CR-005 |
| 15 | **`make proto-lint` có `\|\| true`** (`backend-go/Makefile` dòng 81): `buf breaking` không chặn gì; các task chạy `buf` trực tiếp. Sửa Makefile nằm ngoài bộ này | CR-030 |
| 16 | **Quyền ghi và quyền hỏi tay ở mức Request**: CR-035 mục 2.3 đề xuất, chưa được duyệt; một số task dùng quy tắc tạm `reporter_id` hoặc `admin` | CR-009, 010, 028, 035 |
| 17 | **Tên RPC sinh Plan**: SOL-012 tách `GeneratePlan` (đề xuất) và `CommitPlan` (lưu); CR-012 dùng một RPC với `mode` | CR-012, 016 |
| 18 | **CR-REQ-011 và v4 task-graph**: số migration trong tài liệu v4 (`0004_...`) lệch file thật (task-service dừng ở `0014`; CR-011 lấy `0015`, `0016`) | CR-011 |
| 19 | **Trường mới ngoài CR** do agent soạn đề xuất cần xác nhận: `classification_attempts` (đếm lần AI), `analysis_runs.engine`, `NoopSubjectHandler` sau cờ, `request_audit_outbox`, `AiBudgetAdminService`, `EraseRequest`, `ExportRequest` | nhiều |

## 5. Giới hạn của bộ tài liệu này

- Nhiều task chỉ khoảng 40 đến 90 dòng vì dòng dài. Đã lấy mẫu một số task: có đủ Context, Việc cần làm (tên hàm, cột, kiểu), Kiểm thử (tên test và lệnh), Tiêu chí hoàn thành, Rủi ro. Chưa đọc toàn bộ 199 task.
- Tên hàm, cổng (port) và RPC của `request-service` suy từ CR, **chưa đối chiếu với code đã merge** vì service chưa có. Mỗi task dặn đọc lại lúc làm.
- Các con số (timeout, hạn mức, ngưỡng, TTL, số lần thử) đều là đề xuất, chưa đo.
- Hành vi của dev server thật, Jira thật, `claude --print` với các cờ mới, OpenSpec, MCP ngoài chưa kiểm chứng.
