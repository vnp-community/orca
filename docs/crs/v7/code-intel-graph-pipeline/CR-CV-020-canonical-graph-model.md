# CR-CV-020 — Mô hình graph chuẩn: proto, domain Go, khoá `SymbolRef`, hợp nhất hai nguồn

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-020 |
| **Tên** | Định nghĩa schema graph chuẩn (proto `orca.codeintel.v1` + kiểu domain Go), khoá `SymbolRef`, ánh xạ kind, hợp nhất GitNexus/CodeGraph, chuẩn hoá đường dẫn, giới hạn kích thước |
| **Loại** | Feature (mô hình dữ liệu dùng chung) |
| **Priority** | 🔴 P0 |
| **Effort** | Large (proto + domain thuần + bộ quy tắc hợp nhất có kiểm thử vàng) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 (module `code-intel-service`, thư mục proto `orca/codeintel/v1`, `buf`) |
| **Mở khoá** | CR-CV-021 (collector giải mã vào mô hình này), CR-CV-022 (cache lưu mô hình này), CR-CV-031..038 (dùng `SymbolRef`, `ResultMeta`), CR-CV-040 (gateway), CR-CV-050 (TypeScript mirror) |
| **Tác động** | `backend-go/proto/orca/codeintel/v1/graph_common.proto` (mới), `graph_code.proto` (mới), `backend-go/proto/gen/go/orca/codeintel/v1` (sinh), `backend-go/services/code-intel-service/internal/domain/` (mới, các file nêu ở 2.5) |

---

## 1. Bối cảnh và vấn đề

1. UI không được biết GitNexus hay CodeGraph; chỉ vẽ một schema chuẩn (research 05 mục đầu). README v7 mục 3.4 đã chốt tên kiểu và quy tắc `SymbolRef.key`, nhưng chưa có định nghĩa proto, quy tắc chuyển đổi, hay giới hạn bắt buộc. Ba CR khác (collector, cache, gateway) cần một mô hình cụ thể để cùng bám.
2. Đã chạy thử trên repo Orca ngày 2026-10-05 (chỉ-đọc), phát hiện các điểm lệch giữa hai công cụ mà research 05 §3 chưa nêu:

| Điểm | GitNexus 1.6.9 | CodeGraph 1.4.1 | Hệ quả |
|---|---|---|---|
| Dòng bắt đầu | **0-based**: `NewRelayByDevServer` `startLine=33` | **1-based**: cùng hàm `startLine=34` (file thật: dòng 34, `relay_by_dev_server.go`) | Phải quy về 1-based khi chuẩn hoá, nếu không cạnh/đánh dấu lệch một dòng |
| Tên đủ | id `Method:<path>:Relay.Execute#2` (phân tách `.`, hậu tố `#<arity>`) | `qualifiedName` `Relay::Execute` (phân tách `::`; Kotlin/Swift có thêm tiền tố gói `expo.modules.twowayaudio::AudioEngine::SAMPLE_RATE`) | Cần chuẩn hoá phân tách và bỏ hậu tố |
| Hậu tố `#N` | có ở 31 432 − 2 128 = 29 304 `Method` và 233 `Function` | không có | Quá tải (overload) làm khoá trùng: cần luật xử lý |
| Route | id `Route:/voice-settings` (không chứa tệp; `method` có thể rỗng) | `name` `POST /local`, `qualifiedName` `<file>::POST:/local` | Hai nguồn khó khớp; chấp nhận không hợp nhất route (mục 3) |
| File/Folder | id `File:<path>`, `Folder:<path>` | kind `file`, `qualifiedName=<path>`, `name`=tên cuối | Khoá `file:<path>:<basename>` |
| Kind chỉ có một bên | `Community`, `Process`, `Section` | `import`, `enum_member`, `namespace`, `component`, `field` | Bảng ánh xạ ở 05 §3 thiếu `import`, `enum_member`, `namespace`, `Enum`, `Constructor`, `Property`, `Variable` |
| Cạnh | `CALLS, IMPORTS, ACCESSES, CONTAINS, DEFINES, MEMBER_OF, HAS_METHOD, HAS_PROPERTY, STEP_IN_PROCESS, METHOD_IMPLEMENTS, METHOD_OVERRIDES, IMPLEMENTS, HANDLES_ROUTE, EXTENDS, FETCHES, ENTRY_POINT_OF` | `calls, contains, references, imports, instantiates, implements, extends` | Bảng ánh xạ cạnh cần có (mục 2.4) |

   Kiểm chứng: `gitnexus cypher -r orca "MATCH (n) RETURN labels(n)…"` và `…[r:CodeRelation]… RETURN r.type…` (16 nhãn, 16 loại cạnh); đọc `nodes.kind`/`edges.kind` của `.codegraph/codegraph.db` ở chế độ chỉ-đọc (17 kind node, 7 kind cạnh); `codegraph query --json`. Số liệu khớp research 04 §1.
