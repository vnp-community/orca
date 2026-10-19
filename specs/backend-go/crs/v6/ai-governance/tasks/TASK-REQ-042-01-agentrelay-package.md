# TASK-REQ-042-01: Gói `common/agentrelay`: `ExecPrompt` và `Complete`

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P1
**Service:** `backend-go/common`
**File:** `backend-go/common/agentrelay/{target,exec_prompt,complete,errors,result_parse,options}.go` + test (mới); `backend-go/common/go.mod` (nếu cần phụ thuộc `proto`)
**Depends on:** Không
**Status:** [x] DONE (2026-10-19)

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

- [x] JSON gửi đi của mọi luồng giống hệt trước khi gom (so khớp bản chụp). Cùng tập khóa và giá trị cho cả 16 ca; giống từng byte cho 6 ca request-service; thứ tự khóa của task/workflow/orchestration khác (vô hại với `JSON.parse` của agent). Xem ghi chú.
- [x] Không có đường nào gửi `resolvedApiKey`.
- [x] Lỗi phân loại giống bản request-service (`KNOWLEDGE_CONFIG_` khớp tiền tố, rộng hơn `KNOWLEDGE_CONFIG_MISSING` cũ).

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

---

## Ghi chú triển khai (2026-10-19)

**Blast radius:** gói mới, chưa có caller nào; GitNexus không dùng được nên không chạy `impact`/`detect_changes` (không có symbol sẵn có bị sửa). Không sửa file của service nào.

### File (tất cả dưới `backend-go/common/agentrelay/`)

`doc.go`, `README.md`, `target.go`, `options.go`, `relayer.go` (`Client`, `Relayer`, `New`), `exec_params.go` (`ExecParams`), `exec_builders.go` (`Run`, `Plain`, `ReadOnly`, `WriteProfile`, `Contract`), `exec_prompt.go`, `exec_result.go` (kiểu có tên: `Applied`, `Changes`, `Parsed`, `Truncated`, `Stats`, `Usage`), `result_parse.go` (+ `TruncateUTF8`), `complete.go`, `errors.go`. Test: `wire_test.go`, `legacy_wire_test.go`, `builders_test.go`, `exec_prompt_test.go`, `complete_test.go`, `errors_test.go`, `result_parse_test.go`, `fake_client_test.go`. Golden: `testdata/wire/*.json` (16 file). Mọi file dưới 300 dòng, không disable lint. `go.mod`/`go.sum` của `common` không đổi (`proto` đã có từ `auditclient`, kèm `replace ../proto`; `go build` chạy cả với workspace lẫn `GOWORK=off`).

### Thiết kế đáng chú ý

