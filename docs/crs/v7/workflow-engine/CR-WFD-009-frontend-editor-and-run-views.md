# CR-WFD-009 — Frontend: trình soạn, form input lúc chạy, theo dõi trực tiếp, cổng duyệt, lịch sử phiên bản

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-009 |
| **Tên** | Trình soạn workflow có sinh form theo schema, kiểm tra theo dòng, form input lúc chạy, màn hình chạy theo từng bước, hộp thư cổng duyệt, lịch sử phiên bản, quản lý trigger |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 … 008 (các kênh và schema) |
| **Tác động** | `frontend/src/renderer/src/components/workflow/*`, `hooks/useWorkflow*.ts`, `store/slices/workflow.ts`, `shared/workflow-types.ts`, `components/task/AttachWorkflowTemplateAction.tsx`, i18n |
| **Thay thế một phần** | CR-WF-006 (builder, library, pause/resume), BACKLOG-026 (tìm kiếm/chia sẻ/clone) |

---

## 1. Bối cảnh và vấn đề

Đã đọc code ngày 2026-10-08 (đường dẫn dưới `frontend/src/renderer/src/`):

1. **Trình soạn là form danh sách.** `StepEditor` (83 dòng) chỉ có ô chọn loại bước và hộp chọn `dependsOn`; `DAGPreview` dùng `@xyflow/react` **chỉ đọc** (`nodesDraggable={false}`).
2. **Loại bước lệch backend.** UI cho 5 loại bước backend cộng `approval` (không có backend), **không** cho `action`, `parallel`, `commit_push`, `cleanup_worktrees` (theo phần đã đọc; chưa kiểm chứng các bề mặt khác). Kiểu cấu hình của UI khác backend (shell: `command,args,cwd` so với `script,env`; BUG-FE-TASKV1-007, chưa kiểm chứng đã sửa).
3. **Lưu ghi đè.** `expectedVersion` luôn bằng 0 (`hooks/useWorkflow.ts:70-82`) nên hai người sửa cùng template sẽ ghi đè nhau.
4. **Không có input lúc chạy.** Không có form nhập `inputs`; gateway cũng bỏ `inputs_json` (CR-WFD-003).
5. **Theo dõi nghèo.** `useWorkflowExecution` poll `workflow.getExecution` mỗi 4 giây và chỉ có trạng thái cấp execution (`hooks/useWorkflowExecution.ts:7-22`); không thấy từng bước, lần thử, bước bị bỏ qua, output.
6. **Thiếu màn hình quanh vòng đời mới.** Chưa có chỗ cho cổng duyệt, lịch sử phiên bản, trigger, artifact, chạy lại.
7. **Thư viện chưa hoàn chỉnh.** Có danh sách và "áp dụng" nhưng chưa có tìm kiếm, sắp xếp, chia sẻ, clone (BACKLOG-026).

## 2. Giải pháp đề xuất

Nguyên tắc: UI là **lớp trình bày trên hợp đồng backend**; không tự định nghĩa lại kiểu. Mọi thứ về một loại bước (form, validate) đến từ `ConfigSchema()` do backend cung cấp (CR-WFD-005), nên thêm loại bước mới không cần sửa UI.

### 2.1 Kiểu dùng chung

`shared/workflow-types.ts` sinh/đồng bộ từ proto và schema backend (một nguồn): `WorkflowDefinitionV2`, `StepDefinition`, `StepType` (chuỗi, danh sách lấy từ API `workflow.stepTypes.list`), `InputDecl`, `Diagnostic`, `ExecutionDetail`, `StepRun`, `Gate`, `Artifact`. Xoá các kiểu cấu hình cứng (`AgentStepConfig`…) thay bằng schema động. Test phát hiện lệch giữa kiểu UI và proto.

### 2.2 Trình soạn

| Thành phần | Nội dung |
|---|---|
| Canvas | Đồ thị **chỉnh sửa được** (`@xyflow/react` đã có): thêm/xoá/nối bước, kéo thả, tự sắp xếp tầng; không thêm thư viện bố cục mới ở MVP (theo tinh thần O5 của v7) |
| Bảng thuộc tính | Form sinh từ `ConfigSchema()` của loại bước + các trường chung (`when`, `retry`, `timeoutMs`, `onError`, `outputs`) |
| Biểu thức | Ô nhập có tô màu, gợi ý đường dẫn (`inputs.*`, `steps.<tổ tiên>.outputs.*`), báo lỗi theo dòng từ `workflow.template.validate` |
| Chẩn đoán | Danh sách `Diagnostic` bấm để nhảy tới bước; chặn nút Lưu khi có `error` |
| Input | Trình soạn khai báo `inputs[]` có kiểu và giá trị mặc định |
| Chế độ JSON | Xem/sửa định nghĩa thô (nâng cao), dùng cùng validate |
| Lưu | Gửi `expectedVersion` thật; xung đột → hộp thoại "bản mới hơn đã có" cho phép xem khác biệt và chọn; **không** ghi đè mặc định |
| Phiên bản | Lịch sử (CR-WFD-001 2.3): xem, so sánh hai bản, khôi phục |

### 2.3 Chạy

