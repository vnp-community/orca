# BE-CV-SOL-020: Mô hình graph chuẩn: proto `codeintel_common` + `codeintel_graph`, domain Go, khoá `SymbolRef`, hợp nhất hai nguồn

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/`buf` nào. Nằm trong cổng đồng bộ **G0** của hợp đồng (§7.1): mở khoá mọi proto khác, collector (SOL-021) và gateway. Số field do solution này gán ở mục 2.B là **đề xuất**, chủ sở hữu CR-CV-020 chốt khi merge (hợp đồng §2: "khi CR chưa ghi số, chủ sở hữu gán theo thứ tự khai báo và không đổi về sau").

**CR:** [CR-CV-020](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-020-canonical-graph-model.md)
**Service:** `code-intel-service` (mới, do BE-CV-SOL-010 dựng; chỉ `internal/domain`, `internal/adapter/grpc/graph_proto_mapping.go`) · `backend-go/proto/orca/codeintel/v1/`
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (mục "The dependency rule", "Cross-service shared code policy": domain không import proto/DB, không chia sẻ kiểu domain giữa service, nên mô hình này sống trong `internal/domain` của riêng service), [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md) (ranh giới service), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("gRPC conventions": đặt tên proto, chỉ thêm)
**Hợp đồng:** `CONTRACT-codeintel-proto-and-data-map.md` (§1 PQ, §2, §3.1), `CONTRACT-codeintel-agent-rpc.md` (§2.2, §2.6, §4.2–4.7), `CONTRACT-codeintel-ui-api.md` (§4.1–4.2)

---

## 0. Hợp đồng áp dụng

| Phán quyết / mục | Áp dụng trong solution này |
|---|---|
| PQ-04 | `WorktreeSelector{project_id=1, worktree_ref=2}` định nghĩa ở `codeintel_common.proto` (2.B) |
| PQ-07 | Tên file `codeintel_common.proto`, `codeintel_graph.proto` (không `graph_common/graph_code`); không file nào import `codeintel.proto` (service) |
| PQ-08 | `ToolIndexStatus` (mỗi công cụ) thay `IndexStatus` của CR; `IndexStatus` tổng hợp thuộc `codeintel_binding.proto` (CR-CV-012), **không** khai báo ở đây; enum `IndexScope` có `INDEX_SCOPE_UNSPECIFIED = 0` |
| PQ-09 | **Không** khai báo `ChangeOverlay` và dải số 10–39 (CR-CV-036 sở hữu); `Risk` 5 giá trị chỉ cho `ImpactGraph` |
| PQ-10 | `ArchitectureGraph` (cụm) giữ ở graph proto cho RPC `GetClusterOverview` (CR-021); `GetArchitecture` là C4, không thuộc solution này |
| PQ-12 | `ResultMeta` là hình dạng phẳng, có `etag`, `not_modified`, `from_cache`, `generated_at`; `dev_server_id` không ra WS |
| PQ-14 (1) | Trần mỗi response `CodeIntelService` ≤ **2 MiB** (`proto.Size`), `GetSymbol` ≤ 320 KiB: hạ 3 MiB của CR về 2 MiB |
| PQ-19 (4)(5) | `ImpactGraph.affectedClusters[{id|null,label,hits,impact}]` (không `ClusterRef`); `ImpactGraph` **không có cạnh** |
| PQ-20 | Agent chuẩn hoá `SymbolRef`; backend **kiểm lại và hợp nhất**, không cộng +1 khi `line_base==1`; thiếu `line_base` thì coi GitNexus 0-based |
| PQ-29 | `SourceRef{path,line,kind}` (kind mở rộng `compose|config|adapter|migration|code|proto|wscompat|route|sql`), `ServiceRef{name,proto_service}` ở common |
| PQ-32 | Enum chuỗi trên dây: `SymbolRef.kind` chữ thường; `risk` chữ HOA (`LOW…UNKNOWN`) |
| §2.1 hàng 2–3 | Danh sách message/enum của hai file; §2.3 `ResultMeta`, `SourceInfo`, `WorktreeSelector` nguyên văn |
| §8.3 (1)(3)(4)(6) | Không DB ở solution này; không chuỗi hiển thị |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `CONTRACT-codeintel-*` (cả ba), `docs/crs/v7/code-intel-graph-pipeline/CR-CV-020-canonical-graph-model.md`, `backend-go/proto/buf.yaml` (`lint STANDARD`, `breaking FILE`), `backend-go/proto/buf.gen.yaml` (protoc-gen-go + protoc-gen-go-grpc, `paths=source_relative`, `out: gen/go`), danh sách `backend-go/proto/orca/*` (19 thư mục, **không có `codeintel/`**), danh sách `backend-go/services/*` (**không có `code-intel-service/`**), `backend-go/Makefile` (`proto-lint` kết thúc bằng `|| true`), `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md`.

