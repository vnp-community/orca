# Đánh giá `agent/` Code vs Thiết kế — PTY / AI Agent CLI Integration

**Ngày:** 2026-08-08
**Phạm vi:** `agent/src/**` (PTY spawn, profile-aware agent execution, agent hooks) đối chiếu với:
- `docs/features/F04-ai-agent-support.md`
- `docs/logic/profile/BL-PRF-01-profile-crud.md`, `BL-PRF-02-profile-inheritance.md`, `BL-PRF-04-profile-aware-agent-execution.md`
- `docs/flows/logic/profile.md`, `docs/flows/code/profile/profile-resolution.md`
- Mục "AI Agent CLIs" (§11), "Execution-Host Unification" (§15) trong `docs/hld/dev-server-architecture.md`

**Phương pháp:** Đọc trực tiếp source (GitNexus không có `.codegraph`/`.gitnexus` index sẵn sàng cho phần này nên dùng đọc file + grep có mục tiêu), trích dẫn `file:line`, không suy đoán. Đối chiếu riêng cho package `agent/` — không lẫn với `backend/`/`desktop/` (đã có audit riêng tại `audit/backend/backend-vs-design-review.md`, kết luận `ProfileAwareAgentSpawner` thật nằm ở `backend/src/main/project/`).

---

## 1. Tổng kết mức độ khớp

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| F04 — Danh sách 30+ AI agent | ✅ Khớp phần lớn | `TuiAgent` union (`agent/src/shared/types.ts:2362-2396`) có 32 agent, đặt tên hơi khác doc (`mimo-code`, `command-code`, `kimi`, `qwen-code`, `hermes`...); nhưng đây là roster cho **terminal-injection launch**, không phải cho luồng profile-aware spawn |
| BL-PRF-04 — `ProfileAwareAgentSpawner` | ❌ Không tồn tại trong `agent/` | Chính comment trong code xác nhận: đây là "Orca Server tier" (backend), khác `SubAgentSpawner` (`agent/src/relay/agent-spawner.ts:1-5`) |
| BL-PRF-04 — Agent Binary Resolution (`resolveAgentBinary`) | ⚠️ Một phần, phạm vi hẹp hơn nhiều | Tương đương thật là `resolveAgentSpec()` (`agent/src/relay/agent-spawner.ts:153-161`) nhưng chỉ map **5 model** (claude/codex/gemini/opencode/ollama) — không phải 30+ agent như F04 công bố |
| BL-PRF-04 — Profile hierarchy điều khiển env spawn | ❌ Không có trong `agent/` | `OrcaProfile`/`ResolvedProfile` (`agent/src/main/profile/OrcaProfile.ts`) là **type-only, dead code** — không có `deepMergeProfiles`/`ProfileResolver`/`ProfileCache` nào trong `agent/`; `buildAgentEnv()` không đọc field nào của `OrcaProfile` |
| BL-PRF-04 — `PATH` extension từ `pathAdditions`, `ANTHROPIC_MODEL`, trust preset args | ❌ Không có trong `agent/spawner` | `buildAgentEnv()` không set `PATH` mở rộng, không set `ANTHROPIC_MODEL`; field `trustPreset` có khai báo trong `AgentEnvRequest` nhưng **không được đọc** trong thân hàm (`agent/src/relay/agent-spawner.ts:183-256`) |
| BL-PRF-04 — `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user | ✅ Có, đúng ý tưởng | `agent/src/relay/agent-spawner.ts:221-222` — nhưng build path cứng theo `process.env.HOME`, không nhận từ profile |
| BL-PRF-04 — `relay.call('pty.spawn', {cmd: agentBinary, ...})` | ❌ Sai mô hình | `pty.spawn` thật (`agent/src/relay/pty-handler.ts:513,709`) spawn **shell**, không spawn agent binary trực tiếp; agent CLI được inject như lệnh gõ vào shell (`commandDelivery`/`startupCommand`). Model "spawn agent binary trực tiếp" khớp với `agent.spawn` (khác RPC method) |
| BL-PRF-04 — `initFile`/system preamble | ❌ Không tìm thấy | Không có `initFile`, `systemPreamble`, `buildProjectContext` nào trong `agent/src` |
| HLD §11 — `node-pty.spawn(agentBinary, args, {cwd, env})` | ✅ Có thật | `agent/src/relay/agent-spawner.ts:387-392`, nhưng qua RPC `agent.spawn` (không phải `pty.spawn`) |
| HLD §11.2 — Supported AI Agent CLIs (bảng 6 dòng) | ⚠️ Một phần | Đúng cho claude/codex/gemini/opencode/ollama; "Custom binary" (profile-defined) — **không tìm thấy nhánh xử lý binary tùy ý** trong `resolveAgentSpec()` (trả `undefined` → lỗi `Unknown model`) |
| HLD §11.3 — State machine `idle→running→waiting_for_input→completed/error` | ❌ Không khớp | 3 state machine khác nhau tồn tại trong `agent/`, không cái nào khớp: `AgentLifecycleState` 6-state, `AgentStatusState` 4-state, `AgentStatus` 3-state (chi tiết mục 2.5) |
| `agent.exec` (TG-001, dùng bởi backend `ProfileAwareAgentSpawner`) | ❌ Trùng lặp/dead code nguy hiểm | 2 implementation cùng tên khái niệm: bản **sống** là inline case trong `agent-rpc-dispatch.ts:594-624` (passthrough env đúng); bản **chết hoàn toàn** là `handleAgentExec()` trong `agent-exec-handler.ts:355-451` — không hề được import/dispatch ở đâu, và bản chết này bỏ qua toàn bộ `env`/`extraEnv` từ caller |
| Agent Hooks — POST `http://127.0.0.1:<PORT>/hook/<agent>` + token | ✅ Khớp | `agent-hook-server.ts:1271` (main), `agent-hook-server.ts:300` (relay) check `x-orca-agent-hook-token`; `ORCA_AGENT_HOOK_PORT/TOKEN/ENV/VERSION` set tại `agent-hook-server.ts:271-285` (relay `buildPtyEnv()`) |
| Agent Hooks — mỗi agent có `HookService` riêng (`install`/`installRemote`/`remove`) | ❌ Không nằm trong `agent/` | `hooks.ts`, `kimi/hook-service.ts`, `droid/hook-service.ts`... chỉ tồn tại ở `backend/src/main/`, `desktop/src/main/`, `frontend/src/main/` — **không có bản nào trong `agent/src/main/`** |
| `docs/hld/...` §15 Provider Registry (`DevServerPtyProvider`...) | N/A cho `agent/` | Các class này (`dev-server-pty-provider.ts` etc.) nằm ở `backend/src/main/providers/` và `desktop/src/main/providers/`, **không tồn tại trong `agent/src/main/providers/`** — thư mục cùng tên trong `agent/` chứa nội dung khác hẳn (Windows foreground-process detection) |
| `agent/src/main/codex-cli/command.ts` | ⚠️ Tên module gây hiểu nhầm | Không phải logic riêng cho Codex — là bộ resolver PATH/version-manager dùng chung cho cả `claude` và `codex` (`resolveClaudeCommand`, `resolveCodexCommand`, `resolveCliCommand`) |

