# CR-WFD-008 — Quan sát và API: stream sự kiện, trạng thái từng bước, kênh `wscompat` còn thiếu

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-008 |
| **Tên** | Implement `StreamExecutionEvents`, `GetExecution` trả chi tiết từng bước, bộ sự kiện đầy đủ, các kênh `workflow.*` còn thiếu ở gateway, trace/metric/audit |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-006 (trạng thái và attempt) |
| **Mở khoá** | CR-WFD-009 (màn hình theo dõi trực tiếp), CR-WFD-004 và 007 (kênh cổng, trigger) |
| **Tác động** | `workflow-service` (`adapter/grpc/server.go`, usecase, outbox), `api-gateway` (`channels_workflow.go`, `workspace_events.go`), proto, frontend |
| **Thay thế một phần** | CR-WF-007 (live streaming), trả lời BACKLOG-021, 026 |

---

## 1. Bối cảnh và vấn đề

1. **`StreamExecutionEvents` chưa implement.** Có trong proto nhưng `server.go` nhúng `UnimplementedWorkflowServiceServer`; chỉ thấy tên trong chú thích (`ports.go:244`, `step.go:218`). Suy ra trả `Unimplemented` (chưa kiểm chứng bằng cách gọi).
2. **`WorkflowExecution` không có chi tiết bước.** Message chỉ có `id, template_id, status, root_trace_id, project_id, origin_task_id` (`workflow.proto:193-200`) dù bảng `step_executions` có đầy đủ. UI chỉ thấy trạng thái cấp execution, poll mỗi 4 giây (`hooks/useWorkflowExecution.ts:7-22`).
3. **Sự kiện thiếu thông tin.** Payload `step.completed/failed` chỉ có `{execution_id, step_id, step_type, status, origin_task_id}`, không có thời gian, số lần thử, mã lỗi (`wave_dispatcher.go:288-290`).
4. **Kênh gateway thiếu.** Có `workflow.execute/cancel/pause/resume/getExecution/template.{create,clone,update,list,resolve}/hasActiveExecutions/executeAdHocStep/listExecutions`. **Không có** kênh cho `publish`, duyệt, chia sẻ, preview, import, đánh giá, và không có kênh stream (`channels_workflow.go`; CR-PW-005 từng nêu thiếu). `workflow.listExecutions` trả mảng trần, không bọc envelope (khác quy ước các kênh khác).
5. **Gateway chỉ đẩy hai sự kiện cuối.** `workflow.execution.completed/failed` qua `workspace_events.go:162-163`; sự kiện bước chỉ đến `task_activity` (`channels_task_activity.go:164-166`).
6. **Backlog có thể đã cũ.** BACKLOG-021 ghi "chặn do CR-FLOW-TASK-003 và BE-SOL-006" trong khi sự kiện NATS và `step_executions` đã có.

## 2. Giải pháp đề xuất

### 2.1 Bộ sự kiện

Mọi sự kiện có `schema_version`, `execution_id`, `seq` (tăng đơn điệu theo execution), `ts` (đồng hồ DB), `tenant_id`.

| Loại | Payload bổ sung |
|---|---|
| `execution.started` | `template_id, template_version, trigger{kind,id}, run_as` |
| `execution.status_changed` | `from, to, reason` |
| `execution.completed/failed/cancelled` | `duration_ms, error_code?` |
| `step.started` | `step_id, step_type, attempt` |
| `step.attempt_failed` | `step_id, attempt, error_code, will_retry, next_retry_at?` |
| `step.completed` | `step_id, attempt, duration_ms, outputs_ref` (CR-WFD-003 2.5) |
| `step.failed` | `step_id, error_code, error_message` (đã che secret) |
| `step.skipped` | `step_id, reason` |
| `step.waiting` | `step_id, gate_id?` |
| `step.output` | `step_id, stream: stdout|stderr|log, chunk, chunk_seq` (giới hạn ở 2.3) |
| `gate.opened/decided/expired` | CR-WFD-004 |

Giữ nguyên tên và trường của các chủ đề NATS hiện có (`orca.workflow.step.completed`…), chỉ **thêm** trường; người nghe cũ (task-service) không vỡ. Chủ đề mới thêm cho loại mới.

### 2.2 `GetExecution` chi tiết

Proto: `WorkflowExecution` thêm `steps[]` (`step_id, type, status, attempt, wave, started_at, ended_at, skip_reason, error_code, error_message, outputs_ref`), `template_version`, `definition_hash`, `trigger`, `started_at`, `ended_at`, `row_version`. Tham số `include_steps` (mặc định `true`) và `include_outputs` (mặc định `false`; true mới trả giá trị output, có kiểm quyền và che secret). Trường cũ giữ nguyên số.

`ListExecutions` thêm bộ lọc `status`, `template_id`, `trigger_kind`, `from/to`, phân trang bằng con trỏ; **bọc envelope** `{items, nextCursor}` ở kênh mới, kênh cũ giữ mảng trần cho tương thích.

### 2.3 `StreamExecutionEvents`