Xác nhận đúng: chưa có proto và service `code-intel-service`; `go_package` của `infrafleet.proto` có dạng `github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1;infrafleetv1` nên stub codeintel dùng `.../orca/codeintel/v1;codeintelv1` (đúng CR-020 mục 2.3).

### Correction relative to CR-CV-020 (Lệch giữa CR và hợp đồng)

| # | CR nói | Hợp đồng / mã thật | Xử lý |
|---|--------|--------------------|-------|
| C1 | File `graph_common.proto`, `graph_code.proto` | PQ-07: `codeintel_common.proto`, `codeintel_graph.proto` | Dùng tên của hợp đồng |
| C2 | `ChangeOverlay` cơ sở 7 field + dải 10–39 | PQ-09: CR-CV-036 sở hữu toàn bộ | Bỏ khỏi solution; không comment `reserved for` |
| C3 | `IndexStatus` mỗi công cụ (field 1–10) | PQ-08: `ToolIndexStatus`; `IndexStatus` tổng hợp là message khác ở CR-012 | Đổi tên; giữ số 1–10 của CR, thêm 11–18 (2.B) |
| C4 | `ResultMeta.view` ∈ 10 giá trị `ViewKind` | Hợp đồng §4 (`graph_snapshots.view`) có 19 chuỗi cache + `status`, `symbol` (CR-022 không lưu hai view này) | Mở rộng `ViewKind` (additive, 2.B) và thêm bảng ánh xạ enum ↔ chuỗi (2.D) |
| C5 | Ngân sách response ≤ 3 MiB | PQ-14: ≤ 2 MiB, `GetSymbol` ≤ 320 KiB | `graph_limits.go` dùng 2 MiB |
| C6 | Agent trả dạng gốc, backend +1 dòng GitNexus (Q1) | PQ-20: agent chuẩn hoá, `lineBase=1` | Backend chỉ +1 khi `line_base==0` hoặc vắng (tương thích ngược); thêm `SourceInfo.line_base=5` |
| C7 | `ImpactGraph.affected_clusters` kiểu `ClusterRef`, không có `impacted_count`/`confidence` | PQ-19(4), `ui-api` §4.2: `{id|null,label,hits,impact}`, `impactedCount`, `ImpactSymbol.confidence` | Thêm `AffectedCluster`, `AffectedFlow`, `impacted_count=8`, `ImpactSymbol.confidence=4` |
| C8 | Không có kiểu cho kết quả `GetSymbol` | Hợp đồng §3.1: `data: SymbolDetail` (agent §4.6) | Thêm `SymbolDetail` vào `codeintel_graph.proto` (hợp đồng §2.1 hàng 3 không liệt kê; ghi trong báo cáo thiếu sót) |
| C9 | `Risk`/`IndexState`/`Freshness` | `IndexState` agent có thêm `unknown`; `Freshness` không có enum trong hợp đồng | `IndexState` thêm `UNKNOWN`; thêm enum `Freshness` (đề xuất) |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_common.proto
backend-go/proto/orca/codeintel/v1/codeintel_graph.proto
backend-go/proto/gen/go/orca/codeintel/v1/*.pb.go            # sinh, không sửa tay
backend-go/services/code-intel-service/internal/domain/
  symbol_ref.go                      # SymbolRef, SymbolKind(+String), NewSymbolKey
  symbol_key_normalization.go        # chuẩn hoá khoá, ResolveKeyCollisions, ValidateAgentSymbolRef
  symbol_kind_mapping.go             # KindFromGitNexusLabel, KindFromCodeGraphKind
  edge_kind_mapping.go               # EdgeKindFromGitNexus, EdgeKindFromCodeGraph
  symbol_merge.go                    # MergeSymbolSets, MergeEdges
  repo_path.go                       # NormalizeRepoPath, Platform
  result_meta.go                     # ResultMeta, SourceInfo, ViewKind, ViewKind.CacheName()
  tool_index_status.go               # ToolIndexStatus, IndexStats, PendingChanges, IndexScope, Freshness
  architecture_graph.go  module_graph.go  symbol_graph.go  flow_graph.go
  impact_graph.go  route_map.go  symbol_detail.go
  graph_limits.go                    # hằng giới hạn + LimitModuleGraph/LimitSymbolGraph/LimitArchitecture/LimitImpact…
backend-go/services/code-intel-service/internal/adapter/grpc/graph_proto_mapping.go   # domain <-> proto
backend-go/services/code-intel-service/testdata/symbol-vectors/                       # vector khoá dùng chung với agent (xem Q2)
```

Không có file `helpers/utils/common/misc` (AGENTS.md). Không `max-lines` disable: `symbol_merge.go` và `graph_limits.go` tách theo khái niệm nếu vượt ngưỡng lint (không thêm baseline).

### B. Proto

`codeintel_common.proto` (package `orca.codeintel.v1`, `go_package` như trên, import `google/protobuf/timestamp.proto`). Số field theo CR-020 mục 2.3 làm chuẩn; hợp đồng §2.3 chốt `ResultMeta`, `SourceInfo`, `WorktreeSelector`. Đề xuất số cho phần chưa có số:

| Message | Field (số) | Nguồn số |
|---|---|---|
| `WorktreeSelector` | `project_id=1, worktree_ref=2` | hợp đồng §2.3 |
| `SymbolRef` | `key=1, kind=2 (SymbolKind), native_kind=3, name=4, qualified_name=5, file_path=6, start_line=7, end_line=8, gitnexus_id=9, codegraph_id=10, language=11` | hợp đồng §2.1 / CR |
| `SourceInfo` | `tool=1 (ToolName), version=2, indexed_at=3, commit=4, line_base=5` | hợp đồng §2.3 |
| `SourceRef` | `path=1, line=2, kind=3` | hợp đồng §2.1 |
| `ServiceRef` | `name=1, proto_service=2` | hợp đồng §2.1 |
| `ClusterRef` | `id=1, label=2` | CR |
| `ResultMeta` | `repo=1 … not_modified=13` | hợp đồng §2.3 nguyên văn (13 trường, `view` kiểu `ViewKind`) |
| `ToolIndexStatus` | `tool=1, version=2, repo=3, state=4 (IndexState), indexed_commit=5, head_commit=6, indexed_at=7, stats=8 (IndexStats), pending_changes=9, languages=10`; **đề xuất** `available=11, supported=12, merge_base=13, index_scope=14 (IndexScope), freshness=15 (Freshness), dirty_since_index=16, changed_files_not_in_index=17, indicators=18` | 1–10 theo CR; 11–18 đề xuất, khớp `ui-api` §4.1 |
| `IndexStats` | `files=1, nodes=2, edges=3, communities=4, processes=5` | CR |
| `PendingChanges` | `added=1, modified=2, removed=3` (vắng = `null`) | CR |

Enum (mọi enum có `*_UNSPECIFIED = 0`; tiền tố theo `buf lint STANDARD`, ví dụ `SYMBOL_KIND_FUNCTION`): `SymbolKind` (13 giá trị, hợp đồng §2.1), `EdgeKind` (14 giá trị của CR), `ToolName` (`GITNEXUS, CODEGRAPH` — thông báo `tool:"git"` mang bằng chuỗi, không qua enum, hợp đồng §2.3), `IndexState` (`MISSING, BUILDING, READY, STALE, UNKNOWN`), `IndexScope` (`EXACT, REPO_ROOT, STALE, NONE`; PQ-08), `Freshness` (`FRESH, FRESH_BASE, STALE, UNKNOWN`), `Risk` (`LOW, MEDIUM, HIGH, CRITICAL, UNKNOWN`), `ViewKind`:

```proto
enum ViewKind {
  VIEW_KIND_UNSPECIFIED = 0;
  VIEW_KIND_STRUCTURE = 1; VIEW_KIND_ARCHITECTURE = 2; VIEW_KIND_FLOWS = 3; VIEW_KIND_FLOW = 4;
  VIEW_KIND_SUBGRAPH = 5;  VIEW_KIND_IMPACT = 6;       VIEW_KIND_SYMBOL = 7; VIEW_KIND_ROUTES = 8;
  VIEW_KIND_CHANGE_OVERLAY = 9; VIEW_KIND_STATUS = 10;           // 1..10 theo CR-020
  // 11.. đề xuất: phủ tập `graph_snapshots.view` của hợp đồng §4 (T3)
  VIEW_KIND_CLUSTERS = 11; VIEW_KIND_READING_ORDER = 12; VIEW_KIND_ERD = 13; VIEW_KIND_STORAGE = 14;
  VIEW_KIND_DATAFLOWS = 15; VIEW_KIND_DATAFLOW = 16; VIEW_KIND_FINDINGS = 17; VIEW_KIND_CONTRACT_DIFF = 18;
  VIEW_KIND_CONTRACT_CATALOG = 19; VIEW_KIND_REQUIREMENT_TRACE = 20; VIEW_KIND_AI_SUMMARY = 21;
}
```

`codeintel_graph.proto` (import common). Kiểu dữ liệu (cạnh tham chiếu nút bằng `key`, không lồng nút): `ClusterNode{id=1,label=2,symbol_count=3,cohesion=4 (double),keywords=5,top_files=6,dominant_language=7,area=8}`, `ClusterEdge{from=1,to=2,weight=3,kind_counts=4 map<string,int32>}`, `ArchitectureGraph{nodes=1,edges=2}`, `ModuleNode{id=1,kind=2,language=3,symbol_count=4,loc=5,cluster=6,area=7}`, `ModuleEdge{from=1,to=2,kind=3,count=4}`, `ModuleGraph{nodes=1,edges=2}`, `SymbolNode{ref=1,is_exported=2,signature=3,cluster=4}`, `SymbolEdge{from_key=1,to_key=2,kind=3 (EdgeKind),confidence=4 (float),reason=5,line=6,sources=7 (repeated ToolName)}`, `SymbolGraph{center=1,nodes=2,edges=3,depth=4}`, `FlowSummary{id=1,label=2,process_type=3,step_count=4,communities=5,entry=6,terminal=7}`, `FlowStep{step=1,symbol=2,cluster=3,file_path=4,start_line=5}`, `FlowGraph{flow=1,steps=2,edges=3}`, `ImpactSymbol{symbol=1,via=2,direct=3, confidence=4 (đề xuất)}`, `ImpactLevel{depth=1,symbols=2}`, `ImpactGraph{target=1,direction=2 (string),risk=3 (Risk),levels=4,affected_flows=5 (AffectedFlow),affected_clusters=6 (AffectedCluster),tests_covering=7,impacted_count=8 (đề xuất)}`, `AffectedFlow{flow_id=1,label=2,step_count=3,optional int32 changed_step=4}`, `AffectedCluster{optional string id=1,label=2,hits=3,impact=4}`, `RouteNode{id=1,path=2,method=3,file_path=4,middleware=5,response_keys=6,error_keys=7,side=8}`, `RouteEdge{route_id=1,handler=2,kind=3}`, `RouteMap{routes=1,edges=2}`, và (mới, C8) `SymbolDetail{symbol=1, map<string,RelatedSymbolList> incoming=2, outgoing=3, repeated SymbolFlowRef flows=4, SymbolSource source=5, string source_omitted=6}` với `SymbolSource{text=1,start_line=2,end_line=3,truncated=4}`.

Quy tắc tiến hoá (nhắc lại): chỉ thêm; không đổi số/kiểu/tên; bỏ field thì `reserved` cả số lẫn tên; `native_kind` là `string`; không secret/mã nguồn trừ `SymbolDetail.source` (chỉ response `GetSymbol`, hợp đồng H8).

### C. Domain Go

Hàm thuần, idempotent, không import proto/DB. Chữ ký (mới):

```go
// symbol_key_normalization.go
func NewSymbolKey(kind SymbolKind, filePath, qualifiedName, name string) string   // "<kind>:<filePath>:<qn||name>", NFC
func SymbolRefFromGitNexus(n GitNexusNode, lineBase int) SymbolRef               // lineBase 0 hoặc vắng => +1
func SymbolRefFromCodeGraph(n CodeGraphNode) SymbolRef                           // "::" -> ".", dòng giữ nguyên
func ValidateAgentSymbolRef(in AgentSymbolRef, ws string, plat Platform) (SymbolRef, error)
func ResolveKeyCollisions(refs []SymbolRef) []SymbolRef                          // "#<arity>" rồi "#L<startLine>"
```

`ValidateAgentSymbolRef` (PQ-20): dựng lại khoá từ `kind/filePath/qualifiedName`, so với `key` của agent; khác → dùng khoá của backend và tăng `symbol_key_mismatch_total` (không từ chối); `filePath` đi qua `NormalizeRepoPath`; kind lạ → `VALUE` + giữ `nativeKind` + `unknown_native_kind_total`.

Bảng kind/cạnh (CR-020 mục 2.4) cài thành hai `map` hằng + test bảng từng dòng. Hợp nhất (`symbol_merge.go`): khớp chính theo `key`; khớp phụ khi mỗi phía đúng một ứng viên, cùng `filePath`, `name`, nhóm kind (callable/type), `|Δ startLine| ≤ 2`; ghi cả `gitnexus_id` và `codegraph_id`; `start/end` ưu tiên GitNexus; `signature/language` lấy CodeGraph; `Route` **không** hợp nhất (CR); hai nguồn cùng `key` mà `startLine` lệch > 2 → cờ `sources_disagree` (hợp đồng agent §2.6). `MergeEdges`: cạnh `(from_key,to_key,kind)` trùng giữ một, `sources` hợp, `confidence` lấy max.

`NormalizeRepoPath(raw, workspaceRoot string, plat Platform) (string, error)` (CR mục 2.6): tuyệt đối trong root thì cắt tiền tố (so sánh gập hoa/thường chỉ trên `win32`/`darwin`); tuyệt đối ngoài root hoặc tiền tố UNC `\\wsl$`/`\\wsl.localhost` → `CODEINTEL_PATH_NOT_ALLOWED` (+ bộ đếm `paths_rejected_total`); đổi `\`→`/` chỉ khi `win32`; `path.Clean`, loại `./`, từ chối `..`; NFC. Mã lỗi dùng `apperrors.New(KindPermissionDenied, "CODEINTEL_PATH_NOT_ALLOWED", …)` (`common/apperrors` đã có `KindPermissionDenied`, đã đọc `apperrors.go`).

`graph_limits.go` (PQ-14, agent §4.x): `ClusterNodesMax=500`, `ClusterEdgesMax=5000`, `FlowPageMax=100`, `FlowStepsMax=200`, `SymbolGraphNodesMax=1500`, `SymbolGraphEdgesMax=4000`, `ImpactSymbolsMax=300`, `ChangedSymbolsMax=2000` (dành cho CR-036), `SymbolSourceMaxBytes=200<<10`, `ResponseMaxBytes=2<<20`, `SymbolResponseMaxBytes=320<<10`. Mỗi `Limit*` trả `(đồ thị đã cắt, truncated bool, totalCount int64)`; thứ tự cắt xác định như CR mục 2.7 (cụm theo `symbol_count` giảm dần rồi `id`; cạnh theo `weight` giảm dần rồi `(from,to)`; nút `SymbolGraph` theo khoảng cách tới `center` rồi bậc giảm dần rồi `key`; cạnh mồ côi bị bỏ). Sau khi cắt số lượng mà `proto.Size` > trần thì cắt tiếp cùng thứ tự (hàm nhận một `SizeFunc` để domain không import proto).

### D. Ánh xạ `ViewKind` ↔ tên cache (một nguồn cho SOL-022 và gateway)

`ViewKind.CacheName()` trả chuỗi của hợp đồng §4/T3: `STRUCTURE→structure, ARCHITECTURE→architecture, CLUSTERS→clusters, FLOWS→flows, FLOW→flow, SUBGRAPH→subgraph, IMPACT→impact, SYMBOL→symbol, ROUTES→routes, CHANGE_OVERLAY→changeOverlay, READING_ORDER→readingOrder, ERD→erd, STORAGE→storage, DATAFLOWS→dataflows, DATAFLOW→dataflow, FINDINGS→findings, CONTRACT_DIFF→contractDiff, CONTRACT_CATALOG→contractCatalog, REQUIREMENT_TRACE→requirementTrace, AI_SUMMARY→aiSummary, STATUS→status`. Test song ánh: mỗi giá trị enum (trừ `UNSPECIFIED`) có tên duy nhất và `ParseViewKind(CacheName(v)) == v`.

### E. `graph_proto_mapping.go` (adapter/grpc)

Hai chiều domain ↔ proto cho mọi kiểu ở C; ánh xạ `Risk` ↔ chuỗi HOA, `SymbolKind` ↔ chuỗi thường khi sinh JSON ở gateway (gateway làm; ở đây chỉ proto). Test round-trip không mất field (reflection `protoreflect` duyệt mọi trường có giá trị khác mặc định).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Domain nằm trong `internal/domain` của service, không thành module chung | `arch/03` cấm chia sẻ kiểu domain giữa service; gateway/agent chỉ chia sẻ qua proto/JSON |
| Backend kiểm lại `key` của agent thay vì tin | Lỗi chuẩn hoá ở agent sẽ làm hỏng khoá cache và tiến độ đã xem (key là khoá lâu dài); chi phí tính lại thấp |
| `line_base` nằm trong `SourceInfo` | PQ-20 yêu cầu `lineBase=1` bắt buộc; thiếu thì tương thích ngược (GitNexus 0-based) |
| `ViewKind` mở rộng, không bỏ giá trị nào của CR | Additive (H5); giữ số 1–10 |
| `ImpactGraph` không cạnh | PQ-19(5): GitNexus `byDepth` không có nút cha |
| Cắt theo thứ tự xác định + hàm `SizeFunc` | Cùng input ra cùng output, ETag ổn định (SOL-022) mà domain vẫn không import proto |
| Không tạo `graph_sources`/`ChangeOverlay` ở đây | Mỗi CR feature `code-intel-sources` sở hữu message của mình (PQ-09/29) |

## 4. Phụ thuộc chéo khu vực

| Hướng | Solution | Quan hệ |
|---|---|---|
| Cùng khu vực, trước | `BE-CV-SOL-010-scaffold-code-intel-service` | module `code-intel-service`, thư mục `proto/orca/codeintel/v1`, `buf` CI (cổng G0) |
| Cùng khu vực, sau | `BE-CV-SOL-021-agent-collector` (giải mã vào mô hình), `BE-CV-SOL-022-snapshot-cache` (`ViewKind.CacheName()`, hằng giới hạn), `BE-CV-SOL-036-*`, `BE-CV-SOL-040-*` | import message/kiểu của solution này |
| Agent | `AG-CV-SOL-002-gitnexus-extraction`, `AG-CV-SOL-003-codegraph-extraction`, `AG-CV-SOL-070-golden-fixtures-and-parsers` | cùng vector khoá (PQ-20, CR-CV-070); agent là bên chuẩn hoá |
| Frontend | `FE-CV-SOL-050-types-and-runtime-bridge` | mirror TypeScript sinh từ proto này (`ui-api` §4.1–4.2) |

Thứ tự (hợp đồng §7): G0 = CR-010 + CR-020 → mọi proto khác. Task đầu (TASK-020-01) khoá vector kiểm thử trước khi viết proto.

## 5. Kiểm thử

- **Unit (bảng):** mỗi dòng bảng kind/cạnh một ca (gồm nhãn mới giả lập); chuẩn hoá khoá với dữ liệu thật của CR (Method có `#`, Property TS, Section, Route, File, Folder, field Kotlin có tiền tố gói, route CodeGraph `…::POST:/local`); `NormalizeRepoPath` ba nền tảng (`C:\repo\src\a.ts` root `c:\repo` `win32` → `src/a.ts`; `/home/u/repo/../x` lỗi; `\\wsl$\Ubuntu\…` lỗi; `linux` giữ `a\b.ts`); hợp nhất (cặp `prefetchManagedWorktreeCreateBase` dòng 1041/1042 thành một nút); giới hạn và tính xác định (chạy hai lần).
- **Property:** `NewSymbolKey` idempotent trên đầu vào đã chuẩn hoá; `ResolveKeyCollisions` không sinh hai khoá trùng.
- **Proto:** `buf lint`, `buf breaking`; round-trip domain ↔ proto; kiểm tra phản chiếu: không có field tên chứa `secret|token|password` trong `codeintel_*.proto`.
- **Hai dialect/tenant:** không áp dụng (không có DB ở solution này); ghi nhận để tránh nhầm với §8.3 (3)(4).
- Chưa chạy test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chỉ có **một** ví dụ khớp tay hai nguồn và hai hàm lệch dòng (CR mục 6); tỉ lệ khớp phụ tổng thể chưa đo, CR-CV-070 đo trên fixture vàng.
- Quy tắc va chạm `#<arity>` → `#L<startLine>` phải **giống hệt** ở agent (`AG-CV-SOL-002`); lệch sẽ làm khoá agent ≠ khoá backend và `symbol_key_mismatch_total` tăng (không hỏng nhưng cache không dùng chung được).
- Số field 11–18 của `ToolIndexStatus`, `ViewKind` 11–21, `ImpactGraph.impacted_count=8` là đề xuất; nếu CR-CV-012/060 gán số khác cho cùng message thì phải đồng bộ trước khi merge.
- `SymbolDetail` chưa có trong hợp đồng §2.1; nếu chủ hợp đồng đặt ở file khác thì chuyển.
- Chưa kiểm chứng GitNexus/CodeGraph báo đường dẫn Windows/WSL (mọi ví dụ là Linux).

## 7. Câu hỏi mở

- **Q1.** `Freshness` là enum hay chuỗi trên proto? Solution chọn enum (cần chủ hợp đồng xác nhận).
- **Q2.** Đường dẫn chính thức của bộ vector khoá dùng chung agent/backend (hợp đồng G1 chỉ nêu `testdata/agent-results/*.json`); tạm `services/code-intel-service/testdata/symbol-vectors/`.
- **Q3.** `Section` (`DOC`) có vào mô hình mặc định không (CR Q2): giữ enum, chưa có view dùng.
- **Q4.** Hỗ trợ UNC/WSL (CR Q4): từ chối an toàn (O-14 hợp đồng: chỉ POSIX ở MVP).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-04/07/08/09/10/12/14/19/20/29/32; §2.1, 2.3; §7.1)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md` (§2.2, 2.6, 4.2–4.7)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` (§4.1–4.2)
- `/opt/repos/orca/docs/crs/v7/code-intel-graph-pipeline/CR-CV-020-canonical-graph-model.md`
- `/opt/repos/orca/backend-go/proto/buf.yaml`, `buf.gen.yaml`, `backend-go/Makefile` (dòng `proto-lint`), `backend-go/common/apperrors/apperrors.go`
- `/opt/repos/orca/specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md`
- Mẫu định dạng: `/opt/repos/orca/specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-001-scaffold-request-service.md`
