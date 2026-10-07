# AG-CV-TASK-001-09: `codeintel.status`, bảng method, dispatcher và nối vào `route()`

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.7
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-status.ts` (mới), `agent/src/relay/codeintel-method-table.ts` (mới), `agent/src/relay/agent-rpc-dispatch-codeintel.ts` (mới), `agent/src/relay/agent-rpc-dispatch.ts` (sửa), và test: `codeintel-status.test.ts`, `agent-rpc-dispatch-codeintel.test.ts` (mới), `agent-rpc-dispatch.test.ts` (sửa, nếu cần, ở `agent/src/relay/__tests__/`)
**Depends on:** [05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md), [07](./AG-CV-TASK-001-07-codeintel-tool-detection-and-capabilities.md), [08](./AG-CV-TASK-001-08-result-envelope-and-head-commit.md) (và 01, 02, 06 gián tiếp)
**Status:** [x] DONE

## Context

Đây là task tích hợp của nền. `route()` (`agent-rpc-dispatch.ts:297-382`) là chuỗi `dispatchXxxRpc`; khối mới đặt ngay trước `return makeError(… MethodNotFound …)` (`:381`). `extractTraceFields` (`:92`) cần nhánh `codeintel.` chỉ ghi `workspaceRoot`. Contract §4.1: `codeintel.status` **luôn thành công** khi agent chạy (thiếu công cụ/chỉ mục/repo chưa đăng ký không là lỗi); §3.2: method không tồn tại -> `-32601`; mọi method nhận `workspaceRoot`.

Điểm cắm cho solution khác (đừng hiện thực ở đây): probe chỉ mục GitNexus (AG-CV-SOL-002 task 07) và CodeGraph (AG-CV-SOL-003 task 02) đăng ký qua `registerIndexProbe`; `classifyIndexBasis` và `baseRef` (AG-CV-SOL-080); `sqliteReadAvailable` (AG-CV-SOL-003); các method khác thêm dòng vào bảng; kill-switch `ORCA_CODEINTEL_DISABLED` (AG-CV-SOL-073).

## Việc cần làm

1. **Impact trước khi sửa:** `gitnexus impact -r /opt/repos/orca route --direction upstream` và `extractTraceFields`; `route` và `createRpcDispatcher` là điểm vào của mọi RPC nên blast radius có thể HIGH: báo người duyệt, đặt thay đổi <= 12 dòng.
2. `codeintel-method-table.ts`: `export const CODEINTEL_METHODS: Record<string, CodeIntelMethodDefinition>`; `type CodeIntelMethodDefinition = { validate(params): ValidatedParams; handle(params, ctx): Promise<CodeIntelResultBody>; timeoutMs: number }`; `CodeIntelRequestContext = { config, log, signal, deadline, notifier, perf }` (notifier là kiểu `CodeIntelNotificationSink` của SOL-004; ở task này khai báo kiểu hàm và truyền no-op nếu chưa có). Đăng ký duy nhất `codeintel.status` (timeout 25 s). Handler nạp động (`await import('./codeintel-status')`) bọc lỗi nạp -> `TOOL_FAILED`.
3. `codeintel-status.ts`: `validate` = `workspaceRoot` + `baseRef?` (`assertGitRef`); `handle`: Windows -> `TOOL_UNAVAILABLE unsupported_platform`; `resolveCodeIntelRepo` (chấp nhận `REPO_NOT_REGISTERED` thành `binding.gitnexus:null, codegraph:null` thay vì lỗi, vì `status` luôn thành công); `detectCodeIntelTools`; chạy các probe đã đăng ký song song (lỗi một probe -> `{state:'unknown'}` + cảnh báo); `host` (`os.platform()`, `os.cpus().length`, `os.loadavg()[0]`, `os.freemem()`); `limits` (từ task 02, đúng tên trường §4.1). Kết quả qua `buildCodeIntelResult` với `sources` từ các probe `available`, `stale` theo task 08. **Không** điền `indexScope`, `freshness`, `indexRoot`, `mergeBase`, `dirtySinceIndex`, `changedFilesNotInIndex`, `rootMismatch` (AG-CV-SOL-080); để `StatusIndexEnricher` tuỳ chọn.
4. `registerIndexProbe(tool, probe)`: probe mặc định `{state:'unknown'}`.
5. `agent-rpc-dispatch-codeintel.ts`: `dispatchCodeIntelRpc(rpc, config, log, ws, state): Promise<JsonRpcResponse | null>`: `null` nếu `!rpc.method.startsWith('codeintel.')`; method không có trong bảng -> `makeError(rpc.id, AgentErrorCode.MethodNotFound, 'Method not found: …')`; còn lại: `validate` -> deadline (`timeoutMs`) -> `handle`; thành công `{jsonrpc:'2.0', id, result}`; lỗi -> `toErrorPayload` -> `makeError(rpc.id, p.code, p.message, p.data)`; lỗi lạ: `log.error` đầy đủ (kể cả stack), tải trọng chung `CODEINTEL_TOOL_FAILED`. `ctx.notifier` lấy từ `makeNotifier(ws, state)` (SOL-004 thay bằng sink).
6. `agent-rpc-dispatch.ts`: khối `const fromCodeIntel = await dispatchCodeIntelRpc(rpc, config, log, ws, state); if (fromCodeIntel !== null) { return fromCodeIntel }` ngay trước `// ── Unknown method`; `extractTraceFields`: `if (method.startsWith('codeintel.')) { return { workspaceRoot: truncPath(p['workspaceRoot']) } }` đặt **trước** nhánh `method.startsWith('agent.')`... (không xung đột vì tiền tố khác; vẫn đặt cạnh các nhánh khác). Không `max-lines` disable (file hiện 419 dòng).

