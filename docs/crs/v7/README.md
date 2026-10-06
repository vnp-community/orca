# Change Requests v7 — Xem code (Code Intelligence & Review View)

> **Mục tiêu:** lấy dữ liệu từ GitNexus và CodeGraph đang chạy trên dev server (MCP/CLI local), cộng thêm dữ liệu từ file của repo (SQL migration, proto, compose, cấu hình), chuẩn hoá thành một mô hình đồ thị chung, rồi hiển thị trên giao diện Orca để **review kết quả sau khi agent code**: cấu trúc code, kiến trúc C4 level 3, luồng dữ liệu, ERD, kiến trúc lưu trữ, và các view review nhanh (ảnh hưởng thay đổi, vi phạm lớp, hotspot, khác biệt hợp đồng, khoảng trống test, thứ tự đọc).
>
> **Phạm vi:** `agent/src/relay` (nhóm RPC `codeintel.*`); service Go mới `backend-go/services/code-intel-service`; thay đổi nhỏ ở `infra-fleet-service`, `api-gateway`, (tuỳ chọn) `mcp-service`; frontend `frontend/src/renderer/src`.
>
> **Nguồn:** bộ nghiên cứu [`docs/research/view-code/`](../../research/view-code/README.md): [01 kết nối agent–backend](../../research/view-code/01-agent-backend-connection.md), [02 tương tác MCP local](../../research/view-code/02-local-mcp-interaction.md), [03 luồng lệnh/dữ liệu](../../research/view-code/03-command-and-data-flow.md), [04 dữ liệu thô và pipeline](../../research/view-code/04-raw-data-and-pipeline.md), [05 schema graph](../../research/view-code/05-graph-schemas.md), [06 khoảng trống và lộ trình](../../research/view-code/06-gaps-risks-roadmap.md), [07 quyết định kiến trúc](../../research/view-code/07-architecture-decisions.md), [08 các view](../../research/view-code/08-views-and-review-models.md), [09 nguồn dữ liệu ngoài](../../research/view-code/09-external-inputs-required.md), [10 UX frontend](../../research/view-code/10-frontend-review-ux.md).

> **Trạng thái toàn series:** 📝 Đề xuất, chưa triển khai. Các CR được viết từ khảo sát code và chạy thử CLI ngày 2026-10-05; chưa chạy hệ thống, chưa viết code. Người triển khai phải đọc lại code liên quan trước khi sửa. Nội dung nghiên cứu trong `docs/research/view-code/` cũng là đề xuất, không phải hiện trạng.

## 1. Hiện trạng đã xác nhận (khảo sát 2026-10-05)

| Hạng mục | Thật sự có gì | Bằng chứng |
|---|---|---|
| Kết nối agent → backend | `direct-websocket`: agent dial `wss://<gateway>/agent`, JSON-RPC hai chiều trên cùng WS; Go nhận ở `infra-fleet-service/internal/adapter/agentwsserver`, quản phiên ở `adapter/devserveragent` | `specs/agent/api/connection-modes.md`, code Go |
| Đường lệnh backend → agent | `InfraFleetService.Relay / RelayByDevServer / RelayStream` chuyển `method + params_json` nguyên văn; mặc định timeout 30 s, ngoại lệ `agent.execPrompt` (`execTimeoutForMethod`) | `usecase/relay*.go`, `devserveragent/client.go` |
| Agent có tool `gitnexus` / `codegraph` | CLI thô qua `tools/call` (`args` tự do, không `-r`, `timeout 60 s`, stdout không giới hạn); chỉ Part A (`agent-rpc-dispatch-misc.ts`); Part B (`relay.ts`, `relay-ssh`) không có `tools/*`; backend Go chưa gọi `tools/call` ở đâu | `agent-tool-registry.ts` |
| Handshake mang danh sách tool | `agent.handshake` gửi `tools[]` (từ `discoverTools`) và `capabilities[]` | `agent-session.ts` |
| GitNexus (1.6.9) | Graph LadybugDB trong `<repo>/.gitnexus/`, registry toàn cục nhiều repo (12 repo trên máy khảo sát); **thiếu `-r` thì lỗi "Multiple repositories indexed"**; `cypher` trả `{markdown, row_count}`, không phải JSON theo dòng; `context`, `impact` trả JSON (có trạng thái `ambiguous`); ~1,8 s mỗi lần gọi CLI | chạy CLI trên repo Orca |
| CodeGraph (1.4.1) | SQLite `.codegraph/codegraph.db` (~1,2 GB cho Orca; bảng `nodes`, `edges`, `files`, `unresolved_refs`, `project_metadata`, `schema_versions`); CLI có `--json` cho `query`, `callers`, `callees`, `status`, `files`; `explore`/`node` chỉ trả text | chạy CLI, đọc schema SQLite |
| Số liệu repo Orca | GitNexus 247 556 node, 644 157 edge, 9 039 `Community`, 300 `Process`, 92 `Route`; CodeGraph 295 761 node, 958 441 edge, 15 759 file | `.gitnexus/meta.json`, `codegraph status --json` |
| Dữ liệu cho ERD | 17 service có thư mục `migrations/` (588 file `.sql`); mỗi service sở hữu schema riêng; `infra-fleet-service` có cả `migrations/postgres` và `migrations/mysql` | `backend-go/services/*/migrations` |
| Bố cục service Go | hexagonal `internal/{domain,usecase,adapter,config}`; `infra-fleet-service/internal/adapter` có `postgres`, `mysql`, `eventbus`, `grpc`, `grpcclient`, `agentwsserver`, `devserveragent`, `sshrelay`… | cây thư mục |
| Gateway cho frontend | `api-gateway/internal/adapter/wscompat`: `Registry.Register(channel, handler)`, `StreamHandler → PushEvent`; có `channels_*.go` theo nhóm | code |
| Frontend | React 19, Zustand 5, `@xyflow/react` 12, `mermaid` 11, `monaco-editor`, `@tanstack/react-virtual`, `react-resizable-panels`; có `CombinedDiffViewer`, `diff-comments` + `DiffNotesSendMenu`, `SourceControl`, `ChecksPanel`, `MermaidBlock`, `DAGPreview`; `components/code-review/*` là code chết; hai render target (Electron `window.api`, web `web-preload-api`) | `frontend/package.json`, `components/**` |
| Chưa có | `code-intel-service`, proto `orca.codeintel.v1`, nhóm RPC `codeintel.*` trên agent, kênh `codeIntel.*` ở gateway, bất kỳ UI nào cho review đồ thị | grep toàn repo |

## 2. Quyết định đã đưa ra (nguồn: [07](../../research/view-code/07-architecture-decisions.md), 2026-10-05)

| # | Quyết định | Hệ quả |
|---|---|---|
| D1 | **Dùng lại agent TypeScript**, thêm nhóm `codeintel.*`; không viết agent Go mới | `agent-codeintel` |
| D2 | **Agent chủ động kết nối ra backend** (`direct-websocket`); tính năng chỉ hỗ trợ mode này ở MVP; `relay-websocket` và `relay-ssh` ngoài phạm vi (CR-CV-006 là P2 cho `relay-ssh`) | Không mở cổng vào trên dev server |
| D3 | **`code-intel-service` Go mới** xử lý, chuẩn hoá, cache, phân quyền; `infra-fleet-service` giữ vai trò vận chuyển | `code-intel-service-foundation` |
| D4 | **Kéo theo yêu cầu + đẩy nhẹ**: UI mở view thì backend gọi agent; agent chủ động báo `codeintel.indexChanged`, backend huỷ cache và báo UI. Không đẩy cả graph | CR-CV-004, 023, 024 |
| D5 | Agent chỉ mở **method hẹp** `codeintel.*` với whitelist lệnh, không mở `args` tự do | CR-CV-001 |
| D6 | Các view C4, ERD, lưu trữ, hợp đồng đọc **file nhỏ** (SQL, proto, compose, thư mục) qua RPC `fs.*`/`git.*` đã có và parse ở `code-intel-service`; chỉ cấu trúc và luồng cần `codeintel.*` | `code-intel-sources` |
| D7 | Giao diện là **tab `review` trong tab group**, right sidebar chỉ làm lối vào và tóm tắt | `review-frontend` |