---

## 2. Chi tiết theo mục

### 2.1 F04 — Danh sách AI Agent CLI được hỗ trợ

- Type `TuiAgent` (`agent/src/shared/types.ts:2362-2396`) liệt kê đúng **32 giá trị**: `claude`, `claude-agent-teams`, `openclaude`, `codex`, `autohand`, `opencode`, `mimo-code`, `pi`, `omp`, `gemini`, `antigravity`, `aider`, `goose`, `amp`, `kilo`, `kiro`, `crush`, `aug`, `cline`, `codebuff`, `command-code`, `continue`, `cursor`, `droid`, `kimi`, `mistral-vibe`, `qwen-code`, `rovo`, `hermes`, `openclaw`, `copilot`, `grok`, `devin`, `ante`.
- Đối chiếu với bảng F04 (`docs/features/F04-ai-agent-support.md:31-56`): tất cả tên trong doc đều có tương đương trong code (Kimi Code→`kimi`, MiMo Code→`mimo-code`, Command Code→`command-code`, Qwen Code→`qwen-code`, Hermes Agent→`hermes`). Code có thêm 12 agent không nêu tên cụ thể trong doc (`claude-agent-teams`, `autohand`, `omp`, `aider`, `kilo`, `crush`, `aug`, `codebuff`, `mistral-vibe`, `rovo`, `openclaw`, `ante`) — khớp tinh thần với dòng "+ 10 agent khác — Generic CLI" của doc, dù số lượng thực tế lệch (12 vs "10").
- Mỗi agent trong `TUI_AGENT_CONFIG` (`agent/src/shared/tui-agent-config.ts:73-…`) khai báo `detectCmd`, `launchCmd`, `expectedProcess`, `promptInjectionMode` — cơ chế launch là **gõ lệnh vào shell PTY đã spawn sẵn**, không phải spawn agent binary làm process gốc của PTY.
- **Kết luận quan trọng:** roster 30+ agent này chỉ áp dụng cho luồng "mở terminal, tự động gõ lệnh CLI" (F02/F04 UI thường). Nó **không phải** roster mà BL-PRF-04's `resolveAgentBinary()`/agent-spawner dùng — luồng đó chỉ hỗ trợ 5 agent (mục 2.2).

