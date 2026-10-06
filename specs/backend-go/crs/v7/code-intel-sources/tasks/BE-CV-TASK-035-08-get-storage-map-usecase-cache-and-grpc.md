# BE-CV-TASK-035-08: Use case `GetStorageMap`, cache snapshot, handler gRPC, kiểm thử vàng và canary

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_storage_map.go`, `get_storage_map_test.go`; `internal/usecase/ports.go` (sửa: dùng `RepoSourceReader`, `SnapshotStore`, `TargetResolver`, `FeatureGate`, `Authorizer` của các solution khác); `internal/adapter/grpc/storage_map_handler.go`, `storage_map_handler_test.go`; `testdata/storage/golden/storage_map.json` (mới); `cmd/server/main.go` (sửa: đăng ký handler — một dòng, do `BE-CV-SOL-010` mở sẵn chỗ)
**Depends on:** BE-CV-TASK-035-02, 035-03, 035-04, 035-05, 035-06, 035-07; BE-CV-SOL-012, 013, 022 (các port; nếu 022 chưa có thì chỉ cache bộ nhớ)
**Status:** [ ] TODO

---

## Context

Lắp ráp: kiểm cờ/quyền ⇒ đọc nguồn ⇒ parse độc lập ⇒ hợp nhất ⇒ quét cuối ⇒ cache ⇒ trả. Quyền **trước** cache (hợp đồng §3 bước 7). Thứ tự bước kiểm tra chung ở hợp đồng §3 đầu mục: guard nội bộ, tenant từ metadata, cờ, OPA `read`, phân giải `selector`.

## Việc cần làm

1. `get_storage_map.go`: `type GetStorageMap struct{…}` với `Execute(ctx, in) (StorageMapResult, error)`. Các bước theo solution 035 §2.F. Mọi `Read` đi qua `secretmasking.IsForbiddenSourcePath` trước (đã cấm ⇒ bỏ qua và ghi `warnings`, **không** lỗi). Đọc song song tối đa 4 nguồn; mỗi `Read` có timeout riêng ≤ 8 s; deadline toàn bộ 20 s (PQ-13); hết hạn ⇒ `CODEINTEL_TIMEOUT` kèm hậu tố `{"retryAfterMs":3000,"inProgress":true}` và hoàn tất nền bằng singleflight (100 s).
2. Cờ `include_legacy=false` ⇒ **không đọc** `deploy/old/*`.
3. Parse lỗi từng tệp ⇒ `warnings += "compose_parse_failed:<path>"`; vẫn trả các nguồn khác. Parse lỗi tất cả compose ⇒ vẫn trả (Store rỗng + warnings), không lỗi, vì thiếu dữ liệu không phải sai.
4. Hợp nhất: `MergeStores`, `FilterByEnv`, `canonical order`; sinh `meta` (`etag` = 32 hex của sha256 payload đã chuẩn hoá; `total_count` = tổng phần tử `stores+bindings+topics`; `head_commit` do cổng đọc trả; `truncated=false`; `generated_at`, `from_cache`, `not_modified` khi `if_none_match` khớp, PQ-12).
5. Quét cuối: `secretmasking.Scan(json bytes)`; có finding ⇒ `ErrSecretLeakBlocked`, **không ghi cache**, **không trả dữ liệu**.
6. Cache: `params_hash = sha256(env_filter|include_legacy|parserVersion|sorted(ContentID...))`; ghi `graph_snapshots(view="storage")` qua `SnapshotStore` với khoá `(tenant, binding, view, head_commit, params_hash)` (PQ-15); không có `ContentID` ⇒ bộ nhớ TTL 30 s. Đọc cache chỉ sau kiểm quyền.
7. `storage_map_handler.go`: ánh xạ domain ⇄ proto (`ServiceRef` cho `owner/publishers/subscribers`), lỗi domain ⇒ `apperrors` (`CODEINTEL_*` đầu `Message`, PQ-02), kiểm `proto.Size ≤ 2 MiB`. `ErrSecretLeakBlocked` ⇒ Kind `Internal`.
8. Sinh `testdata/storage/golden/storage_map.json` từ fixture; test snapshot so byte.
9. Thêm test "cổng đọc giả ghi mọi yêu cầu": khẳng định không có `.env*`, không có `deploy/old/*` khi `include_legacy=false`, chỉ có danh sách đường dẫn cố định + `internal/config/*.go` + `internal/adapter` + tệp chứa subject.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run StorageMap` (fake cổng đọc, fake snapshot, fake authorizer): đường chính; quyền sai ⇒ `CODEINTEL_NOT_AUTHORIZED` và **không** gọi cổng đọc/cache; cờ tắt ⇒ `CODEINTEL_DISABLED`; hai tenant cùng `head_commit`/`params_hash` ⇒ cache không đọc chéo (kiểm khoá chứa tenant); lỗi một compose; `if_none_match`; canary ở compose fixture ⇒ 0 lần xuất hiện trong JSON, trong log (logger buffer), trong khoá cache; chèn cố ý token vào Topic payload ⇒ `CODEINTEL_SECRET_LEAK_BLOCKED` và không có dòng cache.
- `go test ./services/code-intel-service/internal/adapter/grpc/ -run StorageMap`: ánh xạ proto, mã lỗi, `proto.Size`.
- Integration (`-tags=integration`) chạy ở ma trận `dialect: [postgres, mysql]` **chỉ khi** `SnapshotStore` thật đã có (solution 022); ghi rõ trong PR nếu bỏ qua.

## Tiêu chí hoàn thành

- [ ] Toàn bộ tiêu chí §9 của solution 035 đạt trên fixture.
- [ ] Không có đường nào đọc `.env`; test cổng giả xanh.
- [ ] Quyền kiểm trước cache; test cô lập tenant xanh.
- [ ] `buf breaking` xanh; kênh `codeIntel.storage` còn **chưa** đăng ký ở gateway (việc của `BE-CV-SOL-040-codeintel-view-channels`).

## Rủi ro và lưu ý

- `ContentID` chưa chắc có (TASK-035-01); đường cache bộ nhớ là phương án đã tính.
- Hạn mức đọc qua SSH 50–200 ms: số lệnh ≈ 6 (compose + init) + 2 mỗi service (config, liệt kê adapter) + quét subject; đo ở `BE-CV-SOL-071`, giữ song song có hạn mức (CR-013 hạn mức đồng thời).
- Nightly golden trên repo thật (không chặn PR) để bắt lệch khi compose đổi.
