# BE-CV-SOL-038-contract-diff: `GetContractDiff`: so hợp đồng proto, kênh `wscompat`, route và migration giữa hai commit, phân loại phá vỡ tương thích

> **📋 Proposed.** Chưa triển khai, chưa chạy test/CLI. P2 (đợt 6). Quy tắc tĩnh `sql.missing-tenant-filter` là solution riêng: [`BE-CV-SOL-038-static-tenant-filter-rule`](./BE-CV-SOL-038-static-tenant-filter-rule.md). Khẳng định về code hiện có do người soạn đọc ngày 2026-10-06; chưa kiểm chứng ghi rõ.

**CR:** [CR-CV-038](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md) (§2.1–2.4, §2.6)
**Service:** `code-intel-service` (mới) · `proto/orca/codeintel/v1`
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-04**, **PQ-06** (`rule→kind`, `sql.drop-policy|sql.disable-rls→rls_removed`), **PQ-07** (`codeintel_contract_diff.proto`), **PQ-12**, **PQ-14**, **PQ-15** (`view=contractDiff`), **PQ-29** (`SourceRef`, `ContractWarning` riêng), **PQ-30** (`ContractChange.kind` gồm `sql-table|sql-column`; `compatibility`; `ruleId`+`details`; `files[]`+`evidence[]`); §2.1 dòng 16, §3.1 (`GetContractDiff`), §4.2 T3. [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) §3.1 (`contractDiff`), §4.5 (`ContractChange`, `ContractDiff`).
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Migration conventions: `DROP COLUMN`/`NOT NULL` không backfill là phá vỡ; up/down/up), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Input validation: `buf`), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§gRPC conventions: tương thích proto), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/proto/buf.yaml` (`lint: STANDARD`, `breaking: FILE`), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (`Register`, `RegisterStream`, `RegisterStreamChannel`, `RegisterBinaryStreamHandler`, `decodeArg[T]` dòng 267), `…/wscompat/channels_git.go` (dòng 50–76: `r.Register("git.commit", …)` với `type commitArgs struct{ WorktreeID string \`json:"worktree"\`; Message …; Paths … }` rồi `decodeArg[commitArgs](args, 0)`), `agent/src/relay/agent-git-handler.ts` (`show` trong whitelist), đầu ra solution `BE-CV-SOL-030` §B (`RepoSourceReader.ChangedFiles`, `FileDiff`, `ReadFile(SourceRef{commit})`), `BE-CV-SOL-031-sql-migration-parser` (`erd.Catalog.Apply` thuần), `BE-CV-SOL-032-proto-and-wscompat-contract-catalog` (`internal/domain/contract`, `internal/adapter/{protoschema,gocallgraph}`), `ls backend-go/proto/orca/*/v1/*.proto` theo 032 (18 tệp, 548 RPC), `ls backend-go/services/api-gateway/internal/adapter/httpgateway | grep routes` (nhiều `*_routes.go`). **Không đọc được**: cách đăng ký route HTTP của `api-gateway` (chưa khảo sát; theo CR chỉ 5 lần xuất hiện `HandleFunc/Handle`).

### Correction relative to CR-CV-038

