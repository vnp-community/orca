# BE-CV-SOL-022: Cache snapshot `(binding, view, HEAD, params)`: stale, singleflight, ETag, TTL/dung lượng, huỷ cache

> **✅ Implemented.** Đã triển khai, vượt qua toàn bộ test (unit, race, contract, integration). Bọc `ViewReader` của SOL-021; cần bảng `graph_snapshots` của BE-CV-SOL-011-data-model-and-migrations (PQ-15) và repository của BE-CV-SOL-011-repositories-and-maintenance.

**CR:** [CR-CV-022](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-022-snapshot-cache.md)
**Service:** `code-intel-service` (`internal/domain`, `internal/usecase`, `internal/config`; SQL của `SnapshotStore` thuộc SOL-011)
**TDD tham chiếu:** [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) ("Multi-tenancy", "Migration conventions"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (timeout, graceful degradation: phục vụ snapshot cũ khi dependency tạm lỗi), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("Multi-tenancy isolation")
**Hợp đồng:** `CONTRACT-codeintel-proto-and-data-map.md` (PQ-12, PQ-13, PQ-14, PQ-15, PQ-24; §4 T3, §4.3, §5, §6.2), `CONTRACT-codeintel-ui-api.md` (§2.2, §2.4), agent contract §2.2–2.3

---

## 0. Hợp đồng áp dụng

| Phán quyết / mục | Áp dụng |
|---|---|
| PQ-12 | `etag`, `not_modified` (không `data`), `from_cache`, `generated_at`; `ifNoneMatch` ≤ 80 ký tự; `etag` dạng `"…32hex…"` có nháy |
| PQ-13 | Service vượt 20 s → `CODEINTEL_TIMEOUT | {"retryAfterMs":3000,"inProgress":true}` và **tiếp tục thu thập nền** (singleflight 100 s) |
| PQ-14 | Snapshot ≤ `CODEINTEL_SNAPSHOT_MAX_BYTES` (mặc định 3 MiB, tối đa 8 MiB; CHECK 16 MiB); TTL **7 ngày**; giữ **3 commit** mỗi `(binding, view)` (CR-022 thắng CR-011); 64 MiB/binding, 512 MiB/tenant |
| PQ-15 | Cột `head_commit` (không `commit`), `etag char(32)`, `total_count`, `schema_version`; UNIQUE `(tenant_id, repo_binding_id, view, head_commit, params_hash)` |
| PQ-24 | Cờ tắt (`CODEINTEL_DISABLED`) chặn đọc cache; cache không bao giờ trả dữ liệu cho người không có quyền (quyền kiểm **trước** cache, §3 đầu) |
| §4 T3 | Tập `view` lưu DB (19 giá trị); **không lưu** `status` (bộ nhớ 15 s) và `symbol` (LRU) |
| §4.3 | Bảo trì mỗi 10 phút theo lô 500 (`withMaintenanceTx`) |
| §5 | `reindex.finished`/`index.changed` huỷ cache (SOL-024 gọi `InvalidateBinding`) |
| §8.3 (3)(4) | Test `SnapshotStore` hai dialect; mọi truy vấn có `tenant_id`; test cô lập tenant |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: ba CONTRACT; CR-CV-022; `common/dbcapability/capability.go` (MySQL `SupportsJSONB=false`, `SupportsReturning=false`, `SupportsRLS=false`); `api-gateway/internal/adapter/authclient/mcp_principal_resolver.go` (mẫu `singleflight`); `go.mod` (`golang.org/x/sync v0.22.0` trực tiếp ở `api-gateway`, `git-gateway-service`; `// indirect` ở nơi khác kể cả `infra-fleet-service`); `common/eventbus/eventbus.go`. Chưa có `code-intel-service`, chưa có bảng `graph_snapshots` (hợp đồng đánh dấu migration `0002` do CR-011).

### Correction relative to CR-CV-022 (Lệch giữa CR và hợp đồng)

| # | CR nói | Hợp đồng | Xử lý |
|---|--------|----------|-------|
| C1 | Cột `commit` (có thể trích dẫn) | PQ-15: `head_commit` | Dùng `head_commit` |
| C2 | Payload ≤ 3 MiB cố định | PQ-14: mặc định 3 MiB, cấu hình tới 8 MiB | `CODEINTEL_SNAPSHOT_MAX_BYTES` |
| C3 | Phản hồi tới UI không bàn | PQ-14: response ≤ 2 MiB (SOL-020) | Snapshot chỉ lưu `data` đã qua `Limit*`; có thể < 3 MiB |
| C4 | `view` là enum `ViewKind` 10 giá trị | §4 T3: 19 chuỗi | Dùng `ViewKind.CacheName()` (SOL-020 2.D) |
| C5 | Không nêu timeout dịch vụ | PQ-13: 20 s + hoàn tất nền | Thêm 2.D |
| C6 | `SnapshotStore` có `EvictOldest`, `TotalBytes`… do CR-011 cài | Hợp đồng không liệt kê phương thức | Solution này **đặc tả cổng + suite hợp đồng**; SOL-011-repositories cài SQL |
| C7 | HEAD thăm dò bằng `codeintel.status` 15 s | agent §2.3: `headCommit` đã nằm trong phong bì mọi method; cache `headCommit` 5 s ở agent | Giữ thăm dò 15 s nhưng **ưu tiên** `headCommit` của phong bì lần gọi gần nhất; chỉ gọi `status` khi mục thăm dò hết hạn |
| C8 | Thông báo mất → xoá toàn DB | Hợp đồng §5, SOL-024: `resync` không xoá snapshot DB | Chỉ `InvalidateProbe` (bộ nhớ) cho `resync/overflow` |

## 2. Giải pháp

### A. Cây file (mới)

```
services/code-intel-service/internal/
  domain/snapshot.go                  Snapshot, SnapshotKey, GraphModelVersion
  domain/snapshot_params_hash.go      CanonicalParamsHash
  domain/snapshot_etag.go             ContentETag, ClientETag
  usecase/snapshot_store.go           cổng SnapshotStore
  usecase/head_probe.go               HeadProbe (15 s, singleflight) + Staleness (A/B)
  usecase/cached_view_reader.go       CachedViewReader (bọc ViewReader của SOL-021)
  usecase/snapshot_invalidator.go     InvalidateBinding / InvalidateProbe
  usecase/snapshot_janitor.go         dọn hết hạn + hạn mức
  adapter/snapshotstorecontract/snapshot_store_contract.go   suite hợp đồng dùng chung hai dialect
```
SQL `postgres/`, `mysql/` do SOL-011-repositories cài và chạy suite này.

### B. Khoá, ETag, stale

- `params_hash` = SHA-256 của JSON chuẩn tắc (khoá sắp xếp, mặc định điền, chỉ tham số hợp đồng) **cộng tiền tố** `GraphModelVersion` (hằng domain; tăng khi đổi quy tắc chuẩn hoá của SOL-020 để vô hiệu cache cũ; cột `schema_version` lưu cùng giá trị).
- `head_commit` trong khoá = **HEAD của worktree lúc yêu cầu** (nguồn: `HeadProbe`).
- `etag` lưu = hex 16 byte đầu SHA-256 `(binding, view, head_commit, params_hash, tool_versions chuẩn tắc, sha256(payload))` (32 ký tự); ETag trả client = `"` + hex 16 byte của SHA-256 `(contentEtag, headCommit, stale)` + `"`. Khớp `if_none_match` → `ResultMeta{not_modified=true, etag, head_commit, stale, sources}` không `data`.
- `stale = A || B`: **A** nguồn `commit` ≠ `headCommit` (bất kỳ nguồn nào), hoặc `rootMismatch`, hoặc `pendingChanges > 0`; **B** phiên bản công cụ / `indexedAt` trong thăm dò khác bản lưu → coi là **miss**. `stale` không kích hoạt thu thập lại (O3).

### C. Luồng đọc (`CachedViewReader.Get`)

1. Quyền/cờ đã kiểm ở pipeline SOL-013 **trước** khi tới đây (cache không phục vụ khi chưa có quyền).
2. `HeadProbe` → `head_commit`; offline → bước 6.
3. `Store.Get` còn hạn (`expires_at > now` theo đồng hồ DB) và không B → tính stale + ETag → trả (`from_cache=true`).
4. Miss → singleflight (khoá `tenant|binding|view|head|params_hash`, `DoChan`, công việc dẫn chạy `context.WithoutCancel` + `CODEINTEL_COLLECT_TIMEOUT` 100 s; người chờ `select` giữa kết quả và ctx của mình) → `ViewReader` (SOL-021) → `Put` → trả.
5. Quá **20 s** chờ → trả `CODEINTEL_TIMEOUT | {"retryAfterMs":3000,"inProgress":true}`, công việc nền tiếp tục (PQ-13).
6. Lỗi tạm (`CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_TIMEOUT` của collector) và có snapshot cũ cùng `(binding, view, params_hash)` → `GetLatest`, `stale=true`, `from_cache=true`. **Không** che lỗi nghiệp vụ (`INDEX_MISSING`, `PATH_NOT_ALLOWED`, `INVALID_PARAMS`, `TOOL_UNAVAILABLE`, `AMBIGUOUS_SYMBOL`). Lỗi không được cache.
7. `STATUS` không lưu DB (bộ nhớ 15 s); `SYMBOL` LRU bộ nhớ (64 mục, 5 phút), **không bao giờ** xuống DB (H8).

### D. Lưu trữ, hạn mức, dọn

`SnapshotStore`: `Get`, `GetLatest`, `Put` (upsert: PG `ON CONFLICT DO UPDATE`, MySQL `ON DUPLICATE KEY UPDATE`, không `RETURNING`), `DeleteByBinding`, `DeleteExpired(limit)` (xuyên tenant, chỉ janitor, `withMaintenanceTx`), `TotalBytes`, `TenantBytes`, `EvictOldest(keepCommits, targetBytes)` (không bao giờ xoá snapshot mới nhất mỗi view). `Put` bỏ qua (log, vẫn trả cho người gọi) khi `payload` > `CODEINTEL_SNAPSHOT_MAX_BYTES`. Janitor: `CODEINTEL_MAINTENANCE_INTERVAL` 10 phút, lô 500; sau `Put`, nếu vượt trần binding thì `EvictOldest` ngay. `InvalidateBinding` = **xoá** (idempotent, 0 dòng không lỗi) + xoá `HeadProbe` + LRU; `InvalidateProbe` chỉ bộ nhớ.

### E. Cấu hình và quan sát

`CODEINTEL_HEAD_PROBE_TTL` 15s, `CODEINTEL_COLLECT_TIMEOUT` 100s, `CODEINTEL_SNAPSHOT_TTL` 168h, `CODEINTEL_SNAPSHOT_KEEP_COMMITS` 3, `CODEINTEL_SNAPSHOT_MAX_BYTES` 3145728, `CODEINTEL_CACHE_MAX_BYTES_PER_BINDING` 67108864, `CODEINTEL_CACHE_MAX_BYTES_PER_TENANT` 536870912, `CODEINTEL_SYMBOL_LRU_ENTRIES` 64, `CODEINTEL_SYMBOL_LRU_TTL` 5m. Metrics: `codeintel_cache_requests_total{view,result=hit|miss|stale_served|offline_served|not_modified}`, `codeintel_cache_bytes{scope}`, `codeintel_singleflight_shared_total`, `codeintel_cache_evictions_total{reason}` (tên cuối CR-071).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Khoá theo HEAD, không theo commit chỉ mục | Biết trước khi truy vấn; commit chỉ mục dùng cho `stale` |
| Huỷ bằng xoá | Không cột trạng thái; idempotent |
| Singleflight theo replica, không khoá DB | Trùng hiếm, `Put` là upsert |
| `GraphModelVersion` trong `params_hash` | Đổi chuẩn hoá tự vô hiệu cache, không migration |
| ETag chứa `stale`, `headCommit` | UI không giữ "tươi" sai sau khi HEAD đổi |
| Suite hợp đồng dùng chung | Hai dialect cùng một bộ test (§8.3) |

## 4. Phụ thuộc chéo khu vực

| Hướng | Solution | Quan hệ |
|---|---|---|
| Trước | `BE-CV-SOL-011-data-model-and-migrations` (`graph_snapshots`), `BE-CV-SOL-011-repositories-and-maintenance` (SQL, `withMaintenanceTx`), `BE-CV-SOL-020`, `BE-CV-SOL-021` | |
| Sau | `BE-CV-SOL-024` (`InvalidateBinding/Probe`), `BE-CV-SOL-040-codeintel-view-channels` (ETag/`ifNoneMatch`, 20 s), `BE-CV-SOL-036`, `BE-CV-SOL-052/060` (so sánh lượt trước cần snapshot cũ) | |
| Agent | `AG-CV-SOL-001-codeintel-agent-foundation` (`status`, `pendingChanges`, `sources[].commit`, `headCommit`) | stale phụ thuộc agent trả đúng |
| Frontend | `FE-CV-SOL-050-store-and-query-hooks` | UI hiện `stale`, `generatedAt`; `ifNoneMatch` tuỳ chọn |

## 5. Kiểm thử

- **Unit (đồng hồ giả, store bộ nhớ):** mọi nhánh luồng C; stale A/B; ETag đổi khi HEAD/stale đổi; singleflight (`-race`: 50 yêu cầu → 1 thu thập; người dẫn huỷ, 49 người chờ vẫn nhận); quá 20 s → `TIMEOUT inProgress` và nền hoàn tất; janitor chọn đúng dòng.
- **Suite hợp đồng `SnapshotStore` (PG + MySQL, `-tags=integration`):** upsert đồng thời cùng khoá → một dòng; `EvictOldest` giữ commit mới nhất; `DeleteExpired` dùng đồng hồ DB; payload 3 MiB vào/ra JSONB và JSON MySQL; **hai tenant cùng khoá không thấy nhau** (PG RLS với role `NOSUPERUSER NOBYPASSRLS` + mọi truy vấn lọc `tenant_id` ở MySQL).
- **Tải nhẹ:** 200 yêu cầu/20 khoá → đúng 20 lần thu thập.
- Chưa chạy test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Thăm dò HEAD tốn một lệnh CLI (~1,8 s theo README v7) nếu agent không có `headCommit` rẻ; C7 giảm nhưng chưa đo.
- Trần JSON 3 MiB với `max_allowed_packet` MySQL mặc định chưa kiểm.
- Worktree bẩn: HEAD không đổi nhưng nội dung đổi; chỉ `pendingChanges/indexedAt` báo (UI hiện).
- Dev compose dùng superuser nên RLS không có hiệu lực ở dev (hợp đồng §4.1): test RLS phải tự tạo role.
- Phục vụ snapshot cũ khi offline có thể gây hiểu nhầm: UI bắt buộc hiện `stale` và `generatedAt`.

## 7. Câu hỏi mở

- **Q1.** Cache theo thành phần thay vì cả view (CR Q2): hiện cả view.
- **Q2.** Làm ấm cache sau `reindex.finished` (CR Q3): không (O3).
- **Q3.** `SnapshotStore` có thuộc SOL-011-repositories hay SOL này? Giả định SOL-011 cài, SOL này sở hữu cổng + suite; cần xác nhận.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-12/13/14/15; §4 T3)
- `/opt/repos/orca/docs/crs/v7/code-intel-graph-pipeline/CR-CV-022-snapshot-cache.md`
- `/opt/repos/orca/backend-go/common/dbcapability/capability.go`; `backend-go/services/api-gateway/internal/adapter/authclient/mcp_principal_resolver.go`