3. Một ví dụ khớp tay đã xác nhận: `prefetchManagedWorktreeCreateBase` có GitNexus id `Method:backend/src/main/runtime/orca-runtime-worktree-creation.ts:RuntimeWorktreeCreationCommands.prefetchManagedWorktreeCreateBase#1` (dòng 1041) và CodeGraph `method`, `RuntimeWorktreeCreationCommands::prefetchManagedWorktreeCreateBase` (dòng 1042). Cùng tệp có lớp khác mà GitNexus gọi `Property` và CodeGraph gọi `property`: kind hai bên khớp trong trường hợp này.
4. Chưa có `code-intel-service` (không có `backend-go/services/code-intel-service`), chưa có proto `orca/codeintel` (liệt kê `backend-go/proto/orca`: không có thư mục này).

## 2. Giải pháp đề xuất

### 2.1 Phạm vi sở hữu (tránh xung đột giữa các CR)

| Nhóm message | File proto | Sở hữu | Ghi chú |
|---|---|---|---|
| `SymbolRef`, `SourceInfo`, `ResultMeta`, `IndexStatus` (+ `IndexStats`, `PendingChanges`), `ClusterRef`, các enum | `graph_common.proto` (mới) | **CR-CV-020** | CR khác chỉ import, không sửa |
| `ArchitectureGraph`, `ModuleGraph`, `SymbolGraph`, `FlowSummary`, `FlowGraph`, `ImpactGraph`, `RouteMap`, `ChangeOverlay` | `graph_code.proto` (mới) | **CR-CV-020** định nghĩa; `ChangeOverlay` mở rộng bởi CR-CV-036/037/038 theo dải số (2.3) | |
| `C4ComponentView`, `DataFlow`, `ErdModel`, `StorageMap` (+ kiểu con) | `graph_sources.proto` | **Feature `code-intel-sources`** (CR-CV-031 `ErdModel`, 033 `C4ComponentView`, 034 `DataFlow`, 035 `StorageMap`) | **CR này không tạo file và không định nghĩa field.** Chỉ cam kết: cùng package `orca.codeintel.v1`, import `graph_common.proto`, tái dùng `SymbolRef`/`ResultMeta`, không định nghĩa lại enum chung; CR đầu tiên của feature đó tạo file |
| Message request/response từng RPC | `code_intel_service.proto` | CR sở hữu RPC (README v7 mục 3.6); CR-CV-010 tạo service rỗng-có-health | Mỗi response nhúng `ResultMeta meta` + payload (2.2) |

### 2.2 Phong bì kết quả (`CodeIntelResult<T>`)

Proto không có generic nên phong bì là **message `ResultMeta` nhúng vào từng response**, không phải một message bọc `Any`:

```proto
// graph_common.proto (mới)
message ResultMeta {
  string repo = 1;               // tên hiển thị repo, lấy từ repo_bindings
  string worktree_id = 2;
  string dev_server_id = 3;
  ViewKind view = 4;
  repeated SourceInfo sources = 5;
  string head_commit = 6;
  bool stale = 7;
  bool truncated = 8;
  int64 total_count = 9;         // tổng trước khi cắt; 0 nếu không biết
  string etag = 10;              // CR-CV-022
  google.protobuf.Timestamp generated_at = 11;
  bool from_cache = 12;
  bool not_modified = 13;        // CR-CV-022: khớp If-None-Match, response không mang data
}
// response mẫu (CR sở hữu RPC viết): GetArchitectureResponse { ResultMeta meta = 1; ArchitectureGraph data = 2; }
```

Khớp tên với định dạng JSON của agent (`sources`, `headCommit`, `stale`, `truncated`, `totalCount`, `data`; README v7 mục 3.2): bộ giải mã ở CR-CV-021 ánh xạ 1-1 sang `ResultMeta` + payload.

### 2.3 Message (đủ field để triển khai)

Tất cả trong `package orca.codeintel.v1;`, `go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1"` (cùng kiểu `orca.request.v1` ở v6). Cạnh tham chiếu nút bằng `key` (chuỗi) chứ không lồng nút, để giữ kích thước.

