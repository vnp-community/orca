# TASK-REQ-042-01: Gói `common/agentrelay`: `ExecPrompt` và `Complete`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `backend-go/common`
**File:** `backend-go/common/agentrelay/{target,exec_prompt,complete,errors,result_parse,options}.go` + test (mới); `backend-go/common/go.mod` (nếu cần phụ thuộc `proto`)
**Depends on:** Không
**Status:** [ ] TODO

---

## Context

- Bốn bản sao `agentExecPromptParams` khác nhau; ba bản `ai.complete`.
- `common/auditclient` đã import `proto`, nên `common` được phép phụ thuộc `proto`.
- Tên JSON phải khớp agent (`ai-complete-handler.ts`, `agent-rpc-dispatch-agent-exec.ts`).

## Việc cần làm

1. Viết `Target`, `ExecParams` (hợp của bốn struct, `omitempty` như bản cũ), `ExecResult` (kiểu có tên cho `Stats`, `Changes`, `Parsed`, `Truncated`, `Warnings`, `Applied`), `ExecPrompt`.
2. Hàm dựng `ReadOnly(...)`, `WriteProfile(...)`, `Contract(...)` chuyển đúng logic của `buildAgentParams` (request-service): chỉ-đọc bỏ trust preset; hồ sơ ghi gửi `trustPreset` mặc định; từ chối đồng thời chỉ-đọc và hồ sơ ghi.
3. `Complete(ctx, Target, CompleteParams)`: chỉ gửi trường có giá trị; **không có** `resolvedApiKey`.
4. `errors.go`: `ErrReadonlyUnsupported` (khớp `READONLY_MODE_UNSUPPORTED`), `ErrKnowledgeHubUnavailable` (`KNOWLEDGE_HUB_UNSUPPORTED`, `KNOWLEDGE_ROOT_*`, `KNOWLEDGE_CONFIG_*`), hết hạn (`DeadlineExceeded`), mọi lỗi khác bọc kèm method.
5. `result_parse.go`: chịu được agent cũ không có `usage`, `toolCalls`, `changes`, `parsed`; `ExitCode` con trỏ.
6. Tuỳ chọn `WithMetadata` (hàm gắn metadata tenant/người dùng) và `WithTimeoutSlack`; mặc định slack bằng `slackOverTimeout` hiện có.
7. Thông báo đầu ra theo dòng (`agent.execPrompt.output`) vẫn do `infra-fleet` chuyển: gói **không** nuốt hay đổi; ghi rõ trong doc gói.
8. Thêm vào `go.work`/CI nếu cần; README ngắn của gói.

## Kiểm thử

- JSON gửi đi: với từng luồng (request chỉ-đọc, request hồ sơ ghi, request contract, task contract, workflow, orchestration, git `Complete`) dựng `ExecParams`/`CompleteParams` và so JSON với **bản chụp của struct cũ** (`testdata/wire/*.json` chụp từ mã hiện tại trước khi xoá).
- Phân loại lỗi, timeout dư, kết quả đầy đủ và kết quả agent cũ.
- `go test ./common/agentrelay/... -v`.

## Tiêu chí hoàn thành

- [ ] JSON gửi đi của mọi luồng giống hệt trước khi gom (so khớp bản chụp).
- [ ] Không có đường nào gửi `resolvedApiKey`.
- [ ] Lỗi phân loại giống bản request-service.

## Rủi ro và lưu ý

- Chụp JSON cũ phải làm **trước** khi xoá struct cũ ở task 02 và 03.
- Giữ gói không phụ thuộc `internal/` của service.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
