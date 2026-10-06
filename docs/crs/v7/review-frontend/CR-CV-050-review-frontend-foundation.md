# CR-CV-050 — Nền frontend Review: kiểu, `window.api.codeIntel`, slice, push, i18n, cờ, loại tab `review`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-050 |
| **Tên** | Nền frontend Review: kiểu TypeScript mirror proto, cầu nối `window.api.codeIntel` (Electron và web), client có phân loại lỗi, slice Zustand `code-intel`, nhận push `codeIntel.*`, cờ `code_intel_enabled`, i18n, token màu, loại tab `review` |
| **Loại** | Feature + Refactor (chạm mô hình tab) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-040 (kênh `codeIntel.*`, push, tên tham số); CR-CV-020 (proto `orca.codeintel.v1`, nguồn kiểu để mirror); CR-CV-023/024 (sự kiện index đổi) cho phần push |
| **Mở khoá** | CR-CV-051 đến 062 |
| **Tác động** | `frontend/src/shared/` (file mới `code-intel-*.ts`, sửa `types.ts`, `workspace-session-schema.ts`); `frontend/src/preload/api-types.ts`; `frontend/src/renderer/src/{web,runtime,hooks,lib,store,components/tab-group,components/tab-bar,i18n,assets/main.css}`; ngoài `frontend/`: `desktop/src/preload/index.ts` (xem 2.2) |

---

## 1. Bối cảnh và vấn đề

Series v7 cần một nền chung để mọi lens (CR-CV-053 đến 059) không tự định nghĩa kiểu, tự gọi RPC, tự xử lý lỗi và tự giữ cache. Đã đọc code ngày 2026-10-05:

- **Không có gì cho code intelligence ở frontend** (grep `codeIntel` trong `frontend/src` không có kết quả; `components/code-review/*` là code chết và không dùng).
- **Mẫu cầu nối gần nhất là MCP**: `PreloadApi.mcp: McpBridgeApi` (`preload/api-types.ts:928-936`), cài ở web bằng `createMcpApi` (`web/web-mcp-api.ts`, gắn tại `web/web-preload-api.ts:797`), bọc ở renderer bằng `mcpClient` (`runtime/runtime-mcp-client.ts`), slice `store/slices/mcp-slice.ts` (luồng sự kiện dùng chung đếm tham chiếu, kết nối lại có backoff ở `mcp-reconnect.ts`), bus `lib/mcp-event-bus.ts`, backend giả `test-support/mcp-fake-backend.ts`.
- **Khác biệt quan trọng so với MCP**: lỗi của MCP đi qua `new Error('<MCP_CODE>: ...')` rồi phân tích bằng regex (`runtime/runtime-mcp-error.ts`). Đường `callRuntimeResult` ở web (`web-preload-api.ts:3512-3522`) ném `new Error(response.error.message)` và **mất `error.code`/`error.data`**. Phong bì RPC thật mang `error: {code, message, data?}` (`shared/runtime-rpc-envelope.ts`) và `callRuntimeRpc` giữ lại qua `RuntimeRpcCallError.code` (`runtime/runtime-rpc-result.ts`). Mã lỗi `CODEINTEL_*` (README v7 mục 3.3) phải sống sót tới UI nên cầu nối phải trả phong bì, không ném chuỗi.
- **`window.api` ở web là một Proxy có đường lui**: `installWebPreloadApi` bọc bằng `withFallback` (`web-preload-api.ts:485-491, 4430-4443`); namespace chưa cài đặt không bao giờ `undefined` mà trả hàm rỗng cho kết quả `Promise<undefined>` (`getFallbackResult`, :4466). Hệ quả: **không thể phát hiện tính năng bằng `typeof window.api.codeIntel === 'object'`** như `mcpClient.isBridgeAvailable` (đúng cho Electron, sai cho web).
- **Cầu nối cấp thấp đã có ở cả hai target**: `window.api.runtime.call` (đích local) và `window.api.runtimeEnvironments.call/subscribe` (đích môi trường), được `callRuntimeRpc` và `subscribeRuntimeStreamChannel` (`runtime/runtime-rpc-client.ts:60-90, 100-...`) dùng; `status.get` trả `RuntimeStatus.capabilities[]` (`shared/runtime-types.ts:54-66`) và `runtimeEnvironmentSupportsCapability` (`runtime/runtime-compatibility-cache.ts:205`) đọc nó. Ghi chú trong `subscribeRuntimeStreamChannel`: tiêu thụ đẩy theo kênh (`PushEvent.Channel`) hiện **không có client nào dùng**, mọi push thật là luồng gắn theo request-id (như `mcp.events.subscribe`).
- **Triển khai preload Electron không nằm trong `frontend/`**: `frontend/src/preload/` chỉ có `api-types.ts`. Preload Electron thật ở `desktop/src/preload/index.ts` (4 613 dòng) và `desktop/src/preload/api-types.ts` (3 474 dòng, ngắn hơn bản frontend 3 661 dòng). `grep mcp desktop/src/preload/index.ts` không có kết quả, nghĩa là **bản desktop đang lệch sau frontend** (không có `mcp`). Cần đối chiếu trước khi nói "Electron có `window.api.codeIntel`".
- **Mô hình tab**: `TabContentType = 'terminal'|'editor'|'diff'|'conflict-review'|'check-details'|'browser'|'simulator'` và `WorkspaceVisibleTabType = 'terminal'|'editor'|'browser'|'simulator'` (`shared/types.ts:799-808`). Mọi loại không phải terminal/browser/simulator hiện được vẽ bằng `EditorPanel` (`components/tab-group/TabGroupPanel.tsx:~353-375`). `simulator` là mẫu gần nhất cho một tab không có bản ghi nền (`ensureSimulatorTab` ở `lib/ensure-simulator-tab.ts`; chạm ít nhất 18 file, liệt kê ở 2.8).
- **Lưu phiên**: `tabContentTypeSchema` và `workspaceVisibleTabTypeSchema` là `z.enum` (`shared/workspace-session-schema.ts:96-104`); khi hydrate, tab không phải terminal/browser/simulator bị loại nếu không có `openFiles` khớp `entityId` (`store/slices/tabs.ts:1912-1920`, `isRenderableTab`).
- **Xoá worktree có hai đường**: `removeWorktree` (gọi `get().prune…?.(liveWorktreeKeys)` ở `store/slices/worktrees.ts:~3729-3738`) và `buildWorktreePurgeState` (đường hàng loạt: `removeProject`, đối soát quét nền, đối soát hydrate; `worktrees.ts:1960`, gọi tại :2454, :2519, :2593, :4988). Test chống rò rỉ hiện có: `generation-records-worktree-removal-leak.test.ts`, `bulk-worktree-purge-terminal-maps-leak.test.ts`, `agent-status-worktree-purge-leak.test.ts`.
- **Token màu**: `main.css` có `--status-success` (+`-background`, `-border`), `--destructive`, `--annotation-highlight`, `--git-decoration-*`, `--chart-1..5` (toàn bộ sắc xanh dương), `--ai-action-accent`; **không có** token cho "đã đổi", "bị ảnh hưởng", "chưa có test", "vi phạm", hay bảng màu danh mục cho khu vực.
- **i18n**: khoá đọc theo tên dạng `auto.components.<Khu>.<Thành phần>.<tên>` trong `i18n/locales/{en,es,ja,ko,zh}.json`, kèm test phủ khoá theo mẫu `i18n/task-jira-link-locale-coverage.test.ts`; `i18n/no-top-level-translate.test.ts` cấm gọi `translate()` ở cấp module.

