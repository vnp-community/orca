# CR-TG-006 — Task Execute Streaming: Agent Output/Exit Relay + Activity Feed Data Plane

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-006 |
| **Tên** | Streaming RPC mới ở `infra-fleet-service` (mẫu `AttachPty`) để Task Execute/Complex Executor nhận output/exit real-time, cộng `chunk` notification cho `agent.execPrompt`/`shell.exec` |
| **Loại** | Feature / Infra |
| **Priority** | P1 — không chặn execution, nhưng chặn Activity Feed thời gian thực |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-TG-004](./CR-TG-004-orchestration-service-coordinator-run-lifecycle.md), [CR-TG-005](./CR-TG-005-task-agent-execution-permission-and-complex-executor.md) (cần dispatch path thật để stream từ đó); nên sequence cùng lúc hoặc sau `docs/crs/v3/flow-task/CR-FLOW-TASK-003-unified-task-activity-event-catalog.md` (kênh event rời rạc) — CR này chỉ bổ sung phần **liên tục** mà CR-003 chủ động scope ra |
| **Tác động** | `backend-go/proto/orca/infra-fleet/v1/infra_fleet.proto`, `backend-go/services/infra-fleet-service/`, `agent/src/relay/agent-print-mode-exec.ts`, `agent/src/relay/*shell-exec*` |

---

## 1. Vấn đề

Cả 4 RPC mà 3 hệ Task/Workflow/Orchestration gọi vào Dev Server Agent
(`agent.execPrompt`, `agent.exec`, `shell.exec`, `ai.complete`) đều theo mô
hình **buffer toàn bộ rồi trả 1 lần** — `agent-print-mode-exec.ts:126-165`:
`child.stdout.on('data', d =&gt; stdout += d)`, chỉ `resolve()` một lần khi
process `close`. Trong khi đó `agent/` **đã có sẵn 3 cơ chế streaming thật**
cho mục đích khác: `pty.data` (qua `AttachPty`), `git.execStream`,
`git.responseChunk` — nghĩa là đây là gap "chưa áp dụng pattern đã biết",
không phải giới hạn năng lực nền tảng.

Hệ quả cụ thể:

- **Task Execute (Engine 2) hoàn toàn không có kênh nhận output/exit** —
  `grep -rn "infrafleet|Relay(" backend-go/services/orchestration-service/internal/`
  = 0 kết quả: `orchestration-service` chưa từng gọi `infra-fleet-service`.
  Ngay cả khi gọi, `Relay`/`RelayByDevServer`
  (`infrafleet.proto:47`) là **unary-only** — `agent.spawn`'s output/exit đến
  dưới dạng JSON-RPC notification ngoài luồng mà 1 request/response unary
  không thể nhận được về mặt cấu trúc.
- Không có Activity Feed real-time nào hiển thị được tiến trình 1 task đang
  chạy agent — người dùng chỉ biết kết quả sau khi xong hoàn toàn.

## 2. Giải pháp đề xuất

### 2.1 Streaming RPC mới ở `infra-fleet-service`, mẫu `AttachPty`

```protobuf
// infra_fleet.proto — cạnh AttachPty (dòng 104) / AttachScreencast (dòng 117) đã có
rpc AttachAgentSpawn(AttachAgentSpawnRequest) returns (stream AgentSpawnEvent);

message AttachAgentSpawnRequest {
  string dev_server_id = 1;
  string spawn_id = 2; // = agent.spawn's ptyId, tái sử dụng convention có sẵn
}
message AgentSpawnEvent {
  oneof event {
    AgentOutputChunk output = 1;   // reuse shape agent.output {ptyId, data}
    AgentExitedEvent exited = 2;   // reuse shape agent.exited {ptyId, exitCode}
  }
}
```

Tái sử dụng nguyên shape `agent.output`/`agent.exited` đã có ở `agent/` —
**không cần đổi gì ở `agent/`** cho phần này, chỉ cần `infra-fleet-service`
subscribe các notification này qua kết nối relay hiện có và forward vào
stream gRPC.