### Mặc định đề xuất cho các điểm còn mở (cần xác nhận khi duyệt)

| # | Điểm mở | Mặc định trong series này |
|---|---|---|
| O1 | Hai dialect DB cho `code-intel-service` | **Postgres và MySQL** theo quy ước series v6 (`common/dbcapability`); nếu bị loại thì sửa CR-CV-010, 011 |
| O2 | Có đi qua chính sách MCP của Orca không | **Không ở MVP**; CR-CV-041 (P2) mở tuỳ chọn |
| O3 | Ai làm mới index | **Người dùng bấm** qua `codeintel.reindex` (có quyền, có tiến trình); không chạy tự động theo lịch |
| O4 | Quy tắc ánh xạ project/worktree → repo GitNexus | Khớp **đường dẫn đã đăng ký** trong registry GitNexus của dev server (không nhận tên repo tự do từ client); chi tiết ở CR-CV-001 và 012 |
| O5 | Thư viện bố cục/treemap mới (`elkjs`/`dagre`, `d3-*`) | **Không thêm** ở MVP: bố cục tầng đơn giản + treemap SVG tự viết; thêm thư viện cần duyệt riêng |
| O6 | Nơi lưu trạng thái review (đã xem, ghi chú) | **Backend (`code-intel-service`)** theo `(worktree, commit)`; frontend chỉ cache; đối chiếu `specs/frontend/storage/*` khi làm CR-CV-052 |
| O7 | Phạm vi review mặc định | Thay đổi của worktree **so với nhánh gốc** (merge-base) |
| O8 | Feature flag | `code_intel_enabled`, **theo tenant**, mặc định tắt |
| O9 | Cổng chất lượng (từ [research 11](../../research/view-code/11-additions-for-quality-control.md)) | **Chế độ chỉ báo** (không chặn Create PR/commit); chặn cứng cần quyết định riêng. Cờ phụ `quality_gate_enabled`, theo tenant, mặc định tắt, chỉ có hiệu lực khi `code_intel_enabled` bật |
| O10 | Index cho worktree của agent | **Đã chỉnh theo CR-CV-080 (xem mục 8, điểm 21):** `codegraph sync` chỉ đúng khi gốc index trùng `workspaceRoot`; ở worktree liên kết dùng **overlay theo diff** trên index của checkout chính (`indexScope=repo_root`) và `gitnexus analyze --index-only` ở nền khi cần. Index riêng từng worktree tốn ~4 GB mỗi worktree (đo bằng `du`), chỉ làm sau khi có quyết định. Tự kích hoạt khi agent báo xong (đổi mặc định O3 cho worktree của agent) |
| O11 | Kiểm tra chạy trên dev server | Chỉ **profile có tên** cấu hình sẵn (không nhận lệnh tuỳ ý), có timeout/giới hạn tài nguyên, không truyền biến môi trường chứa secret; người kích hoạt cần quyền ghi trên project |
| O12 | Công cụ/phụ thuộc mới (coverage, quét bảo mật, thư viện biểu đồ/bố cục) | **Không thêm mặc định**; mỗi cái cần duyệt riêng. Quét bảo mật/phụ thuộc mặc định tắt |
| O13 | Đánh giá bằng AI | **Tắt** mặc định; khi bật chỉ dùng đường `ai.complete` của agent, gắn nhãn "AI suy luận", cấm gửi secret, không dùng làm căn cứ chặn |
| O14 | Nơi lưu kết quả chất lượng | Trong `code-intel-service` (không tách service mới ở series này); nếu vòng đời khác nhiều sẽ tách sau |

## 3. Mô hình chung (hợp đồng giữa các CR)

Mọi CR trong series phải dùng đúng tên và giá trị dưới đây. Nếu cần đổi, sửa tại đây trước. Khi CR và README khác nhau, theo CR (xem mục 8).

### 3.1 Kiến trúc

```
Dev server                                    Backend                                         Frontend
┌─────────────────────────┐   WS /agent   ┌──────────────────────────────┐   gRPC   ┌────────────────────────┐  WS /ws   ┌───────────────┐
│ agent (TS, Part A)      │◀─────────────▶│ infra-fleet-service          │◀────────▶│ code-intel-service     │◀─────────▶│ api-gateway   │──▶ UI
│  codeintel.* (whitelist)│  JSON-RPC 2   │  Relay/RelayByDevServer      │          │  chuẩn hoá, cache,     │  wscompat │ codeIntel.*   │   tab Review
│  gitnexus / codegraph   │  hai chiều    │  StreamCodeIntelEvents (mới) │          │  phân quyền, lưu review│           └───────────────┘
│  fs.* / git.* (đã có)   │               └──────────────────────────────┘          └────────────────────────┘
└─────────────────────────┘
```

### 3.2 Nhóm RPC trên agent (`codeintel.*`, Part A)

Mọi method nhận `workspaceRoot` (đường dẫn tuyệt đối thư mục repo/worktree trên dev server, đã nằm trong workspace root đăng ký) và **không** nhận `args`, tên lệnh CLI hay tên repo tự do. Mọi kết quả có phần đầu chung:

```jsonc
{ "sources": [{ "tool": "gitnexus|codegraph", "version": "…", "indexedAt": "…", "commit": "…" }],
  "headCommit": "…", "stale": false, "truncated": false, "totalCount": 0, "data": { … } }
```

| Method | Mục đích | Tham số chính | Giới hạn mặc định |
|---|---|---|---|
| `codeintel.status` | Công cụ có sẵn không, index tới đâu | — | — |
| `codeintel.overview` | Cụm (community) và cạnh tổng hợp giữa cụm | `topN` | ≤ 500 cụm, ≤ 5 000 cạnh |
| `codeintel.processes` | Danh sách luồng thực thi | `limit`, `offset` | ≤ 100/trang |
| `codeintel.process` | Các bước của một luồng | `processId` | ≤ 200 bước |
| `codeintel.subgraph` | Nút + cạnh quanh một cụm/tệp/symbol | `center`, `depth ≤ 3`, `kinds`, `limit` | ≤ 1 500 nút, ≤ 4 000 cạnh |
| `codeintel.impact` | Blast radius | `target`, `direction`, `depth` | ≤ 300 nút |
| `codeintel.symbol` | Chi tiết và mã nguồn một symbol | `uid` hoặc `name`+`file` | ≤ 200 KiB |
| `codeintel.routes` | Route → handler | `limit` | — |
| `codeintel.detectChanges` | Ánh xạ diff → symbol, luồng bị ảnh hưởng | `base`, `head?` | ≤ 2 000 symbol |
| `codeintel.reindex` | Làm mới index (nền, có tiến trình) | `mode` | một lần/tại một thời điểm |

Thông báo agent → backend: `codeintel.indexChanged {workspaceRoot, tool, commit, indexedAt}`, `codeintel.reindexProgress {jobId, stage, percent, message}`.

### 3.3 Mã lỗi chuẩn (`error.data.code`, chuỗi)

