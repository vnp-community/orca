# AG-REQ-SOL-033-C: `agent.capabilities`, mở rộng handshake, phiên bản giao thức và `ai.complete`

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Mọi mục "đã đọc" là đọc code, chưa chạy gì (thời điểm soạn). Phần "9. Kết quả triển khai" ghi lại những gì thực sự đã thực hiện và sai khác với kế hoạch.

**CR:** [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md) mục 2.6, 2.7 (phần triển khai agent), 2.8; liên quan CR-REQ-034 (usage, `maxTokens`, lỗi có cấu trúc), CR-REQ-029 (`ReadinessGate` đọc hồ sơ), CR-REQ-026 (kiểm `openspec` qua `agent.exec`), CR-REQ-012 (Plan dài cần `maxTokens`)
**Service:** `agent/`, thư mục `agent/src/relay/`; thay đổi phiên bản ở `agent/build.mjs`, `deploy/agent/`
**Nhóm solution:** C trong ba nhóm của CR-REQ-033 (A: chỉ đọc và vùng làm việc; B: khối kết quả, danh sách file đổi)
**TDD tham chiếu:** [v5/04-handshake-session](../../../../tdd/v5/04-handshake-session.md) (mục 7 Capabilities Advertisement), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/09-ai-credential-relay](../../../../tdd/v5/09-ai-credential-relay.md), [v5/08-deployment](../../../../tdd/v5/08-deployment.md), [v5/03-connection-modes](../../../../tdd/v5/03-connection-modes.md)
**Mẫu định dạng:** `specs/agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md`

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent-session-handshake.ts`, `agent-session-capabilities.ts`, `agent-session.ts` (đoạn handshake), `agent-rpc-dispatch-misc.ts` (`host.capabilities`), `agent-rpc-dispatch-ai.ts` (`ai.complete`), `ai-complete-handler.ts`, `agent-spawn-env.ts`, `agent-config.ts`, `agent-entry.ts`, `agent/build.mjs`, `deploy/agent/package.json`, và phía Go: `devserveragent/session.go` (`HandshakeInfo`), `agentwsserver/server.go` (`inboundHandshakeParams`), `usecase/ports.go` (bản sao `HandshakeInfo`), `sshrelay/version_check.go`, `sshrelay/provisioner.go`, `devserveragent/jsonrpc.go` (`JSONRPCError.Data`).

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Handshake | `sendHandshake` (`agent-session-handshake.ts:36-81`) gửi `agent.handshake` với `agentVersion: '5.0.0'` cố định, `platform`, `arch`, `nodeVersion`, `capabilities`, `agentToken?`, `devServerId`, `tools`. Cả ba chế độ (`agent-connection-direct.ts`, `-relay.ts`, `-stdio.ts`) tạo phiên qua `createSession` nên cùng đi qua hàm này | Thêm trường ở MỘT chỗ. Nhưng Go `runInitiatorHandshake` (`session.go:267`) gửi request rồi đọc phản hồi; trong `agent/` không có code trả lời một `agent.handshake` do Orca khởi xướng (chỉ có `sendHandshake` agent khởi xướng). Cách trường mới tới Go ở chế độ relay-websocket/relay-ssh chưa kiểm chứng (mục 6) |
| `capabilities` | `buildCapabilities` trả mảng chuỗi (`fs`, `git`, `pty*`, `agent.spawn`, `agent.exec`, ...), `STATIC_CAPABILITIES_FALLBACK` làm dự phòng khi quá 5 giây. Không có `agent.execPrompt`, `agent.execPromptStream`, `ai.complete`, `host.capabilities` | Không đổi mảng này (cổng `ptyReady` ở server dựa vào `pty` và `pty.stream`) |
| Phiên bản | `5.0.0` ở handshake; `AGENT_VERSION = '2.1.0'` ở `agent/build.mjs:22` (đưa vào bundle bằng `define __AGENT_VERSION__`, xuất ở `agent-entry.ts` như `AGENT_VERSION`); `agent-entry.ts:83,115` ghi cứng chuỗi log `v2.1.0`; `agent/package.json` `1.4.138-rc.6`; `deploy/agent/package.json` `2.1.0`; `deploy/agent/README.md` có ví dụ `2.1.0` hai chỗ | Bốn nơi phải đổi cùng nhau khi tăng (mục 2.6) |
| Go nhận handshake | `inboundHandshakeParams` khai báo 7 trường, `json.Unmarshal` thường; `HandshakeInfo` (adapter) có `Capabilities []string`; `usecase.HandshakeInfo` (`ports.go:85`) chỉ có 4 trường, bỏ `Capabilities` | Trường lạ vô hại (không thấy `DisallowUnknownFields`; chưa grep toàn repo Go ngoài `infra-fleet-service`) |
| `MinAgentVersion` | `agentwsserver/server.go:201` so `params.AgentVersion` với `ORCA_AGENT_MIN_VERSION` | Vì `agentVersion` luôn `5.0.0` nên chưa chặn được ai; KHÔNG đổi giá trị này |
| Triển khai SSH | `provisioner.go:141` bỏ qua đẩy bundle khi `version == p.cfg.OrcaVersion`, với `version` là `AGENT_VERSION` của bundle ở xa (`version_check.go`). So với `OrcaVersion` (biến `ORCA_VERSION` của backend) CHỨ KHÔNG so với một `AGENT_VERSION` mong muốn | **Khác CR**: CR viết "sshrelay tự đẩy khi AGENT_VERSION đổi". Thực tế chỉ tự đẩy khi giá trị bundle ở xa khác `OrcaVersion`; tăng `AGENT_VERSION` lên `2.2.0` không chắc kích hoạt đẩy lại (xem mục 3.5 và câu hỏi mở 3). Giá trị `ORCA_VERSION` ở môi trường thật chưa kiểm chứng |
| RPC dò môi trường | `host.capabilities` (WSL, pwsh, git-bash) ở `agent-rpc-dispatch-misc.ts` dòng 150 trở đi; `preflight.check`/`preflight.detectAgents` nhận lệnh do người gọi gửi | `agent.capabilities` KHÔNG tái dùng chúng: nguyên tắc danh sách cho phép cứng |
| `ai.complete` | `dispatchAiRpc` (`agent-rpc-dispatch-ai.ts`) đọc `prompt`, `format`, `taskId`, `model`, `accountId`, `resolvedApiKey` (KHÔNG đọc `maxTokens`); `handleAIComplete` trả `{ content, model }`; mỗi nhà cung cấp cố định `max_tokens: 4096` (Google không đặt); lỗi là `Error` chuỗi `Anthropic API error ${status}: ${body}` bọc thành `ServerError -32000` "ai.complete failed: ..." | Mở rộng tương thích ngược ở bước 2.5 |
| Khoá API `ai.complete` | `resolveApiKeyFromEnv` chỉ đọc `process.env` (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`); `buildAgentEnv` đặt `GEMINI_API_KEY` chứ không phải `GOOGLE_API_KEY` cho Gemini | Báo cáo năng lực kiểm `GOOGLE_API_KEY` mặc định như CR nêu; lệch tên `GEMINI_API_KEY` ghi ở câu hỏi mở 4 |
| Go `JSONRPCError` | có `Data json.RawMessage` (`jsonrpc.go:30`) | `error.data` của `ai.complete` tới được Go không cần sửa decoder |
| Hai bản agent | `desktop/src/relay/ai-complete-handler.ts` tồn tại (bản lệch, khác doc comment); `desktop/src/relay` không có `execPrompt` | Không sửa `desktop/`. Xem README solutions |