| # | CR nói | Hợp đồng / thực tế | Xử lý |
|---|--------|--------------------|-------|
| C1 | `GetContractDiffRequest{worktree_id|repo_binding_id, base_ref?, kinds[], detail}` | PQ-04: `selector` | Hợp đồng |
| C2 | `kind` thiếu SQL | PQ-30: thêm `sql-table|sql-column` | Hợp đồng |
| C3 | Bên base đọc bằng `git.exec ["show", "<mergeBase>:<path>"]` trực tiếp | `BE-CV-SOL-030`: `RepoSourceReader.ReadFile(SourceRef{Kind:"commit"})` + `FileDiff`; cấm gọi `git.*` trực tiếp (S1) | Dùng cổng; đường dẫn hợp đồng phải thuộc allowlist đuôi (`.proto`, `.go`, `.sql`) |
| C4 | Hàm `parse(content) → catalog` thuần do 031/032 cung cấp | 031: `erd.Catalog.Apply` thuần (nhưng áp **lên chuỗi migration theo thứ tự**, không phải một tệp); 032: bộ phân tích `protoschema`/`gocallgraph` là **adapter** (tên hàm chốt ở solution đó, **chưa đọc**) | Port `ProtoSchemaParser`, `WsChannelExtractor`, `MigrationCatalogBuilder` trong `usecase`; ghi "chữ ký chốt ở TASK-038-01" |
| C5 | `ws.args-opaque` unknown | khớp `ContractWarning` của 032 | Dùng `compatibility:"unknown"` + `ruleId:"ws.args-opaque"` |
| C6 | `MigrationDiff.findings: Finding[]` | `Finding` thuộc `codeintel_findings.proto` (037) | Import một chiều; chỉ `sql.drop-policy`/`sql.disable-rls` ở đây (PQ-06) |
| C7 | `consumers` của RPC nối từ 032 | 032 có `RpcEdge` (client→server gRPC); `wscompat` consumer phía frontend **không có nguồn tự động** | `consumers` cho `proto-*` từ `RpcEdge`; `ws-channel` rỗng ở MVP (UI hiện "Chưa tìm thấy nơi dùng") |
| C8 | `touchedContracts[].breaking` | PQ-30 giữ `breaking` và thêm `compatibility` | Cấp cho 036 qua port `TouchedContractSource` |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_contract_diff.proto
backend-go/services/code-intel-service/
  internal/domain/contractdiff/
      contract_diff.go          # ContractDiff, ContractChange, ConsumerRef, MigrationDiff, SqlChange, TableImpact, Summary
      proto_diff_rules.go       # proto.* (bảng 2.C)
      ws_channel_diff_rules.go  # ws.*
      route_diff_rules.go       # route.* (chỉ khi catalog route có)
      migration_statement_rules.go  # sql.*, migration.*
      compatibility.go          # thứ tự breaking > risky > compatible > unknown, ruleId ổn định
  internal/usecase/get_contract_diff.go, contract_diff_ports.go, contract_file_selection.go
  internal/adapter/grpc/contract_diff_handler.go
  testdata/contract-diff/{proto/*, wscompat/*, migrations/*, golden/*.json}
```

### 2.B Proto `codeintel_contract_diff.proto`

Message (hợp đồng §2.1 dòng 16): `ContractDiff, ContractChange, ConsumerRef, MigrationDiff, SqlChange, TableImpact` và `GetContractDiffRequest{selector=1, base_ref=2, kinds=3, detail=4 (SUMMARY|FULL, UNSPECIFIED=0), if_none_match=5}`, `GetContractDiffResponse{diff=1, meta=2, index_freshness=3}`. `ContractChange{id, kind, name, service, change, compatibility, rule_id, details(map), files[], consumers[], evidence[SourceRef]}`; `SqlChange{table, op, column, compatibility, rule_id, dialect_only}`; `TableImpact{table, service, change, accessors[SymbolRef], columns_referenced[], cross_service, evidence[SourceRef]}`; `MigrationDiff{service, dialects[], files[], statements[], tables[], findings[Finding]}`. `kinds` request ∈ `proto|ws-channel|route|migration` (UI §3.1). Số field do chủ sở hữu CR gán theo thứ tự khai báo; ghi ở TASK-038-02. Chuỗi enum chữ thường (PQ-32).

### 2.C Cách lấy hai phiên bản và chọn tệp

1. `RepoSourceReader.ChangedFiles(repo, baseRef)` (hoặc danh sách từ overlay) ⇒ lọc theo **danh sách mẫu hợp đồng**: `backend-go/proto/orca/**/*.proto`, `backend-go/services/api-gateway/internal/adapter/wscompat/{channels_*.go,registry*.go}`, `backend-go/services/api-gateway/internal/adapter/httpgateway/*_routes.go`, `backend-go/services/*/migrations/{postgres,mysql}/*.sql`. Tệp khác bỏ. ≤ 300 tệp/lần, quá ⇒ `truncated`. `kinds[]` lọc thêm.
2. Mỗi tệp: head = `ReadFile(worktree)`, base = `ReadFile(SourceRef{commit: mergeBase})` hoặc `FileDiff` (cổng chốt). Tệp mới (base `not_found`) ⇒ `added`; tệp xoá ⇒ `removed`; đổi tên (`oldPath`) so nội dung cũ–mới. Tệp > 1 MiB/`Skipped` ⇒ `ContractChange{ruleId:"parse-error"|"skipped", compatibility:"unknown"}`, không làm hỏng phần còn lại.
3. Import `.proto`: dùng head cho tệp không đổi; tệp đổi đã nằm trong danh sách hai phía.
4. Tuân Git 2.25: không lệnh git ở backend; cổng 030 chịu trách nhiệm.

### 2.D Quy tắc phá vỡ (bám `buf breaking` loại `FILE`, CR §2.3)

Mỗi quy tắc có `ruleId` ổn định, bảng test table-driven:

- **Proto** (`proto.`): `service-removed`, `rpc-removed`, `rpc-type-changed`, `rpc-streaming-changed`, `message-removed` (còn được tham chiếu ở base), `field-removed` (hạ `risky` nếu số **và** tên nằm trong `reserved`), `field-number-changed`, `field-renamed`, `field-type-changed` (chỉ hoán đổi cùng wire theo bảng CR: `sint*`↔`sint*` cùng cỡ; `string`↔`bytes` ghi chú), `field-label-changed`, `enum-value-removed|number-changed`, `package-changed`, `go-package-changed`, `oneof-changed` ⇒ `breaking`; `*-added` ⇒ `compatible`; chỉ comment/option ⇒ `compatible` (chỉ liệt kê khi `detail=FULL`).
- **`wscompat`** (`ws.`): từ catalog 032: `channel-removed|kind-changed|arg-field-removed|arg-field-renamed|arg-field-type-changed|arg-position-changed` ⇒ `breaking`; `arg-field-added` ⇒ `compatible`, hạ `risky` nếu trường không có `omitempty`; `target-rpc-changed` ⇒ `risky`; `channel-added` ⇒ `compatible`; `args-opaque` ⇒ `unknown`.
- **Route** (`route.`): chỉ khi 032 cung cấp danh mục route đọc từ tệp (chưa biết, C1 của §1); xoá route/đổi method ⇒ `breaking`, bỏ `responseKeys` ⇒ `breaking`, đổi `errorKeys`/thêm `middleware` ⇒ `risky`.
- **Migration** (`sql.`, `migration.`): theo bảng CR (`sql.drop-table|drop-column|rename-*` ⇒ `breaking`; `alter-column-type` ⇒ `breaking` khi thu hẹp/đổi họ, `risky` khi nới; `add-column-not-null` (không `DEFAULT`) ⇒ `breaking`; `add-constraint` ⇒ `risky`; `drop-policy|disable-rls` ⇒ `breaking` + `Finding` `warning` (`rule` tương ứng, `kind:"rls_removed"`); `create-index`, `add-column-nullable-or-default`, `create-table` ⇒ `compatible`; `migration.modified-applied` ⇒ `risky`; `dialect-parity` (không áp `mcp-service`, chỉ Postgres) ⇒ `risky`; `number-conflict` ⇒ `breaking`; `missing-down` ⇒ `compatible`; `dialect-divergence` ⇒ `risky` kèm hai mức). `ContractChange.kind` cho SQL: `sql-table`/`sql-column`.
- `summary{breaking, risky, compatible, unknown}` đếm **trên toàn bộ trước khi cắt**.

### 2.E "Migration chạm bảng nào, code nào đọc bảng"

`TableImpact.accessors` = `Table.accessedBy` từ `BE-CV-SOL-031-erd-model-and-access-scan` (so khớp **định danh nguyên vẹn**, `columns_referenced` cho `drop/rename column` bằng cách tìm cột trong chuỗi SQL của accessor; cần `evidence` `path:line` của chuỗi SQL từ 031). `cross_service:true` khi service A đọc bảng của B (chỉ ghi, không finding ở MVP). Kết quả cấp `touchedContracts`/`touchedTables` và điểm `CONTRACT_BREAKING`/`MIGRATION_DESTRUCTIVE` cho `BE-CV-SOL-036-reading-order-and-risk`.

### 2.F Use case, giới hạn, cache

`GetContractDiff`: cờ ⇒ quyền `read` ⇒ `selector` ⇒ chọn tệp ⇒ đọc hai phía (song song có hạn mức) ⇒ parse hai phía ⇒ so ⇒ `ContractDiff`. Hạn mức: ≤ 300 tệp, ≤ 2 000 `ContractChange`, ≤ 500 `SqlChange`, ≤ 2 MiB (PQ-14). Cache `graph_snapshots(view="contractDiff", head_commit, params_hash = sha256(mergeBase|kinds|detail|ContentHash của các tệp)`) (PQ-15); cây bẩn: bộ nhớ TTL 30 s. Lỗi từng phần: một tệp parse lỗi ⇒ `ContractChange{ruleId:"parse-error"}`. Mã lỗi: `CODEINTEL_DISABLED`, `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_INVALID_PARAMS`, `CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_TIMEOUT`, `CODEINTEL_PATH_NOT_ALLOWED`. Không trả nguyên tệp; chỉ định danh/đoạn SQL ngắn trong `details`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| So sánh tĩnh nội dung hai điểm, chỉ tệp đã đổi | Chỉ mục đồ thị chỉ có một commit; chi phí tỷ lệ với diff |
| Tự cài quy tắc, bám `FILE` | `buf` không chắc có trên dev server; không cover `wscompat`/migration; CI vẫn là nguồn chân lý |
| `unknown` là giá trị hợp lệ | Không biết thì không khẳng định tương thích (H7) |
| `reserved` hạ `risky` | Đúng tinh thần xoá an toàn |
| Đọc qua cổng 030, không `git` trực tiếp | S1; SSH/GitLab: không phụ thuộc provider |
| Không `buf breaking` trong service | Phụ thuộc binary; không đồng nhất `wscompat` |

## 4. Lệch giữa CR và hợp đồng

| # | CR-CV-038 | Hợp đồng | Theo |
|---|-----------|----------|------|
| L1 | `kind` 9 giá trị | PQ-30 thêm `sql-table|sql-column` | Hợp đồng |
| L2 | request `repo_binding_id` | `selector` | Hợp đồng |
| L3 | `ContractDiff` trả `sources[]`, `indexFreshness` | §3.1: `{diff, sources[], index_freshness}` | Giống nhau |
| L4 | `ContractChange.location` (059) | PQ-30: `files[]`+`evidence[]` | Hợp đồng |
| L5 | `breakingReasons` | PQ-30: `ruleId`+`details` | Hợp đồng |
| L6 | Quyền không nói | §3.1: `read` | Hợp đồng |

## 5. Phụ thuộc chéo khu vực và thứ tự

| Cần | Từ | Dạng |
|-----|----|------|
| Cổng đọc hai phía | `BE-CV-SOL-030-repo-file-access-gateway` | cứng |
| Catalog proto/`wscompat`/`RpcEdge` | `BE-CV-SOL-032-proto-and-wscompat-contract-catalog` | cứng |
| Catalog SQL, `accessedBy` | `BE-CV-SOL-031-sql-migration-parser`, `BE-CV-SOL-031-erd-model-and-access-scan` | cứng |
| `Finding` | `BE-CV-SOL-037-structure-findings-and-dismissals` (proto) | cứng (proto) |
| `scope`, `mergeBase`, `changedFiles` | `BE-CV-SOL-036-change-overlay-pipeline` | mềm |
| Tiêu thụ | `BE-CV-SOL-036-reading-order-and-risk` (`CONTRACT_BREAKING`), kênh `BE-CV-SOL-040-codeintel-view-channels` (`contractDiff`), `FE-CV-SOL-059-contract-lens-and-findings` | sau |
| Phía agent | **không có** | — |

Thứ tự hợp đồng §7.2: 037 → **038**.

## 6. Kiểm thử

- **Bảng quy tắc**: table-driven cho mọi `ruleId`, hai chiều dây/nguồn; mỗi fixture bé. `go test ./services/code-intel-service/internal/domain/contractdiff/...`.
- **Migration hai dialect**: mỗi dòng bảng `sql.*` có test cho Postgres **và** MySQL (`ALTER COLUMN … TYPE` vs `MODIFY`), khai báo ma trận `dialect:[postgres, mysql]` cho phần kiểm chứng với DB thật (tuỳ chọn); phần domain là hàm thuần chạy trong mọi job.
- **Use case** với cổng đọc giả: tệp mới/xoá/đổi tên, hai phía đọc lỗi từng cái, hạn mức 300 tệp, `summary` đếm trước cắt, cache theo `ContentHash`.
- **Cô lập tenant**: hai tenant cùng `mergeBase`/`params_hash` không đọc chéo snapshot; cổng đọc nhận tenant của người gọi.
- **Bảo mật** (`BE-CV-SOL-072`): không đường nào đọc tệp ngoài danh sách mẫu; không trả nguyên tệp; `.env` bị cổng cấm.
- **Hợp đồng**: `buf lint/breaking`; không lệnh git trực tiếp (cổng giả ghi yêu cầu).
- Chưa chạy bất kỳ test nào.

## 7. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy hệ thống; mọi số của CR §1 là kết quả đọc/chạy thử ngày 2026-10-05.
- Cơ chế đăng ký route HTTP của `api-gateway` chưa khảo sát ⇒ `route.*` có thể chưa làm được ở MVP.
- `buf breaking FILE` còn quy tắc không liệt kê (`FILE_SAME_*`, `json_name`, option) ⇒ có thể lệch CI; CI là nguồn chân lý.
- `wscompat`: handler đọc tham số khác `decodeArg` ⇒ `unknown`; tỷ lệ chưa đo trên ~417 lời gọi `r.Register(` (đếm grep ở `channels_*.go`; CR nêu ~475, chênh do đếm khác nhau, chưa đối chiếu).
- Parser SQL nhẹ (hợp đồng 031) có thể bỏ sót cú pháp lạ ⇒ `other/unknown`.
- Chữ ký hàm parse của 031/032 chưa đọc kỹ; xem TASK-038-01.
- Phụ thuộc Part A/`direct-websocket`; `relay-ssh` ngoài MVP.

## 8. Câu hỏi mở

1. Đổi tên trường proto là `breaking` (bám CI) hay `risky`? Mặc định `breaking`.
2. `consumers` frontend của kênh `wscompat` (chỉ mục tên kênh → tệp frontend)?
3. `crossService` thành finding riêng sau này?
4. Đối chiếu DB thật (E14)? Mặc định không.

## 9. Tiêu chí chấp nhận

- [x] Mỗi quy tắc ở 2.D có cặp fixture (base, head) cho đúng `ruleId`/`compatibility`; xoá field có `reserved` ⇒ `risky`; chỉ đổi comment ⇒ không liệt kê (trừ `FULL`).
- [x] `ws.*`: fixture từ `channels_git.go` (`json:"worktree"`→`worktreeId`, xoá `paths`) ⇒ `ws.arg-field-renamed`, `ws.arg-field-removed` `breaking`; `map[string]any` ⇒ `unknown`.
- [x] `sql.*` hai dialect; `ADD COLUMN … NOT NULL` không default ⇒ `breaking`, có default ⇒ `compatible`; sửa migration cũ ⇒ `migration.modified-applied`; trùng số ⇒ `number-conflict`; thiếu một dialect ⇒ `dialect-parity` (trừ `mcp-service`).
- [x] `TableImpact.accessors` của `DROP COLUMN` đúng hàm và `path:line`.
- [x] Không có lời gọi `git.*` trực tiếp; không ký tự bị chặn (cổng giả).
- [x] Tệp mới ⇒ `added`, xoá ⇒ `removed`, đổi tên so đúng.
- [x] Lỗi một tệp không làm hỏng phần còn lại; `summary` đếm trước cắt; payload ≤ 2 MiB.
- [x] Chéo tenant bị từ chối; không trả nguyên tệp.
- [x] `buf lint/breaking` xanh; hai dialect xanh; không `max-lines` disable.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md`
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/{registry.go,channels_git.go}`, `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
- Series: `BE-CV-SOL-030`, `031-*`, `032`, `036-*`, `037`, `040-*`, `072`, `FE-CV-SOL-059`
