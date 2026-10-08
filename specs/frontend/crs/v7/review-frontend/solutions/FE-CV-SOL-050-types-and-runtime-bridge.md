# FE-CV-SOL-050-types-and-runtime-bridge: Kiểu mirror, cầu nối `window.api.codeIntel`, client có phân loại lỗi, backend giả

> Trạng thái (2026-10-07): 01-08: 6 DONE, 2 PARTIAL (06,08). Lệch: hằng kênh/kiểu/mã lỗi ban đầu không khớp CONTRACT-codeintel-ui-api và đã được viết lại theo hợp đồng.

**CR:** [CR-CV-050](../../../../../../docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md) (phần 2.1, 2.2, 2.3 và backend giả)
**Area:** frontend (`frontend/src/shared`, `frontend/src/preload`, `frontend/src/renderer/src/{web,runtime,test-support}`); một task ngoài `frontend/` (`desktop/src/preload/index.ts`)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) (**chính**: U1-U9, §2 phong bì và lỗi, §3 46 kênh, §4 kiểu, §5 push, §7 môi trường), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-02, PQ-03, PQ-04, PQ-11, PQ-12, PQ-13, PQ-14, PQ-32; §7 G3/G4; §8.1; §9 O-1, O-2; §10), `CONTRACT-codeintel-agent-rpc.md` (frontend **không** gọi agent; chỉ tham chiếu mã lỗi §3 qua bảng của UI-API).
**TDD tham chiếu:** [v5/01-architecture-overview §2.3](../../../../tdd/v5/01-architecture-overview.md) (`window.api` abstraction), [v5/03-runtime-client-layer §2 và Addendum `mcp.*`](../../../../tdd/v5/03-runtime-client-layer.md), [v5/06-web-client §7](../../../../tdd/v5/06-web-client.md) (`web-preload-api`), [v5/07-hooks-and-ipc Addendum MCP](../../../../tdd/v5/07-hooks-and-ipc.md); API: [api/rpc-catalog](../../../../api/rpc-catalog.md), [api/ipc-surface](../../../../api/ipc-surface.md) ("`web-preload-api.ts` intentionally NOT trimmed").

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `frontend/src/renderer/src/web/web-preload-api.ts` (:1-3, :485-491, :797-806, :1297-1305, :3485-3522, :4430-4470), `web/web-mcp-api.ts` (84 dòng), `runtime/runtime-rpc-client.ts` (:1-160), `runtime/runtime-rpc-result.ts`, `shared/runtime-rpc-envelope.ts`, `shared/mcp-contract-conformance.test.ts` (:1-12, :79), `store/slices/mcp-reconnect.ts`, `lib/worktree-runtime-owner.ts` (:140-200), `shared/types.ts` (:247, :482-500), `frontend/src/preload/api-types.ts` (:928-936, :3285-3297), `desktop/src/preload/index.ts` (:4012-4044; không có `mcp`), `desktop/electron.vite.config.ts` (:228-236), `frontend/config/vitest.config.ts`.

Xác nhận đúng như CR-CV-050 và hợp đồng:

- Không có gì cho `codeIntel` ở `frontend/src` (thư mục `components/review-map` không tồn tại). `components/code-review/*` là code chết, **không dùng**.
- `window.api` ở web là Proxy: `installWebPreloadApi` đặt `window.api = withFallback(createWebPreloadApi(), [])` (`web-preload-api.ts:490`); namespace thiếu trả hàm rỗng (`createFallbackProxy`, `getFallbackResult`) nên **không** phát hiện tính năng bằng `typeof window.api.codeIntel` (UI-API §6).
- `callRuntimeResult` (`:3512`) ném `new Error(response.error.message)`; `callEnvironmentEnvelope` (`:3498`) và `callRuntimeEnvelope` (`:3485`) trả phong bì nguyên (kèm `runtimeCallQueuePool.enqueue` và `updateEnvironmentFromResponse`). Vì mã `CODEINTEL_*` nằm ở **tiền tố `message`** (PQ-02) nên cả hai đường đều giữ được, nhưng bridge vẫn dùng đường phong bì để không phụ thuộc chuyện `error.code` bị đổi.
- `mcp` là mẫu: `createMcpApi({callRuntimeResult, openStream})` gắn ở `web-preload-api.ts:797-806`; `openStream` dùng `getClientForEnvironment(environment).subscribe(...)`; khung đầu là ack `null`, các khung sau là object trần (`web-mcp-api.ts:38-47`). Hợp đồng PQ-11 cũng chốt khung session-client là object trần có `event`.
- `web-preload-api.ts` đã có `eslint-disable max-lines` từ trước (dòng 1-3): **không thêm** disable mới, chỉ thêm một dòng gắn namespace (AGENTS.md; api/ipc-surface: không trim file này).