`CODEINTEL_TOOL_UNAVAILABLE`, `CODEINTEL_INDEX_MISSING`, `CODEINTEL_REPO_NOT_REGISTERED`, `CODEINTEL_PATH_NOT_ALLOWED`, `CODEINTEL_INVALID_PARAMS`, `CODEINTEL_AMBIGUOUS_SYMBOL` (kèm `candidates`), `CODEINTEL_TIMEOUT`, `CODEINTEL_REINDEX_IN_PROGRESS`, `CODEINTEL_OUTPUT_TOO_LARGE`, `CODEINTEL_TOOL_FAILED`. `stale` và `truncated` là cờ trong kết quả, không phải lỗi. Phía Go dùng tiền tố `CODEINTEL_` trong `apperrors`.

### 3.4 Schema chuẩn và quy tắc id

Các kiểu dữ liệu giữ đúng tên ở [05](../../research/view-code/05-graph-schemas.md) và [08](../../research/view-code/08-views-and-review-models.md): `CodeIntelResult<T>`, `ArchitectureGraph`, `ModuleGraph`, `SymbolGraph`, `FlowSummary`/`FlowGraph`, `ImpactGraph`, `RouteMap`, `ChangeOverlay` (mở rộng `touchedTables[]`, `touchedContracts[]`, `uncoveredSymbols[]`, `violations[]`, `readingOrder[]`), `IndexStatus`, `C4ComponentView`, `DataFlow`, `ErdModel`, `StorageMap`, `SymbolRef`.

- `SymbolRef.key = "<kind>:<filePath>:<qualifiedName||name>"`, kind chuẩn hoá chữ thường theo bảng ánh xạ ở 05 §3. `gitnexusId` và `codegraphId` chỉ là thông tin kèm; `codegraphId` (hash) không ổn định giữa lần index, không dùng làm khoá lâu dài.
- Đường dẫn trong dữ liệu là tương đối gốc repo; chuyển sang đường dẫn worktree ở backend/UI.
- Định nghĩa proto là nguồn sự thật cho schema; TypeScript phía frontend sinh/mirror từ đó (CR-CV-050).

### 3.5 Mô hình dữ liệu của `code-intel-service` (schema/DB `codeintel`)

| Bảng | Trường chính |
|---|---|
| `repo_bindings` | `id`, `tenant_id`, `project_id`, `worktree_id`, `dev_server_id`, `workspace_root`, `gitnexus_repo`, `codegraph_path`, `created_at`, `updated_at` |
| `graph_snapshots` | `id`, `tenant_id`, `repo_binding_id`, `view`, `commit`, `params_hash`, `payload` (JSON, có giới hạn), `payload_bytes`, `truncated`, `tool_versions` (JSON), `created_at`, `expires_at` |
| `review_states` | `id`, `tenant_id`, `worktree_id`, `base_commit`, `head_commit`, `reading_progress` (JSON), `notes` (JSON), `status`, `updated_by`, `updated_at`, `version` |
| `finding_dismissals` | `id`, `tenant_id`, `repo_binding_id`, `finding_key`, `reason`, `dismissed_by`, `at` |
| `c4_overrides` | `id`, `tenant_id`, `repo_binding_id`, `container`, `document` (JSON/YAML text), `updated_by`, `updated_at`, `version` |
| `reindex_jobs` | `id`, `tenant_id`, `repo_binding_id`, `mode`, `status`, `stage`, `started_at`, `finished_at`, `error_code` |
| `outbox_events`, `processed_events` | theo `common/outbox` (như `request-service` của v6) |

Không FK chéo service: `project_id`, `worktree_id`, `dev_server_id` là id tham chiếu. Postgres bật RLS theo `tenant_id`; MySQL kiểm tra ở tầng ứng dụng.

### 3.6 gRPC `orca.codeintel.v1.CodeIntelService` (proto mới, `backend-go/proto/orca/codeintel/v1`)

`GetIndexStatus`, `RequestReindex`, `GetReindexJob`, `GetStructure`, `GetArchitecture`, `ListDataFlows`, `GetDataFlow`, `GetErd`, `GetStorageMap`, `GetSubgraph`, `GetImpact`, `GetSymbol`, `GetRouteMap`, `GetChangeOverlay`, `GetReadingOrder`, `ListFindings`, `DismissFinding`, `GetContractDiff`, `GetReviewState`, `SaveReviewState`, `GetC4Overrides`, `SaveC4Overrides`, `BindRepo`, `ListRepoBindings`, `StreamCodeIntelEvents`. Message do CR sở hữu từng RPC định nghĩa; không khai báo RPC chưa có message (xem v6 CR-REQ-001).

Thêm vào `orca.infrafleet.v1.InfraFleetService`: `StreamCodeIntelEvents` (server-streaming; mẫu `StreamFileChanges`).

### 3.7 Kênh WS (`api-gateway/wscompat`) và push

`codeIntel.status`, `codeIntel.reindex`, `codeIntel.reindexStatus`, `codeIntel.structure`, `codeIntel.architecture`, `codeIntel.dataFlows`, `codeIntel.dataFlow`, `codeIntel.erd`, `codeIntel.storage`, `codeIntel.subgraph`, `codeIntel.impact`, `codeIntel.symbol`, `codeIntel.routes`, `codeIntel.changeOverlay`, `codeIntel.readingOrder`, `codeIntel.findings`, `codeIntel.dismissFinding`, `codeIntel.contractDiff`, `codeIntel.reviewState.get`, `codeIntel.reviewState.save`, `codeIntel.c4.get`, `codeIntel.c4.save`, `codeIntel.bindRepo`; push: `codeIntel.changed`, `codeIntel.reindexProgress`. Tên chốt cuối ở CR-CV-040.

### 3.8 Sự kiện (outbox, subject `orca.codeintel.<entity>.<event>`)

`orca.codeintel.index.changed`, `orca.codeintel.reindex.started`, `orca.codeintel.reindex.finished`, `orca.codeintel.review.saved`.

### 3.9 Các view và nguồn dữ liệu

| View | Nguồn | Nơi dựng |
|---|---|---|
| Cấu trúc code | GitNexus `Folder/File/CONTAINS`, CodeGraph `files` | `code-intel-service` từ `codeintel.subgraph`/`status` |
| Kiến trúc C4 L3 | Cấu trúc thư mục hexagonal + `IMPLEMENTS` + proto + `c4.yaml` | `code-intel-service` (CR-CV-033) |
| Luồng dữ liệu | GitNexus `Process` + nối proto client↔server + kênh `wscompat` | `code-intel-service` (CR-CV-034) |
| ERD | `migrations/*.sql` | `code-intel-service` (CR-CV-031) |
| Lưu trữ | compose, cấu hình, adapter, migration | `code-intel-service` (CR-CV-035) |
| Ảnh hưởng, thứ tự đọc, test gap | `codeintel.detectChanges` + `impact` + diff `git.*` | `code-intel-service` (CR-CV-036) |
| Vi phạm lớp, hotspot, owner, dead code, contract diff, bảo mật tĩnh | graph + `git log` + proto + SQL | `code-intel-service` (CR-CV-037, 038) |

### 3.10 Hợp đồng kiểm soát chất lượng (CR-CV-080 trở đi)

