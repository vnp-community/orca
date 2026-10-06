# CR-CV-022 — Cache snapshot theo `(repo, commit, view, params)`: độ cũ, singleflight, TTL, dung lượng, ETag, huỷ theo sự kiện

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-022 |
| **Tên** | Lớp cache snapshot đồ thị trong `code-intel-service` (bảng `graph_snapshots`), bọc collector của CR-CV-021 |
| **Loại** | Feature (use case + cổng lưu trữ) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-011 (bảng `graph_snapshots`, repository hai dialect), CR-CV-020 (mô hình, `ResultMeta`), CR-CV-021 (collector) |
| **Mở khoá** | CR-CV-024 (huỷ cache theo sự kiện), CR-CV-040 (ETag qua gateway), CR-CV-060 (so sánh với lượt trước cần snapshot commit cũ) |
| **Tác động** | `backend-go/services/code-intel-service/internal/usecase/ports.go` (cổng `SnapshotStore`), `.../usecase/snapshot_cache.go` (mới), `.../usecase/snapshot_janitor.go` (mới), `.../adapter/{postgres,mysql}` (cài `SnapshotStore`, thuộc CR-CV-011), `.../config/config.go` |

---

## 1. Bối cảnh và vấn đề

1. Mỗi lần gọi CLI qua agent tốn khoảng 1,8 s (README v7 mục 1) và đồ thị Orca rất lớn (247 556 nút, 644 157 cạnh); nhiều người có thể mở cùng view của cùng một worktree (research 06 mục 2, "Đa người dùng"). Không cache thì mỗi lần mở view đều gọi agent và chiếm kênh agent chung với `pty.*`/`fs.*`.
2. README v7 mục 3.5 đã chốt bảng `graph_snapshots` (`id, tenant_id, repo_binding_id, view, commit, params_hash, payload, payload_bytes, truncated, tool_versions, created_at, expires_at`) và research 04 §2 điểm 7 mô tả khoá `(tenant, repo, commit, view, paramsHash)`, huỷ khi `codeintel.changed`. Chưa có quy tắc cụ thể về: `commit` là commit nào, định nghĩa `stale`, chống truy vấn trùng, giới hạn dung lượng, ETag.
3. Hiện trạng code: chưa có `code-intel-service` (không có thư mục `backend-go/services/code-intel-service`). `golang.org/x/sync` là phụ thuộc trực tiếp của `api-gateway` và `git-gateway-service` (`go.mod`) và `singleflight` đã được dùng ở `api-gateway/internal/adapter/authclient/mcp_principal_resolver.go:9,45`; chưa phụ thuộc trực tiếp ở `infra-fleet-service` (`// indirect`). `dbcapability` ghi MySQL `SupportsJSONB=false`, `SupportsReturning=false`, `SupportsRLS=false` (`common/dbcapability/capability.go:44-49`) nên cổng lưu trữ phải tránh `RETURNING` và JSONB-only.

## 2. Giải pháp đề xuất

### 2.1 Khoá và đọc

- **Khoá snapshot** (duy nhất theo chỉ mục `UNIQUE(repo_binding_id, view, "commit", params_hash)`, và `tenant_id` luôn trong mọi truy vấn; không bao giờ chia sẻ snapshot giữa các tenant):
  - `commit` = **commit HEAD của worktree tại thời điểm yêu cầu** (không phải commit của chỉ mục). Lý do: HEAD biết ngay từ thăm dò rẻ (2.3) trước khi gọi truy vấn nặng; commit của chỉ mục chỉ biết sau khi truy vấn. Commit của chỉ mục nằm trong `tool_versions` (`sources[].commit`) để tính `stale`.
  - `params_hash` = SHA-256 của JSON chuẩn tắc (khoá sắp xếp, mặc định đã điền, chỉ tham số hợp đồng) cộng tiền tố `GRAPH_MODEL_VERSION` (hằng trong domain, tăng khi thay đổi quy tắc chuẩn hoá của CR-CV-020 để vô hiệu cache cũ).
