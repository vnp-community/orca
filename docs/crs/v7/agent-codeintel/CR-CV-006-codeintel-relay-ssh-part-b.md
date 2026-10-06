# CR-CV-006 — Hỗ trợ `relay-ssh` (Part B, `RelayDispatcher`) cho `codeintel.*`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-006 |
| **Tên** | Đăng ký cùng các method `codeintel.*` trên bề mặt RPC thứ hai của agent (Part B: `relay.ts` + `dispatcher.ts` `RelayDispatcher`), làm rõ khác biệt wire giữa Part A và Part B, và xác nhận đường `relay-ssh` do backend Go khởi tạo |
| **Loại** | Feature (mở rộng phạm vi hỗ trợ) |
| **Priority** | ⚪ P2 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-001, CR-CV-002, CR-CV-003, CR-CV-004, CR-CV-005 (cần các method đã xong để đăng ký) |
| **Mở khoá** | Tính năng review cho dev server nối bằng `relay-ssh` qua app Electron (nếu có nhu cầu, Q1) |
| **Tác động** | `agent/src/relay/relay.ts`, `agent/src/relay/dispatcher.ts` (chuyển `error.data`), file mới `agent/src/relay/codeintel-relay-handlers.ts`; **và** bản triển khai thật `desktop/src/relay/relay.ts`, `desktop/src/relay/dispatcher.ts` cùng các file `codeintel-*.ts` được sao sang `desktop/src/relay/` (xem 2.5) |

---

## 1. Bối cảnh và vấn đề

Các khẳng định dưới đây do người soạn đọc code ngày 2026-10-05.

### 1.1 "relay-ssh" có hai nghĩa trong repo

1. **`relay-ssh` do backend Go khởi tạo = Part A qua chế độ `--stdio`/`--detach`.** `backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go` (đầu tệp) mô tả `adapter/sshrelay` triển khai (SFTP `agent/out/agent.js`) và chạy (`SSH exec channel, node agent.js --stdio`). `agent/src/relay/agent-entry.ts:80-88` xử lý `--stdio` bằng `connectStdio(config, tools, log)` và `:90-108` xử lý `--detach`/`--connect`; `agent-connection-stdio.ts:224-300` (`runDetachedStdioMode`/`runDetachedChildProcess`) dựng một daemon tách rời lắng nghe Unix socket, và **mỗi kết nối socket tạo một `createSession(config, tools, log)`** (cùng `createRpcDispatcher` của Part A, `agent-session.ts:76`; `agent-connection-stdio.ts:272`). Chú thích trong `agent-entry.ts:90-93` ghi `sshrelay.launch()/reattach()` là người gọi duy nhất của `--detach`/`--connect`. Như vậy mọi method thêm vào Part A (CR-CV-001..005) **tự có sẵn** trên đường `relay-ssh` của Go, kể cả `agent.handshake` (`capabilities`, `tools`) vì dùng chung `createSession` (không cần mã mới; cần kiểm thử).
2. **`relay-ssh` theo mô tả cũ = Part B (`RelayDispatcher`).** `specs/agent/api/connection-modes.md` §0 mô tả "Stack B" cho `relay-ssh` với `SshChannelMultiplexer` của backend TypeScript cũ (`backend/src/main/ssh/...`, `agent/src/main/ssh/ssh-channel-multiplexer.ts`). Phần của spec này nói về bản backend TypeScript, trong khi README v7 và các CR khác nhắm tới backend Go. Chưa kiểm chứng backend Go có kết nối vào Part B ở đâu không (`grep` giới hạn trong CR này chỉ thấy đường `--stdio`/`--detach`).
3. **Bản Part B được triển khai thật nằm ở `desktop/`, không phải `agent/`.** `specs/agent/api/compliance-audit-2026-08-15.md` §1: binary `relay.js` cho `relay-ssh` được build từ `desktop/src/relay/relay.ts` (`desktop/config/scripts/build-relay.mjs:19` `RELAY_ENTRY = join(ROOT, 'src', 'relay', 'relay.ts')`, đã xác nhận); `agent/build.mjs` chỉ có entry `agent/src/relay/agent-entry.ts`. Hai cây `agent/src/relay` và `desktop/src/relay` là hai bản độc lập, đồng bộ thủ công, không có script đồng bộ (theo audit; đã thấy hai `relay.ts` và hai `dispatcher.ts` **khác nhau** bằng `diff`). Bản `desktop/` **không có** `relay-auth-status-handlers.ts` (`ls` không thấy) nên cũng chưa có tiền lệ "đăng ký Part A cho Part B" ở đó.

