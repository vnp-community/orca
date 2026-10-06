# CR-REQ-033 — Dev Server Agent: chế độ chỉ đọc, vùng làm việc, danh sách file đổi, khối kết quả và báo cáo năng lực

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-033 |
| **Tên** | `agent.execPrompt` có chế độ chỉ đọc, vùng làm việc rõ ràng, danh sách file thay đổi, khối kết quả JSON; RPC `agent.capabilities` và mở rộng handshake; hồ sơ năng lực lưu ở `infra-fleet-service` |
| **Loại** | Feature (thay đổi agent, CR duy nhất của series chạm `agent/`) |
| **Priority** | 🟠 P1 |
| **Effort** | Medium (6 đến 8 ngày: agent 3 đến 4, infra-fleet 2 đến 3, test và triển khai thử) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | Không có CR nào chặn. Cần một dev server thật có `claude` đã đăng nhập để kiểm chứng (mục 5) |
| **Mở khoá** | CR-REQ-008 (chỉ đọc ép được), CR-REQ-013 (đọc kết quả có cấu trúc), CR-REQ-029 `ReadinessGate` (đề xuất, chưa có file trong v6), CR-REQ-034 (usage và lỗi có cấu trúc của `ai.complete`) |
| **Tác động** | `agent/src/relay/` (file mới và sửa nhỏ), `backend-go/services/infra-fleet-service` (proto, usecase, adapter, migration `0039` hai dialect), `request-service` chỉ là người gọi |

## 1. Bối cảnh và vấn đề (đã đọc code ngày 2026-10-06)

1. `agent.execPrompt` (`agent/src/relay/agent-print-mode-exec.ts`) nhận `prompt`, `worktreePath`, `stepId`, `taskId`, `projectId`, `initFile`, `trustPreset`, `model`, `accountId`, `env`, `timeoutMs`. Nó chạy `claude --print <prompt>` với `cwd=worktreePath`, thêm `--dangerously-skip-permissions` khi `trustPreset=full`, trả `{stdout, stderr, exitCode, timedOut, stepId}`. Không có tham số chỉ đọc. `worktreePath` chỉ bị kiểm "không rỗng"; đường dẫn không tồn tại làm `spawn` lỗi `ENOENT` và trả `stderr` với `exitCode=null`.
2. `stdout` được cộng dồn trong bộ nhớ không giới hạn (`stdout += chunk`), trong khi khung truyền tối đa 16 MiB. Kết quả lớn hơn mức đó chưa rõ xử lý ra sao (chưa kiểm chứng).
3. Handshake (`agent-session-handshake.ts`) gửi `agentVersion: '5.0.0'` cố định, `capabilities` từ `buildCapabilities` (`agent-session-capabilities.ts`) chỉ gồm `fs`, `git`, `pty*`, `agent.spawn`, `agent.exec`... Danh sách không có `agent.execPrompt`, `agent.execPromptStream`, `ai.complete`. Số phiên bản có ba nguồn khác nhau: `5.0.0` (handshake), `AGENT_VERSION='2.1.0'` (`agent/build.mjs`, dùng bởi `sshrelay.remoteVersionAndPresence`), `1.4.138-rc.6` (`agent/package.json`). `ORCA_AGENT_MIN_VERSION` của `agentwsserver` so với giá trị `5.0.0` nên chưa chặn được gì.
4. Đã có các RPC dò môi trường rời rạc: `preflight.check` (dịch vụ), `preflight.detectAgents` (danh sách CLI theo lệnh do người gọi gửi), `host.capabilities` (WSL, pwsh, git-bash), `agent.exec` (chạy binary bất kỳ, tối đa 5 phút). Không có RPC nào trả công cụ cài sẵn kèm phiên bản, trạng thái đăng nhập `claude`, hay tài nguyên.
5. Backend: `devserveragent.HandshakeInfo` có `Capabilities []string` nhưng chỉ nằm trong bộ nhớ phiên; bản sao `usecase.HandshakeInfo` (`usecase/ports.go`) bỏ `Capabilities`. `domain.DevServer` chỉ lưu `Platform`, `Arch`, `NodeVersion`, `AgentVersion`. Không có hồ sơ năng lực lưu bền. Phương thức agent chưa có trả `domain.ErrAgentMethodNotFound` (JSON-RPC -32601), mẫu dùng ở `usecase/get_host_capabilities.go`.
6. Claude CLI: `claude --help` trên máy soạn thảo (phiên bản 2.1.289) liệt kê `--permission-mode` (giá trị `acceptEdits, auto, bypassPermissions, manual, dontAsk, plan`), `--tools`, `--allowedTools`, `--disallowedTools`, `--output-format json`, `--max-budget-usd`, `--json-schema` và lệnh `claude auth status [--json]`. Repo đã dùng `--permission-mode plan` ở `frontend/src/shared/commit-message-agent-spec.ts`. **Chỉ đọc help, chưa chạy `--print` với các cờ này.** Hành vi thật (ví dụ `plan` có chặn `Bash` ghi file hay không, tên công cụ `Read`, `Glob`, `Grep`) chưa kiểm chứng.
7. **Hai bản agent.** `agent/src/relay/` (265 file, có `agent-print-mode-exec.ts`, `agent-binary-specs.ts`, `ai-complete-handler.ts`) và `desktop/src/relay/` (154 file, không có `execPrompt` nào). Không có script đồng bộ: `agent/build.mjs` (`package.json` gốc: `build:agent` = `node agent/build.mjs`) build `agent/src/relay/agent-entry.ts` thành `agent/out/agent.js`; `desktop/config/scripts/build-agent-only.mjs` build bản riêng thành `desktop/out/relay/agent.js`. Hai nơi triển khai dev server đều trỏ `agent/out/agent.js`: `deploy/agent/README.md` (`scp agent/out/agent.js`) và `sshrelay.Config.BundlePath` ("agent/out/agent.js"). `agent/package.json` ghi "isolated copy, split from monorepo". Kết luận: `agent/` là nguồn thật cho dev server; `desktop/src/relay` là bản tách riêng đã lệch, **không cần sửa** cho CR này, nhưng bundle build từ `desktop/` sẽ trả -32601 cho mọi method mới (rơi vào đường degradation 2.7).