**Agent** (Part A), nhóm `quality.*`: `quality.listProfiles`, `quality.run {workspaceRoot, profile, scope: "worktree"|"changed"|"commitRange", base?}` → `{runId}`, `quality.runStatus {runId}`, `quality.cancel {runId}`, `quality.results {runId, offset, limit}`, `quality.coverage {runId}`; thông báo agent → backend: `quality.progress {runId, stage, percent|null, message}`, `quality.finished {runId, status, summary}`. Chỉ nhận **tên profile**, không nhận lệnh. Mã lỗi thêm: `CODEINTEL_PROFILE_UNKNOWN`, `CODEINTEL_ENV_NOT_READY` (thiếu dependency/công cụ), `CODEINTEL_RUN_IN_PROGRESS`, `CODEINTEL_RUN_CANCELLED`.

**Mô hình** (proto trong `orca.codeintel.v1`, chủ sở hữu CR-CV-082):
```jsonc
QualityFinding { "fingerprint": "…", "ruleId": "…", "severity": "error|warning|info",
  "category": "lint|typecheck|test|coverage|complexity|security|dependency|convention|architecture|ai",
  "file": "…", "line": 0, "endLine": 0, "message": "…", "tool": "oxlint", "toolVersion": "…", "fixHint": "…?" }
QualityRun { "id", "worktreeId", "headCommit", "indexCommit", "scope", "profile", "status": "queued|running|succeeded|failed|cancelled",
  "source": "local|ci", "startedAt", "finishedAt", "summary": { "error":0, "warning":0, "info":0 } }
QualityGate { "verdict": "pass|warn|fail|unknown", "reasons": [{ "check": "…", "observed": "…", "threshold": "…", "result": "pass|warn|fail" }],
  "mode": "inform|block", "profile": "…", "basedOn": { "runIds": [], "indexCommit": "…", "stale": false } }
```
`fingerprint` ổn định khi dòng đổi (không chứa số dòng). `verdict: unknown` khi thiếu dữ liệu, không bao giờ suy diễn thành `pass`.

**Bảng** (`code-intel-service`): `quality_profiles`, `quality_runs`, `quality_findings`, `quality_waivers` (có người, lý do, hết hạn), `coverage_reports`, `quality_trend_points`, `agent_turns`. Hai dialect, `tenant_id`, không FK chéo service.

**gRPC** (`orca.codeintel.v1.QualityGateService`): `StartQualityRun`, `GetQualityRun`, `ListQualityRuns`, `ListQualityFindings`, `WaiveFinding`, `GetQualityGate`, `GetQualityProfile`, `SaveQualityProfile`, `GetQualityTrend`, `GetCoverage`, `GetRequirementTrace`, `GenerateReviewSummary`, `ExportReviewReport`.

**Kênh WS**: `codeIntel.quality.start|run|runs|findings|waive|gate|profile.get|profile.save|trend|coverage|trace|summary|report`; push: `codeIntel.quality.progress`, `codeIntel.quality.finished`.

**Sự kiện**: `orca.codeintel.quality.run_finished`, `orca.codeintel.quality.gate_changed`.

## 4. Danh sách feature và CR

