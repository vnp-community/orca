# CR-WFD-011 — Di trú và tương thích: template v1, hai engine TS/Go, task engine 3, automation

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-011 |
| **Tên** | Chạy song song `schemaVersion` 1 và 2, backfill phiên bản, chế độ cảnh báo cho template cũ, nâng cấp `task-service` truyền input/project/trace, đối chiếu engine TS, kế hoạch bật và quay lui |
| **Loại** | Migration + tương thích |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 (lược đồ, bảng phiên bản), CR-WFD-010 (cờ) |
| **Tác động** | `workflow-service` (đường chạy cũ giữ nguyên, bộ chuyển đổi), `task-service` (`adapter/grpcclient/workflow_executor.go`, `AttachWorkflowTemplateAction`), `automation-service`, `frontend`, tài liệu vận hành |
| **Liên quan** | ADR-024 (hai engine workflow song song), CR-FLOW-TASK-001/002 (task engine 3) |

---

## 1. Bối cảnh và vấn đề

1. **Có template và execution đang tồn tại** ở định dạng hiện tại (`dag_json = {steps:[…]}`, không `schemaVersion`). Bất kỳ thay đổi phá vỡ nào cũng làm hỏng chúng.
2. **`task-service` là một client thật của workflow.** `Task.WorkflowTemplateID` chỉ đặt qua `UpdateTask` (không lúc tạo), `ExecuteTask` gọi `Execute` với `OriginTaskId` nhưng **không input, không project, không trace**, và nghe `orca.workflow.step.completed` để phản chiếu trạng thái (`task-service/internal/domain/task.go:140-150`, `adapter/grpcclient/workflow_executor.go:34-41`, `adapter/eventbus/consumer.go:31-34`). Mọi thay đổi sự kiện/trạng thái phải giữ hợp đồng này.
3. **Hai engine song song.** ADR-024 ghi engine TypeScript và engine Go chạy song song và **không tương thích**; docs `logic/` mô tả engine TS (`backend/src/main/workflow/`). Một workflow tạo ở Electron/TS không hiện ở chế độ web/Go (và ngược lại), theo `backend-go-architecture.md` mục "Ai gọi tới nó".
4. **Kiểu `approval` của UI** đã được lưu trong template cũ (BACKLOG-027) dù backend không thực thi; di trú cần quyết định số phận chúng.
5. **Sự kiện NATS và proto** là hợp đồng với người nghe khác; CR-WFD-008 đã nêu nguyên tắc chỉ thêm. CR này kiểm tra nguyên tắc đó cho từng người nghe.

## 2. Giải pháp đề xuất

### 2.1 Hai đường chạy cùng tồn tại

| `schemaVersion` | Dispatcher | Hành vi |
|---|---|---|
| `1` hoặc thiếu | **Đường cũ giữ nguyên** (wave, nội suy chuỗi, `condition` không rẽ nhánh, lỗi một bước = hỏng cả luồng) | Không thay đổi nghĩa; không áp quy tắc nghiêm của v2 |
| `2` | Đường mới (CR-WFD-002/003/006) | Chỉ khi cờ `workflow_engine_v2` bật cho tenant |

- Lựa chọn đường theo `schemaVersion` **của phiên bản đã ghim**, không theo cờ lúc chạy, nên một lượt chạy không đổi đường giữa chừng.
- Giữ mã đường cũ trong một gói riêng (`usecase/legacydispatch`) với test hiện có; không dùng lại các hàm đã đổi hành vi. Xoá đường cũ chỉ khi không còn template `schemaVersion=1` được chạy trong một khoảng quan sát thoả thuận (tối thiểu hai chu kỳ phát hành) và đã có công cụ nâng cấp (2.3).
- `Execute` trên template v1 khi cờ tắt hoặc bật: **cùng hành vi**. Tính năng v2 (gate, `when`, artifact, trigger…) chỉ khả dụng ở template v2.

### 2.2 Backfill phiên bản (CR-WFD-001)

Migration một lần, chạy lặp lại an toàn:

1. Với mỗi `workflow.templates` chưa có dòng `template_versions`: chèn `(template_id, version=templates.version, definition=dag_json, definition_hash)`.
2. Với mỗi `executions` chưa có `template_version`: đặt = phiên bản hiện tại của template **nếu `created_at` execution ≥ `updated_at` template**, ngược lại đặt `NULL` và `definition_hash=NULL` (không tái dựng ngược lịch sử). Execution `NULL` vẫn đọc được qua snapshot hiện có; UI hiển thị "phiên bản không rõ".
3. Không đổi `dag_json`.

### 2.3 Công cụ nâng cấp v1 → v2

`workflow.template.upgrade{templateId}` (và nút trong UI): sinh **bản nháp v2** (không tự lưu) từ v1 bằng các quy tắc **bảo toàn nghĩa**:

| Ở v1 | Ở v2 |
|---|---|
| bước và `dependsOn` | giữ nguyên |
| token `{{…}}` | giữ; `ValidateDefinition` báo tham chiếu không tồn tại |
| bước `condition` | giữ (deprecated) + cảnh báo "kết quả không điều khiển luồng"; **không** tự đoán nhánh |
| kiểu `approval` của UI | chuyển thành bước `gate` với cấu hình mặc định (`approvers` rỗng → bắt buộc người dùng điền trước khi lưu) |
| `continueOnError`/`timeout` của UI nếu có | chuyển sang `onError=continue`/`timeoutMs` **và cảnh báo rằng trước đây backend bỏ qua** các trường này, nên hành vi sẽ khác khi chạy |
| bước `shell/agent` không khai output | thêm `outputs` mặc định `{stdout}`/`{result}` theo `OutputSchema()` |

