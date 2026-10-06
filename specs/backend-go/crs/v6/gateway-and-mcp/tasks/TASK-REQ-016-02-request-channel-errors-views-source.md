# TASK-REQ-016-02: Lỗi kênh, struct view camelCase và luật nguồn Request

**From Solution:** BE-REQ-SOL-016
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_errors.go` (mới), `.../channels_request_views.go` (mới), `.../channels_request_source.go` (mới), cùng các `*_test.go`
**Depends on:** TASK-REQ-016-01
**Status:** `[ ] TODO`

---

## Context

- `channels_mcp.go:85-110` `mcpChannelError`: bỏ tiền tố `rpc error: code = X desc = `, giữ nếu khớp regex mã, ánh xạ `codes.Unavailable`. Mẫu cho `requestChannelError`.
- `channels_task_source.go:13-19`: mẫu struct view camelCase (`taskSourceView`).
- `tool_origin_context.go:12-27`: `ToolOrigin`, `toolOriginFromContext` (không export, cùng package dùng được).
- `channels.go` ghi chú BUG-023: JSON mặc định của proto ra snake_case, phải qua view. `Dispatch` có `normalizeNilSlices` chuẩn hoá slice nil thành `[]` nhưng bỏ qua `proto.Message`.
- Hợp đồng: CONTRACT mục 1 (kiểu), mục 5 (mã lỗi), mục 6.1 (nguồn).

## Việc cần làm

1. `channels_request_errors.go`: `func requestChannelError(err error) error`. Quy tắc: `nil` thì `nil`; lấy `status.Convert(err).Message()`; cắt phần trước `rpc error: code = ... desc = `; nếu khớp `^REQUEST_[A-Z0-9_]+: ` giữ nguyên; `codes.Unavailable` hoặc `DeadlineExceeded` không có mã thì `REQUEST_UNAVAILABLE: request service unavailable`; `PermissionDenied` không mã thì `REQUEST_APPROVAL_FORBIDDEN: forbidden`; còn lại cắt 300 ký tự và đặt tiền tố `REQUEST_INTERNAL: `. Không bao giờ nối giá trị đầu vào vào thông điệp.
2. `channels_request_views.go`: struct và hàm đổi cho `requestView`, `typeHistoryView`, `solutionView` (`Options json.RawMessage`, `ChosenOption int`, `ChosenOptionID string`), `analysisRunView`, `approvalView`, `backlogRequestRowView`, `backlogTaskRowView`, `backlogGroupView`. Tên khoá đúng CONTRACT mục 1. `toRequestView(r *requestv1.Request, withBody bool)`. Enum proto đổi sang chuỗi chữ thường (`SOLUTION_KIND_SOLUTION` thành `solution`, `PENDING` thành `pending`); giá trị lạ thành chuỗi rỗng, không panic. `chosenOptionId` suy từ `options.options[i].id`, lỗi parse thì bỏ.
3. `channels_request_source.go`: `resolveRequestSource(ctx, in createSourceArgs)` theo bảng CONTRACT 6.1; trả `*requestv1.RequestSource` và `origin` (proto `RequestOrigin` do CR-REQ-004 thêm; nếu chưa có, để hàm trả `origin` là `nil` và `TODO` ghi rõ). Lỗi `REQUEST_SOURCE_FORBIDDEN: source provider not allowed from this client`.
4. Hằng số enum hợp lệ cho kiểm ở gateway: tập 11 loại, `ReturnStage`, `LinkReason`, `analysisMode`, `mode` (dùng ở các task sau).

## Kiểm thử

- `channels_request_errors_test.go`: bảng lỗi gRPC thô, đã có mã, `Unavailable`, `body` marker không xuất hiện (`SECRET-BODY-MARKER` đưa vào mọi đầu vào, ép lỗi, `strings.Contains(err.Error(), marker) == false`).
- `channels_request_views_test.go`: marshal mọi view, quét khoá JSON không chứa `_` (trừ nội dung `options`); nil slice thành `[]`; enum lạ không panic.
- `channels_request_source_test.go`: 4 dòng bảng 6.1 và giả mạo `mcp|webhook|manual`; ngữ cảnh có `ToolOrigin` thì `provider=mcp`, `site=ClientName`, bất kể input.
- Lệnh: `go test ./internal/adapter/wscompat/... -run 'Request'`.

## Tiêu chí hoàn thành

- [ ] Ba file và test có, xanh.
- [ ] Không view nào để lộ khoá snake_case; `body` chỉ có khi `withBody`.
- [ ] Giả mạo nguồn bị từ chối đúng mã.

## Rủi ro và lưu ý

- Tên field proto chỉ đúng khi CR-REQ-001 đến 009 đã sinh code; nếu lệch, sửa view, không sửa hợp đồng ở đây mà báo ở CONTRACT.
- `Options` giữ nguyên khoá (CONTRACT C13); đừng chạy `camelize`.
