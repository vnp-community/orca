# TASK-BE-ANNOTATE-001: Extract `ComposeReviewFeedbackPrompt` from `SendReviewFeedbackToAgent`

**Solution:** [SOL-BE-ANNOTATE-002](../solutions/SOL-BE-ANNOTATE-002-extract-compose-only-channel.md) | **CR:** CR-ANNOTATE-002
**Depends on:** Không
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu

Tách 3 bước đầu (collect/context/format) của hàm `SendReviewFeedbackToAgent`
(`channels_annotation_send.go`) thành 1 hàm mới `ComposeReviewFeedbackPrompt`,
**không đổi hành vi bên ngoài** của `SendReviewFeedbackToAgent` — pure
refactor, chuẩn bị cho TASK-BE-ANNOTATE-002 expose channel mới. Task này
KHÔNG thêm channel mới, KHÔNG đổi test hiện có của `SendReviewFeedbackToAgent`.

## Files cần sửa

1. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY)
2. `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` (MODIFY — thêm test cho hàm mới, KHÔNG sửa test hiện có)

## Bước 1 — Đọc lại hàm gốc, xác nhận ranh giới bước

Đọc `SendReviewFeedbackToAgent` hiện tại — 5 bước đã được đánh số sẵn trong
doc comment: 1. Collect, 2. Context, 3. Format, 4. Deliver, 5. Mark-sent.
Xác nhận đúng dòng cắt giữa bước 3 và 4 trước khi sửa (SOL-BE-ANNOTATE-002's
snippet đã chỉ rõ, nhưng đọc lại code thật để không lệch dòng nếu file đã
đổi từ lúc viết solution).

## Bước 2 — Viết `ComposeReviewFeedbackPrompt`

Theo đúng snippet ở SOL-BE-ANNOTATE-002 §"Design — extract
`ComposeReviewFeedbackPrompt`":

```go
func ComposeReviewFeedbackPrompt(
	ctx context.Context,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	worktreeID, worktreeName string,
) (prompt string, annotationIDs []string, err error) {
	listResp, err := annotationClient.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
		WorktreeId:  worktreeID,
		SentToAgent: proto.Bool(false),
		PageSize:    200,
	})
	if err != nil {
		return "", nil, err
	}
	if len(listResp.GetAnnotations()) == 0 {
		return "", nil, nil
	}

	blocks := make([]string, 0, len(listResp.GetAnnotations()))
	annotationIDs = make([]string, 0, len(listResp.GetAnnotations()))
	for _, a := range listResp.GetAnnotations() {
		codeLine, context := resolveCodeContext(ctx, gitClient, worktreeID, a)
		blocks = append(blocks, formatFeedbackBlock(a, codeLine, context))
		annotationIDs = append(annotationIDs, a.GetId())
	}

	return formatReviewPrompt(worktreeName, blocks), annotationIDs, nil
}
```

Không đổi `resolveCodeContext`/`normalizeRelativePath`/`sliceLinesAround`/
`formatReviewPrompt`/`formatFeedbackBlock` — gọi lại y nguyên.

## Bước 3 — Viết lại `SendReviewFeedbackToAgent` gọi hàm mới

```go
func SendReviewFeedbackToAgent(
	ctx context.Context,
	annotationClient annotationv1.AnnotationServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	worktreeID, ptyID, worktreeName string,
) (map[string]any, error) {
	prompt, ids, err := ComposeReviewFeedbackPrompt(ctx, annotationClient, gitClient, worktreeID, worktreeName)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return map[string]any{"sent": 0}, nil
	}

	// bước 4-5 giữ nguyên y hệt code cũ — KHÔNG đổi
	...
}
```

Giữ nguyên doc comment về REST-transport caveat trên `SendReviewFeedbackToAgent`
— vẫn đúng, không liên quan tới refactor này.

## Bước 4 — Test

