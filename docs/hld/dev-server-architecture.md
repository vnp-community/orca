# Dev Server — Vai trò, Chức năng & Kết nối với Orca Backend

**Nguồn:** Trích xuất từ HLD v1 (C1, C2, C3, C4)
**Cập nhật:** 2026-08-14 v4.0 — **Correction pass**, đối chiếu trực tiếp với code `agent/` (đọc qua CodeGraph/GitNexus, trích dẫn `file:line`). Xem bằng chứng đầy đủ tại `audit/agent/connection-wire-protocol-vs-design-review.md`, `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md`, `audit/agent/pty-ai-cli-vs-design-review.md`, `audit/agent/git-ssh-external-api-vs-design-review.md`, `audit/agent/credential-fswatch-telemetry-vs-design-review.md`, và `audit/backend/backend-vs-design-review.md` (đặc biệt §2.12b). v3.0 trở xuống là kiến trúc mục tiêu/tầm nhìn "v6.0", chưa 1:1 với code thực tế; v4.0 tách rõ phần nào là code THẬT hôm nay và phần nào vẫn là 🚧 đề xuất chưa triển khai. §15 Execution-Host Unification (2026-08) giữ nguyên gần như hoàn toàn — đã được audit xác nhận khớp code.

---

> ⚠️ **Đọc trước khi dùng tài liệu này.** Các phiên bản trước (v1.0–v3.0) trình bày lẫn lộn hai thứ: (a) code thật đang chạy trong `agent/src/relay/*`, và (b) kiến trúc "Dev Server Agent v6.0" mà `docs/adrs/v1/ADR-013..015` và `docs/adrs/v2/ADR-016..020` đề xuất (layer model A0–A4, `ContextVerifier`, `SignedExecutionContext`/`RpcExecutionContext` ký HMAC-SHA256, tách Control/Data Plane nghiêm ngặt). **Toàn bộ vision đó có 0 dấu vết trong code** — grep `ContextVerifier`/`SignedExecutionContext`/field `_ctx` trên toàn bộ `agent/src` và `backend/src` cho kết quả rỗng, và chính các ADR này tự ghi trạng thái `❌ Chưa implement (v6.0 proposed)`. Cơ chế THẬT đơn giản hơn nhiều:
> - Không có RPC context nào được ký/verify — request tới đâu, agent thực thi tới đó, dựa vào việc kết nối WS/SSH đã authenticate.
> - FS-path allowlist từng tồn tại đã bị **chủ động gỡ bỏ** (`agent/src/relay/context.ts` — xem §6.2).
> - Token là self-service qua `POST /api/agent-token`, không có admin UI/DB/revoke (xem §6.1).
>
> Từ v4.0, mọi phần còn mô tả vision "v6.0" được đánh dấu rõ **🚧 Proposed, chưa triển khai** thay vì trình bày như hiện trạng — đúng quy ước status-marker của tài liệu này.

---

## 1. Dev Server là gì?

Trong kiến trúc Orca, **Dev Server** là một **remote machine** (cloud server, EC2, VPS) nơi các công việc thực sự được thực thi: chạy AI agent, thao tác Git, đọc/ghi file, chạy shell commands. Nó đối lập với **Gateway** (Orca Web Server) — tầng điều phối và kiểm soát.

> **Mô hình phân tầng:** Gateway = Control Plane | Dev Server = Data Plane
>
> 🚧 Đây là mô hình mục tiêu (ADR-018), không phải một ranh giới được code enforce. ADR-018 tự ghi trạng thái `🚧 Pattern định nghĩa; cần enforce qua code review` — chưa có lint rule/kiến trúc-test nào chặn Gateway code gọi thẳng thứ thuộc Data Plane. Ví dụ cụ thể vi phạm ranh giới này: Backend hiện tự thực thi `gh`/`glab` ngay trong process của mình thay vì relay ra Dev Server Agent (xem §9, §12 — phát hiện nghiêm trọng nhất của đợt audit này).

---

## 2. Vai trò của Dev Server

| Vai trò | Mô tả |
|---------|-------|
| **Execution Environment** | Nơi AI agents thực sự chạy (Claude, Codex, Gemini...) qua PTY |
| **Code Host** | Chứa git repositories và worktrees thực tế |
| **File System Provider** | Cung cấp file tree, file content, file search cho UI |
| **Git Operations Host** | Thực thi git ops qua RPC `git.exec`/`git.execStream` (generic passthrough — Gateway gửi cả câu lệnh git, agent chỉ thực thi và stream kết quả, xem §5) |
| **AI Credential Store** | Lưu trữ credential AI provider dạng mã hóa (đường dẫn thật: `~/.orca/credentials/<accountId>.enc` — **không phải** `~/.orca/ai-providers/`, xem §11.5) |
| **Workflow Step Executor** | 🚧 **Chưa tồn tại trong `agent/`.** Không có nhóm RPC `step.*` nào trong dispatcher thật (`agent/src/relay/agent-rpc-dispatch.ts`) — xem §13 |
| **Health Reporter** | ⚠️ Trên thực tế, Backend (`FleetHealthMonitor`) chỉ poll trạng thái kết nối SSH mỗi 60s — **không thu thập CPU%/RAM/disk/network latency** như dòng dưới đây từng ngụ ý (field `pingLatencyMs` tồn tại trong type nhưng không bao giờ được ghi giá trị — dead field, xác nhận tại `audit/backend/backend-vs-design-review.md` §5.6/F27) |
| **AI Agent CLI Host** | Spawn và quản lý AI agent CLIs trực tiếp trên máy — nhưng qua **hai roster tách biệt, không hợp nhất** (xem §11.2) |
| **External API Caller** | ⚠️ **Đây là ý định thiết kế, không phải hành vi thật hôm nay.** Agent có implementation đầy đủ để gọi GitHub/GitLab qua `gh`/`glab` CLI với per-user auth isolation — nhưng Backend hiện **không relay tới đây**; Backend tự chạy `gh`/`glab` ngay trong process của chính nó, không per-user isolation. Chi tiết đầy đủ ở §9 và §12 |

---

## 3. Thành phần chạy trên Dev Server

Có **hai thế hệ** component chạy trên Dev Server.

### 3a. Orca Relay (v1 — legacy, deployed via SFTP/SSH-exec)

Binary Node.js được upload lên remote host, phục vụ mode `relay-ssh` (§4 Mode 1):

| Component | Chức năng |
|-----------|-----------|
| `PTY Handler` | Tạo/quản lý PTY sessions, stream I/O qua WebSocket/SSH channel |
| `Filesystem Handler` (`agent/src/relay/fs-handler.ts`) | File read/write/list, watching qua cluster `@parcel/watcher` (crash-isolation, quarantine pool, batch cap 5000 events — xem §15.3), ripgrep search |
| `Git Handler` | git diff, status, commit, push, worktree operations trên remote |
| `Port Scanner` | Scan localhost cho open ports |
| `Agent Hook Server` (`RelayAgentHookServer`) | HTTP server intercept agent hooks, forward qua notification `agent.hook` |

> ⚠️ **Build-pipeline của nhánh này chưa xác nhận được.** `agent/build.mjs` chỉ có một entry point (`agent/src/relay/agent-entry.ts` → `out/agent.js`, dùng cho mode `direct-websocket`/`relay-websocket`, xem 3b). Không có script build nào trong gói `agent/` hiện tại biên dịch `relay.ts` (nơi cluster `@parcel/watcher` được wire vào) thành artifact triển khai được qua SCP/SSH-exec như chính header của `relay.ts` mô tả. Cơ chế watcher tinh vi ở đây có code đúng, wiring đúng, nhưng **hiện diện thật trong gói `agent/` build ra sản phẩm cuối cùng thì chưa xác nhận được**.

### 3b. Dev Server Agent (`agent/src/relay/*`, WS-connecting binary)

`agent.js` binary (Node.js, build từ `agent-entry.ts`), các components thật:

| Component | Chức năng thật | Ghi chú |
|-----------|-----------------|---------|
| `createRpcDispatcher()` / `route()` | JSON-RPC 2.0 dispatch, ~40 method (xem §11–§12) | Payload là JSON-RPC nhưng vẫn bọc trong khung nhị phân 13-byte (§5) — **không phải** "pure JSON-RPC over WS" như ADR-014 đề xuất |
| ~~Context Verifier~~ | 🚧 **Không tồn tại.** Không có bước verify HMAC nào trước khi thực thi RPC | `agent/src/relay/context.ts`'s `registerRoot()` chỉ là stub rỗng có chủ đích — xem §6.2 |
| PTY (2 quần thể tách biệt, không hợp nhất) | `PTY_REGISTRY` (`agent-spawner.ts:84-88`, dùng cho `agent.spawn`/AI agent PTY) tách biệt khỏi `pty-daemon-client.ts`/`pty-daemon-server.ts` (dùng cho `pty.*`/terminal thường) | Hai cơ chế khác nhau — xem §11.1 phân biệt `agent.spawn` vs `pty.spawn` |
| Agent Spawner (`SubAgentSpawner`/`resolveAgentSpec()`/`buildAgentEnv()`) | Spawn 5 model family (claude/codex/gemini/opencode/ollama) qua `node-pty.spawn()` trực tiếp | **Không đọc profile hierarchy** — `OrcaProfile.ts` trong `agent/` là type-only dead code, không có nhánh nào set `PATH` từ `pathAdditions` hay `ANTHROPIC_MODEL` (xem §11.1) |
| `git.worktree.*` (trong `GitHandler`) | git worktree add/remove/list | Không có RPC `worktree.fanout` (fan-out N worktrees) |
| Git Engine — **3 implementation song song, không share code** | `GitHandler` (git-handler.ts, engine chính, `git.exec`/`git.execStream` passthrough), `agent-git-handler.ts` (PR/issue/worktree riêng), `external-api-connector.ts` (gh/glab CLI wrapper riêng) | Trùng lặp thật: cả `git.pr.create` (agent-git-handler.ts) và `github.pr.create` (external-api-connector.ts) đều tồn tại cho cùng chức năng "tạo PR" — xem §12 |
| File System Engine (`fs.*`) | readDir/readFile/writeFile/grep/glob/stat/mkdir/rmdir/watch/unwatch | **Không có `SecureFs` enforcement** — allowlist đã bị gỡ (§6.2) |
| AI Provider Credential Store | `agent-credential-store.ts` — AES-256-GCM, path thật `~/.orca/credentials/<accountId>.enc` | Salt ngẫu nhiên mỗi lần ghi (không derive từ `accountId`); có một sibling dead-code (`ai-provider-handler.ts`, 0 caller) claim sai là có mã hoá AES-256-GCM nhưng không hề mã hoá — xem §11.5 |
| ~~Workflow Step Executor~~ | 🚧 **Không tồn tại.** Không có `step.execute`/`step.cancel` nào trong dispatcher | |
| Health Reporter | Không xác nhận được trong `agent/src` — health polling thật nằm ở Backend (`FleetHealthMonitor`, chỉ đọc SSH connection status, không emit CPU/RAM/disk) | |
| Reconnect (chỉ `direct-websocket`) | `RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]` (`agent-connection-direct.ts:26-27`) — cap 30s, không phải "5s→60s" | `relay-websocket` mode có reconnect exponential backoff đầy đủ phía Backend (2s→60s, jitter); `direct-websocket` mode **không có Orca-side reconnect** — dựa hoàn toàn vào agent tự reconnect |
| Notification push (không phải "Event Bus" class riêng) | Không có class `EventBus`. Sự kiện đẩy trực tiếp qua `ws.send`/`dispatcher.notify()`. Notification 1-chiều (`pty.data`/`pty.exit`/`fs.changed`, không có `id`) là năng lực mới thêm 2026-08 — xem §15.3 | |
| ~~Local SQLite~~ | 🚧 **Chưa xác nhận có trong code thật.** Đây là đề xuất của ADR-019 (`agent_pty_sessions`/`agent_worktrees`/`agent_task_runs`), tự ghi `❌ Chưa implement` | |

### Agent Startup & Handshake (thật)

Không có bước "verify HMAC context" hay "capabilities negotiation" phức tạp như tài liệu cũ ngụ ý. Trình tự thật (mode `direct-websocket`, mặc định):

```
agent-entry.ts start
  │
  ├─ 1. Load config (agent-config.ts) — đọc ORCA_URL, ORCA_AGENT_TOKEN, ...
  ├─ 2. Mở WS outbound tới ORCA_URL (mặc định derive port httpPort, xem §4 Mode 3)
  ├─ 3. Ngay khi WS open: agent tự gửi JSON-RPC request
  │       { id: 1, method: 'agent.handshake', params: { agentToken, capabilities, ... } }
  │     (KHÔNG có nonce/challenge-response HMAC nào — ADR-014 mô tả một handshake
  │      có ký khác hẳn, chưa triển khai)
  ├─ 4. Gateway trả JSON-RPC response thường { result: { ok: true } } — không có
  │      message type riêng "handshake-ok"
  ├─ 5. Nếu chưa có agentToken hợp lệ: agent tự POST /api/agent-token để lấy token
  │      (xem §6.1) — không đợi admin cấp qua UI
  └─ 6. Sẵn sàng nhận RPC calls; `AgentTokenManager` chủ động renew token ở 80% TTL
       (mặc định TTL 24h), pre-fetch token kế tiếp để dùng ngay khi reconnect
```

---

## 4. Cách kết nối: Orca Backend ↔ Dev Server

Có **3 connection modes**, tùy cách agent/relay kết nối:

### Mode 1: `relay-ssh` (SSH exec channel)

```
Orca Gateway
    │
    ├── SSH connection (ssh2 library — nằm ở backend/src/main/ssh/, KHÔNG phải agent/)
    │     └── SSH exec channel
    │              │
    │         SshChannelMultiplexer
    │              │
    │         JSON-RPC 2.0 frames (13-byte header)
    │              ↓
    │         Dev Server (Relay binary)
    │              ├── PTY Handler
    │              ├── Git Handler
    │              └── FS Handler
```

- **Auth:** SSH key authentication
- **Transport:** SSH exec channel → binary wire protocol
- **Use case:** Classical SSH remote host
- **Lưu ý phạm vi package:** SSH connection setup (auth, `~/.ssh/config` parsing, reconnect loop) sống hoàn toàn ở `backend/src/main/ssh/` và `desktop/src/main/ssh/`. `agent/src/main/ssh/` chỉ có 6 file phía **relay/remote-platform** (`relay-protocol.ts`, `ssh-channel-multiplexer.ts`, `ssh-remote-platform.ts`, stream readers...) — không có code thiết lập kết nối SSH nào trong `agent/`.

### Mode 2: `relay-websocket` (Orca → Agent, outbound từ Gateway)

```
Orca Gateway
    │
    ├── HTTP Upgrade: ws://agent:<agentPort>/orca-relay   (mặc định agentPort=6799)
    │     Header: Authorization: Bearer <agentToken>  (hoặc ?token=... query)
    │              │
    │         WsTransport
    │              │
    │         SshChannelMultiplexer ⇔ JSON-RPC 2.0
    │              ↓
    │         Dev Server Agent (`agent-connection-relay.ts`)
```

- **Auth:** Bearer token, bắt buộc — nếu `ORCA_AGENT_TOKEN` rỗng agent tự `process.exit(1)`, không có fallback insecure default
- **Transport:** WebSocket, khung nhị phân 13-byte header (§5)
- **Use case:** Agent mở public WS endpoint, Gateway chủ động kết nối vào
- Path `/orca-relay` chỉ là quy ước gợi ý trong thông báo lỗi, không phải route cố định — URL thật do `config.wsUrl` của từng Dev Server quyết định

### Mode 3: `direct-websocket` (Agent → Orca, inbound — default)

```
Dev Server Agent (agent-connection-direct.ts)
    │
    ├── Outbound connect: wss://orca-gateway:6769/agent      ⚠️ CỔNG THẬT LÀ 6769, KHÔNG PHẢI 6768
    │              │
    │         Handshake (không HMAC, không nonce):
    │           Agent → { id: 1, method: 'agent.handshake', params: { agentToken, capabilities, ... } }
    │           Orca  → { result: { ok: true } }   (JSON-RPC response thường, không phải message type riêng)
    │              │
    │         AgentWebSocketServer ⇔ WsTransport ⇔ JSON-RPC
    │              ↓
    │         Gateway routes calls → Agent dispatcher
```

- **Auth:** agentToken tự request qua `POST /api/agent-token` (xem §6.1) — agent **chủ động** connect ra ngoài, không cần mở inbound port
- **Transport:** WebSocket, khung nhị phân 13-byte header

> ⚠️ **Port mismatch — phát hiện quan trọng nhất của mục này.** Gần như mọi tài liệu (kể cả các phiên bản trước của file này) ghi `wss://orca:6768/agent`. Code thật: `AgentWebSocketServer.attach(httpServer)` gắn vào **`httpPort`** (mặc định **6769**), không phải `rpcPort` (mặc định 6768) — bằng chứng `backend/src/server/index.ts:106,108` (log thật in ra `ws://0.0.0.0:${httpPort}/agent`).
>
> **Code còn tự mâu thuẫn với chính nó:** comment tại `backend/src/main/dev-server/agent-ws-server.ts:5-6` ghi đúng ("Browser → :6768/; Agent → :6769/agent"), nhưng thông báo lỗi runtime khi slot hết hạn tại **cùng file đó, dòng 103** lại in `ORCA_URL=ws://<orca-host>:6768${AGENT_WS_PATH}` — hướng dẫn người vận hành cấu hình sai cổng ngay trong chính error message của code. Đây là bug thật trong code, không chỉ là tài liệu lạc hậu.
>
> Bằng chứng gián tiếp xác nhận 6769 là cổng thật: hạ tầng production (`agent/src/relay/agent-config.ts:86`, `ORCA_URL` mặc định trỏ domain thật) dựa vào TLS 443 + reverse-proxy Nginx `→ :6769/agent`.

> **2026-08:** Chính connection này (không cần thêm connection nào khác) giờ là nền tảng cho **Execution-Host Unification** — xem §15.

---

## 5. Wire Protocol Format

Tất cả communication (cả 3 mode) đều dùng cùng khung nhị phân — **và vẫn còn dùng**, dù ADR-014 (🚧 chưa triển khai) từng đề xuất bỏ nó để chuyển sang JSON-RPC thuần over WS:

