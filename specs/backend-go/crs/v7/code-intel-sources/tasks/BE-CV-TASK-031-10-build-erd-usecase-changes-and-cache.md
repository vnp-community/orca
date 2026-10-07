# BE-CV-TASK-031-10: Use case `BuildErd`: khám phá service, ngân sách, `changes[]` base/head, cache snapshot

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/build_erd.go` (mới), `.../usecase/erd_service_discovery.go` (mới), `.../domain/erd/catalog_diff.go` (mới), các `_test.go`
**Depends on:** BE-CV-TASK-031-08, BE-CV-TASK-031-09; BE-CV-SOL-022 (`SnapshotStore` port); BE-CV-SOL-012 (phân giải selector)
**Status:** [x] DONE

---

## Context

SOL-031-erd mục 2.E, 2.F. Kết hợp mọi phần thành `BuildErd(ctx, BuildErdInput) (BuildErdOutput, error)`. Cache qua cổng snapshot của SOL-022 (tên port do SOL-022 đặt: **đọc `internal/usecase` trước khi viết**; nếu chưa có thì dùng port tạm `ErdSnapshotCache` và nối sau).

## Việc cần làm

1. `erd_service_discovery.go`: từ glob `CODEINTEL_ERD_MIGRATION_GLOB` (mặc định `backend-go/services/*/migrations/{postgres,mysql}`) và `ListDir` → danh sách `{service, dialects[]}`; mở rộng glob chỉ gồm `{postgres,mysql}`; bỏ qua thư mục khác; sắp theo tên.
2. `build_erd.go`: kiểm `tenant.RequireTenantID`; `service` rỗng → trả `services[]` (số bảng lấy từ cache nếu có, nếu không chỉ `name`+`dialects` và `table_count=0`; ghi chú); có `service` → đọc migrations (ReadFiles theo thư mục dialect) → `BuildCatalog` → quan hệ (task 08) → `accessedBy` khi `include_access` (task 09) → `changes[]` nếu `base_ref` → ngân sách → `ErdModel`.
3. `catalog_diff.go` (miền thuần): so hai Catalog → `[]ErdChange{table, column, kind, before, after}` (thêm/bớt bảng, cột; đổi type/nullable/default); `migration_file` + `line` lấy từ `Table.LastMigration`/dòng câu lệnh cuối chạm cột (cần Catalog ghi `LastTouched{file, line}` cho cột — thêm vào task 03 nếu thiếu); dùng chung với `BE-CV-SOL-038-contract-diff`.
4. Catalog base: `ChangedFiles(baseRef)`; file migration `added` chỉ áp ở head; `modified`/`deleted` đọc ở `MergeBase` bằng `ReadFile(commit)`; trường hợp không chắc (đổi tên, `git.branchCompare` báo lỗi) → đọc đủ ở base. Ghi `ParseWarning{BASE_PARTIAL}` khi phải suy giảm.
5. Cache: khoá `(tenant, binding, view="erd", head_commit, params_hash)` với `params_hash` = băm `(service, dialect, base_ref, head_ref, include_access, include_inferred, tập (path,ContentHash) bẩn, schema_version)`; `etag` 32 hex từ băm payload; `if_none_match` khớp → `NotModified`; `truncated` không bị cache như đầy đủ (lưu cờ).
6. Singleflight 100 s theo khoá cache khi vượt 20 s (PQ-13): trả `CODEINTEL_TIMEOUT` hậu tố `{"retryAfterMs":3000,"inProgress":true}` ở handler (task 11); use case chỉ trả `ErrInProgress`.
7. Che: bộ che chung trước khi trả `default_expr`/`CHECK`/`comment`.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/... -run Erd` (chưa chạy) với `RepoSourceReader` giả + cache giả.
- Ca: service không tồn tại → `CODEINTEL_INVALID_PARAMS`; không migrations → model rỗng + cảnh báo; `base_ref` thêm một migration thêm cột → `changes` đúng một mục `added`; sửa migration cũ → `BASE_PARTIAL`/đọc đủ; vượt 300 bảng → `truncated`.
- Cache hit/miss; `if_none_match`; file bẩn đổi `params_hash`.
- **Cô lập tenant**: hai tenant, cùng repo và `head_commit`: khoá cache khác; use case từ chối khi `RepoRef.TenantID` ≠ ctx.
- Hai dialect: chạy cùng suite với service có hai thư mục (`infra-fleet-service`) cho `dialect=postgres|mysql`.

## Tiêu chí hoàn thành

- [x] Mọi tiêu chí SOL-031-erd mục 6 liên quan đến use case (đủ bảng, `changes`, `services[]`, `not_modified`, `truncated`).
- [x] Không gọi DB hay đọc file ngoài cổng; không log nội dung.
- [x] Không tên `helpers/utils/common/misc`.

## Rủi ro và lưu ý

- Tối ưu "chỉ đọc file thêm" ở base chưa kiểm chứng; luôn có đường lui đúng-nhưng-chậm.
- `table_count` khi chưa có cache cần dựng parser nhanh; nếu quá chậm chỉ trả `0` và ghi rõ (không đoán).