Hệ quả: nếu "hỗ trợ relay-ssh" chỉ nhắm tới Go, **không cần làm gì thêm ngoài kiểm thử** (1.1). CR này tồn tại cho trường hợp ứng dụng Electron vẫn dùng đường Part B (Q1) và để ghi lại các khác biệt.

### 1.2 Khác biệt wire giữa Part A và Part B (đã đối chiếu code)

| Khía cạnh | Part A (`agent-rpc-dispatch*.ts`) | Part B (`RelayDispatcher`, `dispatcher.ts`) |
|---|---|---|
| Đăng ký | `route()` thử lần lượt các `dispatchXxxRpc(rpc, ...)`, mỗi cái là `switch (rpc.method)` trả `JsonRpcResponse \| null` (`agent-rpc-dispatch.ts:297-382`) | `dispatcher.onRequest(method, handler)` (`dispatcher.ts:154-168`); đăng ký trùng chỉ cảnh báo ra stderr rồi ghi đè |
| Chữ ký handler | `(rpc, config, log, ws, state) → Promise<JsonRpcResponse \| null>`; handler tự dựng `{jsonrpc, id, result}` hoặc `makeError(id, code, message, data?)` | `(params, context: RequestContext) → Promise<unknown>`; trả **giá trị kết quả**; lỗi = `throw` |
| Mã lỗi và `data` | `error: {code, message, data?}` giữ nguyên `data` (`makeError`, `agent-rpc-dispatch.ts:408`) | `catch` trong `handleRequest` (`dispatcher.ts:485-491`) gửi `{ code: err.code ?? -32000, message }` — **bỏ `err.data`** (kiểu `JsonRpcResponse.error` có `data?` nhưng đoạn này không điền) |
| Kiểu id | chuỗi hoặc số (`JsonRpcRequest.id`) | số (`protocol.ts` `JsonRpcRequest.id: number`), id theo từng client |
| Thông báo agent → bên gọi | `makeNotifier(ws, state)` theo kết nối (`agent-rpc-dispatch.ts:280`) | `dispatcher.notify(method, params)` phát tới **mọi** client đang gắn (`dispatcher.ts:202-214`); `notifyBulk` có kiểm soát luồng |
| Huỷ | không có | thông báo `rpc.cancel {id}` kích hoạt `context.signal` (`dispatcher.ts:498-503`) |
| Hết hiệu lực | không | `context.isStale()` (client đổi/ngắt); đáp bị bỏ nếu stale (`dispatcher.ts:481-488`) |
| Nhiều client | mỗi kết nối một `createSession` (stdio/detach tạo mới theo socket) | `attachClient`/`detachClient`, trạng thái tuần tự theo client |
| Cấu hình | `AgentConfig` được truyền vào `createRpcDispatcher` | `relay.ts` không có `AgentConfig`; tiền lệ `relay.ts:473-494`: gọi `loadAgentConfig()` một lần |
| Ghi log | `AgentLogger` (`console.*`, `agent-logger.ts:19-23`) | **không** ghi stdout (kênh giao thức); dùng `relayLogLine` (tệp xoay vòng) hoặc stderr (`relay.ts:473-494`) |
| Handshake | `agent.handshake` có `capabilities` và `tools` | `orca-relay-handshake-ok` chỉ có `version`, `platform`, `arch`, `nodeVersion` (`relay-handshake.ts:180-192`); **không có** danh sách khả năng |
| Phát hiện method không có | `MethodNotFound` -32601 | `{code:-32601, message:'Method not found: …'}` (`dispatcher.ts:456-462`) cùng mã |
| Khung tối đa | 16 MiB | 16 MiB (`protocol.ts:8` `MAX_MESSAGE_SIZE`) |
| Cùng tên khác hợp đồng | `preflight.check`, `pty.exit`, `fs.changed` (`gaps-and-findings.md` §4) | |

