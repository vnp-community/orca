# CR-WFD-005 — Executor bước: sổ đăng ký loại bước, bước `action`/`mcp_call`/`sub_workflow`, cấp worktree cho bước agent

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-005 |
| **Tên** | Giao diện `StepExecutor` có `ConfigSchema()`, sổ đăng ký thay `switch` cố định, danh mục `action` thật, bước `mcp_call`, cấp/chọn worktree cho bước `agent`, truyền `timeoutMs` xuống agent |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 (schema), CR-WFD-003 (output có kiểu) |
| **Mở khoá** | CR-WFD-002 (`sub_workflow` là một loại bước), CR-WFD-009 (editor sinh form từ schema) |
| **Tác động** | `workflow-service` (`domain/step.go`, `adapter/stepexecutors/*`, `adapter/infrafleetclient/*`, `cmd/server/main.go`), `agent` (nhận `timeoutMs`), `mcp-service` (nếu `mcp_call` đi qua chính sách MCP) |
| **Thay thế một phần** | CR-WF-003 (phần `action`) |

---

## 1. Bối cảnh và vấn đề

1. **Tập loại bước đóng.** `StepType.Valid` là `switch` biên dịch sẵn trên 9 giá trị (`WSI/domain/step.go:48-55`); thêm một loại phải sửa lõi domain, proto enum (`workflow.proto:96-110`) và dispatcher.
2. **`action` rỗng.** Executor được đăng ký không có handler nào (`main.go:241`), mọi bước `action` đều lỗi `ErrNoActionHandlerRegistered` (`stepexecutors/action.go:13-21`). CR-WF-003 mô tả nó nhưng thực tế không dùng được.
3. **Bước agent không tự cấp worktree.** `worktreePath` là chuỗi người dùng tự điền; executor không tạo/chọn worktree (`agent_step_executor.go:62-148`), nên workflow muốn mỗi bước làm việc trên một bản làm việc riêng phải tự dựng bằng bước khác.
4. **Không truyền timeout.** Các params `agent.execPrompt` và `shell.exec` không có `timeoutMs` (`agent_step_executor.go`, `shell_step_executor.go`); agent áp mặc định 5 phút (tối đa 15 phút cho agent, 5 phút cho shell — `agent-print-mode-exec.ts:30-32`, `shell-agent-extensions.ts:86-88`). Người dùng không thể đặt thời gian dài hơn dù cần.
5. **Không có cách gọi công cụ ngoài ở mức bước.** Muốn gọi một công cụ MCP hay dịch vụ nội bộ phải bọc bằng `shell`/`webhook`.
6. **Webhook chống SSRF còn là "stub"** theo README (chưa kiểm chứng chi tiết); xử lý ở CR-WFD-010.
7. **`notification` và `commit_push`** có executor nhưng tên phương thức relay ghi là "best effort" trong README, chỉ `agent.execPrompt` được xác nhận là đúng (CR-WF-001).

## 2. Giải pháp đề xuất

### 2.1 Giao diện và sổ đăng ký

```go
type StepExecutor interface {
    Type() StepType
    ConfigSchema() JSONSchema          // dùng cho ValidateDefinition và để UI sinh form
    OutputSchema(cfg json.RawMessage) []OutputDecl   // output mặc định, khi bước không tự khai báo
    Capabilities() Capabilities        // { idempotent bool, cancellable bool, maxTimeout time.Duration, needsHost bool }
    Execute(ctx context.Context, in StepInput) (StepOutput, error)
}
```

- `StepRegistry` đăng ký lúc khởi động; `StepType` đổi từ enum cứng sang **chuỗi định danh** `^[a-z][a-z0-9_]{0,31}$`. Proto giữ enum cũ cho tương thích và thêm trường `type_name string`; khi `type_name` có giá trị thì ưu tiên.
- Thêm loại bước mới = thêm một gói `stepexecutors/<tên>` và một dòng đăng ký; **không đổi dispatcher, wave, hay proto**.
- `Capabilities.idempotent` được CR-WFD-006 dùng để quyết định retry và chạy lại sau khởi động lại; `cancellable` để biết có huỷ được giữa chừng không; `maxTimeout` là trần hợp lệ của `timeoutMs`.
- Loại bước chưa đăng ký khi chạy: `WORKFLOW_STEP_TYPE_UNKNOWN` (bước `failed`).

### 2.2 Danh mục `action`

`action` trở thành bước gọi **một hàm nội bộ có tên và có schema**, không phải ô trống:

| Quy tắc | Nội dung |
|---|---|
| Đăng ký | `ActionRegistry` ánh xạ `actionName → handler` với `ConfigSchema` cho `params` và `OutputSchema` |
| Phạm vi MVP | Chỉ các hành động **chỉ đọc hoặc có phạm vi hẹp** đã có usecase sẵn trong nền tảng; danh sách cụ thể chốt khi duyệt, **không nằm trong CR này** để CR không phụ thuộc quy trình nghiệp vụ cụ thể |
| Quyền | Handler chạy với **danh tính của người khởi chạy** (không phải quyền service), qua cùng lớp kiểm quyền như gọi trực tiếp (CR-WFD-010) |
| Lỗi | `ErrNoActionHandlerRegistered` đổi thành `WORKFLOW_STEP_TYPE_UNKNOWN` kèm tên action |

### 2.3 Bước `mcp_call`