- API là `Relayer` (không trạng thái, an toàn khi dùng song song) thay cho hàm gói, vì cần client gRPC; `Client` là interface hẹp (`Relay`, `RelayByDevServer`) để test bằng giả.
- `Target.DevServerID != ""` -> `RelayByDevServer`, ngược lại `Relay`. Target rỗng gửi nguyên (infra-fleet tự trả lỗi), giữ hành vi cũ.
- `ErrTimeout == context.DeadlineExceeded` (cùng giá trị) để các so sánh `==`/`errors.Is` cũ vẫn chạy.
- Lỗi `ReadonlyUnsupported` trả trần (như bản cũ); `KnowledgeHubUnavailable` bọc bằng `%w: %w` nên giữ nguyên nguyên nhân gRPC (bản cũ dùng `%v`).
- `ai.complete`: lỗi có dạng `AI_COMPLETE_XXX: chi tiết` thành `*CompleteError{Code, Detail, Cause}`; service tự ánh xạ sang mã người dùng (`aiCompleteCodes` của request-service ở lại service). Timeout của `Complete` là `CompleteParams.Timeout` (không gửi đi, đúng bằng timeout cũ, không có slack).
- Thêm `WithoutCallDeadline()` (ngoài `WithMetadata`, `WithTimeoutSlack` theo task): task-service hiện không đặt deadline cho RPC; task 03 dùng tuỳ chọn này nếu muốn giữ nguyên.
- Kiểm hợp lệ trước khi gửi (không gọi client): chỉ-đọc cùng hồ sơ ghi (`ErrConflictingAccess`), `proposeChangesets` không có `knowledgeRoot` (`ErrProposeWithoutRoot`); `WriteProfile` thiếu tên (`ErrProfileRequired`). Không kiểm prompt/worktree rỗng hay nonce rỗng: agent tự từ chối, để không đổi hành vi cũ (và test của service).
- Phân tích kết quả: agent cũ không có `usage/toolCalls/changes/parsed/truncated/applied`, `exitCode` null hoặc thiếu (con trỏ), `null` cho từng đối tượng, khóa lạ đều được chấp nhận. Số liệu `usage` âm bị chặn về 0; tên tool tối đa 128, mỗi tên cắt 100 byte (như `statsFromAgent`). Cắt stdout/stderr là chính sách của caller: gói chỉ xuất `TruncateUTF8`.
- Sửa lỗi ngầm của task-service: `agentExecPromptResult.Truncated` ở đó là `bool`, nhưng agent gửi đối tượng `{stdout,stderr}`; khi cắt đầu ra, unmarshal sẽ lỗi. Gói dùng `*Truncated{Stdout, Stderr}`.
- `ExecResult.Parsed.Value` là `[]byte` JSON (nội dung khối kết quả do caller định nghĩa), không có `json.RawMessage` lộ ra ngoài.
- Không gửi `taskId`/`projectId` của agent.execPrompt (agent đọc nhưng chưa caller Go nào dùng).

### Chứng minh JSON không đổi

Không sửa gói của agent khác. Thay vào đó `legacy_wire_test.go` chép nguyên định nghĩa 4 struct cũ (cùng tag) và logic dựng cũ (`buildAgentParams`, `contractRunOptions.apply`, map của `ai.complete`). Đã kiểm bản chép với nguồn bằng script so sánh tập tag JSON của từng struct (6 cặp, đều khớp, lúc 2026-10-19 trước khi task 02/03 xoá bản gốc). `TestWireEquivalence` chạy 16 ca (request chỉ-đọc, chỉ-đọc + knowledge, hồ sơ ghi, hồ sơ ghi + knowledge, contract, plain; task plain/profile/contract/contract + knowledge; workflow trần/profile; orchestration; `ai.complete` request có/không `accountId`, git) và kiểm: (1) JSON của struct cũ khớp golden, (2) JSON của builder mới khớp golden theo nội dung (khóa sắp xếp), (3) với 6 ca request-service khớp từng byte (thứ tự trường của `ExecParams` giữ thứ tự cũ). Golden sinh bằng `-update` từ struct cũ. Thử đột biến (bỏ `omitempty` của `reportChanges`) làm 7 ca fail rồi hoàn lại.

Khác biệt đã biết: (a) thứ tự khóa của task/workflow/orchestration/`ai.complete` khác bản cũ; (b) request-service cũ gửi `stepId` và `env` không `omitempty`, bản mới bỏ qua khi rỗng (request-service luôn có `env` 2 khóa, `stepId` là `run.ID` ở cả hai nơi gọi (`run_agent_readonly_analysis.go`, `engine_openspec.go`), nên JSON không đổi trong thực tế).

### Lệnh và kết quả

```
cd backend-go/common
go build ./...                                          # OK
go vet ./agentrelay/...                                 # OK
go test ./agentrelay/... -race -count=1 -v              # 25 test PASS, race-clean
GOWORK=off go build/vet/test ./agentrelay/... -race     # OK
```

### Chưa kiểm chứng

- `golangci-lint`/`staticcheck` không chạy được (build bằng go1.25, repo dùng toolchain 1.26), nên lint chưa xác nhận; `gofmt` và `go vet` sạch.
- Chưa chạy qua CI của repo (`backend-go/ci/`); gói nằm trong module `common` đã có trong `go.work`, chưa xác nhận CI chạy `go test ./...` cho `common`.
- Chưa gọi `infra-fleet` hay agent thật; mọi kiểm thử dùng client giả. Việc thay caller của 4 service là task 02/03.