- **Luồng đọc** `CachedViewReader.Get(ctx, tenant, binding, view, params)` (`snapshot_cache.go`):
  1. Thăm dò HEAD (2.3). Thành công → `commit`. Thất bại vì offline → nhảy tới bước 5.
  2. `SnapshotStore.Get(binding, view, commit, params_hash)`; còn hạn (`expires_at > now` theo đồng hồ DB, theo CR-REQ-002 F6 của v6) → tính `stale` (2.3), ETag (2.6), trả.
  3. Miss → `singleflight` (2.4) → `CollectView` (CR-CV-021) → cắt/chuẩn hoá (CR-CV-020) → `Put` → trả.
  4. Collector lỗi và có snapshot cũ của `(binding, view, params_hash)` với commit khác → bước 5; không có → trả lỗi của collector.
  5. **Phục vụ snapshot cũ khi dev server offline hoặc collector lỗi tạm** (`CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_TIMEOUT`): lấy `GetLatest(binding, view, params_hash)`, trả `stale=true`, `from_cache=true`. Không áp dụng cho lỗi nghiệp vụ (`INDEX_MISSING`, `PATH_NOT_ALLOWED`, `INVALID_PARAMS`, `TOOL_UNAVAILABLE`): các lỗi đó không bị che bằng dữ liệu cũ.
- **Không lưu** các view sau vào bảng: `STATUS` (chỉ bộ nhớ, TTL 15 s, dùng làm thăm dò) và `SYMBOL` (mã nguồn tới 200 KiB; README v7 mục 6 chỉ trả mã khi mở một symbol, và không ghi mã nguồn xuống DB). `SYMBOL` có LRU bộ nhớ nhỏ (mặc định 64 mục, TTL 5 phút, khoá như trên) để mở lại panel không gọi lại agent; không chia sẻ giữa replica.

### 2.2 Cổng lưu trữ (mới, `ports.go`)

```go
type SnapshotStore interface {
    Get(ctx, tenantID, bindingID string, view ViewKind, commit, paramsHash string) (Snapshot, bool, error)
    GetLatest(ctx, tenantID, bindingID string, view ViewKind, paramsHash string) (Snapshot, bool, error)
    Put(ctx, s Snapshot) error                                  // upsert theo khoá duy nhất
    DeleteByBinding(ctx, tenantID, bindingID string) (int64, error)
    DeleteExpired(ctx context.Context, limit int) (int64, error) // cross-tenant, chỉ janitor dùng
    TotalBytes(ctx, tenantID, bindingID string) (int64, error)
    TenantBytes(ctx, tenantID string) (int64, error)
    EvictOldest(ctx, tenantID, bindingID string, keepCommits int, targetBytes int64) (int64, error)
}
```

`Snapshot` = các cột của `graph_snapshots` + `etag`, `total_count`. Postgres dùng `INSERT … ON CONFLICT … DO UPDATE`; MySQL `ON DUPLICATE KEY UPDATE` (không `RETURNING`). `payload` là protojson của message `data` (ví dụ `ArchitectureGraph`), `tool_versions` chứa danh sách `SourceInfo`. Phần cài đặt SQL thuộc CR-CV-011; CR này chỉ nêu hợp đồng và các cột/chỉ mục cần thêm vào đó (mục 2.8).

### 2.3 Phát hiện stale so với HEAD

Hai nguồn độ cũ, `stale = A || B`:

- **A: chỉ mục lệch HEAD.** `source.commit != headCommit` với **bất kỳ** nguồn nào trong `sources[]` đã dùng; hoặc `pendingChanges` của CodeGraph khác 0 (research 04 §2 điểm 1: `stale = lệch commit hoặc working tree bẩn`). Dữ liệu có sẵn trong kết quả `codeintel.status`.
- **B: công cụ mới hơn snapshot.** `status.version` của một công cụ khác `tool_versions[].version` đã lưu, hoặc `status.indexedAt` mới hơn → snapshot bị coi là **miss** (không chỉ stale): nội dung có thể khác. Đây là cách phát hiện reindex không có sự kiện (ví dụ người dùng chạy tay trên dev server) trong lúc chưa có `codeintel.indexChanged`.