- Server-streaming theo `execution_id` (và theo `tenant` cho người có quyền xem tất cả). Nguồn là bảng `workflow.execution_events` (mới) ghi trong **cùng giao dịch** với thay đổi trạng thái (mẫu outbox), nên luồng và lịch sử nhất quán.
- Resume: client gửi `after_seq`; server phát lại các sự kiện có `seq > after_seq` rồi chuyển sang nóng. Mất kết nối không mất sự kiện.
- `step.output` (nhật ký trực tiếp): chỉ phát khi executor hỗ trợ stream (`agent.execPromptStream`, `shell` stream đã có ở agent); mỗi chunk ≤ 16 KiB, ≤ 1 MiB/bước được **lưu**, phần vượt chỉ phát trực tiếp mà không lưu và đánh dấu `truncated`. Chỉ người có quyền xem execution nhận; áp che secret trước khi phát.
- Giữ lịch sử sự kiện theo `retention` (CR-WFD-010).

### 2.4 Kênh `wscompat`

| Kênh | Hành vi |
|---|---|
| `workflow.execute` | thêm `inputs`, `originTaskId`, `templateVersion?` (CR-WFD-003) |
| `workflow.getExecution` | trả chi tiết bước (2.2) |
| `workflow.listExecutions.v2` | envelope + lọc + phân trang |
| `workflow.rerun` | CR-WFD-006 2.6 |
| `workflow.template.validate` | CR-WFD-001 2.2, không lưu |
| `workflow.template.versions.list/get/restore` | CR-WFD-001 2.3 |
| `workflow.template.publish`, `workflow.approval.list/resolve` | bao các RPC đã có (BACKLOG-026) |
| `workflow.template.share.*`, `rate` | bao các RPC đã có |
| `workflow.gate.list/get/decide` | CR-WFD-004 |
| `workflow.trigger.*` | CR-WFD-007 |
| `workflow.artifact.get` | CR-WFD-003 |
| `workflow.stream.subscribe/unsubscribe` | đẩy sự kiện 2.1 tới UI qua cơ chế `PushEvent` có sẵn của `wscompat` (`StreamHandler`) |

Quy ước payload: **camelCase** như các kênh khác (nhắc lại cảnh báo snake_case ở `backend-go-architecture.md` mục 4.2); mỗi kênh có test chống lệch hình dạng, theo bài học BUG-023 (BL-TG-05).

### 2.5 Trace, metric, audit

- `root_trace_id` truyền vào mọi lệnh relay và sự kiện (hiện có ở `ExecuteRequest`, bảo đảm không bị rơi).
- Metric (Prometheus): `workflow_executions_total{status,trigger_kind}`, `workflow_step_duration_seconds{step_type,status}`, `workflow_step_attempts_total{step_type,outcome}`, `workflow_gates_open`, `workflow_queue_depth`; nhãn không chứa id người dùng hay tenant ở cấp cao.
- Audit (nhật ký bất biến theo chuẩn của nền tảng): tạo/sửa/khôi phục template, bật/tắt trigger, quyết định cổng, override cổng, chạy lại, huỷ, đọc output có `include_outputs=true`.

## 3. Tiêu chí chấp nhận

1. Gọi `StreamExecutionEvents` nhận lần lượt `execution.started`, các `step.*`, `execution.completed`, `seq` tăng liên tục; ngắt kết nối giữa chừng rồi nối lại với `after_seq` không mất và không lặp sự kiện.
2. `GetExecution` trả đủ trạng thái, số lần thử và lý do bỏ qua của từng bước mà không cần truy vấn phụ.
3. UI không còn phải poll mỗi 4 giây để biết trạng thái bước.
4. Mọi kênh ở 2.4 có test hợp đồng JSON (camelCase, đủ trường) và test từ chối khi không có quyền.
5. `include_outputs=true` bị từ chối với người không có quyền xem execution; giá trị secret được che.
6. Người nghe NATS cũ (`task-service` mirror trạng thái) vẫn chạy mà không đổi mã.
7. Sự kiện lịch sử khớp trạng thái cuối trong `step_executions` (kiểm bằng test nhất quán).

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `execution_events`; chỉ mục `(execution_id, seq)` |
| Hiệu năng | Ghi thêm một dòng sự kiện mỗi thay đổi; `step.output` có thể lớn, hạn mức ở 2.3 |
| Tương thích | Chỉ thêm trường/kênh; không đổi tên gì đang chạy |
| Rủi ro | Luồng `step.output` có thể lộ secret nếu bước in ra; che bằng bộ quy tắc của CR-WFD-010 và không đảm bảo tuyệt đối, cần tài liệu cho người viết workflow |

## 5. Chưa kiểm chứng / câu hỏi mở

- Chưa gọi thử `StreamExecutionEvents` để xác nhận trả `Unimplemented`.
- Cơ chế `StreamHandler`/`PushEvent` của `wscompat` chịu được số người xem đồng thời nào chưa đo.
- BACKLOG-021 có thật sự còn bị chặn hay đã mở: cần đọc lại các CR được nhắc trước khi đóng.
