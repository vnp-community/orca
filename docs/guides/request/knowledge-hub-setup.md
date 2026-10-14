# Bật engine OpenSpec và Solution bằng agent (tri thức tdd-agent)

Hai cờ theo project, mặc định **tắt**; tắt thì Request chạy y như trước.

| Cờ | Tác dụng khi bật |
|---|---|
| `engine = openspec` | `plan generate` cho agent viết một change OpenSpec (trên nhánh `request/<số>-proposal` của worktree riêng); Orca kiểm cấu trúc rồi dựng Plan từ `tasks.md`. |
| `solution_agent = true` | Solution do agent chỉ đọc viết, có đọc đồ thị tri thức của project qua MCP; kết quả có `design_references` (đã đọc gì, thiếu gì, xung đột với Decision/Constraint nào). |

Orca **không** gọi MCP hay đồ thị tri thức; agent mà Orca chạy trên dev server mới gọi. Thiếu phần tri thức thì Solution và Plan vẫn được viết, chỉ không có tham chiếu (cờ `available=false`).

## Cài trên dev server của project

1. **OpenSpec CLI**: `openspec --version` phải chạy được (nếu cần đặt bản tối thiểu: `orca engine set --project <id> --min-version 1.2.0`).
2. **tdd-agent** có trong `PATH` và repo của project đã có `.tdd-agent/config.toml`. Orca chỉ kiểm tệp có mặt, không truy vấn đồ thị.
3. **Skill** (một lần cho mỗi dev server):

   ```bash
   npx skills add https://github.com/stablyai/orca --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global
   ```

   Cập nhật từng skill: `npx skills update <tên> --global`. Máy chủ có thể trỏ nguồn khác bằng `REQUEST_SKILLS_REPOSITORY_URL` (hiện trong gợi ý sửa lỗi).
4. **Agent Orca** đủ mới (báo `agent.execPrompt.knowledgeHub`, `agent.execPrompt.permissionProfile`, ...). Agent cũ không bị hỏng: chỉ không dùng được hai cờ.

## Bật và kiểm tra

```bash
orca engine show --project <id> --check
orca engine set  --project <id> --engine openspec --solution-agent=true     # chỉ admin
```

`--check` hỏi chính dev server (có cache sạch 10 phút, lỗi thì không cache nên sửa xong thấy ngay):

- **blocked** (chặn engine `openspec`): không có dev server hoặc repo path; `openspec` thiếu/cũ/không trả lời; agent không viết được change trong worktree dưới profile quyền.
- **knowledge hub unused** (chỉ cảnh báo, engine vẫn chạy không có tri thức): agent không tải được hub; `tdd-agent` không có trong `PATH`; thiếu skill; repo không có `.tdd-agent/config.toml`.
- **warning**: repo có tệp chưa commit (đề xuất dùng worktree riêng nên không ảnh hưởng); không kiểm được tệp cấu hình.

Mỗi dòng kèm `code` (`REQUEST_ENGINE_*`, `REQUEST_KNOWLEDGE_*`) và gợi ý sửa. Lưu cài đặt không bao giờ bị chặn bởi kiểm tra; đổi cài đặt được ghi audit (`request.engine_settings.set`).

## Khi thiếu khả năng lúc chạy

- `solution_agent` bật nhưng dev server không tải được hub, không có đường dẫn repo hay không đọc được khả năng: Solution đi `ai.complete` như cũ và ghi log lý do.
- Engine `openspec` mà dev server thiếu `openspec` hoặc agent quá cũ: run thất bại với mã `REQUEST_ENGINE_*` kèm gợi ý, Request không bị hỏng.

## Chạy task của một Plan OpenSpec

Task của Plan do engine `openspec` viết chạy trên đường hợp đồng của task-service: agent kết thúc bằng một khối kết quả có nonce, Orca đọc `status`, `checks_run`, `knowledge` và `writeback`; nếu dev server có `agent.execPrompt.knowledgeHub` thì agent còn đọc được đồ thị tri thức từ **thư mục gốc của repo** (không phải worktree của task). Task có dòng `Verify:` chỉ được coi là xong khi `checks_run` có ít nhất một kiểm tra thoát với mã 0; báo `done` mà không có bằng chứng thì tính là một lần thất bại (`VERIFY_NOT_PASSED`, được thử lại theo số lần tối đa). Xem kết quả bằng `orca plan result <task-id>`; phần `knowledge`/`writeback` là lời của agent, Orca không kiểm.

**Rủi ro đã biết (B4):** mọi lần thực thi task vẫn chạy với `trustPreset: full` (agent không có người duyệt từng lệnh). Cổng duyệt Plan và Phase, kiểm phạm vi `Code:` và bằng chứng `checks_run` là các lớp bù, không thay thế việc xem diff trước khi merge.
