# Đánh giá Code vs Thiết kế — AI Provider Credential Relay, Filesystem Watcher & Observability/Telemetry (`agent/`)

**Ngày:** 2026-08-08
**Phạm vi:** `agent/src/relay/{agent-credential-store,ai-complete-handler,ai-provider-handler}.ts`, `agent/src/main/ipc/*` (cluster parcel-watcher + filesystem-watcher + runtime-watcher-process-pool), `agent/src/main/observability/*`, `agent/src/main/telemetry/*`, `agent/src/main/diagnostics/main-thread-churn-probe.ts` — đối chiếu `docs/hld/dev-server-architecture.md` (§6, §9, §15), `docs/features/F35-*.md`, `docs/logic/ai-providers/BL-AIP-01/02/03`, `docs/flows/code/ai-providers/ai-provider-credential.md`.
**Phương pháp:** GitNexus `context`/`impact` (verbatim source + blast-radius/caller-count) trên từng symbol, đối chiếu `file:line`. `agent/` là gói "Orca Dev Server Agent (isolated copy, split from monorepo)" — chính là "Dev Server Agent" mà HLD mô tả (`agent/package.json:4`).

---

## 1. Bảng tổng kết

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| Thư mục lưu credential `~/.orca/ai-providers/*.enc` | ❌ Sai lệch | Code đang **live** (`agent-credential-store.ts`, có caller thật) dùng `~/.orca/credentials/<id>.enc`; đúng path `~/.orca/ai-providers/` chỉ tồn tại trong `ai-provider-handler.ts` — file này **0 caller**, dead code |
| Thuật toán mã hoá at-rest (AES-256-GCM, `scrypt(KEY+accountId, accountId)`) | ⚠️ Một phần | Đúng AES-256-GCM, nhưng salt là `randomBytes(16)` ngẫu nhiên mỗi lần ghi (lưu kèm file), **không phải** `accountId` cố định như tài liệu; agent **double-encrypt cả blob đã mã hoá ở browser** chứ không decrypt-rồi-re-encrypt như flow doc mô tả |
| "Orca Server KHÔNG thấy plaintext key" | ❌ Không khớp cho luồng spawn | Đúng cho luồng **ghi** credential; nhưng luồng **dùng** key khi spawn agent nhận thẳng `resolvedApiKey` (plaintext) từ Orca Server qua params — comment trong code tự thừa nhận Orca Server là bên inject plaintext |
| Fallback đọc credential khi spawn (không có `resolvedApiKey`) | ❌ Lỗi triển khai | Inject thẳng ciphertext (Layer-1, chưa giải mã) vào biến env API key — code tự cảnh báo "agent may fail auth if key not plaintext" |
| Tên RPC method (`ai.credential.write`, `ai.testConnection`, `ai.ping` theo BL-AIP-01/03) | ❌ Sai lệch | Code thật dùng `ai.provider.writeCredential/readCredential/healthCheck/deleteCredential`, `ai.complete` — không method nào khớp tên trong BL-AIP-01/03 |
| `ai.complete` — credential resolution priority (env → AiCredStore) | ⚠️ Một phần | Comment đầu file mô tả đúng 2 tầng, nhưng `resolveApiKey()` **chỉ đọc `process.env`**, không hề gọi credential store |
| Filesystem Watcher — cluster parcel-watcher (`agent/src/main/ipc/*`) đứng sau `fs.watch`/`fs.changed` | ⚠️ Đúng một phần, chỉ cho 1 trong 2 transport | Đúng cho binary `relay.js` (SSH-exec, `relay.ts`→`FsHandler`); **binary `agent.js`** (WS, `agent-entry.ts`) dùng `fs.watch` built-in của Node, hoàn toàn tách biệt khỏi cluster parcel-watcher |
| HLD §15.3 "`fs.changed`... từ `fs.watch` built-in của Node" | ⚠️ Đúng chữ nghĩa nhưng chỉ đúng 1 nhánh | Mô tả này khớp `fs-agent-extensions.ts` (nhánh WS `agent.js`), nhưng cluster parcel-watcher tinh vi hơn nhiều (child-process cô lập crash, quarantine pool, batch cap) lại **không được nhắc tới ở đâu trong HLD** |
| Cluster parcel-watcher có nằm trong build sản phẩm `agent/` không | ⚠️ Không xác nhận được | `agent/build.mjs` chỉ build `out/agent.js` từ `agent-entry.ts`; `relay.ts` (nơi wire cluster parcel-watcher vào) **không có script build nào** trong gói `agent/` hiện tại |
| F40 Full-Flow-Tracing (`agent/src/shared/trace/*`, `createTracer`) | ✅ Có thiết kế | Được document ở `docs/features/F40-full-flow-tracing.md` + `docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md` (khớp trực tiếp với `agent-credential-store.ts`/`ai-complete-handler.ts`) |
| `agent/src/main/observability/{tracer,redactor,instrumentation}.ts` | 📄 Không có thiết kế + dead code | Không tìm thấy tài liệu; code tự trích dẫn "telemetry-error-tracking.md" — file này **không tồn tại** ở bất kỳ đâu trong repo |
| `agent/src/main/telemetry/*` (client/burst-cap/consent/cohort-classifier/validator) | 📄 Không có thiết kế + dead code | `initTelemetry` **0 caller** trong `agent/`; module import `electron`'s `app` — vốn thuộc kiến trúc Desktop, không hợp lý cho một Dev Server headless |
| `agent/src/main/diagnostics/main-thread-churn-probe.ts` | 📄 Không có thiết kế + dead code | `startMainThreadChurnProbe` không được gọi từ đâu trong `agent/src` (kể cả entry point) |

