# CR-WFD-003 — Dữ liệu và artifact: input có kiểu, output có kiểu, biến chạy, artifact có phiên bản

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-003 |
| **Tên** | `inputs[]` có kiểu và kiểm hợp lệ, `outputs[]` khai báo theo bước, kho biến chạy, bảng `workflow.run_artifacts` có phiên bản, chuyển `inputs_json` qua gateway |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 |
| **Mở khoá** | CR-WFD-002 (biểu thức cần ngữ cảnh), CR-WFD-004, 005, 007 |
| **Tác động** | `workflow-service` (`domain/interpolation.go`, `usecase/execute.go`, `wave_dispatcher.go`, migration), `api-gateway` (`channels_workflow.go`), proto, `agent` (chỉ khi artifact tham chiếu tệp trên dev server) |
| **Thay thế một phần** | CR-WF-003 (phần `inputs` và nội suy `outputs`) |

---

## 1. Bối cảnh và vấn đề

1. **Có nội suy, thiếu kiểu.** `domain.Interpolate` thay `{{now()}}`, `{{project.id}}`, `{{user.id}}`, `{{outputs.<stepId>.<path>}}`, `{{<inputKey>}}` bằng **chuỗi** trong JSON config (`WSI/domain/interpolation.go:33-73`). Số/mảng/object chỉ đi qua dạng chuỗi.
2. **Token không giải được vẫn để nguyên chữ.** Không lỗi (`interpolation.go:28-32`), nên gõ sai tên input sinh ra bước chạy với văn bản `{{inputs.typo}}` thật, rất khó phát hiện.
3. **Output chỉ ghi khi là JSON object.** Bước trả chuỗi thuần hay mảng thì mất (`wave_dispatcher.go:307-311`). Chỉ bước ở **wave sau** đọc được.
4. **Input không tới được từ UI.** `workflow.execute` ở gateway chỉ nhận `templateId, projectId, rootTraceId, requestId`, **bỏ `inputs_json` và `origin_task_id`** (`channels_workflow.go:47`). `inputs_json` có ở proto (`ExecuteRequest`) và cột DB (migration 0012) nhưng không đường vào nào cho người dùng.
5. **Không có artifact.** Output đi qua cột `step_executions.output` (JSONB); không có khái niệm tệp/văn bản lớn có phiên bản được bước sau hoặc người dùng đọc. Sự kiện NATS không mang output (`step payload` chỉ có `execution_id, step_id, step_type, status, origin_task_id`).
6. **Không có biến toàn lượt chạy.** Không chỗ nào để các bước cùng đọc/ghi một giá trị chung ngoài việc nối chuỗi output.

## 2. Giải pháp đề xuất

### 2.1 Input có kiểu

`inputs[]` (CR-WFD-001) khai báo `name, type, required, default, enum?, description?`. `Execute` kiểm:

| Quy tắc | Lỗi |
|---|---|
| Input `required` phải có (hoặc có `default`) | `WORKFLOW_INPUT_INVALID{field}` |
| Giá trị đúng kiểu khai báo; `enum` đúng tập | `WORKFLOW_INPUT_INVALID` |
| Tên input lạ (không khai báo) | `WORKFLOW_INPUT_INVALID` (không bỏ qua ngầm) |
| Kích thước `inputs_json` ≤ 256 KiB | `WORKFLOW_INPUT_INVALID` |
| Kiểu `artifact` | giá trị là `artifactId` thuộc cùng tenant, đã tồn tại (2.4) |

Input đã kiểm được lưu trong `executions.inputs_json` (đã có) **dạng chuẩn hoá** (kiểu đúng, `default` đã điền), là bản duy nhất các bước thấy.

Gateway: `workflow.execute` nhận thêm `inputs` (object) và `originTaskId` (tuỳ chọn) và chuyển nguyên xuống `ExecuteRequest`. Trường lạ trong request vẫn bị bỏ qua như hiện nay, nhưng `inputs` không bị rớt.

### 2.2 Output có kiểu và ngữ cảnh dữ liệu

- Bước khai báo `outputs[] {name,type}`; executor trả `map[string]Value`. Giá trị được kiểm theo khai báo; thiếu hoặc sai kiểu → bước `failed` (`WORKFLOW_STEP_OUTPUT_INVALID`).
- Lưu vào `step_executions.output` như hiện nay nhưng **bắt buộc là object theo `outputs[]`** (không còn "chỉ ghi nếu parse được").
- Ngữ cảnh biểu thức/nội suy (README 5.3) dựng một lần cho mỗi bước ngay trước khi chạy: `inputs`, `steps.<id>.{status,outputs}` của **mọi tổ tiên đã xong** (không còn giới hạn wave sau), `run`, `project`, `user`.
- `{{path}}` trong `config`:
  - Vẫn là nội suy chuỗi (tương thích).
  - Khi cả giá trị JSON là **một token duy nhất** (`"count": "{{steps.a.outputs.n}}"`) thì thay bằng **giá trị có kiểu** (số, mảng, object), không bọc chuỗi.
  - Token không giải được: ở `schemaVersion=2` là **lỗi** `WORKFLOW_EXPRESSION_INVALID` (bước `failed`); ở `schemaVersion=1` giữ để nguyên chữ (CR-WFD-011).
- Giới hạn: output một bước ≤ 1 MiB, tổng output một execution ≤ 16 MiB; vượt thì bước `failed`, kèm gợi ý dùng artifact.

