# CR-ANNOTATE-002 — Route "Send to Agent" through `annotation.sendToAgent` for code-context (BR-CR-11), reconcile with existing guarded-delivery UX

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-ANNOTATE-002 |
| **Tên** | Prompt gửi agent thiếu ±2 dòng code context (BR-CR-11) vì không dùng `annotation.sendToAgent` (backend-go, đã ship, 0 caller) — nối lại, giữ nguyên UX chọn agent + guarded-paste đã có |
| **Loại** | Feature completion |
| **Priority** | 🟡 P2 |
| **Effort** | Medium (~3-5 ngày, phụ thuộc quyết định kiến trúc bên dưới) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-11 |
| **Trạng thái** | ✅ DONE (2026-09-11) — xem tasks/ + solutions/ ở cả frontend và backend-go |
| **Tác giả** | Audit trực tiếp mã nguồn theo yêu cầu "tạo CR để thực thi F08 Annotate AI Diffs" |
| **Tác động Features** | F08 (Annotate AI Diffs) |
| **Phụ thuộc** | **Cứng vào [CR-ANNOTATE-001](./CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)** — `annotation.sendToAgent` đọc annotation đã persist với `worktree_id`/`side`/`original_code`; nếu comment vẫn chỉ ở Zustand local, channel này không có gì để đọc |

---

## Bối cảnh & Vấn đề

### 0. Prompt thật đang gửi đi thiếu đúng field mà spec yêu cầu

`docs/features/F08-annotate-ai-diffs.md`'s mục "Feedback to Agent" liệt kê
format cần có: *"File path, Line number(s), Original code (context), Comment
text"*. Prompt thật đang được gửi (`frontend/src/shared/diff-comments-format.ts`,
xác nhận đang chạy qua `DiffNotesSendMenu.tsx`) chỉ có:

```
File: auth.ts
Line: 42
User comment: "Cần check user !== null trước"
```

