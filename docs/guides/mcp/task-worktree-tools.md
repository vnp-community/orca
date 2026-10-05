# Task và worktree qua MCP

**Cập nhật:** 2026-10-05. Đọc từ code (`backend-go/services/api-gateway/internal/adapter/mcpserver/tools/`,
`backend-go/services/task-service/internal/usecase/execute_task.go`); **chưa chạy thử end-to-end** qua một client MCP thật.

Một agent bên ngoài có thể đọc, tạo và chạy **OrcaTask**, rồi để Orca tạo worktree cho task đó. Mỗi công cụ MCP
là một kênh nội bộ (`task.execute`) với dấu `.` đổi thành `_` (tên công cụ: `task_execute`), chạy với danh
nghĩa người dùng đã cấp quyền.

## Công cụ liên quan và quyền cần có

> **Công cụ ghi và exec mặc định bị ẩn.** Gateway chỉ công bố nhóm công cụ (pack) được bật bởi
> `MCP_TOOL_PACKS_ENABLED` (mặc định `1` = chỉ đọc; `2` = ghi, `3` = exec, `4` = xoá/admin). Muốn dùng `task_create` hay
> `task_execute` qua MCP, quản trị viên phải đặt biến này (ví dụ `1,2,3`) trong `.env` của server rồi khởi động lại
> api-gateway. Token có đủ scope vẫn không thấy công cụ của pack chưa bật. Trên server dev hiện chỉ bật pack 1.

| Công cụ | Việc làm | Mức rủi ro / scope |
|---|---|---|
| `project_list`, `project_get`, `task_list`, `task_get`, `task_getSource`, `task_getSubtree`, `task_getDependencies`, `task_hasActiveExecutions` | đọc | read / `orca:read` |
| `task_create`, `task_update`, `task_createFromSource`, `task_addEdge`, `task_aiDecompose`, `worktree_create`, `worktree_createFromIssue` | ghi, hoàn tác được | write / `orca:write` |
| `task_execute` | giao task cho agent chạy | exec / `orca:exec` |
| `task_delete`, `worktree_rm`, `project_delete` | xoá | destructive |

Token không có scope tương ứng thì công cụ **không xuất hiện** trong danh sách. `exec` và `destructive` mặc định
**yêu cầu bạn phê duyệt** ([xem hướng dẫn](./approving-agent-actions.md)); admin có thể đổi bằng policy.

## `task_execute` làm gì

1. Kiểm tra bạn có quyền `execute` trên task. Task đang `in_progress` bị từ chối (`TASK_EXECUTE_ALREADY_IN_PROGRESS`).
2. Hỏi project xem có **dev server đang kết nối** không. Không có thì dừng, task chưa đổi trạng thái (`TASK_EXECUTE_NO_CONNECTION`).
3. **Tạo hoặc tái dùng worktree** cho task:
   - task đã có worktree: dùng lại; nếu worktree đã bị xoá thì tạo mới;
   - issue của task đã có worktree: dùng lại worktree đó;
   - còn lại: tạo worktree mới ở repo đầu tiên của project, nhánh `task/<taskId>`, kèm liên kết issue (provider, khoá, site) nếu task có nguồn.
4. Đánh dấu task `in_progress` (một lời gọi duy nhất thắng nếu gọi trùng), ghi một dòng lịch sử chạy (`execution_links`).
5. Trả về ngay (`async`); agent chạy nền trong worktree.

Tham số `prompt` chỉ dùng được với task không có subtask, phụ thuộc hay workflow (`TASK_EXECUTE_PROMPT_UNSUPPORTED`).

## Trạng thái task sau khi chạy

| Tình huống | Trạng thái task |
|---|---|
| Agent chạy xong (task đơn lẻ) | **`review`** (không tự sang `done`; người duyệt chuyển) |
| Agent lỗi | quay về trạng thái trước khi chạy |
| Task có subtask hoặc workflow | dịch vụ điều phối báo kết quả về sau (`ReportTaskExecutionResult`) |
| Service chết giữa chừng | sau khoảng 90–120 giây task được khôi phục về trạng thái trước |

Lưu ý: với task có subtask/workflow, dòng lịch sử chạy được đánh dấu `completed` ngay lúc giao việc, **không** có nghĩa là xong.

## Liên kết với Jira

- Worktree do `task_execute` tạo **có** liên kết issue nên kích hoạt đồng bộ Jira (To Do → In Progress), nếu task có nguồn Jira.
- `task_createFromSource` qua MCP **chưa nhận tham số `site`**: task tạo từ MCP lưu site rỗng và khớp với mọi site Jira.
- `worktree_create` qua MCP **không có tham số liên kết issue**: worktree tạo bằng công cụ này không liên kết issue và không đồng bộ Jira.
- `worktree_createFromIssue` chưa mang site.

Xem thêm: [Jira ↔ Orca](../jira/jira-orca-mapping.md).