## 2. Giải pháp đề xuất

### 2.1 Tham số mới của `agent.execPrompt` và `agent.execPromptStream`

Mọi tham số tuỳ chọn; thiếu thì hành vi y hệt hiện nay (tương thích ngược). Logic mới đặt ở file mới để không đẩy `agent-print-mode-exec.ts` (382 dòng) quá `max-lines`; chỉ sửa đúng chỗ dựng `args` và chỗ trả kết quả.

| Tham số | Giá trị | Ý nghĩa |
|---|---|---|
| `accessMode` | `"write"` (mặc định) \| `"readonly"` | `readonly`: thêm cờ hạn chế công cụ (2.2) và **bỏ qua** `trustPreset=full` |
| `workspaceKind` | `"worktree"` (mặc định, không kiểm thêm) \| `"repo_root"` \| `"scratch"` | khai báo `worktreePath` là gì (2.3) |
| `reportChanges` | `true` \| `false` (mặc định) | trả `changes` (2.4) |
| `resultBlock` | `{ "nonce": "<16 đến 64 ký tự [A-Za-z0-9]>" }` | bật phân tích khối kết quả (2.5) |
| `maxOutputBytes` | số nguyên, mặc định 4 MiB, trần 12 MiB | cắt `stdout`/`stderr` giữ phần cuối, trả `truncated` |

### 2.2 Chế độ chỉ đọc (`accessMode=readonly`)

Agent thêm vào `args` (file mới `agent-readonly-tool-policy.ts`): `--permission-mode plan` và `--tools` chỉ gồm công cụ đọc. Danh sách mặc định `Read Glob Grep` (tên công cụ **chưa kiểm chứng**, lấy từ hiểu biết chung về Claude Code, help chỉ nói `--tools <tools...>` là danh sách công cụ khả dụng). Không có `Bash`: chạy lệnh shell đọc (`git log`, `cat`) cũng là đường ghi file (`>`), nên v1 chỉ cho công cụ file. Người gọi cần `git log` thì dùng `fs.*` hoặc `agent.exec` ngoài lần chạy.

