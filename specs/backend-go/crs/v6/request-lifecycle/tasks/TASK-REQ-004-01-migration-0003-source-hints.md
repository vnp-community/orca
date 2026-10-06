# TASK-REQ-004-01: Migration `0003_request_source_hints` và trường `SourceHints` trong domain, repository

**From Solution:** BE-REQ-SOL-004
**Priority:** P0
**Service:** `request-service`
**File:** `migrations/postgres/0003_request_source_hints.up.sql`, `.down.sql`; `migrations/mysql/0003_request_source_hints.up.sql`, `.down.sql` (mới); `internal/domain/request.go`, `internal/domain/request_source_hints.go` (mới); `internal/adapter/postgres/{request_repository.go,request_scan.go}`, `internal/adapter/mysql/{request_repository.go,request_scan.go}` (sửa)
**Depends on:** TASK-REQ-002-04, TASK-REQ-002-05
**Status:** [ ] TODO

---

## Context

CR-REQ-004 mục 2.4 thêm cột `requests.source_hints` (JSONB/JSON, NULL được) bằng migration `0003_request_source_hints` để CR-REQ-005 dùng làm gợi ý. **Xác định số migration thật:** chạy `ls services/request-service/migrations/postgres`; nếu đã có `0003_*` từ PR khác (ví dụ CR-REQ-007, 009 chạy sớm) thì lấy số kế tiếp và sửa tên file. Mẫu thêm cột hai dialect: `task-service/migrations/{postgres,mysql}/0014_task_sources_site.up.sql`.

## Việc cần làm

1. Postgres up: `ALTER TABLE request.requests ADD COLUMN source_hints JSONB;` down: `ALTER TABLE request.requests DROP COLUMN source_hints;`.
2. MySQL up: `ALTER TABLE requests ADD COLUMN source_hints JSON NULL;` down: `ALTER TABLE requests DROP COLUMN source_hints;`.
3. `domain/request_source_hints.go`: `type SourceHints struct { IssueType string; Labels []string; Priority string; TypeHint string }` và `func (h SourceHints) IsZero() bool`; `Request.SourceHints SourceHints`.
4. Repository: `Create` và `Update` ghi cột (`nil` JSON khi `IsZero()`, giữ NULL), `request_scan.go` đọc (NULL thành giá trị zero). Postgres `$n::jsonb`, MySQL `[]byte`. Tên khoá JSON: `issue_type`, `labels`, `priority`, `type_hint` (snake_case, ghi trong comment một dòng vì là hợp đồng lưu trữ).
5. Cập nhật `ExpectedColumns()` của `contracttest` (TASK-REQ-002-06) thêm `source_hints` nullable.

## Kiểm thử

- `TestMigration_0003_UpDownUp` hai dialect; down không mất `0002`.
- `TestSourceHints_RoundTrip` (đặt rồi đọc lại; zero ghi NULL; đọc NULL ra zero) hai dialect, thêm vào bộ contract.
- `TestSchemaContract` cập nhật xanh.
- Lệnh: `go test -tags=integration ./services/request-service/internal/adapter/... -run "Migration|SourceHints|Schema" -v`.

## Tiêu chí hoàn thành

- [ ] up/down/up sạch trên Postgres và MySQL.
- [ ] `SourceHints` ghi, đọc đúng; NULL tương ứng zero.
- [ ] Test schema hợp đồng cập nhật và xanh.
- [ ] Số migration đã xác nhận bằng `ls` và ghi trong PR.

## Rủi ro và lưu ý

- Cột này chưa có trong README v6 mục 3.5 (Q1 của SOL-004); báo người duyệt cập nhật README.
- Nếu SOL-005 thêm migration `classification_attempts` trước khi PR này merge, số `0003` có thể bị chiếm; đổi tên theo `ls`.
