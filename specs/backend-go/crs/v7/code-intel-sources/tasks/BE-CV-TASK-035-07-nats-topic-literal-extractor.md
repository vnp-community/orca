# BE-CV-TASK-035-07: Trích Topic NATS từ mã Go bằng `go/parser` (publisher, subscriber, stream, delivery)

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gosourcescan/nats_subject_literals.go` và `nats_subject_literals_test.go` (mới); `internal/adapter/gosourcescan/subject_role_rules.go` (mới)
**Depends on:** BE-CV-TASK-035-03, BE-CV-TASK-035-06 (khai báo `GoSourceScanner`)
**Status:** [x] DONE

---

## Context

CR-035 §2.4. Hiện trạng đọc được: `common/eventbus/eventbus.go` có `Publisher.EnsureStream`, `Publish`, `PublishDedup`, `Consumer.Subscribe(streamName, consumerName, subject, fn)`, `SubscribeEphemeral(streamName, subject, fn)`; `EnsureStream` được gọi trong `cmd/server/main.go` của hầu hết service (ví dụ `project-service` `"PROJECT"` ↔ `orca.project.>`, `mcp-service` `"MCP"` ↔ `orca.mcp.>`, `infra-fleet-service` có ít nhất `INFRAFLEET`, `INFRA`, `INFRA_FLEET`). Subject có ở nhiều tầng (domain const, bảng `SubjectBinding`, `api-gateway`); **không phải mọi literal là phát hoặc nghe**.

## Việc cần làm

1. `nats_subject_literals.go`: `ScanSubjects(content []byte, file string) ([]SubjectUse, error)` dùng `go/parser` + `ast.Inspect`. Chỉ lấy literal chuỗi khớp `^orca\.[a-z0-9_]+(\.[a-z0-9_]+){1,3}$` hoặc `^orca\.[a-z0-9_]+\.>$` nằm trong: khai báo `const`/`var`, composite literal `SubjectBinding{…}` (trường `Subject`, `StreamName`), đối số của lời gọi có tên selector cuối `Publish|PublishDedup|EnsureStream|Subscribe|SubscribeEphemeral`, trường `Subject:` của struct bất kỳ. Comment, chuỗi log (`slog.*`, `fmt.*f`) bị bỏ. `SubjectUse{Subject, Stream, Role(publisher|subscriber|unknown), Delivery, File, Line}`.
2. `subject_role_rules.go`: gán vai trò theo solution 035 §2.D.3: publisher `declared` khi đối số `Publish*`/trường outbox hoặc tên tệp khớp `adapter/eventbus/publisher*.go`; subscriber `declared` khi nằm trong `[]SubjectBinding{}` hoặc đối số `Subscribe*` hoặc tệp `adapter/{eventbus,natsconsumer}/*`; còn lại `unknown`/`inferred`. `Delivery`: `ephemeral` chỉ khi lời gọi là `SubscribeEphemeral` cùng subject; `durable` khi `Subscribe` có tham số `consumerName`; còn lại `unknown`.
3. Ghép stream: từ `EnsureStream("NAME", []string{"orca.x.>"})` và `SubjectBinding.StreamName`; subject cụ thể thuộc wildcard được thừa hưởng `stream`. **Không chuẩn hoá** `orca.infra.*` và `orca.infrafleet.*` (hai tiền tố cùng tồn tại).
4. `BuildTopics(uses []SubjectUse, serviceOfFile func(path string) string) []Topic`: gộp theo subject; `publishers`/`subscribers` là tập `ServiceRef` suy từ đường dẫn tệp (`services/<svc>/...`); `confidence` = mức thấp nhất trong các `SubjectUse` đóng góp; `payload` chỉ khi struct kề có tên kiểu (bỏ qua nếu không chắc). Với `api-gateway`: mọi vai trò là `inferred` (CR §2.4: chưa kiểm chứng từng điểm).
5. Giới hạn: tệp ≤ 1 MiB (vượt ⇒ bỏ + `warnings += "go_file_too_large:<path>"`); `cmd/server/main.go` có hạn mức riêng 2 MiB (CR nêu có thể rất lớn).

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/gosourcescan/ -run Subject` trên fixture `testdata/storage/services/**` (TASK-035-01): literal trong `const`, `SubjectBinding{StreamName:"ORCHESTRATION", Subject:"orca.orchestration.task.statuschanged"}`, `EnsureStream(ctx, "PROJECT", []string{"orca.project.>"})`, subject trong comment (bỏ), subject trong `slog.Info` (bỏ), `SubscribeEphemeral` ⇒ `ephemeral`, hai tiền tố `orca.infra.terminal_session.*` và `orca.infrafleet.terminal.closed` giữ nguyên.
- Golden Orca (thủ công/nightly, không chặn PR): đối chiếu tay 20 topic ngẫu nhiên theo tiêu chí nghiệm thu của CR §6; ghi tỷ lệ đúng vào mô tả PR.
- Chống sai: tệp Go lỗi cú pháp ⇒ `error` cho tệp đó, không làm hỏng các tệp khác (caller gom `warnings`).

## Tiêu chí hoàn thành

- [x] `orca.infrafleet.terminal.closed` có publisher `infra-fleet-service` (fixture), `orca.orchestration.task.statuschanged` có subscriber `task-service`.
- [x] Vai trò không chắc ⇒ `unknown`/`inferred`, không đoán.
- [x] Số lần parse mỗi tệp = 1; kết quả xác định.

## Rủi ro và lưu ý

- Subject ghép chuỗi (`"orca." + svc + ".x"`) không bắt được; chấp nhận, nêu ở §7 solution.
- Tỷ lệ sai publisher/subscriber chưa đo; không hứa độ chính xác ngoài `confidence`.