Chống hạ cấp âm thầm (fail closed): trước lần chạy đầu và mỗi 10 phút, agent đọc `claude --help` (timeout 5 giây, lưu cache) và ghi cờ `claudeFlags = {tools, permissionMode, disallowedTools}`. Nếu help không chứa `--tools` hoặc `--permission-mode` thì `accessMode=readonly` bị từ chối với lỗi `READONLY_MODE_UNSUPPORTED` (mã JSON-RPC `InvalidParams`, `error.data.reason`), **không** chạy ở chế độ ghi. Việc cờ có thật sự chặn ghi phải kiểm bằng thử nghiệm đối kháng (mục 5): prompt yêu cầu tạo file, sửa file, `git commit`; mong đợi cây làm việc không đổi.

`trustPreset=full` với `accessMode=readonly`: agent bỏ cờ YOLO, ghi cảnh báo, trả `warnings: ["TRUST_PRESET_IGNORED_READONLY"]`.

### 2.3 Vùng làm việc (`workspaceKind`)

Hiện agent không phân biệt worktree, repo gốc hay thư mục tạm. Quy tắc mới (file `agent-workspace-validation.ts`, mới):

| Giá trị | Kiểm tra | Điều kiện đi kèm |
|---|---|---|
| `worktree` | như cũ (không kiểm thêm) | không đổi |
| `repo_root` | đường dẫn tồn tại, là thư mục, nằm trong repo Git (`git rev-parse --show-toplevel` bằng `execFile`, chỉ lệnh cơ bản) | **bắt buộc `accessMode=readonly`**, nếu không `REPO_ROOT_REQUIRES_READONLY` |
| `scratch` | tồn tại, là thư mục, nằm dưới `os.tmpdir()` hoặc `<config.workDir>/.orca-scratch/` (chống thoát đường dẫn bằng `realpath`) | caller tạo thư mục trước bằng `fs.mkdir` (đã có), agent không tự tạo hay xoá |

Lý do `repo_root` buộc chỉ đọc: `spike`, `question`, `diagnosis` (CR-REQ-008) dùng `repo_path` của project và hiện không có gì ngăn ghi vào checkout chính. Không thêm lệnh Git mới ngoài `rev-parse` nên không đổi baseline Git 2.25 (`guides/reference/git-compatibility.md`). Agent chạy trên chính host nên đúng cả với SSH, WSL (AGENTS.md).

### 2.4 Danh sách file thay đổi (`reportChanges=true`)

Agent chụp trạng thái trước và sau lần chạy (file `agent-worktree-change-snapshot.ts`, mới), chỉ khi `cwd` thuộc repo Git:

- Lệnh: `git --no-optional-locks status --porcelain=v1 -z --untracked-files=all` và `git rev-parse HEAD` (đều có từ trước Git 2.25; `--no-optional-locks` từ 2.15; tuân quy tắc đặt tuỳ chọn toàn cục trước subcommand).
- Chữ ký mỗi đường dẫn: `status` + `size` + `mtimeMs` của file đang bẩn. `changedFiles` là các đường dẫn có chữ ký khác trước và sau (kể cả file đã bẩn từ trước bị sửa tiếp).
- Kết quả: `changes: { available: true, headBefore, headAfter, headMoved, changedFiles: [{path, change: "added|modified|deleted|renamed|untracked"}], truncated }`, tối đa 2000 đường dẫn.
- Không phải repo Git: `changes: { available: false, reason: "NOT_A_GIT_REPO" }`; với `scratch`, đi bộ thư mục tối đa 5000 mục theo `size`+`mtimeMs`.
- Giới hạn đã biết: file bị `.gitignore` không thấy; thay đổi ngoài `cwd` (ví dụ `~`) không thấy. Đây là phát hiện, không hoàn tác. Với `readonly` mà `changedFiles` khác rỗng hoặc `headMoved`, agent thêm `warnings: ["READONLY_VIOLATION"]` để backend (CR-REQ-008) loại kết quả.

### 2.5 Khối kết quả JSON ở cuối đầu ra

Quy ước chống giả mạo: backend sinh `nonce` ngẫu nhiên mỗi lần chạy, đưa vào prompt và vào `resultBlock.nonce`. Agent chỉ nhận khối có dạng

```
ORCA_RESULT_BEGIN <nonce>
{ ...một đối tượng JSON... }
ORCA_RESULT_END <nonce>
```