---

## 2. Chi tiết theo mục

### 2.1 AI Provider Credential Relay

**Hai implementation song song, chỉ một cái sống:**

- `agent-credential-store.ts:1-10` — comment đầu file: *"Storage: `~/.orca/credentials/<accountId>.enc`"*. `agent-config.ts:34-35,110` xác nhận `credentialDir: join(home, '.orca', 'credentials')` — **khác hẳn** `~/.orca/ai-providers/` mà F35/BL-AIP-01/flow-doc đều mô tả. File này có caller thật: `agent-spawner.ts` (import `readDecryptedKey`) và `agent-rpc-dispatch.ts:355-406` (case `ai.provider.writeCredential/readCredential/healthCheck/deleteCredential`).
- `ai-provider-handler.ts:17` — `PROVIDER_STORE_DIR = join(homedir(), '.orca', 'ai-providers')` — **đúng path theo tài liệu**, nhưng GitNexus `impact({target:"aiProviderHandlers", direction:"upstream"})` trả `impactedCount: 0` cho cả candidate `agent/` lẫn `desktop/` — **không ai import file này**. Đây là bản triển khai cũ/song song bị bỏ rơi, đồng thời **không hề mã hoá lại ở tầng agent** (chỉ ghi thẳng `{encryptedBlob, iv, updatedAt}` ra JSON, không AES-256-GCM at-rest như comment đầu file tự nhận — `ai-provider-handler.ts:1-9` claim sai với chính thân hàm bên dưới).

**Sai khác thuật toán mã hoá tại chỗ (so với `BL-AIP-01` §"Credential Derivation"):**

