# Đánh giá Agent RPC Dispatch & Lifecycle vs Thiết kế

**Ngày:** 2026-08-08
**Phạm vi:** RPC Method Surface, Dispatch & Agent Lifecycle trong `agent/src/relay/**` và `agent/src/main/agent-hooks/**`, đối chiếu với bộ tài liệu `docs/crs/v2/dev-server/CR-DS-001..005`, `docs/adrs/v1/ADR-014`, `ADR-015`, `docs/logic/agent-orchestration/BL-AG-01..05`, và `docs/flows/logic|code/agent-orchestration/*`.
**Phương pháp:** đọc trực tiếp source qua CodeGraph/GitNexus, trích dẫn bằng chứng `file:line`, theo đúng format/văn phong của `audit/backend/backend-vs-design-review.md`.

---

## 0. Phát hiện nền tảng — bộ CR-DS-00x/ADR-014/015 là kiến trúc "v6.0 chưa triển khai"

Trước khi so khớp chi tiết, cần nêu rõ: **toàn bộ 5 CR-DS + 2 ADR trong phạm vi audit đều tự khai báo là chưa được implement.**

- Header mọi CR-DS: `**Trạng thái** | Proposed` (`docs/crs/v2/dev-server/CR-DS-001-dev-server-agent-architecture.md:11`, tương tự CR-DS-002:11, CR-DS-003:11, CR-DS-004:11, CR-DS-005:11).
- ADR-014 và ADR-015 còn ghi rõ ràng hơn ở cuối file:
  - `## Trạng thái Implementation` → `❌ Chưa implement (v6.0 proposed)` — `docs/adrs/v1/ADR-014-gateway-agent-json-rpc-protocol.md:231`
  - Tương tự `docs/adrs/v1/ADR-015-signed-execution-context-gateway-agent.md:289`
- Cả hai ADR đều trỏ path code **chưa tồn tại**: `src/agent/rpc/agent-rpc-server.ts`, `src/agent/rpc/context-verifier.ts`, `src/main/dev-server/signed-context-issuer.ts` (ADR-014:232-236, ADR-015:290-292) — không khớp cấu trúc monorepo thật (`agent/src/relay/*`, không có thư mục `src/agent/rpc/`).

→ Do đó phần lớn khác biệt tìm thấy bên dưới **không phải "tài liệu lỗi thời"** như ở backend audit, mà là **tài liệu mô tả một kiến trúc tương lai (v6.0: Dev Server Agent full-backend, JSON-RPC 2.0 thuần, Signed Execution Context) trong khi code hiện tại triển khai kiến trúc v5.x** (binary-framed relay + JSON-RPC payload, không có signed context, không có `step.execute`/`health.*`). Các BL-AG-01..05 (business logic docs) thì ở giữa — mô tả đúng *hướng* kiến trúc v6.0 (Dev Server là WS client, Orca gửi request ngược) nhưng dùng tên method/tham số không khớp code thật ở nhiều chỗ.

---