Enum (giá trị 0 là `UNSPECIFIED`, tiền tố theo `buf lint STANDARD`):
- `SymbolKind`: `FUNCTION, METHOD, TYPE, VALUE, FILE, FOLDER, ROUTE, COMPONENT, NAMESPACE, IMPORT, CLUSTER, FLOW, DOC`.
- `EdgeKind`: `CALLS, IMPORTS, ACCESSES, EXTENDS, IMPLEMENTS, METHOD_OVERRIDES, METHOD_IMPLEMENTS, HAS_METHOD, HAS_PROPERTY, REFERENCES, INSTANTIATES, CONTAINS, HANDLES_ROUTE, FETCHES` (hợp của 05 §2.3, §2.6 và `CONTAINS` cho `ModuleEdge`).
- `ViewKind`: `STRUCTURE, ARCHITECTURE, FLOWS, FLOW, SUBGRAPH, IMPACT, SYMBOL, ROUTES, CHANGE_OVERLAY, STATUS` (mỗi view tương ứng một khoá cache, CR-CV-022).
- `IndexState`: `MISSING, BUILDING, READY, STALE` (từ 05 §2.8).
- `Risk`: `LOW, MEDIUM, HIGH, CRITICAL, UNKNOWN`.
- `ToolName`: `GITNEXUS, CODEGRAPH`.

| Message | Field (số) |
|---|---|
| `SymbolRef` | `key`=1, `kind`=2, `native_kind`=3 (kind gốc: `Struct`, `type_alias`…), `name`=4, `qualified_name`=5 (đã chuẩn hoá, 2.4), `file_path`=6 (tương đối gốc repo), `start_line`=7, `end_line`=8 (1-based), `gitnexus_id`=9, `codegraph_id`=10, `language`=11 |
| `SourceInfo` | `tool`=1 (`ToolName`), `version`=2, `indexed_at`=3, `commit`=4 |
| `ClusterRef` | `id`=1, `label`=2 |
| `ClusterNode` | `id`=1, `label`=2, `symbol_count`=3, `cohesion`=4 (double), `keywords`=5, `top_files`=6, `dominant_language`=7, `area`=8 |
| `ClusterEdge` | `from`=1, `to`=2, `weight`=3, `kind_counts`=4 (`map<string,int32>`) |
| `ArchitectureGraph` | `nodes`=1, `edges`=2 |
| `ModuleNode` | `id`=1 (đường dẫn), `kind`=2 (`FILE`/`FOLDER`), `language`=3, `symbol_count`=4, `loc`=5, `cluster`=6 |
| `ModuleEdge` | `from`=1, `to`=2, `kind`=3 (`IMPORTS`/`CONTAINS`), `count`=4 |
| `ModuleGraph` | `nodes`=1, `edges`=2 |
| `SymbolNode` | `ref`=1, `is_exported`=2, `signature`=3, `cluster`=4 |
| `SymbolEdge` | `from_key`=1, `to_key`=2, `kind`=3, `confidence`=4 (float, 0 = không biết), `reason`=5, `line`=6, `sources`=7 (`repeated ToolName`) |
| `SymbolGraph` | `center`=1 (`SymbolRef`), `nodes`=2, `edges`=3, `depth`=4 |
| `FlowSummary` | `id`=1, `label`=2, `process_type`=3, `step_count`=4, `communities`=5 (đã bỏ nháy đơn), `entry`=6, `terminal`=7 |
| `FlowStep` | `step`=1, `symbol`=2, `cluster`=3, `file_path`=4, `start_line`=5 |
| `FlowGraph` | `flow`=1, `steps`=2, `edges`=3 |
| `ImpactSymbol` | `symbol`=1, `via`=2 (`EdgeKind`), `direct`=3 |
| `ImpactLevel` | `depth`=1, `symbols`=2 |
| `ImpactGraph` | `target`=1, `direction`=2 (`string`: `upstream`/`downstream`), `risk`=3, `levels`=4, `affected_flows`=5, `affected_clusters`=6, `tests_covering`=7 |
| `RouteNode` | `id`=1, `path`=2, `method`=3, `file_path`=4, `middleware`=5, `response_keys`=6, `error_keys`=7, `side`=8 |
| `RouteEdge` | `route_id`=1, `handler`=2, `kind`=3 |
| `RouteMap` | `routes`=1, `edges`=2 |
| `ChangeOverlay` | **Cơ sở (CR này):** `base`=1, `head`=2, `changed_files`=3, `changed_symbols`=4, `affected_flows`=5, `affected_clusters`=6, `risk`=7. **Dải số dành riêng (ghi bằng comment `// reserved for CR-CV-0xx`, không dùng `reserved` cho đến khi CR đó thêm):** 10–19 CR-CV-036 (`touched_tables`, `uncovered_symbols`, `reading_order`), 20–29 CR-CV-037 (`violations`), 30–39 CR-CV-038 (`touched_contracts`) |
| `IndexStatus` | `tool`=1, `version`=2, `repo`=3, `state`=4, `indexed_commit`=5, `head_commit`=6, `indexed_at`=7, `stats`=8 (`IndexStats{files,nodes,edges,communities,processes}`), `pending_changes`=9 (`PendingChanges{added,modified,removed}`), `languages`=10 |

