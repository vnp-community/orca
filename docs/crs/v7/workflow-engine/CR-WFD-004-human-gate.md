# CR-WFD-004 — Cổng của người: bước `gate`, trạng thái `waiting`, phân quyền, hết hạn, tiếp tục từ cổng

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-004 |
| **Tên** | Loại bước `gate`, bảng `workflow.gates`, trạng thái `waiting` của bước và execution, RPC quyết định cổng, hết hạn tự động, điểm cắm `GateDecider` |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001, CR-WFD-003 (output của cổng), CR-WFD-006 (trạng thái chờ, huỷ) |
| **Mở khoá** | CR-WFD-009 (màn hình duyệt) |
| **Tác động** | `workflow-service` (domain, usecase, migration, proto, executor), `api-gateway` (kênh `workflow.gate.*`), `notification-service` (thông báo người duyệt), frontend |
| **Trả lời** | BACKLOG-027 (cung cấp bước `gate` thật thay vì xoá kiểu `approval` của UI) |

---

## 1. Bối cảnh và vấn đề

1. **Không có điểm dừng chờ người.** Execution chỉ có `pending, running, paused, completed, failed, cancelled`; bước chỉ có `pending, running, completed, failed` (`WSI/domain/execution.go:15-20`, migration 0004). Không biểu diễn được "đang chờ ai đó quyết".
2. **`approval` của UI không có backend.** Frontend khai báo kiểu `approval` (`StepEditor.tsx:39`) nhưng `StepType` ở backend không có (`step.go:19-45`); BACKLOG-027 đang chờ quyết định sản phẩm: xoá hay cung cấp backend.
3. **`Approval` hiện tại là cho việc khác.** Thực thể `Approval` chỉ chặn việc nâng phạm vi template lên company/public, cần quyền admin qua OPA (`WSI/domain/approval.go:41-55`, `usecase/resolve_approval.go:46-79`); không gắn với bước hay execution.
4. **Cơ chế duyệt ở nơi khác không dùng lại được ngay.** `request-service` có Approval chung nhưng subject type không có workflow (`approval_subject.go:6-12`) và trạng thái triển khai thực tế chưa kiểm chứng; `DecisionGate` ở `orchestration-service` thiếu kiểm quyền (theo `docs/crs/v6/approval/README.md`); "duyệt" ở task graph chỉ là quy ước nhãn (BL-TG-05).
5. **Tạm dừng hiện tại không phải chờ.** `paused` do người ấn nút, không có người quyết, không có hết hạn, và dispatcher không dừng thật (xem CR-WFD-006).

## 2. Giải pháp đề xuất

### 2.1 Bước `gate`

```jsonc
{ "id": "review", "type": "gate", "dependsOn": ["draft"],
  "config": {
    "title": "Duyệt kết quả của bước draft",          // hiển thị cho người duyệt
    "instructions": "…",                                // tuỳ chọn, văn bản
    "context": { "doc": "{{steps.draft.outputs.report}}" },   // dữ liệu hiển thị, theo CR-WFD-003
    "decisions": ["approve", "reject", "request_changes"],    // 2..8 giá trị, mặc định 2 đầu
    "approvers": { "users": ["<uuid>"], "roles": ["lead"], "mode": "any|all", "minApprovals": 1 },
    "allowSelfApproval": false,
    "expiresInSec": 259200,
    "onExpire": "fail|skip|decision:<value>"
  },
  "outputs": [ { "name": "decision", "type": "string" }, { "name": "comment", "type": "string" }, { "name": "decidedBy", "type": "json" } ] }
```

- `decisions` đi vào `steps.review.outputs.decision`; các bước sau rẽ nhánh bằng `when` (CR-WFD-002). Không có logic rẽ nhánh riêng trong cổng.
- `approvers` có thể tham chiếu input (`"users": ["{{inputs.owner}}"]`) hoặc vai trò của tenant; phải giải được thành **ít nhất một người** lúc cổng mở, nếu không bước `failed` (`WORKFLOW_GATE_NO_APPROVER`).
- `allowSelfApproval=false` (mặc định): người khởi chạy execution **không được** quyết cổng của chính lượt chạy đó, trừ khi cờ bật rõ ràng.

### 2.2 Bảng `workflow.gates` (mới)

| Cột | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `execution_id`, `step_id` | uuid, varchar(64) | duy nhất `(execution_id, step_id)` (chạy lại bước tạo `attempt` mới) |
| `attempt` | int | từ 1 |
| `status` | varchar(12) | `open`, `decided`, `expired`, `cancelled` |
| `snapshot` | jsonb/json | `title`, `instructions`, `context` đã **nội suy tại thời điểm mở** (bất biến), ≤ 256 KiB |
| `required` | jsonb/json | tập người/vai trò hợp lệ và `mode/minApprovals` đã giải |
| `decisions_allowed` | jsonb/json | |
| `opened_at`, `expires_at`, `decided_at` | timestamptz | đồng hồ DB |
| `outcome` | varchar(32) | giá trị `decision` cuối |

Bảng `workflow.gate_votes` (`gate_id, user_id, decision, comment, voted_at`; duy nhất `(gate_id, user_id)`) — mỗi người một phiếu, không sửa; muốn đổi ý chỉ khi cổng còn `open` bằng cách thay phiếu và ghi audit.

### 2.3 Vòng đời