Thuật toán (file `agent-result-block-parser.ts`, mới, hàm thuần dễ test): tìm cặp BEGIN/END **cuối cùng** có đúng `nonce` trong `stdout`; phần giữa phải là một đối tượng JSON (`JSON.parse`, không phải mảng hay chuỗi), tối đa 256 KiB. Nội dung Request chứa sẵn chuỗi `ORCA_RESULT_BEGIN` không có `nonce` đúng thì bị bỏ qua (chống chèn chỉ dẫn qua nội dung Jira, xem CR-REQ-035).

Kết quả: thêm vào `result` của RPC `result: {..., parsed: {ok: true, value} | {ok: false, code, detail}}`, `code` ∈ `RESULT_BLOCK_MISSING`, `RESULT_BLOCK_INVALID_JSON`, `RESULT_BLOCK_TOO_LARGE`, `RESULT_BLOCK_NOT_OBJECT`. Agent **không** kiểm schema: backend kiểm theo `kind`/TaskSpec (CR-REQ-008, 029). Không có `resultBlock` thì không thêm trường `parsed`. Tên trường `parsed` thay cho `result` để khỏi lồng `result.result` trong JSON-RPC.

### 2.6 `agent.capabilities` và mở rộng handshake

**Handshake** (không chậm: vẫn dưới race 5 giây): thêm vào `params`/kết quả các trường `protocolVersion` (số nguyên; `1` khi vắng = hiện tại, `2` = CR này), `buildVersion` (= `AGENT_VERSION`), `features` (mảng chuỗi tĩnh: `agent.execPrompt`, `agent.execPrompt.readonly`, `agent.execPrompt.workspaceKind`, `agent.execPrompt.changes`, `agent.execPrompt.resultBlock`, `agent.capabilities`, `ai.complete`, `ai.complete.usage`). Không đổi `agentVersion: '5.0.0'` (ResumeAgentSession so sánh bằng nhau, `MinAgentVersion` dùng nó) và không đổi `capabilities` hiện có. `features` tách khỏi `capabilities` để các kiểm tra hiện có (`ptyReady`) không bị ảnh hưởng. Backend Go dùng `json.Unmarshal` thường (không `DisallowUnknownFields`, đã grep) nên trường lạ vô hại.

**`agent.capabilities`** (RPC mới, file `agent-capability-report.ts`, mới, đăng ký ở `agent-rpc-dispatch-misc.ts`). Params tuỳ chọn: `{ tools?: string[], envNames?: string[], refresh?: boolean }`. Kết quả:

```json
{ "schemaVersion": 1,
  "agent": {"buildVersion": "2.1.0", "protocolVersion": 2},
  "host": {"platform": "linux", "arch": "x64", "nodeVersion": "v22.x", "cpuCount": 8,
           "memTotalMb": 32000, "memFreeMb": 20000, "diskFreeMb": 120000, "loadAvg1": 0.4},
  "tools": [{"id": "go", "installed": true, "version": "1.22.3"}, {"id": "openspec", "installed": false}],
  "claude": {"installed": true, "version": "2.1.289", "auth": "logged_in",
             "flags": {"tools": true, "permissionMode": true, "disallowedTools": true}},
  "env": [{"name": "ANTHROPIC_API_KEY", "present": true}],
  "probedAt": "2026-10-06T10:00:00Z", "partial": false }
```

Quy tắc an toàn: (a) **danh sách cho phép cứng** trong agent: `go node pnpm npm git openspec claude codegraph gitnexus rg make semgrep`, mỗi cái một lệnh phiên bản cố định (`--version`), không bao giờ chạy lệnh do người gọi gửi; tên ngoài danh sách vào `unknownTools`. (b) `envNames` phải khớp `^[A-Z][A-Z0-9_]{0,63}$`, tối đa 64; chỉ trả `present` boolean, **không bao giờ** trả giá trị. Mặc định kiểm `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_API_KEY`. (c) `claude.auth` lấy từ `claude auth status --json` (lệnh có trong help; tên trường `loggedIn` **chưa kiểm chứng**): chỉ lấy boolean, không chuyển email hay tổ chức; không parse được thì `unknown`. (d) Mỗi lệnh timeout 3 giây, chạy song song, tổng 8 giây; quá thì `partial: true`. (e) cache 60 giây trong tiến trình, `refresh=true` bỏ cache. (f) `diskFreeMb` cho `config.workDir`.