- Thiết kế: `masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, salt: accountId)` — salt **xác định** theo `accountId`.
- Code thật (`agent-credential-store.ts:65-69`): `salt = randomBytes(SALT_BYTES)` ngẫu nhiên mỗi lần ghi, lưu kèm trong file (`encryptPayload`, dòng 63-73); khoá derive bằng `scryptSync(masterKey, salt, KEY_LEN)` — **không nối `accountId` vào password** như thiết kế.
- Thiết kế mô tả dev-server **decrypt Layer-1 rồi re-encrypt bằng server key** (`ai-provider-credential.md:113-143`). Code thật (`handleWriteCredential`, dòng 103-146) **không decrypt Layer-1** — chỉ bọc nguyên `{encryptedBlob, iv, algorithm}` (browser blob còn nguyên mã hoá) vào một lớp AES-256-GCM ngoài. Về bảo mật đây thực ra **chặt hơn** thiết kế (agent chưa từng thấy plaintext ngay cả thoáng qua khi ghi) — nhưng khác cơ chế mô tả.

**Kiểm tra "Orca Server KHÔNG thấy plaintext key" — đúng cho ghi, sai cho dùng:**

- Luồng ghi (`handleWriteCredential`) đúng tinh thần thiết kế: Orca Server chỉ relay ciphertext, agent double-encrypt mà không decrypt.
- Luồng spawn/dùng key (`agent-spawner.ts:172-181,225-243`): comment tự thừa nhận *"The Orca Server is responsible for injecting resolvedApiKey (plaintext) via the spawn request params when it has the Layer 1 session key. If resolvedApiKey is provided, it takes priority over credStore lookup"* (dòng 179-181). Khi `resolvedApiKey` có, nó được set thẳng plaintext vào `ANTHROPIC_API_KEY/OPENAI_API_KEY/GEMINI_API_KEY` (dòng 227-230) — nghĩa là **Orca Server phải nắm plaintext (hoặc session key để tự giải mã) ở bước spawn**, mâu thuẫn với khẳng định "Gateway không thấy plaintext" ở HLD §6 dòng 176 và §9 dòng 243.
- Nhánh fallback (không có `resolvedApiKey`, dòng 231-243): đọc `readDecryptedKey()` — hàm này chỉ gỡ lớp mã hoá **ngoài** (Layer 2) và trả về nguyên `encryptedBlob` (Layer 1, **vẫn còn mã hoá bởi SubtleCrypto phía browser**) — rồi set thẳng chuỗi đó vào biến env API key (dòng 240), kèm cảnh báo tự nhận lỗi: *"buildAgentEnv: injecting Layer1 blob for … — agent may fail auth if key not plaintext"* (dòng 242). Đây là một khoảng trống triển khai thật (không chỉ lệch tài liệu) — nhánh fallback về bản chất **không thể hoạt động đúng** vì không nơi nào trong `agent/` giải mã Layer-1.

**Tên RPC method sai lệch có hệ thống:**

- BL-AIP-01 dùng `ai.credential.write`, `ai.testConnection`; BL-AIP-03 dùng `ai.ping`. Code thật (`agent-rpc-dispatch.ts:356,367,378,400,669`) dùng nhất quán `ai.provider.writeCredential`, `ai.provider.readCredential`, `ai.provider.healthCheck`, `ai.provider.deleteCredential`, `ai.complete` — không method nào trùng tên với 2 tài liệu BL trên (HLD §9 dòng 239 lại dùng đúng `ai.provider.writeCredential`, tự mâu thuẫn với chính BL-AIP-01 cùng bộ tài liệu).

**`ai.complete` — credential priority không như mô tả:**

- Header comment `ai-complete-handler.ts:1-14` mô tả 2 tầng resolve: (1) env var từ agent spawn, (2) `ORCA_ACCOUNT_ID` → credential store. Nhưng `resolveApiKey()` (dòng 101-115) **chỉ đọc `process.env['ANTHROPIC_API_KEY'|'OPENAI_API_KEY'|'GOOGLE_API_KEY']`** — không có bất kỳ lookup nào vào `agent-credential-store.ts`. Tầng (2) chưa được implement dù đã document trong chính comment của file.

### 2.2 Filesystem Watcher — Cluster parcel-watcher

**Xác nhận cluster parcel-watcher đứng sau `fs.watch`/`fs.changed` — nhưng chỉ cho MỘT trong hai transport:**