Thăm dò: `codeintel.status` qua collector, **bộ nhớ đệm trong tiến trình 15 s** theo `(tenant, binding)` (`CODEINTEL_HEAD_PROBE_TTL`), chia sẻ bằng singleflight; sự kiện `indexChanged` xoá mục này (CR-CV-024). HEAD thay đổi mà chỉ mục chưa đổi (agent code, commit mới) được phát hiện trong tối đa 15 s sau, không cần sự kiện. Chỉ số `pendingChanges` phụ thuộc CR-CV-001/003 (agent) trả đúng; chưa kiểm chứng ở CR này.

`stale` **không** kích hoạt thu thập lại tự động (O3: người dùng bấm làm mới qua `codeintel.reindex`). Thu thập lại cùng chỉ mục cũ cho ra cùng dữ liệu cũ; chỉ tốn tài nguyên.

### 2.4 Singleflight chống truy vấn trùng

- Dùng `golang.org/x/sync/singleflight` (đã là phụ thuộc trực tiếp ở `api-gateway`, `git-gateway-service`; thêm trực tiếp vào `go.mod` của service).
- Khoá nhóm: `tenant|binding|view|commit|params_hash`. Một goroutine dẫn thực hiện bước 3 của 2.1; các goroutine còn lại chờ kết quả.
- **Ngữ cảnh tách rời**: công việc dẫn chạy với `context.WithoutCancel` + timeout riêng (`CODEINTEL_COLLECT_TIMEOUT`, mặc định 100 s, lớn hơn timeout infra-fleet cho `codeintel.*` ở CR-CV-023), dùng `DoChan`; từng người chờ `select` giữa kết quả và `ctx` của mình. Nếu người dẫn huỷ yêu cầu, những người chờ khác không nhận `context canceled`.
- Singleflight chỉ trong một replica. Hai replica có thể cùng thu thập một khoá; kết quả giống nhau và `Put` là upsert nên không hỏng dữ liệu. Chấp nhận (không dùng khoá tư vấn DB) vì chi phí trùng hiếm.
- Lỗi **không** được cache (không lưu "negative cache"), nhưng nhóm singleflight giữ nhiều lỗi đồng thời chia sẻ lỗi của người dẫn.

### 2.5 TTL, dung lượng, dọn dẹp

| Thông số | Mặc định | Ghi chú |
|---|---|---|
| `expires_at` | `created_at` + `CODEINTEL_SNAPSHOT_TTL` = **7 ngày** | Snapshot gắn commit nên nội dung bất biến; TTL chỉ để dọn rác |
| Số commit giữ mỗi `(binding, view)` | **3** (`CODEINTEL_SNAPSHOT_KEEP_COMMITS`) | Cần cho "so sánh với lượt trước" (CR-CV-060) |
| Kích thước một snapshot | ≤ **3 MiB** `payload` | Cùng ngân sách CR-CV-020 mục 2.7; vượt thì không lưu (`Put` bỏ qua, log), vẫn trả cho người gọi |
| Tổng mỗi binding | **64 MiB** (`CODEINTEL_CACHE_MAX_BYTES_PER_BINDING`) | |
| Tổng mỗi tenant | **512 MiB** (`CODEINTEL_CACHE_MAX_BYTES_PER_TENANT`) | |

Janitor (`snapshot_janitor.go`, goroutine nền, khoảng 5 phút, dừng theo `ctx` khi tắt service): `DeleteExpired` theo lô (≤ 500 dòng/lần), rồi với binding vượt trần `EvictOldest` (xoá theo `created_at` tăng dần, **không bao giờ** xoá snapshot của commit mới nhất cho mỗi view). Sau `Put`, nếu `TotalBytes` vượt trần thì chạy cùng hàm ngay cho binding đó. Các thao tác chạy đồng hồ DB.

### 2.6 ETag