## 2. Giải pháp đề xuất

### 2.1 Kiểu và hợp đồng dùng chung (CR này sở hữu)

File mới trong `frontend/src/shared/` (tên theo khái niệm, không `utils`/`helpers`):

| File (mới) | Nội dung |
|---|---|
| `code-intel-types.ts` | Mirror kiểu của [05](../../../research/view-code/05-graph-schemas.md) và [08](../../../research/view-code/08-views-and-review-models.md): `CodeIntelResult<T>`, `CodeIntelSourceInfo`, `SymbolRef`, `IndexStatus`, `ArchitectureGraph`, `ModuleGraph`, `SymbolGraph`, `FlowSummary`, `FlowGraph`, `ImpactGraph`, `RouteMap`, `ChangeOverlay` (đã mở rộng `touchedTables[]`, `touchedContracts[]`, `uncoveredSymbols[]`, `violations[]`, `readingOrder[]`), `C4ComponentView`, `DataFlow`, `ErdModel`, `StorageMap`, `ReviewState`, `ReadingProgress`, `C4Override`, `ReindexJob`, `CodeIntelPushEvent`. Kiểu `Finding` và `ContractDiff` do CR-CV-059 thêm vào cùng file. Enum lạ rơi vào `'unknown'`, không ném. |
| `code-intel-rpc-methods.ts` | `CODE_INTEL_RPC_METHODS` (hằng tên kênh, xem bảng dưới), `CodeIntelMethod`, và bản đồ kiểu `CodeIntelRpcContract: {[M in CodeIntelMethod]: {params; result}}` để `codeIntelClient.call` có kiểu chặt. |
| `code-intel-errors.ts` | `CODE_INTEL_ERROR_CODES` (10 mã ở README v7 3.3), `CodeIntelErrorCode`, `CodeIntelErrorKind`. |
| `code-intel-bridge.ts` | `CodeIntelBridgeApi` và `createCodeIntelBridge(deps)` (2.2). |
| `code-intel-wire-parsers.ts` | `parseCodeIntelEnvelope(raw)` (kiểm tra `sources/headCommit/stale/truncated/totalCount/data`, mặc định an toàn), `parseIndexStatus`, `parseCodeIntelPushEvent`. Parser cho từng lens do CR lens của nó viết. |

Bảng kênh (README v7 3.7; tên đã chốt ở đó, tham số là **đề xuất** vì README không nêu, cần CR-CV-040 xác nhận). Mọi lời gọi mang `worktreeId` (không bao giờ nhận tên repo hay đường dẫn tự do, O4):

| Kênh | Tham số (ngoài `worktreeId`) | Kết quả `data` |
|---|---|---|
| `codeIntel.status` | — | `IndexStatus[]` (một mục mỗi công cụ) |
| `codeIntel.reindex` / `reindexStatus` | `mode` / `jobId` | `{jobId}` / `ReindexJob` |
| `codeIntel.structure` | `path?`, `depth?` | `ModuleGraph` |
| `codeIntel.architecture` | `container?` | `{containers: ContainerRef[]; view: C4ComponentView \| null}` (xem 7.2) |
| `codeIntel.dataFlows` / `dataFlow` | `limit`, `offset` / `flowId` | `DataFlowSummary[]` / `DataFlow` |
| `codeIntel.erd`, `storage` | `service?` | `ErdModel[]`, `StorageMap` |
| `codeIntel.subgraph`, `impact`, `symbol`, `routes` | `center/depth/kinds/limit`, `target/direction/depth`, `uid` hoặc `name+file`, `limit` | `SymbolGraph`, `ImpactGraph`, `SymbolDetail`, `RouteMap` |
| `codeIntel.changeOverlay`, `readingOrder` | `scope: ReviewScope` (CR-CV-051) | `ChangeOverlay`, `ReadingOrderItem[]` |
| `codeIntel.findings`, `dismissFinding`, `contractDiff` | CR-CV-059 | CR-CV-059 |
| `codeIntel.reviewState.get` / `save` | `baseCommit, headCommit` / `+ readingProgress, notes, expectedVersion` | `ReviewState` |
| `codeIntel.c4.get` / `save` | `container` / `+ document, expectedVersion` | `C4Override` |
| `codeIntel.bindRepo` | CR-CV-012 | — |
| `codeIntel.events.subscribe` (**mới, đề xuất**) | — | luồng khung `{channel, data}`, khung đầu là ack `null` |