| Feature (folder) | CR | Nội dung | Priority | Effort |
|---|---|---|---|---|
| [`agent-codeintel`](./agent-codeintel/README.md) | CR-CV-001 | Nền `codeintel.*` trên agent: đăng ký method, whitelist, phân giải repo, giới hạn, phát hiện công cụ, mã lỗi | 🔴 P0 | Large |
| | CR-CV-002 | Trích xuất GitNexus: status, overview, processes, process, subgraph, impact, symbol, routes | 🔴 P0 | Large |
| | CR-CV-003 | Trích xuất CodeGraph: status, query, callers, callees, files, node; ánh xạ id/kind; đọc SQLite read-only (tuỳ chọn) | 🟠 P1 | Large |
| | CR-CV-004 | Làm mới index (nền, tiến trình) và thông báo `codeintel.indexChanged` | 🟠 P1 | Medium |
| | CR-CV-005 | `codeintel.detectChanges`: ánh xạ diff → symbol và luồng bị ảnh hưởng | 🔴 P0 | Medium |
| | CR-CV-006 | Hỗ trợ `relay-ssh` (Part B) cho `codeintel.*` | ⚪ P2 | Medium |
| [`code-intel-service-foundation`](./code-intel-service-foundation/README.md) | CR-CV-010 | Dựng `code-intel-service` (module, proto, DB hai dialect, outbox, wiring, CI) | 🔴 P0 | Medium |
| | CR-CV-011 | Mô hình dữ liệu, migration, repository | 🔴 P0 | Medium |
| | CR-CV-012 | Ánh xạ project/worktree → dev server → repo; tổng hợp trạng thái index | 🔴 P0 | Medium |
| | CR-CV-013 | Phân quyền, audit, hạn mức đồng thời | 🔴 P0 | Medium |
| [`code-intel-graph-pipeline`](./code-intel-graph-pipeline/README.md) | CR-CV-020 | Mô hình graph chuẩn (proto + domain), khoá `SymbolRef`, ánh xạ kind, hợp nhất hai nguồn | 🔴 P0 | Large |
| | CR-CV-021 | Collector: gọi agent qua `RelayByDevServer`, điều phối truy vấn, ánh xạ lỗi, cắt/phân trang | 🔴 P0 | Large |
| | CR-CV-022 | Cache snapshot theo `(repo, commit, view, params)`, độ cũ, singleflight, dung lượng | 🟠 P1 | Medium |
| | CR-CV-023 | `infra-fleet-service`: timeout `codeintel.*`, nhận thông báo từ agent, `StreamCodeIntelEvents`, công bố `tools[]` | 🔴 P0 | Medium |
| | CR-CV-024 | Phân phối sự kiện: huỷ cache, outbox, đẩy lên gateway | 🟠 P1 | Medium |
| [`code-intel-sources`](./code-intel-sources/README.md) | CR-CV-030 | Cổng đọc file repo qua `fs.*`/`git.*` (giới hạn, an toàn đường dẫn) | 🔴 P0 | Medium |
| | CR-CV-031 | Parse SQL migration → ERD (Postgres, MySQL), liên kết bảng ↔ code | 🔴 P0 | Large |
| | CR-CV-032 | Trích xuất proto, nối client↔server, danh mục kênh `wscompat` | 🟠 P1 | Large |
| | CR-CV-033 | C4 level 3: quy tắc hexagonal + `c4.yaml` ghi đè | 🟠 P1 | Large |
| | CR-CV-034 | Luồng dữ liệu: `Process` + nối qua ranh giới service → sequence/DFD | 🟠 P1 | Large |
| | CR-CV-035 | Bản đồ lưu trữ từ compose, cấu hình, adapter, topic (không lộ secret) | ⚪ P2 | Large |
| | CR-CV-036 | Change overlay, thứ tự đọc, khoảng trống test, chấm rủi ro | 🔴 P0 | Large |
| | CR-CV-037 | Phân tích cấu trúc: vi phạm lớp, vòng phụ thuộc, hotspot, mã chết, owner | 🟠 P1 | Large |
| | CR-CV-038 | Khác biệt hợp đồng/schema và kiểm tra bảo mật tĩnh (`tenant_id`) | ⚪ P2 | Large |
| [`code-intel-gateway`](./code-intel-gateway/README.md) | CR-CV-040 | Kênh `codeIntel.*` ở `api-gateway`, push, giới hạn kích thước, parity MCP | 🔴 P0 | Large |
| | CR-CV-041 | (Tuỳ chọn) Tool MCP `codeintel_*` qua chính sách `mcp-service` | ⚪ P2 | Medium |
| [`review-frontend`](./review-frontend/README.md) | CR-CV-050 | Nền frontend: kiểu, `window.api.codeIntel`, slice, i18n, tab `review`, cờ | 🔴 P0 | Large |
| | CR-CV-051 | Khung màn Review: bố cục, thanh tóm tắt, phạm vi, chip index, trạng thái/lỗi | 🔴 P0 | Large |
| | CR-CV-052 | Thứ tự đọc, tiến độ review, phím tắt, lưu trạng thái | 🔴 P0 | Medium |
| | CR-CV-053 | Lens Ảnh hưởng + panel chi tiết symbol + liên kết hai chiều với diff | 🔴 P0 | Large |
| | CR-CV-054 | Lens Cấu trúc (treemap SVG + cây ảo hoá) | 🟠 P1 | Medium |
| | CR-CV-055 | Lens Kiến trúc C4 (+ chỉnh `c4.yaml`) | 🟠 P1 | Large |
| | CR-CV-056 | Lens Luồng dữ liệu (Mermaid) | 🟠 P1 | Medium |
| | CR-CV-057 | Lens ERD | 🔴 P0 | Large |
| | CR-CV-058 | Lens Lưu trữ | ⚪ P2 | Medium |
| | CR-CV-059 | Lens Hợp đồng + danh sách Phát hiện | 🟠 P1 | Large |
| | CR-CV-060 | Ghi chú → gửi lại agent, so sánh với lượt trước | 🟠 P1 | Large |
| | CR-CV-061 | Điểm vào: dashboard, Source Control, Cmd+K, tab right sidebar | 🔴 P0 | Medium |
| | CR-CV-062 | Mobile: màn tóm tắt chỉ đọc | ⚪ P2 | Medium |
| [`quality-rollout`](./quality-rollout/README.md) | CR-CV-070 | Fixture vàng và kiểm thử hợp đồng với phiên bản công cụ | 🔴 P0 | Medium |
| | CR-CV-071 | Ngân sách hiệu năng, metrics, tracing | 🟠 P1 | Medium |
| | CR-CV-072 | Kiểm thử bảo mật: whitelist, đường dẫn, che secret, cô lập tenant | 🔴 P0 | Medium |
| | CR-CV-073 | E2E, feature flag, rollout, tài liệu vận hành | 🔴 P0 | Medium |
| [`quality-signals`](./quality-signals/README.md) (bổ sung theo [research 11](../../research/view-code/11-additions-for-quality-control.md)) | CR-CV-080 | Chiến lược index cho worktree của agent và tự làm mới khi agent xong | 🔴 P0 | Large |
| | CR-CV-081 | Bộ chạy kiểm tra trên agent: profile có tên, tiến độ, huỷ, giới hạn tài nguyên, sẵn sàng môi trường | 🔴 P0 | Large |
| | CR-CV-082 | Mô hình `QualityFinding` và parser (oxlint, tsc, vitest, go vet/test, golangci-lint, buf, opa) kèm fixture | 🔴 P0 | Large |
| | CR-CV-083 | Thu thập coverage và diff coverage | 🟠 P1 | Large |
| | CR-CV-084 | Rule pack quy ước dự án từ `config/scripts/check-*` và AGENTS.md | 🟠 P1 | Large |
| | CR-CV-086 | Gộp kết quả CI/PR (GitHub và GitLab) với kết quả cục bộ | 🟠 P1 | Large |
| | CR-CV-091 | Quét bảo mật/phụ thuộc (tuỳ chọn): lỗ hổng, bí mật, diff phụ thuộc/giấy phép | ⚪ P2 | Medium |
| | CR-CV-094 | Đánh giá các tool GitNexus chưa dùng (`check`, `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query`, `group_*`) | ⚪ P2 | Small |
| [`quality-gate`](./quality-gate/README.md) | CR-CV-085 | Cổng chất lượng: profile, kết luận, lý do, miễn trừ, lịch sử theo lượt | 🔴 P0 | Large |
| | CR-CV-089 | Dấu vết agent (provenance) và đối chiếu "agent tự báo" với kết quả chạy lại | 🟠 P1 | Large |
| | CR-CV-090 | Báo cáo review xuất được (Markdown/HTML, trung lập GitHub/GitLab) | 🟠 P1 | Medium |
| | CR-CV-092 | Truy vết yêu cầu: Request/Task/Plan ↔ code ↔ test | ⚪ P2 | Large |
| | CR-CV-093 | Tóm tắt và đánh giá bằng AI (tuỳ chọn, mặc định tắt) | ⚪ P2 | Medium |
| | CR-CV-095 | Telemetry hiệu quả tính năng review/cổng (chỉ enum/khoảng) | ⚪ P2 | Small |
| [`quality-visualization`](./quality-visualization/README.md) | CR-CV-087 | Frontend chất lượng: scorecard cổng, chú thích phát hiện trên diff, biểu đồ xu hướng, hotspot, DSM | 🔴 P0 | Large |
| | CR-CV-088 | Nền đồ hoạ: quyết định thư viện/bố cục, bộ hình chuẩn, token màu, truy cập được, hiệu năng | 🔴 P0 | Large |

## 5. Thứ tự thực thi

```
CR-010 ─▶ CR-011 ─▶ CR-012 ─▶ CR-013
CR-001 ─┬▶ CR-002 ─┬▶ CR-005
        │          └▶ CR-004
        └▶ CR-003
CR-023 (infra-fleet)            ─▶ CR-021 (collector) ─▶ CR-022 ─▶ CR-024
CR-020 (mô hình chuẩn)          ─▶ CR-021
CR-030 ─▶ CR-031 ─▶ CR-032 ─▶ CR-033 ─▶ CR-034 ─▶ CR-035
CR-036 (cần 005, 020, 021, 030) ─▶ CR-037 ─▶ CR-038
CR-040 (sau các RPC của service) ─▶ CR-041
CR-050 ─▶ CR-051 ─▶ CR-052/053 ─▶ CR-054..060 ─▶ CR-061 ─▶ CR-062     (frontend theo sau khi RPC chạy)
CR-070 ngay sau CR-002/003; CR-071, 072, 073 sau khi lõi ổn định

Kiểm soát chất lượng (bổ sung):
CR-080 (index bắt kịp; sau 004, 012) ─┐
CR-081 (bộ chạy kiểm tra, sau 001) ─▶ CR-082 (mô hình + parser) ─┬▶ CR-083 (coverage)
                                                                  ├▶ CR-084 (rule pack)
                                                                  ├▶ CR-086 (gộp CI)
                                                                  └▶ CR-085 (cổng; cần 011, 036, 037) ─▶ CR-089, CR-090, CR-095
CR-088 (nền đồ hoạ; sau 050) ─▶ CR-087 (cần 051, 053, 059, 085)
CR-091, 092, 093, 094: sau khi CR-085 ổn định
```

| Đợt | CR | Kết quả có thể chạy được |
|---|---|---|
| 1 | 010, 011, 001, 002, 020, 023 | Service chạy; agent trả `codeintel.status`/`overview` thật; infra-fleet nhận lệnh |
| 2 | 012, 013, 021, 005, 030, 031, 040 | Gọi được qua gateway; ERD từ migration; thay đổi → symbol |
| 3 | 036, 050, 051, 052, 053, 057, 061 | **MVP review**: tóm tắt thay đổi, thứ tự đọc, lens Ảnh hưởng và ERD, lối vào |
| 4 | 003, 004, 022, 024, 032, 054, 055, 056 | CodeGraph, làm mới index, cache, C4, luồng, cấu trúc |
| 5 | 037, 059, 060, 070, 071, 072 | Phát hiện cấu trúc, hợp đồng, phản hồi lại agent, kiểm thử chất lượng |
| 6 | 006, 035, 038, 041, 058, 062, 073 | `relay-ssh`, lưu trữ, bảo mật tĩnh, MCP, mobile, rollout |
| 7 | 080, 081, 082, 088 | Index bắt kịp code agent; chạy kiểm tra có tiến độ; phát hiện chuẩn hoá; nền đồ hoạ |
| 8 | 085, 084, 083, 086, 087 | Cổng chất lượng chế độ chỉ báo; rule pack; coverage; gộp CI; scorecard và chú thích trên diff |
| 9 | 089, 090, 091, 092, 093, 094, 095 | Dấu vết agent, báo cáo xuất được, quét bảo mật, truy vết yêu cầu, AI, telemetry |