- `contentEtag` lưu cùng snapshot = hex SHA-256 (16 byte đầu) của `(binding_id, view, commit, params_hash, tool_versions chuẩn tắc, sha256(payload))`, tính khi `Put`.
- ETag trả cho client = `"` + hex SHA-256 (16 byte đầu) của `(contentEtag, headCommit, stale)` + `"` (ETag mạnh). `stale` và `headCommit` nằm trong đó nên khi HEAD đổi hoặc `stale` lật, ETag đổi dù payload không đổi.
- Mọi request đồ thị (CR sở hữu RPC) mang `string if_none_match`. Khớp → response chỉ có `ResultMeta{not_modified=true, etag, head_commit, stale, sources}` không `data` (cần field `not_modified=13` đã thêm vào `ResultMeta` ở CR-CV-020). `ResultMeta.etag` luôn được điền.
- Gateway (CR-CV-040) chuyển `if_none_match` từ tham số `codeIntel.*` xuống gRPC; không có HTTP điều kiện vì kênh là WS.
- Snapshot phục vụ qua nhánh offline (2.1 bước 5) cũng có ETag; nội dung cũ + `stale=true` cho ETag khác ETag lúc còn tươi.

### 2.7 Huỷ theo sự kiện và theo thao tác

API `SnapshotInvalidator.InvalidateBinding(ctx, tenantID, bindingID, reason)` (use case; gọi `DeleteByBinding`, xoá thăm dò HEAD, xoá LRU `SYMBOL` của binding):

| Nguồn gọi | Khi nào |
|---|---|
| CR-CV-024 | nhận `codeintel.indexChanged` hoặc `orca.codeintel.reindex.finished` |
| CR-CV-012 | `workspace_root`/`dev_server_id`/`gitnexus_repo` của binding đổi, hoặc binding bị xoá |
| CR-CV-004 (agent) → CR-CV-024 | reindex hoàn tất |
| Quản trị | xoá thủ công (tuỳ chọn, ngoài phạm vi) |

Ngoài ra có `InvalidateProbe(ctx, tenantID, bindingID)` chỉ xoá thăm dò HEAD và LRU `SYMBOL` **trong bộ nhớ** (không đụng DB), CR-CV-024 gọi khi `resync`/`overflow`: snapshot DB tự được kiểm lại ở lần đọc kế qua quy tắc B (mục 2.3), nên không cần xoá hàng loạt sau mỗi lần triển khai lại.

Huỷ bằng `InvalidateBinding` là **xoá**, không đánh cờ; nhờ vậy không cần cột trạng thái và lần đọc kế tiếp thành miss sạch. Huỷ phải **idempotent** (xoá 0 dòng không lỗi), vì sự kiện giao lặp (README v7 mục 6).

### 2.8 Yêu cầu bổ sung vào `graph_snapshots` (cho CR-CV-011)

README v7 mục 3.5 chưa có các mục sau, cần thêm:

| Thay đổi | Lý do |
|---|---|
| Cột `etag` (CHAR(32)) và `total_count` (BIGINT) | Trả `ResultMeta` mà không giải mã payload 3 MiB; `total_count` cần giữ sau khi cắt |
| Chỉ mục duy nhất `(repo_binding_id, view, commit, params_hash)` | Upsert; `commit` là từ khoá SQL ở một số dialect, cột phải được trích dẫn (`"commit"` Postgres, `` `commit` `` MySQL) hoặc đổi tên `head_commit` trong CR-CV-011 (khuyến nghị) |
| Chỉ mục `(tenant_id, repo_binding_id, created_at)` | `EvictOldest`, `TotalBytes` |
| Chỉ mục `(expires_at)` | `DeleteExpired` |
| `tool_versions` chứa mảng `SourceInfo` (`tool`, `version`, `indexedAt`, `commit`) | Tính `stale` mục A và B |

### 2.9 Cấu hình