### 2.2 Cầu nối `window.api.codeIntel`

`preload/api-types.ts` thêm vào `PreloadApi` (cạnh `mcp`, :935):

```ts
export type CodeIntelBridgeApi = {
  // Trả phong bì, không ném: giữ error.code / error.data (CODEINTEL_*).
  call: (args: { environmentId: string | null; method: CodeIntelMethod; params?: unknown;
                 timeoutMs?: number }) => Promise<RuntimeRpcResponse<unknown>>
  subscribeEvents: (args: { environmentId: string | null },
    handlers: { onEvent: (e: CodeIntelPushEvent) => void; onClose?: () => void;
                onUnsupported?: () => void }) => () => void
}
```

`environmentId` là môi trường sở hữu worktree (`getRuntimeEnvironmentIdForWorktree`, `lib/worktree-runtime-owner.ts:162`), không phải môi trường đang chọn toàn cục: cùng một repo có thể có worktree local và remote. `null` nghĩa là đích local: `call` đi qua `window.api.runtime.call`; `subscribeEvents` gọi `onUnsupported` (không có backend, đúng như `subscribeRuntimeStreamChannel` từ chối đích local).

**Một cài đặt dùng chung** `createCodeIntelBridge(deps)` ở `shared/code-intel-bridge.ts`, nhận ba hàm `callLocal`, `callEnvironment`, `subscribeEnvironment` có hình dạng của `window.api.runtime.call`, `window.api.runtimeEnvironments.call`, `window.api.runtimeEnvironments.subscribe`. Khung đầu tiên của luồng là ack `null`, các khung sau là `{channel, data}` (`codeIntel.changed` hoặc `codeIntel.reindexProgress`), lọc theo `channel`, bỏ khung không nhận ra.

- **Web (mới)**: `renderer/src/web/web-code-intel-api.ts` gọi `createCodeIntelBridge` với `callEnvironmentEnvelope` và `getClientForEnvironment(...).subscribe` có sẵn (mẫu `createMcpApi` + `openStream` ở :797-806); gắn `codeIntel: createCodeIntelApi(...)` cạnh `mcp:` trong `createWebPreloadApi()`. File `web-preload-api.ts` đã có `eslint-disable max-lines` từ trước (dòng 1-3); **không thêm** disable mới, chỉ thêm một dòng gắn namespace.
- **Electron**: `desktop/src/preload/index.ts` gắn `codeIntel` bằng `createCodeIntelBridge` với `ipcRenderer.invoke('runtimeEnvironments:call', …)` và `subscribeRuntimeEnvironmentFromPreload` (cả hai đã có, :4012-4044), không cần handler mới ở main process. **Chưa kiểm chứng** rằng desktop import được `shared/` của frontend (hai gói tách nhau; xem 6) và file này nằm ngoài `frontend/` nên cần chủ sở hữu desktop duyệt. Cho tới khi có, `useCodeIntelSupport` coi Electron local là `unsupported`.
- **Phát hiện tính năng**: không dùng `typeof`. Dùng `useCodeIntelSupport` (2.4).

### 2.3 Client renderer và phân loại lỗi

`renderer/src/runtime/runtime-code-intel-client.ts` (mới):

```ts
codeIntelClient.call<M extends CodeIntelMethod>(worktreeId, method, params, opts?: {signal?, timeoutMs?})
  : Promise<CodeIntelResult<ResultOf<M>>>   // ném CodeIntelRpcError
```

Quy trình: (1) `environmentId = getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId)`; (2) `window.api.codeIntel.call(...)`; (3) `unwrapRuntimeRpcResult` (ném `RuntimeRpcCallError`); (4) `parseCodeIntelEnvelope`; (5) bắt lỗi và đổi sang `CodeIntelRpcError {kind, code, retryable, candidates?, cause}` bằng `classifyCodeIntelError` (`runtime/code-intel-error-classification.ts`, mới). Hỗ trợ huỷ bằng `AbortSignal` (bỏ kết quả, không huỷ RPC phía server). Nếu `environmentId === null` thì ném `kind:'unsupported'` ngay, không gọi.

Mã lỗi ở README v7 3.3 nằm ở `error.data.code`; một số lớp trên có thể đặt vào `error.code`. **Chưa kiểm chứng** vị trí thật, nên bộ phân loại đọc cả hai và tiền tố `CODEINTEL_`.