## 6. Quy ước chung cho mọi CR trong series

- **Hai DB:** mọi migration và repository viết cho cả Postgres và MySQL, dùng `common/dbcapability` (mặc định O1).
- **Không FK chéo service**; liên kết bằng id, kiểm tra ở tầng ứng dụng. Tenant: mọi bảng có `tenant_id`, mọi use case gọi `tenant.RequireTenantID`.
- **Agent chỉ chạy lệnh trong whitelist** (D5); không bao giờ nhận lệnh/args tự do từ backend hay UI. Không có đường nào chạy `analyze`, `clean`, `remove`, `publish`, `setup`, `uninstall` ngoài `codeintel.reindex` có kiểm soát.
- **Cypher chỉ dùng mẫu có tham số, chỉ đọc**; từ chối `CREATE|MERGE|DELETE|SET|DROP|CALL` ghi. Cú pháp mỗi mẫu phải được chạy thử trên LadybugDB trước khi chốt.
- **Không bao giờ trả cả graph**: mọi truy vấn có `limit/depth`, trả `truncated` và `totalCount`; ngân sách ở 3.2 và [04 §2](../../research/view-code/04-raw-data-and-pipeline.md).
- **Không đưa secret vào kết quả/cache/log**: chỉ lưu tên khoá, đường dẫn Vault; che giá trị DSN, token, khoá API. Mã nguồn chỉ trả khi mở một symbol cụ thể.
- **SSH và remote:** không giả định thực thi cục bộ (AGENTS.md); mọi thao tác có thể qua SSH nên phải chịu độ trễ 50–200 ms.
- **Git:** nếu thêm lệnh git, theo [`git-compatibility.md`](../../../guides/reference/git-compatibility.md) (baseline 2.25, `GitCapabilityCache`).
- **Provider:** tên không gắn riêng GitHub; GitLab và nhà cung cấp khác dùng chung khái niệm.
- **Idempotent:** consumer sự kiện chịu giao lặp (at-least-once).
- **UI:** theo [`STYLEGUIDE.md`](../../../guides/STYLEGUIDE.md), shadcn primitives, token từ `main.css`, không hex cứng, `lucide-react`, mọi chuỗi qua `translate()`; tên file theo khái niệm cụ thể, không `utils`/`helpers`/`common`; không thêm `max-lines` disable.
- **Hai render target:** mọi tính năng frontend chạy cả Electron (`window.api` preload) và web (`web-preload-api`).
- **Phiên bản công cụ:** ghi `toolVersion` vào mọi kết quả; kiểm thử hợp đồng với fixture vàng (CR-CV-070).
- **Feature flag:** toàn bộ tính năng sau cờ `code_intel_enabled` (mặc định O8), mặc định tắt.

## 7. Rủi ro toàn series

- Docs/nghiên cứu và code có thể lệch (đã gặp ở v6); mỗi CR yêu cầu đọc lại code trước khi sửa.
- `gitnexus cypher` trả markdown: parse dễ vỡ khi ô chứa `|`, xuống dòng, hoặc phiên bản công cụ đổi định dạng (CR-CV-002, 070).
- Cú pháp Cypher cho gộp cạnh theo cụm trên LadybugDB chưa kiểm chứng; chi phí truy vấn trên 247k node/644k cạnh chưa đo.
- Số `communities` lệch giữa `meta.json` (10 252) và truy vấn (9 039); chưa rõ nguyên nhân.
- CodeGraph và GitNexus có phạm vi trích xuất khác nhau; hợp nhất id có thể sai (CR-CV-020).
- Index có thể cũ hơn HEAD; UI phải luôn hiển thị `indexedAt`/`stale`.
- Registry GitNexus có nhiều repo; ánh xạ sai repo làm lộ dữ liệu repo khác (CR-CV-001, 012, 072).
- Hiệu năng: CLI ~1,8 s mỗi lần, DB CodeGraph ~1,2 GB, đồ thị lớn trong UI.
- C4 và luồng dữ liệu là suy luận heuristic; cần chú thích tay (`c4.yaml`) và nhãn rõ "suy luận".
- Chiều agent → backend hiện chỉ có consumer cho `pty.data`, `fs.changed`…; thông báo mới `codeintel.*` cần consumer (CR-CV-023); chưa kiểm tra repo có event bus dùng chung thay cho RPC stream.
- Dev server offline: backend chỉ xếp hàng gọi tối đa ~20 s (`RECONNECT_WAIT_MS`), UI cần xử lý.

## 8. Điều chỉnh hợp đồng sau khi soạn các CR (cần chốt khi duyệt)

Các CR được soạn song song bởi 8 người soạn độc lập và phát hiện chỗ README này thiếu hoặc lệch code. Mục 3 ở trên giữ nguyên bản gốc để đối chiếu. **Khi hai nơi khác nhau, theo CR.** Chi tiết từng điểm nằm ở mục "Điểm lệch"/"Điều chỉnh hợp đồng" trong README của từng folder. Chưa có điều chỉnh nào được chạy thử trên hệ thống thật.