## 1. Bảng tổng kết

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| ADR-014 — JSON-RPC 2.0 protocol (bỏ binary 13-byte header) | ❌ Chưa triển khai | Code thật **vẫn dùng framing 13-byte header** (`agent-wire.ts`/`relay-protocol.ts`) mà ADR-014 đề xuất loại bỏ; handshake không có nonce challenge-response |
| ADR-015 — Signed Execution Context (HMAC-SHA256, `_ctx` field) | ❌ Không tồn tại | Không có `SignedExecutionContext`, `ContextVerifier`, `RpcExecutionContext`, trường `_ctx` ở bất kỳ đâu trong `agent/src`; `context.ts` chỉ là tiện ích tilde-expand + stub no-op |
| CR-DS-005 — Path/projectRoot enforcement, approvedModels check | ❌ Đã bị gỡ chủ động | Comment trong code xác nhận **FS allowlist đã bị xoá có chủ đích** vì "the relay runs as the SSH user and trusts the renderer process"; không có approvedModels check trước khi spawn agent |
| CR-DS-002 — RPC Method Namespaces (pty/agent/worktree/git/fs/aiProvider/step/health) | ❌ Sai lệch nghiêm trọng | Namespace, tên method, tham số, và cả **các nhóm method (step.\*, health.\*) hoàn toàn không tồn tại**; xem bảng chi tiết §2.2 |
| CR-DS-002 — Error codes (`AgentErrorCodes` 4001-5004) | ❌ Không khớp | Code dùng hệ mã hoàn toàn khác: JSON-RPC chuẩn (-32xxx) + agent-specific (-33001..-33101) |
| CR-DS-001/CR-DS-004 — Kiến trúc/Lifecycle (systemd, Docker, launchd, agent update) | ❌ Không có trong `agent/src` | Không tìm thấy code cài đặt/update/version-negotiation nào trong package `agent/`; các class tên gợi ý (`AgentConnectionManager`, `AgentHookParser`) không tồn tại |
| BL-AG-01 Khởi động Agent | ⚠️ Một phần | Chiều kết nối WS đúng hướng; nhưng tham số `agent.spawn` (taskId/modelId/accountId) khác hẳn design (agentBinary/args/cols/rows); cols/rows luôn hardcode 220×50, không nhận từ caller; response id không mang `ptyId` |
| BL-AG-02 Dừng Agent | ✅ Khớp phần lớn | `agent.sendInput` (Ctrl+C) + `agent.kill` (SIGKILL) đúng cơ chế graceful→force; PTY registry xoá đúng |
| BL-AG-03 Resume Session | ⚠️ Một phần | Claude/Codex resume đúng; **OpenCode và Gemini không hề nhận resume flag** dù bảng thiết kế ghi "✅ Full"/"⚠️ Partial" |
| BL-AG-04 Switch Account | ❌ Không có cơ chế riêng | Không có rate-limit pattern-matching cho AI agent output (`RATE_LIMIT_PATTERNS` claude/codex/opencode) ở đâu trong `agent/src`; chỉ có rate-limit breaker cho `gh` CLI (khác mục đích) |
| BL-AG-05 Monitor Status | ⚠️ Một phần | Không có OSC-133 parser trong `agent/`; cơ chế thật là hook-POST JSON qua `AgentHookServer`/`RelayAgentHookServer`; state model thật có ít nhất **3 state machine khác nhau**, không cái nào khớp bảng 6-state của BL-AG-05 |
| Kiến trúc "tầm nhìn" (RpcServer/ContextVerifier/StepExecutor/EventBus/AgentConnectionManager/AgentHookParser) | ❌ Không tồn tại 1:1 | Xem bảng ánh xạ §2.6 |

---

## 2. Chi tiết theo mục

### 2.1 ADR-014 — Giao thức truyền tải: JSON-RPC thuần vs binary 13-byte header thật

ADR-014 lập luận loại bỏ hoàn toàn binary framing của ADR-005 để chuyển sang "JSON-RPC 2.0 over WebSocket... **Không cần binary framing** vì WebSocket đã có frame boundary" (`docs/adrs/v1/ADR-014-gateway-agent-json-rpc-protocol.md:55-60`).

Code thật (`agent/src/relay/agent-session.ts`) làm ngược lại:
- Import `encodeDataFrame`, `decodeFrame`, `MessageType` từ `./agent-wire` và `../main/ssh/relay-protocol` (`agent-session.ts:20-30`).
- Mọi message — kể cả handshake và JSON-RPC — đều được bọc qua `encodeDataFrame(wireState, ...)` trước khi `ws.send()` (`agent-session.ts:215, 225, 274`).
- `agent/src/shared/agent-wire-protocol.ts:1-13` tự mô tả: *"Frame format (13-byte header, SAME AS relay-protocol.ts): [TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]"* — chính là cấu trúc mà ADR-014 tuyên bố thay thế.

Handshake thật cũng không giống chuỗi `handshake.challenge`(nonce)→`handshake.response`(HMAC signature)→`handshake.ok` mà ADR-014:105-128 mô tả. Thay vào đó agent **tự gửi trước** một JSON-RPC request `id:1, method: AGENT_HANDSHAKE_METHOD ('agent.handshake')` kèm `capabilities`/`agentToken` ngay khi WS mở (`agent-session.ts:196-219`, hằng số tại `agent-wire-protocol.ts:18`) — không có nonce, không có chữ ký HMAC trên handshake.

**Kết luận:** ADR-014 mô tả đúng ý định thiết kế nhưng — như chính nó tự khai báo — chưa implement; code thật vẫn ở mô hình binary-framed cũ.

### 2.2 CR-DS-002 — RPC Method Namespaces: Design vs code thật

Toàn bộ danh sách `case` thật trong `route()` (`agent/src/relay/agent-rpc-dispatch.ts:259-855`):

```
tools/list, tools/call,
git.exec, git.execStream, git.pr.create, git.worktree.list, git.worktree.add, git.worktree.remove,
fs.readDir, fs.readFile, fs.grep, fs.stat, fs.glob, fs.writeFile, fs.mkdir, fs.rmdir, fs.watch, fs.unwatch,
ai.provider.writeCredential, ai.provider.readCredential, ai.provider.healthCheck, ai.provider.deleteCredential,
preflight.check,
github.pr.create, github.pr.merge, github.issue.list, github.issue.create,
gitlab.mr.create, gitlab.pipeline.status,
agent.spawn, agent.kill, agent.sendInput, agent.exec,
ai.complete, shell.eval,
pty.create, pty.attach, pty.write, pty.resize, pty.destroy, pty.scrollback, pty.sendSignal
```
(dòng cụ thể của từng `case`: `agent-rpc-dispatch.ts:259,272,299,310,323,334,345,356,367,378,389,400,411,422,433,444,455,466,477,488,499,510,521,532,543,554,567,580,594,669,698,710,722,734,745,765,778,791,804,817,830,843`)

