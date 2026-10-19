# TASK-REQ-039-01: Migration `0099_request_base_ref` và repository hai dialect

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `services/request-service/migrations/{postgres,mysql}/0099_request_base_ref.{up,down}.sql` (mới), `internal/adapter/{postgres,mysql}/request_repository.go`, `request_scan.go` (sửa), `internal/adapter/contracttest/expected_columns_base_ref.go` (mới), `internal/domain/request.go` (sửa nhỏ)
**Depends on:** Không
**Status:** [x] DONE (2026-10-19)

---

## Context

- Migration mới nhất là `0098_execution_mode` ở cả hai dialect (kiểm lại số khi bắt đầu; nếu có CR khác giữ `0099` thì lấy số kế tiếp ở cả hai dialect và ghi vào PR).
- Mẫu: `0098_execution_mode.up.sql` thêm cột có CHECK; repository đọc cột trong `request_scan.go`; `expected_columns_execution_mode.go` khai báo cột cho test hợp đồng.

## Việc cần làm

1. `up` Postgres: `ALTER TABLE request.requests ADD COLUMN base_ref TEXT NOT NULL DEFAULT '' CHECK (char_length(base_ref) <= 255)`. MySQL: `ADD COLUMN base_ref VARCHAR(255) NOT NULL DEFAULT ''` (kiểm chế độ strict của MySQL để độ dài vượt bị từ chối, không cắt).
2. `down` cho cả hai dialect (`DROP COLUMN`).
3. `domain.Request.BaseRef string`; `request_scan.go` thêm cột vào danh sách và `Scan` (Postgres và MySQL).
4. `RequestRepository.SetBaseRef(ctx, id string, baseRef string, expectedVersion int64) (domain.Request, error)`: `UPDATE ... SET base_ref=$3, version=version+1, updated_at=now() WHERE id=$1 AND tenant_id=$2 AND version=$4`; 0 dòng bị ảnh hưởng thì phân biệt `NotFound` và `ErrVersionConflict`. Hàm nhận transaction của người gọi để task 02 khoá hàng và ghi trong cùng transaction.
5. `RequestRepository.GetForUpdate(ctx, id)` nếu chưa có (khoá hàng trong transaction) cho cả hai dialect.
6. Không backfill: Request cũ có `''`.

## Kiểm thử

- Hợp đồng (`request_repository_contract` hiện có): thêm ca `base_ref` mặc định rỗng, ghi và đọc lại, `SetBaseRef` đúng phiên bản và sai phiên bản, `SetBaseRef` khi `id` không tồn tại, CHECK độ dài (256 ký tự bị từ chối).
- Chạy migration xuôi rồi ngược rồi xuôi trên Postgres 16 và MySQL 8; Request tạo trước migration đọc được.
- `cd backend-go && go test ./services/request-service/internal/adapter/... -run 'Contract' -v`.

## Tiêu chí hoàn thành

- [x] `0099` chạy xuôi và ngược trên cả hai dialect.
- [x] `SetBaseRef` có khoá lạc quan và test chứng minh.
- [x] Request cũ đọc được, `BaseRef==""`.
- [x] Test hợp đồng hai dialect xanh.

## Rủi ro và lưu ý

- Số migration có thể bị CR khác giữ; kiểm trước khi viết.
- MySQL non-strict mode có thể cắt âm thầm; test CHECK/độ dài phải chạy ở chế độ đúng của triển khai.

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

### Tệp đã đổi

- Migration mới (số `0099` còn trống ở cả hai dialect, kiểm bằng `ls`): `migrations/postgres/0099_request_base_ref.{up,down}.sql` (`TEXT NOT NULL DEFAULT ''` + `CONSTRAINT requests_base_ref_length CHECK (char_length(base_ref) <= 255)`), `migrations/mysql/0099_request_base_ref.{up,down}.sql` (`VARCHAR(255) NOT NULL DEFAULT ''`).
- `internal/domain/request.go`: thêm `BaseRef string` (lưu) và `BaseRefLocked`, `BaseRefLockReason` (suy ra khi đọc, không lưu).
- `internal/adapter/{postgres,mysql}/request_scan.go` (cột `base_ref` vào `requestColumns` và `Scan`), `request_repository.go` (`INSERT` ghi `base_ref`; `Create` từ chối ref quá 255 byte trước khi chạm DB), `request_base_ref.go` (mới, `SetBaseRef` có khoá lạc quan và phân biệt `NotFound`/`ErrVersionConflict`).
- `internal/usecase/request_base_ref_writer.go` (cổng hẹp `RequestBaseRefWriter`).
- `internal/domain/erasable_columns.go`: `requests.base_ref` vào danh sách miễn (tên ref, không phải nội dung Request); `TestEveryTextColumnDeclaredErasableOrExempt` vẫn xanh.
- Hợp đồng: `contracttest/expected_columns_base_ref.go` (mới) + `expected_columns.go`; `contracttest/request_base_ref_contract.go` (6 kịch bản mới chạy trong `RunRequestRepositoryContract`); `not_found_contract.go` thêm thăm dò `Requests.SetBaseRef`.
- Kiểm thử tích hợp mới: `postgres/request_base_ref_integration_test.go`, `mysql/request_base_ref_integration_test.go`.

