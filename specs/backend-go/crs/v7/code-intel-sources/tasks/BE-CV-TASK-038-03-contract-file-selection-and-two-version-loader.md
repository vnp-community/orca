# BE-CV-TASK-038-03: Chọn tệp hợp đồng đã đổi và nạp hai phiên bản (base/head) qua `RepoSourceReader`

**From Solution:** BE-CV-SOL-038-contract-diff
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/contract_file_selection.go`, `contract_diff_ports.go`, `contract_version_loader.go` và `_test.go` (mới); `internal/domain/contractdiff/contract_path_patterns.go` (mới)
**Depends on:** BE-CV-TASK-038-01; BE-CV-SOL-030 (`ChangedFiles`, `ReadFile`, `FileDiff`)
**Status:** [ ] TODO

---

## Context

Solution 2.C. Chỉ tệp trong danh sách mẫu hợp đồng được đọc; mọi đọc qua cổng (S1). Port bộ phân tích do TASK-038-01 chốt chữ ký.

## Việc cần làm

1. `contract_path_patterns.go` (domain, thuần): `ClassifyContractPath(rel string) (kind string, ok bool)`: `backend-go/proto/orca/**/*.proto`⇒`proto`; `…/wscompat/{channels_*.go,registry*.go}`⇒`ws-channel`; `…/httpgateway/*_routes.go`⇒`route`; `backend-go/services/<svc>/migrations/{postgres,mysql}/*.sql`⇒`migration` (trả kèm `service`, `dialect`). Đường dẫn có `..`, `\`, tuyệt đối ⇒ `ok=false`. Dùng gói `path`.
2. `contract_diff_ports.go`: `ProtoSchemaParser`, `WsChannelExtractor`, `RouteCatalogExtractor` (có thể `Unavailable`), `MigrationCatalogBuilder` (nhận danh sách `(file, content)` theo thứ tự → `erd.Catalog`), `AccessorSource` (từ 031, có thể `Unavailable`).
3. `contract_file_selection.go`: `SelectContractFiles(changes []FileChange, kinds []string, max int) (selected []ContractFile, truncated bool)`: lọc mẫu + `kinds`; cho cả `oldPath` (đổi tên); sắp xác định; ≤ 300.
4. `contract_version_loader.go`: `LoadVersions(ctx, reader, repo, mergeBase, files) []FilePair{Path, OldPath, Base, Head, Status, Skipped}`: đọc song song có hạn mức (cổng); `not_found` base ⇒ `added`; `not_found` head ⇒ `removed`; `Skipped` (`binary|too_large|not_allowed|symlink_or_special`) ⇒ ghi lại để use case sinh `parse-error`/`skipped`; mỗi tệp ≤ 1 MiB; lỗi đọc một tệp không làm hỏng các tệp khác (trả lỗi theo tệp).
5. Không dùng/không dựng lệnh `git` ở đây; ghi comment: "mọi truy cập git nằm trong cổng 030".

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/contractdiff/ -run Path` và `./internal/usecase/ -run 'ContractFile|VersionLoader'`: các mẫu đường dẫn đúng/sai (kể cả `proto/gen`, `docs/*.proto`, `a/../b.proto`); đổi tên; tệp mới/xoá; `.env` không bao giờ được yêu cầu (cổng giả ghi lại yêu cầu); 301 tệp ⇒ `truncated`; lỗi đọc một tệp; hoán vị đầu vào ⇒ thứ tự ổn định.

## Tiêu chí hoàn thành

- [ ] Chỉ đường dẫn mẫu hợp đồng được đọc (test cổng giả).
- [ ] Đổi tên/mới/xoá đúng; giới hạn 300.

## Rủi ro và lưu ý

- `FileDiff` (nếu dùng) có thể trả diff thay vì hai nội dung; xác nhận ở TASK-038-01 và chọn `ReadFile` hai lần nếu cần.