So với CR-DS-002 (`docs/crs/v2/dev-server/CR-DS-002-gateway-agent-rpc-protocol.md:88-331`):

| Nhóm (doc) | Method trong doc | Method thật trong code | Vấn đề |
|---|---|---|---|
| PTY | `pty.create`, `pty.resize`, `pty.write`, `pty.kill`, `pty.list` | `pty.create`, `pty.resize`, `pty.write`, `pty.destroy`, `pty.attach`, `pty.scrollback`, `pty.sendSignal` | `pty.kill`→ thực tế `pty.destroy`; `pty.list` **không tồn tại**; có 3 method thêm không tài liệu hoá |
| AI Agent | `agent.spawn`, `agent.kill`, `agent.sendPrompt`, `agent.detect` | `agent.spawn`, `agent.kill`, `agent.sendInput`, `agent.exec` | `sendPrompt`→thực tế `sendInput` (khớp BL-AG chứ không khớp CR-DS-002); `agent.detect` **không tồn tại**; `agent.exec` (non-interactive exec) không có trong doc |
| Worktree | `worktree.create`, `worktree.list`, `worktree.remove`, `worktree.fanout` | `git.worktree.list`, `git.worktree.add`, `git.worktree.remove` | Namespace sai hẳn (`worktree.*` → thực tế lồng trong `git.worktree.*`); `create`→`add`; **`worktree.fanout` không tồn tại** (F01 fan-out to N worktrees chưa có RPC) |
| Git | `git.status`,`git.diff`,`git.add`,`git.commit`,`git.push`,`git.log`,`git.generateCommitMessage`,`github.pr.create` | `git.exec`, `git.execStream`, `git.pr.create` | Toàn bộ method Git chi tiết bị thay bằng **1 method passthrough tổng quát** `git.exec` (Orca gửi cả câu lệnh git, agent chỉ exec) — mô hình giao thức khác hẳn: doc là "structured per-operation RPC", code thật là "generic command exec" |
| File System | `fs.readDir`,`fs.readFile`,`fs.writeFile`,`fs.watch`,`fs.search` | `fs.readDir`,`fs.readFile`,`fs.writeFile`,`fs.watch`,`fs.unwatch`,`fs.grep`,`fs.stat`,`fs.glob`,`fs.mkdir`,`fs.rmdir` | `fs.search`→thực tế `fs.grep`; thêm 5 method không tài liệu hoá (`fs.unwatch`,`fs.stat`,`fs.glob`,`fs.mkdir`,`fs.rmdir`) |
| AI Provider | `aiProvider.writeCredential`,`aiProvider.deleteCredential`,`aiProvider.testConnection`,`aiProvider.listConfigured` | `ai.provider.writeCredential`,`ai.provider.readCredential`,`ai.provider.healthCheck`,`ai.provider.deleteCredential` | **Namespace khác hẳn**: camelCase liền `aiProvider.*` (doc) vs dot-separated `ai.provider.*` (code) — đúng loại sai lệch namespace phổ biến nhất đã thấy ở backend audit §9; `testConnection`→`healthCheck`; `listConfigured`→**không tồn tại**; thêm `readCredential` không có trong doc |
| Workflow Step | `step.execute`, `step.cancel` | *(không có)* | **Toàn bộ nhóm không tồn tại** trong `agent/src/relay` |
| Health | `health.get`, `health.diagnose` | *(không có)* | **Toàn bộ nhóm không tồn tại** |
| — | *(không có trong doc)* | `tools/list`,`tools/call` (MCP), `ai.complete`, `shell.eval`, `github.pr.merge`,`github.issue.list`,`github.issue.create`, `gitlab.mr.create`,`gitlab.pipeline.status`, `git.pr.create`, `preflight.check` | 11 method hoàn toàn không có trong CR-DS-002; MCP tool protocol (`tools/list`/`tools/call`) không được nhắc tới dù chiếm vị trí đầu tiên trong router thật |

Grep xác nhận các symbol design không tồn tại trong code: `agent.detect`, `worktree.fanout`, `health.get`, `health.diagnose`, `step.execute`, `step.cancel`, `pty.list`, `agent.sendPrompt`, `worktree.create`, `SignedExecutionContext`, `ContextVerifier`, `RpcExecutionContext` — 0 kết quả trong `agent/src` (đã grep toàn bộ, loại trừ file test).