Quy tắc tiến hoá (để `buf breaking` với `breaking.use: FILE`, `proto/buf.yaml`, luôn xanh):
- Chỉ **thêm** field/message/enum value; không đổi số hay kiểu; không đổi tên field (với `FILE`, đổi tên là phá).
- Field/value bỏ đi: dùng `reserved` cả số lẫn tên; không tái sử dụng.
- Không đổi tên enum value `UNSPECIFIED = 0`; client phải xử lý giá trị lạ (UI hiển thị "khác").
- Hai phiên bản công cụ có thể thêm `native_kind` mới: `native_kind` là `string` chứ không phải enum, để tool đổi không phá hợp đồng.
- Không đặt secret hay mã nguồn trong các message này; mã nguồn chỉ nằm trong response `GetSymbol` (CR-CV-040).

### 2.4 Khoá `SymbolRef` và chuẩn hoá

**Công thức (README v7 mục 3.4):** `key = "<kind>:<filePath>:<qualifiedName||name>"`, `kind` là tên `SymbolKind` viết thường.

Các bước chuẩn hoá (hàm thuần, idempotent, nằm trong `internal/domain`):

1. **filePath**: `NormalizeRepoPath` (2.6).
2. **qualifiedName**:
   - GitNexus: lấy `id`, bỏ tiền tố `<Label>:`, bỏ tiếp `<filePath>:` (dùng `filePath` của nút, không tự tách theo `:`), bỏ hậu tố `#<số>` (lưu `arity`, không vào khoá), kết quả giữ phân tách `.`. Với `Section` (id dạng `Section:README.md:L29:Features`) khoá dùng `doc:<filePath>:L29:Features` (dòng là một phần định danh, GitNexus chưa cho cách khác).
   - CodeGraph: lấy `qualifiedName`, thay mọi `::` bằng `.`; nếu là `file` thì `qualifiedName` bỏ (dùng `name` = tên cuối).
   - File/folder: `name` = tên cuối đường dẫn, `filePath` = đường dẫn đầy đủ → `file:Casks/orca.rb:orca.rb`, `folder:tests:tests`.
   - Cluster/flow: `filePath` rỗng → `cluster::comm_6117`, `flow::proc_0_checkspanel` (id GitNexus nguyên văn).
   - Chuẩn hoá Unicode NFC cho cả khoá; không đổi hoa/thường.
3. **Dòng**: GitNexus `startLine/endLine` cộng 1 (đã xác nhận 0-based); CodeGraph giữ nguyên. Lưu 1-based trong mô hình chuẩn. Quy tắc áp dụng theo thẻ `tool` của nút nên không cộng hai lần; nếu CR-CV-002 (agent) quyết định tự chuẩn hoá dòng thì phải ghi rõ trong hợp đồng agent, xem mục 7 (Q1).
4. **Trùng khoá do overload**: nếu hai nút GitNexus cùng `(kind, filePath, qualifiedName sau bỏ #)` thì khoá của từng nút thêm hậu tố `#<arity>`; nếu vẫn trùng, thêm `#L<startLine>`. Hậu tố chỉ thêm khi có va chạm trong cùng tệp, nên khoá phần lớn ổn định giữa các commit.

**Khớp hai nguồn (hợp nhất).** Đầu vào: tập nút GitNexus (ưu tiên: có `cluster`, `flow`, `confidence`) và CodeGraph (bổ sung: `signature`, `language`, `visibility`, `isAsync`, `component`, `import`).
- Khớp chính: cùng `key`.
- Khớp phụ (chỉ khi khớp chính thất bại và mỗi phía có đúng một ứng viên): cùng `filePath`, cùng `name`, **cùng nhóm kind** (`function/method/value` coi là một nhóm callable; `type` một nhóm), `|startLine chuẩn hoá chênh| ≤ 2`. Mục đích: GitNexus có thể bỏ tên lớp trong `Method` TypeScript (`…:handleData#1`) trong khi CodeGraph luôn có lớp (`Class::handleData`). Độ chính xác của khớp phụ **chưa đo**, CR-CV-070 đo trên fixture vàng.
- Khi khớp: giữ một `SymbolRef`, ghi cả `gitnexus_id` và `codegraph_id`; `start_line/end_line` ưu tiên GitNexus (đã chuẩn hoá); `signature/language` từ CodeGraph; `native_kind` ghép dạng `Function|function`. `codegraph_id` (hash) **không** vào khoá và không dùng làm khoá lâu dài (README v7 mục 3.4).
- Không khớp: giữ nút ở nguồn của nó, `sources` ghi một công cụ.
- **Route**: không hợp nhất (id GitNexus không chứa tệp, `method` có thể rỗng; CodeGraph qualifiedName `<file>::POST:/local`). Giữ hai `RouteNode` riêng, thẻ nguồn khác nhau; khuyến nghị UI chỉ hiển thị nguồn GitNexus nếu có (CR-CV-040).