**Correction relative to CR-CV-050 (hợp đồng và mã thật thắng):**

| # | CR-050 ghi | Hợp đồng / mã thật | Quyết định trong solution |
|---|---|---|---|
| 1 | Mọi lời gọi mang `worktreeId` | PQ-04: mang `{projectId, worktreeId}` phẳng trong `args[0]`; `worktreeId` = `worktree_ref` đã chuẩn hoá; chuỗi ≤ 512, `projectId` ≤ 64 | Client lấy `projectId` từ store (O-1: `Worktree.projectId ?? Repo.projectId`, cả hai **tuỳ chọn** ở `shared/types.ts:247,487`); thiếu → `unsupported`, không gọi (task 050-09) |
| 2 | `status` trả `IndexStatus[]` | PQ-08, UI-API §3.1: `status` trả **một** `IndexStatus` phẳng, **không** phong bì; có `tools: ToolIndexStatus[]`, `overall` (9 giá trị, chữ HOA) | Kiểu và parser theo UI-API §4.1 |
| 3 | `codeIntel.events.subscribe`, khung `{channel, data}` | PQ-11: kênh `codeIntel.subscribe` `{selectors?: WorktreeSel[]}`; mỗi khung là **một object có `event`**; 5 `event` (`changed`, `reindexProgress`, `quality.progress`, `quality.finished`, `quality.gateChanged`); session-client mất tên kênh | Bridge chỉ phân biệt theo `obj.event`; không đọc tên kênh |
| 4 | 10 mã lỗi, mã ở `error.data.code` hoặc `error.code` | PQ-02/PQ-03: mã là **tiền tố `message`** `CODEINTEL_X: mô tả[ \| {json}]`; `error.code` luôn `"internal"`; không có `error.data`; bảng hợp nhất ~45 mã (UI-API §2.3) | `code-intel-errors.ts` theo bảng hợp nhất; test "số mã" so với **bảng này**, không "đúng 10" |
| 5 | Phát hiện cờ bằng capability `code-intel.v1` hoặc thăm dò `status` | UI-API §6: nguồn chuẩn là `settings.get` (`effective.codeIntelEnabled`); capability chỉ tuỳ chọn | Xử lý ở SOL-050-store-and-query-hooks (task 050-12) |
| 6 | Cạnh tranh khoá `worktreeId` thô vs `id:` | PQ-04: bỏ tiền tố `id:`/`repo:`; ba dạng hợp lệ (UUID, `<repoId>::<path>`, repo id trần); `::workspace:` bị `CODEINTEL_WORKTREE_REF_UNSUPPORTED`. Id worktree của frontend là `` `${repoId}::${path}` `` (`shared/types.ts:483`) | Client **không** thêm tiền tố (khác `toRuntimeWorktreeSelector`, `runtime-worktree-selector.ts:7`); chặn id kiểu `workspace`/floating trước khi gọi |
| 7 | Bridge `window.api.codeIntel.call({environmentId,...})` | `runtimeEnvironments.call` nhận `{selector, method, params, timeoutMs}` (`api-types.ts:3285-3297`, `web-preload-api.ts:1299`); đích local là `window.api.runtime.call({method, params})` (`runtime-rpc-client.ts:73-80`) | `environmentId` trong API bridge được ánh xạ sang `selector`; `callRuntimeRpc` còn gọi `ensureRuntimeEnvironmentCompatible` cho đích môi trường (`:68-70`): bridge không gọi nó (mẫu `callEnvironmentEnvelope`), UI-API không yêu cầu |
| 8 | Phong bì kết quả "hợp 05 và README" | PQ-12: phong bì **phẳng** `{repo?, worktreeId, view, sources[], headCommit, stale, truncated, totalCount, etag, fromCache, generatedAt, notModified?, nextPageToken?, data?}`; `devServerId` không ra UI; `ifNoneMatch` tuỳ chọn | `parseCodeIntelEnvelope` theo đúng dạng này; **không** gửi `ifNoneMatch` (U6) |
| 9 | Không nói về `send` | U2: cấm `send` (fire-and-forget nuốt lỗi) | Bridge chỉ có `call` và `subscribe`; test cấm `send` |
| 10 | Giới hạn kích thước chỉ ở phía backend | UI-API §2.4: `args[0]` ≤ 256 KiB (`reviewState.save`), 96 KiB (`c4.save`), 16 KiB còn lại; `SetReadLimit(320 KiB)` là việc của gateway (O-2) | Client kiểm byte UTF-8 **trước khi gửi** (task 050-07), lỗi `validation` cục bộ, không gửi |
| 11 | CR-050 tự tạo `code-intel-fake-backend.ts` | G4 + phối hợp: FE-CV-TASK-073-02 sở hữu fake backend (`createFakeCodeIntelBackend`) | 050 **dùng**/mở rộng fixture (task 050-08); không bản thứ hai |

