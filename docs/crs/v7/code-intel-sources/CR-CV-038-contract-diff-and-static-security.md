# CR-CV-038 — Khác biệt hợp đồng/schema giữa hai commit, phát hiện thay đổi phá vỡ tương thích và kiểm tra tĩnh thiếu lọc `tenant_id`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-038 |
| **Tên** | `GetContractDiff` (proto, kênh `wscompat`, route, migration/ERD) với quy tắc phá vỡ tương thích; liên kết "migration chạm bảng nào, code nào đọc bảng"; quy tắc tĩnh `sql.missing-tenant-filter` |
| **Loại** | Feature (backend `code-intel-service`) |
| **Priority** | ⚪ P2 (đợt 6) |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-031 (parse SQL migration → `ErdModel`, `Table.accessedBy`), CR-CV-032 (trích xuất proto, danh mục kênh `wscompat`, route), CR-CV-036 (`scope`, `changedFiles`, `mergeBase`), CR-CV-037 (`Finding`, `finding_key`, `finding_dismissals`, `ListFindings`), CR-CV-030 (đọc file + `git show`), CR-CV-011/013. Nguồn ngoài (research 09): **E1** migration SQL, **E2** proto, **E8** danh mục kênh `wscompat`, **E5** diff git; tuỳ chọn **E7** quan hệ logic ERD. **Không** cần E13 (`--pdg`): chỉ phân tích SQL tĩnh, không làm taint |
| **Mở khoá** | CR-CV-059 (lens Hợp đồng + Phát hiện), CR-CV-036 (`touchedContracts[].breaking`, điểm `CONTRACT_BREAKING`, `MIGRATION_DESTRUCTIVE`), CR-CV-040 (kênh `codeIntel.contractDiff`) |
| **Tác động** | `backend-go/services/code-intel-service` (mới): package so sánh hợp đồng, bộ quét SQL, RPC `GetContractDiff`; `backend-go/proto/orca/codeintel/v1` (message `ContractDiff`, `ContractChange`, `MigrationDiff`); dùng chung `Finding`/`ListFindings` của CR-CV-037 cho quy tắc `sql.*` |

---

## 1. Bối cảnh và vấn đề

Sau khi agent code, rủi ro lớn nhất không nằm ở thân hàm mà ở **hợp đồng** giữa các phần: proto giữa service, kênh `wscompat` giữa UI và gateway, schema DB giữa migration và mã đọc bảng. R4 và R7 ([08 §7](../../../research/view-code/08-views-and-review-models.md)) yêu cầu so hợp đồng giữa hai commit, tìm "migration chạm bảng nào, code nào đọc bảng đó" và truy vấn thiếu `tenant_id`. Đã đọc code/chạy thử ngày 2026-10-05:

**Proto.** `backend-go/proto/orca/*/v1/*.proto`: 18 tệp, ~354 KB, 17 package nhóm (`aiprovider … workflow`; `mcp` có hai tệp: `mcp.proto`, `external_server.proto`). `infrafleet.proto` có 94 RPC. `backend-go/proto/buf.yaml` đặt `lint: STANDARD` và `breaking: use: FILE`. CI đã có `buf breaking … --against 'origin/main'` cho từng service (ví dụ `.github/workflows/backend-go-mcp-service.yml:30–36`), nhưng chỉ chạy trong CI và chỉ tới nhánh `origin/main`, không hiển thị trên giao diện review sau mỗi lượt agent. Binary `buf` không đảm bảo có trên dev server; vì vậy cần so sánh **ngay trong `code-intel-service`**, tham chiếu bộ quy tắc FILE để kết quả không mâu thuẫn CI.

**Kênh `wscompat`.** `backend-go/services/api-gateway/internal/adapter/wscompat/`: `Registry.Register(channel, handler)`, `RegisterStream`, `RegisterStreamChannel`, `RegisterBinaryStreamHandler` (`registry.go:78–130`); 62 tệp `channels_*.go` (không tính test); ~475 lời gọi đăng ký với tên kênh dạng literal (đếm bằng grep, ví dụ `"git.commit"`, `"git.push"`). Trong thân mỗi handler có **struct tham số cục bộ** có tag JSON và `decodeArg[T](args, i)` (đã đọc `channels_git.go:60–75`: `type commitArgs struct{ WorktreeID string \`json:"worktree"\`; Message …; Paths … }`) và lời gọi gRPC client (`client.Commit(ctx, &gitgatewayv1.CommitRequest{…})`). Tức là có thể trích **hình dạng tham số** và **RPC đích** bằng AST Go, không cần heuristic chuỗi.

**Route HTTP.** `api-gateway/internal/adapter/httpgateway/*_routes.go` (đã thấy `annotation_routes.go`, `git_routes.go`, `agent_proxy_routes.go`…). Mẫu đăng ký route: **chưa khảo sát** (grep `HandleFunc("…`/`Handle("…` chỉ ra 5 lần xuất hiện trong thư mục, nên route phần lớn đăng ký theo cách khác). GitNexus có nút `Route` (92 trên Orca) với `path`, `method`, `responseKeys`, `errorKeys`, `middleware`, nhưng **chỉ mục chỉ phản ánh một commit**, không có trạng thái commit cơ sở.