### 2.7 Phía backend: hồ sơ năng lực và suy giảm

`infra-fleet-service` (đã đọc `domain/dev_server.go`, `usecase/get_host_capabilities.go`, `adapter/devserveragent`):

1. Proto `infrafleet.proto`: RPC `GetDevServerCapabilities(GetDevServerCapabilitiesRequest{connection_id | dev_server_id, bool refresh}) returns (DevServerCapabilityProfile)`. Message có `profile_json`, `source` (`probe|handshake_only`), `agent_build_version`, `protocol_version`, `features[]`, `probed_at`, `degraded`.
2. Bảng `infra.dev_server_capability_profiles` (migration `0039`, Postgres và MySQL; số kế tiếp sau `0038_session_origin`): `dev_server_id` PK, `tenant_id` (RLS `tenant_isolation` như các bảng `infra.*`), `source`, `agent_build_version`, `protocol_version INT`, `features JSON/JSONB`, `profile JSON/JSONB`, `fingerprint CHAR(64)`, `probed_at`, `updated_at`. Không FK chéo service.
3. Use case `RefreshDevServerCapabilities`: gọi `agent.Exec(devServer, "agent.capabilities", params)`. Thành công thì upsert; `fingerprint` đổi thì ghi outbox `orca.infra.dev_server.capabilities_changed`. `ErrAgentMethodNotFound` thì dựng hồ sơ `source=handshake_only` từ `HandshakeInfo` (platform, arch, nodeVersion, agentVersion, `capabilities`), `degraded=true`, `tools`/`claude`/`env` để `unknown`. Cần mở rộng bản sao `usecase.HandshakeInfo` thêm `Capabilities`, `Features`, `ProtocolVersion`, `BuildVersion` và đọc ở `agentwsserver` (`inboundHandshakeParams`), `sshrelay` (struct handshake) và `devserveragent` (relay-websocket).
4. Kích hoạt: sau khi `attachTransport` thành công (không chặn, tối đa 1 lần mỗi 5 phút mỗi dev server), và theo yêu cầu qua RPC. Cũ hơn 24 giờ (cấu hình `INFRA_CAPABILITY_PROFILE_TTL`) thì `GetDevServerCapabilities` tự làm mới khi đang kết nối.
5. **Người dùng ở phía `request-service`:** CR-REQ-029 `ReadinessGate` đọc hồ sơ (`requires.tools`, `requires.env_names`, `claude.auth=logged_in`); CR-REQ-008 chọn đường chạy theo `features` (bảng dưới).

| Tình huống dev server | Hành vi |
|---|---|
| `features` có `agent.execPrompt.readonly` | `agent_readonly` truyền `accessMode=readonly`, `workspaceKind=repo_root`, `reportChanges=true`; ba lớp bảo vệ của CR-REQ-008 vẫn chạy (phòng thủ nhiều lớp) |
| Agent cũ (không có `features`) | Hành vi v1 của CR-REQ-008 (prompt + kiểm repo trước/sau qua git-gateway), run ghi `enforcement=prompt_only`; cờ `REQUEST_REQUIRE_ENFORCED_READONLY=true` thì từ chối `REQUEST_ANALYSIS_AGENT_TOO_OLD` |
| Agent cũ, `agent.capabilities` -32601 | Hồ sơ `handshake_only`; `ReadinessGate` coi công cụ là `unverified` và có thể dùng `agent.exec` `command -v` làm phương án thay (quyết định ở CR-REQ-029) |
| Bundle build từ `desktop/` | Như agent cũ |

Triển khai agent mới: build `agent/out/agent.js`, `scp` rồi `systemctl restart orca-agent` theo `deploy/agent/README.md`; với `relay-ssh`, `sshrelay` tự đẩy khi `AGENT_VERSION` đổi, nên **phải tăng `AGENT_VERSION` trong `agent/build.mjs`** (đề xuất `2.2.0`) cùng `deploy/agent/package.json`. Thứ tự: agent trước, backend sau (backend gọi method mới chỉ khi `features` có), nên không có cửa sổ lỗi.