Có **hai implementation `fs.watch` độc lập** trong `agent/`, mỗi cái thuộc một binary/transport riêng:

1. **`relay.ts`** (`relay.ts:12-20`: *"Orca Relay — lightweight daemon deployed to remote hosts... The Electron app deploys this script via SCP and launches it via an SSH exec channel"* — chính là mode `relay-ssh` trong HLD backend) → `FsHandler.registerHandlers()` (`fs-handler.ts:120-122`) đăng ký `dispatcher.onRequest('fs.watch', ...)` → `RelayFilesystemWatchRegistry.watch()` (`relay-filesystem-watch-registry.ts:61-88`) → `RuntimeWatcherProcessPool`/`WatcherProcessSupervisor` (`agent/src/main/ipc/runtime-watcher-process-pool.ts`, `parcel-watcher-process-supervisor.ts`) — dùng **`@parcel/watcher`** qua child-process cô lập crash (`WatcherProcessCrashFuse`), quarantine pool, batch cap `MAX_BATCHED_WATCHER_EVENTS=5000` (`filesystem-watcher-event-batch.ts:3`). Notification phát ra đúng tên `fs.changed` (`relay-filesystem-watch-registry.ts:192,203`), refcount theo client per rootPath (dòng 61-88) — khớp mô tả HLD §15.3 dòng 688 *"refcounted theo từng path"*.
2. **`agent-entry.ts`** (comment dòng 8-10: *"Modes: direct-websocket / relay-websocket"* — chính là Dev Server Agent WS-connecting mà HLD §15 "Execution-Host Unification" mô tả) → `agent-rpc-dispatch.ts` case `fs.watch` (dòng 734-742) → `fs-agent-extensions.ts::handleFsWatch` (dòng 600-636) — dùng **`watch` built-in của `node:fs`** (`import { watch as fsWatchSync } from 'node:fs'`, dòng 7), refcount bằng `Map` đơn giản (`AGENT_WATCH_MAP`, dòng 598), **không dùng `@parcel/watcher`**, không có crash-isolation/quarantine, và `recursive: true` chỉ hoạt động trên macOS/Windows — comment dòng 619-622 tự ghi nhận Linux chỉ watch top-level directory.

→ HLD §15.3 dòng 688 mô tả *"`fs.changed`... từ `fs.watch` built-in của Node"* — mô tả này **khớp chữ nghĩa với nhánh (2)**, nhưng cluster parcel-watcher tinh vi hơn (nhánh 1) — chính là phạm vi được giao audit trong `agent/src/main/ipc/*` — **không được nhắc đến ở bất kỳ đâu trong HLD**. Đây là một gap tài liệu thật: cơ chế phức tạp nhất (cô lập crash bằng subprocess, quarantine, batching) lại vô hình với thiết kế.

**Cluster parcel-watcher có thực sự nằm trong sản phẩm build của `agent/` không:** `agent/build.mjs:19,40,46` chỉ có `entryPoints: [AGENT_ENTRY]` với `AGENT_ENTRY = src/relay/agent-entry.ts`, output `out/agent.js`. Không có script build nào trong gói `agent/` (đã "isolated, split from monorepo") biên dịch `relay.ts` thành `relay.js` để deploy qua SCP/SSH-exec như header của chính `relay.ts` mô tả — `package.json` chỉ có `"build"`/`"build:watch"`/`"start"` (không có target build cho `relay.js` hay `out/cli/index.js` dù `bin.orca` trỏ tới đó). Nghĩa là trong phạm vi gói `agent/` hiện tại, đường dẫn dùng cluster parcel-watcher (nhánh 1) **có code đúng, wiring đúng, nhưng không xác nhận được có build pipeline nào đóng gói nó thành artifact triển khai được** — khác với nhánh (2) `fs.watch` built-in, vốn chắc chắn nằm trong `out/agent.js` vì được import xuyên suốt từ `agent-entry.ts`.

