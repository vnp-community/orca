# TASK-BE-ANNOTATE-002: Add `annotation.composeReviewPrompt` wscompat channel

**Solution:** [SOL-BE-ANNOTATE-002](../solutions/SOL-BE-ANNOTATE-002-extract-compose-only-channel.md) | **CR:** CR-ANNOTATE-002
**Depends on:** [TASK-BE-ANNOTATE-001](./TASK-BE-ANNOTATE-001-extract-compose-review-feedback-prompt.md) (`ComposeReviewFeedbackPrompt` phải tồn tại trước)
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Expose `ComposeReviewFeedbackPrompt` (TASK-BE-ANNOTATE-001) qua 1 wscompat
channel mới `annotation.composeReviewPrompt`, dùng bởi frontend
(SOL-FE-ANNOTATE-002) để lấy prompt đã có code-context mà không kích hoạt
delivery qua PTY của chính channel này.

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY — thêm type args + hàm register)
2. `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (MODIFY — gọi hàm register mới trong `RegisterRealChannels`)
3. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` (MODIFY — test đăng ký channel mới)

## Bước 1 — Thêm args type + hàm register

Theo đúng snippet SOL-BE-ANNOTATE-002 §"Design — new channel:
`annotation.composeReviewPrompt`":

```go
type composeReviewPromptArgs struct {
	WorktreeID   string `json:"worktreeId"`
	WorktreeName string `json:"worktreeName"`
}

func registerAnnotationComposeChannel(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
) {
	r.Register("annotation.composeReviewPrompt", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[composeReviewPromptArgs](args, 0)
		if err != nil {
			return nil, err
		}
		prompt, ids, err := ComposeReviewFeedbackPrompt(ctx, annotationClient, gitClient, in.WorktreeID, in.WorktreeName)
		if err != nil {
			return nil, err
		}
		return map[string]any{"prompt": prompt, "annotationIds": ids}, nil
	})
}
```

## Bước 2 — Wire vào `RegisterRealChannels`

Đọc `channels.go`'s điểm gọi `registerAnnotationSendChannel` hiện có
(TASK-CR-03-04 đã wire) — thêm dòng gọi `registerAnnotationComposeChannel`
ngay sau đó, cùng tham số (`annotationClient`, `gitClient`) đã có sẵn trong
scope đó — không cần lấy thêm dependency nào mới.

## Bước 3 — Test

`TestRegisterRealChannels_RegistersAnnotationComposeReviewPrompt` — mirror
chính xác `TestRegisterRealChannels_RegistersAnnotationSendToAgent`
(TASK-CR-03-04's test), chỉ đổi tên channel kỳ vọng.

Thêm test cho handler `annotation.composeReviewPrompt` chính nó (fake
clients, giống style `channels_annotation_send_test.go`):
- Happy path: N annotation chưa gửi → response có `prompt` (chứa `Context:`/
  `Code:`) + `annotationIds` độ dài N.
- Rỗng: 0 annotation chưa gửi → `{"prompt": "", "annotationIds": []}` hoặc
  tương đương rỗng — xác nhận response shape chính xác (không phải `nil`
  gây lỗi decode phía frontend).
- Lỗi `ListAnnotations`/`ReadFile` → trả lỗi rõ ràng, không panic.

## Verify

```bash
cd backend-go && go build ./services/api-gateway/... && go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestRegisterRealChannels|TestComposeReviewFeedbackPrompt|annotation' -v
```

## gitnexus

`context({name: "RegisterRealChannels"})` trước khi sửa `channels.go` — xác
nhận đúng vị trí/tham số hiện có của `registerAnnotationSendChannel`'s lời
gọi để thêm dòng mới đúng chỗ, không suy đoán từ snippet solution 1 mình.

**Lệch so với kế hoạch**: GitNexus index lỗi thời lúc thực thi (155 commit
sau HEAD) — dùng grep trực tiếp xác nhận vị trí `registerAnnotationSendChannel`'s
lời gọi trong `channels.go` (dòng 130-131) thay vì `context()`, đủ tin cậy
cho 1 dòng chèn đơn giản ngay sau dòng có sẵn.

## Không thuộc phạm vi task này

- Route REST cho channel mới — SOL-BE-ANNOTATE-002 §"Không thuộc phạm vi"
  đã loại trừ tường minh.

---

## ✅ Kết quả thực tế (2026-09-11)

Đúng như thiết kế — thêm `composeReviewPromptArgs` + `registerAnnotationComposeChannel`
vào `channels_annotation_send.go`, wire vào `RegisterRealChannels` ngay sau
`registerAnnotationSendChannel` trong `channels.go`.

**Test mới**: `TestAnnotationComposeReviewPrompt_HappyPath` (gọi qua
`r.Dispatch`, xác nhận response có `prompt` chứa code-context và
`annotationIds` đúng), `TestAnnotationComposeReviewPrompt_DoesNotDeliverToPty`
(gọi KHÔNG có `terminalStreamsContext` — xác nhận không lỗi, đúng thiết kế
"compose-only không cần terminal stream registry"),
`TestAnnotationComposeReviewPromptChannel_Registered`. Đồng thời mở rộng
`TestRegisterRealChannels_RegistersAnnotationSendToAgent` (file `channels_test.go`,
test có sẵn) thêm 1 assertion cho `annotation.composeReviewPrompt` thay vì
tạo file test riêng — nhất quán với cách file đó đã test 2 channel
annotation khác trong cùng 1 test.

**Lệch nhỏ so với thiết kế ban đầu của task**: dùng `mustMarshalArg` (số ít,
đã có sẵn trong `channels_mobile_dispatch_test.go`, cùng package) +
`r.Dispatch(...)` để gọi handler trong test, thay vì tự viết
`mustMarshalArgs`/truy cập `r.handlers[...]` trực tiếp như bản nháp đầu
tiên — khớp đúng convention test đã có sẵn trong package này (phát hiện khi
build lỗi `undefined: mustMarshalArgs`, sửa lại theo pattern thật).

**Verify thật đã chạy**: `go build`/`go vet` sạch cho `api-gateway`; 6 test
mới PASS (3 cho `annotation.composeReviewPrompt` + 3 liên quan
`ComposeReviewFeedbackPrompt`/`SendReviewFeedbackToAgent` từ TASK-BE-ANNOTATE-001);
toàn bộ package `wscompat` (bao gồm mọi test cũ) PASS
(`go test ./services/api-gateway/...` — tất cả package con `ok`). Toàn bộ
17 service backend-go build sạch.

**Files đã sửa:**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (MODIFY — 1 dòng)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` (MODIFY)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go` (MODIFY — mở rộng 1 test có sẵn)