### 2.3 Biến chạy

Không thêm kho biến thay đổi được (tránh phụ thuộc thứ tự trong cùng wave). Thay vào đó:

- Giá trị dùng chung = **input** (bất biến) hoặc **output của bước tổ tiên** (bất biến sau khi xong).
- Bước muốn "cập nhật một giá trị" thì sinh output mới và bước sau đọc output đó. Đây là quyết định có chủ ý để lượt chạy tái lập được và không có tranh chấp.

### 2.4 Artifact có phiên bản

Bảng `workflow.run_artifacts` (mới):

| Cột | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `execution_id` | uuid | NOT NULL, tạo bởi lượt chạy nào |
| `step_id` | varchar(64) | NULL nếu là input tải lên |
| `name` | varchar(128) | tên logic, ví dụ `report` |
| `version` | int | tăng theo `(tenant_id, execution_id, name)`, từ 1 |
| `media_type` | varchar(100) | `text/markdown`, `application/json`, … |
| `storage` | varchar(12) | `inline` hoặc `host_file` |
| `inline_body` | text/bytea | khi `inline`, ≤ 256 KiB (O2) |
| `host_ref` | jsonb/json | khi `host_file`: `{devServerId, path}` — **không sao chép nội dung** |
| `size_bytes` | bigint | |
| `sha256` | char(64) | băm nội dung lúc ghi |
| `created_at` | timestamptz | đồng hồ DB |

Bất biến: một dòng không bao giờ bị sửa; "cập nhật" là version mới. Duy nhất `(tenant_id, execution_id, name, version)`. Postgres bật RLS theo `tenant_id`; MySQL lọc `tenant_id` ở mọi `WHERE` (như `workflow-service` hiện có).

- Bước tham chiếu artifact bằng giá trị kiểu `artifact` = `{artifactId, name, version, sha256}`. Đọc nội dung qua RPC `GetArtifact` (kiểm quyền, kiểm `sha256` với `host_file`; sai hash → `WORKFLOW_ARTIFACT_CHANGED`).
- `host_file` chỉ chấp nhận đường dẫn nằm trong thư mục làm việc của execution trên dev server và đã qua danh sách cho phép (CR-WFD-010); đọc/băm do agent thực hiện (RPC đọc tệp đã có `fs.*`, **không thêm RPC agent mới nếu đủ dùng** — cần kiểm tra).
- Artifact kế thừa vòng đời execution: dọn theo `retention` của tenant (CR-WFD-010).
- Artifact được đưa vào **sự kiện** dưới dạng tham chiếu (id, name, version, sha256), không chứa nội dung (2.5).

### 2.5 Sự kiện mang tham chiếu

Payload `orca.workflow.step.completed` thêm `outputs_ref` (danh sách `{name,type}` và `artifacts[]`), vẫn **không** nhúng giá trị lớn. Trường cũ giữ nguyên. Người nghe cần giá trị gọi `GetExecution` / `GetArtifact`.

## 3. Tiêu chí chấp nhận

1. Chạy workflow có input `required` thiếu: bị từ chối với `WORKFLOW_INPUT_INVALID` chỉ rõ trường; input lạ cũng bị từ chối.
2. Input số `3` đến bước dưới dạng số khi token là toàn bộ giá trị JSON, dạng chuỗi khi nằm giữa văn bản.
3. Gõ sai tên input trong `config` của template v2: lưu bị từ chối (tham chiếu không tồn tại), không đợi đến lúc chạy.
4. Bước sau đọc được output của bước tổ tiên ở **cùng wave logic** khi có phụ thuộc, và **không** đọc được output của bước không phải tổ tiên.
5. Output sai kiểu khai báo làm bước `failed` với mã `WORKFLOW_STEP_OUTPUT_INVALID`.
6. Ghi lại artifact cùng `name` hai lần tạo version 1 và 2; version 1 vẫn đọc được nguyên văn.
7. `host_file` bị sửa sau khi ghi: `GetArtifact` trả `WORKFLOW_ARTIFACT_CHANGED`.
8. Gateway chuyển `inputs`; test chứng minh `inputs` tới `ExecuteRequest.inputs_json`.
9. Test hai dialect cho `run_artifacts`; test RLS chặn đọc chéo tenant.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `run_artifacts`; không đổi dữ liệu cũ |
| Tương thích | `schemaVersion=1` giữ nội suy cũ (chuỗi, không lỗi khi thiếu); chỉ v2 áp quy tắc nghiêm |
| Bảo mật | Artifact có thể chứa dữ liệu nhạy cảm: kiểm quyền theo template/execution, che secret (CR-WFD-010) |
| Dung lượng | `inline_body` làm phình bảng; hạn mức theo tenant và dọn theo retention |
| Rủi ro | `host_file` phụ thuộc dev server còn online và tệp còn tồn tại; chấp nhận, lỗi rõ `WORKFLOW_ARTIFACT_UNAVAILABLE` |

## 5. Chưa kiểm chứng / câu hỏi mở

- Agent hiện có RPC đọc/băm tệp đủ cho `host_file` hay cần RPC mới (cần đọc `agent/src/relay` phần `fs.*`).
- Ngưỡng 256 KiB / 1 MiB / 16 MiB là đề xuất, chưa so với kích thước output thực tế của các bước agent.
- Việc chuẩn hoá `inputs_json` có thể làm lệch với task-service (gọi `Execute` không input); cần đối chiếu ở CR-WFD-011.
