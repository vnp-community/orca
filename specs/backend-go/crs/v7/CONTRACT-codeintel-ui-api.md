# CONTRACT: Hợp đồng kênh WS `codeIntel.*` giữa api-gateway (wscompat) và frontend/mobile (v7)

> **Nguồn sự thật cho mọi thứ frontend gọi/nhận từ `api-gateway` trong series v7.** `BE-CV-SOL-040-*` hiện thực đúng file này; `FE-CV-SOL-*` chỉ dùng những gì có ở đây. Bộ ba: [`CONTRACT-codeintel-agent-rpc.md`](./CONTRACT-codeintel-agent-rpc.md), file này, [`CONTRACT-codeintel-proto-and-data-map.md`](./CONTRACT-codeintel-proto-and-data-map.md). **Mục "Phán quyết" (PQ-01…PQ-37) nằm ở file proto, mục 1; file này trích `PQ-xx`.** Mẫu: [`../v6/gateway-and-mcp/CONTRACT-request-ui-api.md`](../v6/gateway-and-mcp/CONTRACT-request-ui-api.md).
> CR gốc: 040, 041, 050–062, 085–090, 092, 093, 095, 087, 088. README v7 mục 8 thắng mục 3; phán quyết thắng CR.
> **Trạng thái: 📋 Proposed.** Chưa chạy. Mọi tham số kênh là đề xuất hợp nhất của hợp đồng; kiểu TS là bản mirror để frontend sao chép.

## 0. Bằng chứng đã đọc (code thật)

| Khẳng định | Nơi đọc |
|---|---|
| Hai dialect vào: native `{"id","type":"invoke","channel","args":[…]}`; session-client `{"id","authToken","method","params"}` (không `type`) → `normalizeInboundMessage` đặt `Channel=Method`, `Args=[params]`, `params` vắng/`null` → `{}` | `backend-go/services/api-gateway/internal/adapter/wscompat/session_dialect.go`, `envelope.go` |
| Ra native: `{"type":"result","id","result"}` / `{"type":"error","id","message"}`; ra session-client: `{"id","ok":true,"result","_meta"}` / `{"id","ok":false,"error":{"code":"internal","message"},"_meta"}` — **mã lỗi luôn `internal`, chỉ `message` mang thông tin; không có `error.data`** | `session_dialect.go` `writeDialectError`, `envelope.go` |
| Push native `{"type":"push","channel","args":[…]}`; session-client: khung `streaming:true` mang `Args[0]` (mất tên kênh) + khung cuối `{"type":"end"}` | `push_bridge.go` `pipePushForDialect`, `pushEventResult` |
| `ChannelHandler func(ctx, Identity, args []json.RawMessage) (any, error)`; `Dispatch` gắn Identity (kể cả `Role`), trần 60 s; `invokeTimeout` WS 25 s; `decodeArg[T](args, i)` lỏng | `registry.go`, `handler.go:247` |
| `Identity{TenantID, UserID, DeviceID, Role}` (`DeviceID` ≠ rỗng = phiên thiết bị mobile) | `registry.go` |
| `normalizeNilSlices` chỉ sửa slice cấp đầu; không trả message proto thô (snake_case) | `registry.go` |
| Tiền lệ lỗi có mã: `mcpChannelError` bóc `rpc error: code = … desc = `, giữ `^MCP_…: `; v6 `requestChannelError` | `channels_mcp.go:85-119`; `CONTRACT-request-ui-api.md` C4 |
| `/ws` **không** gọi `SetReadLimit` (grep); `coder/websocket v1.8.15`; giới hạn đọc mặc định của thư viện chưa kiểm chứng | `go.mod`, grep |

## 1. Nguyên tắc tương thích

| # | Nguyên tắc |
|---|---|
| U1 | Mọi kênh nhận **đúng một object JSON ở `args[0]`** (`{}` hợp lệ nếu mọi trường tuỳ chọn); **không có `args[1+]`**; `len(args) > 1` → `CODEINTEL_INVALID_PARAMS`. Giải mã chặt (`DisallowUnknownFields`, hàm `decodeCodeIntelArgs[T]`, **không** dùng `decodeArg` lỏng); lỗi nêu **tên trường**, không giá trị. Session-client: `params` = `args[0]`. Vì mọi kênh dùng một object, "tham số theo vị trí" của hợp đồng này là **vị trí 0 duy nhất** |
| U2 | Chỉ dùng `invoke` (native) / `call`. **Không dùng `send`** (fire-and-forget): lỗi bị nuốt (`handleSend` chỉ log) |
| U3 | Danh tính **chỉ** từ session; tham số `tenantId, userId, deviceId, role, devServerId, workspaceRoot, repo, args, command, cypher` bị từ chối `CODEINTEL_INVALID_PARAMS` (PQ-04 chốt định danh là `{projectId, worktreeId}`) |
| U4 | JSON **camelCase**; thời gian RFC 3339 UTC; slice rỗng `[]` (view tự bảo đảm slice lồng); enum lạ ở client → `'unknown'` |
| U5 | **Mã lỗi nằm ở tiền tố `message`**: `CODEINTEL_X: mô tả[ \| {json}]` (PQ-02). Client **không** đọc `error.code` (luôn `"internal"`) và **không** tìm `error.data` (không tồn tại trên WS). `callRuntimeResult` làm mất `error.code`/`data` nên bridge CR-050 dùng đường trả phong bì nguyên (không ném chuỗi) và tự tách tiền tố |
| U6 | Kết quả view = phong bì phẳng `{...meta, data}` (PQ-12); `ifNoneMatch`/`notModified` là **tuỳ chọn** (client không bắt buộc dùng) |
| U7 | Cờ tắt → `CODEINTEL_DISABLED` ở mọi kênh ngoài `settings.get|set`; client coi là `kind:'disabled'` (ẩn tính năng, không toast) |
| U8 | Phiên thiết bị (`DeviceID ≠ ""`) bị từ chối `CODEINTEL_NOT_AUTHORIZED` ở mọi kênh `codeIntel.*` trừ `settings.get` (mobile dùng đường host, PQ-36) |
| U9 | Mọi chuỗi tự do từ backend (message phát hiện, tên) có thể chứa dữ liệu không tin cậy: render văn bản thuần, không HTML/markdown thô |

## 2. Quy ước chung

### 2.1 Định danh và tham số chung
```ts
type WorktreeSel = { projectId: string; worktreeId: string }   // PQ-04; worktreeId = worktree_ref đã chuẩn hoá
```
Mọi kênh có "(sel)" dưới đây nhận **các trường của `WorktreeSel` ngay trong `args[0]`** (phẳng): `{ projectId, worktreeId, ...riêng }`. Chuỗi: `worktreeId` ≤ 512, `projectId` ≤ 64.

### 2.2 Phong bì kết quả (mọi kênh "view")
```ts
type CodeIntelEnvelope<T> = {
  repo?: string; worktreeId: string; view: string              // 'status'|'structure'|…
  sources: { tool: 'gitnexus'|'codegraph'; version: string; indexedAt: string|null; commit: string|null; lineBase?: 1 }[]
  headCommit: string|null; stale: boolean; truncated: boolean
  totalCount: number                                           // 0 = không biết; chỉ có nghĩa khi > 0
  etag: string; fromCache: boolean; generatedAt: string
  notModified?: true                                           // khi ifNoneMatch khớp: KHÔNG có data
  nextPageToken?: string                                       // kênh có phân trang
  data?: T
}
```
`devServerId` và mọi đường dẫn tuyệt đối trên dev server **không** ra UI. `etag` có dạng `"…32hex…"` (có nháy); `ifNoneMatch` ≤ 80 ký tự.

### 2.3 Lỗi tới client — bảng hợp nhất (PQ-03)
Dạng `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`; phần mô tả 1 dòng ≤ 200 ký tự, không mã nguồn, không đường dẫn tuyệt đối (`/…` hoặc `X:\…` ≥ 3 phân đoạn → `<path>`), không giá trị tham số. Nhóm 3 = `data` JSON ≤ 2 KiB.