### 2.8 Mở rộng nhỏ của `ai.complete` (CR-REQ-034 cần)

Để ngân sách và dự phòng model (CR-REQ-034) không phải đoán, `handleAIComplete` (`ai-complete-handler.ts`) thêm, tương thích ngược: (a) kết quả `usage: {inputTokens, outputTokens}` đọc từ phản hồi nhà cung cấp (trường `usage` của Anthropic/OpenAI; Google `usageMetadata`; tên trường **chưa kiểm chứng**) và `provider`, `latencyMs`; (b) tham số `maxTokens` (mặc định giữ 4096, trần 16384): hiện `max_tokens: 4096` cố định có thể cắt cụt JSON Plan dài; (c) lỗi có `error.data: {provider, httpStatus, retryable}` thay vì chỉ thông điệp chuỗi `Anthropic API error 429: ...`. Tham số `model` đã có sẵn ở agent, chỉ phía Go chưa gửi.

### 2.9 Tệp sẽ tạo hoặc sửa

`agent/src/relay/` (mới): `agent-readonly-tool-policy.ts`, `agent-workspace-validation.ts`, `agent-worktree-change-snapshot.ts`, `agent-result-block-parser.ts`, `agent-capability-report.ts`, kèm `*.test.ts`. Sửa: `agent-print-mode-exec.ts` (gọi các hàm mới, trả thêm trường), `agent-rpc-dispatch-misc.ts` (đăng ký `agent.capabilities`), `agent-session-handshake.ts` (thêm trường), `ai-complete-handler.ts` (2.8), `agent/build.mjs` và `deploy/agent/package.json` (tăng phiên bản). `infra-fleet-service`: `proto/orca/infrafleet/v1/infrafleet.proto`, `internal/usecase/refresh_dev_server_capabilities.go`, `get_dev_server_capabilities.go`, `internal/adapter/{postgres,mysql}/capability_profile_repository.go`, migration `0039_dev_server_capability_profiles.{up,down}.sql` (hai dialect), sửa `usecase/ports.go`, `agentwsserver/server.go`, `sshrelay/provisioner.go`, `devserveragent/session.go`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| 1 | Chỉ đọc là cờ hạn chế công cụ của chính `claude`, không tự viết sandbox | Đúng nguồn kiểm soát quyền; chi phí thấp. Không tuyệt đối nên giữ kiểm tra trước/sau |
| 2 | Từ chối thay vì hạ cấp khi CLI không hỗ trợ cờ | Hạ cấp âm thầm biến "chỉ đọc" thành lời hứa suông |
| 3 | Không cho `Bash` trong chỉ đọc v1 | Lệnh shell có thể ghi file; muốn đọc lịch sử Git dùng đường khác |
| 4 | `repo_root` bắt buộc `readonly` | Ghi vào checkout chính là rủi ro lớn nhất của `spike`/`question` |
| 5 | Khối kết quả dùng `nonce` do backend sinh | Nội dung Jira/GitHub không tin cậy có thể chèn khối giả |
| 6 | Agent không kiểm schema kết quả | Schema theo `kind`/TaskSpec đổi thường xuyên; tránh triển khai lại agent |
| 7 | `agent.capabilities` dùng danh sách cho phép cứng | Tránh biến RPC dò thành đường chạy lệnh tuỳ ý |
| 8 | Chỉ trả `present` cho biến môi trường | Không đưa bí mật qua RPC, đúng nguyên tắc log của `ai-complete-handler.ts` |
| 9 | Không đổi `agentVersion` và `capabilities`; thêm `protocolVersion`, `features` | `ResumeAgentSession`, `MinAgentVersion`, `ptyReady` phụ thuộc giá trị cũ |
| 10 | Không sửa `desktop/src/relay` | Bản đã lệch, không có `execPrompt`; dev server dùng `agent/out/agent.js` |

## 4. Tiêu chí chấp nhận