**Electron (đã đọc, quan trọng):** `desktop/src` có `renderer/src` riêng (74 file ở `store/slices` so với 204 ở `frontend`) và `electron.vite.config.ts` đặt alias `@` → `src/renderer/src` của desktop (:228-236); `desktop/src/preload/index.ts` không có `mcp`. Nghĩa là bundle Electron hiện **không** dùng mã `frontend/src/renderer`. Chưa kiểm chứng pipeline build thật; xem rủi ro 6.1 và câu hỏi mở 1.

## 2. Hợp đồng áp dụng (trích, không chép lại)

| Việc | Mục hợp đồng |
|---|---|
| Quy ước tham số (một object ở `args[0]`, chặt, cấm khoá danh tính) | UI-API U1, U3, §2.1 |
| Phong bì kết quả và `notModified` | UI-API §2.2, U6, PQ-12 |
| Bảng lỗi hợp nhất, regex tách mã, ánh xạ khi không có tiền tố | UI-API §2.3, U5, PQ-02, PQ-03 |
| Timeout, giới hạn, phân trang opaque | UI-API §2.4 |
| 46 kênh (26 + 20) và quyền | UI-API §3.1, §3.2 |
| Kiểu mirror | UI-API §4.1-§4.6 (§4.7 chất lượng: file riêng, xem 2.1) |
| Push và đăng ký | UI-API §5, PQ-11 |
| Môi trường frontend, `environmentId` theo worktree | UI-API §7 |
| Enum chữ hoa/thường | PQ-32 (`risk.level`, `overall` HOA; còn lại thường) |

## 3. Lệch giữa CR và hợp đồng

Xem bảng "Correction" ở mục 1 (10 dòng). Thêm hai điểm:

- CR-050 2.6 nói backoff "chậm dần tới 5 phút sau 6 lần lỗi" (copy `mcp-reconnect.ts:5-15`); UI-API §5 chốt **1 s → 30 s** và client phải mở lại sau `changed{resync:true}`. Theo hợp đồng (xử lý ở SOL-050-store-and-query-hooks).
- CR-050 2.1 liệt kê kiểu `Finding`/`ContractDiff` "do CR-059 thêm"; hợp đồng đã chốt sẵn (UI-API §4.5). Task 050-01 khai báo **luôn** để mọi solution dùng một nguồn; 059 chỉ thêm hành vi.

## 4. Giải pháp

### 4.1 Cây file