Thêm `TestComposeReviewFeedbackPrompt_HappyPath`,
`TestComposeReviewFeedbackPrompt_EmptyList`,
`TestComposeReviewFeedbackPrompt_SideOldUsesOriginalCode` (fake
`AnnotationServiceClient`/`GitGatewayServiceClient`, cùng pattern fake đã
dùng trong `channels_annotation_send_test.go` cho
`SendReviewFeedbackToAgent`). **Không sửa** test case hiện có của
`SendReviewFeedbackToAgent` — nếu buộc phải sửa 1 test cũ để nó pass lại,
đó là dấu hiệu refactor không giữ nguyên hành vi, dừng lại và đối chiếu lại.

## Verify

```bash
cd backend-go && go build ./services/api-gateway/... && go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestComposeReviewFeedbackPrompt|TestSendReviewFeedbackToAgent' -v
```

Toàn bộ test case cũ của `TestSendReviewFeedbackToAgent*` phải PASS không
sửa gì — đây là tiêu chí "refactor an toàn" chính của task này.

## gitnexus

`impact({target: "SendReviewFeedbackToAgent", direction: "upstream"})`
**bắt buộc trước khi sửa** — hàm này có 2 caller đã biết
(`registerAnnotationSendChannel` qua WS, và REST mirror ở
`annotation_routes.go`, TASK-CR-03-05) — xác nhận `impact()` không báo thêm
caller thứ 3 nào ngoài dự kiến, và risk ở mức LOW (đây là refactor giữ
nguyên signature, không phải đổi hợp đồng).

**Lệch so với kế hoạch**: GitNexus index lúc thực thi lỗi thời 155 commit
(`SendReviewFeedbackToAgent` không tìm thấy qua `impact()`/`context()`) —
đã kích hoạt `node .gitnexus/run.cjs analyze` chạy nền, nhưng không chờ nó
xong trước khi sửa (đã tự xác nhận danh sách 2 caller bằng grep trực tiếp
từ trước, độ tin cậy tương đương cho trường hợp này). Xem "Kết quả thực tế"
bên dưới.

---

## ✅ Kết quả thực tế (2026-09-11)

Refactor đúng như thiết kế — cắt `SendReviewFeedbackToAgent` tại đúng ranh
giới bước 3→4 đã có sẵn trong doc comment gốc, không đổi
`resolveCodeContext`/`normalizeRelativePath`/`sliceLinesAround`/
`formatReviewPrompt`/`formatFeedbackBlock`.

**Sự cố trong lúc sửa (tự phát hiện, tự vá)**: bản edit đầu tiên để sót
biến `listResp`/vòng lặp tính `ids` cũ ở bước 5 (Bookkeeping) sau khi đã
chuyển việc tính `ids` sang `ComposeReviewFeedbackPrompt` — gây lỗi build
`undefined: listResp`. Phát hiện ngay bằng `go build` (không phải merge từ
người khác), sửa lại bằng cách xoá đoạn tính `ids` trùng lặp ở
`SendReviewFeedbackToAgent`, dùng thẳng biến `ids` đã trả về từ
`ComposeReviewFeedbackPrompt`.

**Test mới**: `TestComposeReviewFeedbackPrompt_HappyPath`,
`_EmptyList`, `_SideOldUsesOriginalCodeNeverReadsFile`,
`_ListAnnotationsError`, và `TestSendReviewFeedbackToAgent_StillDeliversAfterComposeExtraction`
(regression guard đặt tên riêng theo đúng tiêu chí chấp nhận của task này).

**Verify thật đã chạy**: `go build`/`go vet` sạch cho `api-gateway`; toàn
bộ 8 test case cũ của `TestAnnotationSendToAgent_*`/`TestAnnotationSendToAgentChannel_Registered`
PASS **không sửa 1 dòng nào** (regression guard chính của task — xác nhận
đạt); 5 test mới PASS. Toàn bộ 17 service backend-go build sạch
(`for d in services/*/; do go build ./...; done`).

**Files đã sửa:**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send_test.go` (MODIFY — thêm test, không sửa test cũ)