1. Khi bước `gate` đến lượt, dispatcher: nội suy `context`, giải `approvers`, ghi `gates(open)`, đặt bước `waiting` và execution `waiting` (nếu **không còn bước nào khác đang chạy**; nếu còn thì execution vẫn `running`), phát sự kiện `gate.opened` và gọi thông báo.
2. **Dispatcher không giữ goroutine cho cổng.** Cổng mở là một dòng DB; tiến trình khởi động lại không mất cổng (khác với bước agent/shell). `RecoverExecutions` bỏ qua bước `waiting`.
3. `DecideGate{gateId, decision, comment}`: kiểm quyền (2.4), ghi phiếu; khi đủ `minApprovals` (hoặc có bất kỳ `reject` nếu `mode=all`) đặt `decided`, ghi `outputs` của bước, bước `completed`, **tái kích hoạt lập lịch** cho các bước phụ thuộc.
4. Hết hạn: bộ quét định kỳ (cùng tần suất bộ quét sự kiện) đặt `expired` và áp `onExpire` (`fail` → bước `failed` mã `WORKFLOW_GATE_EXPIRED`; `skip` → `skipped`; `decision:<v>` → hoàn tất với quyết định đó, ghi `decidedBy = system:expiry`).
5. Huỷ execution (CR-WFD-006) đặt mọi cổng `open` của nó thành `cancelled`.
6. `PauseExecution` khi có cổng mở: cổng vẫn mở; quyết định vẫn được ghi nhưng việc kích hoạt bước sau chờ tới `Resume`.

### 2.4 Quyền

- `DecideGate` chỉ chấp nhận người thuộc `required`, trừ admin tenant khi tenant bật `workflow_gate_admin_override` (mặc định tắt); mọi lần override ghi audit riêng.
- Người không thuộc tập hợp: `WORKFLOW_GATE_FORBIDDEN`. Cổng đã `decided/expired`: `WORKFLOW_GATE_CLOSED`.
- Kiểm quyền thực hiện ở backend, **không** dựa vào cờ UI.
- `snapshot.context` chỉ hiển thị cho người trong `required` và người xem execution; áp che secret (CR-WFD-010).

### 2.5 Điểm cắm `GateDecider`

Giao diện Go `GateDecider{ ResolveApprovers(ctx, gate) ; OnOpened(ctx, gate) ; OnDecided(ctx, gate) }` với cài đặt mặc định tự có trong `workflow-service` (O4). Cài đặt thay thế (ví dụ chuyển cổng sang hệ duyệt chung của nền tảng) cắm qua cấu hình; **không** bắt buộc `request-service` và không đổi hợp đồng cổng ở các CR khác.

### 2.6 Kênh và sự kiện

| Thành phần | Nội dung |
|---|---|
| RPC | `ListGates{executionId?|userId pending}`, `GetGate`, `DecideGate` |
| `wscompat` | `workflow.gate.list`, `workflow.gate.get`, `workflow.gate.decide` (CR-WFD-008) |
| NATS | `orca.workflow.gate.opened`, `.decided`, `.expired` (payload: `execution_id, step_id, gate_id, status`, không nhúng `snapshot`) |
| Thông báo | Dùng `notification-service` có sẵn; người duyệt nhận liên kết tới màn hình cổng (CR-WFD-009) |

## 3. Tiêu chí chấp nhận

1. Workflow `A → gate → B` dừng ở `gate`: bước `waiting`, execution `waiting`, B chưa chạy; sau `DecideGate(approve)` B chạy.
2. Với `decisions=[approve,reject]` và hai bước sau có `when` đối lập, đúng một nhánh chạy.
3. Khởi động lại `workflow-service` khi cổng đang mở không làm mất hay đóng cổng.
4. Người không thuộc `approvers` nhận `WORKFLOW_GATE_FORBIDDEN`; người khởi chạy duyệt cổng của chính mình bị từ chối khi `allowSelfApproval=false`.
5. `mode=all` với 2 người: cần cả hai `approve`; một `reject` kết thúc cổng.
6. Hết hạn áp đúng `onExpire`; sự kiện `gate.expired` được phát một lần.
7. Huỷ execution đóng mọi cổng mở thành `cancelled`.
8. Gọi `DecideGate` hai lần đồng thời cho cùng cổng chỉ một lần có tác dụng (khoá theo hàng/CAS).
9. Test hai dialect; test RLS chặn đọc cổng chéo tenant.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `gates`, `gate_votes`; mở rộng CHECK trạng thái (CR-WFD-002 mục 2.3) |
| UI | Kiểu `approval` hiện có trong UI đổi tên thành `gate` kèm form cấu hình; template cũ dùng `approval` được di trú (CR-WFD-011) |
| Tương thích | Client cũ không biết `waiting` sẽ hiển thị lạ; UI phải xử lý (CR-WFD-009) |
| Rủi ro | Cổng treo vô hạn nếu không đặt `expiresInSec`: **mặc định bắt buộc có hạn** (tối đa 30 ngày, cấu hình theo tenant) |
| Rủi ro | `context` chứa dữ liệu nhạy cảm hiển thị cho người duyệt: kiểm quyền và che secret là bắt buộc |

## 5. Chưa kiểm chứng / câu hỏi mở

- Cách xác định "vai trò" của tenant dùng trong `approvers.roles` (nguồn: `auth-service` hay `tenant-service`) chưa đọc.
- Khả năng dùng lại `notification-service` cho thông báo tới người cụ thể chưa kiểm tra.
- Có nên cho phép cổng nhiều bước (nhiều vòng yêu cầu chỉnh sửa) hay chỉ một lượt quyết: mặc định một lượt; vòng lặp biểu diễn bằng `foreach`/`sub_workflow` ở CR-WFD-002.