**Error codes:** CR-DS-002:442-453 định nghĩa `AgentErrorCodes` dùng dải 4001-5004 (`UNAUTHORIZED:4001`, `PROJECT_ROOT_VIOLATION:4010`...). Code thật (`agent/src/shared/agent-wire-protocol.ts:36-51`) dùng dải hoàn toàn khác: JSON-RPC chuẩn (`ParseError:-32700`...`ServerError:-32000`) + agent-specific riêng (`CommandNotFound:-33001`...`AuthFailed:-33101`) — **không một mã nào trùng**.

### 2.3 ADR-015 / CR-DS-005 — Signed Execution Context: khoảng trống bảo mật lớn nhất

Đây là phát hiện nghiêm trọng nhất của mảng này, không chỉ là sai tên.

ADR-015 thiết kế: mỗi RPC call từ Gateway phải mang `SignedExecutionContext` (HMAC-SHA256, TTL 30s) ở field `params._ctx`; Agent phải verify chữ ký, kiểm tra `expiresAt`, và validate `projectRoot` nằm trong `allowedRoots` trước khi cho phép truy cập file (`docs/adrs/v1/ADR-015-signed-execution-context-gateway-agent.md:43-159`), đồng thời `SecureFs.validatePath()` chặn path traversal (`docs/crs/v2/dev-server/CR-DS-005-agent-session-context-propagation.md:159-178`).

Code thật hoàn toàn không có cơ chế này:
- `agent/src/relay/context.ts:20-34` — toàn bộ nội dung file chỉ là `expandTilde()` (mở rộng `~`) và class `RelayContext` với method `registerRoot()` **rỗng có chủ đích** (`// intentionally empty`).
- Comment ngay trong file giải thích lý do có chủ đích: *"the relay runs as the SSH user and trusts the renderer process. A compromised renderer can already weaponize pty.spawn and git.exec to reach any path the SSH user can reach, so the FS-side allowlist provided friction without meaningfully narrowing the blast radius"* (`context.ts:20-24`, trỏ tới `docs/relay-fs-allowlist-removal.md`).
- Không có `_ctx` field nào được đọc trong `route()` (`agent-rpc-dispatch.ts`) — không handler nào parse `params._ctx`, verify HMAC, hay check `expiresAt`.
- `buildAgentEnv()` (`agent/src/relay/agent-spawner.ts:194-257`) — hàm build môi trường trước khi spawn AI agent — **không có bất kỳ check `approvedModels`** nào trước khi inject API key, trái với acceptance criterion của ADR-015 ("approvedModels check trước khi spawn agent" — `docs/adrs/v1/ADR-015-signed-execution-context-gateway-agent.md:307`).

→ Đây không phải "tài liệu lạc hậu" mà là **quyết định kiến trúc đối lập rõ ràng**: team đã chủ động gỡ path-allowlist ở tầng relay (xem `docs/relay-fs-allowlist-removal.md` được trỏ tới trong comment) và dồn toàn bộ trust boundary vào renderer/Orca Server phía trên, khác hẳn mô hình "Agent tự verify mọi request" mà ADR-015/CR-DS-005 đề xuất.

### 2.4 CR-DS-001 / CR-DS-004 — Kiến trúc & Lifecycle Management

- CR-DS-004 mô tả agent như một **service độc lập, tự cài đặt** (systemd/Docker/launchd, `agent.requestUpdate` RPC, `config.yaml` với `allowedRoots`, `security.allowedCommands` whitelist) — `docs/crs/v2/dev-server/CR-DS-004-agent-lifecycle-management.md:17-238`.
- Grep toàn bộ `agent/src` cho `systemd|launchd|agent.requestUpdate|autoUpdate` không cho kết quả liên quan (các file trùng chỉ do khớp chuỗi con "auto" ngẫu nhiên, ví dụ `pty-agent-bridge.ts`) — **không có code cài đặt/update/version-negotiation nào trong package `agent/`**.
- File `agent/src/main/agent-hooks/migration-unsupported-pty-state.ts` — dù tên gợi ý "migration" (có thể hiểu nhầm là DB/version migration liên quan CR-DS-004) — thực chất là một event bus nhỏ theo dõi các PTY không hỗ trợ cơ chế hook-status mới (`entriesByPtyId`, `setMigrationUnsupportedPty()`, `migration-unsupported-pty-state.ts:1-77`), **không liên quan gì đến Agent Lifecycle/Install/Update** của CR-DS-004. Đây là một trùng tên gây hiểu nhầm phạm vi khi audit theo tên file trong task, không phải sai lệch thiết kế.
- `AgentLifecycleState` thật sự tồn tại trong code (`agent/src/relay/agent-spawner.ts:46`: `'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'`) nhưng đây là **state machine của một PTY con của AI agent trong 1 phiên**, không phải "Agent Lifecycle" ở tầm cài đặt/deploy/version mà CR-DS-004 nói tới.
- Reconnect backoff **có tồn tại** (khác điểm với phần còn lại của mảng), nhưng lịch khác: code thật `RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]` (`agent/src/relay/agent-connection-direct.ts:26-27`) vs CR-DS-002's `5s→10s→20s→40s→60s max` (`docs/crs/v2/dev-server/CR-DS-002-gateway-agent-rpc-protocol.md:468-473`).