- [ ] `agent.execPrompt` không tham số mới cho kết quả giống hệt trước (test hồi quy với `agent-print-mode-exec.test.ts` hiện có).
- [ ] `accessMode=readonly` thêm `--permission-mode plan` và `--tools ...` vào `args` (test với `spawn` giả); không bao giờ thêm cờ YOLO.
- [ ] CLI giả không có `--tools` thì `READONLY_MODE_UNSUPPORTED`, không có tiến trình con nào được spawn.
- [ ] `repo_root` kèm `accessMode=write` bị `REPO_ROOT_REQUIRES_READONLY`; `scratch` ngoài `os.tmpdir()` bị từ chối, kể cả khi đi vòng bằng liên kết tượng trưng.
- [ ] `reportChanges` trả đúng `added/modified/deleted/untracked`, `headMoved` khi có commit, và `available:false` ngoài repo Git.
- [ ] Khối kết quả: nonce đúng được phân tích; nonce sai, thiếu, JSON hỏng, quá 256 KiB trả đúng mã; chuỗi giả trong `stdout` không có nonce đúng bị bỏ qua.
- [ ] `stdout` vượt `maxOutputBytes` bị cắt, `truncated=true`, phản hồi vẫn dưới 16 MiB.
- [ ] `agent.capabilities` không chạy lệnh ngoài danh sách; `envNames` chứa `x; rm -rf` bị bỏ; không có giá trị biến nào trong phản hồi.
- [ ] Handshake có `protocolVersion`, `buildVersion`, `features`; backend Go cũ vẫn bắt tay thành công.
- [ ] `GetDevServerCapabilities` với agent cũ trả `source=handshake_only`, `degraded=true`; với agent mới lưu hồ sơ và phát sự kiện khi `fingerprint` đổi.
- [ ] Migration `0039` lên và xuống sạch trên Postgres 14+ và MySQL 8.0.1+.

## 5. Kiểm thử

- **Unit (agent, vitest):** `agent-result-block-parser` (bảng đúng/sai, nonce, lồng, Unicode tiếng Việt), `agent-readonly-tool-policy` (đọc help giả), `agent-workspace-validation` (symlink, đường dẫn tương đối, Windows), `agent-worktree-change-snapshot` (repo tạm thật bằng `git init`), `agent-capability-report` (allowlist, timeout, `partial`). Lệnh: `pnpm --filter orca-agent test` hoặc `cd agent && pnpm test`.
- **Unit (Go):** `refresh_dev_server_capabilities_test.go` (thành công, -32601, agent trả lỗi, `fingerprint` không đổi thì không phát sự kiện), repository hai dialect: `go test ./services/infra-fleet-service/...`.
- **Hợp đồng:** `buf breaking` cho `infrafleet.proto`; test golden JSON của `agent.capabilities` dùng chung giữa agent (TypeScript) và decoder Go.
- **Thử nghiệm đối kháng trên dev server thật (chưa chạy, bắt buộc trước khi bật `REQUEST_REQUIRE_ENFORCED_READONLY`):** repo mẫu, prompt yêu cầu tạo file, sửa file, `git commit`, ghi qua đường dẫn tuyệt đối; kiểm cây làm việc không đổi. Thêm một thử nghiệm chạy `claude --print` với `--output-format json` và `--max-budget-usd` để biết đầu ra có `usage`, `total_cost_usd` hay không (phục vụ CR-REQ-034).
- **Triển khai thử:** dev server chạy agent cũ cạnh agent mới, xác nhận đường suy giảm.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên công cụ `Read Glob Grep` và hành vi `--permission-mode plan` với `--print` chưa kiểm chứng; nếu sai, chế độ chỉ đọc từ chối chạy (an toàn nhưng làm CR-REQ-008 phải dùng đường v1).
- Cờ CLI đổi theo phiên bản `claude`; `claudeFlags` đọc từ help giảm rủi ro nhưng không loại bỏ.
- `claude auth status --json` có thể cần mạng hoặc in thông tin định danh; chỉ đọc boolean, nhưng độ trễ chưa đo.
- Probing `--version` của nhiều công cụ tốn vài giây trên máy chậm; cache 60 giây và `partial` là biện pháp.
- Hai số phiên bản (5.0.0, 2.1.0) vẫn tồn tại; CR này không dọn, chỉ thêm `buildVersion`. Dọn thuộc CR riêng.
- Bundle `desktop/` nếu ai đó triển khai nhầm sẽ thiếu mọi method; đã có đường suy giảm nhưng không có cảnh báo chủ động.
- `reportChanges` bỏ sót file bị ignore và thay đổi ngoài `cwd`.