```
frontend/src/shared/
  code-intel-types.ts              (mới) kiểu UI-API §4.1-§4.6 (+ CodeIntelEnvelope<T>, PushEvent*)
  code-intel-quality-types.ts      (mới, chỉ dành tên) §4.7 do FE-CV-SOL-085/087 điền; 050 chỉ tạo file rỗng có chú thích
  code-intel-rpc-methods.ts        (mới) CODE_INTEL_RPC_METHODS (46), CODE_INTEL_PUSH_EVENTS (5), CodeIntelRpcContract
  code-intel-errors.ts             (mới) CODE_INTEL_ERROR_CODES, CodeIntelErrorKind, parseCodeIntelErrorMessage
  code-intel-wire-parsers.ts       (mới) parseCodeIntelEnvelope, parseIndexStatus, parseCodeIntelPushEvent
  code-intel-bridge.ts             (mới) CodeIntelBridgeApi (kiểu dùng chung), createCodeIntelBridge(deps)
  code-intel-contract-conformance.test.ts (mới) so bảng kênh/mã lỗi/push của hợp đồng với hằng số (mẫu mcp-contract-conformance.test.ts)
frontend/src/preload/api-types.ts  (sửa) PreloadApi.codeIntel: CodeIntelBridgeApi (cạnh `mcp`, :936)
frontend/src/renderer/src/web/web-code-intel-api.ts       (mới) createCodeIntelApi(transport)
frontend/src/renderer/src/web/web-preload-api.ts          (sửa, một dòng) codeIntel: createCodeIntelApi({...}) cạnh mcp (:797)
frontend/src/renderer/src/runtime/runtime-code-intel-client.ts       (mới) codeIntelClient.call
frontend/src/renderer/src/runtime/code-intel-error-classification.ts (mới) classifyCodeIntelError
frontend/src/renderer/src/test-support/code-intel-fake-backend.ts    (DO FE-CV-TASK-073-02 sở hữu; 050 chỉ DÙNG và bổ sung fixture)
desktop/src/preload/index.ts       (sửa, NGOÀI frontend/, cần chủ sở hữu desktop duyệt)
```

### 4.2 Bridge (chữ ký)

```ts
// shared/code-intel-bridge.ts
export type CodeIntelWireResponse = RuntimeRpcResponse<unknown>      // shared/runtime-rpc-envelope.ts
export type CodeIntelBridgeApi = {
  // Trả phong bì (kể cả ok:false); KHÔNG ném chuỗi. Mã CODEINTEL_* nằm ở error.message (PQ-02).
  call: (args: { environmentId: string | null; method: CodeIntelMethod; params?: Record<string, unknown>;
                 timeoutMs?: number }) => Promise<CodeIntelWireResponse>
  // Một luồng codeIntel.subscribe cho môi trường; onEvent nhận object có `event`.
  subscribeEvents: (args: { environmentId: string | null; selectors?: WorktreeSel[] },
    handlers: { onEvent: (e: CodeIntelPushEvent) => void; onClose?: () => void; onUnsupported?: () => void }) => () => void
}
export type CodeIntelBridgeDeps = {
  callLocal: (a: { method: string; params?: unknown }) => Promise<CodeIntelWireResponse>
  callEnvironment: (a: { selector: string; method: string; params?: unknown; timeoutMs?: number }) => Promise<CodeIntelWireResponse>
  subscribeEnvironment: (a: { selector: string; method: string; params?: unknown },
    cb: { onResponse: (r: CodeIntelWireResponse) => void; onError?: (e: { message: string }) => void; onClose?: () => void })
      => Promise<{ unsubscribe: () => void }>
}
export function createCodeIntelBridge(deps: CodeIntelBridgeDeps): CodeIntelBridgeApi
```

Quy tắc: `environmentId === null` → `call` đi `callLocal` (backend local sẽ trả `method_not_found`; client coi là `unsupported`), `subscribeEvents` gọi `onUnsupported()` ngay. Khung đầu của luồng là ack `null` (bỏ); khung sau là object; chỉ chuyển tiếp object có `event` thuộc 5 giá trị đã biết (lạ → bỏ, không ném); `{type:'end'}` hoặc `onClose` → `onClose()`. `params` rỗng gửi `{}` (UI-API U1; session-client chuyển `params` làm `args[0]`).