### Lệnh đã chạy và kết quả

Docker qua colima: `export DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`.

- `cd backend-go/services/request-service && go test -tags=integration ./internal/adapter/postgres/ -run 'RequestRepositoryContract|Migration_BaseRef|Migration_UpDownUp|SchemaContract|NotFound' -count=1 -v`: PASS (Postgres 16-alpine). Gồm `TestPostgres_Migration_UpDownUp` (xuôi, ngược, xuôi toàn chuỗi, có 0099), `TestPostgres_Migration_BaseRef` (Request tạo trước 0099 đọc ra `""`; 255 ký tự nhận, 256 bị CHECK từ chối, NULL bị từ chối; chỉ `down` của 0099 rồi `up` lại, dữ liệu còn nguyên), `TestPostgres_SchemaContract_InformationSchema`, và 6 kịch bản `BaseRef*`/`SetBaseRef*`/`UpdateKeepsBaseRef` trong `RequestRepositoryContract`.
- Cùng bộ lệnh cho `./internal/adapter/mysql/` (MySQL 8.0): PASS, thêm `TestMySQL_BaseRef_NonStrictServerDoesNotTruncate` (phiên `sql_mode=''` chứng minh server cắt âm thầm còn repository vẫn từ chối).
- Chạy toàn bộ gói với `-tags=integration` (sau khi cả task 02 đã vào): Postgres 402 PASS, MySQL 362 PASS (đếm dòng `--- PASS` gồm test con). Hai lỗi không thuộc task này: `ApproverCanChoose` ở `TestPostgres_SolutionContract` và `TestMySQL_SolutionContract` (`REQUEST_SOLUTION_FORBIDDEN` kỳ vọng nhưng `nil`; đến từ việc agent khác đang sửa `solution_authorization.go` trong cùng cây làm việc), và `TestTwoRelaysConcurrentlyNoLoss` (MySQL) lỗi một lần vì NATS container không kết nối được, chạy riêng 3 lần đều PASS.
- Kiểm tra đột biến: bỏ khoá lạc quan khỏi `SetBaseRef` thì `SetBaseRefConcurrentSameVersion` đỏ (đã hoàn nguyên).
- `go build ./...`, `go vet ./...`, `go test -race ./...` của `services/request-service`: xanh.
- Lệnh gốc trong task (`go test ./services/request-service/internal/adapter/... -run 'Contract' -v`) cần thêm `-tags=integration` mới biên dịch các test hợp đồng (build tag có sẵn của repo); không có tag thì không chạy gì.

### Blast radius (không có GitNexus; tìm bằng grep)

- `requestColumns` / `scanRequest`: dùng chung ở `getBy`, `Update`, `List`, `backlog_requests.go`, `request_content_repository.go`, `approval_request_lock.go` (cả hai dialect). Cột mới đi cùng một danh sách và một `Scan` nên không có chỗ lệch; các test hợp đồng đã chạy đủ các đường đó.
- `RequestRepository.Create` (cả hai dialect): gọi từ `CreateRequest.CreateWithinTx`, `child_request_creator.go` (tạo con, `BaseRef` rỗng), và các hàm dựng của hợp đồng. `Update` không ghi `base_ref` (có test `UpdateKeepsBaseRef`), nên các ghi lifecycle không thể đè giá trị cũ.
- `domain.Request` thêm trường: mọi literal trong repo đều có khoá tên, `reflect.DeepEqual` của `createGetRoundTrip` vẫn xanh.

### Lệch so với văn bản task

1. `SetBaseRef` nằm ở cổng hẹp `usecase.RequestBaseRefWriter` (repository thoả mãn), không thêm vào `usecase.RequestRepository`: đúng mẫu `ExecutionModePinner` của CR-REQ-038 và tránh sửa mọi fake của repository.
2. Không thêm `GetForUpdate`: `RequestLocker.LockRequest` (`approval_request_lock.go`, cả hai dialect) đã làm đúng việc đó (`SELECT ... FOR UPDATE` trong transaction, trả `domain.Request`).
3. `SetBaseRef` luôn kiểm phiên bản chính xác (không có "0 = bỏ qua"); quy ước 0 nằm ở use case (task 02).
4. Thêm kiểm tra độ dài ở repository (`checkStorableBaseRef`) ngoài CHECK/`VARCHAR`, vì MySQL non-strict cắt âm thầm (đã chứng minh bằng test).
5. MySQL không thêm CHECK độ dài: cột `VARCHAR(255)` + strict mode mặc định + chặn ở repository là đủ, và CHECK không chặn được việc cắt ở non-strict.

### Chưa kiểm chứng

- MySQL < 8.0.16 và các phiên bản Postgres khác 16 chưa chạy (chỉ `postgres:16-alpine`, `mysql:8.0`).
- `golangci-lint` không chạy được trong môi trường này (bản cài biên dịch bằng Go cũ hơn module, lỗi typecheck từ thư viện); chỉ có `gofmt` và `go vet`.