### 2.2 `chunk` notification cho `agent.execPrompt`/`shell.exec`

```typescript
// agent/src/relay/agent-print-mode-exec.ts — bổ sung, tái dùng convention pty.data
child.stdout.on('data', (chunk) => {
  stdout += chunk
  notify('agent.execPrompt.chunk', { requestId, data: chunk.toString() }) // MỚI
})
```

Tương tự cho `shell.exec`. Notification này KHÔNG thay thế response cuối
cùng (vẫn trả `{stdout, exitCode}` khi xong) — chỉ bổ sung observability
real-time, giữ nguyên hành vi hiện tại cho consumer chưa quan tâm streaming.

### 2.3 Quyết định kênh: gộp vào outbox của CR-FLOW-TASK-003 hay tách riêng?

Khuyến nghị: **tách riêng** — `chunk` events có tần suất cao (mỗi vài trăm ms
khi agent đang in output), không phù hợp đi qua outbox transactional
(overhead ghi DB mỗi chunk). CR-FLOW-TASK-003's `task.activity` channel dành
cho event rời rạc (status/dispatch/step completed); kênh `chunk` này nên là 1
WS subscription riêng theo `spawnId`/`requestId`, không transactional,
best-effort (mất 1 vài chunk giữa chừng chấp nhận được, khác với
status-changed event phải đảm bảo delivery).

### 2.4 Thứ tự triển khai khuyến nghị (theo audit)

1. Đảm bảo `workflow-service`'s agent step executor gọi đúng
   `agent.execPrompt` trước (xem [`docs/crs/v4/workflow/CR-WF-001`](../workflow/CR-WF-001-fix-agent-step-executor-relay-method.md))
   — không xây streaming trên 1 shape đang gọi sai.
2. Thêm `AttachAgentSpawn` ở `infra-fleet-service` (§2.1).
3. Thêm `chunk` notification ở `agent/` (§2.2), tái dùng `pty.data` convention.
4. Quyết định kênh gộp/tách theo §2.3, wire vào `task.activity` UI (CR-TG-007)
   hoặc kênh riêng.

## 3. Rủi ro / Không thuộc phạm vi

- Không xây `step.execute`/`event.stepOutput` theo đúng tên đã đề xuất ở
  `docs/crs/v2/dev-server/CR-DS-002-gateway-agent-rpc-protocol.md` — CR đó
  vẫn ở trạng thái "Proposed", implementation thật đã rẽ hướng dùng RPC theo
  từng step-type riêng lẻ (`agent.execPrompt`/`shell.exec`/`notification.send`)
  thay vì 1 envelope `step.execute` chung. CR này đi theo hướng thật đang có,
  không giả định `CR-DS-002`'s protocol tồn tại.
- Không thuộc phạm vi: `Task Execute` (Engine 2) dispatch dashboard UI thật
  (thay thế `OrchestrationPage.tsx`'s storyboard) — đó là 1 CR follow-up
  riêng, gated bởi CR-TG-004's `orchestration-service` RPC tồn tại trước.
- `chunk` streaming là best-effort — không cam kết delivery, không dùng cho
  audit trail (audit trail dùng event rời rạc của CR-FLOW-TASK-003).

## Acceptance Criteria

- [ ] `AttachAgentSpawn` stream trả đúng output/exit của 1 `agent.spawn` đã
      biết `spawnId`, test bằng cách spawn 1 process in nhiều dòng rồi thoát.
- [ ] `agent.execPrompt.chunk`/`shell.exec.chunk` notification bắn ra trong
      lúc process đang chạy, không chặn response cuối cùng.
- [ ] Ngắt kết nối relay giữa chừng không làm crash `infra-fleet-service`
      (test: kill connection khi đang stream).
- [ ] Xác nhận `workflow-service`'s agent step executor đã dùng đúng
      `agent.execPrompt` (CR-WF-001) trước khi CR này merge phần liên quan
      tới workflow step.
- [ ] Không có thay đổi hành vi cho consumer hiện tại chưa subscribe
      streaming (response cuối cùng giữ nguyên shape).