| `kind` | Điều kiện | UI (chi tiết ở CR-CV-051) |
|---|---|---|
| `unsupported` | `RuntimeRpcCallError.code === 'method_not_found'` (mã thật, đã dùng ở `lib/workspace-port-actions.ts:292`), hoặc không có môi trường | Ẩn lối vào, không lỗi, không toast |
| `disabled` | `forbidden` kèm cờ tắt, hoặc mã `CODEINTEL_DISABLED` nếu CR-CV-040 thêm (7.1) | Như `unsupported` |
| `tool-unavailable` | `CODEINTEL_TOOL_UNAVAILABLE` | Lỗi persistent inline có hướng xử lý |
| `index-missing` | `CODEINTEL_INDEX_MISSING` | Trạng thái rỗng + "Lập chỉ mục" |
| `repo-not-registered` / `path-not-allowed` | `CODEINTEL_REPO_NOT_REGISTERED` / `CODEINTEL_PATH_NOT_ALLOWED` | Lỗi inline + giải thích (không lộ đường dẫn thô) |
| `ambiguous` | `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates` | Danh sách chọn |
| `timeout` | `CODEINTEL_TIMEOUT`, timeout truyền tải | Lỗi inline + Thử lại |
| `reindex-in-progress` | `CODEINTEL_REINDEX_IN_PROGRESS` | Gắn vào job đang chạy |
| `too-large` | `CODEINTEL_OUTPUT_TOO_LARGE` | Gợi ý thu hẹp phạm vi |
| `tool-failed` | `CODEINTEL_TOOL_FAILED` | Lỗi inline + chi tiết có thể sao chép |
| `offline` | `isConnectivityLikeRpcError(err)` (`store/slices/connectivity-status.ts`, đã export; nhận `timeout`, `agent_not_connected`, `relay_starting`, `worker_cold`…); lỗi logic thông thường không thuộc loại này | Trạng thái ngoại tuyến, giữ dữ liệu cache |
| `conflict` | mã xung đột phiên bản (7.1; tạm nhận `conflict`, `aborted`) | Tải lại và gộp (CR-CV-052/055) |
| `validation` | `CODEINTEL_INVALID_PARAMS` | Lỗi cạnh trường nếu có |
| `forbidden` | `forbidden` | Khoá thao tác ghi, giữ đọc |
| `unknown` | còn lại | Lỗi inline chung kèm mã thô |

Khi gặp `offline`, gọi `maybeTriggerConnectivityPollAfterRpcFailure(err, target)` (có sẵn) để cập nhật `connections`. `stale` và `truncated` là cờ trong kết quả (README 3.3), không phải lỗi.

### 2.4 Cờ `code_intel_enabled` và `useCodeIntelSupport`

README v7 O8 chỉ nêu cờ theo tenant, không nêu **cách frontend đọc**. Chưa có cơ chế cờ tenant ở frontend (đã rà `hooks/`, `store/slices/settings.ts`; chỉ có `capabilities[]` của `status.get`). Đề xuất:

1. Ưu tiên: backend quảng bá capability `code-intel.v1` trong `status.get` khi cờ bật (cần CR-CV-040); frontend đọc bằng `runtimeEnvironmentSupportsCapability(envId, 'code-intel.v1')`, tái dùng bộ nhớ đệm `status.get` có TTL 60 s.
2. Dự phòng: thăm dò một lần `codeIntel.status` cho worktree đang mở. Kết quả ok hoặc mọi lỗi `CODEINTEL_*` nghĩa là **enabled** (tính năng có mặt; lỗi là của repo cụ thể); `method_not_found` nghĩa là `unsupported`; `forbidden` nghĩa là `disabled`.

`hooks/useCodeIntelSupport.ts` (mới) trả `{state: 'unknown'|'enabled'|'unsupported'|'disabled', environmentId}`; cache theo `environmentId` trong slice (`codeIntelSupportByEnvironment`), TTL 60 s cho trạng thái không-enabled (như `RUNTIME_CAPABILITY_STATUS_TTL_MS`), reset khi đổi môi trường. `unsupported` và `disabled` được xử lý như nhau ở UI: không có nút, không có tab, không lỗi.

### 2.5 Slice `code-intel`

`store/slices/code-intel.ts` (mới), đăng ký ở `store/index.ts` (cạnh `createMcpSlice`, :55-57 và :124-126), `store/types.ts`, và **`store/slices/store-test-helpers.ts`** (`createTestStore` hiện liệt kê từng slice; thiếu thì test rò rỉ không chạy được):

```
CodeIntelSlice {
  codeIntelSupportByEnvironment: Record<string, {state; checkedAt}>
  codeIntelIndexStatusByWorktree: Record<worktreeId, {statuses: IndexStatus[]; meta: ResultMeta|null;
                                    loadedAt; loading; error: CodeIntelRpcErrorSnapshot|null}>
  codeIntelReindexByWorktree: Record<worktreeId, {jobId; stage; percent; message; startedAt;
                                    state:'running'|'finished'|'failed'; errorCode?}>
  codeIntelResultsByWorktree: Record<worktreeId, Record<queryKey, CachedCodeIntelResult>>   // LRU 16/worktree
  codeIntelEventsState: 'off'|'connecting'|'on'|'polling'
  codeIntelResyncCounter: number
  loadCodeIntelIndexStatus(worktreeId, {force?}) / requestCodeIntelReindex(worktreeId, mode)
  applyCodeIntelEvent(event) / invalidateCodeIntelWorktree(worktreeId)
  readCodeIntelResult(worktreeId, queryKey) / writeCodeIntelResult(worktreeId, queryKey, value)
  startCodeIntelEvents(): () => void            // ref-count, một luồng (mẫu startMcpEvents)
  pruneCodeIntelWorktrees(liveWorktreeIds: ReadonlySet<string>)
  resetCodeIntel()
}
```