| # | Nội dung | CR nắm quyết định |
|---|---|---|
| 1 | **Agent không có "workspace root đăng ký"**: `registerRoot` là no-op, `fs.*` nhận mọi đường dẫn; chỉ `fs.writeFile` có kiểm tra "SecureFs" (`agent/src/relay/fs-agent-write-extensions.ts:43`), không có cơ chế chặn đường dẫn dùng chung cho thao tác đọc. Ranh giới thật là registry GitNexus cộng phân quyền backend; `codeintel.*` và `CR-CV-030` phải tự chặn đường dẫn. Quy ước mục 6 và mặc định O4 hiểu theo đó | 001, 012, 030, 072 |
| 2 | Mục 1 nói Go lưu `tools[]` qua `LastHandshakeInfo`: **sai**, code Go không đọc `tools`; `tools[]` chỉ là mảng tên. Cần thêm `capabilities` `codeintel*` và để `infra-fleet-service` giữ chúng | 001, 023 |
| 3 | "Xếp hàng ~20 s khi dev server offline" (D2, mục 7) chỉ có ở cầu nối TS cũ; `RelayByDevServer` của Go trả lỗi ngay, nên collector tự chờ/thử lại | 021, 023 |
| 4 | `RelayByDevServer` làm mất `error.data.code` của agent; mã `CODEINTEL_*` chỉ tới được `code-intel-service` sau CR-CV-023 (phía Go dùng kiểu lỗi mới mang mã). Part B (`RelayDispatcher`) cũng làm rơi `error.data` | 023, 006 |
| 5 | Mục 3.3 thiếu mã lỗi: `CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_AGENT_UNSUPPORTED`, `CODEINTEL_RESULT_INVALID`, `CODEINTEL_SYMBOL_NOT_FOUND`, `CODEINTEL_SECRET_LEAK_BLOCKED`, `CODEINTEL_UNAVAILABLE`, `CODEINTEL_DISABLED`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_RESPONSE_TOO_LARGE`, `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_RATE_LIMITED`, `CODEINTEL_INTERNAL` | 020, 021, 035, 040 |
| 6 | Mục 3.2 thiếu method agent: `codeintel.watch`, `reindexStatus`, `reindexCancel`, `structuralFacts` (vòng, import theo lớp, mã chết, kích thước tệp, in-degree; vì `subgraph` bị giới hạn 4 000 cạnh); đề xuất thêm `codegraphSearch`, `files`. `codeintel.impact` thêm `includeTests`. `detectChanges` hỗ trợ "merge-base → working tree", lọc kind `Section`, timeout phía Go ≥ 60 s (hiện 30 s). `percent` cho phép `null`; `indexChanged` thêm `reason`, `headCommit`, `stale`, `tool:"git"`. Kết quả agent thêm khối `perf`; chưa chốt `data` là dạng gốc công cụ hay đã chuẩn hoá | 001, 004, 005, 021, 036, 037 |
| 7 | **`gitnexus analyze` mặc định ghi `AGENTS.md`, `CLAUDE.md` và skills vào cây làm việc**: `codeintel.reindex` bắt buộc `--index-only`. Stdout của `gitnexus` bị cắt cụt qua pipe nhưng vẫn `exit 0` (phải ghi ra tệp tạm); `cypher` không có tham số ràng buộc, lỗi trả `{error}` với exit 0, markdown không thoát `|` và gộp xuống dòng; tiêm cờ `--repo=` trả dữ liệu repo khác; `CALL show_tables()` vẫn chạy được | 002, 004, 070, 072 |
| 8 | Mục 7 ghi cú pháp gộp cạnh theo cụm "chưa kiểm chứng": đã chạy được trên Orca (~4,5 s, một lần đo một máy). `detect-changes` chỉ in text, tối đa 15 symbol và 10 luồng. GitNexus tính dòng 0-based (CodeGraph 1-based); id GitNexus có hậu tố `#arity`; CodeGraph dùng `::` thay `.`; thiếu kind `import`, `enum_member`, `namespace` ở bảng ánh xạ 05 §3. Agent không có driver SQLite | 002, 003, 005, 020 |
| 9 | **`Process` của GitNexus không đi vào use case, adapter DB hay gRPC ở backend Go** (0 Process có bước ở các lớp đó). Luồng nghiệp vụ phải dựng bằng duyệt tĩnh (kênh → RPC → handler → use case → cổng → adapter); research coi `Process` là nguồn chính cho view luồng là chưa đúng cho Go | 034 |
| 10 | Mục 3.5 bảng dữ liệu: thêm `tenant_settings`; `repo_bindings` thêm `repo_id`, `scope_key`, `path_hash`, `index_scope`, `last_status`, `version`; `review_states` khoá theo `repo_binding_id` (id worktree có 3 dạng chuỗi) và lưu `notes`, `turns[]`, `sentBatches[]`; `c4_overrides` và `finding_dismissals` khoá theo `repo_id`, `finding_dismissals` thêm `disposition`, `finding_key` không chứa số dòng; `reindex_jobs` thêm `active_key` UNIQUE; `graph_snapshots` thêm `etag`, `total_count`, chỉ mục duy nhất; thêm chỗ lưu `erd-links.yaml` (E7). Ranh giới: RPC hiện chưa có message `Finding`, `ContractChange`; thiếu RPC lộ catalog hợp đồng/kênh `wscompat`; `GetArchitecture` mơ hồ giữa C4 và đồ thị cụm | 011, 012, 020, 031, 032, 033, 059 |
| 11 | Tên message chung ở 08 (`Table`, `Relation`, `Component`, `FlowStep`…) va chạm trong một package proto; dùng tiền tố `Erd*`, `C4*`, `DataFlow*`. `ErdModel` thêm `changes[]`, `op` trong `accessedBy`, `service` ở đầu mút liên kết, `warnings[]`; `GetErd` nhận `base/head`. `StorageMap` thêm `view:"storage"`, `Store.env:"legacy"`, `deployed`, `supportedByCode`, `confidence`, `evidence`, `sourceFiles`, `change`, `origin`. Thêm kiểu `readingOrder[]`, `violations[]`, `riskReasons`, `ContainerRef`, `DataFlowSummary`, `ResultMeta.not_modified`; `ImpactGraph` chưa có cạnh. Parser SQL/proto là **hàm thuần** (nội dung tệp vào, catalog ra) để so base/head | 020, 031, 032, 035, 036, 057 |
| 12 | Mục 3.7: chốt **26 kênh** (25 unary, 1 stream); thêm `codeIntel.subscribe` (mở push `codeIntel.changed`/`reindexProgress`), `codeIntel.settings.get/set`; thêm RPC `GetSettings`/`SetSettings` cho cờ `code_intel_enabled`. Tên tool MCP theo D7 v5 là `codeIntel_*`, không phải `codeintel_*` | 040, 041, 073 |
| 13 | Frontend: preload Electron thật ở `desktop/src/preload/index.ts` (ngoài `frontend/`, chưa có `mcp`); `window.api` ở web là Proxy `withFallback` nên không phát hiện tính năng bằng `typeof`; `callRuntimeResult` làm mất `error.code` nên cầu nối trả phong bì; tab `review` chạm ít nhất 18 chỗ (không chỉ hai union); `ConnectionStatusBanner` chỉ có ở web và dùng hex, không dùng lại được; `DiffViewer` chưa có API cuộn tới dòng (CR-CV-053 thêm `pendingDiffReveal`); `openDiff` chưa nhận số dòng | 050, 051, 053, 060 |
| 14 | Mobile gọi runtime của host, không qua `api-gateway`: cần method host `codeIntel.reviewSummary` và thêm vào `MOBILE_RPC_METHOD_ALLOWLIST` (`desktop/src/main/runtime/runtime-rpc.ts`). Chưa rõ desktop main có tới được `api-gateway` không | 062 |
| 15 | "Lượt" của agent chưa có dữ liệu lưu bền (`agentStatusByPaneKey` chỉ trong bộ nhớ); CR-CV-060 tự tạo "mốc lượt" và so sánh bằng dấu vân tay thô. Ghi chú đã gửi bị `clearDeliveredDiffComments` xoá nên phải lưu lô đã gửi | 060 |
| 16 | Hạ tầng Go: gRPC trong `backend-go` đang ở trần mặc định 4 MiB (không có `MaxCallRecvMsgSize`); `StreamFileChanges` (mẫu của `StreamCodeIntelEvents`) có vẻ không gắn tenant cho stream; `auditclient.Append` nuốt lỗi và thiếu `actor_type`/`target_type`; chỉ `mcp-service` dùng RLS thật, RLS không chạy ở dev (compose dùng superuser `orca`); CI không chạy test của `agent/`; v6 CR-REQ-001 lệch code ở 5 điểm mà CR-CV-010 không sao lại (RLS kiểu task-service vô hiệu, `TxRunner`, không có gRPC health, `make proto-lint` có `|| true`, thiếu wiring `deploy/dev/docker/postgres/init-databases.sh` và `build-local.sh`) | 010, 013, 023, 072 |
| 17 | Đường dẫn: `deploy/` nằm ở gốc repo, không phải `backend-go/deploy`; repo **không có** `CODEOWNERS` (owner ở CR-CV-037 chưa có nguồn); `deploy/prod/docker-compose.yml` chỉ có `orca-server` | 035, 037 |
| 18 | Worktree liên kết **không có `.gitnexus` riêng**: chỉ có chỉ mục của checkout chính, nên phần lớn worktree ở trạng thái `repo_root`/stale; chưa chốt cách làm mới (`analyze` trong worktree hay `--branch`). Chưa kiểm chứng agent phân biệt được `exact` và `repo_root` | 001, 004, 012 |
| 19 | Đường dẫn đúng: `guides/STYLEGUIDE.md` và `guides/reference/git-compatibility.md` (AGENTS.md trỏ `docs/...` không tồn tại) | tất cả |