**Ánh xạ kind (kiểm chứng bằng kinds thật, 2026-10-05):**

| `SymbolKind` | GitNexus (nhãn, đếm Orca) | CodeGraph (kind, đếm) |
|---|---|---|
| `FUNCTION` | `Function` 80 348 | `function` 59 620 |
| `METHOD` | `Method` 31 432, `Constructor` 18 | `method` 37 979 |
| `TYPE` | `Struct` 4 740, `Class` 932, `Interface` 459, `Enum` 19 | `struct` 4 442, `class` 956, `interface` 458, `enum` 33, `type_alias` 19 350 |
| `VALUE` | `Const` 34 424, `Variable` 1 111, `Property` 26 345 | `constant` 22 353, `variable` 2 985, `property` 63 589, `field` 184, `enum_member` 122 |
| `FILE` / `FOLDER` | `File` 20 174 / `Folder` 1 616 | `file` 15 724 (không có folder) |
| `ROUTE` | `Route` 92 | `route` 184 |
| `COMPONENT` | — | `component` 62 |
| `NAMESPACE` | — | `namespace` 4 |
| `IMPORT` | — (GitNexus dùng cạnh `IMPORTS`, không có nút) | `import` 67 865 |
| `CLUSTER` | `Community` 9 039 | — |
| `FLOW` | `Process` 300 | — |
| `DOC` | `Section` 36 507 | — |

Kind không có trong bảng (phiên bản công cụ mới): ánh xạ `VALUE` + giữ `native_kind`, tăng bộ đếm `unknown_native_kind_total`, **không** làm hỏng kết quả. Nút `IMPORT` không vào `SymbolGraph` mặc định (tăng 67 k nút vô ích); chỉ dùng để dựng `ModuleEdge`.

**Ánh xạ cạnh:**

| `EdgeKind` | GitNexus | CodeGraph | Dùng ở |
|---|---|---|---|
| `CALLS` | `CALLS` | `calls` | `SymbolEdge` |
| `IMPORTS` | `IMPORTS` | `imports` | `ModuleEdge`, `SymbolEdge` |
| `ACCESSES` | `ACCESSES` | — | `SymbolEdge` |
| `REFERENCES` | — | `references` | `SymbolEdge` (không gộp với `ACCESSES`: ngữ nghĩa khác) |
| `INSTANTIATES` | — | `instantiates` | `SymbolEdge` |
| `EXTENDS`, `IMPLEMENTS` | `EXTENDS`, `IMPLEMENTS` | `extends`, `implements` | `SymbolEdge`; C4 (CR-CV-033) |
| `METHOD_OVERRIDES`, `METHOD_IMPLEMENTS` | cùng tên | — | `SymbolEdge` |
| `HAS_METHOD`, `HAS_PROPERTY` | cùng tên | — | `SymbolEdge` (hiển thị gộp vào nút cha) |
| `CONTAINS` | `CONTAINS` | `contains` | `ModuleEdge` |
| `HANDLES_ROUTE`, `FETCHES` | cùng tên | — | `RouteMap` |
| (cấu trúc, không thành `SymbolEdge`) | `DEFINES`, `MEMBER_OF`, `STEP_IN_PROCESS`, `ENTRY_POINT_OF` | — | dựng `cluster`, `FlowGraph` |

Cạnh trùng `(from_key, to_key, kind)` từ hai nguồn: giữ một cạnh, `sources` hợp, `confidence` lấy lớn nhất (CodeGraph không có `confidence` → 0 không ảnh hưởng `max`).

### 2.5 Domain Go (mới, trong `backend-go/services/code-intel-service/internal/domain/`)

Domain thuần, không import proto/DB. Tên file theo khái niệm (AGENTS.md cấm `helpers/utils/common`):