- `queryKey = "<view>|<scopeKey>|<paramsHash>"`; `scopeKey` chứa commit đã biết phía client (`mergeBase..headOid` từ `GitBranchCompareSummary`, `store/slices/editor.ts:696`) để khoá theo commit. Mỗi giá trị cache giữ `headCommit` và `stale` của phong bì. Tối đa 16 mục mỗi worktree, LRU theo lần đọc; tối đa 6 worktree giữ cache (loại worktree cũ nhất): một kết quả có thể tới ~1 500 nút (README 3.2), nên không để tăng vô hạn.
- `applyCodeIntelEvent`: `changed` → `invalidateCodeIntelWorktree` (xoá cache của worktree, đánh dấu `IndexStatus` cũ, tăng `codeIntelResyncCounter`); `reindexProgress` → cập nhật `codeIntelReindexByWorktree`; phát lại qua `lib/code-intel-event-bus.ts` (mới, mẫu `mcp-event-bus.ts`, nuốt lỗi từng listener).
- Mọi action trả object một phần, không mutate (ghi chú ở `slices/task.ts` về lỗi thay thế toàn state).
- **Không rò rỉ khi xoá worktree.** Hằng `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` (đặt ở `code-intel.ts`) liệt kê khoá theo `worktreeId`; `pruneCodeIntelWorktrees` xoá khoá khỏi mọi map trong danh sách. CR-CV-051 và 052 thêm khoá của chúng vào hằng này. Gọi ở **hai** chỗ: `removeWorktree` (cạnh `prunePullRequestGenerationRecords?.(liveWorktreeKeys)`, `worktrees.ts:~3737`, dạng optional chain vì một số assembly test không có slice) và `buildWorktreePurgeState` (đường hàng loạt; thêm các bản đồ đã lọc vào object trả về, như `rightSidebarTabByWorktree` ở :2262). Huỷ cả luồng chờ/timer theo worktree (module-level) khi prune.

### 2.6 Push `codeIntel.changed` và `codeIntel.reindexProgress`

`startCodeIntelEvents` mở **một** luồng cho mỗi môi trường đang có worktree mở Review (ref-count; `unsubscribe` ở web chỉ bỏ callback cục bộ, server giữ goroutine tới khi đóng socket, như đã ghi ở `mcp-reconnect.ts`). Kết nối lại: backoff 1 s tới 30 s, chậm dần tới 5 phút sau 6 lần lỗi, luồng sống ≥ 30 s tính là khoẻ (cùng hằng số với `mcp-reconnect.ts`; **sao chép** vào `store/slices/code-intel-stream-reconnect.ts`, không import tên `Mcp*`). Sau mỗi lần nối lại tăng `codeIntelResyncCounter` để lens tải lại.

Nếu `onUnsupported` hoặc `method_not_found` ở `codeIntel.events.subscribe`: chuyển `codeIntelEventsState='polling'`, gọi `codeIntel.status` mỗi 30 s cho worktree Review đang hiển thị bằng `installWindowVisibilityInterval` (`lib/window-visibility-interval`, đã dùng ở `connectivity-status.ts`), và `codeIntel.reindexStatus` mỗi 2 s chỉ khi có job `running`. `useCodeIntelEvents()` (mới) gắn một lần ở `App.tsx`.

### 2.7 Hook (mới, `renderer/src/hooks/`)

| Hook | Trả về | Ghi chú |
|---|---|---|
| `useCodeIntelSupport()` | `{state, environmentId}` | 2.4 |
| `useCodeIntelIndexStatus(worktreeId)` | `{statuses, stale, freshestIndexedAt, headCommit, loading, error, refresh}` | Dùng `IndexStatus` ([05 §2.8](../../../research/view-code/05-graph-schemas.md)) |
| `useCodeIntelQuery(worktreeId, view, params, {enabled})` | `{data, meta, status:'idle'\|'loading'\|'ready'\|'error', error, stale, truncated, refetch}` | Đọc cache trước; huỷ khi gỡ; không gọi khi `enabled=false` hay hỗ trợ khác `enabled`; mọi lens dùng hook này, không tự gọi RPC |
| `useCodeIntelReindex(worktreeId)` | `{start, job, isRunning}` | Nút bị khoá ngay (rubric SSH), hiển thị theo thời lượng ở CR-CV-051 |
| `useCodeIntelEvents()` | void | 2.6 |

Mọi hook huỷ yêu cầu khi gỡ (cờ `cancelled`, như `hooks/useTaskSource.ts`), không dùng `any`.

### 2.8 Loại tab `review`

Quyết định: thêm `'review'` vào **cả** `TabContentType` và `WorkspaceVisibleTabType`, theo mẫu `simulator` (không có bản ghi nền trong `openFiles`), **một tab Review mỗi worktree**, `Tab.entityId = worktreeId`. Không dùng chế độ ảo của editor như `check-details` vì (a) `store/slices/editor.ts` đã hơn 4 600 dòng, (b) `OpenFile` ép các trường file không hợp, (c) tab đó chạm gần 25 chỗ cũng như `simulator`. Điểm vào dùng `ensureReviewTab(worktreeId, {targetGroupId?, placement?: 'activeGroup'|'rightSplit', surfacePane?})` ở `lib/ensure-review-tab.ts` (mới, mẫu `ensure-simulator-tab.ts`): tạo hoặc kích hoạt, trả `tabId` hoặc `null` khi `useCodeIntelSupport` không phải `enabled`.

Danh sách chỗ phải sửa, **đã đọc từ các vị trí `'simulator'` hiện có** (chưa kiểm chứng đầy đủ: phải chạy `rg "'simulator'" frontend/src` và `pnpm lint:switch-exhaustiveness` để bắt sót):

