# CR-WFD-001 — Lược đồ định nghĩa workflow v2: điều khiển bước, kiểm tra khi lưu, phiên bản bất biến

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-001 |
| **Tên** | Định nghĩa workflow v2 (`schemaVersion: 2`), bảng `workflow.template_versions` bất biến, kiểm tra cấu hình theo loại bước lúc lưu, ghim phiên bản vào lượt chạy |
| **Loại** | Feature (nền của series) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | — |
| **Mở khoá** | CR-WFD-002, 003, 004, 005, 006, 007, 010, 011 |
| **Tác động** | `workflow-service` (domain `dag.go`, `step.go`, `template.go`; usecase `create_template`, `update_template`, `execute`; migration hai dialect; proto `workflow.proto`), `api-gateway` (`channels_workflow.go`), `frontend` (`shared/workflow-types.ts`) |
| **Thay thế một phần** | CR-WF-004 (chỉ phần "tăng version có điều kiện") |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-08:

1. **Bước không có chỗ cho điều khiển.** `Step{id,type,config,dependsOn[]}` (`WSI/domain/dag.go:39-44`). Không có guard, retry, timeout, cách xử lý lỗi, khai báo output. Mọi hành vi ngoài "chạy rồi lỗi thì dừng hết" đều không biểu diễn được.
2. **`config` là JSON thô và chỉ kiểm cấu trúc DAG.** `NewWorkflowTemplate` gọi `dag.Validate` (chu trình, tham chiếu `dependsOn`); không kiểm cấu hình theo loại bước (chưa kiểm chứng: có nơi nào khác kiểm hay không). Lỗi cấu hình chỉ lộ lúc chạy.
3. **Phiên bản chỉ là bộ đếm ghi đè.** `Version` tăng mỗi `UpdateTemplate`, không có lịch sử, không ghim, không quay lui (`WSI/domain/template.go:54-58,92-95`). UI gửi `expectedVersion=0` (`useWorkflow.ts:70-82`) nên khoá lạc quan thực tế vô hiệu, ghi sau thắng.
4. **Lượt chạy tách khỏi định nghĩa.** README nói execution "đóng băng DAG"; nhưng không có khoá ngoại tới một phiên bản bất biến, nên không truy vết được "lượt chạy này dùng bản nào" sau khi template bị sửa (chưa kiểm chứng chi tiết cột nào chứa snapshot).
5. **Hai kiểu dữ liệu song song.** UI có kiểu `AgentStepConfig`, `ShellStepConfig`… khác cấu hình backend (UI `shell` có `command,args,cwd`; backend nhận `script,env`), ghi ở BUG-FE-TASKV1-007 (chưa kiểm chứng đã sửa chưa).

Vấn đề: không có một **hợp đồng định nghĩa duy nhất, có kiểu, có phiên bản** để mọi CR sau dựa vào.

## 2. Giải pháp đề xuất

### 2.1 Tài liệu định nghĩa v2