| File | Nội dung |
|---|---|
| `symbol_ref.go` | `SymbolRef`, `SymbolKind` (+ `String()` viết thường), `NewSymbolKey` |
| `symbol_key_normalization.go` | `SymbolRefFromGitNexus`, `SymbolRefFromCodeGraph`, bỏ `#N`, đổi `::`→`.`, NFC, cộng dòng |
| `symbol_kind_mapping.go` | hai bảng ánh xạ kind (2.4), hàm `KindFromGitNexusLabel`, `KindFromCodeGraphKind` |
| `edge_kind_mapping.go` | bảng ánh xạ cạnh |
| `symbol_merge.go` | `MergeSymbolSets` (khớp chính/phụ, ghi cả hai id) và `MergeEdges` |
| `repo_path.go` | `NormalizeRepoPath` (2.6) |
| `result_meta.go` | `ResultMeta`, `SourceInfo`, `ViewKind` |
| `architecture_graph.go`, `module_graph.go`, `symbol_graph.go`, `flow_graph.go`, `impact_graph.go`, `route_map.go`, `change_overlay.go`, `index_status.go` | struct tương ứng proto |
| `graph_limits.go` | hằng giới hạn (2.7) và `LimitSymbolGraph`, `LimitArchitecture`… |
| `graph_proto_mapping.go` | **đặt ở `internal/adapter/grpc/`**, không ở domain: domain ↔ proto |

Giải mã từ JSON của agent vào các kiểu này là việc của CR-CV-021 (`internal/adapter/infrafleetclient/agent_result_decoding.go`); CR này cung cấp hàm chuẩn hoá mà bộ giải mã gọi.

### 2.6 Chuẩn hoá đường dẫn (Windows / WSL / SSH)

`NormalizeRepoPath(raw, workspaceRoot string, hostPlatform Platform) (string, error)`; `hostPlatform` lấy từ `platform` trong handshake (`HandshakeInfo.Platform`, `session.go:47-53`; `win32`, `darwin`, `linux`).

