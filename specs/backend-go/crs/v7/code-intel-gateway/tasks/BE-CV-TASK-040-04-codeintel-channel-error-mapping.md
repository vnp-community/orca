# BE-CV-TASK-040-04: `codeIntelChannelError`: bóc, che, một dòng, hậu tố JSON, ánh xạ `status.Code`

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_errors.go` (mới), `channels_codeintel_errors_test.go` (mới)
**Depends on:** không (độc lập; TASK-040-03 dùng cùng định dạng thông điệp)
**Status:** [x] DONE

---

## Context

- PQ-02 chặng 4 và UI-API 2.3 là bảng chuẩn. Client đọc **tiền tố `message`** (`error.code` luôn `internal` ở session-client, `session_dialect.go:94-101`; native chỉ có `message`, `envelope.go`).
- Mẫu: `mcpChannelError` (`channels_mcp.go:89-119`: bóc `rpc error: code = ... desc = `, giữ thông điệp có mã, còn lại ánh xạ theo `status.Code`).
- Cú pháp chuẩn: `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`; phần người đọc 1 dòng ≤ 200 ký tự, không mã nguồn, không đường dẫn tuyệt đối, không giá trị tham số; `data` JSON ≤ 2 KiB.
- MCP: `mapExecError` (`tools/executor.go:414`) bắt `^([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+):` trên `err.Error()`; nên hàm này phải trả `errors.New(...)` thuần (không `status`), để CR-041 giữ được mã `CODEINTEL_*`.

## Việc cần làm

1. Hằng: `codeIntelMsgMaxRunes = 200`, `codeIntelDataMaxBytes = 2048`, `codeIntelTimeoutSuffix = {"retryAfterMs":3000,"inProgress":true}`.
2. `func codeIntelChannelError(err error) error`:
   a. `nil` => `nil`;
   b. lấy `msg` từ `status.FromError(err)` (nếu `ok` và code != `OK`) hoặc `err.Error()`; bóc `rpc error: code = X desc = ` lần xuất hiện **cuối** (regexp giống `grpcErrorWrapper` ở `channels_mcp.go`; không import từ file MCP, khai báo riêng);
   c. nếu khớp regexp chuẩn => dựng lại `CODE: <human>[ | <json>]`: `human` = một dòng (thay mọi `\s+` bằng một khoảng trắng, trim), cắt `codeIntelMsgMaxRunes` rune (thêm `…`? **không**; cắt cứng), che đường dẫn tuyệt đối `(?:[A-Za-z]:\\|/)[^/\\\s|]+(?:[/\\][^/\\\s|]+){2,}` => `<path>`; `json` giữ nếu là object hợp lệ ≤ 2 KiB; nếu vượt và có mảng `candidates` thì bỏ phần tử cuối lặp tới khi đủ (tối đa 10 phần tử theo UI-API 2.3), còn lại bỏ cả hậu tố;
   d. không khớp => bảng `status.Code` (UI-API 2.3 cuối mục): `Unavailable|Unimplemented` => `CODEINTEL_UNAVAILABLE: code-intel unavailable`; `DeadlineExceeded` hoặc `errors.Is(err, context.DeadlineExceeded)` => `CODEINTEL_TIMEOUT: code-intel did not respond in time | {"retryAfterMs":3000,"inProgress":true}`; `Canceled` => `CODEINTEL_TIMEOUT: request cancelled` (không hậu tố); `NotFound|PermissionDenied` => `CODEINTEL_NOT_AUTHORIZED: not found or not permitted`; `InvalidArgument` => `CODEINTEL_INVALID_PARAMS: invalid params`; `FailedPrecondition` => `CODEINTEL_TOOL_FAILED: tool failed`; `ResourceExhausted` => `CODEINTEL_RESPONSE_TOO_LARGE: response too large` nếu `msg` (chữ thường) chứa `size|bytes|too large|larger than max`, ngược lại `CODEINTEL_RATE_LIMITED: rate limited`; `Aborted|AlreadyExists` => `CODEINTEL_VERSION_CONFLICT: version conflict`; còn lại `CODEINTEL_INTERNAL: internal error`. Thông điệp ánh xạ là **hằng**, không chép `msg` gốc.
   e. trả `errors.New(<kết quả>)`.
3. Các lỗi do gateway sinh ra (`codeIntelParamError`, `errCodeIntelUnavailable`, `errCodeIntelNotFound`, `errCodeIntelNotAuthorized`, `errCodeIntelResponseTooLarge(bytes, limit)`) dựng ở đây hoặc `channels_codeintel_runner.go` dưới cùng định dạng và **không** đi lại qua `codeIntelChannelError`.
4. Không thêm kênh `codeIntel.*` vào `sensitiveArgChannels` (`channel_args_redaction.go`); quyết định Q5 của solution xử lý riêng.

## Kiểm thử

Bảng (`channels_codeintel_errors_test.go`):
- mỗi `codes.*` trong bảng: `status.Error(code, "boom /home/dev/proj/secret.go")` => thông điệp là hằng, không chứa `/home/dev`;
- `status.Error(codes.FailedPrecondition, "CODEINTEL_DISABLED: code intelligence is disabled")` => nguyên mã (`CODEINTEL_DISABLED`), **không** thành `TOOL_FAILED`;
- thông điệp có nhiều dòng, `\t`, > 200 rune => một dòng ≤ 200;
- `CODEINTEL_AMBIGUOUS_SYMBOL: ambiguous symbol | {"candidates":[...12 phần tử...]}` => ≤ 10 phần tử, ≤ 2 KiB, JSON hợp lệ;
- hậu tố không phải JSON object (`| nope`) => bị bỏ, mã giữ;
- đường dẫn `/home/dev/x/y.go`, `C:\Users\a\b\c.go` => `<path>`; `/a/b` (2 phân đoạn) giữ nguyên; URL `https://host/a/b/c` không bị phá sai (khẳng định hành vi hiện tại; ghi rõ trong test);
- `rpc error: code = Unavailable desc = CODEINTEL_TOOL_UNAVAILABLE: x | {"tool":"gitnexus","reason":"unsupported_version"}` => qua nguyên mã và `data`;
- `context.DeadlineExceeded` bọc `fmt.Errorf("%w")`;
- kết quả luôn khớp regexp chuẩn của UI-API 2.3 (test thuộc tính trên toàn bảng);
- khẳng định `mapExecError` của `tools` bắt được mã (test nằm ở TASK-041-05, nhắc ở đây để không quên).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelChannelError'`.

## Tiêu chí hoàn thành

- [x] Bảng 2.3 UI-API được phủ từng dòng ánh xạ.
- [x] Không thông điệp nào chứa mã nguồn, đường dẫn tuyệt đối từ 3 phân đoạn, hay tham số.
- [x] `NotFound|PermissionDenied` => `NOT_AUTHORIZED` (lệch L3 với CR).
- [x] Hậu tố `inProgress` chỉ do gateway thêm khi **chính nó** hết hạn.

## Rủi ro và lưu ý

- `inProgress:true` khi gateway tự hết hạn là khẳng định của PQ-13 về việc service tiếp tục thu thập nền; nếu service đã bị hủy theo ctx thì client thử lại tốn công vô ích. Service phải dùng ctx tách rời cho việc nền (CR-021/022).
- Regexp che đường dẫn có thể bắt nhầm chuỗi như `a/b/c` trong mô tả; chấp nhận (an toàn hơn), ghi trong test.