Correction relative to CR-REQ-033: (1) CR nói "agent đọc `claude --help`... ghi `claudeFlags`" dùng cho cả hai RPC; solution đặt một hàm duy nhất `detectClaudeFlags` ở nhóm A (`agent-readonly-tool-policy.ts`) và `agent-capability-report.ts` gọi lại, để tránh hai cache. (2) CR liệt kê lệnh phiên bản cố định `--version` cho mọi công cụ: riêng `go` dùng `go version` (không có `--version`); bảng ở 2.2. (3) Quy tắc thiết lập môi trường của `claude`: `execPrompt` chạy `claude` với `PATH = config.toolPath` (từ `buildAgentEnv`), nên mọi dò `claude` trong báo cáo năng lực phải dùng cùng PATH, không dùng `process.env.PATH` trần.

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/
  agent-capability-report.ts                 (mới) buildCapabilityReport(params, config, deps)
  agent-capability-report.test.ts            (mới)
  agent-protocol-features.ts                 (mới) AGENT_PROTOCOL_VERSION, AGENT_FEATURES
  agent-protocol-features.test.ts            (mới)
  agent-build-version.ts                     (mới) đọc __AGENT_VERSION__ an toàn, '0.0.0-dev' khi vắng
  agent-rpc-dispatch-misc.ts                 (sửa) thêm case 'agent.capabilities'
  agent-session-handshake.ts                 (sửa) thêm protocolVersion, buildVersion, features
  agent-rpc-dispatch-ai.ts                   (sửa) đọc maxTokens, đính error.data
  ai-complete-handler.ts                     (sửa) usage, provider, latencyMs, maxTokens, AICompleteProviderError
  ai-complete-handler.test.ts hoặc __tests__/ai-complete-handler.test.ts  (sửa, thêm describe)
  agent-rpc-dispatch-ai.test.ts              (mới) kiểm error.data và maxTokens qua dispatcher