Điều chỉnh từ các CR kiểm soát chất lượng (CR-CV-080..095, soạn 2026-10-06; chi tiết ở README của `quality-signals`, `quality-gate`, `quality-visualization`):

| # | Nội dung | CR nắm quyết định |
|---|---|---|
| 20 | **Đường dẫn repo đã tách** (`frontend/`, `desktop/`, `agent/`, `backend/`): `config/scripts/` ở gốc chỉ còn 3 file; các script `check-*`, `vitest.config.ts`, `max-lines-baseline.txt` nằm ở `desktop/config/`; 9 file mà `lint`/`typecheck`/`test` của `package.json` gốc trỏ tới không có ở gốc; guard `.d.ts` của `pr.yml` quét `src/preload src/shared` ở gốc nơi không có `src/`; có hai bản baseline `max-lines`. Profile kiểm tra phải chạy với cwd đúng; chưa chạy thử | 081, 084 |
| 21 | O10 đã chỉnh: `codegraph sync` ở worktree liên kết trả `pendingChanges` của checkout chính; dùng overlay theo diff + `indexScope=repo_root`; thêm trạng thái `OVERLAY` ở CR-CV-012 và nhận `indexScope=stale`; `reindex_jobs.requested_by` cho phép NULL và thêm cột `trigger` (CR-CV-011). Tín hiệu "agent xong": `agent.hook` không có ở Part A và Go không giải mã `state`; dùng `orca.infra.agent.statusChanged` nhưng payload thiếu `worktree_id`/`dev_server_id` và publish trực tiếp (at-most-once); CR-CV-089 cần spike ghi envelope thật | 080, 011, 012, 089 |
| 22 | **`toolEnv` của agent chứa toàn bộ `process.env`** (gồm `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`); PATH thiếu `~/go/bin`. Profile kiểm tra (O11) tự lọc biến môi trường theo danh sách cho phép, không dùng thẳng `toolEnv`; tool `gitnexus`/`codegraph` hiện có cũng đang chạy với các biến đó. `ensure-native-runtime --runtime=node` có thể chạy `pnpm rebuild` (ghi `node_modules`): preflight chỉ dùng `--check-only` | 081, 072 |
| 23 | Mục 3.10 cần thêm: mã lỗi `CODEINTEL_RUN_NOT_FOUND`; `quality.results` nhận `view`; `QualityRun` thêm `errorCode` (phân biệt hỏng công cụ với tìm thấy lỗi), `treeFingerprint`, cột CI; `QualityFinding` thêm trường bổ sung; chữ ký `IndexBasis`; `reasons[].result` thêm `unknown`, cùng `code`/`params`; `quality.cancel` và RPC liệt kê profile (thiếu ở kênh); push `quality.progress|finished` phải mang `worktreeId`; message cho trend, coverage, hotspot, "đã miễn trừ" | 081, 082, 085, 087 |
| 24 | RPC/kênh còn thiếu: `RecordAgentTurn`, `ListAgentTurns`, `GetAgentTurn`, `ConfirmRequirementEvidence`, `LinkWorktreeTask`, push `gateChanged`; `ExportReviewReport` trả mô hình có cấu trúc, không trả văn bản; bảng `requirement_trace_links`; cột mới trong `tenant_settings`; hành động OPA `quality_*`; sự kiện `agent_turn.recorded`; cờ tenant `quality_security_scan_enabled` | 085, 089, 090, 092, 091 |
| 25 | Hợp nhất khái niệm: `quality_waivers` tách khỏi `finding_dismissals`; `finding_dismissals` khoá `repo_id` (CR-CV-011) lệch `repo_binding_id` (CR-CV-037/059); `Finding` (CR-CV-059) và `QualityFinding` trùng khái niệm (bản đầu: một dock, hai nguồn tách biệt, không trộn danh sách); tên mã lỗi khi cờ tắt không thống nhất (`CODEINTEL_FEATURE_DISABLED` ở CR-013, `CODEINTEL_DISABLED` ở CR-073) — chọn một; bốn mã lỗi chất lượng mới chưa có trong test "đúng 10 mã" của CR-CV-050 | 011, 013, 037, 050, 059, 073, 085, 087 |
| 26 | CI/PR: backend Go chưa có RPC check/pipeline; cần `scmintegration.ListCommitChecks`, `RefreshCiRun`, kênh `codeIntel.quality.ci` và cột CI trong `quality_runs`; `getPRChecks` của desktop bỏ qua `headSha`, `PRCheckDetail` không mang SHA. CI chạy 18 workflow `backend-go-*` (không phải 17), không chạy `golangci-lint` hay `opa test`; `make proto-lint` có `\|\| true`; Go CI 1.25 lệch `go.work` 1.26; `golangci-lint` v1.62.2 (build go1.22.2) có thể không chạy với `go.work` 1.26 (chưa kiểm chứng) | 086, 081 |
| 27 | Tool GitNexus: `check` và `group *` có CLI; `shape_check`, `api_impact`, `route_map`, `tool_map`, `explain`, `pdg_query` chỉ có ở MCP; chỉ mục Orca không có lớp PDG | 094 |
| 28 | Frontend chất lượng: điểm chèn "Create PR" thật là `CreateHostedReviewComposer` và `SourceControl.tsx` (`pr-create-dialog` là code chết; nhánh `CommitArea` chưa đọc hết); `--warning` chưa có trong `main.css`; `--review-untested` ở chế độ sáng chỉ ~3,19:1 trên `card` (tính tay, cần test); repo đã có `status-bar/workspace-space-layout.ts` (treemap) nên CR-CV-054 nói "không có treemap" là chưa đúng; Monaco `setModelMarkers`, `glyphMargin`, F8 chưa dùng ở repo | 054, 087, 088 |
| 29 | CR-CV-089 phụ thuộc dữ liệu `agent.hook` mà backend chưa dùng (Go chỉ giải mã `providerSession`; bộ phát chỉ thấy ở `relay.ts:554` Part B; payload không có mã thoát của lệnh) | 089 |

**Số lượng CR:** 59: `agent-codeintel` 6, `code-intel-service-foundation` 4, `code-intel-graph-pipeline` 5, `code-intel-sources` 9, `code-intel-gateway` 2, `review-frontend` 13, `quality-rollout` 4, `quality-signals` 8, `quality-gate` 6, `quality-visualization` 2.

**Điểm chưa ai chốt**
- Tên và tham số kênh `codeIntel.*`, vị trí mã lỗi (`error.code` hay `error.data.code`).
- Thư viện parse SQL (CR-CV-031) và proto (CR-CV-032): các CR đề xuất bộ tự viết, chưa chạy thử.
- Ngưỡng điểm rủi ro (CR-CV-036) và tỷ lệ báo nhầm của quy tắc `tenant_id` (CR-CV-038) chưa đo.
- Schema `c4.yaml`; `git.branchDiff` có chấp nhận compare tổng hợp cho khoảng commit không.
- Quy tắc ghép `workspaceRoot` với binding; Windows chưa kiểm tra; cách `analyze`/`init` ghi vào registry toàn cục; cách cài CodeGraph không tương tác.
- Số đo hiệu năng chỉ là một lần chạy trên một máy.
