# 05 — Schema (graph) để hiển thị và kiểm soát code

Đây là **schema chuẩn** [mới] mà backend cung cấp cho UI. Mục tiêu: UI không biết GitNexus hay CodeGraph; chỉ vẽ model này. Đề xuất đặt trong proto (`orca/codeintel/v1`) và mirror sang TypeScript giống cách các service khác làm.

## 1. Khung chung (envelope)

```jsonc
CodeIntelResult<T> {
  "repo":        "orca",                 // tên repo GitNexus / thư mục
  "worktreeId":  "…",
  "devServerId": "…",
  "view":        "architecture|flows|flow|subgraph|impact|symbol|routes",
  "sources":     [{ "tool":"gitnexus","version":"1.6.9","indexedAt":"…","commit":"d819812…" },
                  { "tool":"codegraph","version":"1.4.1","indexedAt":"…" }],
  "headCommit":  "…", "stale": false,
  "truncated":   false, "totalCount": 9039,
  "data":        T
}
```

## 2. Các graph cần có

### 2.1 Architecture graph — "code đang chia thành những khối nào" (cấp cụm)
```
ArchitectureGraph { nodes: ClusterNode[], edges: ClusterEdge[] }
ClusterNode { id, label, symbolCount, cohesion, keywords[], topFiles[], dominantLanguage, area? }
ClusterEdge { from, to, weight, kinds: {CALLS:n, IMPORTS:n,…} }
```
Nguồn: GitNexus `Community` + `MEMBER_OF`, cạnh tổng hợp từ `CALLS/IMPORTS`. `area` (frontend/backend-go/agent/…) suy từ tiền tố đường dẫn đa số, cho phép tô màu theo khu vực.

### 2.2 Module/file graph — kiểm soát phụ thuộc
```
ModuleGraph { nodes: ModuleNode[], edges: ModuleEdge[] }
ModuleNode { id(path), kind:"folder|file", language, symbolCount, loc?, cluster }
ModuleEdge { from, to, kind:"IMPORTS|CONTAINS", count }
```
Nguồn: GitNexus `Folder/File`, `IMPORTS/CONTAINS`; CodeGraph `file`, `imports`, `contains`. Dùng để phát hiện vòng phụ thuộc, vi phạm biên giữa service.

### 2.3 Symbol/call graph — quanh một symbol
```
SymbolGraph { center: SymbolRef, nodes: SymbolNode[], edges: SymbolEdge[], depth }
SymbolNode { id, kind, name, qualifiedName, filePath, startLine, endLine, isExported, language, cluster?, signature? }
SymbolEdge { from, to, kind:"CALLS|IMPORTS|ACCESSES|EXTENDS|IMPLEMENTS|METHOD_OVERRIDES|METHOD_IMPLEMENTS|HAS_METHOD|HAS_PROPERTY|REFERENCES|INSTANTIATES",
             confidence?, reason?, line? }
```
`confidence/reason` có ở GitNexus (0.85 `local-call`…), UI hiển thị cạnh yếu bằng nét đứt.

### 2.4 Execution flow — luồng thực thi
```
FlowSummary { id, label, processType:"cross_community|intra_community", stepCount, communities[], entry:SymbolRef, terminal:SymbolRef }
FlowGraph   { flow: FlowSummary, steps: [{ step:int, symbol:SymbolRef, cluster, filePath, startLine }], edges: SymbolEdge[] }
```
Nguồn: GitNexus `Process`, `STEP_IN_PROCESS(step)`, `ENTRY_POINT_OF`. 300 luồng/Orca nên liệt kê được toàn bộ (phân trang).

### 2.5 Impact graph — blast radius
```
ImpactGraph { target: SymbolRef, direction:"upstream|downstream", risk:"LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN",
              levels: [{ depth:1, symbols: ImpactSymbol[] }], affectedFlows: FlowSummary[], affectedClusters: ClusterRef[], testsCovering: SymbolRef[] }
ImpactSymbol { symbol: SymbolRef, via: EdgeKind, direct: bool }
```
Nguồn: `gitnexus impact`; `codegraph impact/affected` làm đối chứng và để tìm test bị ảnh hưởng. Có trạng thái `ambiguous` → UI mở chọn symbol.