1. Dữ liệu đúng hợp đồng là **tương đối gốc repo, dấu `/`** (research 05 §3). Hàm vẫn phòng thủ khi gặp đường dẫn tuyệt đối:
   - nếu `raw` tuyệt đối và nằm dưới `workspaceRoot` (so sánh có hiểu nền tảng: `win32` không phân biệt hoa/thường và coi `\` = `/`, chữ ổ đĩa viết hoa; `darwin` mặc định không phân biệt hoa/thường; `linux` phân biệt) → cắt tiền tố;
   - nếu tuyệt đối nhưng ngoài `workspaceRoot` → lỗi `CODEINTEL_PATH_NOT_ALLOWED`, nút bị bỏ và tăng bộ đếm `paths_rejected_total` (không trả đường dẫn lạ về UI).
2. Chỉ đổi `\` thành `/` khi `hostPlatform == win32` (trên POSIX, `\` là ký tự tên tệp hợp lệ).
3. `path.Clean`; loại `./` đầu; từ chối kết quả bắt đầu bằng `..`; NFC.
4. Không đổi hoa/thường khi lưu; chỉ khi **so sánh** (ví dụ ghép `changedFiles` với nút) trên `win32`/`darwin` dùng so sánh gập hoa/thường. Hai nút chỉ khác hoa/thường trên máy phân biệt hoa/thường được giữ riêng.
5. **WSL**: agent chạy trong WSL báo `linux` và đường dẫn POSIX nên đi nhánh `linux`. Agent chạy trên Windows nhưng repo nằm ở `\\wsl$\…`/`\\wsl.localhost\…` (UNC): chưa xử lý, coi là ngoài phạm vi (Q4); hàm trả `CODEINTEL_PATH_NOT_ALLOWED` cho tiền tố UNC thay vì đoán.
6. **SSH/remote**: khoá không chứa gốc worktree nên cùng một khoá dùng được trên mọi worktree của repo. Ghép với đường dẫn tuyệt đối trên dev server (mở tệp bằng `fs.*`) làm ở backend/UI bằng `workspaceRoot` của `repo_bindings` (CR-CV-012), không ở mô hình này. Mọi thao tác chịu độ trễ SSH (AGENTS.md); mô hình không giả định thực thi cục bộ.

### 2.7 Giới hạn kích thước (bắt buộc ở mô hình)

Hằng ở `graph_limits.go`, khớp README v7 mục 3.2; nút/cạnh bị cắt theo thứ tự xác định (xem dưới) và `ResultMeta.truncated=true`, `total_count` giữ tổng gốc:

| Đồ thị | Giới hạn |
|---|---|
| `ArchitectureGraph` | ≤ 500 `ClusterNode`, ≤ 5 000 `ClusterEdge` |
| `FlowSummary` (danh sách) | ≤ 100 mỗi trang |
| `FlowGraph` | ≤ 200 bước |
| `SymbolGraph` | ≤ 1 500 nút, ≤ 4 000 cạnh, `depth ≤ 3` |
| `ImpactGraph` | ≤ 300 `ImpactSymbol` |
| `ChangeOverlay.changed_symbols` | ≤ 2 000 |
| Mã nguồn một symbol | ≤ 200 KiB (ở response `GetSymbol`) |
| Kích thước proto sau tuần tự hoá, mỗi response | **≤ 3 MiB** (dưới trần mặc định 4 MiB của gRPC; repo không đặt `MaxCallRecvMsgSize`/`MaxRecvMsgSize` ở đâu, kiểm bằng grep `backend-go`) |

Thứ tự cắt (xác định, để cùng input ra cùng output và ETag ổn định): cụm theo `symbol_count` giảm dần rồi `id`; cạnh theo `weight` giảm dần rồi `(from,to)`; nút `SymbolGraph` theo khoảng cách tới `center` tăng dần, rồi bậc giảm dần, rồi `key`; cạnh mồ côi (đầu mút bị cắt) bị bỏ. Nếu vẫn quá 3 MiB sau khi áp giới hạn số lượng, cắt tiếp theo cùng thứ tự đến khi vừa.

### 2.8 Sinh mã

Chạy `buf generate` theo `proto/buf.gen.yaml` (protoc-gen-go, protoc-gen-go-grpc, `paths=source_relative`); stub vào `backend-go/proto/gen/go/orca/codeintel/v1`. Mirror TypeScript do CR-CV-050 làm, nguồn là proto này.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| `ResultMeta` nhúng vào từng response thay vì `Any` | Proto không có generic; `Any` làm mất kiểm tra kiểu ở gateway/TS |
| Cạnh tham chiếu `key` chuỗi | Giữ kích thước (≤ 4 000 cạnh); nút nằm một chỗ |
| Bỏ `#<arity>` khỏi khoá, chỉ thêm khi va chạm | Khớp được CodeGraph (không có arity); khoá ít đổi giữa commit |
| Chuẩn hoá dòng về 1-based tại backend, theo thẻ `tool` | Đã đo lệch 1; sửa một chỗ thay vì ở từng consumer |
| Không hợp nhất `Route` | id hai bên không tương ứng; hợp nhất sai gây nhầm hơn là hiển thị hai nguồn |
| `native_kind` là `string` | Công cụ đổi nhãn không phá hợp đồng proto |
| `IMPORT` không vào `SymbolGraph` mặc định | 67 865 nút vô ích với mục tiêu review |
| Không tạo `graph_sources.proto` ở CR này | Mỗi CR feature `code-intel-sources` sở hữu field của mình; tránh hai CR sửa cùng message |
| Dải số dành riêng cho `ChangeOverlay` | CR-CV-036/037/038 cùng sửa một message, dải khác nhau thì không đụng số |

## 4. Tiêu chí chấp nhận

- [ ] `buf lint` và `buf breaking` (so với `main`) xanh; stub sinh vào `backend-go/proto/gen/go/orca/codeintel/v1`.
- [ ] `graph_common.proto` và `graph_code.proto` chứa đủ message và enum ở 2.3; không có message của `graph_sources.proto` trong hai file này.
- [ ] `SymbolRefFromGitNexus("Method", "Method:backend-go/services/infra-fleet-service/internal/usecase/relay.go:Relay.Execute#2", …)` cho `key = "method:backend-go/services/infra-fleet-service/internal/usecase/relay.go:Relay.Execute"`; `SymbolRefFromCodeGraph` với `qualifiedName "Relay::Execute"` cho **cùng** khoá.
- [ ] `startLine` GitNexus `33` → `34` (ví dụ `NewRelayByDevServer`); CodeGraph `34` → `34`.
- [ ] Hai `Method` GitNexus cùng tệp/tên/khác arity có khoá khác nhau (hậu tố `#<arity>`); khoá không có hậu tố khi không va chạm.
- [ ] `MergeSymbolSets` hợp nhất cặp `prefetchManagedWorktreeCreateBase` (GitNexus `…RuntimeWorktreeCreationCommands.prefetchManagedWorktreeCreateBase#1` dòng 1041 và CodeGraph `…::prefetchManagedWorktreeCreateBase` dòng 1042) thành một nút có cả `gitnexus_id` và `codegraph_id`.
- [ ] Kind lạ (`native_kind` không có trong bảng) không gây lỗi, vào `VALUE` và tăng bộ đếm.
- [ ] `NormalizeRepoPath`: `C:\repo\src\a.ts` với root `c:\repo` và `win32` → `src/a.ts`; `/home/u/repo/../x` → lỗi; `\\wsl$\Ubuntu\home\…` → `CODEINTEL_PATH_NOT_ALLOWED`; trên `linux`, `a\b.ts` giữ nguyên.
- [ ] Với đầu vào lớn hơn giới hạn, mỗi `Limit*` trả đúng số lượng, `truncated=true`, giữ `total_count`, và thứ tự ổn định (chạy hai lần ra cùng kết quả).
- [ ] Không có tên file `helpers`, `utils`, `common`, `misc`; không `max-lines` disable mới.

## 5. Kiểm thử

- **Unit (bảng):** ánh xạ kind/cạnh (mỗi dòng bảng 2.4 một ca, gồm nhãn mới giả lập); chuẩn hoá khoá với dữ liệu thật đã lấy ở mục 1 (Method có `#`, Property TS, Section, Route, File, Folder, field Kotlin có tiền tố gói, route CodeGraph `…::POST:/local`); `NormalizeRepoPath` đủ ba nền tảng; giới hạn và tính xác định.
- **Fixture vàng:** lưu mẫu JSON thật của `gitnexus cypher/context/impact` và `codegraph query/callers --json` cho một thư mục nhỏ (ví dụ `backend-go/services/infra-fleet-service/internal/usecase`), chạy hợp nhất, so khoá và số cặp khớp. Tỉ lệ khớp phụ ghi nhận làm đường cơ sở cho CR-CV-070.
- **Proto:** `buf breaking` trong CI; test round-trip domain ↔ proto (`graph_proto_mapping.go`) không mất field.
- **Property test:** `NormalizeSymbolKey(NormalizeSymbolKey(x)) == NormalizeSymbolKey(x)` (idempotent).
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chỉ có **một** ví dụ khớp tay hai nguồn (mục 1, điểm 3) và một chiều lệch dòng (hai hàm); tỉ lệ khớp tổng thể trên 295 k/247 k nút **chưa đo**. Khớp phụ có thể gộp nhầm các hàm cùng tên trong một tệp (ngưỡng ±2 dòng giảm nhưng không loại bỏ).
- Chưa kiểm chứng GitNexus/CodeGraph báo đường dẫn với dấu nào khi index chạy trên dev server Windows (tất cả ví dụ ở trên là Linux).
- Phát hiện lệch 0-based/1-based dựa trên hai hàm Go và một hàm TS; chưa kiểm Python/Swift/Kotlin.
- Chưa rõ `communities` 10 252 (`meta.json`) so với 9 039 (`Community` truy vấn được); `ClusterNode` dựa trên số truy vấn được.
- 3 MiB là ngân sách do CR này đặt; chưa đo kích thước thực của `SymbolGraph` 1 500 nút/4 000 cạnh.
- Hậu tố `#<arity>`: chưa rõ GitNexus tính arity thế nào cho hàm biến tham số; nếu `arity` đổi khi sửa chữ ký, khoá va chạm đổi theo (chỉ ảnh hưởng nút bị va chạm).

## 7. Câu hỏi mở

- **Q1.** Agent trả `data` ở dạng **gốc công cụ** (id, kind, dòng 0-based) hay đã chuẩn hoá? README v7 mục 3.2 không nói. CR này giả định gốc công cụ, kèm thẻ `tool` mỗi nút/cạnh; các hàm chuẩn hoá idempotent nên nếu CR-CV-002/003 chuẩn hoá một phần, cần ghi rõ để khỏi cộng dòng hai lần. Cần chốt trong CR-CV-002.
- **Q2.** Có đưa `Section` (tài liệu Markdown, 36 507 nút) vào mô hình hay loại hẳn? Hiện giữ `DOC` nhưng không có view dùng.
- **Q3.** Chọn `#<arity>` hay `#L<line>` làm hậu tố chính khi va chạm? Chọn arity vì ổn định hơn qua các commit; cần xác nhận khi có dữ liệu thật về overload TS.
- **Q4.** Hỗ trợ agent Windows với repo trong WSL UNC có thuộc phạm vi series? Hiện từ chối an toàn.
- **Q5.** `float` hay `double` cho `confidence`/`cohesion`: GitNexus trả 0..1 một vài chữ số thập phân; chọn `float`/`double` theo proto trên (`confidence` float, `cohesion` double), chưa kiểm độ chính xác `cohesion`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.2, 3.4, 3.6)
- `/opt/repos/orca/docs/research/view-code/05-graph-schemas.md`, `08-views-and-review-models.md`, `04-raw-data-and-pipeline.md`
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md` (quy ước proto, `buf breaking`)
- `/opt/repos/orca/backend-go/proto/buf.yaml` (`breaking.use: FILE`), `/opt/repos/orca/backend-go/proto/buf.gen.yaml`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go` (dòng 47-53, `HandshakeInfo.Platform`)
- `/opt/repos/orca/.codegraph/codegraph.db` (bảng `nodes`, `edges`; đọc chỉ-đọc), `.gitnexus/` (qua `gitnexus cypher -r orca`)
- `/opt/repos/orca/AGENTS.md`