```jsonc
{ "id": "q", "type": "mcp_call",
  "config": { "server": "<tên đã đăng ký>", "tool": "<tên công cụ>", "arguments": { }, "timeoutMs": 30000 } }
```

- Gọi một công cụ của MCP server đã đăng ký trong tenant (bảng `external_servers` của `mcp-service`, chưa kiểm chứng đầy đủ khả năng gọi ra ngoài). Output = nội dung kết quả công cụ theo `OutputSchema` khai báo hoặc `result: json`.
- **Đi qua chính sách MCP của Orca** (phê duyệt, kill switch, che dữ liệu); công cụ có rủi ro cao cần phê duyệt theo chính sách hiện có, không có đường vòng.
- Mặc định **tắt** theo cờ tenant `workflow_mcp_call_enabled`; xem O2 ở CR-WFD-010.
- Loại bước này cho phép một workflow đọc/ghi hệ thống ngoài mà không cần `shell`.

### 2.4 Bước `agent`: worktree và timeout

Thêm vào `config` của `agent` (và `shell` khi có `cwd`):

| Trường | Nội dung |
|---|---|
| `workspace` | `{ "mode": "existing", "path": "..." }` (hành vi hiện tại) hoặc `{ "mode": "provision", "from": "<ref>", "name": "<mẫu tên>", "reuse": "run|step|none" }` hoặc `{ "mode": "inherit", "fromStep": "<id>" }` |
| `timeoutMs` | Truyền xuống agent; hợp lệ khi ≤ `maxTimeout` của loại bước |

- `provision` gọi RPC tạo worktree **đã có** (`git-gateway`/`project-service`), với danh tính người khởi chạy, và ghi worktree tạo ra vào output của bước (`outputs.workspacePath`, `outputs.worktreeId`). `reuse=run` dùng chung một worktree cho cả lượt chạy; `none` tạo mới mỗi lần.
- `inherit` cho bước sau làm việc tiếp trên worktree của bước trước (cần phụ thuộc rõ ràng).
- Dọn: worktree do workflow tạo được đánh dấu `origin=workflow` và dọn theo chính sách của execution (mặc định giữ để người xem kết quả; `cleanup_worktrees` đã có để dọn tường minh).
- Trần thời gian phía agent (15 phút cho agent, 5 phút cho shell) là giới hạn thật hiện nay. CR này **không đổi** trần phía agent; nếu cần dài hơn phải có CR agent riêng (chuỗi nhiều lần gọi bằng `retry`/`foreach` là cách vượt trần hợp lệ trong lúc đó). Executor trả `WORKFLOW_STEP_TIMEOUT` kèm trần thật khi cấu hình vượt trần.

### 2.5 Xác minh tên phương thức relay

Trước khi coi một loại bước là "sẵn sàng", executor phải qua **kiểm tra hợp đồng**: gọi thử phương thức relay trên một agent thật (hoặc agent giả có contract test) và so schema trả về. Các phương thức hiện ghi là "best effort" (`notification`, `commit_push`, `webhook`) phải được xác nhận hoặc sửa trước khi bật cho tenant.

## 3. Tiêu chí chấp nhận

1. Thêm một loại bước thử nghiệm bằng cách thêm một gói và một dòng đăng ký; không sửa `wave_dispatcher.go`, `step.go` hay proto.
2. `ValidateDefinition` từ chối `config` sai theo `ConfigSchema()` của loại bước và liệt kê đường dẫn lỗi.
3. Bước `action` với tên chưa đăng ký bị từ chối lúc lưu (không đợi lúc chạy); với tên đã đăng ký, handler chạy bằng danh tính người khởi chạy và bị chặn nếu người đó không có quyền.
4. Bước `agent` với `workspace.mode=provision` tạo worktree và trả `workspacePath`; bước sau dùng `inherit` làm việc trên cùng worktree.
5. `timeoutMs` vượt `maxTimeout` bị từ chối lúc lưu; trong phạm vi thì được truyền tới agent (quan sát qua tham số relay).
6. `mcp_call` bị chặn khi cờ tenant tắt (`WORKFLOW_DISABLED`) và bị chặn bởi chính sách MCP khi công cụ cần phê duyệt.
7. Contract test cho mỗi loại bước chạy trong CI với agent giả.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Proto | Thêm `type_name`; không xoá enum cũ; client cũ vẫn đọc được |
| Migration | Không đổi bảng; `templates.dag_json` giữ nguyên, `type` vẫn là chuỗi |
| Bảo mật | `action`, `mcp_call`, `provision` mở rộng khả năng tác động; bắt buộc chạy bằng danh tính người khởi chạy, không dùng quyền service |
| Rủi ro | `provision` có thể tạo nhiều worktree nếu `reuse=none` trong `foreach`; giới hạn 20 worktree/repo đã có ở `git-gateway` là chốt chặn, và phải báo lỗi rõ |
| Rủi ro | Trần 5/15 phút của agent làm các bước dài thất bại; cần tài liệu rõ và `retry` hợp lý |

## 5. Chưa kiểm chứng / câu hỏi mở

- `mcp-service` hiện có thật sự cho gọi công cụ của server ngoài từ phía backend hay chỉ phục vụ client đi vào (bảng `external_servers` mới thấy ở migration).
- Cấu hình thật của `webhook`, `notification`, `commit_push` chưa đọc.
- Danh mục `action` cụ thể cần chốt riêng; CR này chỉ cung cấp cơ chế.
- Có cần sandbox/chroot cho `shell` hay không: ngoài phạm vi, xem CR-WFD-010.
