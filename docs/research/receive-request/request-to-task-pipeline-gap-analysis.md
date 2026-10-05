# Pipeline Request → Solution → Plan → Phase → Task: đối chiếu hiện trạng Orca

| Trường | Giá trị |
|---|---|
| **Ngày** | 2026-10-05 |
| **Loại** | Nghiên cứu (chưa phải CR) |
| **Phương pháp** | Đọc docs trong repo. **Chưa kiểm tra code.** |
| **Liên quan** | [request-classification-and-flows.md](./request-classification-and-flows.md), [request-pipeline-existing-capabilities-and-build-scope.md](./request-pipeline-existing-capabilities-and-build-scope.md) |

> **Lưu ý:** file này viết từ docs. Kiểm tra code sau đó cho thấy code không có status `backlog` và `mcp-service` đã tồn tại; xem file build-scope để có hiện trạng chính xác và các quyết định kiến trúc đã chốt.

## 1. Luồng mục tiêu

Nguồn yêu cầu (Jira, GitHub, MCP) vào một thực thể **Orca Request**. Người dùng bấm sinh giải pháp, agent đề xuất Solution, người duyệt, rồi hệ thống sinh Plan, Phase và Task, mỗi cấp có cổng duyệt riêng. Agent thực thi từng task và cập nhật ngược lên Plan, Solution, Request. Không thực thi được thì trả về backlog của cấp tương ứng, riêng Request trả về **Request backlog** kèm Request gốc và lý do.

```
Nguồn (Jira/GitHub/MCP) → Request → [bấm sinh giải pháp] → Agent đề xuất Solution
  → ▣ duyệt Solution → Plan → Task backlog theo plan → ▣ duyệt Plan
  → Plan thực thi + Task chi tiết theo Phase → Execute backlog → ▣ duyệt từng Phase
  → Agent chạy từng Task, cập nhật trạng thái
       ├─ task lỗi            → Execute backlog
       ├─ hết Phase           → cập nhật Plan
       ├─ Plan không thực hiện → Task backlog
       └─ xong tất cả         → cập nhật Solution → cập nhật trạng thái Request
  Không thực thi được → Request backlog (gắn Request gốc + lý do)
```

## 2. Đối chiếu từng bước

Ký hiệu: ◼ đã có, ◧ một phần, ◻ chưa có.

| # | Yêu cầu | Trạng thái | Căn cứ trong docs |
|---|---|---|---|
| 1 | Nguồn Jira | ◧ | Xem issue, "Start work", liên kết worktree và task ([CR-TG-008](../../crs/v4/task-graph/CR-TG-008-jira-source-link-and-durable-direct-agent.md), [jira-orca-mapping](../../guides/jira/jira-orca-mapping.md)). Backend và frontend đã code, chưa chạy end-to-end thật. Liên kết task nằm sau cờ Experimental, mặc định tắt. |
| 1 | Nguồn GitHub | ◧ | Xem issue/PR ở trang Tasks, liên kết worktree. `task.createFromSource` được mô tả riêng cho Jira. |
| 1 | Nguồn MCP | ◻ | [CR v5](../../crs/v5/README.md) mới ở mức đề xuất, chưa có service hay SDK MCP. Đó là hướng agent gọi vào Orca, không phải nguồn yêu cầu. |
| 2 | Thực thể Orca Request | ◻ | Chỉ có `OrcaTask` và `task.task_sources`. |
| 3 | Bấm sinh giải pháp | ◧ | Có "Generate Spec" trên một task ([BL-TG-05](../../logic/task-graph/BL-TG-05-spec-approve-build-loop.md)), chưa có trên Request. |
| 4 | Agent đề xuất Solution | ◧ | Agent sinh một spec dạng file markdown trong worktree. Không có đối tượng Solution, không có nhiều phương án. |
| 5 | Người xem trước và duyệt | ◧ | Xem qua tab Git. Duyệt bằng label `phase:spec-approved`. Không có trang xem riêng. |
| 6 | Sinh Plan thực thi | ◧ | `task.aiDecompose` chia task thành subtask, dependency, estimate ([BL-TG-02](../../logic/task-graph/BL-TG-02-ai-task-planning.md)). Chia từ task, không từ Solution đã duyệt. |
| 7 | Task backlog theo plan | ◧ | Status `backlog` có trong DB và Board ([CR-TG-001](../../crs/v4/task-graph/CR-TG-001-orcatask-data-model-widening.md), [CR-TG-007](../../crs/v4/task-graph/CR-TG-007-frontend-task-crud-board-grant-ui.md)). Không có Plan để gom nhóm. |
| 8 | Duyệt Plan | ◻ | Chỉ có duyệt spec theo task. |
| 9 | Plan execute + task theo Phase | ◻ | Không có khái niệm Phase như yêu cầu. Các `phase:*` hiện có chỉ là pha spec → code của một task. |
| 10 | Duyệt theo Phase | ◻ | [BL-TG-06](../../logic/task-graph/BL-TG-06-centralized-spec-code-merge.md) duyệt từng task hoặc hàng loạt. `DecisionGate` chưa có UI (CR-TG-008 mục chưa làm). Step `approval` của workflow không có backend ([BACKLOG-027](../../backlog/BACKLOG-027-workflow-approval-step-type-removal-decision.md)). |
| 11 | Agent chạy từng task, cập nhật trạng thái | ◼ | `task.execute` rẽ nhánh agent đơn hoặc orchestration. Task xong về `review`. Task kẹt được trả về trạng thái trước, Engine 1 phục hồi sau restart (CR-TG-008). |
| 12 | Task lỗi → Execute backlog | ◧ | Task thất bại về `previous_status`. Không có backlog riêng, không ghi lý do có cấu trúc. |
| 13 | Hết Phase → cập nhật Plan; Plan lỗi → Task backlog | ◻ | Không có Plan. |
| 14 | Xong → cập nhật Solution và trạng thái Request | ◻ | Đồng bộ ngược duy nhất là trạng thái issue Jira (In Progress, In Review, Done). |
| 15 | Trả Request backlog kèm lý do | ◻ | Không có. |

## 3. Kết luận

- **Đã đáp ứng:** bước 11 (thực thi và cập nhật trạng thái task), cộng nền cho bước 1 (Jira, GitHub) và bước 7 (status backlog).
- **Một phần:** bước 3 đến 7 và 12. Vòng spec → duyệt → code chạy được cho một task, nhưng toàn bộ nằm ở cấp task.
- **Chưa có:** Request, Solution, Plan, Phase, cổng duyệt theo từng cấp, vòng trả ngược (task → plan → solution → request), Request backlog kèm lý do, nguồn MCP.
- Phần thiếu chính là **mô hình dữ liệu** (Request → Solution → Plan → Phase → Task). Thực thi và hạ tầng phần lớn đã có. `DecisionGate` (đã có bảng và event notification `orca.orchestration.decision_gate.opened`) là ứng viên cho cổng duyệt.

## 4. Phân loại Jira hiện tại (bối cảnh)

Hệ thống chỉ phân loại theo hai trục: preset lọc (Assigned, Reported, All Open, Done) và nhóm trạng thái Jira dùng để đồng bộ ngược (`worktree.created` → In Progress, PR tạo → In Review, PR merge → Done; chỉ khi issue đang ở nhóm `in_progress`, chuyển theo **tên** trạng thái). Docs không nhắc việc dùng issue type, priority, label, epic hay component để phân loại hay định tuyến. Chi tiết: [jira-orca-mapping](../../guides/jira/jira-orca-mapping.md).