## Kiểm thử

`agent-rpc-dispatch-codeintel.test.ts` (khuôn `MockWs`, `createWireState` của `agent-rpc-dispatch-misc.test.ts`): method không `codeintel.` -> `null`; `codeintel.nope` -> `-32601`; khoá lạ -> `-32602` `data.code==='CODEINTEL_INVALID_PARAMS'`, `data.field`; `workspaceRoot` xấu -> `-33002`, spawn = 0 (spy); lỗi lạ -> `-32000` không stack; phản hồi có `jsonrpc/id`.
`codeintel-status.test.ts`: repo tạm + registry giả + binary giả -> phong bì đúng hình dạng §4.1 (so với golden JSON đặt cạnh test); không công cụ nào -> `tools.*.available:false`, `binding` null, vẫn thành công; probe ném -> `state:'unknown'`; Windows giả -> lỗi `unsupported_platform`; `baseRef:'-x'` -> `INVALID_PARAMS`; không có trường `args`/tên lệnh CLI ở đầu ra.
Thêm test hồi quy trong `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts`: method lạ không thuộc codeintel vẫn `-32601`; `extractTraceFields` cho `codeintel.status` chỉ có `workspaceRoot` (nếu hàm không xuất, kiểm qua span giả của `rpcTracer`; **chưa kiểm** cách mock tracer ở test hiện có).
Phản chiếu schema (contract §8.3 mục 5): `assertSchemaHasNoForbiddenParams` cho mọi method trong `CODEINTEL_METHODS`.

Lệnh: `pnpm exec vitest run src/relay/codeintel-status.test.ts src/relay/agent-rpc-dispatch-codeintel.test.ts src/relay/__tests__/agent-rpc-dispatch.test.ts`; rồi `pnpm test`.

## Tiêu chí hoàn thành

- [x] `codeintel.status` đúng hình dạng §4.1 phần nền; luôn thành công khi binding phân giải được hoặc thiếu công cụ.
- [x] Trace chỉ có `workspaceRoot`.
- [x] `route()` các method khác không đổi (test hồi quy xanh).
- [x] Không file nào tên `helpers/utils/common/misc`; không `max-lines` disable mới.

## Rủi ro và lưu ý

- Contract §1.1 (Windows -> lỗi) và §4.1 ("luôn thành công") xung đột; làm theo §1.1, ghi vào PR.
- `ws`/`state` chỉ dùng để dựng notifier ở dispatcher; lõi không nhận.
- Hàng đợi/cổng đã ở task 02/05: dispatcher không tự giới hạn.
