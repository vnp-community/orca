# CR-WFD-007 — Trigger: thủ công có input, lịch, sự kiện, webhook, workflow hoàn tất

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-007 |
| **Tên** | Bảng `workflow.triggers`, bộ khởi chạy hợp nhất cho mọi nguồn (thủ công, lịch, sự kiện NATS, webhook, workflow hoàn tất), ánh xạ payload → input, chống chạy trùng |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 (ghim phiên bản), CR-WFD-003 (input có kiểu) |
| **Mở khoá** | CR-WFD-009 (màn hình trigger) |
| **Tác động** | `workflow-service` (domain, usecase `FireTrigger`, scheduler, consumer NATS, HTTP webhook), `api-gateway` (kênh `workflow.trigger.*`, đường nhận webhook), `automation-service` (dùng lại bộ lập lịch hoặc chuyển giao), `task-service` (nơi gắn workflow) |

---

## 1. Bối cảnh và vấn đề

1. **Chỉ hai đường khởi chạy template.** (a) Thủ công qua kênh `workflow.execute`, **bỏ mất `inputs_json`** (`wscompat/channels_workflow.go:47`); (b) gắn vào task: `Task.WorkflowTemplateID` đặt qua `UpdateTask`, `ExecuteTask` gọi `Execute` **không input, không project, không trace** (`task-service/.../workflow_executor.go:34-41`).
2. **`automation-service` có trigger nhưng không chạy template.** Có cron/rrule, sự kiện (`agent:completed, agent:error, worktree:created, pr:merged, issue:assigned`) và trigger ngoài; nhưng mỗi hành động gọi `ExecuteAdHocStep` cho **một bước lẻ**, không khởi chạy workflow (`automation-service/.../execute_automation_chain.go:220-229`). Chưa kiểm chứng các trigger sự kiện chạy đầu-cuối đến đâu.
3. **Không có webhook vào workflow.** Hệ thống ngoài không thể kích hoạt một workflow.
4. **Workflow không kích hoạt được workflow.** Không có loại bước hay trigger "khi workflow X hoàn tất".
5. **Không chống chạy trùng.** Sự kiện NATS giao lặp, lịch chạy trùng khi nhiều bản service, hay người dùng bấm hai lần đều tạo thêm execution.

## 2. Giải pháp đề xuất

### 2.1 Mô hình trigger

Bảng `workflow.triggers` (mới):

| Cột | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `template_id` | uuid | NOT NULL |
| `pinned_version` | bigint | NULL = luôn dùng bản mới nhất **tại thời điểm bắn**; có giá trị = ghim |
| `kind` | varchar(16) | `schedule`, `event`, `webhook`, `workflow` (thủ công không cần dòng) |
| `config` | jsonb/json | theo `kind`, kiểm lúc lưu |
| `input_mapping` | jsonb/json | ánh xạ payload → `inputs` (2.3) |
| `enabled` | boolean | mặc định `false` khi tạo, bật tường minh |
| `dedupe_window_sec` | int | 0…86 400, mặc định 300 |
| `created_by`, `created_at`, `updated_at` | | |
| `run_as` | uuid | danh tính chạy (2.5) |

Giới hạn: 50 trigger/template, 500/tenant (cấu hình).

### 2.2 Các loại

| `kind` | `config` | Nguồn |
|---|---|---|
| `schedule` | `{ "cron": "…", "timezone": "Asia/Ho_Chi_Minh", "misfire": "skip|fire_once" }` | Bộ lập lịch trong `workflow-service` với **khoá lãnh đạo** (advisory lock) để chỉ một bản bắn; `misfire=fire_once` bù đúng một lần sau gián đoạn |
| `event` | `{ "subject": "orca.<…>", "filter": "<biểu thức>" }` | Consumer JetStream durable; `filter` theo ngữ pháp biểu thức của CR-WFD-002 trên `event.*` |
| `webhook` | `{ "secretRef": "…", "method": "POST", "maxBodyKiB": 256 }` | URL cố định `…/workflow/hooks/{triggerId}`; xác thực HMAC (`X-Orca-Signature`) bằng secret lưu ở Vault, so sánh hằng thời gian; có chống phát lại bằng `X-Orca-Timestamp` ± 5 phút |
| `workflow` | `{ "sourceTemplateId": "…", "on": ["completed","failed"] }` | Nghe `orca.workflow.execution.completed/failed` của template nguồn; payload = tóm tắt execution nguồn |

Thủ công: `workflow.execute` nhận `inputs` (đã sửa ở CR-WFD-003) và `triggerSource="manual"`.

### 2.3 Ánh xạ payload → input

`input_mapping` là object `inputName → expression` đánh giá trên ngữ cảnh `event`/`payload`/`trigger` (cùng ngữ pháp CR-WFD-002):