- Nút Chạy mở **form input** sinh từ `inputs[]` (đúng kiểu, bắt buộc, enum, giá trị mặc định); gửi `workflow.execute` với `inputs`. Chọn phiên bản (mặc định mới nhất).
- Từ task: `AttachWorkflowTemplateAction` thêm cấu hình input cho lần chạy gắn với task (CR-WFD-007 2.6).

### 2.4 Theo dõi

- Màn hình execution: đồ thị chỉ đọc tô màu theo trạng thái bước (`pending, running, waiting, completed, failed, skipped, cancelled`), số lần thử, thời gian; chọn bước xem log trực tiếp (`step.output`), output, artifact, lỗi.
- Nguồn dữ liệu là `workflow.stream.subscribe` (CR-WFD-008); **polling chỉ là phương án dự phòng** khi kết nối stream thất bại, và báo rõ trạng thái "đang dùng chế độ chậm".
- Hành động theo trạng thái: tạm dừng, tiếp tục, huỷ, chạy lại từ bước lỗi (`workflow.rerun`); bước `non-cancellable` đang chạy được ghi chú trung thực.
- Trạng thái lạ (client cũ chưa biết) hiển thị trung tính, không làm vỡ màn hình.

### 2.5 Cổng duyệt

- Hộp thư "Chờ tôi duyệt": danh sách cổng `open` mà người dùng có quyền quyết (`workflow.gate.list`), có huy hiệu số lượng và thông báo.
- Màn hình cổng: tiêu đề, hướng dẫn, `context` (render Markdown/JSON an toàn, không chạy HTML), nút theo `decisions[]`, ô ghi chú, hạn còn lại. Không hiển thị hành động khi người dùng không có quyền (việc chặn thật ở backend).

### 2.6 Trigger và thư viện

- Tab Trigger của template: tạo/sửa/bật/tắt trigger, xem `trigger_firings` (đã chạy, trùng, bị chặn, lý do), lấy URL webhook và hiển thị secret **một lần** khi tạo.
- Thư viện: tìm kiếm, sắp xếp, chia sẻ, clone, đánh giá (các kênh ở CR-WFD-008); duyệt công bố template (approval cũ) có màn hình riêng.

### 2.7 Nguyên tắc UI/UX và tuân thủ

- Theo `docs/STYLEGUIDE.md`, token màu/kiểu chữ trong `main.css`, và các primitive `components/ui/`; không thêm giá trị màu mới.
- Mọi chuỗi qua i18n (`vi`, `en`); phím tắt theo quy tắc đa nền tảng (Cmd/Ctrl).
- Hoạt động trên mọi chế độ chạy (web, SSH) vì chỉ nói chuyện qua `wscompat`.
- Các tệp lớn tách theo khái niệm (`workflow-canvas`, `workflow-step-inspector`, `workflow-run-timeline`…), **không** dùng tên chung chung (`helpers`, `utils`), và không thêm bản vô hiệu hoá quy tắc `max-lines`.

## 3. Tiêu chí chấp nhận

1. Tạo workflow có rẽ nhánh, cổng và workflow con **hoàn toàn bằng UI**, không sửa JSON tay.
2. Lưu với `expectedVersion` cũ hiện hộp thoại xung đột, không ghi đè.
3. Lỗi biểu thức hiển thị tại bước và dòng gây lỗi, chặn Lưu.
4. Form input chặn gửi khi thiếu input bắt buộc hoặc sai kiểu; backend vẫn kiểm lại.
5. Màn hình execution cập nhật trạng thái từng bước **không cần chờ 4 giây** và hiển thị bước `skipped/waiting`.
6. Người có quyền thấy cổng trong hộp thư và quyết được; người không có quyền không thấy nút (và bị backend từ chối nếu thử).
7. Chạy lại từ bước lỗi, tạm dừng, tiếp tục, huỷ hoạt động từ UI.
8. Thêm một loại bước thử ở backend (CR-WFD-005 tiêu chí 1) xuất hiện trong palette và có form mà không sửa UI.
9. Test component cho canvas, form schema, trạng thái; test kiểu chống lệch với proto.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Quy mô | Lớn nhất series về số tệp; chia giai đoạn: (1) kiểu + form schema + validate, (2) canvas, (3) theo dõi, (4) cổng/trigger/thư viện |
| Tương thích | UI mới cần backend CR-WFD-001/003/008; trong lúc chuyển tiếp, giữ trình soạn cũ sau cờ |
| Rủi ro | Canvas chỉnh sửa trên đồ thị lớn (200 bước) chậm; dùng ảo hoá có sẵn của `@xyflow/react`, giới hạn hiển thị chi tiết |
| Rủi ro | Render `context` của cổng là bề mặt XSS; chỉ Markdown đã làm sạch, không HTML |

## 5. Chưa kiểm chứng / câu hỏi mở

- Hiện trạng thật của `AttachWorkflowTemplateAction` và các bề mặt khác ngoài `StepEditor` chưa đọc hết.
- Chưa kiểm chứng BUG-FE-TASKV1-007 đã sửa chưa.
- Thư viện soạn biểu thức (ô nhập có tô màu): dùng `monaco-editor` đã có trong dự án hay ô đơn giản; mặc định dùng ô đơn giản, nâng cấp sau.