```
┌──────────────────────────────────────────────────────┐
│ TYPE[1B] | SEQ[4B BE] | ACK[4B BE] | LEN[4B BE] | PAYLOAD[LEN] │
└──────────────────────────────────────────────────────┘
         = 13 bytes header total
PAYLOAD  = UTF-8 JSON-RPC 2.0
TYPE     = 0x01 Regular | 0x09 KeepAlive
MAX PAYLOAD = 16 MiB (`MAX_MESSAGE_SIZE`, agent/src/main/ssh/relay-protocol.ts:15)
```

Bằng chứng: `agent/src/main/ssh/relay-protocol.ts:14,18-21`, `agent/src/shared/agent-wire-protocol.ts:1-13`. Đây là điểm khớp tốt giữa thiết kế Phase-2 (CR-AG-001) và code — chỉ số liệu keepalive/close-code là sai (bên dưới).

### 5.1 Keepalive & Close Codes (giá trị thật)

| | Tài liệu cũ hay ghi | Code thật |
|---|---|---|
| Keepalive interval | 30s | **5s** (`AGENT_KEEPALIVE_INTERVAL_MS = 5_000`) |
| Timeout | 90s (3 lần miss) | **20s**, ngắt ngay khi quá hạn, không có logic "3 lần miss" (`AGENT_TIMEOUT_MS = 20_000`) |
| Close code (auth fail) | Custom `4001` | WS chuẩn **`1008`** (Policy Violation) |
| Close code (server error) | Custom `4002`/`4003` | WS chuẩn **`1011`** (Server Error) |

Bằng chứng: `agent/src/shared/agent-wire-protocol.ts:21-22`, `agent/src/main/ssh/relay-protocol.ts:24-25`; close codes tại `agent/src/relay/agent-connection-relay.ts:123`, `agent/src/relay/agent-session.ts:246,309`, `backend/src/main/dev-server/agent-ws-server.ts:156`, `backend/src/main/dev-server/ws-handshake.ts:200`. Lỗi auth **có** kèm JSON-RPC error frame `code: -33101 AuthFailed` trước khi đóng WS — phần này đúng thiết kế, chỉ WS close code là sai. `AGENT_MIN_VERSION` là hằng số khai báo nhưng không hề được đọc/so sánh ở đâu — cơ chế "version mismatch" chỉ tồn tại trên giấy.

---

## 6. Security Model

| Điểm bảo mật | Cơ chế thật |
|--------------|--------|
| **Auth (kết nối)** | Bearer token, hash SHA-256 trước khi tra cứu — nhưng lưu trong `Map` in-memory, không phải DB. Chi tiết vòng đời token: §6.1 |
| **Context integrity (per-RPC-call)** | 🚧 **Không tồn tại.** Không có `RpcExecutionContext`, không HMAC-sign, không TTL trên từng call. Đây là đề xuất của ADR-015, tự ghi `❌ Chưa implement` |
| **Credential relay (ghi)** | Browser encrypt (SubtleCrypto AES-GCM) → Gateway relay ciphertext nguyên vẹn (KHÔNG decrypt) → Dev Server bọc thêm 1 lớp AES-256-GCM riêng của agent rồi mới ghi file — **chặt hơn** thiết kế cũ mô tả (Gateway thực sự chưa từng thấy plaintext ở bước ghi) |
| **Credential relay (dùng khi spawn agent)** | ⚠️ **Khác hẳn khẳng định "Gateway không thấy plaintext".** Comment trong code (`agent-spawner.ts:179-181`) tự thừa nhận: *"The Orca Server is responsible for injecting resolvedApiKey (plaintext) via the spawn request params when it has the Layer 1 session key."* Khi có `resolvedApiKey`, nó được set thẳng plaintext vào biến env API key. Nhánh fallback (không có `resolvedApiKey`) còn tệ hơn — nó set **ciphertext Layer-1 chưa giải mã** vào biến env, và code tự cảnh báo *"agent may fail auth if key not plaintext"* — nhánh này về bản chất không hoạt động đúng |
| **File path safety** | 🚧 **Đã bị chủ động gỡ bỏ**, không phải "chưa làm". Xem §6.2 |
| **User isolation (GH/GLAB)** | `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per userId — **có thật nhưng chỉ trong `agent/`** (`external-api-connector.ts`). Backend's own `gh`/`glab` calls (chạy cục bộ trên Backend host, xem §12) **không hề set** 2 biến này — 0 kết quả grep trong `backend/src/main` — nghĩa là trong Web Server multi-user mode, mọi user hiện đang share chung một gh/glab auth context trên host Backend |
| **PTY ownership** | `ptyId` bind với `userId`, đúng thiết kế |
| **Shell safety** | `execFile()`/`spawn(..., {shell:false})` (không dùng shell) — đúng ở `agent-git-handler.ts`, `external-api-connector.ts` |
| **Git author identity** | 🚧 **Không tồn tại như thiết kế hứa.** Xem §7 |
| **Audit** | Không xác nhận/phủ nhận được trong phạm vi audit `agent/` lần này — cần verify riêng trước khi dựa vào claim "mọi RPC call được log" |

### 6.1 Token Lifecycle (thật) — hoàn toàn khác mô hình admin-UI + DB

Các tài liệu cũ mô tả một hệ thống: Admin bấm "Generate Token" trên UI → `crypto.randomBytes(32)` 64-hex → lưu bảng DB `orca_agent_tokens` → revoke qua `DELETE /admin/api/agent-tokens/:id`.

**Không có gì trong số đó tồn tại.** Cơ chế thật:

- Agent **tự** request token qua `POST /api/agent-token` (`backend/src/server/agent-token-routes.ts`), auth bằng `Authorization: Bearer <ORCA_AGENT_API_SECRET>` — một shared secret cấp cho toàn bộ fleet, **không phải** admin session.
- Token có định dạng **đoán được**: `agt-<devServerId>-<timestamp>` (`generateAgentToken()`, `agent/src/shared/agent-wire-protocol.ts:89-91`) — không phải `crypto.randomBytes(32)`.
- **Không có bảng DB nào.** Chỉ có `Map` in-memory (`pendingMeta`/`pendingSlots`, `backend/src/main/dev-server/agent-ws-server.ts:46`).
- **Không có endpoint revoke.**
- Token có hash SHA-256 trước khi dùng làm key tra cứu — điểm cải thiện bảo mật thật so với lưu plaintext, nhưng không thay đổi kết luận: mô hình tổng thể (nguồn phát, nơi lưu, cách thu hồi) khác hoàn toàn "admin UI + DB".
- **Renewal chủ động** (không có tài liệu cũ nào mô tả cơ chế này): `AgentTokenManager` (`agent/src/relay/agent-token-manager.ts`) tự renew ở 80% TTL (`TOKEN_RENEW_RATIO = 0.8`), TTL mặc định 24h (`AGENT_TOKEN_DEFAULT_TTL_SEC = 86_400`), pre-fetch token kế tiếp để dùng ngay khi reconnect.

### 6.2 FS Path Allowlist — đã bị chủ động gỡ bỏ

`agent/src/relay/context.ts` chỉ còn `expandTilde()` và class `RelayContext` với method `registerRoot()` **rỗng có chủ đích** (`// intentionally empty`). Comment ngay trong file giải thích lý do:

> *"the relay runs as the SSH user and trusts the renderer process. A compromised renderer can already weaponize pty.spawn and git.exec to reach any path the SSH user can reach, so the FS-side allowlist provided friction without meaningfully narrowing the blast radius."*

(trỏ tới `docs/relay-fs-allowlist-removal.md`). Đây là **quyết định kiến trúc có chủ đích** — không phải khoảng trống chưa làm — dồn toàn bộ trust boundary lên renderer/Orca Server phía trên, khác hẳn mô hình "Agent tự verify mọi request qua `SecureFs.validatePath()`" mà tài liệu cũ mô tả.

---

## 7. Agent Isolation Model

Dev Server Agent không `fork()` per user như Gateway. Isolation được enforce qua:

| Mechanism | How (thật) |
|-----------|-----|
| PTY ownership | `ptyId` bound to `userId`, router checks ownership — đúng thiết kế |
| File path | 🚧 Không có `SecureFs.validatePath()`. Xem §6.2 |
| **Git author** | ❌ **Không tồn tại như thiết kế hứa** ("Injected từ `ctx.userEmail`, không thể bị override"). Grep `git-handler.ts`/`agent-git-handler.ts` cho `userEmail`/`GIT_AUTHOR`/`GIT_COMMITTER` → 0 kết quả. Cơ chế thật duy nhất: `preflight.setGitIdentity` chạy `git config --global user.name/user.email` **một lần, toàn cục** — không gắn với request context, và bất kỳ `git.exec` nào sau đó cũng có thể tự đổi lại `git config` — không có gì ngăn override |
| AI env | `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per userId — đúng, nhưng chỉ áp dụng trong `agent/` (xem cảnh báo ở §6 về Backend không set 2 biến này) |
| Shell commands | `execFile()`/`spawn(shell:false)` — đúng. ⚠️ "`disallowedCommands` whitelist" **không được enforce trong `agent/`** — field này chỉ tồn tại trong `OrcaProfile.security` (type-only, dead code, không có runtime logic nào đọc nó trong `agent/`) |
| Audit | Không xác nhận được trong phạm vi audit lần này |

---

## 8. Sơ đồ tổng quan Control Plane ↔ Data Plane (thật)

```
═══════════════ GATEWAY (Control Plane) ═══════════════

 ProfileResolver  → resolved profile (Company ← Dept ← User)     [BACKEND-tier, không có trong agent/]
 ProjectService   → project.devServerId
 AIProviderSvc    → provider metadata (no credentials)
 WorkflowOrch     → DAG dispatch to dev servers
 TaskService      → task → agent spawn via relay
         │
         │  AgentWebSocketServer / DevServerRelayBridge
         │  (Không ký RpcExecutionContext nào — không có bước "sign" ở đây)
         │
         └──── [wss:// outbound :6769/agent, hoặc SSH channel] ────┐
                                                                    ↓
═══════════════ DEV SERVER AGENT (Data Plane) ═══════════════

 createRpcDispatcher() / route()   (KHÔNG có ContextVerifier — mọi request được thực thi
     │                              trực tiếp, không verify chữ ký nào)
     ├── PTY_REGISTRY + SubAgentSpawner        ← spawn 5 model family (agent.spawn)
     ├── pty-daemon-client/server               ← terminal thường (pty.*), quần thể PTY riêng
     ├── GitHandler + agent-git-handler.ts       ← 2 trong 3 implementation git/gh song song
     │       + external-api-connector.ts          (xem §12, không share code)
     ├── fs.* handlers (KHÔNG có SecureFs enforcement)
     ├── agent-credential-store.ts (AES-256-GCM, ~/.orca/credentials/)
     ├── (KHÔNG có StepExecutor — step.* RPC group không tồn tại)
     └── ws.send()/dispatcher.notify() → notification trực tiếp tới Gateway
             (bao gồm pty.data/pty.exit/fs.changed — xem §15.3)
```

---

## 9. Key Flows liên quan Dev Server

### Flow: AI Credential Write (Relay-Only, không qua Gateway plaintext)

```
Admin nhập API key
    → Browser: SubtleCrypto.encrypt(sessionKey, apiKey)     [Layer 1]
    → POST /rpc (encrypted blob)
    → Orca Server: relay.call('ai.provider.writeCredential')     (đúng tên method thật)
    → Dev Server Agent (KHÔNG decrypt Layer 1 — chỉ bọc thêm)
    → Encrypt lại bằng khoá riêng của agent (AES-256-GCM, salt ngẫu nhiên)   [Layer 2]
    → Write ~/.orca/credentials/<accountId>.enc          ⚠️ KHÔNG PHẢI ~/.orca/ai-providers/
    [Orca Server KHÔNG thấy plaintext key — đúng cho luồng GHI]
```

> ⚠️ Luồng **spawn/dùng** key lại khác — xem §6 bảng "Credential relay (dùng khi spawn agent)": Orca Server phải cung cấp `resolvedApiKey` plaintext khi có thể, nên khẳng định "Gateway không bao giờ thấy plaintext" chỉ đúng một nửa.

### Flow: Project Workspace Switch

```
User chọn Project
    → WorkspaceContext.switchProject(projectId)
    → ProjectService.get() → { devServerId, repoPath }
    → RelayConnectionPool.getOrConnect(devServerId) [reuse nếu có]
    → Promise.all [git.exec('status'), git.worktree.list, fs.readDir, workflow.getActive]
    → WorkspaceContext ready
    → ExplorerPanel, GitPanel, AgentPanel render
```

### Flow: Task → Agent → Git → PR (End-to-end)

```
Lead tạo Task → AI decompose → subtasks
Developer mở Task → [Run Agent]
    → relay: agent.spawn({ taskId, userId, modelId, accountId, cwd, worktreePath, ... })
        (KHÔNG nhận cols/rows từ caller — hardcode 220×50; ptyId KHÔNG về đồng bộ trong
         response — chỉ xuất hiện sau, qua notification agent.output/agent.exited)
    → Agent completes → notification 'agent.exited'
        → GitPanel auto-refresh (git status)
        → ExplorerPanel refresh decorations
    → Developer: [Stage All] → [AI: Generate commit message]
        → relay: git.exec(['diff', '--staged']) → LLM → message
    → [Commit & Push]
        → relay: git.exec(['push']) (stream progress qua git.execStream)
    → [Create PR]
        → ❌ KHÔNG relay ra Dev Server Agent như tài liệu cũ mô tả.
          Backend tự chạy `gh pr create` NGAY TRONG PROCESS của chính Backend
          (`backend/src/main/github/*` → `ghExecFileAsync` → `child_process.execFile`),
          không per-user isolation, không set GH_CONFIG_DIR.
          Phía Agent CÓ implementation đúng thiết kế cho việc này
          (`agent-git-handler.ts` case 'git.pr.create', `external-api-connector.ts`
          case 'github.pr.create') — nhưng Backend không hề gọi tới các RPC method này
          (đã grep toàn bộ backend/src: 0 kết quả cho relay.call/mux.request tới
          git.pr.*/github.*/gitlab.* nghiệp vụ PR/issue/MR). Xem §12 để biết chi tiết đầy đủ.
    → PR URL → Task.prUrl = PR.url
    → Task status → 'review'
```

### Flow: Remote File Explorer

```
User expands 📁 src/ trong Explorer
    │
    ▼ relay.call('fs.readDir', { path: '/srv/projects/vnp-blc/src', depth: 1 })
    │
    ├── relay → dev server: fs.readdir('/srv/projects/vnp-blc/src')
    ├── returns: [{ name: 'auth', isDir: true }, { name: 'index.ts', size: 2048 }]
    │
    ▼ overlay git status decorations (pre-fetched in WorkspaceContext.gitStatus)
    │
    ▼ Render file tree với inline git status badges [M] [A] [?]
```

---

## 10. Communication Matrix (Gateway ↔ Dev Server & Externals)

| From | To | Protocol | Format |
|------|----|----------|--------|
| Workflow Engine | Dev Server | relay RPC | JSON-RPC 2.0 (trong khung 13-byte) |
| Task Service | Dev Server | relay RPC | JSON-RPC 2.0 |
| Project Workspace | Dev Server | relay RPC | JSON-RPC 2.0 |
| Dev Server Agent | Gateway | WebSocket outbound, **:6769**/agent (⚠️ không phải :6768, xem §4) | Binary frames + JSON-RPC |
| Fleet Health Monitor | Dev Server | SSH poll | Chỉ đọc SSH connection status — không CPU/RAM/disk/latency |
| **Dev Server Agent** | **AI Agent CLIs** | **node-pty (agent.spawn) — chỉ 5 model family** | **stdin/stdout, chủ yếu hook-JSON qua HTTP loopback, không phải OSC-133 parsing như tài liệu cũ khẳng định** |
| ⚠️ **Backend** | **GitHub API** | **HTTPS (`gh` CLI, chạy TRỰC TIẾP trên host Backend)** | **Không per-user isolation — xem §9, §12** |
| ⚠️ **Backend** | **GitLab API** | **HTTPS (`glab` CLI, chạy TRỰC TIẾP trên host Backend)** | **Không per-user isolation** |
| Dev Server Agent | GitHub/GitLab API | HTTPS (`gh`/`glab` CLI, per-user `GH_CONFIG_DIR`) | Implementation có sẵn, đúng thiết kế, nhưng **hiện không được Backend gọi tới cho nghiệp vụ PR/issue/MR** (dead từ góc nhìn Backend — xem §12) |
| **Dev Server Agent** | **Gateway** | **WebSocket outbound, unsolicited push** | **JSON-RPC 2.0 notification, không có `id` — `pty.data`/`pty.exit`/`fs.changed` (2026-08, xem §15.3)** |

---

## 11. AI Agent CLIs trên Dev Server

> ⚠️ **Đổi khung quan trọng:** `ProfileAwareAgentSpawner` mà tài liệu cũ mô tả **không tồn tại trong `agent/`** — đây là component thuộc Backend-tier (`backend/src/main/project/ProfileAwareAgentSpawner.ts`), và ngay cả ở đó, `spawn()` luôn đi qua relay (`relay.call('agent.exec', ...)`) chứ không tự spawn `node-pty` cục bộ. Đối trọng thật trong `agent/` là `SubAgentSpawner`/`resolveAgentSpec()`/`buildAgentEnv()` (`agent/src/relay/agent-spawner.ts`), tự nhận trong comment đầu file là "Dev Server tier" — code tự phân biệt ranh giới tốt hơn tài liệu.

### 11.1 Agent Spawn — Kiến trúc thật (`agent.spawn` RPC)

```
Gateway gửi RPC: agent.spawn({ taskId, userId, modelId/model, accountId, cwd,
                                resumeId, worktreePath, branchName })
    │  ⚠️ KHÔNG có agentBinary/args/cols/rows trong params như tài liệu cũ mô tả
    ▼ Dev Server: SubAgentSpawner
    │
    ├── [1] KHÔNG có validate model whitelist nào trước khi spawn
    │       🚧 "approvedModels check" là đề xuất của ADR-015, chưa implement.
    │       resolveAgentSpec(modelId) chỉ map model → 1 trong 5 spec cứng
    │       (claude/codex/gemini/opencode/ollama); model lạ → lỗi "Unknown model"
    │       thẳng (không có fallback "→ claude" như tài liệu BL-PRF-04 từng mô tả)
    │
    ├── [2] LOAD AI credentials — 2 nguồn, ưu tiên khác thiết kế cũ
    │       (a) Backend inject thẳng resolvedApiKey (PLAINTEXT) qua spawn params
    │           nếu đã có Layer-1 session key — đường đi CHÍNH trong thực tế
    │       (b) Fallback: agent-credential-store.readDecryptedKey() — chỉ gỡ được
    │           lớp mã hoá NGOÀI (Layer 2) của chính agent, KHÔNG giải mã được
    │           Layer-1 (browser SubtleCrypto blob) — nhánh này BỊ HỎNG, code tự
    │           cảnh báo "agent may fail auth if key not plaintext"
    │
    ├── [3] BUILD agent environment (buildAgentEnv(), agent-spawner.ts:194-257)
    │       HOME, PATH=config.toolPath (⚠️ KHÔNG từ profile.shell.pathAdditions —
    │           field đó dead code, không đọc profile nào cả)
    │       TERM, ORCA_AGENT_CWD/ACCOUNT_ID/TASK_ID/USER_ID/PROJECT_ID
    │       GH_CONFIG_DIR=~/.config/gh/<userId>/     ← per-user isolation, hardcode
    │           theo process.env.HOME, không đọc từ profile
    │       GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/
    │       ANTHROPIC_API_KEY / OPENAI_API_KEY / GEMINI_API_KEY  (từ bước 2)
    │       ⚠️ KHÔNG set ANTHROPIC_MODEL, KHÔNG set trust-preset args — field
    │           `trustPreset` có khai báo trong interface nhưng KHÔNG được đọc
    │       ...rồi spread extraEnv opaque từ caller (bất kỳ giá trị nào caller gửi)
    │
    ├── [4] SPAWN agent via node-pty
    │       node-pty.spawn(spec.binary, args, { cwd, env, cols: 220, rows: 50 })
    │       ⚠️ cols/rows HARDCODE — không nhận từ caller dù RPC cho phép gửi
    │
    ├── [5] Theo dõi trạng thái — KHÔNG phải OSC-133 parsing là cơ chế chính
    │       Cơ chế chính: agent CLI tự POST JSON tới HTTP loopback nội bộ khi có
    │       hook lifecycle event (PreToolUse/PostToolUse/Stop...) — xem §11.3.
    │       OSC/terminal-title parsing chỉ là fallback thứ yếu, phần lớn nằm
    │       ngoài `agent/` (backend/src/shared/terminal-title-status.ts)
    │
    ├── [6] STREAM PTY output → Gateway
    │       pty.onData → notification 'agent.output' { ptyId, data }  (KHÔNG có id)
    │       ⚠️ ptyId KHÔNG về đồng bộ trong response ban đầu. Dispatcher gọi
    │       `void handleAgentSpawn(...)` (fire-and-forget) rồi trả ngay
    │       { result: { type: 'spawn.accepted' } } cho request id gốc — giá trị
    │       ptyId thật do handleAgentSpawn trả về BỊ BỎ QUA vì caller dùng `void`.
    │       ptyId chỉ lộ diện gián tiếp qua notification 'agent.output'/'agent.exited'
    │       sau đó.
    │
    └── [7] CLEANUP on completion
            pty.kill() → notification 'agent.exited' { taskId, ptyId, exitCode }
            (Không xác nhận được "AiCredStore key zeroized from memory" trong
             phạm vi audit — không khẳng định claim này)
```

### 11.2 Hai roster AI Agent CLI tách biệt — không hợp nhất

Đây là điểm dễ gây nhầm lẫn nhất: tài liệu cũ trình bày như thể có MỘT danh sách agent thống nhất. Thực tế có **hai roster độc lập, phục vụ hai cơ chế launch khác nhau**:

| | `TUI_AGENT_CONFIG` | `AGENT_SPECS` |
|---|---|---|
| Số lượng agent | **32** (`agent/src/shared/types.ts` union `TuiAgent`) | **5**: claude, codex, gemini, opencode, ollama |
| Cơ chế launch | RPC `pty.spawn` — spawn **shell**, sau đó gõ/paste lệnh agent CLI vào shell đã chạy (`commandDelivery: 'renderer'\|'provider'`) | RPC `agent.spawn` — `node-pty.spawn(spec.binary, ...)` spawn **thẳng binary agent** làm process gốc của PTY |
| Dùng cho | F02/F04 UI "mở terminal, tự động gõ lệnh" | Luồng headless/profile-aware spawn (Task → Agent) |
| Model whitelist / trust preset | N/A ở tầng agent | 🚧 Không enforce (§11.1 bước 1) |
| "Custom binary" (profile-defined) | Có thể qua generic CLI entry | ❌ Không có nhánh xử lý — `resolveAgentSpec()` chỉ có 5 spec cứng, model lạ → lỗi |

> **`pty.spawn` ≠ `agent.spawn`.** Đây là 2 RPC method khác nhau với model I/O hoàn toàn khác nhau — tài liệu cũ (và một số BL-PRF-04) từng nhầm lẫn dùng tên `pty.spawn({cmd: agentBinary})` để mô tả hành vi thật của `agent.spawn`.

Với 5 model family của `AGENT_SPECS`:

| Agent | Binary | Args (cố định theo model) | API key env |
|-------|--------|-------------|-------------|
| **Claude Code** | `claude` | `['--output-format', 'stream-json', '--verbose']` hoặc `['--resume', id]` | `ANTHROPIC_API_KEY` |
| **OpenAI Codex** | `codex` | `['--session-file', '~/.codex/<id>.json']` khi resume | `OPENAI_API_KEY` |
| **Gemini CLI** | `gemini` | `['--stream']` cố định — **không nhận `resumeId`** dù bảng cũ ghi "⚠️ Partial" support | `GEMINI_API_KEY` |
| **OpenCode** | `opencode` | `buildArgs: () => []` — **không nhận `resumeId` dưới bất kỳ hình thức nào**, dù tài liệu cũ ghi "✅ Full" resume support | per-provider |
| **Ollama (local)** | `ollama` | model-dependent | Không cần (local HTTP) |

> ❌ **Sửa lại claim resume:** chỉ Claude và Codex thực sự nhận `resumeId`. Gemini và OpenCode KHÔNG hỗ trợ resume ở tầng agent, bất kể tài liệu tính năng ghi gì.

### 11.3 State machine trạng thái Agent — 3 mô hình song song, không hợp nhất

Không có state machine 5-state thống nhất `idle→running→waiting_for_input→completed/error` dựa trên OSC 133 như tài liệu cũ mô tả. Thực tế có **3 state machine độc lập**, phục vụ 3 mục đích khác nhau, không cái nào khớp mô tả cũ:

| State machine | Định nghĩa tại | States | Mục đích |
|---|---|---|---|
| `AgentLifecycleState` | `agent/src/relay/agent-spawner.ts:46` | `idle, spawning, running, stopping, stopped, error` (6 state) | Vòng đời PTY-process của `SubAgentSpawner` |
| `AgentStatusState` | `agent/src/shared/agent-status-types.ts:18` | `working, blocked, waiting, done` (4 state) | UI status hook-driven, dùng bởi `AgentHookServer` |
| `AgentStatus` | `agent/src/shared/agent-title-core.ts:12` (backend-side) | `working, permission, idle` (3 state) | Fallback OSC/terminal-title parsing khi không có hook |

Cơ chế thu thập trạng thái **chính** không phải OSC-133: agent CLI (Claude/Codex/...) tự POST JSON tới HTTP loopback nội bộ (`AgentHookServer`/`RelayAgentHookServer`) khi có lifecycle event thật (`PreToolUse`, `PostToolUse`, `Stop`...) — cơ chế này có logic anti-flicker/dedupe khá phức tạp (`shouldKeepClaudePermissionVisible` và tương tự, `server.ts:312-436`). OSC/title-parsing chỉ là fallback thứ yếu.

### 11.4 Error Recovery & Resilience

Error code thật dùng 2 dải: JSON-RPC chuẩn (`-32700`..`-32000`) và agent-specific riêng (`-33001`..`-33101`, ví dụ `AuthFailed = -33101`) — không khớp dải `4001-5004`/`4xxx` mà một số tài liệu CR-DS-002 từng mô tả. Không có bước "Model not whitelisted" trả lỗi — vì không có check nào tồn tại (§11.1 bước 1). Các chi tiết vận hành khác (buffer 10MB khi mất kết nối, timeout "agent hung 5 phút") **chưa được audit lần này xác nhận** trong `agent/src` — không khẳng định các con số cụ thể này cho tới khi verify riêng.

### 11.5 Agent Isolation trên Dev Server

| Mechanism | Thật |
|-----------|----------------|
| AI credential | Load từ `~/.orca/credentials/<accountId>.enc` (⚠️ không phải `~/.orca/ai-providers/`) |
| **Dead-code trap** | `agent/src/relay/ai-provider-handler.ts` dùng đúng path `~/.orca/ai-providers/` mà tài liệu cũ mô tả — nhưng file này có **0 caller** (xác nhận qua `impact()`), và tự claim sai trong comment đầu file là "AES-256-GCM" dù thân hàm chỉ ghi thẳng `{encryptedBlob, iv, updatedAt}` ra JSON, không mã hoá gì thêm. Đừng sửa file này tưởng là đang sửa hành vi thật — path thật đang chạy là `agent-credential-store.ts` |
| GH config | `GH_CONFIG_DIR=~/.config/gh/<userId>/` riêng mỗi user — đúng, nhưng chỉ trong `agent/` |
| PTY ownership | `ptyId` bind với `userId` |
| Model whitelist | 🚧 Không tồn tại — xem §11.1 bước 1 |
| Usage tracking | 🚧 "Ghi vào `agent_task_runs` local SQLite" chưa xác nhận có code thật — đây là đề xuất ADR-019 |
| Env zeroization | Chưa xác nhận được trong phạm vi audit |

### 11.6 `agent.exec` — bẫy tài liệu thật sự trong code

Có **2 implementation cùng tên khái niệm `agent.exec`**, chỉ một cái sống:

- **Đang chạy thật:** inline `case 'agent.exec'` trong `agent-rpc-dispatch.ts:594-624` — nhận `binary`/`args`/`cwd`/`env` trực tiếp từ params, generic passthrough executor, **không có docblock giải thích**.
- **Chết hoàn toàn:** `handleAgentExec()` trong `agent-exec-handler.ts:355-451` — có docblock đầy đủ, tự nhận là con đường mà backend's `ProfileAwareAgentSpawner` gọi tới, tự `resolveAgentSpec()`, tự build env riêng — nhưng **không được import/dispatch ở bất kỳ đâu** ngoài chính file nó và test riêng. **Bỏ qua `params.env`/`extraEnv` hoàn toàn.**

⚠️ Bất kỳ ai sửa `handleAgentExec()` nghĩ rằng đang sửa hành vi thật của RPC `agent.exec` sẽ không thấy tác dụng gì — đây là bẫy tài liệu-trong-code đáng lưu ý khi maintain.

### 11.7 Profile Hierarchy không điều khiển agent spawn trong `agent/`

`agent/src/main/profile/OrcaProfile.ts` định nghĩa đầy đủ shape 3-tầng company/dept/user (`OrcaProfile`/`ResolvedProfile`/`AgentProfileSection`/...) — nhưng đây là **type-only dead code trong phạm vi `agent/`**: không có `ProfileResolver`/`ProfileCache`/`deepMergeProfiles` nào trong `agent/src`, và `buildAgentEnv()` (§11.1 bước 3) không đọc field nào của `ResolvedProfile`. Nếu PATH/env/model/trust preset thật sự được áp dụng, chúng phải đến từ Backend qua `extraEnv`/`params.env` passthrough — `agent/` **tin tưởng mù quáng vào caller** thay vì tự enforce chính sách profile.

---

## 12. External APIs từ Dev Server — Connectors

> ❌ **Phát hiện quan trọng nhất của mục này:** Backend/Gateway **không relay** các nghiệp vụ GitHub/GitLab (PR/issue/MR) tới Dev Server Agent như phần dưới đây mô tả — Backend tự thực thi `gh`/`glab` ngay trong process của chính mình. Phần §12.1–§12.5 mô tả **implementation đã có sẵn phía Agent, đúng nguyên tắc thiết kế "CLI-based, per-user isolation, auth never through Gateway"** — nhưng tính đến thời điểm audit, implementation này **không được Backend gọi tới cho bất kỳ nghiệp vụ PR/issue/MR nào** (đã grep toàn bộ `backend/src` cho `relay.call`/`mux.request` tới `git.pr.*`/`github.*`/`gitlab.*` nghiệp vụ — 0 kết quả). Xem bằng chứng đầy đủ tại `audit/backend/backend-vs-design-review.md` §2.12b.
>
> Luồng thật mà RPC `github.*`/`gitlab.*` phục vụ frontend hiện đang chạy: Backend RPC handler → `OrcaRuntimeService` → `ghExecFileAsync`/`gitExecFileAsync` → `child_process.execFile('gh'/'glab', ...)` **ngay trên host Backend** — không set `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`, nghĩa là trong Web Server multi-user mode mọi user share chung một gh/glab auth context.
>
> Chỉ 2 luồng hẹp thật sự đi qua relay: `*.startAuthLogin` và `preflight.check` khi có `devServerId` (§12.4).

### 12.1 GitHub Connector (gh CLI-based) — implementation phía Agent

```
Dev Server Agent nhận RPC (2 case ĐỘC LẬP cho cùng chức năng "tạo PR"):

  case 'git.pr.create'      → agent-git-handler.ts::handleGitPrCreate()
  case 'github.pr.create'   → external-api-connector.ts::handleGitHubPrCreate()
                                (CÓ idempotency check — checkExistingPr() — cái kia không có)
    │
    ├── Set env: GH_CONFIG_DIR=~/.config/gh/<userId>/
    ├── execFile('gh', ['pr', 'create', '--title', title, '--body', body, '--base', base])
    │     → HTTPS api.github.com/graphql (gh CLI handles OAuth token)
    ├── Parse stdout: PR URL
    └── Return: { url, stdout, stderr }
```

**RPC methods thật đã wire vào dispatcher** (`agent-rpc-dispatch.ts`, namespace `github.*`, KHÔNG phải `git.*` phẳng như bảng cũ):

| Operation | RPC Method thật | Ghi chú |
|-----------|-----------|-------|
| Create PR | `git.pr.create` **và** `github.pr.create` | Duplicate — xem cảnh báo trên |
| Merge PR | `github.pr.merge` | |
| List issues | `github.issue.list` | |
| Create issue | `github.issue.create` | |
| Preflight | `preflight.check` | 1 trong 2 luồng thật sự relay |

**Có implementation trong `external-api-connector.ts` nhưng KHÔNG có case trong dispatcher (chưa wire, code chết):** `handleGitLabMrList` (mr.list), `handleGitHubAuthStatus`, `handleGitLabAuthStatus`. `git.pr.view`, `git.mr.view`, `git.repo.clone` **không có implementation nào** trong `agent/` — clone dùng `GitCloneHandler` qua đường khác, không qua RPC `repo.clone`.

**Auth isolation (đúng trong `agent/`, nhưng chỉ có hiệu lực khi Backend thực sự relay tới đây):**
```
~/.config/gh/<userId>/               ← per-user gh config dir
GH_CONFIG_DIR env var set bởi buildAgentEnv() + external-api-connector.ts
```

### 12.2 GitLab Connector (glab CLI-based)

Tương tự — chỉ `gitlab.mr.create` và `gitlab.pipeline.status` được wire vào dispatcher; `mr.view`/`mr.list`/`issue.create` GitLab có code trong `external-api-connector.ts` nhưng chưa có case dispatch.

### 12.3 AI Provider APIs (qua credentials lưu local)

AI agent CLIs gọi AI Provider APIs trực tiếp từ Dev Server:

| Provider | Agent | Endpoint | Auth | Credential source |
|----------|-------|---------|------|------------------|
| Anthropic | claude | `api.anthropic.com` | `ANTHROPIC_API_KEY` | `agent-credential-store.ts` hoặc plaintext injected (§11.1) |
| OpenAI | codex | `api.openai.com` | `OPENAI_API_KEY` | như trên |
| Google | gemini | `generativelanguage.googleapis.com` | `GEMINI_API_KEY` | như trên |
| **Ollama (local)** | any | `http://localhost:11434` | None | Env `OLLAMA_HOST` |

### 12.4 Preflight Check Flow

```
Gateway → relay.call('preflight.check', { userId })
    │
    ▼ Dev Server thực thi song song: gh auth status, glab auth status, node --version,
      ollama api/version (nếu có), df -h /
    ▼ Stream về Gateway
```

Đây là **một trong hai luồng hẹp** mà Backend thực sự relay ra Dev Server cho gh/glab (cùng với `*.startAuthLogin`) — phần lớn nghiệp vụ GitHub/GitLab khác thì không (xem cảnh báo đầu §12).

### 12.5 External API Connector — Nguyên tắc thiết kế vs thực thi

| Nguyên tắc | Đúng trong `agent/`? | Ghi chú |
|-----------|----------------|---|
| CLI-based, not SDK | ✅ | `execFileCaptured`/`spawn('gh'/'glab', ...)` |
| Per-user isolation | ✅ trong `agent/`, ❌ ở Backend | Backend không set `GH_CONFIG_DIR` (§6, §9) |
| No shell injection | ✅ | `shell: false` |
| Idempotency (PR create) | ⚠️ Một phần | Chỉ bản `external-api-connector.ts` có `checkExistingPr()`; bản `agent-git-handler.ts` (`git.pr.create`) thì không |
| Rate-limit protection dùng chung | ❌ **Không bảo vệ đường PR/MR thật** | `agent/src/main/git/runner.ts` có circuit-breaker + retry + WSL-routing tinh vi (`ghExecFileAsync`/`glabExecFileAsync`), nhưng có **đúng 1 caller trong toàn bộ `agent/`**: `commit-message-text-generation.ts`. Cả `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts` đều tự viết lại `execFile`/`spawn` thô, **bypass hoàn toàn** circuit-breaker này — nghĩa là chính con đường PR/MR-create thật sự lại không được bảo vệ khỏi rate-limit storm mà `gh-rate-limit-breaker.ts` được thiết kế để chặn |
| Auth never through Gateway | ✅ trong `agent/`, ❌ ở Backend (Backend giữ session gh/glab cục bộ) | |

**3 lớp thực thi git/gh song song, không share code:** `GitHandler` (engine chính, `git.exec`/`git.execStream` generic passthrough + 35+ method nội bộ), `agent-git-handler.ts` (PR/issue/worktree riêng), `external-api-connector.ts` (gh/glab CLI wrapper riêng) — mỗi lớp tự viết `execFile`/`spawn`, tự build env, không dùng chung `runner.ts`.

---

## 13. Feature → Dev Server Component Mapping (đã sửa)

| Feature | Gateway Component | Dev Server Component (thật) |
|---------|------------------|---------------------|
| F01 Parallel Worktrees | ProjectService (quota, RBAC) | `GitHandler` (git worktree add/remove/list qua `git.worktree.*`) — không có RPC `worktree.fanout` |
| F02 Terminal Splits | WsSessionRouter (routing) | `pty-daemon-client/server` (quần thể PTY riêng, khác `agent.spawn`) |
| F04 AI Agent Support | ProfileResolver + ProviderResolver (Backend-tier) | `SubAgentSpawner`/`resolveAgentSpec()` — chỉ 5 model family, không đọc profile (§11.7) |
| F06 GitHub/Linear | WebCredentialStore (tokens) | ⚠️ Implementation có sẵn (`external-api-connector.ts`) nhưng **không được Backend gọi** — Backend tự chạy `gh`/`glab` cục bộ (§12) |
| F12 File Explorer | — (proxy) | `fs.*` handlers (không có `SecureFs` enforcement) |
| F27 Fleet Health | FleetHealthMonitor (chỉ SSH status, không CPU/RAM/disk) | — |
| F28 Dev Server Onboard | DevServerProvisioner | First-connect bootstrap |
| F30 Remote Integrations | WebCredentialStore | ⚠️ Cùng caveat F06 — chỉ `*.startAuthLogin`/`preflight.check` thật sự relay |
| F34 Project Binding | ProjectService | 🚧 Không có "ContextVerifier (enforce)" nào ở agent-side — binding chỉ enforce ở tầng routing của Backend (`ProjectServerRouter`) |
| F35 AI Provider Mgmt | AIProviderService (meta only) | `agent-credential-store.ts` (`~/.orca/credentials/`) — không phải `AiCredStore`/`~/.orca/ai-providers/` |
| F36 Workflow Orchestration | WorkflowOrchestrator (DAG, dispatch) | 🚧 **Không có StepExecutor trong `agent/`** — nhóm RPC `step.*` hoàn toàn không tồn tại |
| F37 Task Graph | TaskService (grant, plan) | `SubAgentSpawner` (không phải `ProfileAwareAgentSpawner` — component đó là Backend-tier) |
| F38 Project Workspace | WorkspaceContext | `fs.*` + `GitHandler` |
| F39 Remote Git UI | — (proxy) | `GitHandler` qua `DevServerGitProvider` — **ném lỗi "not supported" cho git log, AI commit message, branch/commit diff, submodule status** khi repo host là Dev Server (xác nhận `audit/backend/backend-vs-design-review.md` §5.18/F39) |

---

## 14. Sơ đồ tổng quan (đã sửa)

```
═══════════════ GATEWAY (Control Plane) ═══════════════

 ProfileResolver  → resolved profile (Company ← Dept ← User)   [chỉ áp dụng ở Backend]
 ProjectService   → project.devServerId
 AIProviderSvc    → provider metadata (no credentials)
 WorkflowOrch     → DAG dispatch to dev servers
 TaskService      → task → agent spawn via relay
         │
         │  AgentWebSocketServer / DevServerRelayBridge
         │  (KHÔNG ký RpcExecutionContext nào)
         │
         └──── [wss:// :6769/agent outbound / SSH channel] ────┐
                                                                ↓
═══════════════ DEV SERVER AGENT (Data Plane) ═══════════════

 createRpcDispatcher() / route()   (KHÔNG có ContextVerifier)
     │
     ├── PTY_REGISTRY + SubAgentSpawner
     │           │
     │           ├──── node-pty ────→ claude / codex / gemini / opencode / ollama
     │           │                     (chỉ 5 model family — KHÔNG phải "Custom agent binary")
     │           │                     │ stdin/stdout + hook-JSON qua HTTP loopback
     │           │                     ↓
     │           └── agent-credential-store.ts ──→ AI Provider APIs
     │                                 api.anthropic.com / api.openai.com /
     │                                 generativelanguage.googleapis.com /
     │                                 localhost:11434 (Ollama)
     │
     ├── GitHandler (git.exec/git.execStream generic passthrough, git.worktree.*)
     │
     ├── agent-git-handler.ts ──→ gh CLI ──→ api.github.com   ⚠️ Backend hiện KHÔNG
     ├── external-api-connector.ts ──→ glab CLI → gitlab.com     gọi tới nhánh này (§12)
     │
     ├── fs.* (KHÔNG có SecureFs enforcement)
     ├── (KHÔNG có StepExecutor — step.* group không tồn tại)
     ├── (Health polling thật nằm ở Backend, không phải agent emit)
     └── ws.send()/dispatcher.notify() → notification tới Gateway
             (pty.data/pty.exit/fs.changed — §15.3)
```

---

## 15. Execution-Host Unification qua Provider Registry (2026-08 — ĐÃ TRIỂN KHAI)

> **Phạm vi mục này:** Đây là mô tả **chính xác những gì đã code & deploy thực tế** trong session 2026-08, và đã được audit vòng này **xác nhận khớp code** (`audit/backend/backend-vs-design-review.md` §2.9: "✅ Khớp hoàn toàn"). Nó **KHÔNG** thay thế hay viết lại phần kiến trúc mục tiêu/tầm nhìn "v6.0" đã bị loại bỏ khỏi §3–§14 phía trên (`ContextVerifier`, `SignedExecutionContext`...) — coi vision đó là một hướng phát triển khác, chưa được build.

### 15.1 Vấn đề trước đây

Trước session này, Orca có **hai con đường tách biệt** để bind một repo/project vào remote host:

1. **SSH Targets/Hosts** (hệ cổ điển) — mọi thao tác file/git/terminal đều đi qua provider-registry abstraction (`IFilesystemProvider` / `IGitProvider` / `IPtyProvider`, keyed theo connection-id dạng chuỗi opaque).
2. **Dev Server** — agent connect **outbound** qua WebSocket về Gateway (chính là mode `direct-websocket` ở §4), nhưng **vô hình** với hệ Repo/execution-host cổ điển ở trên — chỉ được wire vào một luồng onboarding riêng, hẹp.

Kết quả: hai kiến trúc song song, không dùng chung interface, dù cả hai đều "chạy lệnh trên máy remote".

### 15.2 Thay đổi: một execution-host abstraction duy nhất

Connection outbound sẵn có của Dev Server agent giờ được đăng ký thẳng vào **cùng bộ provider registry** mà hệ SSH đã dùng từ trước. Một repo giờ có thể bind vào SSH Target **hoặc** Dev Server thông qua **một** execution-host abstraction duy nhất — Dev Server **không cần** một kết nối thứ hai/riêng biệt nào.

> ⚠️ **Lưu ý phạm vi package:** ba provider class dưới đây sống ở `backend/src/main/providers/` (và bản song song `desktop/src/main/providers/`) — **KHÔNG phải** trong `agent/src/main/providers/`. Thư mục cùng tên trong `agent/` chứa nội dung hoàn toàn khác (Windows foreground-process detection cho agent status recognition, không liên quan tới provider registry này).

**Ba provider class mới** (`backend/src/main/providers/`) đóng gói RPC surface hẹp của agent (`fs.*`, `git.exec`, `pty.*`) thành đúng interface `IFilesystemProvider`/`IGitProvider`/`IPtyProvider` mà phần còn lại của Orca đã kỳ vọng ở bất kỳ execution host nào:

| Provider | File | Chức năng |
|----------|------|-----------|
| `DevServerFilesystemProvider` | `dev-server-filesystem-provider.ts` | fs.stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep — nay có thêm fs.watch/fs.unwatch real-time (xem §15.3) |
| `DevServerGitProvider` | `dev-server-git-provider.ts` | Một method generic đã whitelist `git.exec({ args, cwd })` + các method worktree add/remove/list riêng; tái dùng nguyên status-porcelain parser đang có, không đổi. ⚠️ Ném lỗi "not supported" cho git log, AI commit message, branch/commit diff, submodule status khi repo host là Dev Server (xem §13/F39) |
| `DevServerPtyProvider` | `dev-server-pty-provider.ts` | pty.create/write/resize/destroy/scrollback/sendSignal + nhận push real-time pty.data/pty.exit |

`DevServerPtyProvider` là **mảnh ghép cuối cùng** hoàn thiện trong session này. Một số method của `IPtyProvider` (`getCwd`, `hasChildProcesses`, `getForegroundProcess`, `serialize`/`revive` để persist session qua restart) không có tương đương trung thực phía agent → được implement như **approximation an toàn, có ghi chú rõ trong code**, thay vì throw (vì interface bắt buộc phải có các method này):

- `getCwd()` trả về cwd tại thời điểm spawn — không có live shell-integration/OSC 7 tracking để biết cwd hiện tại thực sự.
- `hasChildProcesses()` luôn trả `false` — permissive default thay vì đoán mò.
- `serialize()`/`revive()` là no-op vì agent không có cross-restart session persistence.

Mỗi provider được đăng ký/hủy đăng ký **tự động** bởi `wireDevServerProviders()` (`dev-server-provider-lifecycle.ts`) mỗi khi Dev Server connect/disconnect. Đây là **pure listener** trên các event lifecycle connection sẵn có (`devServer:statusChanged`, `devServer:removed`) — nó không tự quản lý connection, DevServerManager/AgentWebSocketServer vẫn giữ trách nhiệm đó.

### 15.3 Notification Relay — Agent chủ động push (năng lực mới)

Trước session này agent **chỉ trả lời request** — không có cách nào chủ động đẩy dữ liệu về Gateway. Session này thêm cơ chế **one-way JSON-RPC notification** (cùng wire format ở §5, chỉ khác là không có field `id`), cho phép agent push:

| Notification | Ý nghĩa |
|---------------|---------|
| `pty.data` / `pty.exit` | Output/exit của terminal, stream real-time thay vì chỉ pollable qua scrollback buffer |
| `fs.changed` | File-change event real-time, **refcounted theo từng path** — để nhiều user cùng share 1 Dev Server không vô tình tear down watch của nhau |

> ⚠️ **`fs.watch` thật ra có 2 implementation song song, theo transport — không phải một cơ chế duy nhất như đoạn trên ngụ ý:**
> - Binary `agent.js` (mode `direct-websocket`/`relay-websocket`, entry point `agent-entry.ts`) → `handleFsWatch()` (`fs-agent-extensions.ts`) dùng `fs.watch` **built-in của Node** — đơn giản, `Map` refcount, `recursive:true` chỉ hoạt động trên macOS/Windows (Linux chỉ watch top-level directory theo comment trong code).
> - Binary `relay.js` (mode `relay-ssh`, §3a) → cluster `@parcel/watcher` tinh vi hơn nhiều (crash-isolation qua child-process riêng, quarantine pool, batch cap 5000 events) — **không được nhắc tới ở đâu trong thiết kế**, và build-pipeline của nó trong gói `agent/` hiện tại chưa xác nhận được (§3a).
>
> Mô tả "`fs.watch` built-in của Node" trong đoạn dưới chỉ đúng cho nhánh `agent.js`/WS.

**Vấn đề cần giải quyết:** trong multi-user web mode, mỗi user đăng nhập được xử lý bởi **một per-user child process riêng** (`SessionManager` + `fork()` — xem [backend-server-architecture.md](./backend-server-architecture.md) §5, §7), nhưng connection Dev Server thật sự chỉ sống trong **một process cha/gateway dùng chung**. Để notification đến đúng nơi:

```
Dev Server Agent
    │ pty.data / pty.exit / fs.changed  (JSON-RPC notification, không có id)
    ▼
Gateway (parent process) — DevServerManager
    │ emit('devServer:notification', devServerId, method, params)
    ▼
SessionManager → broadcast tới MỌI user child process:
    proc.process.send({ type: 'devServer:proxyNotification', devServerId, method, params })
    │
    ▼ mỗi child process: GatewayDevServerManagerProxy.handleNotification()
    → dispatch tới local subscriber thực sự quan tâm devServerId đó
      (DevServerPtyProvider đang giữ PTY đó, hoặc 1 filesystem watcher đang watch path đó)
```

Cơ chế broadcast `devServer:proxyNotification` này chạy **song song** với `devServer:event` (status broadcast: added/removed/statusChanged) vốn đã có từ trước — cùng pattern IPC, khác payload/mục đích.

### 15.4 Capability Negotiation — theo dõi end-to-end

Agent vốn đã luôn advertise một list `capabilities` lúc handshake (vd. `fs`, `git`, `pty`, `preflight`), nhưng trước đây **Gateway bỏ qua hoàn toàn** — không có chỗ nào parse list này. Session này thêm phần plumbing còn thiếu: handshake receiver → connection bridge → `DevServerManager` runtime state → object `DevServer` expose ra phần còn lại của hệ thống — để capabilities được track theo từng Dev Server.

**Hai capability string mới:** `fs.watch` và `pty.stream` — cho phép Gateway phân biệt một agent binary cũ (chỉ hỗ trợ fs/pty theo kiểu request/response) với một agent hỗ trợ real-time push mới. Đây chính là điều kiện `wireDevServerProviders()` kiểm tra trước khi quyết định có đăng ký `DevServerPtyProvider` cho một Dev Server hay không:

```typescript
const ptyReady = capabilities.includes('pty') && capabilities.includes('pty.stream')
```

> Nếu thiếu capability này, `DevServerPtyProvider` **không được đăng ký** — khác với file-watching (có thể fallback về polling khi thiếu `fs.watch`), **không có đường fallback nào cho một terminal thiếu live output stream**.

### 15.5 Yêu cầu vận hành: `node-pty` phải được cài riêng trên mỗi Dev Server

PTY support trên một Dev Server cần binary native của `node-pty` — thứ này **cố tình không được bundle** vào `agent.js`, vì đa số Dev Server không cần PTY và đây là một dependency native/compiled làm nặng agent binary một cách không cần thiết.

**Phát hiện thực tế khi rollout:** cả 3 Dev Server production hiện có đều ban đầu báo hoàn toàn không hỗ trợ PTY. Root cause **không phải bug code** — `node-pty` đơn giản là chưa từng được cài trên các máy đó. Khắc phục bằng cách, trên từng Dev Server:

1. Cài build tools: `g++`/`gcc`/`make`
2. `npm install node-pty` tại home directory của agent
3. Restart agent service để nó re-detect native module vừa có sẵn

> **Đây là bước bắt buộc phải lặp lại cho MỌI Dev Server mới** cần hỗ trợ terminal. Git/filesystem operations hoạt động ngay không cần cài thêm gì — riêng PTY mới cần bước cài đặt này.

### 15.6 Cố ý chưa làm (không phải thiếu sót của session này)

Clone một repo **hoàn toàn mới** lên Dev Server (khác với mở một folder/checkout đã có sẵn trên đó) **chưa được implement**. Code clone hiện tại phụ thuộc vào các khái niệm chỉ tồn tại ở SSH (remote host-platform detection, remote home-directory resolution) mà Dev Server chưa có tương đương. Thử thao tác này bây giờ sẽ trả về lỗi rõ ràng — thay vì rủi ro âm thầm-sai trước đây (sẽ vô tình clone vào filesystem local của chính container Gateway).

---

## 16. Nợ kiến trúc & khoảng trống bảo mật đã xác nhận (tổng hợp)

Mục này tổng hợp lại — không lặp chi tiết — các phát hiện nghiêm trọng nhất từ 6 audit làm nguồn cho bản sửa này, để người đọc không phải lục lại từng section:

1. **Backend tự thực thi GitHub/GitLab thay vì relay Dev Server Agent** (§9, §12) — vi phạm trực tiếp nguyên tắc "Auth never through Gateway" mà chính tài liệu này đặt ra; không per-user isolation trong Web Server multi-user mode.
2. **Plaintext AI credential injection ở luồng spawn** (§6, §11.1) — mâu thuẫn với khẳng định "Gateway không thấy plaintext key"; nhánh fallback không có `resolvedApiKey` thực chất bị hỏng (inject ciphertext Layer-1 chưa giải mã).
3. **Không có author-identity enforcement thật** (§7) — "Git author injected từ ctx.userEmail, không thể override" không tồn tại; chỉ có `git config --global` một lần, mutable, không gắn request context.
4. **3 implementation git/gh song song, duplicate `git.pr.create`/`github.pr.create`** (§12.5) — circuit-breaker/retry (`runner.ts`) tinh vi có sẵn nhưng chỉ 1 caller trong toàn `agent/`, không bảo vệ đường PR/MR-create thật.
5. **FS-path allowlist đã bị chủ động gỡ bỏ** (§6.2) — quyết định kiến trúc có chủ đích, dồn trust boundary lên renderer/SSH-user.
6. **Token lifecycle self-service, không revoke được** (§6.1) — định dạng token đoán được (`agt-<devServerId>-<timestamp>`), không DB, không admin UI.
7. **Port 6768 vs 6769** (§4) — sai trong hầu hết tài liệu cũ, và sai ngay trong error message runtime của chính code (`agent-ws-server.ts:103`).
8. **3 state machine trạng thái agent không hợp nhất** (§11.3) — không cái nào khớp mô tả "OSC-133, 5 state" của tài liệu cũ.
9. **`OrcaProfile` là dead code trong `agent/`** (§11.7) — profile hierarchy không điều khiển agent spawn ở tầng này, dù tài liệu nhiều nơi ngụ ý có.
10. **`handleAgentExec()` là bẫy tài liệu-trong-code** (§11.6) — có docblock đầy đủ nhưng chết hoàn toàn; code thật không có docblock.
11. **Telemetry/observability/diagnostics trong `agent/` là dead code copy từ `desktop/`** (kể cả import `electron` trong một gói headless Node) — không có thiết kế nào tương ứng; không đưa vào bản sửa chi tiết ở trên vì các module này không nằm trong sơ đồ kiến trúc chính, nhưng cần dọn dẹp riêng (xem `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.3 để biết chi tiết đầy đủ).