```jsonc
{ "target": "payload.item.id", "note": "'auto: ' + trigger.name" }
```

Kết quả được kiểm theo `inputs[]` của phiên bản đã ghim (CR-WFD-003). Thiếu input `required` hoặc sai kiểu → **không tạo execution**, ghi một dòng `trigger_firings` kèm `reason=input_invalid`; không retry vô hạn.

### 2.4 Chống chạy trùng và quan sát

- Mỗi lần bắn có `dedupe_key`: `event` = `subject + id sự kiện` hoặc băm payload; `schedule` = `triggerId + thời điểm kế hoạch`; `webhook` = header `Idempotency-Key` nếu có, ngược lại băm thân; `workflow` = `execution_id` nguồn.
- Bảng `workflow.trigger_firings(id, trigger_id, dedupe_key, fired_at, outcome, execution_id, reason)` với duy nhất `(trigger_id, dedupe_key)` trong cửa sổ `dedupe_window_sec` (dọn theo retention). Bắn trùng ghi `outcome=deduplicated`, không tạo execution.
- Giới hạn tốc độ theo trigger (mặc định 60 lần/phút; vượt thì `outcome=throttled`).
- Chống vòng: trigger `workflow` mang `depth` (số lần kích hoạt dây chuyền); vượt 5 tầng bị chặn (`outcome=loop_guard`) và tạo cảnh báo.

### 2.5 Danh tính và quyền

- `run_as` mặc định là người tạo trigger; execution do trigger sinh ra mang danh tính này cho mọi kiểm quyền (action, workspace, cổng), **không** dùng quyền service.
- Nếu người đó mất quyền (rời tenant, bị gỡ quyền chạy template), trigger tự chuyển `enabled=false` với `reason=run_as_revoked` và báo cho người tạo và admin.
- Webhook: người gọi chỉ cần chữ ký hợp lệ; quyền do `run_as` quyết định.

### 2.6 Khởi chạy từ task

`task-service` gọi `Execute` có `origin_task_id` như hiện nay; CR này chỉ yêu cầu bổ sung **chuyển được `inputs`** (cấu hình trên task) và `project_id`/`root_trace_id` (hiện bỏ trống). Chi tiết ở CR-WFD-011.

### 2.7 Quan hệ với `automation-service`

Không thay `automation-service`. Đề xuất hai điều:

1. Thêm loại hành động `run_workflow{templateId, version?, inputs}` vào chuỗi hành động của automation, để automation (với trigger lịch/sự kiện sẵn có) khởi chạy được template.
2. Trigger mới ở `workflow-service` dành cho nhu cầu **chỉ cần chạy workflow**; ai đã có automation thì dùng (1) để khỏi lặp bộ lập lịch.

Quyết định ranh giới cuối cùng giữa hai nơi cần xác nhận khi duyệt (chưa kiểm chứng mức trưởng thành của trigger sự kiện ở automation).

## 3. Tiêu chí chấp nhận

1. Trigger `schedule` bắn đúng múi giờ; với hai bản service chạy đồng thời chỉ một execution mỗi mốc.
2. Sự kiện NATS giao hai lần chỉ tạo một execution (`deduplicated` cho lần hai).
3. Webhook sai chữ ký hoặc `X-Orca-Timestamp` quá hạn bị từ chối `401` và không ghi execution; đúng chữ ký tạo execution với `inputs` ánh xạ đúng.
4. Payload không đủ input `required`: không tạo execution, `trigger_firings.reason=input_invalid`.
5. Trigger `workflow` A→B→A dừng ở 5 tầng với `loop_guard`.
6. `run_as` bị gỡ quyền làm trigger tự tắt và có thông báo.
7. `workflow.execute` từ UI chuyển được `inputs` (kiểm bằng test gateway).
8. Test hai dialect cho `triggers` và `trigger_firings`.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `triggers`, `trigger_firings` |
| Bảo mật | Webhook công khai là bề mặt tấn công mới: HMAC, chống phát lại, giới hạn kích thước/tốc độ, không trả thông tin chi tiết khi lỗi |
| Vận hành | Bộ lập lịch cần khoá lãnh đạo; bản service bị treo giữ khoá cần có hết hạn (lease) |
| Rủi ro | Trùng chức năng với `automation-service`; xem 2.7 |

## 5. Chưa kiểm chứng / câu hỏi mở

- Cơ chế lãnh đạo dùng được ở cả Postgres và MySQL (advisory lock có ở Postgres; MySQL dùng `GET_LOCK` hoặc bảng lease) chưa chốt.
- Danh mục chủ đề NATS mà người dùng được phép nghe (không nên mở mọi chủ đề): cần danh sách cho phép, chốt ở CR-WFD-010.
- Có cho `schedule` tới mức giây hay tối thiểu một phút: mặc định tối thiểu một phút.