Thiếu hẳn dòng "Original code" — đúng là gap thật so với cả spec lẫn
BR-CR-11 ("2 lines of code context before/after each comment for
disambiguation"). Agent nhận prompt này phải tự đoán dòng 42 là dòng nào
trong file hiện tại — dễ sai nếu file đã thay đổi giữa lúc comment và lúc
agent đọc.

### 1. Backend-go đã xây xong đúng thứ này — nhưng mồ côi

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go`
(xác nhận file tồn tại thật, không phải task claim suông) implement
`annotation.sendToAgent`:

- Đọc annotation theo `worktree_id`, lọc `sent_to_agent=false`.
- Với mỗi annotation: đọc ±2 dòng file hiện tại qua `git-gateway-service`
  (`resolveCodeContext`), fallback về `original_code` đã lưu nếu đọc file
  lỗi hoặc `side=old`.
- Format đúng theo template BL-CR-03 (`formatReviewPrompt`/`formatFeedbackBlock`)
  — CÓ dòng `Code:` + `Context:` mà client-side format đang thiếu.
- Chuẩn hoá path về repo-root-relative (BR-CR-10).
- Gửi qua `terminal.send`-tương-đương (`PtyClientFrame_Input` trực tiếp tới
  PTY theo `ptyId` truyền vào).
- Gọi `MarkAnnotationsSent` sau khi gửi thành công.

12/12 task của cả CR-02 (data model) và CR-03 (channel này) đã `[x]` DONE,
`go build`/`go vet` sạch (tự xác nhận). Nhưng **0 nơi nào trong frontend gọi
`annotation.sendToAgent`** (grep xác nhận) — toàn bộ logic ±2-dòng-context
này chưa từng chạy cho người dùng thật.

### 2. Vì sao chưa đơn giản là "gọi channel đó thay vì code cũ" — 2 cơ chế delivery khác nhau

`annotation.sendToAgent`'s delivery step gửi thẳng 1
`PtyClientFrame_Input` — không có bước "đợi agent idle"
(`terminal.wait`), không có "guarded bracketed-paste" (gói prompt trong
escape sequence để terminal/TUI không hiểu nhầm là lệnh gõ tay), không có
"gửi Enter riêng sau khi paste xong". Trong khi đó,
`frontend/src/renderer/src/lib/active-agent-note-send.ts`'s
`sendPromptWithGuardedPasteAndEnter` (đang chạy thật, đã qua kiểm chứng với
nhiều loại TUI agent — có cả nhánh fallback cho SSH runtime cũ hơn) làm đúng
3 việc đó — thiết kế cẩn thận hơn hẳn.

Gọi thẳng `annotation.sendToAgent` như thiết kế gốc (SOL-CR-03) sẽ **đánh
đổi lùi** độ tin cậy delivery đã có, chỉ để lấy code-context. Đây không phải
quyết định kỹ thuật đơn thuần khi code — ảnh hưởng trực tiếp tới độ tin cậy
tính năng cốt lõi của F08.

## Giải pháp đề xuất

### Quyết định kiến trúc cần chốt trước khi code (không tự chọn trong CR này)

1. **Tách compose khỏi delivery ở backend-go** — thêm 1 RPC/channel mới
   `annotation.composeReviewPrompt` (hoặc mở rộng `annotation.sendToAgent`
   với 1 flag `deliverViaPty: false`) chỉ trả về `prompt` (string) đã compose
   sẵn với code-context, KHÔNG tự gửi PTY. Frontend gọi channel này lấy
   `prompt`, rồi tự gửi qua `sendNotesToActiveAgentSession` như hiện tại
   (giữ nguyên toàn bộ độ tin cậy delivery đã có), cuối cùng tự gọi
   `annotation.markSent`. **Ít thay đổi backend nhất, giữ nguyên độ tin cậy
   frontend** — nghiêng về hướng này.
2. **Port `resolveCodeContext` sang frontend** — đọc file hiện tại qua RPC
   `files.read`/tương đương đã có sẵn (frontend có thể đã có quyền truy cập
   này), tự tính ±2 dòng client-side, không đổi gì ở backend-go. Tránh
   round-trip thêm, nhưng **trùng lặp logic** đã viết đúng 1 lần ở
   `channels_annotation_send.go` — vi phạm nguyên tắc "1 nơi implement 1
   business rule" mà chính SOL-CR-03 đã lập luận kỹ khi đặt composition ở
   `api-gateway`.
3. **Nâng cấp `annotation.sendToAgent`'s delivery step lên guarded-paste** —
   port `sendPromptWithGuardedPasteAndEnter`'s logic (đợi idle, bracketed
   paste, Enter riêng) vào Go, dùng cho cả channel này. Consistent nhất về
   lâu dài (1 cơ chế delivery duy nhất, dùng ở cả 2 phía), nhưng effort lớn
   nhất và cần test lại toàn bộ ma trận TUI agent đã được frontend cover.

CR này giả định **phương án 1** (tách compose khỏi delivery) là điểm khởi
đầu rẻ nhất và an toàn nhất (không đụng cơ chế delivery đã ổn định), nhưng
**đội triển khai phải xác nhận lại** trước khi khoá — đổi hướng ở đây làm
thay đổi đáng kể "Changes Required" bên dưới.

### Thiết kế theo phương án 1 (mặc định)

```
backend-go: channels_annotation_send.go
  → tách composeReviewFeedbackPrompt(worktreeId) ra khỏi
    SendReviewFeedbackToAgent (hàm export hiện có, TASK-CR-03-03) —
    hàm mới chỉ làm bước 1-3 (collect, resolve context, format), KHÔNG làm
    bước 4-5 (deliver, mark-sent)
  → wscompat: channel mới `annotation.composeReviewPrompt({worktreeId})`
    → { prompt: string, annotationIds: string[] }
  → SendReviewFeedbackToAgent (dùng cho REST mirror, TASK-CR-03-05) giữ
    nguyên hành vi cũ (compose + deliver + mark-sent trong 1 call) — không
    breaking change cho route REST đã có

frontend: DiffNotesSendMenu.tsx / ReviewNotesSendMenuContent.tsx
  → trước khi gọi sendNotesToActiveAgentSession(prompt: formatDiffComments(...)),
    thử gọi annotation.composeReviewPrompt({worktreeId}) lấy prompt có code-context
  → fallback về formatDiffComments(...) client-side nếu RPC lỗi (không chặn
    gửi — 1 annotation-service tạm gián đoạn không được làm mất tính năng
    core "send to agent" đã hoạt động)
  → sau khi sendNotesToActiveAgentSession trả 'sent', gọi
    annotation.markSent(annotationIds) thay vì chỉ clearDeliveredDiffComments
    cục bộ (annotationIds lấy từ response của composeReviewPrompt)
```

## Changes Required

| File | Thay đổi |
|------|---------|
| `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` | Tách `composeReviewFeedbackPrompt` khỏi `SendReviewFeedbackToAgent`; thêm channel `annotation.composeReviewPrompt` |
| `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` | Test mới cho hàm compose-only; giữ nguyên test cũ cho `SendReviewFeedbackToAgent` |
| `frontend/src/renderer/src/lib/active-agent-note-send.ts` hoặc file gọi nó | Gọi `annotation.composeReviewPrompt` trước khi format client-side, fallback khi lỗi |
| `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` | Nhận `annotationIds` từ compose response, truyền qua `onDelivered` để gọi `annotation.markSent` đúng id thật (thay vì suy ra từ local `comments` list) |
| `frontend/src/shared/diff-comments-format.ts` | Giữ nguyên làm fallback — không xoá, theo đúng nguyên tắc "không hỏng tính năng core khi 1 phần phụ trợ lỗi" |

## Không thuộc phạm vi CR này

- Phương án 2/3 ở trên nếu đội triển khai chọn khác — CR này chỉ đặc tả chi
  tiết phương án 1.
- Nâng cấp delivery mechanism của REST mirror route
  (`POST /v1/annotations/send-to-agent`) — route đó đã tự nhận giới hạn
  kiến trúc riêng (không có stream registry per-connection, xem
  TASK-CR-03-05's note), không thuộc phạm vi CR này.
- BR-CR-10 (path normalization) — đã có sẵn ở `channels_annotation_send.go`,
  không cần sửa gì thêm cho CR này.

## Tiêu chí chấp nhận

- [ ] Prompt thật gửi tới agent (quan sát trong PTY/terminal) có dòng
      `Context:`/`Code:` với ±2 dòng xung quanh, khớp `formatFeedbackBlock`'s
      template
- [ ] Cơ chế delivery (guarded paste, đợi idle, Enter riêng, chọn agent
      target) hoạt động y hệt trước CR này — không có regression
- [ ] `annotation.composeReviewPrompt` lỗi (vd. `git-gateway-service` down)
      → vẫn gửi được prompt (fallback client-side), không chặn người dùng
- [ ] Sau khi gửi thành công → `sent_to_agent`/`sent_at` phía server đúng
      cho đúng tập annotation đã gửi (không phải toàn bộ buffer)
- [ ] `go build`/`go vet`/`go test` sạch cho `api-gateway`; `tsc --noEmit`/
      `vitest run` sạch cho frontend

## Impact analysis (gitnexus)

| Symbol | Direction | Risk | Ghi chú |
|---|---|---|---|
| `SendReviewFeedbackToAgent` (`channels_annotation_send.go`, export dùng chung bởi WS channel + REST mirror, TASK-CR-03-03/05) | upstream | Tách hàm mới KHÔNG được đổi signature/hành vi hàm export hiện có — `impact()` bắt buộc trước khi sửa vì REST mirror route đang phụ thuộc | Giữ `SendReviewFeedbackToAgent` nguyên vẹn, hàm compose-only là hàm MỚI gọi chung phần logic con (refactor an toàn: extract, không thay behavior cũ) |
| `sendPromptWithGuardedPasteAndEnter`/`sendNotesToActiveAgentSession` (`active-agent-note-send.ts`) | upstream | Không sửa hàm này — chỉ đổi nguồn `prompt` truyền vào từ nơi gọi | LOW nếu đúng là chỉ đổi nguồn dữ liệu đầu vào, không đổi logic gửi |

`detect_changes({scope: "compare", base_ref: "main"})` bắt buộc trước khi
commit.

## Liên quan

- [CR-ANNOTATE-001](./CR-ANNOTATE-001-persist-diff-comments-to-annotation-service.md) (phụ thuộc cứng)
- [docs/crs/v4/annotate/README.md](./README.md)
- `specs/backend-go/bugs/logic-v1/solutions/SOL-CR-03-review-feedback-prompt-composition.md` (thiết kế gốc của `annotation.sendToAgent`, đã DONE — CR này KHÔNG viết lại, chỉ tách compose khỏi delivery)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go`
- `frontend/src/renderer/src/lib/active-agent-note-send.ts` (delivery guarded-paste thật)
- `frontend/src/shared/diff-comments-format.ts` (format fallback)
- [F08-annotate-ai-diffs.md](../../features/F08-annotate-ai-diffs.md) §"Feedback to Agent"