### 2.5 BL-AG-01 — Khởi động AI Agent

- **Chiều kết nối WS đúng hướng thiết kế**: `AgentConnectionMode = 'direct-websocket' | 'relay-websocket'` (`agent/src/relay/agent-config.ts:8`), agent là WS client tự kết nối ra ngoài, khớp mô tả "Dev Server chủ động mở WebSocket kết nối vào Orca Server" (BL-AG-01 `docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:19-22`).
- **Nhưng port mặc định trong code không phải 6768** như BL-AG-01:22 (`ws://orca:6768/agent`) hay CR-DS-002 ghi — fallback HTTP suy ra từ `deriveHttpUrl()` là `http://localhost:6769` (`agent/src/relay/agent-config.ts:67`), khớp với phát hiện đã có ở backend audit (§6.2: agent WS port thật là 6769, không phải 6768).
- **Tham số `agent.spawn` khác hẳn design.** BL-AG-01:131-138 kỳ vọng `{ agentBinary, args, cwd, env, userId, cols, rows }`. Code thật nhận `{ taskId, userId, modelId/model, accountId, cwd, resumeId, worktreePath, branchName }` (`agent/src/relay/agent-spawner.ts:261-296`) — không có `agentBinary` (được suy ra qua `resolveAgentSpec(modelId)`, `agent-spawner.ts:153-161`), không có `args` (build sẵn theo `AGENT_SPECS`, `agent-spawner.ts:119-142`), và **không nhận `cols`/`rows` từ caller** — PTY luôn được tạo với kích thước hardcode `cols: 220, rows: 50` (`agent-spawner.ts:389`).
- **Response không mang `ptyId` qua đường id-based như thiết kế.** `route()` case `'agent.spawn'` (`agent-rpc-dispatch.ts:554-563`) gọi `void handleAgentSpawn(...)` (fire-and-forget) rồi trả ngay `{ result: { type: 'spawn.accepted' } }` cho request `id` gốc. Bên trong `handleAgentSpawn`, khi PTY spawn thành công, hàm `return { ..., result: { ok: true, ptyId } }` (`agent-spawner.ts:438`) — nhưng giá trị trả về này **bị bỏ qua** vì caller dùng `void`. `ptyId` chỉ xuất hiện gián tiếp trong các notification không có `id` sau đó (`agent.output`, `agent.exited` — `agent-spawner.ts:409-414, 429-434`). Thiết kế BL-AG-01 kỳ vọng `Dev Server trả result: { ptyId, pid }` đồng bộ (BL-AG-01:140) — thực tế không đúng như vậy.
- BR-AG-01 ("chỉ 1 agent chính per worktree per userId") không được agent-side enforce: `ptyId = pty-${userId}-${taskId}-${Date.now()}` (`agent-spawner.ts:354`) không có dedupe/check theo worktree.

### 2.6 BL-AG-02 — Dừng Agent

Khớp khá tốt với thiết kế:
- `agent.sendInput({ ptyId, data: '\x03' })` → `entry.pty.write(data)` — Ctrl+C graceful stop (`agent/src/relay/agent-spawner.ts:498-543`), đúng BL-AG-02:29-31.
- `agent.kill({ ptyId, signal })` → `entry.pty.kill(signal)`, xoá khỏi `PTY_REGISTRY`, tôn trọng cả `SIGTERM`/`SIGKILL` (`agent-spawner.ts:455-492`), khớp BL-AG-02:44-49 (force kill dialog → SIGKILL).
- Điểm khác biệt nhỏ: BL-AG-02 nói UI đợi 10s trước khi hỏi force-kill — logic timeout này nằm ở phía Orca Server (ngoài phạm vi `agent/`), không thấy trong code agent — hợp lý vì đây là quyết định UI-layer, không phải agent-side.

### 2.7 BL-AG-03 — Resume Agent Session

`buildAgentArgs()`/`AGENT_SPECS` (`agent/src/relay/agent-spawner.ts:119-142, 165-167`) so với bảng "Session Resume Support by Agent" (BL-AG-03:81-89):