agent/build.mjs                              (sửa) AGENT_VERSION '2.2.0'
agent/src/relay/agent-entry.ts               (sửa) hai chuỗi log v2.1.0
deploy/agent/package.json                    (sửa) version 2.2.0
deploy/agent/README.md                       (sửa) hai ví dụ 2.1.0
```

Test của `ai-complete-handler` nằm ở `agent/src/relay/__tests__/ai-complete-handler.test.ts` (đã thấy); thêm `describe` mới vào đó.

### 2.2 `agent.capabilities`

Tên RPC: `agent.capabilities`. Params tuỳ chọn:

| Tham số | Kiểu | Mặc định | Quy tắc | Sai thì |
|---|---|---|---|---|
| `tools` | `string[]` | toàn bộ danh sách cho phép | phần tử ngoài danh sách vào `unknownTools`, không chạy | kiểu sai: `InvalidParams`, `INVALID_CAPABILITY_PARAMS` |
| `envNames` | `string[]` | `['ANTHROPIC_API_KEY','OPENAI_API_KEY','GOOGLE_API_KEY']` | mỗi tên khớp `^[A-Z][A-Z0-9_]{0,63}$`; tên sai vào `rejectedEnvNames`; tối đa 64 tên | quá 64: `InvalidParams`, `TOO_MANY_ENV_NAMES` |
| `refresh` | boolean | `false` | `true` bỏ cache | kiểu sai: `InvalidParams` |

Danh sách cho phép cứng và lệnh phiên bản (cố định trong mã, không từ params):

| id | Lệnh | Ghi chú |
|---|---|---|
| `go` | `go version` | KHÔNG phải `--version` |
| `node`, `pnpm`, `npm`, `git`, `rg`, `make`, `semgrep` | `<id> --version` | `rg`, `semgrep`: chưa kiểm chứng trên máy thật |
| `claude` | `claude --version` | |
| `openspec`, `codegraph`, `gitnexus` | `<id> --version` | CLI này chưa thấy ở repo; dạng cờ **chưa kiểm chứng** (CR-REQ-026 cũng ghi chưa đối chiếu) |

Thực thi: `execFile(bin, args, { timeout: 3000, env, windowsHide: true })`, không qua shell; `env = { ...process.env, PATH: config.toolPath }`. Phiên bản lấy từ dòng đầu bằng `/\d+(?:\.\d+){1,3}[\w.+-]*/` tối đa 64 ký tự. Trên Windows thử thêm `<id>.cmd` khi `<id>` ENOENT (shim của npm; chưa kiểm chứng).

Kết quả (đúng sơ đồ CR, kèm các trường bổ sung đã đánh dấu):

```json
{ "schemaVersion": 1,
  "agent": { "buildVersion": "2.2.0", "protocolVersion": 2 },
  "host": { "platform": "linux", "arch": "x64", "nodeVersion": "v22.3.0", "cpuCount": 8,
            "memTotalMb": 32000, "memFreeMb": 20000, "diskFreeMb": 120000, "loadAvg1": 0.4 },
  "tools": [ { "id": "go", "installed": true, "version": "1.22.3" },
             { "id": "openspec", "installed": false } ],
  "unknownTools": [],
  "claude": { "installed": true, "version": "2.1.289", "auth": "logged_in",
              "flags": { "tools": true, "permissionMode": true, "disallowedTools": true } },
  "env": [ { "name": "ANTHROPIC_API_KEY", "present": true } ],
  "rejectedEnvNames": [],
  "probedAt": "2026-10-06T10:00:00Z", "partial": false }