| Mã | Khi nào | `data` | Client `kind` |
|---|---|---|---|
| `CODEINTEL_DISABLED` | cờ tenant/công tắc tắt | — | `disabled` |
| `CODEINTEL_QUALITY_GATE_DISABLED` | cờ chất lượng tắt | — | `quality-disabled` (ẩn phần chất lượng) |
| `CODEINTEL_AI_REVIEW_DISABLED` | AI tắt | — | `ai-disabled` |
| `CODEINTEL_UNAVAILABLE` | `CODE_INTEL_SERVICE_ADDR` rỗng, `Unavailable`, `Unimplemented` | — | `unsupported`/`offline` |
| `CODEINTEL_NOT_AUTHORIZED` | không phải thành viên/không quyền/phiên thiết bị/tenant khác | — | `forbidden` |
| `CODEINTEL_NOT_FOUND` | id tài nguyên con (job, run, waiver, turn) không thuộc tenant; Identity rỗng | — | `not-found` |
| `CODEINTEL_INVALID_PARAMS` | khoá lạ, >1 args, vượt giới hạn, `params too large`, ref lạ | `field`, `reason?` | `validation` |
| `CODEINTEL_PATH_NOT_ALLOWED` | `path`/`file` không an toàn (`..`, tuyệt đối, `\`, `%2e`, NFKC) hoặc agent từ chối | `hint?` | `path-not-allowed` (không hiện đường dẫn) |
| `CODEINTEL_WORKTREE_NOT_FOUND`, `CODEINTEL_WORKTREE_REF_UNSUPPORTED`, `CODEINTEL_NO_DEV_SERVER`, `CODEINTEL_DEV_SERVER_NOT_APPROVED`, `CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED` | phân giải đích (CR-012) | — | `no-binding` (nút mở hướng dẫn) |
| `CODEINTEL_DEV_SERVER_OFFLINE` | dev server mất kết nối quá 20 s chờ | — | `offline` (giữ cache) |
| `CODEINTEL_TOOL_UNAVAILABLE` | thiếu GitNexus/CodeGraph, phiên bản ngoài dải | `tool`, `reason` | `tool-unavailable` |
| `CODEINTEL_INDEX_MISSING` | chưa có chỉ mục | `tool`, `hint` | `index-missing` (nút Làm mới) |
| `CODEINTEL_REPO_NOT_REGISTERED` | repo chưa đăng ký trong GitNexus/CodeGraph | `hint` | `repo-not-registered` |
| `CODEINTEL_AMBIGUOUS_SYMBOL` | symbol mơ hồ | `candidates[≤10]: {key?, uid, name, kind, filePath, line, score?, impactedCount?, risk?}` | `ambiguous` |
| `CODEINTEL_SYMBOL_NOT_FOUND` | symbol/flow không có | `kind` | `not-found` |
| `CODEINTEL_TIMEOUT` | vượt hạn (20 s đọc / 8 s ghi / agent) | `retryAfterMs?`, `inProgress?` | `timeout` (tự thử lại) |
| `CODEINTEL_REINDEX_IN_PROGRESS` | đang làm mới | `jobId`, `stage?` | `reindex-in-progress` (gắn vào job, không báo lỗi) |
| `CODEINTEL_REINDEX_COOLDOWN` | nghỉ 5 phút sau job thành công | `retryAfterSeconds` | `rate-limited` |
| `CODEINTEL_OUTPUT_TOO_LARGE` | đầu ra công cụ/agent vượt trần | `bytes?`, `limit?` | `too-large` |
| `CODEINTEL_RESPONSE_TOO_LARGE` | phản hồi > 2 MiB (symbol 320 KiB) | `bytes`, `limit` | `too-large` |
| `CODEINTEL_TOOL_FAILED` | công cụ lỗi/định dạng lạ | `reason?`, `retryable?` | `tool-failed` |
| `CODEINTEL_RESULT_INVALID` | collector không giải mã được | — | `tool-failed` |
| `CODEINTEL_AGENT_UNSUPPORTED` | agent cũ (-32601) | — | `unsupported` |
| `CODEINTEL_RATE_LIMITED`, `CODEINTEL_CONCURRENCY_LIMIT` | hạn mức | `retryAfterSeconds?`, `scope?` | `rate-limited` |
| `CODEINTEL_VERSION_CONFLICT` | `expectedVersion` cũ | `currentVersion?` | `conflict` |
| `CODEINTEL_PAYLOAD_TOO_LARGE` | lưu vượt giới hạn cột | `limit` | `validation` |
| `CODEINTEL_PROFILE_INVALID` | profile chất lượng sai | `field` | `validation` |
| `CODEINTEL_PROFILE_UNKNOWN` | tên profile lạ (hoặc profile bảo mật khi cờ tắt) | `available[]` | `profile-unknown` |
| `CODEINTEL_ENV_NOT_READY` | môi trường chạy kiểm tra chưa sẵn sàng | `missing[]`, `reason?` | `env-not-ready` |
| `CODEINTEL_RUN_IN_PROGRESS` | worktree đang có run | `runId`, `reason` | `run-in-progress` |
| `CODEINTEL_RUN_NOT_FOUND`, `CODEINTEL_RUN_CANCELLED` | run lạ/đã huỷ | `runId?` | `not-found`/`run-cancelled` |
| `CODEINTEL_WAIVER_EXPIRY_INVALID` | hạn miễn trừ quá giới hạn/quá khứ | `maxDays` | `validation` |
| `CODEINTEL_AI_NO_RELAY`, `CODEINTEL_AI_BAD_OUTPUT` | AI không có dev server / đầu ra sai | — | `ai-error` |
| `CODEINTEL_SECRET_LEAK_BLOCKED` | quét cuối phát hiện secret trong view | — | `tool-failed` (không hiển thị thô) |
| `CODEINTEL_AUTHZ_UNAVAILABLE`, `CODEINTEL_INTERNAL` | hạ tầng | — | `unknown` |
Ánh xạ khi message **không** có tiền tố `CODEINTEL_`: `Unavailable|Unimplemented|nil client`→`UNAVAILABLE`; `DeadlineExceeded`→`TIMEOUT`; `NotFound|PermissionDenied`→`NOT_AUTHORIZED` (không phân biệt "không có" và "không phải của bạn"); `InvalidArgument`→`INVALID_PARAMS`; `FailedPrecondition`→`TOOL_FAILED`; `ResourceExhausted`→`RATE_LIMITED` (hoặc `RESPONSE_TOO_LARGE` nếu nhắc kích thước); `Aborted|AlreadyExists`→`VERSION_CONFLICT`; còn lại `INTERNAL: internal error`. Client thêm `kind: 'offline'` khi `isConnectivityLikeRpcError`. Tiêu chí: số mã trong test của CR-050 là **bảng này** (không còn "đúng 10 mã").

### 2.4 Timeout và giới hạn (gateway)
- Đọc view: 20 s; ghi/trạng thái/settings: 8 s (< `invokeTimeout` 25 s < `INVOKE_TIMEOUT_MS` 30 s của client). Service tôn trọng deadline; hết hạn → `CODEINTEL_TIMEOUT` + hậu tố `{"retryAfterMs":3000,"inProgress":true}` và hoàn tất nền (singleflight). Client tự thử lại tối đa 90 s tổng.
- `args[0]`: `reviewState.save` ≤ 256 KiB; `c4.save` ≤ 96 KiB (`document` ≤ 64 KiB); `quality.profile.save` ≤ 96 KiB; `quality.trace.confirm|link` ≤ 8 KiB; còn lại ≤ 16 KiB; chuỗi `reason`/`note` ≤ 500, `findingKey` ≤ 128, `key` ≤ 1024, `pageToken` ≤ 512, `kinds` ≤ 32 phần tử. Gateway phải `conn.SetReadLimit(320<<10)` (PQ-14, O-2).
- Số: `depth` 1..3, `limit` trong ngân sách; **ngoài khoảng bị từ chối, không kẹp ngầm**.
- Phản hồi ≤ 2 MiB (`proto.Size`), `symbol` ≤ 320 KiB.
- Phân trang: `limit` + `pageToken` opaque (gateway không diễn giải); kết quả có `nextPageToken` (rỗng/vắng khi hết).

### 2.5 Quyền
Gateway không tự kiểm quyền (service thi hành, CR-013): cột "Quyền" là mức service: `read`, `read_source`, `review_write`, `reindex`, `c4_write`, `quality_read`, `quality_waive`, `quality_profile_write`, `admin`, `member`.

## 3. Danh sách kênh (46 kênh: 45 unary + 1 stream)

> Cột "Tham số `args[0]`": `(sel)` = `projectId`, `worktreeId` (bb). `?` tuỳ chọn. Mọi kênh view nhận thêm `ifNoneMatch?`. Kết quả: `Env<T>` = `CodeIntelEnvelope<T>`. RPC xem `CONTRACT-codeintel-proto-and-data-map.md` §3.

### 3.1 Kênh `codeIntel.*` (26 kênh, CR-040)

| Kênh | Tham số `args[0]` | Kết quả | Quyền | T/o |
|---|---|---|---|---|
| `status` | (sel), `refresh?: bool` | `{overall, tools[], scopeMismatch, activeJob?, indexBasis[], lastStatusAt, errorCode?, binding}` (`IndexStatus`, mục 4.1) — **không có phong bì** | read | 8 s |
| `reindex` | (sel), `mode?: 'incremental'\|'full'` (mặc định `incremental`) | `{jobId, status, mode, trigger}` | reindex | 8 s |
| `reindexStatus` | (sel), `jobId` | `ReindexJob` (4.1) | read | 8 s |
| `structure` | (sel), `path?` (tương đối), `depth?` 1..3 (2), `limit?`, `pageToken?` | `Env<ModuleGraph>` | read | 20 s |
| `architecture` | (sel), `container?`, `includeHidden?` | `Env<{containers: ContainerRef[]; view: C4ComponentView\|null}>` | read | 20 s |
| `dataFlows` | (sel), `triggerKind?`, `query?` ≤ 128, `service?`, `limit?` ≤ 100, `pageToken?` | `Env<{flows: DataFlowSummary[]}>` + `nextPageToken`, `total` | read | 20 s |
| `dataFlow` | (sel), `flowId`, `dialect?: 'postgres'\|'mysql'`, `detail?: 'service'\|'component'`, `maxServiceHops?` ≤ 8, `maxSteps?` ≤ 200, `includeSequence?`, `includeDfd?` | `Env<{flow: DataFlow; sequence?: SequenceModel; dfd?: DfdModel}>` | read | 20 s |
| `erd` | (sel), `service?`, `dialect?`, `base?`, `head?`, `includeAccess?` (true), `includeInferred?` (false) | `Env<ErdModel>` hoặc, khi không có `service`, `Env<{services: ErdServiceInfo[]}>` | read | 20 s |
| `storage` | (sel), `env?: 'dev'\|'prod'\|'legacy'`, `includeLegacy?` | `Env<StorageMap>` | read | 20 s |
| `subgraph` | (sel), `center` = đúng một `{symbol: string}`\|`{file: string}`\|`{cluster: string}`, `depth?` 1..3 (1), `kinds?: string[]`, `limit?` ≤ 1500 (800) | `Env<SymbolGraph>` | read | 20 s |
| `impact` | (sel), `target` = `{key: string}`\|`{name: string; file?: string; kind?: string}`, `direction?: 'upstream'\|'downstream'` (upstream), `depth?` 1..3 (2), `includeTests?` | `Env<ImpactGraph>` | read | 20 s |
| `symbol` | (sel), `key?` ≤ 1024 **hoặc** (`name`, `file`), `includeSource?` (true) | `Env<SymbolDetail>` | read_source | 20 s |
| `routes` | (sel), `limit?` ≤ 500 (200), `pageToken?` | `Env<RouteMap>` | read | 20 s |
| `changeOverlay` | (sel), `base?`, `head?`, `mode?: 'worktree'\|'committed'`, `detail?: 'summary'\|'full'` (full) | `Env<ChangeOverlay>` | read | 20 s |
| `readingOrder` | (sel), `base?`, `head?` | `Env<{steps: ReadingStep[]; components: ComponentGroup[]}>` | read | 20 s |
| `findings` | (sel), `rules?: string[]`, `severities?`, `pathPrefix?`, `includeDismissed?` (false), `scope?: 'all'\|'changed'` (changed), `base?`, `limit?` ≤ 200 (100), `pageToken?` | `Env<{findings: Finding[]; dismissedCount: number; indexFreshness: IndexFreshness}>` | read | 20 s |
| `dismissFinding` | (sel), `findingKey`, `action?: 'dismiss'\|'restore'` (dismiss), `disposition?: 'ignored'\|'resolved'`, `reason?` ≤ 500, `note?` ≤ 500 | `{findingKey, dismissed: boolean, disposition?}` | review_write | 8 s |
| `contractDiff` | (sel), `base?`, `kinds?: ('proto'\|'ws-channel'\|'route'\|'migration')[]`, `detail?: 'summary'\|'full'` | `Env<ContractDiff>` | read | 20 s |
| `reviewState.get` | (sel), `baseCommit?`, `headCommit?` | `ReviewState` (chưa có → `version:0`, mặc định) | read | 8 s |
| `reviewState.save` | (sel), `baseCommit`, `headCommit`, `readingProgress`, `notes?`, `turnMarkers?`, `status?`, `expectedVersion` | `ReviewState` | review_write | 8 s |
| `c4.get` | (sel), `container?` | `{container, document, version, updatedBy, updatedAt, seedSource?}` | read | 8 s |
| `c4.save` | (sel), `container`, `document` (YAML text ≤ 64 KiB), `expectedVersion` | `{version, warnings: {code, message}[]}` | c4_write | 8 s |
| `bindRepo` | (sel) | `{binding: RepoBinding; status: IndexStatus}` | read | 8 s |
| `settings.get` | `{}` | `Settings` (4.1) — luôn trả được | thành viên tenant | 8 s |
| `settings.set` | `{ codeIntelEnabled?, qualityGateEnabled?, qualitySecurityScanEnabled?, indexPolicy?, aiReviewLevel?, aiReviewModel?, agentTurnStorePromptExcerpt?, agentClaimTextEnabled?, hotspotWindowDays? }` (≥ 1 trường) | `Settings` | admin | 8 s |
| `subscribe` (stream) | `{ selectors?: WorktreeSel[] }` (0..50) | ack rồi push (mục 5) | read | — |
Ghi chú: không kênh cho `ListRepoBindings` (`status` đã trả binding). `bindRepo` idempotent. `reindex` trả ngay; tiến trình qua push, dự phòng `reindexStatus`. `symbol.file` đi qua `CleanWorktreePath` (từ chối `\`, tiền tố `/`, `%XX`, `..`, NFKC).

### 3.2 Kênh `codeIntel.quality.*` (20 kênh, PQ-27)

| Kênh | Tham số `args[0]` | Kết quả | Quyền | T/o |
|---|---|---|---|---|
| `quality.start` | (sel), `profile` (tên profile/suite), `scope: 'worktree'\|'changed'\|'commitRange'`, `base?` | `{run: QualityRun}` | review_write | 8 s |
| `quality.cancel` | (sel), `runId` | `{run: QualityRun}` | review_write | 8 s |
| `quality.run` | (sel), `runId` | `QualityRun` (4.3) | quality_read | 8 s |
| `quality.runs` | (sel), `limit?` ≤ 50 (20), `source?: 'local'\|'ci'`, `pageToken?` | `{runs: QualityRun[]; nextPageToken}` | quality_read | 8 s |
| `quality.findings` | (sel), `runId?`, `severities?`, `categories?`, `file?`, `inScope?`, `limit?` ≤ 500 (500), `pageToken?` | `{findings: QualityFinding[]; totalCount; truncated; outsideScopeCount; nextPageToken}` | quality_read | 20 s |
| `quality.waive` | (sel), `subjectKind: 'finding'\|'structure_finding'\|'check'`, `subjectKey`, `action?: 'waive'\|'revoke'`, `reason` (1..1000; `check` ≥ 20), `expiresAt` (RFC 3339 ≤ 30 ngày), `scope?: 'repo'\|'binding'` | `{waiver: QualityWaiver}` | quality_waive | 8 s |
| `quality.gate` | (sel), `base?`, `profileName?`, `turnKey?`, `record?`, `includeWaivedDetail?` | `{gate: QualityGate; waivers: QualityWaiver[]; evaluatedAt; profileDefinitionDigest; comparison: CiComparison[]}` | quality_read | 8 s |
| `quality.profile.get` | (sel), `name?` | `{profile: QualityProfile; origin: 'repo'\|'tenant'\|'builtin'; version; runnableProfiles: RunnableProfile[]}` | quality_read | 8 s |
| `quality.profile.save` | (sel), `scope: 'tenant'\|'repo'`, `name`, `definition`, `expectedVersion` | `{profile; warnings: string[]}` | quality_profile_write | 8 s |
| `quality.trend` | (sel), `from?`, `to?`, `limit?` ≤ 200 (50), `groupBy?: 'commit'\|'turn'` | `{points: QualityTrendPoint[]; truncated; totalCount}` | quality_read | 8 s |
| `quality.coverage` | (sel), `runId?` hoặc `headCommit?` | `{report: CoverageReport\|null; reason?}` | quality_read | 20 s |
| `quality.trace` | (sel), `base?`, `includeInferred?` (true), `taskId?`, `turnKey?` | `{trace: RequirementTrace; evaluatedAt}` | quality_read | 20 s |
| `quality.trace.confirm` | (sel), `requirementKey`, `evidenceKind`, `evidenceRef`, `linkKind: 'confirm'\|'reject'`, `scope?` | `{trace}` | review_write | 8 s |
| `quality.trace.link` | (sel), `taskId` (rỗng = gỡ) | `{trace}` | review_write | 8 s |
| `quality.summary` | (sel), `base?`, `profileName?`, `level: 'metadata'\|'diff'`, `dryRun?`, `forceRefresh?`, `locale?` | `{summary: AiReviewSummary\|null; manifest; cache; labels}` | quality_read ∧ read_source | 120 s ngoại lệ (xem dưới) |
| `quality.report` | (sel), `base?`, `profileName?`, `turnKey?`, `sections?`, `maxFindings?` ≤ 50 (10), `maxReadingSteps?` ≤ 50 (15), `includePeople?` (false), `includeWaiverReasons?` (false) | `{model: ReviewReportModel; generatedFor; warnings}` | quality_read ∧ read | 20 s |
| `quality.ci` | (sel), `force?` | `{run: QualityRun\|null; comparison: CiComparison[]; stale; rateLimited; resetAt?}` | quality_read | 20 s |
| `quality.turn.record` | (sel), `clientTurnId`, `agentType`, `model?`, `endedAt`, `startedAt?`, `interrupted?`, `endHeadCommit`, `treeDirtyEnd`, `filesChangedCount`, `filesDigest`, `promptDigest`, `promptExcerpt?`, `commandsSummary?`, `claims?` | `{turn: AgentTurn}` | review_write | 8 s |
| `quality.turns` | (sel), `limit?` ≤ 50 (20), `before?` | `{turns: AgentTurn[]}` | quality_read | 8 s |
| `quality.turn` | (sel), `turnId` | `{turn: AgentTurn}` | quality_read | 8 s |
`quality.summary`: 20 s là trần WS của gateway **cộng** `invokeTimeout` 25 s ⇒ **không thể** vượt 25 s trên đường đồng bộ; vì vậy `quality.summary` với `dryRun:false` trả `CODEINTEL_TIMEOUT` có hậu tố `{"inProgress":true,"retryAfterMs":3000}` sau ≤ 24 s và hoàn tất nền (cache 24 h); client thử lại (PQ-13). Timeout ghi 120 s ở bảng nghĩa là ngân sách phía service/infra-fleet, không phía WS.
Push thêm của nhóm này đi qua `subscribe` (mục 5). Tổng kênh: 26 + 20 = 46.

### 3.3 Kênh không có ở v7
`codeIntel.bindings`, `codeIntel.events.subscribe` (thay bằng `subscribe`), `codeIntel.quality.listProfiles` (dùng `quality.profile.get` với `runnableProfiles`), `codeIntel.hintAgentTurnFinished` (P1, thêm sau cùng cặp kiểu `{projectId, worktreeId, paneKey?}` → `{}`).

## 4. Mô hình dữ liệu chuẩn (JSON + TypeScript)

> Frontend đặt tại `frontend/src/shared/code-intel-types.ts` (+ `code-intel-quality-types.ts`). Trường `?` = có thể vắng; `| null` = luôn có, giá trị null. Enum lạ → `'unknown'` (client phải chịu). Mọi số đếm là số nguyên không âm.

### 4.1 Chung, index, settings
```ts
type SymbolKind = 'function'|'method'|'type'|'value'|'file'|'folder'|'route'|'component'|'namespace'|'import'|'cluster'|'flow'|'doc'
type SymbolRef = { key: string; kind: SymbolKind; nativeKind?: string; name: string; qualifiedName?: string
  filePath: string; startLine?: number; endLine?: number; language?: string
  gitnexusId?: string; codegraphId?: string; signature?: string; isExported?: boolean }     // dòng 1-based
type SourceRef = { path: string; line?: number; kind: 'compose'|'config'|'adapter'|'migration'|'code'|'proto'|'wscompat'|'route'|'sql' }
type IndexScope = 'exact'|'repo_root'|'stale'|'none'
type Freshness = 'fresh'|'fresh_base'|'stale'|'unknown'
type ToolIndexStatus = { tool: 'gitnexus'|'codegraph'; available: boolean; version?: string; supported: boolean
  state: 'missing'|'building'|'ready'|'stale'|'unknown'; indexedCommit?: string; indexedAt?: string; headCommit?: string; mergeBase?: string
  indexScope: IndexScope; freshness: Freshness; dirtySinceIndex?: boolean; changedFilesNotInIndex?: number
  stats?: { files?: number; nodes?: number; edges?: number; communities?: number; processes?: number }
  pendingChanges?: { added: number; modified: number; removed: number } | null; languages?: string[]; indicators?: string[] }
type IndexOverall = 'OFFLINE'|'UNKNOWN'|'NOT_INSTALLED'|'BUILDING'|'MISSING'|'DEGRADED'|'OVERLAY'|'STALE'|'READY'
type IndexStatus = { overall: IndexOverall; tools: ToolIndexStatus[]; scopeMismatch: boolean
  activeJob?: { id: string; stage: string; percent: number | null }
  indexBasis: IndexBasis[]; lastStatusAt?: string; errorCode?: string
  binding: { id: string; projectId: string; repoId: string; worktreeId?: string; indexScope: 'exact'|'repo_root'|'unresolved'; version: number } }
type IndexBasis = { tool: 'gitnexus'|'codegraph'; indexScope: IndexScope; freshness: Freshness; indexedCommit?: string; headCommit?: string
  mergeBase?: string; indexedAt?: string; dirtySinceIndex: boolean; changedFilesNotInIndex: number
  refreshState: 'idle'|'queued'|'running'|'deferred'|'failed'|'skipped'; trigger?: 'manual'|'agent_done'|'head_change'; toolVersion?: string
  indexPolicy: 'auto_in_place'|'per_worktree'|'off' }
type ReindexJob = { jobId: string; status: 'queued'|'running'|'succeeded'|'failed'|'cancelled'; mode: 'incremental'|'full'
  trigger: 'manual'|'agent_done'|'head_change'|'schedule'; stage: string; percent: number | null; message: string; outcome?: string
  errorCode?: string; startedAt?: string; finishedAt?: string }
type Settings = { effective: { codeIntelEnabled: boolean; qualityGateEnabled: boolean; qualitySecurityScanEnabled: boolean; aiReviewEnabled: boolean }
  tenant: { codeIntelEnabled: boolean; qualityGateEnabled: boolean; qualitySecurityScanEnabled: boolean; indexPolicy: 'auto_in_place'|'per_worktree'|'off'
    aiReviewLevel: 'off'|'metadata'|'diff'; aiReviewModel: string; agentTurnStorePromptExcerpt: boolean; agentClaimTextEnabled: boolean; hotspotWindowDays: number }
  updatedBy?: string; updatedAt?: string }
```
`codeIntel.status` luôn có `IndexStatus` phẳng (không `data`). `activeJob.percent` là `null` khi chưa biết.

### 4.2 Đồ thị cấu trúc và ảnh hưởng
```ts
type ModuleNode = { id: string; kind: 'file'|'folder'; language?: string; symbolCount: number; loc?: number; cluster?: string; area?: string }
type ModuleEdge = { from: string; to: string; kind: 'imports'|'contains'; count: number }
type ModuleGraph = { nodes: ModuleNode[]; edges: ModuleEdge[] }
type SymbolEdge = { fromKey: string; toKey: string; kind: string; confidence: number; reason?: string; line?: number; sources: ('gitnexus'|'codegraph')[] }
type SymbolGraph = { center: SymbolRef; nodes: (SymbolRef & { cluster?: string })[]; edges: SymbolEdge[]; depth: number }
type FlowSummary = { id: string; label: string; processType: string; stepCount: number; communities: string[]; entry: SymbolRef|null; terminal: SymbolRef|null }
type ImpactSymbol = { symbol: SymbolRef; via: string; confidence?: number; direct: boolean }
type ImpactGraph = { target: SymbolRef; direction: 'upstream'|'downstream'; risk: 'LOW'|'MEDIUM'|'HIGH'|'CRITICAL'|'UNKNOWN'; impactedCount: number
  levels: { depth: number; symbols: ImpactSymbol[] }[]
  affectedFlows: { flowId: string; label: string; stepCount: number; changedStep?: number }[]
  affectedClusters: { id: string|null; label: string; hits: number; impact: string }[]
  testsCovering: SymbolRef[] }                       // KHÔNG có cạnh ở v7 (hạn chế đã biết)
type RouteMap = { routes: { id: string; path: string; method: string; filePath: string; side: 'server'|'client'; handler: SymbolRef|null; middleware?: string[]; responseKeys?: string[]; errorKeys?: string[] }[]
  edges: { route: string; handler: string; kind: 'handles_route'|'fetches' }[] }
type SymbolDetail = { symbol: SymbolRef; incoming: Record<string, {uid?: string; name: string; filePath: string}[]>; outgoing: Record<string, {uid?: string; name: string; filePath: string}[]>
  flows: { id: string; label: string; stepCount: number; step: number }[]
  source: { text: string; startLine: number; endLine: number; truncated: boolean } | null
  sourceOmitted: 'gitignored'|'binary'|'sensitive_path'|'not_requested'|null }
```
(`ArchitectureGraph` cụm: không có kênh ở v7, PQ-10.) `FlowGraph` (steps) chỉ dùng nội bộ ở MCP/tương lai.

### 4.3 ChangeOverlay (CR-036; PQ-09/30/31)
```ts
type ChangedFile = { path: string; oldPath?: string; status: 'added'|'modified'|'deleted'|'renamed'|'copied'|'untracked'; added?: number; removed?: number
  area: string; componentId?: string; isTest: boolean; isGenerated: boolean; isDoc: boolean; mappingConfidence: 'exact'|'approx'|'none'; owner?: Owner }
type ChangedSymbol = { symbol: SymbolRef; changeKind: 'added'|'modified'|'renamed'|'unknown'; linesChanged: number; directCallers?: number; flows: number; tested: 'yes'|'no'|'unknown' }
type ReasonCode = 'contract'|'dependency-of'|'leaf'|'cycle'|'no-edges'|'test'|'doc'|'generated'|'overflow'
type ReadingStep = { stepKey: string; n: number; file: string; symbols: SymbolRef[]; hunks: { startLine: number; endLine: number }[]
  reason: ReasonCode; reasonParams?: Record<string,string>; dependsOn: string[]; tests: SymbolRef[]; cycleGroup?: string; layer: string }
type ComponentGroup = { componentId: string; containerId: string; label: string; files: number; symbols: number; added: number; removed: number; riskPoints: number; stepKeys: string[] }
type RiskReason = { code: string; points: number; messageKey: string; params?: Record<string,string>; evidence: (SymbolRef|string)[] }
type RiskAssessment = { level: 'LOW'|'MEDIUM'|'HIGH'|'CRITICAL'; score: number; incomplete: boolean; confidence: 'high'|'medium'|'low'; reasons: RiskReason[]; modelVersion: '1'; toolRisk?: string }
type IndexFreshness = { state: 'fresh'|'behind'|'dirty'|'missing'; indexedCommit?: string; headOid?: string; commitsBehind?: number; dirtyFiles: number; unindexedFiles: string[]; generatedAt: string }
type TouchedTable = { table: string; service: string; via: 'migration'|'code'; migrations: string[]; accessors: SymbolRef[] }
type TouchedContract = { kind: 'proto-rpc'|'ws-channel'|'route'|'file'; name: string; change: 'added'|'modified'|'removed'|'unknown'; files: string[]; compatibility: 'breaking'|'risky'|'compatible'|'unknown'; breaking?: boolean }
type ViolationRef = { findingKey: string; rule: string; severity: 'error'|'warning'|'info'; file: string; status: 'touched'|'introduced' }
type ChangeOverlay = { scope: { baseRef: string; baseOid?: string; mergeBase?: string; headOid?: string; mode: 'worktree'|'committed'; includesUncommitted: boolean }
  emptyReason?: 'unborn-head'
  changedFiles: ChangedFile[]; changedSymbols: ChangedSymbol[]; affectedFlows: FlowSummary[]; affectedClusters: { id: string; label: string }[]
  touchedTables: TouchedTable[]; touchedContracts: TouchedContract[]; uncoveredSymbols: SymbolRef[]; violations: ViolationRef[]
  readingOrder: ReadingStep[]; components: ComponentGroup[]; risk: RiskAssessment; indexFreshness: IndexFreshness
  limits: { truncated: { files: boolean; symbols: boolean; flows: boolean; steps: boolean; impact: boolean }; totalCounts: Record<string, number> } }
```
`riskReasons` top-level của README **không dùng**: dùng `risk.reasons` (PQ-09). `detail:'summary'` chỉ trả `scope`, đếm (`limits.totalCounts`), `risk`, `components`, `indexFreshness`; mảng chi tiết rỗng.

### 4.4 C4, luồng dữ liệu, ERD, lưu trữ (CR-031–035; tiền tố PQ-29)
```ts
type ContainerRef = { id: string; name: string; path: string; kind: 'service'|'agent'|'frontend'|'desktop'|'other' }
type C4Component = { id: string; name: string; kind: 'domain'|'usecase'|'adapter'|'grpc-server'|'grpc-client'|'config'|'other'; path: string
  description?: string; descriptionSource: 'c4.yaml'|'package-doc'|'none'; symbolCount: number; techHint?: string
  origin: 'derived'|'merged'|'declared'; packagePaths: string[]; hidden: boolean }
type C4Relation = { from: string; to: string; kind: 'uses'|'implements'|'calls-rpc'|'reads'|'writes'|'publishes'|'subscribes'; evidence: SymbolRef[]; count: number
  origin: 'derived'|'declared'; confidence: number; violatesLayering: boolean; label?: string }
type C4External = { id: string; name: string; kind: 'service'|'database'|'queue'|'vault'|'external-api'; description?: string; origin: 'derived'|'declared' }
type C4ComponentView = { container: ContainerRef; components: C4Component[]; relations: C4Relation[]; externals: C4External[]
  warnings: { code: string; message: string }[]; overridesVersion: string; hasOverrides: boolean }
type ComponentRef = { container: string; componentId: string; name: string; kind: 'component'|'external'|'ui' }
type StoreRef = { id: string; kind: string; name: string; schema?: string }
type DataFlowStep = { n: number; from: ComponentRef; to: ComponentRef; kind: 'call'|'rpc'|'event'|'db-read'|'db-write'|'ws-push'; method?: string; symbol?: SymbolRef
  sync: boolean; confidence: number; origin: 'static-fieldtype'|'static-name'|'process'|'declared'; evidence: SymbolRef[]; requestType?: string; responseType?: string; unimplemented?: boolean }
type DataFlow = { id: string; label: string; trigger: { kind: 'ws-channel'|'http'|'grpc'|'event'|'cron'; name: string }; steps: DataFlowStep[]
  stores: { step: number; store: StoreRef; table?: string; op: 'read'|'write'; confidence: number }[]
  completeness: 'complete'|'partial'; gaps: { afterStep: number; code: string; message: string }[]; services: string[]
  relatedProcesses: { processId: string; label: string; stepCount: number; relation: 'contains-symbol' }[] }
type DataFlowSummary = { id: string; label: string; trigger: DataFlow['trigger']; entryService: string; entryRpc: string; serviceHops: number; completeness: 'complete'|'partial' }
type SequenceModel = { participants: { id: string; label: string; kind: 'ui'|'component'|'external'; group?: string }[]
  messages: { n: number; from: string; to: string; label: string; kind: string; sync: boolean; dashedReturn: boolean; note?: string; confidence: number }[] }
type DfdModel = { nodes: { id: string; label: string; kind: 'ui'|'gateway'|'service'|'store'|'queue'|'external'; group?: string }[]
  edges: { from: string; to: string; label: string; kind: string; data: string[]; count: number }[] }
type ErdColumn = { name: string; type: string; canonicalType: string; nullable: boolean; defaultExpr?: string; isPk: boolean; isFk: boolean; comment?: string; generated: boolean }
type ErdTable = { name: string; schema: string; columns: ErdColumn[]; pk: string[]
  indexes: { name: string; columns: string[]; unique: boolean; partial?: string; method?: string; emulatesPartialUnique: boolean }[]
  checks: { name: string; expr: string }[]; rls: { name: string; command: string; usingExpr?: string; withCheckExpr?: string }[]
  rlsState: 'enabled'|'forced'|'none'|'unknown'; comment?: string; tenantScoped: boolean; degraded: boolean
  firstMigration: string; lastMigration: string; accessedBy: { symbol: SymbolRef; op: 'read'|'write'|'readwrite'; confidence: number }[] }
type ErdEndpoint = { service?: string; table: string; columns: string[] }
type ErdRelation = { kind: 'fk'|'logical'; from: ErdEndpoint; to: ErdEndpoint; cardinality?: 'one-to-one'|'many-to-one'; crossService: boolean
  source: 'ddl'|'declared'|'comment'|'naming'; confidence: number; onDelete?: string; note?: string }
type ErdChange = { table: string; column?: string; kind: 'added'|'removed'|'modified'; before?: { type: string; nullable: boolean; default?: string }
  after?: { type: string; nullable: boolean; default?: string }; migrationFile: string; line?: number }
type ErdModel = { service: string; dialect: 'postgres'|'mysql'; schema: string; asOfMigration: string; tables: ErdTable[]; relations: ErdRelation[]
  externalRefs: { service: string; table: string }[]; changes: ErdChange[]; warnings: { file: string; line?: number; code: string; message: string }[] }
type ErdServiceInfo = { name: string; dialects: ('postgres'|'mysql')[]; tableCount: number }
type Store = { id: string; kind: 'postgres'|'mysql'|'redis'|'object'|'volume'|'vault'|'queue'|'other'; name: string; env: 'dev'|'prod'|'legacy'; owner?: { name: string }
  schemas?: string[]; deployed: boolean; supportedByCode: boolean; external: boolean; evidence: SourceRef[]; confidence: 'declared'|'derived'|'inferred'; change?: 'added'|'removed'|'modified' }
type StorageMap = { stores: Store[]
  bindings: { service: string; store: string; access: 'rw'|'ro'; via: string; configKey?: string; evidence: SourceRef[]; confidence: Store['confidence']; change?: Store['change'] }[]
  topics: { name: string; stream?: string; publishers: string[]; subscribers: string[]; delivery: 'durable'|'ephemeral'|'unknown'; payload?: string; evidence: SourceRef[]; confidence: Store['confidence'] }[]
  sources: SourceRef[]; redactedCount: number; asOfCommit: string; warnings: string[] }
```
Chuỗi tự do trong `StorageMap` đã qua che ở backend; frontend vẫn che lớp thứ hai (`maskSensitiveText`). Không có trường giá trị secret.

### 4.5 Phát hiện cấu trúc và hợp đồng (CR-037/038; PQ-05/06/30)
```ts
type Owner = { source: 'codeowners'|'history'; names: string[]; share?: number }
type Finding = { findingKey: string; rule: string; kind: 'layer_violation'|'dependency_cycle'|'hotspot'|'missing_tenant_id'|'dead_code'|'rls_removed'
  severity: 'error'|'warning'|'info'; titleKey: string; params: Record<string,string>; subject: string
  evidence: { path: string; line?: number; symbol?: SymbolRef }[]; metrics: Record<string, number>; owner?: Owner
  scope: { service?: string; componentId?: string; layer?: string }; origin: 'introduced'|'touched'|'preexisting'|'unknown'
  confidence: 'high'|'medium'|'low'; dismissed?: { by: string; at: string; reason: string; disposition: 'ignored'|'resolved'; note?: string } }
type ContractChange = { id: string; kind: 'proto-service'|'proto-rpc'|'proto-message'|'proto-field'|'proto-enum'|'ws-channel'|'ws-channel-arg'|'route'|'route-field'|'sql-table'|'sql-column'
  name: string; service?: string; change: 'added'|'removed'|'modified'; compatibility: 'breaking'|'risky'|'compatible'|'unknown'; ruleId: string
  details: Record<string,string>; files: string[]; consumers: { kind: 'rpc-client'|'ws-channel'|'route'; service?: string; symbol?: SymbolRef; path?: string }[]; evidence: SourceRef[] }
type ContractDiff = { scope: ChangeOverlay['scope']; summary: { breaking: number; risky: number; compatible: number; unknown: number }
  changes: ContractChange[]; migrations: { service: string; dialects: string[]; files: string[]
    statements: { table: string; op: string; column?: string; compatibility: string; ruleId: string; dialectOnly?: string }[]
    tables: { table: string; service: string; change: string; accessors: SymbolRef[]; columnsReferenced: string[]; crossService?: boolean; evidence: SourceRef[] }[]
    findings: Finding[] }[]
  truncated: boolean; totalCount: number }
```
UI ánh xạ `compatibility` sang nhãn; **không tự phân loại** `breaking`; `consumers: []` với `breaking` → "Chưa tìm thấy nơi dùng" (không "không ai dùng"). `ruleId` mã lạ hiển thị nguyên văn.

### 4.6 Review state, ghi chú, lượt (CR-052/060; PQ-22/31)
```ts
type ReadingProgress = { version: 1; entries: Record<string /*stepKey*/, { state: 'seen'|'unseen'; at: number }>; lastFocusedKey: string | null }
type ReviewNoteAnchor =
  | { kind: 'diff-line'; filePath: string; startLine?: number; lineNumber: number }
  | { kind: 'graph-node'; lens: 'impact'|'architecture'|'dataflow'|'erd'|'storage'|'structure'|'contract'|'quality'|'requirements'; nodeKey: string; filePath: string; startLine?: number; endLine?: number; label: string }
  | { kind: 'finding'; findingKey: string; filePath: string; startLine?: number; label: string }
type ReviewSentBatch = { batchId: string; sentAt: number; turnId: string|null; targetPaneKey: string|null; agentType: string|null
  notes: { commentId: string; anchor: ReviewNoteAnchor; filePath: string; startLine?: number; lineNumber: number; body: string; fileIdentityAtSend?: string }[] }
type ReviewTurnMarker = { turnId: string /* `${paneKey}:${doneAt}` */; worktreeId: string; paneKey: string; agentType: string|null; startedAt: number|null; endedAt: number
  interrupted?: boolean; baseOid: string|null; headOid: string|null; files: { p: string; o?: string; h: string }[]; symbolKeys?: string[]; overlayAvailable: boolean }
type ReviewState = { baseCommit: string; headCommit: string; readingProgress: ReadingProgress; notes: { anchors: Record<string, ReviewNoteAnchor>; sentBatches: ReviewSentBatch[] }
  turnMarkers: ReviewTurnMarker[]; status: 'open'|'reviewed'; updatedBy?: string; updatedAt?: string; version: number }   // version 0 = chưa có bản ghi
```
Ghi: `expectedVersion` = `version` đã đọc; `0` = tạo; lệch → `CODEINTEL_VERSION_CONFLICT` (client tải lại, `mergeReadingProgress` last-writer-wins theo `at`). Giới hạn: `readingProgress` ≤ 64 KiB, `notes` ≤ 256 KiB/500 mục, `turnMarkers` ≤ 5 phần tử. `turnMarkers` chỉ ở dòng mức worktree (`baseCommit=''`, `headCommit=''`). **Không lưu prompt người dùng.** `AgentTurn` (4.7).

### 4.7 Chất lượng (CR-082/085/083/089/090/092/093; PQ-33/34)
```ts
type QualityFinding = { fingerprint: string; fpVersion: number; ruleId: string; severity: 'error'|'warning'|'info'
  category: 'lint'|'typecheck'|'test'|'coverage'|'complexity'|'security'|'dependency'|'convention'|'architecture'|'ai'
  file: string; line: number; endLine: number; column: number; endColumn: number; message: string; tool: string; toolVersion: string; fixHint?: string
  stepId: string; inScope: boolean; waiver?: { by: string; reason: string; expiresAt: string } }
type QualityStep = { id: string; profileId: string; status: 'passed'|'findings'|'failed'|'timeout'|'cancelled'|'skipped'|'env_not_ready'
  failureKind: ''|'format_drift'|'output_too_large'|'exit_unexpected'|'parser_error'|'env'; envReason?: string; exitCode: number; durationMs: number
  tool: string; toolVersion: string; errorCount: number; warningCount: number; infoCount: number; totalCount: number; truncated: boolean; outsideScopeCount: number }
type QualityRun = { id: string; worktreeId: string; headCommit: string; indexCommit: string; indexBasis: IndexBasis[]; scope: 'worktree'|'changed'|'commitRange'; baseCommit?: string
  profile: string; status: 'queued'|'running'|'succeeded'|'failed'|'cancelled'; source: 'local'|'ci'; startedAt: string|null; finishedAt: string|null
  summary: { error: number; warning: number; info: number; stepsTotal: number; stepsWithFindings: number; stepsFailed: number; stepsEnvNotReady: number; outsideScope: number; truncated: boolean }
  steps: QualityStep[]; errorCode?: string; treeFingerprint?: string; dirty?: boolean; workTreeChangedDuringRun: boolean; scopeWidened: boolean
  ci?: { provider: string; headSha: string; url?: string; fetchedAt: string; staleAfter?: string } }
type GateResult = 'pass'|'warn'|'fail'|'unknown'
type QualityGate = { verdict: GateResult
  reasons: { check: string; observed: string; threshold: string; result: GateResult; code?: string; params?: Record<string,string>; runId?: string; waivedCount?: number; category?: string; tool?: string }[]
  mode: 'inform'|'block'; profile: string               // "<name>@<scope>/v<version>"
  basedOn: { runIds: string[]; indexCommit: string; stale: boolean; headCommit?: string; baseCommit?: string; evaluatedAt?: string; profileVersion?: number } }
type QualityWaiver = { id: string; subjectKind: 'finding'|'structure_finding'|'check'; subjectKey: string; scope: string; reason: string; createdBy: string; createdAt: string; expiresAt: string; revokedAt?: string }
type RunnableProfile = { id: string; title: string; kind: string; ready: boolean; heavy: boolean; scopes: string[]; missing: { check: string; reason: string; hint?: string }[]; suite?: string[] }
type QualityProfile = { name: string; mode: 'inform'|'block'; definition: { schemaVersion: 1
  checks: { id: string; profile: string; category: string; required: boolean; maxErrors?: number; maxWarnings?: number; maxFailed?: number }[]
  findings: { countScope: 'changedFiles'|'all'; blockingSeverities: string[]; warnBudget: number }
  coverage: { required: boolean; diffCoverageWarnBelow: number|null; diffCoverageFailBelow: number|null }
  structure: { newLayerViolationErrorFails: boolean; newLayerViolationWarningWarns: boolean; newCyclesWarn: boolean }
  freshness: { indexMustMatchHead: boolean } }; version: number }
type QualityTrendPoint = { turnKey: string; headCommit: string; baseCommit: string; profileRef: string; verdict: GateResult
  counts: { error: number; warning: number; info: number; byCategory: Record<string, number> }
  metrics: { diffCoverage?: number; newLayerViolations?: number; newCycles?: number; testsFailed?: number }   // vắng = không có số
  runIds: string[]; indexCommit: string; source: 'local'|'ci'; createdAt: string }
type CoverageReport = { source: 'measured'|'estimated'; language: 'go'|'ts'|'mixed'; mode?: 'set'|'count'|'atomic'|'v8'; headCommit: string; baseCommit: string; dirty: boolean
  totals: { stmts?: number; covered?: number; pct?: number; changedSymbolsTested?: number; changedSymbolsUntested?: number; changedSymbolsUnknown?: number }
  diff: { changedExecutable: number; covered: number; uncovered: number; diffCoverage: number|null; reason?: string; partial: boolean; excludedFiles: { path: string; reason: string }[] } | null
  files: { path: string; stmts: number; covered: number; pct: number; uncoveredRanges?: [number, number][] }[]; truncated: boolean; totalCount: number
  toolVersions: Record<string,string>; estimatedNote?: string }
type CiComparison = { profile: string; headCommit: string
  local: { runId?: string; status?: string; finishedAt?: string; dirty?: boolean }; ci: { runId?: string; status?: string; url?: string; fetchedAt?: string; sha?: string }
  relation: 'agree_pass'|'agree_fail'|'local_pass_ci_fail'|'local_fail_ci_pass'|'local_only'|'ci_only'|'ci_pending'|'sha_mismatch'|'not_comparable'; reasonsHint?: string[] }
type AgentTurn = { id: string; clientTurnId: string; agentType: string; model?: string; source: 'renderer'|'hook'|'both'; startedAt?: string; endedAt: string; interrupted: boolean
  baseHeadCommit?: string; endHeadCommit: string; treeDirtyEnd: boolean; filesChangedCount: number
  commandsSummary: { v: 1; totalToolUses: number; commands: { name: string; sub?: string; category: 'test'|'lint'|'typecheck'|'build'|'install'|'git'|'other'; count: number }[]; toolCounts: Record<string, number>; truncated: boolean }
  claims: { v: 1; items: { kind: 'tests_pass'|'tests_fail'|'lint_clean'|'typecheck_clean'|'build_ok'|'all_done'; basis: 'ran_command'|'stated'; confidence: 'medium'|'low'; evidence?: string
    agreement?: 'consistent'|'contradicted'|'unverified'|'not_claimed'; agreementReason?: string; verifyingRunId?: string }[] }
  gate?: { verdict: GateResult; evaluatedAt: string } }
type RequirementEvidence = { kind: 'change'|'test'|'check_run'|'manual_confirmation'; ref: string; label: string; confidence: 'explicit'|'derived'|'inferred'
  matchedBy: 'satisfies'|'commit_trailer'|'path_hint'|'name_overlap'|'test_edge'|'check_profile'|'user_confirmed' }
type Requirement = { key: string; text: string; origin: 'structured'|'checklist_heuristic'|'title_only'; verifyHint: 'test'|'metric'|'manual'|'review'|null; retired: boolean
  state: 'has_evidence'|'partial'|'no_evidence'|'manual_pending'|'unknown'; evidence: RequirementEvidence[] }
type RequirementTrace = { subject: { source: 'task'|'request'; taskId?: string; requestId?: string; taskNumber?: number; externalRef?: { provider: string; ref: string }
  worktreeRef: string; headCommit: string; baseCommit: string; indexCommit: string; indexStale: boolean }
  linkConfidence: 'explicit'|'derived'|'inferred'|'none'; requirements: Requirement[]; unlinkedChanges: { file: string; symbols: string[]; reason: string }[]
  summary: { total: number; hasEvidence: number; partial: number; noEvidence: number; unknown: number }; warnings: string[] }
type AiReviewSummary = { summary: string; risks: { text: string; refs: string[] }[]; readFirst: { file: string; why: string }[]; model: string
  level: 'metadata'|'diff'; promptVersion: string; inputDigest: string; generatedAt: string; refsDropped: number }
type ReviewReportModel = { schemaVersion: 1
  subject: { branch: string; baseRef: string; headCommit: string; baseCommit: string; indexCommit: string; indexStale: boolean; provider: string; turnKey: string|null }
  reproducibility: { profileRef: string; toolVersions: Record<string,string>; runIds: string[]; modelDigest: string }
  summary: { files: number; added: number; removed: number; symbols: number; flows: number; components: string[] }
  risk: { level: 'LOW'|'MEDIUM'|'HIGH'|'CRITICAL'|'UNKNOWN'; reasons: { code: string; params?: Record<string,string> }[] }
  gate: { verdict: GateResult; reasons: QualityGate['reasons']; waivers: { count: number; earliestExpiry?: string } }
  findings: { counts: { error: number; warning: number; info: number; byCategory: Record<string, number> }
    top: { ruleId: string; severity: string; category: string; file: string; line: number; message: string; origin: 'quality'|'structure' }[] }
  contracts: { protoRpc: { name: string; change: string; breaking: boolean }[]; wsChannels: { name: string; change: string; breaking: boolean }[]; tables: { table: string; service: string; op: 'added'|'altered'|'dropped'; breaking: boolean }[] }
  readingOrder: { n: number; file: string; reason: string; symbols: string[] }[]
  diagrams: { kind: 'components'|'erd'|'flow'; mermaid: string; alt: string[]; truncated: boolean }[]
  limits: { truncated: { findings: boolean; readingOrder: boolean; diagrams: boolean }; totalCounts: Record<string, number> }; warnings: string[] }
```
Quy tắc hiển thị bắt buộc: `unknown` luôn "Chưa đủ dữ liệu để kết luận" (biểu tượng khác `pass`), không bao giờ màu/biểu tượng `pass`; cấm câu "an toàn", "đã đáp ứng", "AI đã review"; không điểm số đơn; `mode:'block'` vẫn chỉ cảnh báo (O9); nhãn "Tóm tắt do AI suy luận" + `model` + `level`; `CoverageReport.source:'estimated'` hiển thị khác `measured`; `QualityFinding` **không** có dòng nguồn; `Finding` và `QualityFinding` hai nguồn tách (một dock, không trộn).

## 5. Kênh push và đăng ký

Đăng ký: gọi `codeIntel.subscribe` một lần mỗi kết nối WS (lần hai thay lần đầu) với `{selectors?: WorktreeSel[]}`; **không** mở khi `settings.get` báo `effective.codeIntelEnabled=false`. Ack đầu: `null`. Sau đó mỗi sự kiện là **một object có `event`** (PQ-11):
- **Native**: `{"type":"push","channel":"<tên>","args":[obj]}`, `channel` ∈ `codeIntel.changed|codeIntel.reindexProgress|codeIntel.quality.progress|codeIntel.quality.finished|codeIntel.quality.gateChanged`.
- **Session-client**: `{"id": <id subscribe>,"ok":true,"result": obj,"streaming":true}` (không có tên kênh) → client **chỉ** dùng `obj.event`; khi stream đóng: `{"result":{"type":"end"}}`.

```ts
type PushBase = { event: string; projectId: string; worktreeId: string; occurredAt: string }
type PushChanged = PushBase & { event: 'changed'; tools: string[]; commit?: string; headCommit?: string; indexedAt?: string; stale: boolean
  indexScope?: IndexScope; freshness?: Freshness; reason: 'index_changed'|'reindex_finished'|'head_changed'|'resync'|'overflow'; resync?: true }
type PushReindexProgress = PushBase & { event: 'reindexProgress'; jobId: string; stage: string; percent: number | null; message: string; status?: 'queued'|'running'|'succeeded'|'failed'|'cancelled' }
type PushQualityProgress = PushBase & { event: 'quality.progress'; runId: string; stage: string; stepIndex?: number; stepCount?: number; percent: number | null; message: string }
type PushQualityFinished = PushBase & { event: 'quality.finished'; runId: string; status: 'succeeded'|'failed'|'cancelled'|'interrupted'; summary: QualityRun['summary']; headCommit: string }
type PushGateChanged = PushBase & { event: 'quality.gateChanged'; headCommit: string; previousVerdict: GateResult|null; verdict: GateResult; profile: string }
```
- Không mang đồ thị, mã nguồn, đường dẫn tuyệt đối, `tenantId`. ≤ 1 KiB. `percent: null` = chưa biết.
- Khi service/stream đứt: gateway phát **một** `changed` với `resync:true` rồi đóng; client **phải** mở lại (backoff 1 s → 30 s) và tải lại view đang xem (`codeIntelResyncCounter`). Gateway không tự mở lại.
- Quy tắc client: `changed` → huỷ cache worktree + đánh dấu chip index cũ, **không** tự tải lại giữa lúc tương tác (chip "Có dữ liệu mới"); `reindexProgress` → slice job; `quality.finished` → tải lại `gate`, `runs`, `findings`; `gateChanged` → tải lại `gate`.
- Số luồng: 1/kết nối; ≤ `CODE_INTEL_MAX_STREAMS` (500)/replica; vượt → `CODEINTEL_RATE_LIMITED`. Dự phòng polling khi không có stream: `status` mỗi 30 s, `reindexStatus` mỗi 2 s khi `running`, `quality.run` mỗi 2 s.

## 6. Cờ và phát hiện tính năng

- `codeIntel.settings.get` lúc khởi động (và mỗi 60 s khi tab Review mở). `effective.codeIntelEnabled=false` ⇒ ẩn mọi lối vào, **không** gọi kênh nào khác, không mở `subscribe`. `qualityGateEnabled=false` ⇒ ẩn phần chất lượng.
- **Không** phát hiện bằng `typeof window.api.codeIntel` (web bọc `window.api` bằng Proxy `withFallback`). Capability `code-intel.v1` và `quality-gate.v1` trong `status.get` là **tuỳ chọn** (nếu backend phát) — nguồn chuẩn là `settings.get`.
- Gateway: `CODE_INTEL_SERVICE_ADDR` rỗng ⇒ mọi kênh `CODEINTEL_UNAVAILABLE` (client `kind:'unsupported'`, ẩn tính năng, không toast).

## 7. Môi trường frontend (đã đọc trong CR; chưa kiểm chứng mã)

- Preload Electron thật ở `desktop/src/preload/index.ts` (ngoài `frontend/`, chưa có `mcp`); Electron local `environmentId === null` ⇒ `unsupported` cho tới khi preload có `codeIntel`. Web: `web-code-intel-api.ts`.
- Mỗi lời gọi mang `environmentId` của môi trường sở hữu worktree (`getRuntimeEnvironmentIdForWorktree`), không phải môi trường toàn cục.
- `window.api.codeIntel.call({environmentId, method, params, timeoutMs})` trả **phong bì** `{ok, result|error:{code,message}}`; client tách mã bằng regex mục 2.3 **từ `error.message`**. `subscribeEvents(...)` ref-count.

## 8. Mobile (CR-062; PQ-36)
Mobile không đi qua gateway: `client.sendRequest('codeIntel.reviewSummary', { worktree: 'id:<worktreeId>', scope?: 'branch' })` tới host desktop (`MOBILE_RPC_METHOD_ALLOWLIST`). Kết quả (đọc thủ công, enum lạ ép về giá trị đã biết):
```ts
type MobileReviewSummary = { available: boolean; reason?: 'flag_off'|'no_binding'|'index_missing'|'tool_unavailable'
  index?: { state: 'missing'|'building'|'ready'|'stale'; indexedCommit?: string; headCommit?: string; indexedAt?: string }
  counts?: { files: number; symbols: number; flows: number; tables: number; contracts: number; uncovered: number }
  risk?: { level: 'LOW'|'MEDIUM'|'HIGH'|'CRITICAL'|'UNKNOWN'; reasons: string[] }
  findings?: { totalOpen: number; bySeverity: { error: number; warning: number; info: number; high?: number; medium?: number; low?: number }
    items: { key: string; kind: string; severity: string; title: string; summary: string; origin: 'introduced'|'touched'|'preexisting'|'unknown'; filePath?: string; startLine?: number; inChangedFiles: boolean }[]; truncated: boolean }
  stale?: boolean; truncated?: boolean; headCommit?: string }
```
`bySeverity` dùng `error|warning|info` (PQ-06; `high|medium|low` của CR-062 bỏ). ≤ 50 mục; `title`/`summary` host dựng từ `titleKey`+`params`; không `evidence`, không ghi. Việc host tới gateway: điểm mở O-4; host cũ → `method_not_found`/`forbidden` → `unavailable`.

## 9. MCP (CR-041, P2, mặc định tắt)
Mọi kênh `codeIntel.*` nằm trong `excluded_channels.yaml` bằng **một** dòng `codeIntel.*` (category `code-intel-v1`, reason ≥ 20 ký tự, cùng PR đăng ký kênh; `TestChannelInventory`/`TestToolParity`). Nếu O2 mở: 9 tool đọc `codeIntel_status|changeOverlay|readingOrder|impact|symbol|routes|dataFlows|erd|findings` (tên giữ chữ hoa, D7 v5), `Untrusted` cả 9, `MaxResultBytes` nhỏ hơn WS (8–48 KiB); không tool cho `reindex`, `reviewState.*`, `c4.*`, `bindRepo`, `settings.*`, `subscribe`, `quality.*`. Cờ tắt → `CODEINTEL_DISABLED` qua `toolErr`, tool vẫn hiện trong `tools/list`.

## 10. Việc ở frontend (tóm tắt cho `FE-CV-SOL-*`)

| Việc | CR |
|---|---|
| Sao chép kiểu mục 4; parser phong bì; bộ phân loại lỗi theo mục 2.3 (đọc `message`); cập nhật test số mã | 050 |
| Phản ánh đổi vs CR gốc: `status` trả `IndexStatus` đơn; `architecture` trả `{containers, view}`; `reviewState.save` thêm `notes`, `turnMarkers`; tiến độ đọc khoá `stepKey`; `Finding.severity` `error|warning|info` + `origin`; `ContractChange.compatibility`; `QualityTrendPoint`/`CoverageReport` theo backend; ánh xạ `reasons[].category/tool` | 050–062, 087 |
| `subscribe` thay `events.subscribe`; payload có `event` | 050 |
| Gửi `{projectId, worktreeId}` mọi kênh | tất cả |
| Không dùng `send`; không `ifNoneMatch` bắt buộc | 050 |

*Kết thúc `CONTRACT-codeintel-ui-api.md`.*