### 2.3 Observability / Telemetry / Diagnostics — Gap tài liệu + dead code

**Không tìm thấy thiết kế nào** cho 3 module này khi grep toàn bộ `docs/` với "telemetry|diagnostics|observability|tracer|redact":
- Chỉ có 2 nhắc chung chung ở mức PRD/URD (`docs/PRD.md:202,490-501`, `docs/URD.md:611`) mô tả **F40 Full-Flow-Tracing** — nhưng F40 tương ứng với module **khác**: `agent/src/shared/trace/` (`createTracer`, `Tracers.*`), có tài liệu đầy đủ (`docs/features/F40-full-flow-tracing.md`, `docs/crs/v2/full-flow-tracing/CR-TRACE-016-ai-providers.md` — khớp trực tiếp với `agent-credential-store.ts:18,113-116` và `ai-complete-handler.ts:20,32-34`).
- Module trong phạm vi audit (`agent/src/main/observability/*`, `agent/src/main/telemetry/*`, `agent/src/main/diagnostics/*`) **hoàn toàn không có tài liệu tương ứng**. Code tự trích dẫn một tài liệu tên **"telemetry-error-tracking.md"** ở nhiều vị trí (`tracer.ts:1-20`, `redactor.ts:1-13`, `instrumentation.ts:1-22`) — grep toàn repo (kể cả `notes/`, `specs/`, `guides/`, lịch sử git) xác nhận **file này không tồn tại**.

**Dead code trong gói `agent/` — xác nhận bằng GitNexus:**
- `initTelemetry` (`agent/src/main/telemetry/client.ts:117`): `impact({direction:"upstream"})` → **0 caller** trong `agent/` (so với 1 caller thật trong `desktop/src/main/telemetry/client.ts`).
- `withGitSpan` (`agent/src/main/observability/instrumentation.ts:175`): chỉ **2 caller** trong `agent/` (so với 172 ở `desktop/`, 156 ở `backend/` và `frontend/` — cùng file, cùng hàm, y hệt tên). 2 caller đó nằm ở `agent/src/main/git/runner.ts` — bản thân file này cũng không được import từ `agent-entry.ts` (entry point thật của `out/agent.js`), chỉ được `agent/src/main/text-generation/commit-message-text-generation.ts` gọi — file này cũng không nằm trong đồ thị import từ `agent-entry.ts`.
- `startMainThreadChurnProbe` (`main-thread-churn-probe.ts:144`): không có caller nào trong `agent/src` ngoài file định nghĩa nó.
- `client.ts` (`agent/src/main/telemetry/client.ts:29`) `import { app } from 'electron'` — import Electron API vào một gói được mô tả là "Node.js CJS bundle... Runtime: Node.js 22+ on any Linux/macOS/Windows dev server" (`agent/build.mjs:9`) và `external: [..., 'electron']` (`agent/build.mjs:46`) — dấu hiệu rõ ràng đây là code Electron-desktop bị copy nguyên xi sang `agent/` trong quá trình "isolated copy, split from monorepo", không phải thiết kế dành riêng cho Dev Server Agent headless.

---

## 3. Nhận định tổng quan