Giữ `dag_json` làm nơi chứa, thêm `schemaVersion`. Hình dạng đầy đủ ở [README mục 5.1](./README.md#51-định-nghĩa-v2-rút-gọn-chi-tiết-ở-cr-wfd-001). Quy tắc:

| Trường | Quy tắc |
|---|---|
| `schemaVersion` | `1` hoặc `2`. Thiếu = `1` (CR-WFD-011). |
| `inputs[]` | Tên duy nhất, `[a-z][a-z0-9_]{0,63}`; kiểu ∈ `string, number, boolean, json, artifact` (CR-WFD-003). Tối đa 64. |
| `steps[]` | Tối đa 200; `id` duy nhất `[a-z][a-z0-9_-]{0,63}`; `type` phải có trong sổ đăng ký (CR-WFD-005). |
| `steps[].dependsOn` | Tham chiếu `id` có thật, không chu trình, không tự tham chiếu (đã có). |
| `steps[].when` | Biểu thức bool (CR-WFD-002); thiếu = luôn chạy. |
| `steps[].retry`, `timeoutMs`, `onError` | CR-WFD-006. Mặc định `retry.max=0`, `onError="fail"`, timeout theo loại bước. |
| `steps[].outputs[]` | Khai báo tên và kiểu output mà bước hứa sinh ra; tham chiếu `steps.<id>.outputs.<name>` bị kiểm lúc lưu. |
| `outputs[]` | Output của cả workflow, chỉ được tham chiếu `steps.*.outputs.*` đã khai báo. |
| Kích thước | Toàn tài liệu ≤ 512 KiB, UTF-8 hợp lệ. |

### 2.2 Kiểm tra khi lưu (`ValidateDefinition`)

Hàm thuần `domain.ValidateDefinition(def, registry) []Diagnostic`, không I/O. Mỗi `Diagnostic{path, code, severity: error|warning, message}`. Lưu chỉ thất bại khi có `error`.

| Nhóm | Kiểm tra |
|---|---|
| Cấu trúc | Chu trình, `dependsOn` mồ côi, id trùng, vượt giới hạn |
| Loại bước | `type` có trong sổ đăng ký; `config` hợp lệ theo **schema của loại bước** do executor cung cấp (CR-WFD-005 `ConfigSchema()`) |
| Biểu thức | Cú pháp đúng; tham chiếu `steps.<id>` phải là **tổ tiên** của bước chứa biểu thức (tránh đọc output chưa có) |
| Dữ liệu | Mọi tham chiếu `inputs.*`, `steps.*.outputs.*` tồn tại và đúng kiểu có thể suy ra |
| Cảnh báo | Bước không có đường tới output nào; `onError=continue` trên bước mà bước sau đọc output của nó |

API `workflow.template.validate` (CR-WFD-008) trả danh sách `Diagnostic` mà không lưu, để UI báo lỗi theo dòng.

### 2.3 Phiên bản bất biến

Bảng `workflow.template_versions` (mới):

| Cột | Kiểu | Ghi chú |
|---|---|---|
| `template_id` | uuid | PK phần 1 |
| `version` | bigint | PK phần 2, tăng từ 1 |
| `tenant_id` | uuid | NOT NULL |
| `definition` | jsonb/json | NOT NULL, bất biến |
| `definition_hash` | char(64) | SHA-256 của dạng chuẩn hoá (khoá đã sắp, không khoảng trắng) |
| `created_by` | uuid | NOT NULL |
| `created_at` | timestamptz | đồng hồ DB |
| `note` | varchar(500) | ghi chú thay đổi, tuỳ chọn |

- Ghi: `UpdateTemplate` **chèn một dòng mới** và cập nhật `templates.version` trong cùng giao dịch; bản cũ không bao giờ bị sửa hay xoá. Nếu `definition_hash` trùng bản mới nhất thì **không tăng version** (thay đổi vô nghĩa, giữ ý của CR-WF-004).
- `expectedVersion` **bắt buộc** (`0` không còn nghĩa "bỏ qua"); sai thì `WORKFLOW_VERSION_CONFLICT`. Frontend phải gửi version đã đọc (CR-WFD-009).
- `templates.dag_json` giữ bản mới nhất để tương thích đọc; nguồn sự thật là `template_versions`.
- RPC mới `ListTemplateVersions`, `GetTemplateVersion`, `RestoreTemplateVersion` (tạo **version mới** có nội dung bản cũ, không ghi đè lịch sử).

### 2.4 Ghim phiên bản vào lượt chạy

- `executions` thêm `template_version bigint NOT NULL` và `definition_hash char(64)`; `Execute` đọc `template_versions` theo `(template_id, version)` (mặc định bản mới nhất, cho phép chỉ định) và **chạy đúng tài liệu đó**.
- Lượt chạy đang dở sau khi khởi động lại (`RecoverExecutions`) dùng đúng bản đã ghim, không đọc lại template.
- Kế thừa (`parent_template_id`, `overrides_json`, `inject_steps_json`, `remove_steps_json`): kết quả **giải kế thừa tại thời điểm `Execute`** được lưu thành `resolved_definition` trên execution, để lượt chạy không đổi khi cha đổi. Quy tắc gộp giữ như hiện tại (độ sâu 5).

### 2.5 Hợp đồng proto

Thêm `schema_version`, `template_version`, `definition_hash` vào `WorkflowTemplate` và `WorkflowExecution`; thêm RPC ở 2.2–2.3. Trường cũ giữ nguyên số, chỉ thêm số mới.

## 3. Tiêu chí chấp nhận

1. Lưu template có `config` sai theo loại bước bị từ chối với `Diagnostic` chỉ rõ đường dẫn; không còn lỗi cấu hình phát hiện lúc chạy.
2. Hai lần `UpdateTemplate` liên tiếp với `expectedVersion` cũ: lần hai nhận `WORKFLOW_VERSION_CONFLICT`.
3. Sửa template khi một lượt chạy đang dở **không** làm đổi lượt chạy đó; khởi động lại service chạy tiếp đúng bản đã ghim.
4. `RestoreTemplateVersion(v3)` tạo version mới có `definition_hash` bằng bản 3, bản 3 vẫn nguyên.
5. Cập nhật không thay đổi nội dung chuẩn hoá không tăng `version`.
6. Template `schemaVersion=1` hiện có vẫn chạy y như trước (CR-WFD-011).
7. Test bảng cho `ValidateDefinition` (mỗi mã `Diagnostic` một ca) và test hai dialect.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `template_versions`, thêm cột vào `executions`; **backfill** mỗi template hiện có thành `version` hiện tại (một dòng) để execution cũ có chỗ tham chiếu. Chạy được lặp lại an toàn. |
| Tương thích | `expectedVersion` bắt buộc là thay đổi phá vỡ với client gửi 0; cần giai đoạn chuyển tiếp: nhận 0 khi cờ `workflow_strict_versioning` tắt (mặc định tắt). |
| Hiệu năng | `Execute` đọc thêm một dòng; không đáng kể. |
| Rủi ro | Kiểm tra theo `ConfigSchema()` có thể từ chối template cũ đang "chạy được nhờ may mắn"; xử lý ở CR-WFD-011 bằng chế độ cảnh báo cho `schemaVersion=1`. |

## 5. Chưa kiểm chứng / câu hỏi mở

- Cột hiện tại của `executions` nào đang giữ snapshot DAG (tên, kiểu).
- Có nơi nào ngoài `dag.Validate` kiểm cấu hình theo loại bước hay không.
- Giới hạn 200 bước và 512 KiB là mặc định đề xuất, chưa đối chiếu với template thực tế đang có.
