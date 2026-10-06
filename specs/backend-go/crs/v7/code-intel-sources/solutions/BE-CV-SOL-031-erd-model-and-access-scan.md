# BE-CV-SOL-031-erd-model-and-access-scan: `ErdModel`, quan hệ logic, `accessedBy`, proto `codeintel_erd.proto` và RPC `GetErd`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần 2/2 của CR-CV-031; cần [`BE-CV-SOL-031-sql-migration-parser`](./BE-CV-SOL-031-sql-migration-parser.md) (`Catalog`) và [`BE-CV-SOL-030`](./BE-CV-SOL-030-repo-file-access-gateway.md) (đọc file).

**CR:** [CR-CV-031](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-031-sql-migration-to-erd.md)
**Service:** `code-intel-service` — `proto/orca/codeintel/v1/codeintel_erd.proto` (mới) + `codeintel.proto` (thêm `rpc GetErd`), `internal/usecase/build_erd.go` (mới), `internal/adapter/gosqlscan` (mới), `internal/adapter/grpc` (handler)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) ("Multi-tenancy", "Read models / query needs across service boundaries"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("AuthZ", "Multi-tenancy isolation"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("gRPC conventions"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) ("The three pillars")

---

## 0. Hợp đồng áp dụng

| Mã | Áp dụng | Mục |
|---|---|---|
| PQ-04 | Request nhận `WorktreeSelector selector = 1`; bỏ `repo_binding_id` | §1, §3.1 |
| PQ-07 | File `codeintel_erd.proto`; RPC thêm vào `service CodeIntelService` ở `codeintel.proto`; mọi enum có `_UNSPECIFIED = 0` nếu có | §1 PQ-07, §2.1 hàng 9 |
| PQ-12 | Kết quả có `ResultMeta` (`etag`, `not_modified`, `from_cache`…); tham số `if_none_match` | §1 PQ-12, §2.3 |
| PQ-14 | Response ≤ 2 MiB (`proto.Size`); ngân sách ≤ 300 bảng/≤ 2 000 cột → `truncated` | §1 PQ-14 |
| PQ-15 | `graph_snapshots` view `erd`, `head_commit`, `params_hash`, `etag`, `schema_version` | §1, §4 T3 |
| PQ-28 | `erd-links.yaml` là **tệp trong repo** `docs/code-intel/erd-links.yaml` đọc qua `RepoSourceReader`; không có bảng DB | §1 PQ-28 |
| PQ-29 | `TableAccess` giữ riêng (không gộp `StoreAccess`); `ParseWarning` riêng; `SourceRef`/`SymbolRef` do CR-020 | §1 PQ-29 |
| PQ-03 | Mã lỗi (`CODEINTEL_INVALID_PARAMS`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_TIMEOUT`…) | §1 PQ-03 |
| PQ-13 | Đọc view 20 s ở gateway; service trả `CODEINTEL_TIMEOUT` có hậu tố `retryAfterMs` và tiếp tục nền (singleflight) | §1 PQ-13 |
| PQ-20 | `SymbolRef.key` do backend dựng cho Go theo cùng hàm với agent/CR-020 | §1 PQ-20 |
| PQ-23 | `ERD_MIGRATION_GLOB` → `CODEINTEL_ERD_MIGRATION_GLOB` | §1 PQ-23 |
| §3.1 | RPC `GetErd`: `service, dialect, base_ref, head_ref, include_access, include_inferred`; OPA `read`; kênh `codeIntel.erd` | §3.1 |
| UI §4.4 | Hình dạng JSON `ErdModel`, `ErdTable`, `ErdRelation`, `ErdChange`, `ErdServiceInfo` | `CONTRACT-ui-api` §3.1, §4.4 |
| H10, §8.3-3/4 | Mọi nhánh hai dialect có test hai dialect; cache/`graph_snapshots` có `tenant_id` và test cô lập tenant | §0, §8.3 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/proto/orca/**` (không có `codeintel/`); `backend-go/services/infra-fleet-service/internal/adapter/{postgres,mysql}/` (có `repository.go`; `ls` thấy thư mục adapter: `agentwsserver backendrelaysshprovisioner devserveragent ephemeralsshconn eventbus grpc grpcclient metrics mysql portalloc portevents postgres sshconn sshrelay webhook`); `migrations/postgres` có số lớn nhất `0038`; các file đã nêu ở SOL-031-parser; `CONTRACT-ui-api` §3.1 (dòng kênh `erd`) và §4.4 (type `ErdModel`…).

Xác nhận: `code-intel-service` và `proto/orca/codeintel` chưa tồn tại; `docs/code-intel/` **chưa có** (`ls` báo không tồn tại) nên `erd-links.yaml` chưa có và E7 chỉ có từ chú thích `logical FK`; không có `CODEOWNERS`.

### Correction relative to CR-CV-031

| # | CR nói | Hợp đồng/mã | Xử lý |
|---|---|---|---|
| C1 | `GetErdRequest{repo_binding_id=1, service=2, dialect=3, ref=4, include_access=5, include_inferred=6}` | PQ-04 + §3.1: `selector`, `base_ref`, `head_ref` thay `ref` | Mục 2.A: `selector=1, service=2, dialect=3, base_ref=4, head_ref=5, include_access=6, include_inferred=7, if_none_match=8` (số do chủ sở hữu CR gán, đề xuất) |
| C2 | `TableAccess.op` ∈ `read|write` | UI §4.4: `read|write|readwrite` | Dùng `readwrite` cho `INSERT … SELECT` hoặc hàm vừa đọc vừa ghi cùng bảng |
| C3 | `ErdModel` không có `changes[]` | PQ… README v7 mục 8 điểm 11 + UI §4.4: `changes[]`; contract §2.1 liệt kê `ErdChange (mới)` | Thêm `repeated ErdChange changes = 8` vào `ErdModel` |
| C4 | `GetErd` luôn trả một `ErdModel` | UI §3.1: không có `service` → `{services: ErdServiceInfo[]}` | **Hợp đồng thiếu**: `GetErdResponse` §3.1 không có chỗ cho `services`. Solution đề xuất thêm `repeated ErdServiceInfo services = 4` (xem G1) |
| C5 | `UI ErdModel.warnings[]` nằm trong model | §3.1: response `{model, meta, warnings[]}` | Giữ `warnings` ở **response** (proto); gateway (CR-040) gộp vào `data.warnings` — ghi G2 |
| C6 | "Số `logical FK` 17 file Postgres" | 16 Postgres + 11 MySQL (SOL-031-parser C2) | Mẫu nhận dạng cho cả hai dialect |

## 2. Giải pháp

### A. Proto `codeintel_erd.proto` (mới) và RPC

Import `codeintel_common.proto` (`WorktreeSelector`, `ResultMeta`, `SymbolRef`); không import `codeintel.proto` (PQ-07 quy tắc phụ thuộc).

```proto
// codeintel.proto: thêm rpc GetErd(GetErdRequest) returns (GetErdResponse);
message GetErdRequest {
  WorktreeSelector selector = 1;
  string service = 2;            // thư mục service; rỗng = liệt kê dịch vụ
  string dialect = 3;            // "postgres" | "mysql"; rỗng = postgres
  string base_ref = 4;           // rỗng = không tính changes[]
  string head_ref = 5;           // rỗng = working tree; hoặc commit 40/64 hex
  bool include_access = 6;
  bool include_inferred = 7;     // bật quan hệ source="naming"
  string if_none_match = 8;
}
message GetErdResponse {
  ErdModel model = 1; ResultMeta meta = 2; repeated ParseWarning warnings = 3;
  repeated ErdServiceInfo services = 4;   // chỉ khi service rỗng (G1)
}
message ErdModel {
  string service = 1; string dialect = 2; string schema = 3; string as_of_migration = 4;
  repeated ErdTable tables = 5; repeated ErdRelation relations = 6; repeated ErdExternalRef external_refs = 7;
  repeated ErdChange changes = 8;
}
message ErdServiceInfo { string name = 1; repeated string dialects = 2; int32 table_count = 3; }
// ErdTable(1..14), ErdColumn(1..9), ErdIndex(1..6), ErdCheck, ErdPolicy, ErdRelation(1..9),
// ErdEndpoint{service=1,table=2,columns=3}, ErdExternalRef, TableAccess, ParseWarning:
// số field theo CR-CV-031 §2.7 (chuẩn, hợp đồng §2 "khi CR đã ghi số thì số đó là chuẩn").
message ErdChange {
  string table = 1; string column = 2; string kind = 3;       // added|removed|modified
  ErdColumnShape before = 4; ErdColumnShape after = 5;
  string migration_file = 6; int32 line = 7;                   // số do chủ sở hữu CR gán
}
message ErdColumnShape { string type = 1; bool nullable = 2; string default_expr = 3; }
```

`ErdEndpoint.service` thêm theo README v7 mục 8 điểm 11 (CR đã ghi `service=1`). `TableAccess{ SymbolRef symbol=1; string op=2; double confidence=3 }` (+ `source` nếu cần: `static-scan`, do chủ sở hữu CR gán). Mọi `string` kiểu enum ở đây là chuỗi mở (CR dùng chuỗi); không enum proto → không cần `_UNSPECIFIED`.

### B. Cây file (mới)

```
proto/orca/codeintel/v1/codeintel_erd.proto
internal/domain/erd/erd_model.go            # ErdModel miền: Table, Relation, Access, Change (từ Catalog)
internal/domain/erd/relation_inference.go   # fk, logical(comment|declared|naming), cardinality, tenant_scoped
internal/domain/erd/dialect_drift.go        # so tập bảng/cột hai dialect → DIALECT_DRIFT
internal/adapter/erdlinks/declared_links.go # đọc docs/code-intel/erd-links.yaml qua RepoSourceReader (yaml.v3, KnownFields, cấm alias)
internal/adapter/gosqlscan/scan_table_access.go   # go/parser: literal SQL → (bảng, op, hàm bao)
internal/usecase/build_erd.go               # BuildErd: khám phá service, Catalog, quan hệ, access, changes, ngân sách
internal/adapter/grpc/get_erd_handler.go    # RPC GetErd
```

### C. Quan hệ (CR 2.5; theo ưu tiên, mỗi quan hệ có `source` + `confidence`)

1. `fk` (`source:"ddl"`, 1.0): từ `REFERENCES`/`FOREIGN KEY` cùng service; `on_delete`, `on_update`.
2. `logical`, `cross_service:true`:
   - `declared` (1.0): `docs/code-intel/erd-links.yaml` `{from:{service,table,column}, to:{…}, note}` (PQ-28); thiếu tệp = không lỗi; YAML dùng `yaml.v3` với `KnownFields(true)`, cấm alias/anchor (cùng quy tắc `c4-overrides-yaml`), ≤ 64 KiB.
   - `comment` (0.8): mẫu `logical FK\s*(->|to|into|back to)\s*([a-z_-]+)(\.([a-z_]+))?(\.([a-z_]+))?` trên cùng dòng khai báo cột; chỉ tên service → cột đích `id`, bảng đích suy từ tên cột nếu duy nhất; biến thể có hậu tố (`; nullable`, `(no cross-database FK)`) chịu được; **quét cả hai dialect** (SOL-031-parser C2). Parser phải **giữ chú thích cùng dòng** của cột (token comment không được bỏ mất trước khi nhận dạng): `Column.TrailingComment` trong `erd` miền — việc bổ sung vào tokenizer là phụ thuộc, ghi ở task 08.
   - `naming` (0.4): `*_id` khớp tên bảng số ít duy nhất của service khác; **tắt mặc định**, chỉ khi `include_inferred`.
3. Không vẽ `tenant_id` thành cạnh; `Table.tenant_scoped = có cột tenant_id`.
4. `cardinality`: `one-to-one` nếu cột nguồn nằm trong UNIQUE/PK đơn; còn lại `many-to-one`.
5. `external_refs`: quan hệ `logical` đi ra/đi vào service đang xem.
6. Lệch hai dialect: `ParseWarning{code:"DIALECT_DRIFT"}` (ví dụ `mcp-service` chỉ Postgres). `tenantEnforcement:"application"` cho MySQL: thông tin suy ra từ `rls_state` của model MySQL (`none`), không thêm field proto riêng.

### D. `accessedBy` (CR 2.6)

Với service đã chọn: liệt kê `internal/adapter/postgres/*.go` và `internal/adapter/mysql/*.go` không-test (qua `ListDir`/`ReadFiles`); `go/parser` `ParseFile`, duyệt `*ast.BasicLit` chuỗi và `fmt.Sprintf`; mẫu `\b(FROM|JOIN|INTO|UPDATE|DELETE\s+FROM|TRUNCATE(\s+TABLE)?)\s+([a-z_][a-z0-9_]*\.)?([a-z_][a-z0-9_]*)` (không phân biệt hoa/thường), **đối chiếu với tập bảng của Catalog** để loại từ khoá/bí danh. `op`: `FROM|JOIN`→`read`; `INSERT INTO|UPDATE|DELETE FROM|TRUNCATE`→`write`; `INSERT … SELECT` và cùng hàm vừa đọc vừa ghi cùng bảng → `readwrite`. `SymbolRef.key = "method:<relPath>:<Receiver>.<Name>"` hoặc `function:<relPath>:<Name>` (PQ-20; hàm dựng khoá của BE-CV-SOL-020, chưa kiểm chứng tên). `confidence 0.9` khi khớp có tiền tố schema (Postgres), `0.6` khi chỉ khớp tên trần; literal chứa `%s|%v` ngay sau `FROM|INTO|UPDATE` → cờ `dynamicSqlSuspected` (cảnh báo `DYNAMIC_SQL_SUSPECTED`). Chỉ `adapter/{postgres,mysql}` (không quét `usecase`); bảng không ai truy cập → `accessedBy=[]` (phát hiện hợp lệ).

### E. `changes[]` (base/head, README v7 điểm 11)

`base_ref` rỗng → không tính. Có `base_ref`: `ChangedFiles(base)` (BE-CV-SOL-030) cho `MergeBase` và danh sách file; các migration **mới** (`added`) chỉ áp trên `head`; catalog `base` = tập head trừ file `added`, cộng nội dung gốc (`git show <mergeBase>:<path>`) của file `modified`/`deleted` (sửa migration cũ là mùi, vẫn xử lý). `changes[]` = so catalog base và head (bảng/cột thêm/bớt/đổi kiểu-nullable-default), kèm `migration_file` và `line` của câu lệnh gây ra (lần cuối chạm). Tối ưu "chỉ đọc file thêm" **chưa kiểm chứng**; trường hợp không chắc → đọc đủ ở base (chậm hơn nhưng đúng). Dùng chung với `BE-CV-SOL-038-contract-diff` (`MigrationDiff`/`SqlChange`): cùng hàm so catalog.

### F. Use case, cache, quyền, lỗi

- `GetErd`: `tenant.RequireTenantID`; phân giải `selector` → binding (BE-CV-SOL-012); OPA `read` (BE-CV-SOL-013); `ResolveHead`, `DirtyPaths(scope=migrations/**, internal/adapter/{postgres,mysql}/**)`.
- Khám phá: glob `CODEINTEL_ERD_MIGRATION_GLOB` (mặc định `backend-go/services/*/migrations/{postgres,mysql}`); `service` không có → `services[]` (đếm bảng bằng parser nhanh hoặc từ cache).
- Cache `graph_snapshots(view="erd")` qua BE-CV-SOL-022: khoá `(tenant, repo_binding, view, head_commit, params_hash)`; `params_hash` = băm `(service, dialect, base_ref, head_ref, include_*, tập (path, ContentHash) bẩn, schema_version)`. `etag` 32 hex; `if_none_match` khớp → `not_modified`. Không lưu mã nguồn thô; chỉ `ErdModel` đã dựng.
- Lỗi: `service` không có → `CODEINTEL_INVALID_PARAMS` (`field:"service"`); dialect lạ → `INVALID_PARAMS`; không tìm thấy thư mục migrations → `ErdModel` rỗng + `ParseWarning{MIGRATIONS_NOT_FOUND}`; vượt ngân sách → `truncated`; vượt 20 s → `CODEINTEL_TIMEOUT` + tiếp tục nền.
- Che: `default_expr`/`CHECK` không chứa secret, nhưng vẫn qua bộ che chung (O-16) trước khi trả.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| `GetErd` theo một service mỗi lần | Ngân sách và "service sở hữu DB riêng" (CR) |
| `TableAccess.op` có `readwrite` | UI §4.4 là chuẩn |
| `erd-links.yaml` là tệp repo, không bảng | PQ-28; không có UI sửa |
| `changes[]` tính từ danh sách file đổi | Không đọc 294 file ở base |
| Quét SQL chỉ ở `adapter/{postgres,mysql}` | Kiến trúc hexagonal; giảm dương tính giả |
| `naming` tắt mặc định | Nhiễu (CR Q3) |
| Cảnh báo `degraded`/`unknown` hiển thị | H7 |

## 4. Lệch giữa CR và hợp đồng

C1–C6 ở mục 1. Thêm: L1 `ERD_MIGRATION_GLOB` → `CODEINTEL_ERD_MIGRATION_GLOB` (PQ-23). L2 Q2 CR "erd-links trong repo hay bảng DB" đã chốt PQ-28 = repo. L3 Q4 CR (schema MySQL) — xem SOL-031-parser O-S1. L4 CR đặt message trong `erd.proto`; PQ-07 đổi `codeintel_erd.proto`.

## 5. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-031-sql-migration-parser` | `Catalog` |
| BE | `BE-CV-SOL-030-repo-file-access-gateway` | đọc file, `DirtyPaths`, `ChangedFiles` |
| BE | `BE-CV-SOL-010`, `011` (view `erd`, `graph_snapshots`), `012` (selector→binding), `013` (OPA `read`), `022` (cache), `020` (`SymbolRef`, `ResultMeta`, `codeintel_common.proto`) | nền |
| BE | `BE-CV-SOL-034-data-flow-model`, `035-storage-map`, `038-contract-diff`, `038-static-tenant-filter-rule` | tiêu thụ `ErdModel`/`accessedBy`/so sánh catalog |
| BE | `BE-CV-SOL-040-codeintel-view-channels` | kênh `codeIntel.erd` (mapping `ErdServiceInfo`, `warnings`) |
| FE | `FE-CV-SOL-057-erd-lens` | tiêu thụ `ErdModel` (kiểu `CONTRACT-ui-api` §4.4) |
| AG | — | không có việc agent |

## 6. Tiêu chí chấp nhận

- [ ] `GetErd(infra-fleet-service, postgres)` đủ 20 bảng, mỗi bảng có `tenant_scoped`, RLS đúng, `as_of_migration=0038_…`; `approval_status`, `group_id` (từ `0030`) có mặt.
- [ ] `logical` từ chú thích: đủ cả hai dialect, mỗi quan hệ trỏ đúng service đích hoặc báo đích chưa xác định; `naming` chỉ khi `include_inferred`.
- [ ] `accessedBy`: `infra.dev_servers` ở `repository.go` có `op` đúng và `SymbolRef.key` hợp lệ; `provider_registry_entries` (không truy cập) → `[]`.
- [ ] `DIALECT_DRIFT` đúng cho `mcp-service`.
- [ ] `base_ref` có → `changes[]` đúng cho một migration mới thêm cột; file migration chưa commit (untracked) có trong `head`.
- [ ] Không `service` → `services[]`; có `ResultMeta` (`head_commit`, `stale`, `truncated`, `etag`); `if_none_match` khớp → `not_modified` và không `model`.
- [ ] Hai tenant cùng `repo`, cache/snapshot không chéo; truy vấn snapshot có `tenant_id`.
- [ ] Không mã nguồn migration trong DB/log; không `helpers/utils/common/misc`; không `max-lines` disable; `buf lint` + `buf breaking` xanh.

## 7. Kiểm thử, rủi ro, câu hỏi mở

**Kiểm thử.** Unit: suy quan hệ (từng `source`), cardinality, quét SQL bảng ca (`Sprintf`, raw nhiều dòng, bí danh, `JOIN`, `INSERT … SELECT`, động), so catalog base/head. Golden (CR-CV-070): `ErdModel` JSON cho `infra-fleet-service` (hai dialect), `mcp-service`, `task-service`. Integration `-tags=integration` (từng dialect trong ma trận CI `dialect: [postgres, mysql]`): ghi/đọc `graph_snapshots` view `erd`, upsert, hết hạn, **cô lập tenant bằng role `NOSUPERUSER NOBYPASSRLS`** (Postgres) và test AST/`WHERE tenant_id` (MySQL) — repository do `BE-CV-SOL-011-repositories-and-maintenance`; solution này chỉ thêm ca view `erd`. Hiệu năng: dựng `infra-fleet-service` với agent giả trễ 100 ms, mục tiêu < 10 s lạnh / < 1 s nóng (mục tiêu, chưa đo). Lệnh dự kiến: `go test ./services/code-intel-service/...`, `cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (không dùng `make proto-lint` có `|| true`).

**Rủi ro.** Quét literal bỏ sót SQL dựng động; độ chính xác chưa đo; heuristic E7 mức tên cột chưa thử; tối ưu `changes[]`; chi phí đọc 35 file adapter + ~140 migration qua RTT SSH (ước tính, chưa đo).

**Hợp đồng thiếu/mâu thuẫn (báo chủ hợp đồng).** G1: `GetErdResponse` §3.1 không có chỗ cho `services[]` dù UI §3.1 yêu cầu. G2: `warnings` ở response (§3.1) so với `ErdModel.warnings` (UI §4.4): cần ghi rõ ai gộp. G3: `ErdChange`/`ErdColumnShape` chưa có số field trong hợp đồng.

**Câu hỏi mở.** Q3 CR (hiển thị `naming`): mặc định tắt. Q5 CR (`information_schema` dev). Số field của `ErdChange`/`TableAccess.source`: do chủ sở hữu CR-031 gán.

## 8. Tham chiếu

- `docs/crs/v7/code-intel-sources/CR-CV-031-sql-migration-to-erd.md`; `docs/crs/v7/README.md` mục 8 điểm 10, 11
- `specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-03, 04, 07, 12–15, 20, 23, 28, 29; §2.1, §3.1, §4 T3, §8.3), `CONTRACT-codeintel-ui-api.md` (§3.1, §4.4)
- `backend-go/services/infra-fleet-service/internal/adapter/{postgres,mysql}/repository.go`, `backend-go/services/*/migrations/*/*.up.sql`
- `backend-go/proto/buf.yaml`