### 4.3 Client renderer và phân loại lỗi

```ts
codeIntelClient.call<M extends CodeIntelMethod>(
  worktreeId: string, method: M, params: Omit<ParamsOf<M>, 'projectId' | 'worktreeId'>,
  opts?: { signal?: AbortSignal; timeoutMs?: number; retryInProgress?: boolean }
): Promise<ResultOf<M>>          // ném CodeIntelRpcError {kind, code, message, data?, retryable}
```

Quy trình: (1) `resolveCodeIntelSelector` (050-09) → `{projectId, worktreeId, environmentId}` hoặc ném `unsupported`; (2) kiểm khoá cấm (U3: `tenantId,userId,deviceId,role,devServerId,workspaceRoot,repo,args,command,cypher` → lỗi lập trình, ném ngay); (3) kiểm kích thước `JSON` UTF-8 theo bảng §2.4 (050-07); (4) `window.api.codeIntel.call`; (5) `ok:false` → `parseCodeIntelErrorMessage(response.error.message)` (regex `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`) → `kind` theo bảng UI-API §2.3; không có tiền tố thì ánh xạ theo `error.code` thô: `method_not_found` → `unsupported`, `forbidden` → `forbidden`, kết nối (`isConnectivityLikeRpcError`, `store/slices/connectivity-status.ts:72`) → `offline`; (6) `ok:true` → `parseCodeIntelEnvelope` (view) hoặc trả nguyên (kênh không phong bì: `status`, `reviewState.*`, `settings.*`, `reindex*`, `c4.*`, `bindRepo`, `dismissFinding`).

`kind` (đầy đủ, khớp cột "Client `kind`" ở UI-API §2.3): `disabled`, `quality-disabled`, `ai-disabled`, `unsupported`, `offline`, `forbidden`, `not-found`, `validation`, `path-not-allowed`, `no-binding`, `tool-unavailable`, `index-missing`, `repo-not-registered`, `ambiguous`, `timeout`, `reindex-in-progress`, `rate-limited`, `too-large`, `tool-failed`, `conflict`, `profile-unknown`, `env-not-ready`, `run-in-progress`, `run-cancelled`, `ai-error`, `unknown`. `CODEINTEL_TIMEOUT` có `data.inProgress` → client tự thử lại (đã ở SOL-050-store-and-query-hooks, tổng ≤ 90 s). `maybeTriggerConnectivityPollAfterRpcFailure` được gọi khi `offline` (hàm có sẵn trong `connectivity-status.ts`; chưa đọc chữ ký đầy đủ, task 050-07 phải đọc).

### 4.4 Backend giả (G4): dùng của 073-02, không tạo bản thứ hai

`test-support/code-intel-fake-backend.ts` (`createFakeCodeIntelBackend`) và `code-intel-fixtures.ts` do [FE-CV-TASK-073-02](../../quality-rollout/tasks/FE-CV-TASK-073-02-code-intel-fake-backend.md) (feature quality-rollout) sở hữu theo hợp đồng G4. Solution này **dùng** nó cho mọi test bridge/client/hook và chỉ **mở rộng fixture** khi cần (task 050-08): `IndexStatus` đủ 9 `overall`, `CODEINTEL_TIMEOUT` có `inProgress`, `VERSION_CONFLICT`. 073-02 phụ thuộc kiểu/bộ phân loại của solution này (050-01, 050-03, 050-07); các test của 050-04/05/07 trước khi 073-02 xong dùng stub cục bộ trong file test, không đưa vào `test-support`.

## 5. Quyết định thiết kế

- Bridge trả phong bì, **không** ném; mã lỗi đọc từ `message` (PQ-02); `error.code` chỉ dùng cho `method_not_found`/`forbidden` (mã thật đã dùng ở runtime).
- Một `createCodeIntelBridge` dùng chung web và Electron, nhận hàm gọi vào (test được bằng giả).
- Hằng số 46 kênh khai báo **một lần** kể cả `quality.*` để test hợp đồng bao trùm; kiểu chất lượng do nhóm quality điền.
- Không `send`, không `ifNoneMatch` (U2, U6). Không thêm thư viện (O5/O12).
- Enum lạ → `'unknown'` ở parser (U4, H5); không ném.