### 2.2 BL-PRF-04 — Agent Binary Resolution

- Doc mô tả hàm thuần `resolveAgentBinary(model)` map `claude-opus-4-5|claude-sonnet→claude, codex→codex, gemini→gemini, opencode→opencode` (`BL-PRF-04:88-98`).
- Tương đương thật trong `agent/`: `resolveAgentSpec()` (`agent/src/relay/agent-spawner.ts:153-161`), dùng prefix-matching qua `MODEL_PREFIX_MAP` (`agent/src/relay/agent-spawner.ts:144-151`): `claude→spec0, gpt-/codex→spec1, gemini→spec2, opencode→spec3, ollama→spec4`. Về ý tưởng khớp (bao gồm cả fallback tới `claude` không đúng — code trả `undefined` cho model lạ, dẫn tới lỗi `Unknown model` ở `agent-spawner.ts:317-327`, khác doc: `AGENT_MAP[model] ?? 'claude'` — **doc có fallback 'claude', code thì báo lỗi thẳng**, đây là sai lệch hành vi thật).
- **Không tìm thấy** nhánh "Custom binary" như HLD §11.2 liệt kê (`Custom binary | <path> | profile-defined`). `resolveAgentSpec()` chỉ có 5 spec cứng (`AGENT_SPECS`, `agent-spawner.ts:119-142`) — không đọc profile để build spec động.
- `buildAgentArgs(trustPreset)` (doc) không có tương đương chạy thật: `resolveAgentSpec().buildArgs()` build args cố định theo model (vd. claude: `['--output-format', 'stream-json', '--verbose']` hoặc `['--resume', id]` — `agent-spawner.ts:120-127`), **không nhận `trustPreset`** để build `--trust`/`--dangerously-skip-permissions` như doc mô tả.

### 2.3 BL-PRF-04 — Profile injection vào Agent Environment

- `agent/src/main/profile/OrcaProfile.ts` định nghĩa đầy đủ `OrcaProfile`/`ResolvedProfile`/`AgentProfileSection`/`ShellProfileSection`/`SecurityProfileSection` (dòng 10-73) đúng shape 3-tầng company/dept/user như BL-PRF-02.
- Nhưng đây là **dead code trong phạm vi `agent/`**: `grep` toàn `agent/src` chỉ thấy `OrcaProfile.ts` tự tham chiếu chính nó và 1 type-only import ở `agent/src/shared/project-types.ts:50` (`resolvedProfile: import('../main/profile/OrcaProfile').ResolvedProfile`) — không một hàm runtime nào trong `agent/` đọc `profile.shell.envVars`, `profile.shell.pathAdditions`, hay `profile.agent.preferredModel`.
- Không có `ProfileResolver`, `ProfileCache`, hay `deepMergeProfiles` nào trong `agent/src` (đã grep xác nhận `0` kết quả) — các class này chỉ tồn tại ở `backend/src/main/profile/ProfileResolver.ts` và `desktop/src/main/profile/ProfileResolver.ts`. Việc merge 3 tầng — nếu có xảy ra — **xảy ra hoàn toàn ngoài package `agent/`**.
- `buildAgentEnv()` (`agent/src/relay/agent-spawner.ts:194-257`) build env từ: `HOME`, `PATH=config.toolPath`, `TERM`, `ORCA_AGENT_CWD/ACCOUNT_ID/TASK_ID/USER_ID/PROJECT_ID`, `GH_CONFIG_DIR`, `GLAB_CONFIG_DIR`, API key (`ANTHROPIC_API_KEY`/`OPENAI_API_KEY`/`GEMINI_API_KEY` — tất cả set cùng lúc nếu có `resolvedApiKey`, dòng 227-230), rồi cuối cùng spread `extraEnv` opaque từ caller (dòng 255-256). Không có logic nào tương ứng "PATH extension từ `pathAdditions`" hay "`ANTHROPIC_MODEL` env var từ `preferredModel`" như tiêu chí chấp nhận của BL-PRF-04 (`BL-PRF-04.md:161-163`) yêu cầu.
- **Kết luận:** trong ranh giới `agent/`, "profile-aware" chỉ còn lại đúng phần GH/GLAB config-dir isolation theo `userId` — phần còn lại (PATH, ANTHROPIC_MODEL, envVars merge, trust preset args) hoàn toàn không hiện diện. Nếu các giá trị này thực sự được inject, chúng phải đến từ backend qua `extraEnv`/`params.env` passthrough — nghĩa là `agent/` **tin tưởng mù quáng** vào caller thay vì tự áp dụng chính sách profile.