**Migration.** 588 tệp `.sql` ở 17 service; `0001_init.up.sql`… đánh số, mỗi `*.up.sql` có `*.down.sql`; thư mục `postgres/` và `mysql/` **trùng tên tệp tuyệt đối** ở 16/16 service có cả hai (đã so `diff <(ls …)`: 0 khác biệt); `mcp-service` chỉ có `postgres/` (151 `up` Postgres vs 143 `up` MySQL, chênh đúng bằng `mcp-service`). Đánh số có khoảng trống (infra-fleet nhảy `0006` → `0013`), không phải lỗi.

**Isolation tenant: bằng chứng không đồng nhất.**
- Postgres: bảng có `ENABLE ROW LEVEL SECURITY` + policy `tenant_id = current_setting('app.tenant_id', true)::uuid` (ví dụ `task-service/migrations/postgres/0001_init.up.sql:33–35`). **Chỉ `mcp-service` thật sự đặt biến** (`mcp-service/internal/adapter/postgres/tenant_tx.go`: `withTenantTx` gọi `SELECT set_config('app.tenant_id', $1, true)`). Các service khác ghi rõ trong comment rằng `app.tenant_id` **không bao giờ được đặt**: `task-service/migrations/mysql/0001_init.up.sql:46–52`, `task-service/internal/adapter/postgres/share_link.go:17–20`, `annotation-service/internal/adapter/mysql/repository.go:30`, `scm-integration-service/internal/adapter/mysql/rate_limit_cache.go:37` ("RLS chưa từng kích hoạt").
- MySQL: không có RLS (`dbcapability.Capabilities.SupportsRLS=false`); bảo vệ duy nhất là mệnh đề `WHERE tenant_id = ?` trong từng truy vấn (ví dụ `task-service/internal/adapter/mysql/edges.go`).
Do đó **với đa số service, thiếu `tenant_id` trong WHERE là rò rỉ chéo tenant thật**, không được RLS đỡ; riêng `mcp-service` (Postgres) có lớp phòng thủ thứ hai.

**Đo sơ bộ (regex ngoài, chưa phải triển khai, chưa kiểm tay)** trên 171 tệp `adapter/{postgres,mysql}/*.go` (≈ 1,2 MB, trừ `_test.go`): trong 1 214 chuỗi SQL dạng raw-string, 466 (38 %) không chứa chữ `tenant_id`; thu hẹp xuống bảng **có cột `tenant_id`** (theo migration Postgres) còn 116; bỏ `outbox_events` (22; relay cố ý xuyên tenant — `withRelayTx` ở `mcp-service` và `common/outbox`) còn ~94. Chia tầng thêm: 77 truy vấn có "anh em" cùng bảng ở cùng tệp **có** lọc `tenant_id`, 11 truy vấn theo khoá toàn cục (`*_hash`/`token`/`code`), 6 còn lại là ứng viên mạnh nhất, tập trung ở `project-service/internal/adapter/{postgres,mysql}/folder_workspace_repository.go` (`UPDATE … WHERE id = $1`, `DELETE … WHERE id = $1`; đã đọc, hàm `Update`/`Delete` chỉ nhận `id`, chưa kiểm tra use case gọi có chặn tenant chưa) và `scm-integration-service/…/webhook_delivery_repository.go` (`SELECT 1 … WHERE provider = $1 AND delivery_id …`, có thể cố ý toàn cục). Chuỗi SQL còn bị nối bằng `+ folderWorkspaceColumns` nên regex bỏ sót; cần AST. Con số trên chỉ dùng để định cỡ độ ồn của heuristic, **không** phải kết quả bảo mật.

Vấn đề cần giải: (a) hai phiên bản hợp đồng cần so sánh mà không thể lấy từ chỉ mục đồ thị, (b) định nghĩa chính xác "phá vỡ", (c) một quy tắc bảo mật ồn ào phải có tầng độ tin cậy, cách tắt và tỷ lệ báo nhầm đo được.

## 2. Giải pháp đề xuất

### 2.1 RPC và mô hình (mới)

`GetContractDiff(GetContractDiffRequest{worktree_id|repo_binding_id, base_ref?, kinds[] ∈ {proto, ws-channel, route, migration}, detail: SUMMARY|FULL}) returns (GetContractDiffResponse{ContractDiff, sources[], indexFreshness})`. `scope` giống CR-CV-036 (mặc định: merge-base → worktree, O7).