Không có điểm nào ở bảng cản trở việc đăng ký cùng tên và cùng kết quả; điểm duy nhất ảnh hưởng hợp đồng là **mất `error.data`** (làm hỏng `data.code` chuỗi, `candidates`, `jobId` của README v7 mục 3.3).

### 1.3 Tiền lệ trong repo

`agent/src/relay/relay-auth-status-handlers.ts` (`registerAuthStatusHandlers(dispatcher, config, log)`) là mẫu đúng cho việc "dùng lại handler của Part A cho Part B": gọi lại hàm xử lý trung lập transport (`handleGitHubAuthStatus`) và biến `{result|error}` thành giá trị trả hoặc `throw Object.assign(new Error(message), { code })`. Gọi từ `relay.ts:473-494` kèm `loadAgentConfig()` và logger `relayLogLine`. Hàm `unwrapExternalApiResponse` cũng **làm rơi `data`** của lỗi.

## 2. Giải pháp đề xuất

### 2.1 Lõi trung lập transport (ràng buộc lên CR-CV-001 và CR-CV-004)

Mọi handler trong `codeintel-method-table.ts` (CR-CV-001 2.1) có chữ ký:

```ts
// ví dụ minh hoạ hình dạng, không phải mã cuối
export type CodeIntelRequestContext = {
  config: AgentConfig
  log: AgentLogger
  signal?: AbortSignal                       // Part B: context.signal; Part A: không có
  notifier: CodeIntelNotificationSink        // CR-CV-004: sink gắn với transport hiện hành
}
export type CodeIntelMethod = {
  validate(params: Record<string, unknown>): ValidatedParams     // ném CodeIntelError
  handle(params: ValidatedParams, ctx: CodeIntelRequestContext): Promise<CodeIntelResultBody>
  timeoutMs: number
}
```

Quy tắc: các file `codeintel-*.ts` **không** import `ws`, `WireState`, `makeNotifier`, `makeError` hay bất cứ thứ gì của `agent-rpc-dispatch.ts`/`dispatcher.ts`; chỉ `agent-rpc-dispatch-codeintel.ts` (Part A) và `codeintel-relay-handlers.ts` (Part B) biết transport. Nhờ đó sao chép sang `desktop/src/relay/` không kéo theo phụ thuộc vòng. `CodeIntelNotificationSink` (`(method, params) => void`, CR-CV-004 `codeintel-notification-sink.ts`) là cổng duy nhất để phát `codeintel.reindexProgress`/`codeintel.indexChanged`.

### 2.2 Bộ đăng ký cho Part B (`codeintel-relay-handlers.ts`, mới)

```ts
// minh hoạ hình dạng
export function registerCodeIntelHandlers(dispatcher: RelayDispatcher, config: AgentConfig, log: AgentLogger): void {
  setCodeIntelNotificationSink((method, params) => dispatcher.notify(method, params))
  for (const [method, def] of Object.entries(CODEINTEL_METHODS)) {
    dispatcher.onRequest(method, async (params, context) => {
      try {
        return await runCodeIntelMethod(def, params, { config, log, signal: context.signal, notifier: codeIntelSink })
      } catch (err) {
        throw toRelayThrowable(err)       // Object.assign(new Error(message), { code, data })
      }
    })
  }
}
```