## 6. Phụ thuộc chéo khu vực

| Cần | Ở đâu | Cổng | Ghi chú |
|---|---|---|---|
| Đăng ký 46 kênh, giải mã chặt, `codeIntelChannelError`, `SetReadLimit` | `BE-CV-SOL-040-codeintel-channel-foundation` | **G3** | Trước G3 dùng fake backend (050-08); sau G3 chạy lại cùng test với kênh thật |
| Kênh đọc | `BE-CV-SOL-040-codeintel-view-channels` | G3+ | Hình dạng theo UI-API §3.1 |
| `reviewState.*`, `c4.save`, `reindex`, `subscribe` | `BE-CV-SOL-040-codeintel-write-and-stream-channels` | G3+ | |
| `quality.*` (20 kênh) | `BE-CV-SOL-040-codeintel-quality-channels` | sau cùng | Chỉ hằng số ở 050 |
| Cờ `settings.get/set` | `BE-CV-SOL-073-settings-flag-and-rollout` | G3+ | Xem SOL-050-store-and-query-hooks |
| Preload desktop | chủ sở hữu `desktop/` | — | Task 050-06 |

Fake backend (G4) làm **trước**; không task nào của solution này chặn bởi backend thật.

## 7. Tiêu chí chấp nhận

- [ ] `shared/code-intel-{types,rpc-methods,errors,bridge,wire-parsers}.ts` tồn tại; `CODE_INTEL_RPC_METHODS` có đúng 46 mục (26 + 20) khớp UI-API §3; `CODE_INTEL_PUSH_EVENTS` có 5 giá trị.
- [ ] Bảng mã lỗi hằng số khớp UI-API §2.3 từng mã (test đối chiếu tệp hợp đồng, bỏ qua khi không có tệp như `mcp-contract-conformance.test.ts`).
- [ ] `window.api.codeIntel` khai báo ở `preload/api-types.ts`, cài ở web; phong bì `ok:false` giữ nguyên `error.message`; không có `send`.
- [ ] `classifyCodeIntelError` ánh xạ đủ cột "Client `kind`" và có nhánh `unknown`; `error.code` `"internal"` không bị coi là mã.
- [ ] Worktree không có `projectId`, hoặc không thuộc môi trường nào: `unsupported` **không gọi mạng**.
- [ ] Tham số chứa khoá cấm U3 hoặc `args[1]` bị từ chối ở client; `args[0]` vượt giới hạn byte thì `validation` cục bộ.
- [ ] Electron: hoặc có `codeIntel` ở preload, hoặc có ghi chú chặn trong PR và `useCodeIntelSupport` trả `unsupported`.
- [ ] Không hex mới; không `max-lines` disable mới; `web-preload-api.ts` chỉ thêm một dòng gắn namespace.

## 8. Kiểm thử

Vitest (`pnpm --dir frontend test`, `vitest run --config config/vitest.config.ts`, môi trường mặc định `node`): `shared/code-intel-bridge.test.ts`, `shared/code-intel-contract-conformance.test.ts`, `shared/code-intel-wire-parsers.test.ts`, `shared/code-intel-errors.test.ts`, `web/web-code-intel-api.test.ts` (mẫu `web-mcp-api.test.ts`), `runtime/code-intel-error-classification.test.ts`, `runtime/runtime-code-intel-client.test.ts`, `test-support/code-intel-fake-backend.test.ts`. Chưa chạy bất kỳ test nào.

## 9. Rủi ro và điểm chưa kiểm chứng