### 2.4 `agent.exec` — hai implementation, một cái chết hoàn toàn

- RPC dispatcher (`agent/src/relay/agent-rpc-dispatch.ts:594-624`, case `'agent.exec'`) là bản **đang chạy thật**: nhận `binary`/`args`/`cwd`/`env` trực tiếp từ params, `spawnEnv = { ...process.env, ...extraEnv }` (dòng 623) — đây là generic passthrough executor, khớp với vai trò "backend đã resolve xong, agent chỉ chạy hộ" (comment tại `agent-exec-handler.ts:307-311` xác nhận: *"Called by: StepExecutors.executeAgent()... ProfileAwareAgentSpawner via relay.call('agent.exec', ...)"*).
- Nhưng ngay trong file `agent/src/relay/agent-exec-handler.ts`, có một hàm khác cùng mục đích tên `handleAgentExec()` (dòng 355-451) — tự parse `prompt`/`model`/`trustPreset`, tự `resolveAgentSpec()`, tự build `toolEnv` riêng (chỉ có `HOME/PATH/TERM/NO_COLOR/ORCA_TASK_ID/ORCA_WORKTREE_PATH`, **không đọc `params.env`/`extraEnv` ở đâu cả** — dòng 379-388) và tự spawn bằng `node:child_process.spawn`.
- Grep xác nhận `handleAgentExec` **không được import ở bất kỳ đâu ngoài chính file nó và test riêng** (`agent-exec-handler-test-harness.ts`) — không có `case` nào trong `agent-rpc-dispatch.ts` gọi tới nó. Đây là dead code thật sự, nhưng nó nằm cạnh comment "TG-001: Non-interactive subprocess execution (for task graph steps)" và docblock nói nó là con đường mà `ProfileAwareAgentSpawner` gọi — **gây hiểu nhầm nghiêm trọng cho người đọc code** vì code path thật (dispatcher inline case) không có docblock tương đương, còn code path có docblock đầy đủ thì lại chết.

### 2.5 State machine — 3 mô hình khác nhau, không cái nào khớp HLD §11.3

HLD mô tả: `IDLE → RUNNING → WAITING_FOR_INPUT ⇄ RUNNING → COMPLETED` (hoặc `ERROR`), dựa trên OSC 133 (`docs/hld/dev-server-architecture.md:379-412`). Trong `agent/src` có 3 state machine riêng biệt, không cái nào khớp:

| State machine | Nơi định nghĩa | Các state |
|---|---|---|
| `AgentLifecycleState` | `agent/src/relay/agent-spawner.ts:46` | `idle, spawning, running, stopping, stopped, error` (6 state — quản lý vòng đời PTY process của `SubAgentSpawner`, transition rules tại dòng 97-110) |
| `AgentStatusState` | `agent/src/shared/agent-status-types.ts:18` | `working, blocked, waiting, done` (4 state — dùng bởi `AgentHookServer` để track UI status từ hook events) |
| `AgentStatus` | `agent/src/shared/agent-title-core.ts:12` | `working, permission, idle` (3 state — parse từ OSC/terminal title, fallback khi không có hook) |

Không state machine nào có `WAITING_FOR_INPUT` tách biệt khỏi `RUNNING` theo đúng nghĩa HLD mô tả (state gần nhất là `waiting`/`blocked`/`permission` tùy machine). Không có state `COMPLETED` — machine gần nhất dùng `done`/`stopped`.

