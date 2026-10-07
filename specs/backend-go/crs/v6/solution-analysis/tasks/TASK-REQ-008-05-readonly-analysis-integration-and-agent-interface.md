# TASK-REQ-008-05: Test tích hợp hai DB, hợp đồng JSON và ghi giao diện với agent

**From Solution:** [BE-REQ-SOL-008](../solutions/BE-REQ-SOL-008-diagnosis-findings-answer-analysis.md) mục G, 5
**Priority:** P1
**Service/Area:** `request-service` / test; tài liệu giao diện
**File:** `internal/adapter/{postgres,mysql}/agent_readonly_flow_integration_test.go` (mới), `testdata/analysis_documents/*.json` (mới, dùng chung task 01), `backend-go/services/request-service/README.md` (sửa: mục "Chế độ chỉ đọc và giới hạn đã biết")
**Depends on:** TASK-REQ-008-03, TASK-REQ-008-04
**Status:** [x] DONE

## Context

- Phần thay đổi agent (TypeScript) thuộc CR-REQ-033, không làm ở đây. Task này chỉ chốt hợp đồng phía backend: tên khoá `params_json` tuỳ chọn, hành vi khi agent cũ không hiểu khoá.
- `CountRunning` trên MySQL phải đúng khi có nhiều run đồng thời (không có partial index).
- Thử nghiệm thủ công với `agent.execPrompt` thật chưa từng chạy; task này tạo kịch bản kiểm, không khẳng định kết quả.

## Việc cần làm

1. Test tích hợp hai DB: chạy bộ test repository của SOL-007 với `mode=agent_readonly`; đếm run `running` đồng thời đúng (hai goroutine cùng cố vượt ngưỡng 2, đúng một bị `BUSY`); luồng đầy đủ `question`: `GenerateSolution` (Relay giả trả JSON `answer`) -> Approval `answer` -> `Approve` -> Request `completed`, không có Plan hay Task.
2. Luồng `hotfix`: không Approval, Solution `approved`, outbox `solution.approved` có `auto=true`.
3. Test JSON vàng cho ba `kind` bằng `testdata/analysis_documents/{diagnosis,findings,answer}_valid.json` và `_invalid_*.json`.
4. README của service: ghi rõ ba lớp bảo vệ chỉ đọc và giới hạn đã biết (không ép được; `HEAD` không được so; `repo_check=skipped` khi thiếu `worktree_id`), cờ `REQUEST_AGENT_READONLY_USE_AGENT_FLAG` và việc chờ CR-REQ-033.
5. Kịch bản kiểm thủ công (viết vào PR, đánh dấu "chưa chạy"): `agent.execPrompt` với `trustPreset=default` trên dev server cục bộ và SSH; hỏi agent sửa file thử; quan sát chặn, hỏi quyền hay cho phép; kết quả quyết định có cần khẩn CR-REQ-033.

## Kiểm thử

- Lệnh: `go test ./internal/adapter/postgres/... ./internal/adapter/mysql/... -run AgentReadonlyFlow`.
- Chạy toàn bộ: `go test ./services/request-service/...` từ `backend-go`.

## Tiêu chí hoàn thành

- [x] Tiêu chí chấp nhận 7 (đồng thời), 9 (answer hoàn tất), 4 (hotfix) có test tích hợp xanh trên hai DB.
- [x] README ghi đủ giới hạn đã biết.
- [x] PR nêu rõ phần kiểm thử thủ công "chưa chạy".

## Rủi ro và lưu ý

- Test đua cần timeout ngắn để lộ khoá chết.
- Không nhận định về hành vi agent thật cho tới khi chạy kịch bản thủ công.