| Agent | Design nói | Code thật |
|---|---|---|
| Claude | ✅ Full — `--resume <id>` | ✅ khớp: `req?.resumeId ? ['--resume', req.resumeId] : [...]` |
| Codex | ✅ Full — session file | ✅ khớp: `['--session-file', `~/.codex/${req.resumeId}.json`]` |
| OpenCode | ✅ Full — `resume <id>` | ❌ **không hỗ trợ** — `buildArgs: () => []` luôn trả mảng rỗng, bỏ qua `resumeId` hoàn toàn (`agent-spawner.ts:139`) |
| Gemini | ⚠️ Partial | ❌ cũng không nhận resume — `buildArgs: () => ['--stream']` cố định (`agent-spawner.ts:137`), không dùng `req.resumeId` |
| Cursor | ❌ Không hỗ trợ | *(không có trong `AGENT_SPECS` — không phải agent được hỗ trợ)* |

→ Đây là sai lệch **hành vi thực tế**, không chỉ tên gọi: design ghi OpenCode "✅ Full" nhưng code không truyền `resumeId` cho OpenCode dưới bất kỳ hình thức nào.

### 2.8 BL-AG-04 — Switch Account / Provider khi Rate Limited

- BL-AG-04:62-70 định nghĩa `RATE_LIMIT_PATTERNS` (regex cho claude/codex/opencode) áp dụng lên PTY output nhận qua `agent.output`.
- Grep toàn bộ `agent/src` cho `rate.?limit` chỉ tìm thấy `agent/src/main/git/gh-rate-limit-breaker.ts` — cơ chế circuit-breaker cho **`gh` CLI rate limit** (GitHub API), hoàn toàn khác mục đích (đây là điều tiết gọi `gh`/GitHub API, không phải phát hiện AI provider bị rate-limit từ PTY output).
- Không tìm thấy pattern-matching nào trên PTY/agent output để phát hiện rate-limit AI provider trong `agent/src`. Cơ chế "kill PTY cũ → resolve credential mới → spawn PTY mới" (BL-AG-04:35-46) về mặt hạ tầng **có thể** thực hiện được bằng cách gọi tuần tự `agent.kill` rồi `agent.spawn` với `accountId` khác (cả hai RPC đều tồn tại và hoạt động, §2.5-2.6) — nhưng **không có RPC hay logic riêng nào cho "switch account"**; toàn bộ orchestration (phát hiện rate-limit, chọn account mới, priority cascade) phải nằm ở phía Orca Server, ngoài phạm vi package `agent/`.

### 2.9 BL-AG-05 — Monitor Trạng thái Agent Real-time

BL-AG-05 mô tả một pipeline: PTY output → OSC 133 A/B/D parsing + pattern-match text ("waiting for input", "task completed") → `AgentHookParser` → `agent:statusChanged` (BL-AG-05:38-52, 68-88), với bảng 6 state `idle|running|waiting|completed|error|stopped`.

Code thật có kiến trúc khác hẳn, và **3 state machine song song không hợp nhất**:

1. **`AgentLifecycleState`** (PTY-process lifecycle) — 6 state nhưng khác tên: `'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'` (`agent/src/relay/agent-spawner.ts:46`). Không có `waiting`/`completed`.
2. **`AgentStatusState`** (hook-based UI status, dùng bởi `AgentHookServer`) — chỉ 4 state: `['working', 'blocked', 'waiting', 'done']` (`agent/src/shared/agent-status-types.ts:18-19`).
3. Một `AgentStatus` 3-state khác (`'working'|'permission'|'idle'`) đã ghi nhận ở backend audit tại `backend/src/shared/agent-title-core.ts:12`, dùng cho OSC-title parsing phía Orca Server.

**Cơ chế thu thập trạng thái thật không phải OSC-133 parsing trong `agent/`.** Có 2 đường song song:
- **Structured hook JSON**: agent CLI (Claude/Codex/...) tự POST JSON tới một HTTP loopback server nội bộ khi hook lifecycle event xảy ra (`PreToolUse`, `PostToolUse`, `Stop`...). Phía Orca process nội bộ dùng `AgentHookServer` (`agent/src/main/agent-hooks/server.ts:438-1335`, ví dụ `applyNormalizedStatus()` dòng 751-831). Phía relay-qua-SSH dùng bản tương đương `RelayAgentHookServer` (`agent/src/relay/agent-hook-server.ts:112-520`), forward qua notification `agent.hook` (hằng số `AGENT_HOOK_NOTIFICATION_METHOD = 'agent.hook'`, `agent/src/shared/agent-hook-relay.ts:97`) — đây là **notification** (không có `id`), nên hoàn toàn không xuất hiện trong bảng RPC method ở §2.2 (route() switch chỉ xử lý request có `id`).
- **OSC/terminal-title fallback**: có tồn tại (`AgentHookServer.ingestTerminalStatus()`, `server.ts:1039-1091`) nhưng đây là *fallback* thứ yếu, không phải cơ chế chính; việc parse OSC thật (nếu có) nằm ở `backend/src/shared/terminal-title-status.ts` theo backend audit, ngoài phạm vi `agent/`.