### 2.6 PTY spawn — `pty.spawn` không phải là nơi agent binary được launch trực tiếp

- BL-PRF-04 Step 7 viết: `relay.call('pty.spawn', { cmd: resolveAgentBinary(...), args: buildAgentArgs(...), cwd, env, initFile })` → `Dev Server: node-pty.spawn(cmd, {env, cwd})` (`BL-PRF-04.md:71-79`).
- RPC `pty.spawn` thật (`agent/src/relay/pty-handler.ts:513` đăng ký handler, `spawn()` tại dòng ~650-710) luôn spawn **shell** (`resolveDefaultShell()`/`shellOverride`/`env.SHELL`, dòng 668-669), *không* spawn `agentBinary` trực tiếp: `pty.spawn(shell, shellLaunch.args, {...})` (`pty-handler.ts:709`). Agent CLI (nếu có) được đưa vào qua `params.command` + cơ chế `commandDelivery: 'renderer'|'provider'` (dòng 682-684) — tức gõ/paste lệnh vào shell đã chạy, khớp với launch model của `TUI_AGENT_CONFIG` (mục 2.1), không phải "spawn thẳng binary agent" của doc.
- RPC method thật sự spawn agent binary trực tiếp bằng `node-pty.spawn(spec.binary, args, {cwd, env})` là **`agent.spawn`** (khác `pty.spawn`) — `agent/src/relay/agent-spawner.ts:387-392`, đăng ký tại `agent-rpc-dispatch.ts:554` (`case 'agent.spawn'`). Không có `initFile`/preamble injection nào ở đây.
- **Kết luận:** doc dùng nhầm tên RPC method (`pty.spawn` thay vì `agent.spawn`) và mô tả sai model I/O (`cmd` = agent binary trực tiếp thay vì shell + inject command).

### 2.7 Agent Hooks — khớp tốt ở phần "server nhận hook", lệch ở phần "cài hook"

- Managed script POST → `http://127.0.0.1:<ORCA_AGENT_HOOK_PORT>/hook/<agent>` + header `X-Orca-Agent-Hook-Token` — ✅ khớp đúng, xác nhận tại `agent/src/main/agent-hooks/server.ts:1271` (main-process HTTP server check token header) và `agent/src/relay/agent-hook-server.ts:300` (relay-side HTTP server, cùng cơ chế cho Dev Server/SSH remote host). Env vars `ORCA_AGENT_HOOK_PORT/TOKEN/ENV/VERSION` set tại `agent/src/relay/agent-hook-server.ts:271-285` (`buildPtyEnv()`).
- Cơ chế 2 tầng (main-process server local + relay-side server remote, forward qua JSON-RPC notification `agent.hook`) khớp với docstring trong chính code (`agent/src/relay/agent-hook-server.ts:1-13`) và test tích hợp `agent/src/relay/agent-hook-integration.test.ts:1-13` xác nhận round-trip: hook POST trên relay → `RelayDispatcher`/`SshChannelMultiplexer` → `AgentHookServer.ingestRemote()` (main-process).
- **Nhưng** "Mỗi agent có `HookService` riêng (`getStatus`/`install`/`installRemote`/`remove`)" (F04 doc dòng 91) — các class này (`hooks.ts`, `kimi/hook-service.ts`, `droid/hook-service.ts`, `cursor/hook-service.ts`, `amp/hook-service.ts`, `hermes/hook-service.ts`...) chỉ tồn tại tại `backend/src/main/`, `desktop/src/main/`, `frontend/src/main/` — **hoàn toàn vắng mặt trong `agent/src/main/`**. `installRemote()` (SFTP install lên SSH host) cũng chỉ có ở `backend/src/main/agent-hooks/wsl-hook-relay-deps.ts` và tương đương ở desktop, không có trong `agent/`.
- Vì vậy trong ranh giới audit `agent/`: package này đóng vai trò **"bên nhận"** (hook event listener/server, cả local lẫn relay) chứ không phải **"bên cài đặt"** (install hook config vào settings.json/config.toml của từng agent) — trách nhiệm đó nằm ở backend/desktop.

### 2.8 `agent/src/main/providers/` — không phải Provider Registry của HLD §15