```
ContractDiff { scope, summary {breaking, risky, compatible, unknown: int}, changes: ContractChange[], migrations: MigrationDiff[],
               truncated, totalCount, limits }
ContractChange { id, kind:"proto-service|proto-rpc|proto-message|proto-field|proto-enum|ws-channel|ws-channel-arg|route|route-field",
                 name, change:"added|removed|modified", compatibility:"breaking|risky|compatible|unknown",
                 ruleId, details: map, files: string[], consumers: ConsumerRef[], evidence: SourceRef[] }
ConsumerRef  { kind:"rpc-client|ws-channel|route", service?, symbol?: SymbolRef, path? }
MigrationDiff { service, dialects:["postgres","mysql"], files[], statements: SqlChange[],
                tables: TableImpact[], findings: Finding[] }
SqlChange    { table, op:"create-table|drop-table|add-column|drop-column|alter-column-type|set-not-null|rename-table|rename-column|
                          add-index|drop-index|add-constraint|drop-constraint|drop-policy|disable-rls|other",
               column?, compatibility, ruleId, dialectOnly? }
TableImpact  { table, service, change, accessors: SymbolRef[], columnsReferenced: string[] /*cột bị đổi mà mã có nhắc*/ , evidence }
```

### 2.2 Cách lấy hai phiên bản (không dùng chỉ mục đồ thị)