| Vị trí | Việc |
|---|---|
| `shared/types.ts:799-808` | Thêm `'review'` vào hai union |
| `shared/workspace-session-schema.ts:96-104` | Thêm vào `tabContentTypeSchema` và `workspaceVisibleTabTypeSchema` |
| `store/slices/tabs.ts:411` (`toVisibleTabType`), `store/slices/worktrees.ts:300` (cùng tên) | Trả `'review'` |
| `store/slices/tabs.ts:1912-1920` (`isRenderableTab`) | `review` luôn renderable (khi cờ tắt, tab hiển thị thông báo; không âm thầm bị loại) |
| `store/slices/tabs.ts:~440-500` (`deriveActiveSurfaceForWorktree`), `editor.ts:~4453` | Giữ `activeTabType='review'` hợp lệ |
| `store/selectors.ts:~86` | Đếm tab Review ở không gian nổi như `simulator` |
| `components/tab-group/TabGroupPanel.tsx:~135-147, 353-375` | Nhánh vẽ `ReviewWorkspace` (lazy, `Suspense`) thay cho `EditorPanel`; `activeTabType='review'` |
| `components/tab-group/useTabGroupWorkspaceModel.ts:~284, 349, 432` | Mục tab, kích hoạt, `setActiveTabType('review')` |
| `components/tab-group/useTabDragSplit.ts:58`, `tab-drag-preview-activation.ts:~54` | Kiểu `tabType` và kích hoạt khi kéo |
| `components/tab-bar/TabBar.tsx` (~176-190, 860-976), `group-tab-order.ts`, `reconcile-order.ts` | Loại mục `review`, thứ tự, trạng thái chọn; icon `lucide-react` `GitPullRequestArrow` hoặc `ScanSearch` (chọn một, test icon tồn tại trong bản `lucide-react` 0.577) |
| `lib/tab-number-shortcuts.ts:~96`, `components/terminal/tab-type-cycle.ts:1,31`, `components/Terminal.tsx:~2020-2128` | Chuyển tab số/vòng, bỏ qua lớp phủ terminal |
| `lib/workspace-tab-palette-search.ts:~41`, `lib/workspace-session.ts:~117-188` | Tìm tab, lưu `activeTabTypeByWorktree` |
| `runtime/sync-runtime-graph.ts:~866` (`isEditorSurfaceTab`) | **Không** đưa Review vào đồ thị runtime đồng bộ cho mobile (chỉ `editor`/`diff` được phản chiếu) |

Lưu phiên: tab Review **được lưu** (vỏ tab) nhưng trạng thái bên trong (phạm vi, lens, chọn) không nằm trong phiên (mặc định lại khi mở; trạng thái đã xem ở backend, CR-CV-052). Rủi ro khi lùi phiên bản: một bản cũ gặp `contentType:'review'` có thể làm hỏng cả lần phân tích phiên vì là `z.enum`; hành vi `catch` ở cấp trên của schema chưa kiểm chứng (6).

### 2.9 i18n

- Khoá đọc theo tên, tiền tố `auto.components.reviewMap.<Thành phần>.<tên>` (UI) và `auto.hooks.codeIntel.<tên>` (hook), đủ `en, es, ja, ko, zh`. Không gọi `translate()` ở cấp module.
- Test phủ khoá `i18n/code-intel-locale-coverage.test.ts` (mẫu `task-jira-link-locale-coverage.test.ts`): mỗi khoá là chuỗi không rỗng ở 5 locale và 4 locale không phải `en` không trùng văn bản tiếng Anh. Mỗi CR lens thêm khoá của mình vào mảng `KEYS`.
- Văn bản không khẳng định điều chưa xác thực (STYLEGUIDE "UI copy must not overclaim"): ví dụ không ghi "an toàn", chỉ "chưa phát hiện vấn đề trong phạm vi index".

### 2.10 Token màu (sửa `assets/main.css`)

Thêm vào `:root` và `.dark` (sau `--annotation-highlight`), rồi bind trong khối `@theme inline` (cạnh `--color-status-success`, :101). Giá trị tham chiếu biến Tailwind như `--ai-action-accent: var(--color-violet-500)` đã làm, không hex:

| Token | Vai trò | Sáng / tối |
|---|---|---|
| `--review-changed` | Đã đổi (viền đậm + chấm) | `var(--color-blue-600)` / `var(--color-blue-400)` |
| `--review-affected` | Bị ảnh hưởng (viền vừa) | `var(--color-teal-600)` / `var(--color-teal-400)` |
| `--review-untested` | Chưa có test (nét đứt) | `var(--color-amber-600)` / `var(--color-amber-400)` |
| `--review-violation` | Vi phạm lớp | `var(--destructive)` (cùng giá trị, tên riêng để đổi sau) |
| `--review-area-1` đến `--review-area-6` | Màu danh mục theo khu vực (treemap, nhóm) | `sky, emerald, amber, rose, indigo, stone` mức 500 / 400 |

Màu chỉ để biểu thị trạng thái; mọi lớp phủ **kèm** dấu không phải màu (viền, nét đứt, icon) để người mù màu vẫn phân biệt (CR-CV-053 mục 2.5). Độ tương phản chưa đo; cần kiểm trên cả sáng và tối.

### 2.11 Cấu trúc file

```
shared/code-intel-{types,rpc-methods,errors,bridge,wire-parsers}.ts   (mới)
renderer/src/web/web-code-intel-api.ts                                (mới)
renderer/src/runtime/{runtime-code-intel-client,code-intel-error-classification}.ts   (mới)
renderer/src/lib/{code-intel-event-bus,ensure-review-tab}.ts          (mới)
renderer/src/store/slices/{code-intel,code-intel-stream-reconnect}.ts (mới)
renderer/src/hooks/useCodeIntel{Support,IndexStatus,Query,Reindex,Events}.ts   (mới)
renderer/src/test-support/code-intel-fake-backend.ts                  (mới, mẫu mcp-fake-backend.ts)
```

## 3. Quyết định thiết kế