- HLD §15.2 mô tả "Ba provider class mới (`src/main/providers/`)" — `DevServerFilesystemProvider`/`DevServerGitProvider`/`DevServerPtyProvider` — cài đặt `IFilesystemProvider`/`IGitProvider`/`IPtyProvider` để Gateway gọi RPC vào Dev Server Agent.
- Các file này **tồn tại thật, nhưng ở `backend/src/main/providers/dev-server-{git,pty,filesystem}-provider.ts` và bản song song `desktop/src/main/providers/...`** — không có bản nào trong `agent/src/main/providers/`.
- `agent/src/main/providers/` (nội dung thực tế: `types.ts` — 476 dòng, định nghĩa `IPtyProvider`/`IFilesystemProvider`/`IGitProvider`/`IProviderRegistry` interface, cộng `windows-agent-foreground-process.ts`, `windows-foreground-process-rows.ts`, `ssh-pty-id.ts` — 2 dòng) là bộ **Windows foreground-process detection** cho agent status recognition (`isAgentForegroundWrapperProcess`, `recognizeAgentProcessFromCommandLine` — `windows-agent-foreground-process.ts:1-4`), khớp với F04 doc's "Process recognition (phân biệt agent process với process thường)" (`F04-ai-agent-support.md:90,132`) — không liên quan tới HLD §15's Gateway-side Provider Registry dù trùng tên thư mục `providers/`.

### 2.9 `agent/src/main/codex-cli/command.ts` — tên module gây hiểu nhầm

- Thư mục `codex-cli/` chỉ có 1 file: `command.ts`. Nội dung không phải business logic riêng cho Codex, mà là bộ resolver PATH/version-manager dùng chung: `resolveCliCommand()`, `resolveCliCommands()`, `resolveCodexCommand()`, `resolveClaudeCommand()`, `getVersionManagerBinPaths()` (dòng 160-226) — probe PATH + các thư mục version manager (`volta`, `asdf`, `fnm`, `mise`, `nvm`, `pnpm`, `yarn`, `bun`) để tìm binary `claude`/`codex`/bất kỳ tên lệnh nào.
- Duy nhất nơi dùng các hàm này ngoài chính file: `agent/src/main/text-generation/commit-message-text-generation.ts` (AI commit-message generator, không phải luồng agent PTY chính). `agent-spawner.ts`'s `resolveAgentSpec()` (mục 2.2) **không dùng** bộ resolver này — nó spawn `spec.binary` (chuỗi literal `'claude'`/`'codex'`/...) trực tiếp và trông cậy vào PATH resolution mặc định của OS/node-pty, nghĩa là luồng agent-spawn chính **không có** fallback qua nvm/volta/asdf như luồng commit-message-generation có.

---

## 3. Nhận định tổng quan