Chỉ mục GitNexus/CodeGraph ở một commit nên **không thể** cho trạng thái `base`. Vì vậy toàn bộ so sánh là **phân tích tĩnh nội dung tệp tại hai điểm**, chỉ trên tệp đã đổi (từ `changedFiles` của CR-CV-036; `git.branchCompare` + `git.status`):
- Bên **head**: `fs.read` tệp trong worktree (gồm cả thay đổi chưa commit).
- Bên **base**: `git.exec ["show", "<mergeBase>:<path>"]` (`show` có trong whitelist Part A; không chứa ký tự bị chặn; kích thước tệp nhỏ). Không dùng `ls-tree`/`diff-tree` (ngoài whitelist); danh sách tệp lấy từ `branchCompare.entries` (có `status`, `oldPath` cho đổi tên).
- Bộ phân tích (proto, kênh, SQL) phải là **hàm thuần** `parse(content) → catalog`, do CR-CV-031/032 cung cấp; CR này gọi hai lần rồi so sánh (yêu cầu ghi ở "Điều chỉnh hợp đồng").
- Với proto: phân giải `import` giữa các tệp `.proto` dùng bản head cho tệp không đổi (giả định tệp được import không đổi; nếu đổi thì cũng nằm trong `changedFiles` và được đọc ở hai phía).
- Chỉ đọc tệp thuộc danh sách mẫu đường dẫn hợp đồng: `backend-go/proto/orca/**/*.proto`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_*.go` (+ `registry.go`), `backend-go/services/api-gateway/internal/adapter/httpgateway/*_routes.go`, `backend-go/services/*/migrations/{postgres,mysql}/*.sql`. Tệp khác bị bỏ. Tối đa 300 tệp/lần; vượt thì `truncated`.
- Không lệnh git nào mới ngoài `show`; tuân git ≥ 2.25 (`guides/reference/git-compatibility.md`).

### 2.3 Quy tắc phá vỡ tương thích

Mỗi quy tắc có `ruleId` ổn định (để dismiss/giải thích). Hai chiều: **dây** (wire: nhị phân/JSON giữa client/server cũ-mới) và **nguồn** (source: mã sinh Go/TS biên dịch). Theo `buf breaking` loại `FILE` mà repo dùng, ta coi vi phạm nguồn là `breaking` luôn.

**Proto** (`ruleId` tiền tố `proto.`):

| Quy tắc | Điều kiện | Mức |
|---|---|---|
| `proto.service-removed` | service biến mất | breaking |
| `proto.rpc-removed` | RPC bị xoá | breaking |
| `proto.rpc-type-changed` | kiểu request/response đổi | breaking |
| `proto.rpc-streaming-changed` | đổi unary↔stream hoặc hướng stream | breaking |
| `proto.message-removed` | message bị xoá (và còn được tham chiếu ở bản base) | breaking |
| `proto.field-removed` | field bị xoá; **hạ xuống risky** nếu số và tên đã nằm trong `reserved` | breaking / risky |
| `proto.field-number-changed` | cùng tên, khác số | breaking |
| `proto.field-renamed` | cùng số, khác tên (đổi tên trường sinh mã và JSON) | breaking (nguồn/JSON) |
| `proto.field-type-changed` | kiểu đổi (trừ nhóm hoán đổi cùng wire: `int32/uint32/int64/uint64/bool` KHÔNG hoán đổi cho nhau; chỉ `sint*` ↔ `sint*` cùng cỡ; `string`↔`bytes` hoán đổi được nhưng nguồn đổi) | breaking (trừ ghi chú) |
| `proto.field-label-changed` | singular ↔ `repeated`, hoặc `optional` bật/tắt | breaking |
| `proto.enum-value-removed` / `proto.enum-value-number-changed` | giá trị enum xoá/đổi số | breaking |
| `proto.package-changed`, `proto.go-package-changed` | đổi `package` hoặc `option go_package` | breaking |
| `proto.oneof-changed` | field chuyển vào/ra khỏi `oneof` | breaking |
| `proto.rpc-added`, `proto.message-added`, `proto.field-added` (số mới), `proto.enum-value-added` | thêm | compatible |
| `proto.comment-or-option-only` | chỉ đổi comment/option không ảnh hưởng | compatible (không liệt kê trừ `detail=FULL`) |

`consumers` của thay đổi RPC lấy từ phép nối client↔server của CR-CV-032 (`grpcclient` các service gọi RPC đó). Nếu CR-CV-032 chưa nối được → `consumers:[]` và `compatibility` giữ nguyên.

**Kênh `wscompat`** (`ws.`): trích bằng AST Go: tên kênh (literal đầu của `Register*`), kiểu (`request|stream|binary-stream`), tham số (mọi `decodeArg[T](args, i)` → vị trí `i`, trường JSON của `T` gồm tên tag, kiểu Go), RPC đích (lời gọi `X.Method(ctx, &pkg.Req{…})` → cặp `(service alias, RPC)` nối với proto của CR-CV-032).

| Quy tắc | Điều kiện | Mức |
|---|---|---|
| `ws.channel-removed` | kênh biến mất (nếu có kênh mới cùng RPC đích → đề xuất "đổi tên", vẫn breaking cho UI cũ) | breaking |
| `ws.channel-kind-changed` | `request` ↔ `stream` | breaking |
| `ws.arg-field-removed` / `ws.arg-field-renamed` | trường JSON của tham số bị xoá/đổi tag | breaking |
| `ws.arg-field-type-changed` | kiểu Go của trường đổi (ví dụ `string`→`[]string`) | breaking |
| `ws.arg-position-changed` | vị trí tham số đổi hoặc bỏ tham số | breaking |
| `ws.arg-field-added` | thêm trường mới | compatible (nếu handler dùng giá trị mặc định khi thiếu — **không biết** → `risky` nếu trường không có `omitempty`) |
| `ws.target-rpc-changed` | RPC đích đổi | risky |
| `ws.channel-added` | thêm kênh | compatible |
| `ws.args-opaque` | tham số không đọc qua `decodeArg[T]` (ví dụ `json.Unmarshal` thủ công, `map[string]any`) → **không so được** | unknown |

Người dùng của kênh là frontend (hai render target) — liên kết tới nơi gọi (`window.api…`, `web-preload-api`) **chưa có** nguồn tự động; `consumers` rỗng ở MVP (xem 6, 7).

**Route HTTP** (`route.`): chỉ khi CR-CV-032 cung cấp danh mục route đọc được từ tệp (không từ chỉ mục): xoá route hoặc đổi method → breaking; bỏ phần tử `responseKeys` → breaking, thêm → compatible; đổi `errorKeys`, thêm `middleware` (ví dụ bắt xác thực) → risky.

**Migration** (`sql.` — phân loại từng câu lệnh trong `*.up.sql` mới/đổi; cả hai dialect, câu lệnh được chuẩn hoá trước khi so):

| Quy tắc | Điều kiện | Mức |
|---|---|---|
| `sql.drop-table` | `DROP TABLE` | breaking (mã còn đọc bảng → thêm finding) |
| `sql.drop-column` | `DROP COLUMN` | breaking |
| `sql.rename-table` / `sql.rename-column` | `RENAME` | breaking |
| `sql.alter-column-type` | Postgres: `ALTER COLUMN … TYPE`; MySQL: `MODIFY`/`CHANGE` đổi kiểu | breaking nếu thu hẹp/đổi họ kiểu, risky nếu nới rộng |
| `sql.add-column-not-null` | `ADD COLUMN … NOT NULL` **không** `DEFAULT` | breaking (lỗi nếu bảng đã có dòng) |
| `sql.add-constraint` | `ADD UNIQUE/CHECK/FOREIGN KEY`/PK | risky (có thể fail theo dữ liệu hiện có) |
| `sql.drop-policy` / `sql.disable-rls` | bỏ chính sách RLS hoặc tắt RLS | breaking + gắn finding bảo mật mức `warning` |
| `sql.create-index` | `CREATE INDEX` trên bảng có sẵn | compatible (ghi chú khoá bảng; `CREATE INDEX CONCURRENTLY` không chạy trong giao dịch — không phân tích) |
| `sql.add-column-nullable-or-default`, `sql.create-table` | thêm cột có thể NULL/có default; bảng mới | compatible |
| `migration.modified-applied` | **sửa nội dung** migration đã có ở base (không phải tệp mới) | risky (migration đã chạy không chạy lại) |
| `migration.dialect-parity` | tệp `up` mới chỉ có ở một dialect (trừ service chỉ Postgres như `mcp-service`) | risky |
| `migration.number-conflict` | trùng số thứ tự trong cùng thư mục dialect | breaking (golang-migrate từ chối) |
| `migration.missing-down` | có `up` mà không có `down` | compatible (info) |

Dialect khác biệt được chuẩn hoá: câu lệnh MySQL và Postgres cho cùng migration được so ở mức ý nghĩa; nếu hai dialect cho hai kết quả khác nhau, báo `sql.dialect-divergence` (risky) kèm hai mức.

### 2.4 "Migration chạm bảng nào, code nào đọc bảng đó"

1. Với mỗi migration mới/đổi (đã phân loại ở 2.3), lấy tập `(service, table)` bị tác động (từ `ErdModel` hai phía hoặc câu lệnh).
2. `TableImpact.accessors` = `Table.accessedBy` (CR-CV-031: quét tên bảng `schema.table`/`table` trong chuỗi SQL của adapter). Với `drop/rename column`, bổ sung `columnsReferenced`: chuỗi SQL của accessor có nhắc cột đó (so khớp định danh nguyên vẹn, không chuỗi con). Yêu cầu CR-CV-031 giữ `evidence` = `path:line` của chuỗi SQL, không chỉ tên hàm.
3. Hai lớp liên kết: **trong service** (bình thường) và **ngoài service** (service A đọc bảng của service B — vi phạm sở hữu schema; chỉ ghi là `crossService:true`, không tạo finding riêng ở MVP).
4. Kết quả cho view "bảng đổi ↔ hàm đọc" (CR-CV-057/059) và vào `touchedTables[].accessors` (CR-CV-036).

### 2.5 Kiểm tra tĩnh: truy vấn SQL thiếu lọc `tenant_id` (`sql.missing-tenant-filter`)

Loại: `Finding` (CR-CV-037), `rule:"sql.missing-tenant-filter"`.

**Trích xuất** (AST Go `go/parser`, không regex): trong `internal/adapter/{postgres,mysql}/**.go` (loại `_test.go`), lấy chuỗi hằng/literal và phép nối `+` giữa các literal, đối số đầu của `fmt.Sprintf`; theo dõi gán `const`/`var` trong cùng tệp. Hàm bao ngoài (`enclosingFunc`) cho mỗi chuỗi. Chỉ xét chuỗi bắt đầu bằng `SELECT|UPDATE|DELETE|INSERT|WITH` (sau comment `--`). Mã dựng SQL động bằng `strings.Builder`/`WriteString` **không** phân tích được → bỏ (không báo). Chuẩn hoá: viết thường, gộp khoảng trắng, `$n` và `?` → `?`.

**Bảng và cột**: danh sách bảng của service lấy từ `ErdModel` head của đúng dialect (CR-CV-031); `tenantTables(service)` = bảng có cột `tenant_id`. Bảng trích từ `FROM|JOIN|UPDATE|DELETE FROM|INSERT INTO` (bỏ tiền tố schema Postgres).

**Điều kiện báo**:
- `SELECT/UPDATE/DELETE` (kể cả trong CTE/subquery) có ít nhất một bảng ∈ `tenantTables` **và** toàn bộ văn bản truy vấn không có định danh `tenant_id` (ranh giới định danh), → ứng viên.
- `INSERT INTO <bảng tenant>` mà danh sách cột không có `tenant_id` → `sql.insert-missing-tenant` (cùng cơ chế, đo riêng; chưa đo tỷ lệ).

**Tầng độ tin cậy** (giảm báo nhầm; hiển thị rõ trong `Finding.confidence`):

| Mức | Điều kiện | `severity` |
|---|---|---|
| `high` | không thuộc các ngoại lệ dưới; MySQL hoặc service Postgres **không** đặt `app.tenant_id` | `warning` |
| `medium` | có truy vấn **cùng bảng** có `tenant_id` trong **cùng hàm bao ngoài** (có thể là bước kiểm tra tenant trước đó); hoặc service Postgres có `set_config('app.tenant_id'…)` (hiện chỉ `mcp-service`: RLS đỡ lại) | `info` |
| `low` | lọc theo khoá toàn cục duy nhất (`*_hash`, `token`, `code`, `delivery_id` — danh sách cấu hình) | `info` |
| bỏ qua | bảng trong allowlist cố định `outbox_events`, `processed_events`; câu lệnh trong hàm `*Outbox*`/`relay` dùng `withRelayTx`; truy vấn có chú thích cho phép (xem dưới) | không có |

Phát hiện service có RLS hiệu lực: tìm literal `set_config('app.tenant_id'` hoặc `SET LOCAL app.tenant_id` trong `internal/adapter/postgres`. Đây là heuristic nên chú thích "RLS có thể đang hiệu lực" (không khẳng định).

**`finding_key`** (CR-CV-037): `sql.missing-tenant-filter:<16 hex sha256("<service>::<dialect>::<đường dẫn tệp>::<enclosingFunc>::<SQL chuẩn hoá>")>` — ổn định khi dòng dịch chuyển, đổi khi SQL đổi.

**Cách tắt/giảm ồn** (từ ít sang nhiều phạm vi):
1. **Dismiss từng finding** bằng `DismissFinding` (`finding_dismissals`, CR-CV-037) kèm lý do bắt buộc.
2. **Chú thích trong mã**, ngay dòng trước câu lệnh hoặc dòng trước hàm: `// codeintel:allow sql.missing-tenant-filter <lý do ≥ 10 ký tự>`. Bộ quét đọc comment bằng AST; chú thích thiếu lý do bị bỏ qua và báo `info`. Chú thích là mã trong repo nên đi qua review như mọi thay đổi khác.
3. **Tắt cả quy tắc** theo repo: khoá `findings.disabledRules` trong `c4.yaml` (CR-CV-033, bảng `c4_overrides`) — **chờ chủ CR-CV-033 xác nhận** (xem 7); mặc định bật.
4. Không có công tắc theo tenant ở MVP.

**Phạm vi chạy**: `scope=CHANGED` (mặc định khi gọi từ overlay): chỉ tệp adapter đã đổi — rẻ (vài tệp). `scope=ALL`: quét 171 tệp (~1,2 MB) qua `fs.*` như một job nền (tuân hạn mức CR-CV-013 và CR-CV-021), cache theo `(repo, commit, "sql-tenant")`. Phát hiện **mới do thay đổi** (`introduced`): câu lệnh có trong head nhưng không có trong base (so khoá `finding_key`; base lấy bằng `git show` cho các tệp đã đổi).

**Tỷ lệ báo nhầm — mục tiêu và cách đo**: chưa có số đo đúng. Ước lượng thô (xem mục 1) cho thấy sau tầng chia, tập `high` rất nhỏ (~6 câu hai dialect) nhưng chưa biết bao nhiêu là rò rỉ thật. Trước khi bật mặc định cần **bộ vàng**: 100 truy vấn lấy mẫu có chủ ý (50 từ `high`+`medium`, 50 từ nhóm đã lọc), người phân loại thật/giả. Tiêu chí ra mắt: precision của `high` ≥ 60 % (ước lượng ban đầu, cần chốt), toàn bộ `info` không hiện mặc định trên UI (bộ lọc "Hiện độ tin cậy thấp"). Nếu không đạt, giữ quy tắc ở trạng thái "thử nghiệm" (ẩn khỏi danh sách mặc định, bật bằng bộ lọc).

### 2.6 Hạn mức, cache, lỗi

- ≤ 300 tệp hợp đồng, ≤ 2 000 `ContractChange`, ≤ 500 `SqlChange`, payload ≤ 2 MiB; vượt → `truncated` + `totalCount`. `summary` đếm trên toàn bộ trước khi cắt.
- Cache `(repo, mergeBase, headOid, "contractDiff", params_hash)`; worktree bẩn dùng TTL ngắn (như CR-CV-036).
- Lỗi từng phần: một tệp parse lỗi → `ContractChange{kind:"…", compatibility:"unknown", ruleId:"parse-error"}` kèm `path`, không làm hỏng phần còn lại; `git show` thất bại với tệp (ví dụ tệp mới nên không có ở base) được hiểu là `added`, không phải lỗi.
- Quyền, tenant, flag: như các view khác (CR-CV-013, O8).
- Đưa `summary.breaking/risky` vào `touchedContracts[].breaking` và điểm `CONTRACT_BREAKING`/`MIGRATION_DESTRUCTIVE` của CR-CV-036; CR-CV-036 chạy được trước, CR này chỉ làm giàu.

## 3. Quyết định thiết kế

1. **So sánh tĩnh trên nội dung tệp tại hai điểm** thay vì dùng chỉ mục (không có trạng thái base), và chỉ trên tệp đã đổi để giữ chi phí tỷ lệ với diff.
2. **Tự cài đặt quy tắc thay vì gọi `buf breaking`**: `buf` không chắc có trên dev server, không cover kênh `wscompat`/migration; vẫn **bám loại `FILE`** để không mâu thuẫn CI. Khi có mâu thuẫn, CI là nguồn chân lý và đây là cảnh báo sớm.
3. **AST Go cho `wscompat`/SQL**: handler có struct tham số cục bộ và lời gọi gRPC rõ ràng → trích chính xác, không dò chuỗi; mã động bị bỏ thay vì đoán.
4. **`unknown` là giá trị hợp lệ**: không biết thì không khẳng định tương thích.
5. **Quy tắc tenant theo tầng độ tin cậy, mặc định không làm ồn**: vì bằng chứng RLS không đồng nhất (mục 1), một mức cố định sẽ hoặc bỏ sót hoặc làm ngập.
6. **Không coi RLS là đủ**: đa số service không bao giờ đặt `app.tenant_id` (comment của chính repo), nên thiếu `WHERE tenant_id` là lỗ thật.
7. **Tắt bằng dismiss/chú thích có lý do**, không có công tắc im lặng: lý do bắt buộc để review sau này còn ngữ cảnh.
8. **Không làm taint/PDG** (E13): ngoài phạm vi, tránh phụ thuộc `analyze --pdg`.
9. **Đọc `git show`, không checkout**: không chạm cây làm việc của người dùng; phù hợp SSH/relay (mọi thứ qua agent).

## 4. Tiêu chí chấp nhận

- [ ] Fixture proto: với từng quy tắc ở bảng 2.3 (breaking và compatible) có một cặp (base, head) cho đúng `ruleId`/`compatibility`; xoá field mà có `reserved` hạ xuống `risky`; chỉ đổi comment không sinh thay đổi (trừ `detail=FULL`).
- [ ] Kênh `wscompat`: fixture từ `channels_git.go` (đổi tag JSON `worktree`→`worktreeId`, xoá `paths`) cho `ws.arg-field-renamed`, `ws.arg-field-removed` breaking; thêm kênh mới compatible; tham số đọc kiểu `map[string]any` cho `ws.args-opaque`/`unknown`.
- [ ] Migration: mỗi dòng bảng `sql.*` có test hai dialect (Postgres và MySQL) cho cùng ý nghĩa; `ADD COLUMN … NOT NULL` không default → breaking; có default → compatible; sửa migration đã có ở base → `migration.modified-applied`; hai tệp cùng số → `migration.number-conflict`; thiếu một dialect → `migration.dialect-parity` (không áp cho `mcp-service`).
- [ ] `TableImpact.accessors` của một migration `DROP COLUMN` trả đúng các hàm adapter nhắc cột đó (so khớp định danh), kèm `path:line`.
- [ ] `GetContractDiff` chỉ gọi `git show <mergeBase>:<path>`/`fs.read`; không dùng `ls-tree`, `diff-tree`, `merge-base` trực tiếp; không có đối số chứa ký tự bị chặn (kiểm bằng agent giả ghi nhận lời gọi).
- [ ] Tệp tạo mới (không có ở base) cho `added`; tệp xoá cho `removed`; đổi tên (`oldPath`) so nội dung cũ–mới đúng.
- [ ] `sql.missing-tenant-filter`: bộ vàng chứa ≥ 1 ca mỗi tầng; `outbox_events` không bao giờ bị báo; truy vấn có `// codeintel:allow … <lý do>` bị bỏ; chú thích thiếu lý do không có tác dụng; `INSERT` thiếu cột `tenant_id` được báo `sql.insert-missing-tenant`.
- [ ] Trên Orca: `folder_workspace_repository.go` (`Update`/`Delete`) xuất hiện với `confidence:"high"`; `project-service` còn lại và `mcp-service` (Postgres, `set_config` có mặt) hạ xuống `medium`; truy vấn theo `token_hash`/`code_hash` là `low`.
- [ ] Mỗi finding `sql.*` có `finding_key` ổn định: dịch chuyển dòng không đổi khoá; đổi SQL đổi khoá; dismiss ẩn finding đó ở `ListFindings` (CR-CV-037).
- [ ] Số báo nhầm của `high` trên bộ vàng được ghi lại; nếu precision < ngưỡng đã chốt thì quy tắc ở trạng thái "thử nghiệm".
- [ ] Hạn mức và `truncated/totalCount` đúng; lỗi parse một tệp không làm hỏng phần còn lại; `summary` đếm trước khi cắt.
- [ ] Cô lập tenant: gọi chéo tenant bị từ chối; không có mã nguồn ngoài đoạn SQL/định danh trong payload (không trả nguyên tệp).
- [ ] `buf lint`/`buf breaking` pass cho proto `orca.codeintel.v1`; không khai báo RPC chưa có message.

## 5. Kiểm thử

- **Bảng quy tắc**: bảng-test (table-driven) cho mọi `ruleId` ở 2.3, hai chiều dây/nguồn; mỗi proto fixture bé, đọc được.
- **AST Go**: `decodeArg[T]`, struct cục bộ, nhiều lời gọi gRPC, `Sprintf`, chuỗi nối `+` (đúng trường hợp `… RETURNING ` + `folderWorkspaceColumns`), const gián tiếp, comment `codeintel:allow` hợp lệ/không hợp lệ.
- **SQL**: parser nhẹ cho 2 dialect (Postgres `schema.table`, MySQL không tiền tố); CTE, subquery, `RETURNING`, `FOR UPDATE`, `ON CONFLICT`, `INSERT … SELECT`.
- **Bộ vàng bảo mật**: 100 truy vấn có nhãn tay (xem 2.5), chạy thường kỳ ở CR-CV-070; ghi precision/recall mỗi phiên bản quy tắc.
- **Git thật**: repo tạm có base và head (proto thêm/xoá field, migration mới/sửa, đổi tên tệp), chạy với git 2.25 và bản mới; so kết quả với `git diff`.
- **Hai dialect** cho bảng cache/finding (O1).
- **Bảo mật** (CR-CV-072): không đường nào đọc tệp ngoài danh sách mẫu hợp đồng; không trả nguyên tệp; cô lập tenant.
- **Hiệu năng** (CR-CV-071): `GetContractDiff` với diff điển hình < 5 s; `scope=ALL` SQL quét < 60 s (ngân sách tạm, cần đo).

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa chạy hệ thống.** Mọi số ở mục 1 là kết quả đọc/chạy thử ngày 2026-10-05. Phép đo SQL `tenant_id` là **regex thô của người soạn**, bỏ sót chuỗi nối `+` và SQL động, phân loại tầng bằng xấp xỉ cùng-tệp thay vì cùng-hàm; chỉ dùng để định cỡ.
- Chưa kiểm chứng rằng `folder_workspaces` Update/Delete là lỗ hổng thật (use case có thể kiểm tenant trước đó). Không được coi là phát hiện bảo mật khi chưa xác minh.
- Cơ chế đăng ký route HTTP của `api-gateway` chưa khảo sát; `route.*` phụ thuộc CR-CV-032. Không biết có tách được từ tệp tĩnh không.
- `buf breaking` `FILE` còn các quy tắc không liệt kê ở đây (ví dụ `FILE_SAME_*`, thay đổi `json_name`, `reserved`, option); phiên bản đầy đủ nên đối chiếu từng quy tắc trong tài liệu `buf` trước khi chốt. Điểm khác biệt có thể làm kết quả lệch CI.
- Kênh `wscompat`: handler kiểu `args []json.RawMessage` có thể đọc phần tử bằng cách khác `decodeArg`; tỷ lệ `unknown` chưa đo trên 475 kênh. Người tiêu thụ phía frontend chưa có nguồn nối tự động.
- Dịch chuyển giữa dialect: câu lệnh MySQL đặc thù (`MODIFY`, `CHANGE`, `ENGINE=`) và Postgres (`ALTER COLUMN TYPE`, `CREATE INDEX CONCURRENTLY`, extension) cần bộ parser nhẹ; parser đầy đủ không có; câu lệnh lạ → `other`/`unknown`.
- Ngưỡng precision 60 % là giả định chưa có dữ liệu.
- `git show` cho tệp lớn: stdout nằm trong bộ nhớ agent không giới hạn; tệp hợp đồng nhỏ nên chấp nhận được, vẫn cần chặn kích thước (≤ 1 MiB/tệp).
- Phụ thuộc Part A/`direct-websocket`; không chạy ở `relay-ssh` MVP (CR-CV-006).
- Phát hiện RLS hiệu lực (`set_config`) là heuristic; service dùng wrapper khác có thể bị coi sai.
- Hai tiền tố nhóm migration `postgres`/`mysql` giả định cấu trúc hiện tại; service mới có thể khác (cần đọc lại trước khi triển khai).

## 7. Câu hỏi mở

1. Chấp nhận mức `breaking` cho đổi tên trường proto (theo `FILE`), hay chỉ `risky` (vì wire không đổi)? Đề xuất bám CI (`breaking`).
2. Ở đâu đặt `findings.disabledRules`: trong `c4.yaml` (CR-CV-033), một tệp cấu hình riêng trong repo, hay cấu hình tenant? Có bật/tắt từng quy tắc theo repo không?
3. Danh sách khoá toàn cục cho tầng `low` (`*_hash`, `token`, `delivery_id`…) do ai duyệt; có cho cấu hình theo repo?
4. Có muốn kiểm tra thêm: truy vấn ở **usecase/gRPC server** không truyền `tenant` (không chỉ SQL)? Hiện ngoài phạm vi.
5. `crossService` (service A đọc bảng của service B) có nên thành finding riêng (vi phạm sở hữu schema) ở phiên bản sau?
6. Frontend consumers của kênh `wscompat`: có chấp nhận một chỉ mục tên kênh → tệp frontend (từ GitNexus `FETCHES`/tìm literal) để liệt kê "ai bị ảnh hưởng"?
7. Chấp nhận quét `scope=ALL` như job nền ~171 tệp, hay chỉ `scope=CHANGED`?
8. Có cần đối chiếu migration với DB thật (`information_schema`, E14) để phát hiện lệch — mặc định không.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md`; `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§5, §7 R4, R7), `09-external-inputs-required.md` (E1, E2, E5, E7, E8, E13, E14), `05-graph-schemas.md` (§2.6 RouteMap)
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/backend-go/proto/orca/**/v1/*.proto`, `/opt/repos/orca/.github/workflows/backend-go-mcp-service.yml` (dòng 27–36)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`, `…/channels_git.go` (dòng 57–76), `…/channels_*.go`; `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/httpgateway/*_routes.go`
- `/opt/repos/orca/backend-go/services/*/migrations/{postgres,mysql}/*.sql`; `/opt/repos/orca/backend-go/services/task-service/migrations/postgres/0001_init.up.sql`, `…/mysql/0001_init.up.sql` (dòng 40–52), `…/task-service/internal/adapter/postgres/share_link.go`, `…/adapter/mysql/edges.go`
- `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go`; `/opt/repos/orca/backend-go/services/annotation-service/internal/adapter/mysql/repository.go`; `/opt/repos/orca/backend-go/services/scm-integration-service/internal/adapter/mysql/rate_limit_cache.go`
- `/opt/repos/orca/backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go` (dòng 68–95), `…/mysql/folder_workspace_repository.go`; `/opt/repos/orca/backend-go/services/scm-integration-service/internal/adapter/*/webhook_delivery_repository.go`
- `/opt/repos/orca/agent/src/relay/agent-git-handler.ts` (`ALLOWED_GIT_SUBCOMMANDS` có `show`), `/opt/repos/orca/agent/src/relay/agent-git-handler-extended.ts`, `/opt/repos/orca/agent/src/relay/git-handler-ops.ts`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`
- CR liên quan: CR-CV-011, 013, 030, 031, 032, 033, 036, 037, 040, 057, 059, 070, 071, 072
