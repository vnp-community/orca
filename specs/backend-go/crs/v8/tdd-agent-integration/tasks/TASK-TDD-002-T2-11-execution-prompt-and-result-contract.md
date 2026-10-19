# TASK-TDD-002-T2-11: Thực thi task: `knowledgeRoot`, prompt có tiêu chí và `Verify`, kết quả có `Knowledge`/`Verification`

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5
**Checklist:** [T2.11](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** task-service + request-service (Go)
**Files:** `task-service/internal/adapter/grpcclient/{simple_executor,simple_executor_contract}.go`, `usecase/{execute_task,contract_executor}.go`, `request-service/internal/adapter/grpcclient/task_plan_writer.go` (`planTaskContext`), test đi kèm; đọc: `task-service` `task_specs` (migration 0020), `specs/backend-go/crs/v6/execution-contract/`
**Depends on:** T2.1, T2.6
**Status:** `[ ]` TODO

---

## Context

Dữ kiện đã kiểm:
- `buildExecutePrompt` dùng nguyên văn `task.PromptTemplate` (nếu có) rồi `Task/Description/Context`, task cha, task phụ thuộc đã xong; `ai_context` do request-service ghi chỉ là `REQ-n: tiêu đề` + summary Plan (tối đa 2000 ký tự); tiêu chí chấp nhận không nằm trong prompt thực thi.
- Mọi lần thực thi task dùng `TrustPreset: "full"` (chạy không giám sát, cờ YOLO): đây là vị thế B4, task này không đổi nó.
- `ResolveConnection` trả (connectionID, đường dẫn repo, worktreeID, devServerID, connected, err); giá trị thứ hai là đường dẫn **repo** (comment BUG-028), dùng làm `KnowledgeRoot`.
- task-service **đã có** hợp đồng thực thi (CR-REQ-029): `TaskSpecV2` (phạm vi `Include/Exclude/Create`, `MaxFiles`), `ExecuteWithContract` với result block có nonce, parser `agentParsedResult`, khối kết quả `{status: done|blocked|failed|needs_info, summary, files_changed, checks_run[{id,exit}], outputs, questions, notes}`. Chưa thấy request-service bật đường này (không có cờ `REQUEST_EXECUTION_CONTRACT_ENABLED` trong mã).

## Việc cần làm

1. Khảo sát (ghi kết quả vào PR): phần nào của hợp đồng đã được nối từ request-service (tạo `TaskSpecV2`, gửi `ResultNonce`), phần nào chưa; quyết định **dùng hợp đồng** (ưu tiên) hay làm bản tối thiểu dưới đây.
2. `agentExecPromptParams` (task-service): thêm `KnowledgeRoot`, `ProposeChangesets` (omitempty); chỉ gửi khi dev server khai báo `agent.execPrompt.knowledgeHub` (cần cách task-service đọc khả năng; nếu thiếu thì thêm vào `ResolveConnection`/capability reader).
3. `planTaskContext` (request-service): ngoài `REQ-n: tiêu đề` + summary, thêm văn bản các AC mà task phục vụ (`Satisfies`) trong trần 2000 ký tự; `prompt_template` (T2.6) đã mang `Serves/Code/Verify/Done when`.
4. Hợp đồng: sinh `TaskSpecV2` từ `Code:` (`modify` ⇒ `Scope.Include`, `create` ⇒ `Scope.Create`, `MaxFiles` theo số đích) và danh sách kiểm từ `Verify:`; kết quả `checks_run` là bằng chứng `Verification`. Bản tối thiểu (nếu hợp đồng chưa nối): chạy như cũ với `ResultBlock{nonce}`.
5. Mở rộng khối kết quả của agent (trong skill `tdd-orca-task` và prompt): thêm `knowledge{snapshot, read[], gaps[], conflicts[], available}` và `writeback{documents[], proposal_id, proposal_ok, touched}`; parse bằng `agentParsedResult.Value`, lưu vào bản ghi thực thi, hiển thị ở `orca` CLI.
6. Ngữ nghĩa hoàn thành (đề xuất, chờ xác nhận): task có dòng `Verify` chỉ `done` khi `checks_run` có ít nhất một kiểm tra `exit=0`; ngược lại `needs_info` hoặc `failed` theo hợp đồng.
7. Giữ `TrustPreset: "full"` như hiện trạng; ghi chú rủi ro B4 vào tài liệu của task.

## Tiêu chí hoàn thành

- [ ] Agent task nhận `knowledgeRoot` đúng (repo gốc, không phải worktree task) khi feature có mặt; không có thì chạy như cũ.
- [ ] Prompt thực thi chứa id yêu cầu, AC liên quan, đích mã và lệnh `Verify`.
- [ ] Khối `knowledge`/`writeback` được lưu và hiển thị; thiếu khối thì cảnh báo, không làm hỏng run.
- [ ] Không đổi hành vi của task không thuộc Plan OpenSpec.

## Kiểm thử

- Test `buildExecutePrompt` (golden), `planTaskContext` (trần ký tự), tham số có/không feature, parser khối kết quả mở rộng, trạng thái hoàn thành.

## Rủi ro và ghi chú

- Chạy `gitnexus_impact` cho `buildExecutePrompt`, `runAgentPrompt`, `ExecuteWithContract` trước khi sửa.
- Nếu hợp đồng chưa nối được, ghi rõ phần còn thiếu thành task riêng thay vì làm nửa vời.