### 2.6 API/route map — hợp đồng giữa tầng
```
RouteMap { routes: RouteNode[], edges: [{ route, handler: SymbolRef, kind:"HANDLES_ROUTE|FETCHES" }] }
RouteNode { id, path, method?, filePath, middleware[], responseKeys[], errorKeys[], side:"server|client" }
```
Nguồn: GitNexus `Route`, `HANDLES_ROUTE`, `FETCHES`; CodeGraph `route`. Hữu ích để kiểm soát UI → gateway → service.

### 2.7 Change-impact overlay — kiểm soát thay đổi
```
ChangeOverlay { base, head, changedFiles[], changedSymbols: SymbolRef[], affectedFlows: FlowSummary[], affectedClusters: ClusterRef[], risk }
```
Nguồn: `gitnexus detect-changes` / diff từ `git.*` của agent; phủ lên bất kỳ đồ thị nào ở trên để đánh dấu nút "đã đổi" và "bị ảnh hưởng". Đây là phần phục vụ mục tiêu "kiểm soát code".

### 2.8 Index health
```
IndexStatus { tool, version, repo, state:"missing|building|ready|stale", indexedCommit, headCommit, indexedAt,
              stats{files,nodes,edges,communities?,processes?}, pendingChanges?{added,modified,removed}, languages[] }
```

## 3. Tham chiếu và hợp nhất id

```
SymbolRef { key, kind, name, qualifiedName?, filePath, startLine?, endLine?, gitnexusId?, codegraphId? }
```
- `key` (chuẩn, ổn định giữa hai công cụ): `"<kind>:<filePath>:<qualifiedName||name>"` (kind chuẩn hoá về chữ thường; GitNexus `Function/Method/Const/...` ↔ CodeGraph `function/method/constant/...`).
- `gitnexusId` ví dụ `Function:backend/src/main/gitea/pull-request-mappers.ts:mapGiteaPullRequestState`; `codegraphId` ví dụ `struct:ce4b2dcbf8f4bf86…` (hash, không ổn định giữa lần index → không dùng làm khoá lâu dài).
- Khi hai nguồn trả cùng `key`, backend giữ một nút và ghi cả hai id.
- Bảng ánh xạ kind:

| Chuẩn | GitNexus | CodeGraph |
|---|---|---|
| function | Function | function |
| method | Method, Constructor | method |
| type | Struct, Class, Interface, Enum | struct, class, interface, enum, type_alias |
| value | Const, Variable, Property | constant, variable, property, field |
| file/folder | File, Folder | file |
| route | Route | route |
| component | — | component |
| cluster | Community | — |
| flow | Process | — |
| doc | Section | — |

Đường dẫn trong dữ liệu là tương đối gốc repo trên dev server; UI hiển thị theo worktree đã mở, chuyển đổi tại backend (lưu ý WSL/Windows/SSH — xem README).

## 4. Dùng cho từng mục tiêu "kiểm soát code"

| Câu hỏi | View | Nguồn |
|---|---|---|
| Hệ thống chia khối nào, khối nào phình to / kém gắn kết? | Architecture | Community (`cohesion`, `symbolCount`) |
| Service A có đang import service B trái luật không? | Module | IMPORTS, tiền tố đường dẫn |
| Sửa hàm X thì ảnh hưởng gì? | Impact | `impact` + `affectedFlows` |
| Luồng đăng nhập chạy qua những file nào? | Flow | Process/STEP_IN_PROCESS |
| API nào do hàm nào xử lý, UI nào gọi? | Route | Route/HANDLES_ROUTE/FETCHES |
| PR này chạm vào đâu, rủi ro bao nhiêu? | ChangeOverlay | detect-changes |
| Dữ liệu này tươi đến đâu? | IndexStatus | meta.json, `pendingChanges` |

## 5. Gợi ý cho giao diện (theo `docs/STYLEGUIDE.md`)

- React Flow cho Architecture/Flow/Impact (số nút nhỏ); Cytoscape.js hoặc Sigma.js cho subgraph lớn.
- Tô màu theo `area`/`language`, độ dày cạnh theo `weight`, nét đứt cho `confidence < 0.8`.
- Panel bên phải cho symbol: tên, file:line, chữ ký, mã nguồn (từ `codeintel.symbol`), nút "mở trong editor/worktree".
- Hiển thị luôn `indexedAt` và `stale`; nút "Làm mới index".