→ Kết luận: mô hình "1 status field đơn giản qua OSC 133" của BL-AG-05 không phản ánh đúng độ phức tạp thật (hook-driven, đa nguồn, có cache/dedupe/anti-flicker rất phức tạp — xem các hàm `shouldKeepClaudePermissionVisible`, `isClaudePermissionResumingApprovedTool`, `attachClaudePermissionToolUseId` trong `server.ts:312-436`).

### 2.10 Ánh xạ kiến trúc "tầm nhìn" → code thật

| Tên trong CR-DS-00x/ADR-014/015 | Tồn tại? | Trách nhiệm tương đương nằm ở đâu trong code |
|---|---|---|
| `AgentRpcServer` (ADR-014 code ref) | ❌ | `createRpcDispatcher()` + `route()` trong `agent/src/relay/agent-rpc-dispatch.ts:194-229, 247-857` |
| `ContextVerifier` (ADR-015) | ❌ | Không tồn tại — không có bước verify nào; `RelayContext` (`agent/src/relay/context.ts:30-34`) chỉ còn stub rỗng |
| `SignedContextIssuer` (ADR-015, phía Gateway) | ❌ | Không tìm thấy tương đương phía agent trong scope audit |
| `AgentConnectionManager` (BL-AG docs, phía Gateway) | ❌ (ngoài `agent/`) | Không tồn tại trong `agent/src`; grep toàn repo (loại `backend/src/main/runtime/runtime-rpc.ts`, không phải class này) không tìm thấy symbol trùng tên |
| `AgentHookParser` (BL-AG-05) | ❌ (tên khác) | `AgentHookServer` (`agent/src/main/agent-hooks/server.ts:438`) và `RelayAgentHookServer` (`agent/src/relay/agent-hook-server.ts:112`) |
| `StepExecutor` (CR-DS-001 Phase 4) | ❌ | Không tồn tại; RPC nhóm `step.*` hoàn toàn vắng mặt (§2.2) |
| `EventBus` (CR-DS-001 Phase 1) | ❌ | Không có class `EventBus`; sự kiện được đẩy bằng notification JSON-RPC trực tiếp qua `ws.send`/`dispatcher.notify` (`agent-rpc-dispatch.ts:240-245`, `dispatcher.ts:178-190`) |
| `PtyManager` (CR-DS-001) | ⚠️ tên khác, phân mảnh | `PTY_REGISTRY` (agent.spawn PTYs, `agent-spawner.ts:84-88`) tách biệt với `pty-daemon-client.ts`/`pty-daemon-server.ts` (pty.create terminals) — **2 quần thể PTY khác nhau, không hợp nhất** |
| `AiCredentialStore` (CR-DS-001 Phase 2) | ✅ tên khác | `agent/src/relay/agent-credential-store.ts` (`readDecryptedKey`, dùng bởi `buildAgentEnv`) |
| `GitEngine`/`WorktreeEngine` (CR-DS-001) | ⚠️ tên khác, gộp chung | `agent/src/relay/agent-git-handler.ts` (`handleGitExec`, `handleGitWorktreeList/Add/Remove`) — 1 file duy nhất, không tách 2 engine |

---

## 3. Nhận định tổng quan