## 7. Câu hỏi mở

1. Có chấp nhận cấm `Bash` hoàn toàn ở chế độ chỉ đọc v1, hay cho một danh sách lệnh đọc (`Bash(git log *)`, `Bash(git diff *)` qua `--allowedTools`) sau khi thử nghiệm đối kháng?
2. `agent.capabilities` có nên lấy thêm `codegraph`/`gitnexus` index sẵn sàng (có thư mục `.codegraph/`) hay để CR khác?
3. TTL hồ sơ 24 giờ và tần suất làm mới 5 phút là đề xuất, chưa đo.
4. Có hợp nhất ba nguồn số phiên bản agent trong một CR dọn dẹp không (ảnh hưởng `MinAgentVersion`)?
5. Hỗ trợ model nội bộ (Ollama, vLLM) cho `ai.complete` và `execPrompt` cần CR agent riêng; CR-REQ-034 chỉ nêu giới hạn.

## 8. Tác động tới CR hiện có (không sửa trong lần này)

| CR | Cần sửa gì |
|---|---|
| CR-REQ-008 | Mục 2.2: thêm đường chọn theo `features` (bảng 2.7), truyền `accessMode`, `workspaceKind=repo_root`, `reportChanges`, `resultBlock.nonce`; mục 2.3 lớp 3 "dài hạn" thành lớp 1 khi có tính năng; thêm `enforcement` vào `analysis_runs`; mã lỗi `REQUEST_ANALYSIS_AGENT_TOO_OLD`; bỏ nhận định "agent.execPrompt không có tham số chỉ đọc" khi agent mới |
| CR-REQ-007 | Mục 1.3/2.1: `ai.complete` có `usage`, `maxTokens`; mục 6 rủi ro "chưa có hạn mức" trỏ CR-REQ-034 |
| CR-REQ-012 | Gửi `maxTokens` cho Plan dài (4096 mặc định có thể cắt JSON) |
| CR-REQ-013 | Dùng `resultBlock` và `reportChanges` cho task thực thi (kết hợp CR-REQ-029) |
| CR-REQ-005 | Đọc `GetDevServerCapabilities` để báo sớm "thiếu khoá API" thay vì `REQUEST_AI_NO_PROVIDER` mơ hồ |
| README v6 mục 8 | Thêm dòng: agent có ba nguồn phiên bản; `agent/` là nguồn thật, `desktop/src/relay` đã lệch |
| CR-REQ-025 | Thêm bước kiểm tra phiên bản agent trước khi bật cờ; runbook nâng cấp agent |

## 9. Tham chiếu

- `/opt/repos/orca/agent/src/relay/agent-print-mode-exec.ts`, `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch-misc.ts`, `agent-rpc-dispatch-fs.ts`, `agent-spawner.ts`, `agent-binary-specs.ts`, `agent-session-capabilities.ts`, `agent-session-handshake.ts`, `agent-spawn-env.ts`, `agent-preflight-handler.ts`, `ai-complete-handler.ts`, `agent-entry.ts`
- `/opt/repos/orca/agent/src/shared/agent-wire-protocol.ts`, `tui-agent-permissions.ts`; `/opt/repos/orca/agent/build.mjs`, `package.json`; `/opt/repos/orca/deploy/agent/README.md`, `package.json`
- `/opt/repos/orca/desktop/config/scripts/build-agent-only.mjs`, `desktop/src/relay/` (bản lệch)
- `/opt/repos/orca/frontend/src/shared/commit-message-agent-spec.ts` (tiền lệ `--permission-mode plan`)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go`, `client.go`; `adapter/agentwsserver/server.go`, `config.go`; `adapter/sshrelay/provisioner.go`, `config.go`, `version_check.go`; `usecase/get_host_capabilities.go`, `ports.go`; `domain/dev_server.go`, `agent_relay.go`
- `/opt/repos/orca/guides/reference/git-compatibility.md`; `/opt/repos/orca/docs/research/receive-request/ai-steps-and-dev-server-connection-flows.md`, `openspec-and-ai-tooling-integration.md` (mục 2.3), `artifact-formats-ontology-and-execution-readiness.md` (mục 10, 11)