- **6.1 Electron**: `desktop/src/renderer` là bản riêng (không có `mcp`), có thể không build từ `frontend/`. Nếu đúng, Electron chỉ nhận Review sau khi đồng bộ renderer (ngoài phạm vi); MVP chạy web trước.
- Chưa đọc `maybeTriggerConnectivityPollAfterRpcFailure` (chữ ký) và `runtimeCallQueuePool` (tác dụng lên stream/hàng đợi).
- Hành vi Proxy `withFallback` với hàm `call` mới chỉ suy từ mã.
- `AbortSignal` chỉ bỏ kết quả, không huỷ RPC phía server.
- Giới hạn đọc mặc định của `coder/websocket` (32 KiB theo tài liệu thư viện) chưa kiểm chứng; nếu O-2 không được duyệt thì `reviewState.save` > 32 KiB sẽ lỗi: client phải tách lời gọi (xem câu hỏi 3).

## 10. Câu hỏi mở

1. Electron có thuộc MVP không (CR-050 mục 7.6; `desktop/src/renderer` là bản riêng)?
2. `ReindexMode` đã chốt `incremental|full` (UI-API §3.1) — không còn mở.
3. O-2: gateway nâng `SetReadLimit` lên 320 KiB hay tách `reviewState.save`? Nếu tách, kiểu `ReviewState.notes` cần "vá" từng phần: hợp đồng chưa có.
4. `withFallback` + `codeIntel.call` trả `undefined` khi bridge không có (web chưa cài): bridge phải trả phong bì `ok:false` giả (`method_not_found`) hay ném? Đề xuất: phong bì giả.

## 11. Danh sách task

| Task | Tên | Priority |
|---|---|---|
| [FE-CV-TASK-050-01](../tasks/FE-CV-TASK-050-01-shared-code-intel-types.md) | Kiểu mirror `code-intel-types.ts` | P0 |
| [FE-CV-TASK-050-02](../tasks/FE-CV-TASK-050-02-rpc-method-constants-and-contract-conformance.md) | Hằng 46 kênh, 5 sự kiện push, test đối chiếu hợp đồng | P0 |
| [FE-CV-TASK-050-03](../tasks/FE-CV-TASK-050-03-error-codes-and-envelope-parsers.md) | Mã lỗi, tách tiền tố `message`, parser phong bì và push | P0 |
| [FE-CV-TASK-050-04](../tasks/FE-CV-TASK-050-04-shared-bridge-factory-and-preload-types.md) | `createCodeIntelBridge` và kiểu `PreloadApi.codeIntel` | P0 |
| [FE-CV-TASK-050-05](../tasks/FE-CV-TASK-050-05-web-code-intel-api.md) | Cài đặt web `web-code-intel-api.ts` | P0 |
| [FE-CV-TASK-050-06](../tasks/FE-CV-TASK-050-06-electron-preload-code-intel.md) | Preload Electron (ngoài `frontend/`) | P1 |
| [FE-CV-TASK-050-07](../tasks/FE-CV-TASK-050-07-renderer-client-and-error-classification.md) | `codeIntelClient` và `classifyCodeIntelError` | P0 |
| [FE-CV-TASK-050-08](../tasks/FE-CV-TASK-050-08-code-intel-fake-backend.md) | Dùng và mở rộng fixture của fake backend 073-02 (G4) | P0 |

Thứ tự: 050-01 → 050-02, 050-03 → 050-04 → 050-05, 050-06 (song song) ; 050-07 cần 050-03, 050-04, và task 050-09 (SOL-050-store-and-query-hooks); 050-08 cần 050-01. Task 050-01 đến 050-04 không cần backend; 050-08 cần FE-CV-TASK-073-02.

## 12. Tham chiếu

`/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md`, `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md`, `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-050-review-frontend-foundation.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/frontend/src/renderer/src/web/web-preload-api.ts`, `/opt/repos/orca/frontend/src/renderer/src/web/web-mcp-api.ts`, `/opt/repos/orca/frontend/src/renderer/src/runtime/runtime-rpc-client.ts`, `/opt/repos/orca/frontend/src/shared/mcp-contract-conformance.test.ts`, `/opt/repos/orca/frontend/src/preload/api-types.ts`, `/opt/repos/orca/desktop/src/preload/index.ts`, `/opt/repos/orca/desktop/electron.vite.config.ts`.