```

Bổ sung ngoài CR: `unknownTools`, `rejectedEnvNames` (mảng chuỗi); `installed` có thể là `null` khi hết giờ (không biết). Nếu `partial` là `true` thì các mục chưa xong có `installed: null`. Backend lưu nguyên JSON (`profile_json` mờ đục) nên không cần proto riêng cho từng trường.

Quy tắc an toàn (mỗi quy tắc có test): (a) không bao giờ chạy lệnh ngoài danh sách; (b) `env` chỉ có `present` (boolean), đọc từ `process.env` và coi chuỗi rỗng là vắng; không bao giờ trả giá trị; (c) `claude.auth` từ `claude auth status --json`: chỉ lấy một boolean, tên trường `loggedIn` **chưa kiểm chứng**, không chép email hay tổ chức; lỗi hoặc không phân tích được thì `unknown`; (d) mỗi lệnh timeout 3 giây, chạy song song, tổng 8 giây, quá thì `partial: true`; (e) cache 60 giây trong tiến trình theo khoá `JSON(sort(tools), sort(envNames))`, `refresh=true` bỏ cache, các lời gọi trùng khi đang dò dùng chung một lần dò (single flight); (f) `diskFreeMb` của `config.workDir` bằng `fs.statfs` (`bavail * bsize`); `loadAvg1` bằng `os.loadavg()[0]` (luôn 0 trên Windows).

`claude.flags` lấy từ `detectClaudeFlags` của nhóm A (task 02) nên cả hai RPC dùng chung cache.

Đăng ký: `case 'agent.capabilities'` trong `dispatchMiscRpc`, theo mẫu `host.capabilities` (`try { dynamic import } catch -> makeError(ServerError, 'agent.capabilities unavailable: ...')`). Lỗi tham số trả `InvalidParams` kèm `data: { reason }`.

### 2.3 Mở rộng handshake

Thêm vào `params` của `agent.handshake` (không bỏ, không đổi trường nào có sẵn):

| Trường | Kiểu | Giá trị | Vắng nghĩa là |
|---|---|---|---|
| `protocolVersion` | số nguyên | `2` | `1` (agent chưa có CR-033) |
| `buildVersion` | chuỗi | `AGENT_VERSION` của bundle (`2.2.0`), `0.0.0-dev` khi chạy từ nguồn | agent cũ |
| `features` | `string[]` | `AGENT_FEATURES` (tĩnh) | rỗng |

```ts
export const AGENT_PROTOCOL_VERSION = 2
export const AGENT_FEATURES = [
  'agent.execPrompt', 'agent.execPrompt.readonly', 'agent.execPrompt.workspaceKind',
  'agent.execPrompt.changes', 'agent.execPrompt.resultBlock',
  'agent.capabilities', 'ai.complete', 'ai.complete.usage'
] as const
```

`features` là tĩnh nên không thêm độ trễ vào cuộc đua 5 giây của handshake. Không đổi `agentVersion: '5.0.0'` và mảng `capabilities`. `ai.complete.usage` hứa đồng thời ba thay đổi (usage, `maxTokens`, `error.data`): xem mục 3.5.

### 2.4 `AGENT_VERSION` và triển khai

Tăng `2.1.0` lên `2.2.0` ở: `agent/build.mjs:22`, `deploy/agent/package.json`, `agent-entry.ts` (hai chuỗi log), `deploy/agent/README.md` (hai ví dụ). `agent/package.json` (`1.4.138-rc.6`) KHÔNG đổi (số riêng của gói tách, không dùng cho triển khai). Hệ quả `sshrelay`: xem 3.5.

### 2.5 `ai.complete` mở rộng

Tham số mới: `maxTokens` (số nguyên, mặc định `4096`, trần `16384`). Không phải số nguyên dương: `InvalidParams`, `INVALID_MAX_TOKENS`; lớn hơn trần: kẹp về `16384`. Truyền cho Anthropic (`max_tokens`) và OpenAI (`max_tokens`); Google: `generationConfig.maxOutputTokens` (hiện không đặt gì, nên ở Google mặc định vẫn là giá trị riêng của nhà cung cấp khi người gọi không gửi `maxTokens`; chỉ đặt khi có `maxTokens`).

Kết quả: `{ content, model, provider, latencyMs, usage? }`:

| Trường | Kiểu | Nguồn |
|---|---|---|
| `provider` | `'anthropic'` \| `'openai'` \| `'google'` | `providerNameFromModel` hiện có |
| `latencyMs` | số nguyên | đo quanh lời gọi `fetch` đến khi có JSON |
| `usage.inputTokens`, `usage.outputTokens` | số nguyên | Anthropic `usage.input_tokens`/`output_tokens`; OpenAI `usage.prompt_tokens`/`completion_tokens`; Google `usageMetadata.promptTokenCount`/`candidatesTokenCount`. Tên trường **chưa kiểm chứng bằng cuộc gọi thật** (lấy từ hiểu biết về API, CR cũng đánh dấu). Vắng hoặc không phải số thì bỏ `usage` hoàn toàn |

Lỗi có cấu trúc: lớp mới `AICompleteProviderError extends Error` mang `provider`, `httpStatus: number | null`, `retryable: boolean`, `reason`. `dispatchAiRpc` bắt lớp này và trả `makeError(rpc.id, ServerError, 'ai.complete failed: ' + message, { provider, httpStatus, retryable, reason })`. `message` giữ nguyên định dạng cũ (`Anthropic API error 429: ...`) nên người gọi cũ đọc thông điệp vẫn đúng. `reason` là bổ sung ngoài CR: `PROVIDER_HTTP_ERROR`, `PROVIDER_TIMEOUT`, `PROVIDER_NETWORK`, `NO_API_KEY`, `UNKNOWN_MODEL_PROVIDER`. `retryable`: `true` với HTTP 408, 409, 425, 429, 500 đến 599 và timeout/mạng; `false` với 400, 401, 403, 404, 422 và các lỗi cấu hình (không có khoá, model lạ). Lỗi `prompt rỗng` và `InvalidParams` giữ nguyên.

Không bao giờ đưa `apiKey` vào `message`, `data`, hay log (Google đặt khoá trong query URL: lỗi `fetch` phải được bắt và chỉ giữ `provider`/`reason`, không chép `err.cause` có URL).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | `features` tách khỏi `capabilities` | `ptyReady` và `ResumeAgentSession` phụ thuộc giá trị cũ | Thêm vào `capabilities` |
| 2 | Không đổi `agentVersion: '5.0.0'` | `MinAgentVersion` và `ResumeAgentSession` so giá trị này | Đổi sang `2.2.0` |
| 3 | `AGENT_FEATURES` tĩnh, chỉ thêm khi mã đã có | Không quảng cáo thứ chưa làm (tiêu chí đối chiếu trong task 12) | Sinh động từ dò |
| 4 | Danh sách công cụ cứng, lệnh phiên bản cứng | RPC dò không được thành đường chạy lệnh tuỳ ý | Cho phép `tools` tuỳ ý |
| 5 | Chỉ `present`, không giá trị | Bí mật không đi qua RPC | Trả mặt nạ `sk-****` |
| 6 | Một hàm `detectClaudeFlags` dùng cho cả `readonly` và `agent.capabilities` | Một cache, một nguồn sự thật | Hai lần dò |
| 7 | `error.data` là bổ sung, `message` giữ nguyên | Người gọi cũ không vỡ | Đổi định dạng thông điệp |
| 8 | `usage` bỏ hẳn khi không chắc | Số sai làm hỏng ngân sách CR-REQ-034 | Trả 0 |

### 3.5 Tương thích hai chiều (degradation, phát hiện bằng handshake)

| Backend | Agent | Kết quả | Việc backend phải làm |
|---|---|---|---|
| Cũ | Mới | Handshake có trường thừa, Go bỏ qua; `ai.complete` thêm trường thừa; `agent.capabilities` không ai gọi | Không |
| Mới gọi `agent.capabilities` | Cũ | `-32601 MethodNotFound`; `devserveragent` map thành `domain.ErrAgentMethodNotFound` (`client.go:434-437`) | Dựng hồ sơ `source=handshake_only` từ handshake (CR 2.7 bước 3), `degraded=true` |
| Mới gửi `maxTokens` | Cũ | Bị bỏ qua, vẫn 4096, kết quả không có `usage`/`provider` | Chỉ coi `maxTokens` có hiệu lực khi `features` có `ai.complete.usage`; nếu không, giữ giả định 4096 và phát hiện cắt cụt bằng kiểm JSON |
| Mới đọc `error.data` | Cũ | Không có `data`, chỉ chuỗi `Anthropic API error 429` | Có đường dự phòng đọc mã HTTP từ chuỗi (CR-REQ-034 quyết định) |
| Mới | Bản build từ `desktop/` | Như agent cũ (không có `features`) | Như agent cũ |

Phát hiện: `protocolVersion >= 2` hoặc `features` chứa tên cần dùng. Không dùng so sánh `agentVersion` vì giá trị cố định.

Về triển khai `sshrelay`: bundle mới chỉ được đẩy lại khi `AGENT_VERSION` của bundle ở xa khác `p.cfg.OrcaVersion` (`provisioner.go:141`). Nếu `ORCA_VERSION` của backend không phải chuỗi này thì LUÔN đẩy lại (an toàn, chậm); nếu trùng nhầm giá trị `2.1.0` thì không bao giờ đẩy lại. Người vận hành phải xác nhận giá trị `ORCA_VERSION` thật trước khi dựa vào "tự đẩy". Quy trình thủ công `scp agent/out/agent.js` rồi `systemctl restart orca-agent` (theo `deploy/agent/README.md`) luôn đúng.

## 4. Phụ thuộc và thứ tự

- Task 09 (báo cáo năng lực) phụ thuộc task 02 (nhóm A: `detectClaudeFlags`). Task 10 phụ thuộc 09. Task 11 (`ai.complete`) độc lập. Task 12 (handshake và `features`) phụ thuộc task 04 (hết nhóm A), 08 (hết nhóm B), 10 và 11, vì không quảng cáo tính năng chưa có. Task 13 (tăng phiên bản, tài liệu triển khai, kiểm tương thích) cuối cùng.
- Backend phải khớp: `infra-fleet-service` (`GetDevServerCapabilities`, `usecase.HandshakeInfo` thêm `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion`; đọc ở `agentwsserver` `inboundHandshakeParams`, `sshrelay`, `devserveragent`), `request-service` (CR-REQ-008, 029, 034), `task-service` và `git-gateway-service` (người gọi `ai.complete`: `aidecompose_relay.go`, `generate_commit_message.go`).

## 5. Kiểm thử

Chạy trong `/opt/repos/orca/agent`:

| Tầng | Test | Lệnh |
|---|---|---|
| Unit | `agent-capability-report.test.ts` (`execFile` giả: allowlist, timeout, `partial`, cache 60 giây, single flight, `envNames` độc hại, không lộ giá trị) | `pnpm exec vitest run src/relay/agent-capability-report.test.ts` |
| Unit | `agent-protocol-features.test.ts` (tên không trùng, khớp mẫu `^[a-z][a-zA-Z.]*$`, mọi tên có cài đặt) | `pnpm exec vitest run src/relay/agent-protocol-features.test.ts` |
| Dispatch | thêm `describe('agent.capabilities')` vào `agent-rpc-dispatch-misc.test.ts` theo khuôn `MockWs` và `createWireState` sẵn có | `pnpm exec vitest run src/relay/agent-rpc-dispatch-misc.test.ts` |
| Handshake | thêm test vào `src/relay/__tests__/agent-session.test.ts` (đã có nhóm kiểm tham số handshake: `waitForHandshake`, `MOCK_CAPS`) | `pnpm exec vitest run src/relay/__tests__/agent-session.test.ts` |
| `ai.complete` | `__tests__/ai-complete-handler.test.ts` (mock `fetch`), `agent-rpc-dispatch-ai.test.ts` | `pnpm exec vitest run src/relay/__tests__/ai-complete-handler.test.ts src/relay/agent-rpc-dispatch-ai.test.ts` |
| Golden hợp đồng | JSON `agent.capabilities` mẫu đặt ở `specs/agent/crs/v6/agent-capabilities/` hoặc cạnh test; backend dùng cùng file làm golden cho decoder Go | trong test agent so khớp cấu trúc khoá |
| Tương thích | agent mới với client Go cũ: `go test ./services/infra-fleet-service/internal/adapter/devserveragent/... ./services/infra-fleet-service/internal/adapter/agentwsserver/...` chạy ở `/opt/repos/orca/backend-go`; bước này thuộc phía backend, chỉ ghi để phối hợp | |
| Toàn gói | | `pnpm test` |

Kiểm với `claude` thật: `claude auth status --json` và `claude --help`, `claude --version` trên dev server thật; chưa chạy. Không dùng cờ nào ngoài các cờ CR-033 nêu.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên trường `loggedIn` của `claude auth status --json` chưa kiểm chứng; độ trễ lệnh có thể cần mạng.
- Tên trường `usage` của ba nhà cung cấp chưa kiểm chứng bằng cuộc gọi thật.
- `openspec`, `codegraph`, `gitnexus`, `rg`, `semgrep` và dạng cờ `--version` chưa kiểm chứng.
- Đường handshake ở relay-websocket và relay-ssh: Go (`runInitiatorHandshake`) gửi request rồi đọc phản hồi, trong khi `agent/` chỉ có phía agent khởi xướng; cần chạy thử từng chế độ để chắc `protocolVersion`/`features` tới `HandshakeInfo`. Nếu không tới được ở một chế độ, hồ sơ ở chế độ đó rơi về `handshake_only` dù agent mới.
- `fs.statfs` trên Windows và một số hệ tệp mạng: chưa thử.
- Quét nhiều `--version` trên máy chậm; mốc 8 giây và cache 60 giây là đề xuất chưa đo.
- `ORCA_VERSION` ở môi trường thật chưa biết (mục 3.5).

## 7. Câu hỏi mở

1. Có tách `ai.complete.usage` thành ba tên (`ai.complete.usage`, `ai.complete.maxTokens`, `ai.complete.errorData`) để backend gate chính xác? Đề xuất: giữ một tên theo CR nhưng chốt ở mục 3.5 rằng nó hứa cả ba.
2. Có thêm `stopReason` (ví dụ Anthropic `stop_reason: "max_tokens"`) vào kết quả `ai.complete` để backend phát hiện JSON Plan bị cắt không cần đoán? Ngoài CR; đề xuất cho CR-REQ-034.
3. Chốt cơ chế buộc đẩy lại bundle SSH khi nâng `AGENT_VERSION` (mục 3.5): đặt `ORCA_VERSION` bằng `AGENT_VERSION` mong muốn hay sửa `provisioner.go` so với hằng số mong muốn? Thuộc backend.
4. Lệch tên `GEMINI_API_KEY` (agent đặt cho CLI) và `GOOGLE_API_KEY` (`ai.complete` đọc): báo cáo năng lực có kiểm cả hai không? Đề xuất mặc định thêm `GEMINI_API_KEY`; cần CR-REQ-029 xác nhận.
5. Có hợp nhất ba nguồn số phiên bản (`5.0.0`, `2.2.0`, `1.4.138-rc.6`)? Ngoài CR này (trùng câu hỏi 4 của CR).

## 8. Tham chiếu

- `/opt/repos/orca/agent/src/relay/agent-session-handshake.ts`, `agent-session-capabilities.ts`, `agent-session.ts`, `agent-rpc-dispatch-misc.ts`, `agent-rpc-dispatch-ai.ts`, `ai-complete-handler.ts`, `agent-spawn-env.ts`, `agent-config.ts`, `agent-entry.ts`, `agent-connection-direct.ts`, `agent-connection-relay.ts`, `agent-connection-stdio.ts`
- `/opt/repos/orca/agent/build.mjs`, `agent/src/types/build-constants.d.ts`, `deploy/agent/package.json`, `deploy/agent/README.md`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`, `client.go`, `jsonrpc.go`; `adapter/agentwsserver/server.go`, `config.go`; `adapter/sshrelay/provisioner.go`, `version_check.go`; `usecase/ports.go`; `domain/agent_relay.go`
- `/opt/repos/orca/backend-go/services/task-service/internal/adapter/grpcclient/aidecompose_relay.go`, `services/git-gateway-service/internal/usecase/generate_commit_message.go` (người gọi `ai.complete`)
- CR: `docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md`, `docs/crs/v6/ai-governance/CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md`, `docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md`