- Tên method, tham số, kết quả, giới hạn: **y hệt Part A** (cùng `CODEINTEL_METHODS`). Kết quả phải qua `JSON.stringify` giống nhau (test tương đương, 5).
- `toRelayThrowable(err)`: `CodeIntelError` → `Object.assign(new Error(message), { code: <số theo CR-CV-001 2.8>, data: { code: 'CODEINTEL_…', … } })`; lỗi khác → `code: -32000` không `data`.
- **Sửa nhỏ `dispatcher.ts`** (cả hai cây) để chuyển `data`: trong `handleRequest` (`dispatcher.ts:485-491`) lấy `const data = (err as { data?: unknown }).data` và gửi `{ code, message, ...(data !== undefined && { data }) }`. Cho phép vì kiểu lỗi đã có `data?`. Ảnh hưởng: mọi handler Part B khác throw `Error` có `.data` sẽ bắt đầu gửi `data` (hiện không có handler nào làm vậy theo đọc nhanh; chưa kiểm chứng toàn bộ). Chạy `gitnexus impact` trên `handleRequest` và `dispatcher.test.ts` trước khi sửa. **Phương án dự phòng nếu không được sửa `dispatcher.ts`:** mã hoá vào `message` dạng `CODEINTEL_TIMEOUT: <mô tả>` (tiền tố chuỗi mã) và nhận ở backend bằng tách tiền tố; `candidates`/`jobId` khi đó phải đi trong kết quả thay vì lỗi (không khuyến nghị; chỉ dùng nếu Q2 bị từ chối).
- **Huỷ:** `context.signal` → tuỳ chọn `signal` của `runCodeIntelTool` (CR-CV-001 2.5): khi bên gọi gửi `rpc.cancel`, tiến trình CLI bị kill, handler trả lỗi bị bỏ vì `isStale()`. Part A không có tính năng này (chỉ timeout).
- **`isStale`**: không cần; `RelayDispatcher` tự bỏ đáp stale.
- **Logger/cấu hình:** gọi trong `relay.ts` ngay sau `registerAuthStatusHandlers(...)` (`relay.ts:494`) theo đúng tiền lệ: dùng `authStatusConfig`/`authStatusLogger` có sẵn (hoặc đặt tên chung nếu đổi). Không `console.log`. `ExperimentalWarning` của `node:sqlite` đi ra stderr (CR-CV-003) nên không phá giao thức; nhưng stderr của `relay.ts` có thể được đưa vào log: chấp nhận.
- **Phát hiện khả năng:** Part B không có danh sách `capabilities`. Backend gọi `codeintel.status` và coi `-32601` là "không hỗ trợ" (Go đã ánh xạ `-32601` thành `domain.ErrAgentMethodNotFound`, `client.go:435-437`). Không thêm trường vào `relay.status` (`relay.ts:715`).

### 2.3 `codeintel.*` trên chế độ stdio/detach của Go (Part A) — việc cần kiểm thử, không cần mã

- Mở rộng kiểm thử stdio (`agent/src/relay/agent-connection-stdio.test.ts` đã có) bằng một ca gửi khung `codeintel.status` qua `StdioWebSocketAdapter` và nhận đúng kết quả; một ca cho `runDetachedStdioMode` nếu có thể (chạy tiến trình thật chậm, đặt `ORCA_RELAY_DETACHED_CHILD=1`).
- **Trạng thái module sống qua kết nối**: trong daemon `--detach`, các Map ở mức module của CR-CV-004 (job reindex, watcher, notifier) sống qua nhiều kết nối socket (mỗi kết nối `createSession` mới; `agent-connection-stdio.ts:272`); `stop()` của một phiên không được huỷ job (CR-CV-004 2.5) và cần `cleanupCodeIntelWatchers()` chỉ dừng thăm dò. Có thể có **nhiều phiên đồng thời** (nhiều bridge `--connect`): "notifier hiện hành" của CR-CV-004 chỉ là phiên gọi `codeintel.*` gần nhất; thông báo không tới các phiên khác. Chấp nhận ở MVP; ghi vào rủi ro.
- Trong chế độ `--stdio` trực tiếp (không `--detach`), `AgentLogger` ghi `console.log` ra **stdout** vốn là kênh khung (`agent-logger.ts:19`); chưa kiểm chứng Go dùng chế độ nào trong thực tế (chú thích `agent-entry.ts:90-93` nói `--detach`/`--connect` là đường `launch()/reattach()`). Mã `codeintel-*.ts` chỉ log qua `AgentLogger` được truyền vào, không `console.*` trực tiếp, và không in gì trước khi kết nối.