1. **AI Provider Credential Relay lệch tài liệu ở cả 3 lớp** (đường dẫn lưu trữ, chi tiết mã hoá, tên RPC method) — và tài liệu (BL-AIP-01/03) tự mâu thuẫn với HLD §9 về tên method, gợi ý các tài liệu này được viết ở các thời điểm/level khác nhau, chưa từng đối chiếu chéo. Đáng chú ý nhất: **file khớp đúng path/mô tả tài liệu nhất (`ai-provider-handler.ts`) lại chính là code chết** (0 caller) — cho thấy tài liệu được viết theo một implementation cũ đã bị thay thế bởi `agent-credential-store.ts` mà không cập nhật.
2. **Khẳng định bảo mật cốt lõi "Orca Server không thấy plaintext key"** chỉ đúng cho luồng **ghi** credential. Ở luồng **spawn agent** (dùng key), chính comment trong code xác nhận Orca Server phải cung cấp `resolvedApiKey` dạng plaintext — đây là rủi ro/khoảng trống bảo mật thật cần đội backend xác nhận, không chỉ là lỗi tài liệu. Nhánh fallback (khi không có `resolvedApiKey`) còn tệ hơn: tự thừa nhận không hoạt động đúng (inject ciphertext thay vì plaintext).
3. **Filesystem watcher**: HLD mô tả đúng NHƯNG chỉ cho một trong hai transport (`agent.js`/WS, dùng `fs.watch` built-in). Cluster parcel-watcher tinh vi (crash-isolation, quarantine, batch cap) — đúng là cơ chế đứng sau `fs.watch`/`fs.changed` như yêu cầu xác minh, nhưng chỉ cho binary `relay.js` (SSH-exec) — một binary **không có build pipeline xác nhận được** trong gói `agent/` hiện tại, và **không được nhắc tới ở đâu trong HLD**.
4. **Observability/telemetry/diagnostics là khoảng trống tài liệu thật sự, không phải lỗi có thể bỏ qua**: không một dòng thiết kế nào trong `docs/` mô tả 3 module này (khác hẳn F40 Full-Flow-Tracing, vốn có tài liệu đầy đủ và đúng). Nghiêm trọng hơn, bằng chứng GitNexus cho thấy code này **hầu như chết trong `agent/`** — bị copy nguyên xi từ `desktop/` (cùng tên file, cùng comment, kể cả import `electron`) trong quá trình tách gói nhưng chưa từng được wire vào entry point thật (`agent-entry.ts`). Đây vừa là gap tài liệu vừa là rác code cần dọn.

## 4. Khuyến nghị

- **Cập nhật tài liệu AI Provider Credential**: thống nhất một tên RPC namespace (`ai.provider.*` theo code thật) xuyên suốt HLD/F35/BL-AIP-01/03; sửa path lưu trữ thành `~/.orca/credentials/`; viết lại phần "Credential Derivation" theo salt ngẫu nhiên lưu kèm file (không phải salt=accountId).
- **Làm rõ với đội bảo mật/backend**: luồng spawn agent có thực sự cần Orca Server nắm plaintext hay không — nếu có, cập nhật khẳng định "Gateway không thấy plaintext" trong HLD §6/§9 để phản ánh đúng phạm vi (chỉ đúng cho ghi, không đúng cho spawn); nếu không, đây là bug cần fix (hoàn thiện nhánh giải mã Layer-1 phía agent thay vì dựa vào Orca Server).
- **Xoá `ai-provider-handler.ts`** (dead code, đồng thời tự claim sai về mã hoá at-rest) hoặc hợp nhất vào `agent-credential-store.ts`.
- **Bổ sung vào HLD §15.3**: mô tả rõ có 2 implementation `fs.watch` song song theo transport (SSH-relay dùng `@parcel/watcher` cluster; WS-agent dùng `fs.watch` built-in), và xác nhận/loại bỏ `relay.ts` khỏi gói `agent/` nếu binary này không còn được build/deploy.
- **Dọn hoặc viết thiết kế cho observability/telemetry/diagnostics trong `agent/`**: xác nhận với đội phát triển liệu các module này có ý định chạy trên Dev Server hay không; nếu không, xoá khỏi `agent/src/main/` (kể cả tham chiếu tới `electron`); nếu có, viết design doc tương đương "telemetry-error-tracking.md" mà code đang trích dẫn nhưng không tồn tại, và wire chúng vào `agent-entry.ts`.

---

*Phạm vi: AI Provider Credential Relay, Filesystem Watcher & Observability/Telemetry của `agent/` — một trong 5 mảng của audit tổng `agent/`, xem chỉ mục tại `audit/agent/agent-vs-design-review.md`.*