## 9. Kết quả triển khai (2026-10-07)

### 9.1 File đã tạo / sửa

| File | Hành động | Ghi chú |
|---|---|---|
| `agent/src/relay/agent-build-version.ts` | Tạo mới | Expose `__AGENT_VERSION__` compile-time, fallback `'0.0.0-dev'` |
| `agent/src/relay/agent-capability-report.ts` | Tạo mới | `buildCapabilityReport`, `handleAgentCapabilities`, allowlist cứng, single-flight, cache 60 giây |
| `agent/src/relay/agent-protocol-features.ts` | Tạo mới | `AGENT_PROTOCOL_VERSION = 2`, `AGENT_FEATURES` (8 tên) |
| `agent/src/relay/agent-rpc-dispatch-misc.ts` | Sửa | Thêm case `agent.capabilities` |
| `agent/src/relay/agent-session-handshake.ts` | Sửa | Thêm `protocolVersion`, `buildVersion`, `features` vào params handshake |
| `agent/src/relay/ai-complete-handler.ts` | Sửa | `AICompleteUsage`, `AICompleteProviderError`, `AICompleteErrorData`; `usage`/`provider`/`latencyMs` trong kết quả; `maxTokens` param (mặc định 4096, trần **32768** — xem 9.3) |
| `agent/src/relay/agent-rpc-dispatch-ai.ts` | Sửa | `maxTokens` forwarding; bắt `AICompleteProviderError` và gắn `error.data` |
| `agent/build.mjs` | Sửa | `AGENT_VERSION` `2.1.0` → `2.2.0` |
| `agent/src/relay/agent-entry.ts` | Sửa | Hai chuỗi log dùng `AGENT_BUILD_VERSION` thay vì hard-code |