### 2.4 Cắm vào `relay.ts`

Đúng một dòng gọi `registerCodeIntelHandlers(dispatcher, authStatusConfig, authStatusLogger)` ngay sau `registerAuthStatusHandlers` (`relay.ts:494`). Chỉ tên, không đổi cấu trúc `main()`. Dùng `import()` động bên trong để lỗi nạp không phá khởi động relay (cùng cách Part A: `try/catch` quanh `import`).

### 2.5 Hai cây `agent/` và `desktop/` (nơi file thật chạy)

Vì binary `relay-ssh` thật build từ `desktop/src/relay/relay.ts` (1.1.3), việc đăng ký phải có ở **cả hai** cây, nếu không tính năng không chạy trên đường Electron. Đề xuất:

1. Sao `codeintel-*.ts`, `gitnexus-*.ts`, `codegraph-*.ts`, `codeintel-symbol-ref*.ts` và `codeintel-relay-handlers.ts` vào `desktop/src/relay/` ở **cùng một PR** (dữ liệu hai cây có thể có kiểu/nhập khẩu khác; kiểm tra các import `../shared/...` có tương ứng trong `desktop/src/shared/` — `git-capability-cache.ts` và `agent-wire-protocol.ts` đã có ở cả hai cây theo `find`).
2. Thêm kiểm tra CI (script mới, đặt tên cụ thể `config/scripts/check-codeintel-relay-parity.mjs`) so khớp nội dung các tệp `codeintel-*`, `gitnexus-*`, `codegraph-*` giữa hai cây (loại trừ khác biệt import đường dẫn đã biết), thất bại khi lệch. Script như vậy chưa tồn tại (audit: "no sync script found"); đây là đề xuất.
3. Không sửa các khác biệt có sẵn giữa hai `dispatcher.ts`/`relay.ts`; mỗi cây chỉ thêm đúng thay đổi của CR này.
4. Nếu quyết định (Q1) không hỗ trợ đường Electron, **không làm** mục 2.2-2.5, chỉ giữ 2.3 và tài liệu hoá giới hạn.

### 2.6 Tài liệu

Cập nhật `specs/agent/api/agent-rpc-catalog-runtime.md` (hoặc catalog tương ứng) với nhóm `codeintel.*`, cột "Part A / Part B", và `specs/agent/api/gaps-and-findings.md` §4 (hợp đồng khác nhau) khi CR được triển khai. Chưa chỉnh các tệp này trong CR (không phải phạm vi soạn thảo).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Lõi trung lập transport, hai bộ chuyển (Part A: `agent-rpc-dispatch-codeintel.ts`; Part B: `codeintel-relay-handlers.ts`) | Tiền lệ `relay-auth-status-handlers.ts`; tránh hai bản logic |
| Chuyển `error.data` ở `dispatcher.ts` | Hợp đồng lỗi (`data.code`) là cốt lõi của README v7 mục 3.3 |
| Không thêm `capabilities` cho Part B | Backend Go đã phân biệt được bằng `-32601`; tránh đổi handshake của relay |
| `relay-ssh` do Go khởi tạo xem là Part A | Đã xác nhận bằng code (1.1.1); không cần mã |
| Tuỳ chọn đường Electron là P2 và có thể bỏ | Hai cây lệch nhau, không có script đồng bộ; chi phí duy trì cao |
| Cùng tên, tham số, kết quả với Part A | UI/backend không phải biết transport |

## 4. Tiêu chí chấp nhận