Người dùng xem khác biệt rồi mới lưu; bản v1 vẫn nguyên trong lịch sử.

### 2.4 `task-service` (engine 3)

Thay đổi tối thiểu, **tương thích ngược**:

1. `workflow_executor.go` truyền thêm `project_id`, `root_trace_id` và `inputs` (lấy từ cấu hình đính kèm trên task) vào `ExecuteRequest`; trường rỗng như hiện nay nếu task không cấu hình.
2. Chọn `template_version`: mặc định bản mới nhất tại thời điểm chạy; task có thể ghim.
3. Consumer `step.completed` không đổi; các sự kiện mới (`step.skipped`, `step.waiting`, `gate.*`) **bị bỏ qua** an toàn (không crash) cho tới khi task-service chọn dùng. Cần test xác nhận người nghe bỏ qua loại lạ.
4. Task có workflow chứa cổng: trạng thái task phản chiếu `waiting` thành trạng thái hiển thị phù hợp của task (không đổi enum trạng thái task trong CR này; ánh xạ `waiting → in_progress` kèm nhãn, chốt khi duyệt).

### 2.5 `automation-service`

Giữ `ExecuteAdHocStep`. Thêm hành động `run_workflow` (CR-WFD-007 2.7) là **bổ sung**; không đổi chuỗi hành động hiện có. Trigger đang chạy không bị ảnh hưởng.

### 2.6 Engine TypeScript (ADR-024)

- CR này **không** thêm tương thích TS↔Go. Quy tắc: template do engine TS tạo vẫn thuộc engine TS; nâng cấp lên v2 chỉ có ở đường Go.
- Cần một mục trong tài liệu vận hành nói rõ engine nào chạy ở chế độ nào, và UI không hiển thị tính năng v2 khi đang nối engine TS (kiểm khả năng bằng `workflow.capabilities`, kênh trả `{engine: "go|ts", schemaVersions: [1,2], features: [...]}`).
- Việc hợp nhất hai engine thành một là quyết định kiến trúc riêng, ngoài phạm vi.

### 2.7 Kế hoạch bật và quay lui

| Giai đoạn | Nội dung | Điều kiện sang giai đoạn sau |
|---|---|---|
| 0 | Triển khai mã, cờ tắt toàn bộ | Test hai đường qua CI |
| 1 | Backfill phiên bản (2.2) | Đối soát số dòng; không lỗi |
| 2 | Bật `workflow_engine_v2` cho tenant thử nghiệm nội bộ | Chạy bộ test hợp đồng thật với agent thật; theo dõi metric lỗi/độ trễ |
| 3 | Bật dần theo tenant | Không tăng lỗi `v1` (đối chiếu trước/sau) |
| 4 | Công bố công cụ nâng cấp (2.3) | Phản hồi người dùng |

**Quay lui:** tắt cờ tenant → template v2 không chạy mới (`WORKFLOW_DISABLED`), execution v2 đang dở **được chạy nốt** bởi đường mới (không bỏ rơi), template v1 không bị ảnh hưởng. Bảng/cột thêm vào là cộng thêm nên không cần xoá khi quay lui; migration chỉ cộng, không xoá.

## 3. Tiêu chí chấp nhận

1. Bộ test hiện có của `workflow-service` vẫn xanh trên đường cũ không sửa mã kiểm.
2. Một template v1 chạy trước và sau khi triển khai (cờ tắt và bật) cho **cùng kết quả**, cùng số bước, cùng sự kiện.
3. Backfill chạy hai lần liên tiếp không tạo dòng trùng và không thay đổi dữ liệu.
4. Task gắn workflow chạy được như cũ; thêm `inputs` từ task tới `ExecuteRequest.inputs_json`; sự kiện mới không làm consumer của `task-service` lỗi.
5. `workflow.template.upgrade` trên template mẫu có `condition`, `approval`, `continueOnError` sinh bản nháp có cảnh báo đúng và **không tự lưu**.
6. Tắt cờ khi có execution v2 đang `running`: execution đó vẫn kết thúc đúng; execution mới bị từ chối.
7. `workflow.capabilities` trả đúng cho Go; UI ẩn tính năng v2 khi nối engine không hỗ trợ.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Chi phí | Duy trì hai đường chạy là gánh nặng; giới hạn thời gian sống của đường cũ (2.1) |
| Rủi ro | Template v1 có hành vi "may mắn" (ví dụ token sai tên vẫn chạy) sẽ đổi khi nâng cấp; công cụ nâng cấp phải cảnh báo, không đoán |
| Rủi ro | Backfill gán sai phiên bản cho execution cũ; đã chọn bảo thủ (NULL khi không chắc) |
| Rủi ro | Ánh xạ `waiting → in_progress` ở task có thể gây hiểu nhầm; chốt khi duyệt |

## 5. Chưa kiểm chứng / câu hỏi mở

- Chưa đọc `AttachWorkflowTemplateAction` và nơi lưu cấu hình đính kèm trên task để biết chỗ đặt `inputs`.
- Số template/execution v1 thực tế đang tồn tại (ảnh hưởng chi phí backfill) chưa đếm.
- Trạng thái dùng engine TS ở môi trường người dùng thực chưa rõ; ảnh hưởng đến mức ưu tiên của `workflow.capabilities`.
- Khoảng quan sát trước khi xoá đường cũ ("hai chu kỳ phát hành") là đề xuất, cần chốt.