`CODEINTEL_HEAD_PROBE_TTL` (15s), `CODEINTEL_COLLECT_TIMEOUT` (100s), `CODEINTEL_SNAPSHOT_TTL` (168h), `CODEINTEL_SNAPSHOT_KEEP_COMMITS` (3), `CODEINTEL_CACHE_MAX_BYTES_PER_BINDING` (67108864), `CODEINTEL_CACHE_MAX_BYTES_PER_TENANT` (536870912), `CODEINTEL_SYMBOL_LRU_ENTRIES` (64), `CODEINTEL_SYMBOL_LRU_TTL` (5m). Tất cả đọc qua `commonconfig` như CR-CV-010.

### 2.10 Quan sát

`codeintel_cache_requests_total{view,result="hit|miss|stale_served|offline_served|not_modified"}`, `codeintel_cache_bytes{scope}`, `codeintel_singleflight_shared_total`, `codeintel_cache_evictions_total{reason}` (tên cuối ở CR-CV-071).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| `commit` trong khoá là HEAD, không phải commit chỉ mục | Biết trước khi truy vấn; commit chỉ mục dùng để tính `stale` |
| `stale` không kích hoạt thu thập lại | O3: chỉ người dùng làm mới; cùng chỉ mục cũ cho cùng kết quả |
| Huỷ bằng xoá | Không thêm cột trạng thái; idempotent |
| Không lưu `SYMBOL` và `STATUS` vào DB | Mã nguồn không nên nằm ở DB; `status` đổi liên tục |
| `GRAPH_MODEL_VERSION` trong `params_hash` | Đổi quy tắc chuẩn hoá (CR-CV-020) tự vô hiệu cache cũ mà không cần migration |
| Singleflight theo replica, không khoá DB | Trùng hiếm, `Put` là upsert; tránh độ phức tạp hai dialect |
| ETag chứa `stale` và `headCommit` | Tránh UI giữ trạng thái "tươi" sai sau khi HEAD đổi |
| Phục vụ snapshot cũ khi offline nhưng không che lỗi nghiệp vụ | Offline là tạm thời; `INDEX_MISSING` v.v. phản ánh sự thật cần người dùng xử lý |
| Giữ 3 commit mỗi view | Phục vụ so sánh lượt trước (CR-CV-060), vẫn nằm trong trần dung lượng |

## 4. Tiêu chí chấp nhận

- [ ] Lần gọi đầu miss, lần hai cùng khoá hit mà **không** gọi agent (đếm lời gọi `RelayByDevServer` bằng giả).
- [ ] 50 yêu cầu đồng thời cùng khoá chỉ tạo **một** lần thu thập; người dẫn huỷ `ctx` giữa chừng, 49 người chờ vẫn nhận kết quả.
- [ ] Khi `sources[].commit != headCommit`, kết quả trả `stale=true` và không tự gọi lại agent; khi `pendingChanges` khác 0, `stale=true`.
- [ ] Phiên bản công cụ hoặc `indexedAt` trong `status` khác bản lưu → miss, thu thập lại, snapshot mới ghi đè.
- [ ] `If-None-Match` đúng ETag trả `not_modified=true` không `data`; sau khi HEAD đổi (cùng payload) ETag khác.
- [ ] `InvalidateBinding` xoá mọi snapshot của binding (mọi view/commit) và mục thăm dò HEAD; gọi lại hai lần không lỗi.
- [ ] Dev server offline + có snapshot: trả snapshot với `stale=true`, `from_cache=true`; offline + không có snapshot: lỗi `CODEINTEL_DEV_SERVER_OFFLINE`; `CODEINTEL_INDEX_MISSING` không bị che dù có snapshot cũ.
- [ ] `payload` > 3 MiB không được lưu nhưng vẫn trả cho người gọi; tổng binding vượt 64 MiB bị đưa về ≤ 64 MiB và vẫn giữ commit mới nhất của từng view.
- [ ] Không có dòng `graph_snapshots` cho view `STATUS` và `SYMBOL`.
- [ ] Hai tenant cùng `(binding, view, commit, params_hash)` không thấy dữ liệu của nhau (Postgres RLS và kiểm thử mọi truy vấn có `tenant_id` trên MySQL).
- [ ] Chạy ở cả Postgres và MySQL; không dùng `RETURNING` ở đường MySQL.