- **Cầu nối trả phong bì**, không ném chuỗi như MCP: bảo toàn `error.code`/`data` qua cả hai target.
- **Một cài đặt `createCodeIntelBridge` dùng chung** cho web và Electron, truyền hàm gọi vào: không có hai bản lệch nhau, và test được bằng backend giả.
- **Không phát hiện bằng `typeof`** vì Proxy `withFallback`; dùng capability rồi mới thăm dò.
- **Tab `review` theo mẫu `simulator`**, một tab mỗi worktree; không bắt chước `check-details` (đường editor).
- **Cache trong store có giới hạn** thay vì Map cấp module: cùng một đường dọn khi xoá worktree (hai đường, đều có test).
- **Sao chép logic backoff** thay vì import từ `mcp-reconnect.ts`: tệp đó ghi rõ là singleton của MCP.
- **Không thêm thư viện**; `yaml` và `zod` đã có trong `frontend/package.json` (dùng ở CR-CV-055).

## 4. Tiêu chí chấp nhận

- [ ] `shared/code-intel-types.ts`, `code-intel-rpc-methods.ts`, `code-intel-errors.ts`, `code-intel-bridge.ts`, `code-intel-wire-parsers.ts` tồn tại; `CODE_INTEL_ERROR_CODES` khớp đúng 10 mã ở README v7 3.3 (test so từng mã).
- [ ] `window.api.codeIntel` được khai báo ở `preload/api-types.ts` và cài ở `web/web-code-intel-api.ts`; web gọi `call` với đích môi trường trả đúng phong bì kể cả khi `ok:false` (giữ `error.code`, `error.data`).
- [ ] Cài đặt Electron: có hoặc có ghi chú chặn rõ ràng trong PR; khi chưa có, `useCodeIntelSupport` trả `unsupported` cho worktree local.
- [ ] `classifyCodeIntelError` ánh xạ đủ bảng 2.3 (kể cả `method_not_found` và lỗi kết nối) và có nhánh `unknown`.
- [ ] Worktree không thuộc môi trường nào: `codeIntelClient.call` ném `unsupported` mà không gọi mạng.
- [ ] `unsupported`/`disabled`: không nút, không tab, không toast, không log lỗi.
- [ ] Slice `code-intel` có trong `store/index.ts`, `store/types.ts`, `store-test-helpers.ts`; cache giới hạn 16 mục/worktree và 6 worktree.
- [ ] Xoá worktree bằng `removeWorktree` **và** bằng đường hàng loạt (`buildWorktreePurgeState`) đều xoá mọi khoá trong `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`; worktree còn lại không bị ảnh hưởng.
- [ ] `codeIntel.changed` xoá cache worktree và tăng `codeIntelResyncCounter`; `codeIntel.reindexProgress` cập nhật job; luồng chỉ mở một lần dù nhiều caller; khi luồng không được hỗ trợ thì polling.
- [ ] `ensureReviewTab` không tạo trùng; `review` có trong hai union, hai schema zod; tab Review hydrate lại không bị loại; `rg "'simulator'"` không còn vị trí nào cần `review` mà thiếu.
- [ ] Phiên lưu có tab Review khi cờ tắt: tab hiển thị thông báo, không ném lỗi.
- [ ] Token `--review-*` có ở `:root`, `.dark` và `@theme inline`; không có hex mới trong file TSX/TS của CR này.
- [ ] Khoá i18n có đủ 5 locale; `code-intel-locale-coverage.test.ts` xanh.

## 5. Kiểm thử

Vitest, môi trường mặc định `node`; test component thêm `// @vitest-environment happy-dom`, `@testing-library/react`, `@testing-library/jest-dom/vitest` (mẫu `components/task/__tests__/TaskDAGView.test.tsx`). Chạy bằng `pnpm --dir frontend test`.

- `shared/code-intel-bridge.test.ts`: `callLocal` khi `environmentId=null`; `callEnvironment` khi có; khung ack bị bỏ; lọc `channel`; `onUnsupported` ở đích local; huỷ đăng ký không còn nhận khung.
- `web/web-code-intel-api.test.ts` (mẫu `web-preload-api.test.ts`, `web-mcp-api.test.ts`): phong bì lỗi giữ `error.code/data`; không có môi trường hoạt động thì không ném.
- `runtime/code-intel-error-classification.test.ts`: từng dòng bảng 2.3; mã ở `error.code` và ở `error.data.code`; lỗi kết nối không bị gán `unknown`.
- `runtime/runtime-code-intel-client.test.ts`: phân giải môi trường theo worktree; `unsupported` không gọi mạng; huỷ bằng `AbortSignal`; kết quả sai hình dạng rơi vào lỗi `unknown`, không làm vỡ.
- `store/slices/code-intel.test.ts` (mẫu `mcp-slice.test.ts`: `vi.hoisted` + `vi.mock` client): LRU 16 và 6 worktree; `changed` xoá cache; ref-count luồng; nối lại có backoff (giả đồng hồ); chuyển sang polling.
- `store/slices/code-intel-worktree-removal-leak.test.ts` (mẫu `generation-records-worktree-removal-leak.test.ts`) và `code-intel-bulk-purge-leak.test.ts` (mẫu `bulk-worktree-purge-terminal-maps-leak.test.ts`): gieo từng khoá trong `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` cho hai worktree, xoá một bằng mỗi đường, khẳng định chỉ khoá của worktree bị xoá biến mất; test duyệt chính hằng đó để khoá mới của CR sau không thể quên.
- `lib/ensure-review-tab.test.ts`, `store/slices/tabs.review.test.ts`: tạo, tái dùng, hydrate, `isRenderableTab`; `shared/workspace-session-schema.test.ts` mở rộng cho `review`.
- `hooks/useCodeIntelSupport.test.tsx`: capability có/không, thăm dò, TTL, reset khi đổi môi trường.
- `i18n/code-intel-locale-coverage.test.ts`.
- Cập nhật `components/tab-bar/*` và `TabGroupPanel` test hiện có cho mục `review`.
- E2E (khi CR-CV-040 chạy): web và Electron, môi trường có và không có kênh, xoá worktree khi tab Review mở.
- Chưa chạy bất kỳ test nào; danh sách trên là kế hoạch.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Tên tham số của kênh `codeIntel.*`**, vị trí mã lỗi (`error.code` hay `error.data.code`), và dạng `worktreeId` mà gateway chấp nhận (id thô hay selector `id:` như `toRuntimeWorktreeSelector`) chưa kiểm chứng; README v7 không nêu.
- **Desktop lệch frontend**: bản preload Electron thiếu `mcp`; chưa kiểm chứng desktop import được `shared/` của frontend, hoặc `desktop/src/renderer` có dùng chung mã với `frontend/src/renderer`. Nếu không, chỉ web chạy trọn vẹn ở MVP.
- **Tab mới chạm nhiều chỗ** (≥ 18 file đã đọc); danh sách 2.8 có thể thiếu, một `switch` không đầy đủ có thể lọt nếu bỏ qua `lint:switch-exhaustiveness`.
- **Phiên lưu**: bản cũ không biết `review`; khi nào `z.enum` làm hỏng cả phiên (so với `.catch` ở cấp trên) chưa kiểm chứng.
- **`status.get` capability** cần backend thêm; nếu không có, mỗi môi trường tốn một lần thăm dò `codeIntel.status` (độ trễ SSH ~200 ms).
- Giới hạn cache theo số mục không phản ánh kích thước byte; một vài kết quả ~1 500 nút có thể nặng.
- Không kiểm chứng biểu tượng `lucide-react` được chọn, độ tương phản màu mới, hay hành vi Proxy `withFallback` với hàm `call` mới ngoài suy luận từ mã.