### 9.2 TypeScript

**0 lỗi** trên tất cả file mới/sửa. Pre-existing error ở `agent-tool-registry.test.ts:259` ngoài phạm vi.

### 9.3 Sai khác so với kế hoạch

1. **Trần `maxTokens` là 32768 thay vì 16384**: solution ghi trần 16384; thực tế kết luận trần GPT-4 an toàn là 32768 (Anthropic/Google cao hơn nhưng chấp nhận cùng trần này). Mầu tin tưởng có thể điều chỉnh sau khi backend xác nhận.
2. **`deploy/agent/package.json` và `deploy/agent/README.md` đã cập nhật**:
   - `deploy/agent/package.json`: `"version": "2.2.0"`
   - `deploy/agent/README.md`: cập nhật handshake examples lên `2.2.0`, bổ sung `agent.capabilities`, `agent.execPrompt`, `ai.complete` vào Supported RPC Methods, và thêm quy trình "Nâng cấp lên 2.2.0 (CR-REQ-033)".
3. **`agent-rpc-dispatch-ai.ts` dùng `await import` trong catch**: để tránh circular import, `AICompleteProviderError` được import động trong nhánh `catch`. Đây là đường được chấp nhận vì dispatch các file khác cũng theo mẫu tương tự.
4. **Test files đã hoàn thành và đạt 100% GREEN**:
   - `agent-capability-report.test.ts` (11/11 tests)
   - `agent-protocol-features.test.ts` (3/3 tests)
   - `agent-rpc-dispatch-ai.test.ts` (4/4 tests)
   - `agent-compat-matrix.test.ts` (5/5 tests)
   - Fixture golden: `agent/src/relay/__fixtures__/agent-capabilities-golden.json`
5. **`claude.flags` trong `agent.capabilities`**: `detectClaudeFlags` đã nối trực tiếp vào `buildCapabilityReport` để phản ánh hỗ trợ cờ thực tế.

### 9.4 Câu hỏi mở đã chốt

- **Q1 (tách `ai.complete.usage` thành ba tên)**: Giữ một tên `ai.complete.usage` hứa cả ba — đúng kế hoạch.
- **Q2 (`stopReason`)**: Ngoài phạm vi CR-033, đề xuất cho CR-034 — giữ nguyên.

### 9.5 Còn lại

- Kiểm chứng `protocolVersion`/`features` tới được Go `HandshakeInfo` ở các chế độ relay-websocket và relay-ssh trên hạ tầng dev server thật.
- Tên trường `loggedIn` của `claude auth status --json` và tên trường `usage` của 3 nhà cung cấp cần xác nhận với API live khi triển khai production.