## 5. Kiểm thử

- **Unit** (đồng hồ giả, `SnapshotStore` giả trong bộ nhớ): luồng 2.1 mọi nhánh; tính `stale` A/B; ETag; quy tắc singleflight (race detector `-race`); janitor chọn đúng dòng xoá.
- **Tích hợp (`-tags=integration`, testcontainers, hai dialect):** upsert đồng thời hai lần cùng khoá (không lỗi, một dòng); `EvictOldest` giữ commit mới nhất; `DeleteExpired` dùng đồng hồ DB; RLS Postgres; JSON 3 MiB vào/ra cả JSONB và JSON MySQL.
- **Hồi quy tải nhẹ:** 200 yêu cầu song song trên 20 khoá, đo số lần thu thập = 20.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Thăm dò HEAD qua `codeintel.status` tốn một lời gọi CLI (~1,8 s theo README); với TTL 15 s và nhiều binding có thể tạo tải đều trên agent. Phương án nhẹ hơn (đọc `.git/HEAD`/`git rev-parse` qua `git.*`) chưa được khảo sát; nếu `codeintel.status` của agent lấy từ `.gitnexus/meta.json` thì rẻ (CR-CV-001 chưa chốt).
- Giả định agent trả `pendingChanges` và `sources[].commit` đúng; GitNexus `meta.json` có `lastCommit` (research 04 §1.1), CodeGraph có `pendingChanges` (đã thấy trong `codegraph status --json`), nhưng việc `codeintel.status` hợp nhất chúng chưa có code.
- Trần 3 MiB với JSON MySQL và `max_allowed_packet` mặc định của môi trường chưa kiểm.
- `commit` có thể trùng từ khoá SQL; cần xác nhận tên cột ở CR-CV-011.
- Worktree có thay đổi chưa commit: HEAD không đổi nhưng nội dung đổi; chỉ `pendingChanges`/`indexedAt` báo ra, nên snapshot có thể "tươi" so với HEAD mà thực ra lệch working tree. Chấp nhận (UI hiển thị `pendingChanges`), cần đo.
- Phục vụ snapshot cũ khi offline có thể làm người dùng tưởng dữ liệu mới; UI bắt buộc hiện `stale` và `generated_at` (CR-CV-051).

## 7. Câu hỏi mở

- **Q1.** `commit` trong khoá dùng HEAD hay "commit chỉ mục"? CR này chọn HEAD (2.1); nếu CR-CV-011 đã đặt tên khác thì đồng bộ tên cột.
- **Q2.** Có cần cache **thành phần** (cache từng cụm/process) thay vì cả view không? Hiện cache cả view theo `params_hash`.
- **Q3.** Thêm chuyển tiếp "làm ấm cache" sau `reindex.finished` (tự tải `ARCHITECTURE`, `FLOWS`)? Hiện không (O3, tránh tải bất ngờ); đo tỉ lệ cold-start rồi quyết định.
- **Q4.** `STATUS` có cần lưu để vẽ lịch sử index không? Hiện không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.5 `graph_snapshots`, mục 6)
- `/opt/repos/orca/docs/research/view-code/04-raw-data-and-pipeline.md` (mục 2 điểm 1, 7, 8), `06-gaps-risks-roadmap.md` (mục 2)
- `/opt/repos/orca/backend-go/common/dbcapability/capability.go` (dòng 44-49)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/authclient/mcp_principal_resolver.go` (dòng 9, 45, mẫu `singleflight`)
- `/opt/repos/orca/backend-go/services/api-gateway/go.mod`, `/opt/repos/orca/backend-go/services/git-gateway-service/go.mod` (`golang.org/x/sync`)
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/README.md` (F6: đồng hồ DB)
- CR-CV-011, CR-CV-020, CR-CV-021, CR-CV-024 (cùng series)