## 7. Câu hỏi mở

1. Cờ tắt thì gateway trả gì: `method_not_found`, `forbidden`, hay mã `CODEINTEL_DISABLED` (không có trong README 3.3)? Đề nghị CR-CV-040 thêm capability `code-intel.v1` và một mã rõ ràng.
2. `codeIntel.architecture` trả được danh sách container không? README 3.6 chỉ có `GetArchitecture`; lens C4 (CR-CV-055) cần liệt kê container trước khi chọn.
3. Tên mã lỗi xung đột phiên bản cho `reviewState.save` và `c4.save` (README 3.3 không có); tạm nhận `conflict`/`aborted`.
4. Luồng sự kiện: `codeIntel.events.subscribe` có được chấp nhận không, hay gateway đẩy theo kênh (`PushEvent.Channel`, hiện không client nào dùng)?
5. Phong bì README 3.2 (`sources, headCommit, stale, truncated, totalCount, data`) và 05 §1 (thêm `repo, worktreeId, devServerId, view`) khác nhau; mirror theo hợp (trường thêm là tuỳ chọn).
6. Desktop (Electron) có nằm trong phạm vi MVP không, hay chỉ web? Quyết định này đổi mức độ chặn của 2.2.
7. `ReindexMode` có những giá trị nào (README chỉ ghi `mode`)? Tạm `incremental` và `full`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (D7, O4, O6, O8, 3.2 đến 3.8)
- `/opt/repos/orca/docs/research/view-code/10-frontend-review-ux.md` (§3, §9, §10), `05-graph-schemas.md`, `08-views-and-review-models.md`
- `/opt/repos/orca/guides/STYLEGUIDE.md`
- `/opt/repos/orca/frontend/src/preload/api-types.ts` (:928-936, :3285-3310), `/opt/repos/orca/desktop/src/preload/index.ts` (:4012-4044)
- `/opt/repos/orca/frontend/src/renderer/src/web/web-preload-api.ts` (:485-491, :797-806, :3485-3522, :4430-4492), `web/web-mcp-api.ts`
- `/opt/repos/orca/frontend/src/renderer/src/runtime/runtime-rpc-client.ts`, `runtime-rpc-result.ts`, `runtime-compatibility-cache.ts`, `runtime-mcp-client.ts`, `runtime-mcp-error.ts`
- `/opt/repos/orca/frontend/src/shared/runtime-rpc-envelope.ts`, `runtime-types.ts`, `types.ts` (:799-808), `workspace-session-schema.ts` (:96-104)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/{mcp-slice,mcp-reconnect,connectivity-status,tabs,worktrees,editor,store-test-helpers}.ts`, `store/index.ts`, `store/types.ts`, `store/selectors.ts`
- `/opt/repos/orca/frontend/src/renderer/src/lib/{worktree-runtime-owner,ensure-simulator-tab,mcp-event-bus,tab-number-shortcuts}.ts`
- `/opt/repos/orca/frontend/src/renderer/src/components/tab-group/{TabGroupPanel.tsx,useTabGroupWorkspaceModel.ts}`, `components/tab-bar/{TabBar.tsx,group-tab-order.ts}`
- `/opt/repos/orca/frontend/src/renderer/src/assets/main.css`, `i18n/task-jira-link-locale-coverage.test.ts`, `store/slices/generation-records-worktree-removal-leak.test.ts`, `bulk-worktree-purge-terminal-maps-leak.test.ts`
- `/opt/repos/orca/specs/frontend/storage/{README,browser-storage-catalog,feature-persistence-matrix}.md`
- Mới: `frontend/src/shared/code-intel-*.ts`, `renderer/src/web/web-code-intel-api.ts`, `runtime/runtime-code-intel-client.ts`, `store/slices/code-intel.ts`, `lib/ensure-review-tab.ts`, `hooks/useCodeIntel*.ts`