- [ ] Qua `agent.js --stdio` (adapter stdio trong test) và qua socket của `--detach`, `codeintel.status` trả đúng như Part A; `agent.handshake` có `codeintel*` khi binary tồn tại.
- [ ] Trên `RelayDispatcher` thật (test), mọi method trong `CODEINTEL_METHODS` được đăng ký; method lạ `codeintel.nope` trả `-32601`.
- [ ] Cùng đầu vào, `JSON.stringify` của `result` từ Part A và Part B **bằng nhau** (trừ các trường thời gian đã khai báo: `toolTimingsMs`, `startedAt`).
- [ ] Lỗi có `data.code`, `candidates`, `jobId` đến được bên gọi qua Part B (sau khi sửa `dispatcher.ts`); với `error` không có `data`, khung lỗi không thay đổi so với hiện tại (test hồi quy ở `dispatcher.test.ts`).
- [ ] `rpc.cancel` giữa chừng một truy vấn dài kill tiến trình CLI con và không để lại tiến trình mồ côi; kết quả không gửi (stale).
- [ ] Thông báo `codeintel.indexChanged`/`codeintel.reindexProgress` tới được client Part B qua `dispatcher.notify`.
- [ ] Relay Part B không ghi bất cứ gì ra stdout từ mã `codeintel-*` (test bắt `process.stdout.write`).
- [ ] Hai cây: `check-codeintel-relay-parity` xanh; `desktop/` và `agent/` đều build (`desktop/config/scripts/build-relay.mjs`, `agent/build.mjs`).
- [ ] Tài liệu `specs/agent/api/*` được cập nhật (cột Part A/B) khi triển khai.
- [ ] Không file nào tên `helpers/utils/common/misc`; không `max-lines` disable mới (lưu ý `dispatcher.ts` hiện đã có `eslint-disable max-lines` ở đầu tệp; **không** thêm cái mới, và đặt thay đổi ≤ 5 dòng).

## 5. Kiểm thử

| File test (mới) | Nội dung |
|---|---|
| `agent/src/relay/codeintel-relay-handlers.test.ts` | Dùng `RelayDispatcher` thật với `write` giả (theo mẫu `relay-auth-status-handlers.test.ts`): đăng ký đủ method, `-32601`, lỗi có `data`, `rpc.cancel`, `notify`, `isStale` |
| `agent/src/relay/codeintel-wire-parity.test.ts` | Chạy cùng một tập ca (binary giả phát lại fixture) qua `dispatchCodeIntelRpc` (Part A) và `registerCodeIntelHandlers` (Part B), so JSON kết quả và mã lỗi |
| `agent/src/relay/agent-connection-stdio-codeintel.test.ts` | Gửi `codeintel.status` qua `StdioWebSocketAdapter`; kiểm `capabilities` trong handshake (theo `agent-connection-stdio.test.ts`) |
| `agent/src/relay/dispatcher.test.ts` (sửa) | Hồi quy: lỗi có/không `data` |
| `desktop/src/relay/codeintel-relay-handlers.test.ts` (nếu Q1 = có) | Bản sao cho cây `desktop/` |
| `config/scripts/check-codeintel-relay-parity.mjs` (script, mới) | Thực thi trong CI/`pnpm lint`; có test nhỏ của chính script |