1. **`ProfileAwareAgentSpawner` không tồn tại trong `agent/` — đúng như dự đoán, vì đây là thành phần Gateway-tier (backend/desktop)**, đã được backend audit xác nhận riêng. Trong `agent/`, đối trọng gần nhất là `SubAgentSpawner`/`resolveAgentSpec`/`buildAgentEnv` (`agent/src/relay/agent-spawner.ts`) — code tự ghi rõ trong comment đầu file rằng đây là "Dev Server tier", khác Gateway tier, để tránh nhầm lẫn. Đây là điểm hiếm hoi tài liệu-nội-bộ-code tự làm rõ ranh giới trách nhiệm tốt hơn cả doc thiết kế.
2. **"Profile hierarchy điều khiển agent spawn" — về cơ bản KHÔNG xảy ra bên trong `agent/`.** `OrcaProfile.ts` là type definition mồ côi (dead code), không một dòng runtime logic nào trong `agent/` đọc `ResolvedProfile`. Tất cả những gì BL-PRF-04 hứa hẹn (PATH mở rộng, `ANTHROPIC_MODEL`, `envVars` 3-tầng, trust-preset args) — nếu tồn tại — phải được backend resolve trước và truyền xuống qua `extraEnv`/`params.env` passthrough. `agent/` chỉ tự làm đúng một phần nhỏ: `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-userId (hardcode, không đọc profile).
3. **Danh sách agent 30+ trong F04 chỉ đúng cho luồng terminal-injection (`TUI_AGENT_CONFIG`, 32 agent) — không đúng cho luồng profile-aware/headless spawn (`AGENT_SPECS`, 5 agent).** Doc không phân biệt 2 phạm vi này, khiến người đọc nghĩ nhầm rằng tất cả 30+ agent đều spawn được qua `agent.spawn`/`agent.exec` có prefix-model-resolution.
4. **`agent.exec` có nguy cơ bug-do-tài-liệu-sai:** một implementation đầy đủ docblock/comment mô tả đúng vai trò TG-001 nhưng **hoàn toàn không được gọi** (`handleAgentExec` trong `agent-exec-handler.ts`), trong khi implementation thật sự chạy (`agent-rpc-dispatch.ts` inline case) không có docblock giải thích. Bất kỳ ai sửa `handleAgentExec` nghĩ rằng đang sửa hành vi thật của `agent.exec` sẽ không thấy tác dụng gì.
5. **3 state machine "agent status" khác nhau cùng tồn tại** trong `agent/` (`AgentLifecycleState` 6-state, `AgentStatusState` 4-state, `AgentStatus` 3-state) — không cái nào khớp state machine 5-state HLD §11.3 mô tả. Đây không hẳn là bug — 3 machine phục vụ 3 mục đích khác nhau (PTY process lifecycle / hook-driven UI status / OSC-title fallback) — nhưng HLD trình bày như thể chỉ có 1 state machine thống nhất.
6. **RPC method naming trong BL-PRF-04 sai:** `pty.spawn` (shell spawn + command injection) bị nhầm với `agent.spawn` (direct binary spawn qua node-pty) — đây là 2 RPC method khác nhau với model I/O khác nhau hoàn toàn.
7. **Trùng tên thư mục gây hiểu nhầm:** `agent/src/main/providers/` (Windows foreground-process detection) trùng tên với `backend|desktop/src/main/providers/` (Gateway-side `IPtyProvider`/`IFilesystemProvider`/`IGitProvider` registry mà HLD §15 mô tả) nhưng nội dung hoàn toàn khác nhau.

---

## 4. Khuyến nghị

- **Viết lại BL-PRF-04** để phản ánh đúng 2 tier thực tế: (a) resolve/merge profile + build env hoàn chỉnh xảy ra ở **backend** (`ProfileAwareAgentSpawner`, ngoài phạm vi `agent/`), (b) `agent/` chỉ là **executor thụ động** nhận `binary/args/env/cwd` đã resolve sẵn qua `agent.exec` (dispatcher inline case, `agent-rpc-dispatch.ts:594`) hoặc `agent.spawn` (`agent-spawner.ts`). Xóa mô tả `pty.spawn({cmd: agentBinary})` — không khớp implementation.
- **Dọn dead code:** xóa hoặc wire `handleAgentExec()` (`agent/src/relay/agent-exec-handler.ts:355-451`) vào dispatcher thật, tránh 2 implementation cùng tên RPC gây nhầm lẫn khi debug/maintain. Tương tự xét xóa hoặc thực sự dùng `agent/src/main/profile/OrcaProfile.ts` — nếu profile injection không bao giờ chạy trong `agent/`, giữ file type định nghĩa không dùng chỉ gây hiểu nhầm là có business logic ở đây.
- **Làm rõ trust preset:** field `trustPreset` trong `AgentEnvRequest` (`agent-spawner.ts:190`) hiện không được đọc trong `buildAgentEnv()` — hoặc implement, hoặc xóa khỏi interface để tránh giả định sai rằng trust preset được enforce ở tầng này.
- **Tách rõ 2 roster agent trong F04:** ghi chú rõ ràng "30+ agent" chỉ áp dụng cho terminal-injection launch (`TUI_AGENT_CONFIG`), còn headless/profile-aware spawn (`resolveAgentSpec`) chỉ hỗ trợ 5 model family — hoặc mở rộng `AGENT_SPECS` để khớp doc nếu đó là ý định sản phẩm.
- **Đổi tên `agent/src/main/codex-cli/command.ts`** thành tên phản ánh đúng chức năng chung (vd. `agent-cli-binary-resolver.ts`) theo đúng rule "File and Module Naming" của `AGENTS.md` — tên hiện tại khiến người đọc nghĩ nhầm đây là logic riêng cho Codex.
- **Thống nhất hoặc ít nhất liệt kê rõ trong doc** 3 state machine agent-status khác nhau và phạm vi áp dụng của từng cái, thay vì trình bày như một state machine 5-state thống nhất.

---

*Báo cáo đối chiếu `agent/src/**` với thiết kế PTY/AI Agent CLI Integration — một trong 5 mảng của audit tổng `agent/`, xem chỉ mục tại `audit/agent/agent-vs-design-review.md`.*
