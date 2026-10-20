# TASK-REQ-046-04: Cắt chuỗi theo byte không làm hỏng UTF-8 (`NewEvidence`) và kiểm giới hạn với văn bản tiếng Việt

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/evidence.go` (sửa), `internal/domain/evidence_truncation_test.go` (mới), `internal/domain/vietnamese_limits_test.go` (mới), `internal/adapter/contracttest/solution_contract.go` (sửa: thêm ca), test tích hợp `internal/adapter/{postgres,mysql}/solution_integration_test.go` (chạy lại)
**Depends on:** không (độc lập, làm bất kỳ lúc nào)
**Status:** [ ] TODO

---

## Context

- `domain.NewEvidence` (`internal/domain/evidence.go:44`): `if len(excerpt) > 4096 { excerpt = excerpt[:4096] }` (`:45-46`) cắt theo byte và có thể để lại nửa ký tự; tiếng Việt (2 đến 3 byte mỗi chữ có dấu) làm lỗi lộ ra trước. Ngưỡng 4096 là theo **byte** vì có `CHECK (octet_length(excerpt) <= 4096)` ở `migrations/postgres/0005_context_sources.up.sql:64`, nên phải giữ ngưỡng byte nhưng cắt ở ranh giới rune.
- Hiện `NewEvidence` không có nơi gọi và `EvidenceRepository` (`usecase/ports.go:86`) không có bản cài đặt (đã `grep` toàn `backend-go`): lỗi là thật nhưng chưa chạm tới ở runtime. Vẫn sửa, vì đường dẫn sẽ được nối.
- Các chỗ cắt byte khác đã rà và để nguyên vì đã hợp lệ UTF-8: `domain/request_activity.go:306` (`truncateActivityText`), `usecase/get_execution_summary.go:279` (`truncateSummaryText`), `usecase/report_task_outcome.go:138`, `domain/analysis_run.go:120` (`TruncateRaw`), `domain/analysis_document.go:25`. Chúng cho ít ký tự hơn với tiếng Việt (400 byte ≈ 150 đến 200 ký tự) nhưng không sinh UTF-8 hỏng.
- Giới hạn của tài liệu AI đếm rune (`solution_options.go:150,170-176`, `analysis_evidence.go:10,29`, `answer_document.go:5,42`, `plan_validation.go:79,112,149,159,169`): cần test chứng minh với chữ có dấu, kể cả chữ dạng NFD.

## Việc cần làm

1. `NewEvidence`: nếu `len(excerpt) > 4096` thì lùi `cut` từ 4096 về ranh giới rune (`utf8.RuneStart(excerpt[cut])`) và cắt `excerpt[:cut]`. Giữ chú thích một dòng nêu lý do (CHECK theo byte, cắt ở ranh giới rune).
2. `evidence_truncation_test.go`: ASCII đúng 4096 (không đổi) và 4097 (cắt còn 4096); chuỗi tiếng Việt có ranh giới rơi giữa ký tự 2 byte và giữa ký tự 3 byte (dựng bằng cách lặp `ệ`, `ợ`, `ă` và đệm ASCII để kiểm cả ba độ lệch); kết quả luôn `utf8.ValidString`, `len <= 4096`, và là tiền tố của đầu vào; chuỗi ngắn không đổi; chuỗi rỗng.
3. `vietnamese_limits_test.go` (bảng): `SolutionOptions.Validate` với `approach` đúng 4000 rune có dấu (qua) và 4001 (từ chối, đúng thông báo), `title` 120, `summary` 600; `validateEvidence` với `excerpt` 600 rune có dấu (qua) và 601; `answer_markdown` 8000 rune (qua) và 8001; `ValidatePlanProposal` với tiêu đề task 200 rune có dấu; mỗi ca có biến thể NFC và NFD của cùng một văn bản và ghi nhận (assert) rune đếm được khác nhau, để R5 của CR có bằng chứng.
4. Hợp đồng lưu trữ: thêm vào danh sách ca của `RunSolutionContract` (`contracttest/solution_contract.go:63`) ca `LargeVietnameseDocumentSurvivesTheDatabase`: tài liệu bốn option mỗi `approach` 4000 rune có dấu, một `RawOutput` lớn (vài trăm KB) trong run; ghi, đọc lại, so khớp byte và `Validate`. Đã có `UnicodeAndDigestSurviveTheDatabase` (`:867`) cho tài liệu nhỏ; ca mới chứng minh không chạm trần cột (MySQL cột `JSON`, `MEDIUMTEXT`) trên hai dialect.
5. Không thêm migration; không đổi cột.

## Kiểm thử

- `go test ./internal/domain/ -run 'Evidence|Vietnamese' -race -count=1`.
- Hai dialect (máy dev này, Colima): `export DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock; cd backend-go/services/request-service && go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'Solution' -count=1 -p 1`: cả hai xanh, gồm ca mới.
- Cố ý trả lại `excerpt[:4096]` và thấy `evidence_truncation_test.go` đỏ (kiểm đột biến), rồi hoàn lại.

## Tiêu chí hoàn thành

- [ ] `NewEvidence` không bao giờ trả UTF-8 không hợp lệ hoặc quá 4096 byte; test bắt được lỗi cũ.
- [ ] Giới hạn rune chạy đúng với tiếng Việt có dấu ở ngưỡng và ngay trên ngưỡng.
- [ ] Ca hợp đồng tài liệu lớn xanh trên Postgres 16 và MySQL 8.

## Rủi ro và lưu ý

- Văn bản NFD đếm nhiều rune hơn NFC; CR không chuẩn hoá (R5). Test chỉ ghi nhận sự khác biệt, không đổi hành vi.
- Không đổi các hàm cắt đã hợp lệ ở danh sách trên (ngoài phạm vi, sẽ làm lệch giới hạn hiện có).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
