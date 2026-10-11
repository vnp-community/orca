# TASK-V2-038: Xác nhận hình dạng tài liệu `answer` qua kênh gateway `solution.list` cho frontend, chốt bằng test

**From:** lượt test Request #1136, lỗi (1) UI `AnswerView` không đọc `answer_markdown` (sửa ở frontend, agent khác) · **Solution:** [SOL-V2-003](../solutions/SOL-V2-003-project-own-project-and-repo-dedupe.md) mục 3
**Priority:** P1 · **Service/Area:** api-gateway (wscompat) — chỉ test; request-service chỉ đọc
**Files:** `api-gateway/internal/adapter/wscompat/channels_solution_answer_shape_test.go` (mới). Không đổi mã sản xuất nếu test xác nhận hình dạng đúng.
**Depends on:** không · **Status:** `[x]` XONG (2026-10-11): mã, test (đơn vị, hợp đồng, tích hợp hai dialect, đua) và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

## Kết luận đã xác minh (đọc mã)

`solutions.options_json` của kind `answer` là `AnswerDocument` đã chuẩn hoá (`request-service/internal/usecase/analysis_document_validation.go:41-49`, `domain/answer_document.go`); gRPC trả nguyên chuỗi (`adapter/grpc/server_solution.go:132`); gateway `SolutionViewOf` giữ JSON thô trong `options` (`wscompat/channels_request_views.go:112-123`), KHÔNG đổi khoá. Hình dạng cho frontend:

```
solution.list -> { solutions: [ { id, requestId, kind:"answer", status, options:
  { schema_version:1, kind:"answer", answer_markdown:"<markdown>", confidence:0..1,
    citations?:[{ref, excerpt?}], limitations?:[string],
    suggested_follow_up?:{type:"change_request"|"task", title} },
  chosenOption, generationRunId?, createdAt, version } ], runs, nextPageToken }
```
Khoá trong `options` là snake_case; không có `assumptions` (tương đương là `limitations`); `chosenOptionId` không có ý nghĩa cho `answer`.

## Việc cần làm

- [x] Test gateway: kênh `solution.list` với một Solution `kind answer` có `options_json` đầy đủ trả đúng `options.answer_markdown`, `confidence`, `citations[0].ref`, `limitations`, `suggested_follow_up.type` nguyên vẹn (so JSON, không đổi tên); kind khác không bị ảnh hưởng.
- [x] Ghi hình dạng ở trên vào báo cáo cho agent frontend.
- [x] Nếu test phát hiện mất/đổi tên trường thì sửa phía backend (kỳ vọng: không phải sửa).

## Kết quả thực thi

- Xác nhận bằng đọc mã và test: gateway không đổi tên/làm mất trường; `options` là JSON thô của request-service. Hình dạng cho frontend ở mục trên (`solution.options.answer_markdown`, snake_case; `limitations` chứ không phải `assumptions`; `kind` bỏ trống khi gọi `solution.list` là mọi kind, có thể lọc `kind: "answer"`).
- Test mới: `api-gateway/internal/adapter/wscompat/channels_solution_answer_shape_test.go` (`solution.list` giữ nguyên tài liệu `answer`, không có khoá camelCase, không có `chosenOptionId`) và `request-service/internal/domain/answer_document_wire_test.go` (khoá marshal của `AnswerDocument` là hợp đồng với UI). Cả hai xanh.
