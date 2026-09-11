# backend-go Tasks — Annotate AI Diffs (F08, v4)

**Solutions:** [../solutions/](../solutions/README.md)

| Task | Solution | Depends on | Status |
|---|---|---|---|
| [TASK-BE-ANNOTATE-001](./TASK-BE-ANNOTATE-001-extract-compose-review-feedback-prompt.md) — extract `ComposeReviewFeedbackPrompt` | SOL-BE-ANNOTATE-002 | Không | ✅ DONE |
| [TASK-BE-ANNOTATE-002](./TASK-BE-ANNOTATE-002-add-compose-review-prompt-channel.md) — thêm channel `annotation.composeReviewPrompt` | SOL-BE-ANNOTATE-002 | TASK-BE-ANNOTATE-001 | ✅ DONE |
| [TASK-BE-ANNOTATE-003](./TASK-BE-ANNOTATE-003-fix-annotation-channels-snake-case-json-bug.md) — sửa bug snake_case JSON của `annotation.*` | không có (phát hiện live khi thực thi TASK-FE-ANNOTATE-002) | Không | ✅ DONE |

**Lưu ý**: TASK-BE-ANNOTATE-003 không nằm trong kế hoạch ban đầu của solution
nào — phát hiện trong lúc thực thi phía frontend (xem
[TASK-FE-ANNOTATE-002](../../../../frontend/crs/v4/annotate/tasks/TASK-FE-ANNOTATE-002-hydrate-and-backfill.md)'s
"Kết quả thực tế"). Toàn bộ 3 task trong nhóm backend-go này đã ✅ DONE.

## Why SOL-BE-ANNOTATE-001 has no tasks

SOL-BE-ANNOTATE-001 (cho CR-ANNOTATE-001) là assessment-only — kết luận
tường minh "zero backend-go code change required" vì toàn bộ CRUD +
field/RPC mà CR-ANNOTATE-001 cần đã tồn tại thật, đã ship, đã test (CR-02
series, 12/12 task DONE trước đó). Tạo task cho 1 solution mà toàn bộ nội
dung là "không cần sửa gì" sẽ là bịa ra việc không tồn tại — mirrors
`specs/agent/crs/v4/task-graph/tasks/README.md`'s "Why SOL-AG-TG-001 has no
tasks" precedent.

## Thứ tự thực thi

```
TASK-BE-ANNOTATE-001 (extract, pure refactor, không đổi hành vi cũ)
        │
        ▼
TASK-BE-ANNOTATE-002 (expose channel mới, dùng hàm đã tách)
```

Cả 2 task đều nằm trong 1 file duy nhất (`channels_annotation_send.go`) +
1 điểm wire (`channels.go`) — không có song song hoá, làm tuần tự.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore` trước khi sửa symbol** —
  `SendReviewFeedbackToAgent` có 2 caller đã biết (WS channel + REST
  mirror), bắt buộc xác nhận không sót caller thứ 3 trước khi refactor.
- **TASK-BE-ANNOTATE-001 là refactor giữ nguyên hành vi** — nếu phải sửa
  bất kỳ test case cũ nào của `SendReviewFeedbackToAgent` để nó pass lại,
  đó là dấu hiệu refactor sai, dừng lại và đối chiếu lại thiết kế trước khi
  tiếp tục, không tự ý sửa test cũ cho khớp.
- **Test thật, không giả định pass** — chạy `go build`/`go vet`/`go test`
  thật, ghi lại kết quả cụ thể, không suy đoán "chắc sẽ pass".