Thủ công: nối thật qua SSH (dev server thử) bằng đường Go (`--detach`), gọi `codeintel.status`; nếu hỗ trợ Part B, nối bằng app Electron và gọi qua multiplexer. Chưa chạy gì ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- **Chưa kiểm chứng backend Go có dùng Part B ở đâu không**; nếu không, mục 2.2-2.5 chỉ phục vụ Electron và có thể là chi phí không cần thiết.
- **Hai cây lệch nhau** và không có script đồng bộ: mọi tệp thêm vào phải sao tay; nguy cơ trôi. Đề xuất script parity chưa có.
- **`error.data` ở Part B** phải sửa `dispatcher.ts` ở hai cây; chưa kiểm chứng toàn bộ người dùng `data` của lỗi từ handler throw (chưa tìm thấy ca nào).
- **Nhiều phiên đồng thời** trong daemon `--detach`: thông báo chỉ tới phiên gọi gần nhất; chưa đo số phiên thực tế.
- **`loadAgentConfig()` trong `relay.ts`**: tiền lệ dùng nhánh stdio; chưa kiểm chứng khi `relay.ts` chạy ở chế độ socket/`--detach` của riêng nó.
- **Node cũ ở relay Part B**: chú thích `external-automations-handler.ts:497` nói relay từ xa vẫn nhắm Node 18; `node:sqlite` không có → chỉ dùng CLI (CR-CV-003 đã có đường lùi). `AbortSignal`, `fetch`... không cần; chưa kiểm chứng các API Node khác mà `codeintel-*.ts` dùng có sẵn ở Node 18 (ví dụ `fs.realpath.native` có; `structuredClone` có từ 17).
- **Ranh giới tin cậy khác nhau:** Part B tin client (renderer) hơn (`context.ts` ghi chú "relay trusts the renderer", allowlist FS đã bỏ); `codeintel.*` giữ cùng quy tắc nghiêm ngặt như Part A, nên không nới ở đây.
- Chưa chạy gì.

## 7. Câu hỏi mở

- **Q1.** App Electron có thật sự cần xem code qua đường Part B không? Nếu không, đóng phần 2.2-2.5 và chỉ giữ 2.3.
- **Q2.** Có chấp nhận sửa `dispatcher.ts` ở hai cây để chuyển `error.data` (2.2)? Nếu không: phương án mã trong `message` (kém).
- **Q3.** Script parity nên nằm ở `config/scripts/` của `agent/` hay `desktop/`? Chưa có tiền lệ.
- **Q4.** Daemon `--detach` có nhiều phiên đồng thời không (nhiều bridge `--connect`)? Nếu có, cần danh sách notifier thay vì "hiện hành".

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (D2, CR-CV-006 trong mục 4)
- `/opt/repos/orca/specs/agent/api/gaps-and-findings.md` (§4 hợp đồng khác nhau; ghi chú `desktop/src/relay`), `/opt/repos/orca/specs/agent/api/compliance-audit-2026-08-15.md` §1, `/opt/repos/orca/specs/agent/api/connection-modes.md` §0, §6
- `/opt/repos/orca/agent/src/relay/dispatcher.ts` (`onRequest` `:154`, `notify` `:202`, `handleRequest` `:456-491`, `rpc.cancel` `:498`), `/opt/repos/orca/agent/src/relay/protocol.ts` (`:8`, `:80-100`), `/opt/repos/orca/agent/src/relay/relay.ts` (`:473-494`, `:715`), `/opt/repos/orca/agent/src/relay/relay-auth-status-handlers.ts`, `/opt/repos/orca/agent/src/relay/relay-handshake.ts` (`:180-192`)
- `/opt/repos/orca/agent/src/relay/agent-entry.ts` (`:80-108`), `/opt/repos/orca/agent/src/relay/agent-connection-stdio.ts` (`runDetachedStdioMode`, `runDetachedChildProcess`), `/opt/repos/orca/agent/src/relay/agent-session.ts`, `/opt/repos/orca/agent/src/relay/agent-logger.ts`, `/opt/repos/orca/agent/src/relay/context.ts`
- `/opt/repos/orca/desktop/config/scripts/build-relay.mjs` (`:19`), `/opt/repos/orca/desktop/src/relay/relay.ts`, `/opt/repos/orca/desktop/src/relay/dispatcher.ts`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/sshconn/connector.go`, `.../adapter/sshrelay/`, `.../adapter/devserveragent/client.go` (`:435-437`)
- `/opt/repos/orca/docs/crs/v7/agent-codeintel/CR-CV-001-codeintel-agent-foundation.md` … `CR-CV-005-codeintel-detect-changes.md`