1. **Tài liệu CR-DS-00x/ADR-014/015 mô tả một kiến trúc v6.0 "target state" tự nhận là chưa triển khai** — khác về bản chất với backend audit, ở đây phần lớn "sai lệch" là *khoảng cách giữa roadmap và hiện trạng* chứ không phải tài liệu quên cập nhật theo code đã đổi. Điều này cần được nêu rõ khi đọc báo cáo: không nên coi các mục ❌ ở đây là "bug", mà là "chưa làm" hoặc "đã chọn hướng khác".
2. **Điểm đáng lo ngại thật sự** là ADR-015 (Signed Execution Context): code không chỉ "chưa có" mà đã **chủ động gỡ bỏ cơ chế path-allowlist tương đương ở agent-side** (`context.ts` + `docs/relay-fs-allowlist-removal.md`), dồn toàn bộ trust vào tầng trên (renderer/Orca Server qua SSH). Đây là quyết định kiến trúc có chủ đích, không phải sơ suất — nhưng tài liệu ADR-015 hiện tại không phản ánh quyết định này, khiến review bảo mật dựa theo ADR-015 sẽ đọc sai mô hình đe doạ thật.
3. **RPC method surface thật (`agent-rpc-dispatch.ts`) đã đi theo một hướng khác CR-DS-002 khá xa**: dùng generic passthrough (`git.exec`, `agent.exec`) thay vì method chi tiết theo từng operation, có thêm hẳn 1 lớp giao thức MCP (`tools/list`/`tools/call`) không được nhắc tới trong bất kỳ CR-DS/ADR nào, và **hoàn toàn thiếu 2 nhóm method quan trọng** (`step.*`, `health.*`) mà CR-DS-003 (Feature Delegation Matrix) coi là nền tảng cho F36 Workflow và F27 Fleet Health.
4. **3 state machine trạng thái agent không hợp nhất** (`AgentLifecycleState` 6-state PTY-process, `AgentStatusState` 4-state hook-based, `AgentStatus` 3-state OSC-title ở backend) — cùng một vấn đề đã ghi nhận ở backend audit §6.3, xác nhận đây là nợ kiến trúc thật xuyên suốt 2 package, không phải trùng hợp riêng lẻ.
5. **BL-AG-01..05 (business logic docs) khớp thiết kế tốt hơn CR-DS-00x** ở tầm khái niệm (chiều kết nối WS, graceful-stop→force-kill, resume theo `--resume`), nhưng sai ở chi tiết tham số/response shape và bỏ sót các hành vi thật quan trọng (OpenCode/Gemini không resume được dù bảng ghi có hỗ trợ; `cols`/`rows` không truyền được; `ptyId` không về theo đường đồng bộ).

## 4. Khuyến nghị

- **Đánh dấu rõ trạng thái "Proposed/chưa triển khai" ngay trong bảng CR/README** (`docs/crs/v2/dev-server/README.md`) thay vì chỉ ở field ẩn trong từng file, để người đọc không nhầm đây là tài liệu mô tả hệ thống hiện hành.
- **Cập nhật ADR-015 để phản ánh quyết định thật**: hoặc (a) triển khai `ContextVerifier`/`_ctx` như thiết kế nếu đội ngũ muốn khôi phục path-allowlist ở agent-side, hoặc (b) ghi nhận chính thức mô hình trust hiện tại (renderer/SSH-user là trust boundary, agent tin tưởng hoàn toàn) như một ADR mới thay thế ADR-015, tham chiếu `docs/relay-fs-allowlist-removal.md`.
- **Viết lại CR-DS-002 theo method surface thật**: liệt kê đúng 40 method hiện có (namespace `ai.provider.*`, `git.exec`/`git.execStream` passthrough, `tools/list`/`tools/call` MCP layer, `agent.exec` non-interactive), và quyết định rõ `step.*`/`health.*`/`worktree.fanout`/`pty.list`/`agent.detect` có còn nằm trong roadmap hay bỏ khỏi tài liệu.
- **Sửa BL-AG-03 bảng resume-support**: OpenCode và Gemini hiện KHÔNG hỗ trợ resume ở tầng agent — hoặc implement `buildArgs` cho 2 agent này, hoặc hạ xuống "❌ Không hỗ trợ" trong tài liệu để tránh kỳ vọng sai ở UI.
- **BL-AG-01**: bổ sung truyền `cols`/`rows` thật từ client vào `agent.spawn` (hiện hardcode 220×50) và làm rõ trong tài liệu cơ chế lấy `ptyId` thật (qua notification `agent.output`/`agent.exited`, không qua response đồng bộ của request `id` gốc).
- **Hợp nhất 3 state machine trạng thái agent** (`AgentLifecycleState`, `AgentStatusState`, `AgentStatus`) hoặc ít nhất tài liệu hoá rõ ràng phạm vi/quan hệ giữa chúng — đây là điểm trùng với khuyến nghị đã đưa ra ở backend audit, càng củng cố mức ưu tiên.
- **BL-AG-04**: nếu switch-account khi rate-limit vẫn là tính năng mong muốn, cần bổ sung rate-limit pattern-matching cho AI agent output (khác với `gh-rate-limit-breaker.ts` hiện có, vốn chỉ dành cho GitHub CLI) — hiện không có bằng chứng cơ chế này tồn tại ở bất kỳ layer nào đã khảo sát trong `agent/`.

---

*Báo cáo đối chiếu `agent/src/relay/**`, `agent/src/main/agent-hooks/**` với `docs/crs/v2/dev-server/CR-DS-001..005`, `docs/adrs/v1/ADR-014, ADR-015`, `docs/logic/agent-orchestration/BL-AG-01..05`, `docs/flows/logic|code/agent-orchestration/*` — một trong 5 mảng của audit tổng `agent/`, xem chỉ mục tại `audit/agent/agent-vs-design-review.md`.*
