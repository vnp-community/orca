# Agent → provides → Backend: `pty.*`, `ai.*`/`agent.*`, connectors, misc

Every RPC method the **agent** process exposes for the **backend** to call,
covering every namespace *other than* `git.*`/`fs.*` (see
[`agent-rpc-catalog-git-fs.md`](./agent-rpc-catalog-git-fs.md) for those). See
[`connection-modes.md`](./connection-modes.md) for the transport these calls
ride on, and [`backend-rpc-catalog.md`](./backend-rpc-catalog.md) for the
reverse direction.

Same two-surface architecture as the git/fs catalog:

| Transport | Entry point | Router | Handler style |
|---|---|---|---|
| **Part A — Direct WebSocket** ("Dev Agent") | `agent/src/relay/agent-connection-direct.ts` (agent dials out) / `agent-connection-relay.ts` (agent listens) | `agent/src/relay/agent-rpc-dispatch.ts` — flat `switch(rpc.method)` in `route()`, dynamic `import()` per case | Free functions, dynamically imported |
| **Part B — SSH-relay** ("Orca Relay") | `agent/src/relay/relay.ts` — standalone script deployed over SSH, framed JSON-RPC over stdin/stdout | `agent/src/relay/dispatcher.ts` `RelayDispatcher` — `Map<method, handler>`, `.onRequest()`/`.onNotification()` | Handler classes (`PtyHandler`, `ExternalAutomationsHandler`, `PortScanHandler`, `AgentExecHandler`, `WorkspaceSessionHandler`, `PreflightHandler`) constructed on one shared dispatcher, self-register in their constructors |
| **Part C — WSL-guest micro-relay** | `agent/src/relay/wsl-agent-hook-relay.ts` — launched inside a WSL distro by the Windows host | Own `RelayDispatcher` instance, same framed-stdio protocol | `wsl-hook-fs-bridge.ts` (`wslfs.*`, home-scoped) |

## `pty.*` — two independent implementations

### Part A: `agent-rpc-dispatch.ts` → `pty-daemon-client.ts` → Unix socket → `pty-daemon-server.ts` → `pty-agent-bridge.ts` (detached daemon process, see [`connection-modes.md`](./connection-modes.md) §9)

| Method | Registration | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|---|
| `pty.create` | `agent-rpc-dispatch.ts:948` | `pty-daemon-client.ts:194` → `bridge.ts:161` | `{cols?, rows?, cwd?, env?, shellOverride?, paneKey?, tabId?, command?, commandDelivery?, userId?}` | `{id, cols, rows, cwd, shell}` | Spawns a PTY in the daemon via lazy `import('node-pty')`. Shell: `shellOverride`→`env.SHELL`→`process.env.SHELL`→`/bin/sh`. `command`+`commandDelivery:'provider'` writes the command into the PTY ~50ms after spawn (fixed, see gaps-and-findings.md #5) | `safeCwd()` guard; `userId` + `command` starting with `gh `/`glab ` triggers per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` isolation |
| `pty.attach` | `:919` | `pty-daemon-client.ts:203` → `bridge:228` | `{id, cols?, rows?, suppressReplayNotification?, expectedPaneKey?, expectedTabId?}` | `{replay}` if suppressed, else `{}` + `pty.replay` push | Reattach after WS reconnect/agent restart | `attachIdentityMismatches()` |
| `pty.write` | `:932` | `pty-daemon-client.ts:212` → `bridge:318` | `{id, data}` | `{ok:true}` | Write stdin | none |
| `pty.resize` | `:945` | `pty-daemon-client.ts:221` → `bridge:343` | `{id, cols?, rows?}` (default 80x24) | `{ok:true}` | Resize | none |
| `pty.destroy` | `:958` | `pty-daemon-client.ts:230` → `bridge:381` | `{id, graceful?}` (default true) | `{ok:true}` / `{ok:true, alreadyDead:true}` | Windows: `kill()`; POSIX: SIGTERM(graceful)/SIGKILL | none |
| `pty.scrollback` | `:971` | `pty-daemon-client.ts:239` → `bridge:420` | `{id, lines?}` (default 100) | `{data: string}` | Returns tail of scrollback buf | `SCROLLBACK_LINES=500` cap |
| `pty.sendSignal` | `:984` | `pty-daemon-client.ts:248` → `bridge:441` | `{id, signal}` (default SIGTERM) | `{ok:true}` | Signal PTY process | `ALLOWED_SIGNALS`: SIGTERM/SIGKILL/SIGINT/SIGHUP/SIGTSTP |
| `pty.listProcesses` | `:999` | `pty-daemon-client.ts:257` → `bridge:498` | none | `[{id,cwd,title}]` | Enumerate all PTYs the daemon tracks | none |

No max-PTY cap on this path. Grace period fixed `PTY_GRACE_PERIOD_MS=120_000`.

Confirmed real backend caller: `backend/src/main/providers/dev-server-pty-provider.ts`
calls `pty.attach`/`pty.create`/`pty.write`/`pty.resize`/`pty.destroy`/
`pty.sendSignal`/`pty.listProcesses`.

### Part B: `pty-handler.ts`, class `PtyHandler` (`RelayDispatcher.onRequest` at `pty-handler.ts:514-527`)

| Method | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `pty.spawn` | `pty-handler.ts:633-808` | `{cols?, rows?, cwd?, env?, envToDelete?, shellOverride?, command?, terminalWindowsWslDistro?, commandDelivery?, userId?, paneKey?, tabId?, startupCommandDelivery?}` | `{id, cols, rows, cwd, shell}` | Spawns via node-pty; per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`; optional startup command gated on shell-ready marker | `validatePtyCwd()`; `ALLOWED_WINDOWS_SHELL_OVERRIDES` on Windows; **50-PTY cap** |
| `pty.attach` | `pty-handler.ts:810-877` | `{id, expectedPaneKey?, expectedTabId?, suppressReplayNotification?}` | `{replay?}` | Reattach; liveness via `isProcessAlive(pid)` | `attachIdentityMismatches()` |
| `pty.shutdown` | `pty-handler.ts:903-967` | `{id, immediate?}` | void | SIGTERM + 5s SIGKILL fallback (or sync SIGKILL if immediate) | none |
| `pty.sendSignal` | `pty-handler.ts:969-983` | `{id, signal}` | void | `ALLOWED_SIGNALS`: SIGINT/SIGTERM/SIGHUP/SIGKILL/SIGTSTP/SIGCONT/SIGWINCH/SIGUSR1/SIGUSR2 (superset of Part A) | -- |
| `pty.getCwd` | `pty-handler.ts:985-992` | `{id}` | `string` | Live cwd via `resolveProcessCwd` | none |
| `pty.getInitialCwd` | `pty-handler.ts:994-1001` | `{id}` | `string` | Spawn-time cwd | none |
| `pty.clearBuffer` | `pty-handler.ts:1003-1009` | `{id}` | void | Native screen clear | none |
| `pty.hasChildProcesses` | `pty-handler.ts:1011-1018` | `{id}` | `boolean` | `processHasChildren(pid)` | none |
| `pty.getForegroundProcess` | `pty-handler.ts:1020-1027` | `{id}` | `string\|null` | `getForegroundProcessName` | none |
| `pty.listProcesses` | `pty-handler.ts:1029-1042` | none | `{id,cwd,title,terminalHandle?}[]` | Enumerate PTYs in this relay process | none |
| `pty.getDefaultShell` | `pty-handler.ts:524` | none | `string` | Resolves host default shell | none |
| `pty.serialize` | `pty-handler.ts:1044-1069` | `{ids:string[]}` | JSON of `SerializedPtyEntry[]` | Metadata snapshot for relay-restart survival | none |
| `pty.revive` | `pty-handler.ts:1071-1170` | `{state:string}` | void | Re-adopts serialized PTYs whose pid is still alive | `sanitizeEnvToDelete()` bounds 1024 entries |
| `pty.getProfiles` | `pty-handler.ts:527` | none | shell profile list | Enumerate shell profiles | none |

Notifications, client→relay (`pty-handler.ts:529-533`): `pty.data {id,data}`,
`pty.resize {id,cols,rows}` (clamped 1-500), `pty.ackData` (empty no-op stub --
flow control not yet enforced).

Confirmed backend caller for SSH-relay path: `backend/src/main/providers/ssh-pty-provider.ts`.

### `pty.create` command delivery (fixed — was a real bug, not just naming)

Part A's `pty.create` now accepts `command`/`commandDelivery`/`userId`
(`pty-agent-bridge.ts`), mirroring Part B's `pty.spawn`: when
`commandDelivery === 'provider'`, the agent writes the command into the PTY
itself ~50ms after spawn (matching `pty-handler.ts`'s
`STARTUP_COMMAND_WRITE_DELAY_MS` non-shell-ready-gated default — Part A
doesn't wire the OSC shell-ready-marker scanner Part B uses for its more
finicky multiline-prompt cases). `userId` triggers the same
`GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation as Part B
(`command.startsWith('gh ')`/`'glab '` — a prefix match, not exact, since
`command` carries the full shell line). `backend/src/main/ipc/onboarding-ipc.ts`
and `runtime/rpc/methods/github-auth.ts`/`gitlab-auth.ts` now route through
`getRemotePtyProvider(devServerId)` (picks `SshPtyProvider`/
`DevServerPtyProvider` per connection type) instead of a raw, connection-
type-unaware `relay.call('pty.spawn', ...)`, which threw `MethodNotFound` on
the default connection mode. Full before/after in
[`gaps-and-findings.md`](./gaps-and-findings.md) #5.

## `agent.*` / `ai.*`

| Method | Registration | Impl | Params | Returns | Description | Security/isolation |
|---|---|---|---|---|---|---|
| `agent.spawn` | `agent-rpc-dispatch.ts:690` (Part A only) | `agent-spawner.ts:337` | `model/modelId, taskId, userId, accountId, cwd?, resumeId?, worktreePath?, branchName?, cols?, rows?, trustPreset?, resolvedApiKey?` | `{type:'spawn.accepted'}` immediately (fire-and-forget); real result streams via notifications | Spawns interactive PTY-based AI CLI (claude/codex/gemini/opencode/ollama) via `resolveAgentSpec()`, tracked in `PTY_REGISTRY` | `ptyId=pty-${userId}-${taskId}-${Date.now()}`; API key only from param or pre-injected env; per-user `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` |
| `agent.kill` | `:703` | `agent-spawner.ts:543` | `ptyId(required), signal?(default SIGTERM)` | `{ok:true}` idempotent | Kills tracked PTY | Windows: signal arg ignored |
| `agent.sendInput` | `:716` | `agent-spawner.ts:586` | `ptyId(required), data:string` | `{ok:true}` | Writes to PTY stdin | No content validation beyond type check |
| `agent.exec` | `:733-802` (inline, Part A only) | same file | `binary(required), args?, cwd?, stdin?, env?, timeoutMs?(clamped 1000-300000, default 300000), stepId?, taskId?, parentTraceId?` | `{stdout, stderr, exitCode, timedOut}` | Non-interactive spawn, no PTY. Used by `StepExecutors.executeAgent()`/`ProfileAwareAgentSpawner` | No output-size cap, no cancel companion RPC |
| `agent.execNonInteractive` | `agent-exec-handler.ts:140` (Part B only) | `agent-exec-handler.ts:156` | `binary(required), args?, cwd?, stdin?, timeoutMs?(clamped 1000-300000, default 60000), env?, operation?` | `{stdout, stderr, exitCode, timedOut, canceled?, spawnError?}` | SSH-relay equivalent, used for AI commit-message generation; one in-flight job per `(operation,cwd)` lane | **Output capped 4MB/stream**; `.cmd`/`.bat` dangerous-metachar args rejected |
| `agent.cancelExec` | `agent-exec-handler.ts:143` | `agent-exec-handler.ts:146` | `cwd, operation?` | `{canceled:boolean}` | Cancels matching lane's in-flight job | Lane key = `[operation,cwd]` |
| `ai.complete` | `:808` | `ai-complete-handler.ts:47` | `prompt(required), format?(default text), taskId?, model?(param→config.defaultModel→env→'claude-opus-4-5'), accountId?, resolvedApiKey?` | `{content, model?}` | Single-shot completion for `TaskAIPlanner.decompose()` + commit-message gen; routes by model prefix, 120s timeout | Key priority: pre-injected env → `resolvedApiKey` → error; never logs prompt/response |
| `ai.provider.writeCredential` | `:455` | `agent-credential-store.ts:103` | `accountId(required), encryptedBlob(required), iv(required), algorithm?` | `{ok:true}` | Persists Layer-1 blob (browser-encrypted), itself Layer-2-encrypted, to `~/.orca/credentials/<accountId>.enc` | dir 0700/file 0600; accountId charset-validated |
| `ai.provider.readCredential` | `:466` | `agent-credential-store.ts:154` | `accountId(required), parentSpanId?` | `{accountId, encryptedBlob, iv, algorithm}` | Decrypts only Layer 2 -- never returns plaintext key | Master key `ORCA_AI_CREDENTIAL_KEY`, throws if unset |
| `ai.provider.healthCheck` | `:477` | `agent-credential-store.ts:208` | `accountId, provider?(default anthropic)` | `{ok, latencyMs, note, credentialFound}` or `-32001` error | Credential-exists check + 5s HEAD probe | Reuses read-credential handler |
| `ai.provider.testConnection` | new case (Part A) | `agent-credential-store.ts:handleTestConnection` | same as `ai.provider.healthCheck` | same as `ai.provider.healthCheck` | Called by `AIProviderService.ts` for Settings "Test connection" + shadow-credential rotation verification. Thin alias of `handleHealthCheck` (matches the pattern the separate `desktop/` codebase already used) — previously unimplemented, see [`gaps-and-findings.md`](./gaps-and-findings.md) #1 | same as `ai.provider.healthCheck` |
| `ai.provider.deleteCredential` | `:499` | `agent-credential-store.ts:294` | `accountId(required)` | `{ok:true, deleted}` idempotent | Deletes `.enc` file | same path-safe helper |

Encryption: two-layer -- Layer 1 (browser SubtleCrypto AES-GCM) + Layer 2
(`scryptSync` + `aes-256-gcm`) -> `{version,salt,iv2,authTag,data}` base64 at
`~/.orca/credentials/<accountId>.enc`. Agent never obtains the plaintext key.

Confirmed real backend callers: `TaskAIPlanner.ts:62` (`ai.complete`);
`StepExecutors.ts:107` / `ProfileAwareAgentSpawner.ts:130` (`agent.exec`);
`AIProviderService.ts:315,378,446` (`ai.provider.writeCredential`);
`AIProviderService.ts:374` (`ai.provider.readCredential`).

## `github.*` / `gitlab.*` connectors (`external-api-connector.ts`, Part A switch)

| Method | Registration | Params | Returns | Description | Security/isolation |
|---|---|---|---|---|---|
| `github.pr.create` | `:591-599` → `:169-234` | `title(required), body?, base?(default main), cwd?, userId?, draft?` | `{url,number,title,state}` (+`alreadyExisted` on dedup) | `gh pr create ...`, dedups by branch via `gh pr list --head` first | title/base metachar-checked; `buildGhEnv`; rate-limit circuit breaker; 30s timeout |
| `github.pr.merge` | `:602-610` → `:238-273` | `prNumber(required), cwd?, userId?, method?(default squash)` | `{ok:true, stdout}` | `gh pr merge --squash/--merge/--rebase --auto` | no retry/breaker; `buildGhEnv`; 30s timeout |
| `github.issue.list` | `:613-621` → `:277-307` | `cwd?, userId?, limit?(cap 50), state?(default open)` | `{issues, total}` | `gh issue list --json ...` | read-only, 30s timeout |
| `github.issue.create` | `:624-632` → `:311-350` | `title(required), body?, cwd?, userId?` | `{number,url,title}` | `gh issue create ...` | title metachar-checked, no retry/dedup |
| `github.auth.status` | `:668-676` → `:354-382` | `userId?` | `{ok, stdout, stderr}` | `gh auth status` | `buildGhEnv`, 10s timeout |
| `gitlab.mr.create` | `:635-643` → `:386-436` | `title(required), description?, targetBranch?(default main), cwd?, userId?` | `{url, stdout, stderr}` | `glab mr create --yes`, URL scraped from stdout | `buildGlabEnv`, `idempotent:false`, 30s timeout |
| `gitlab.mr.list` | `:657-665` → `:440-469` | `cwd?, userId?, state?(default opened)` | `{mrs, total}` | `glab mr list --output json` | read-only, 30s timeout |
| `gitlab.pipeline.status` | `:646-654` → `:473-502` | `cwd?, userId?` | `{status, raw}` | `glab pipeline status --output json` | read-only, 30s timeout |
| `gitlab.auth.status` | `:679-687` → `:506-534` | `userId?` | `{ok, stdout, stderr}` | `glab auth status` | `buildGlabEnv`, 10s timeout |

`git.pr.create` (git/fs catalog) is a deliberate alias of `github.pr.create` --
same impl, same idempotency behavior.

Isolation: `buildGhEnv(userId)` -> `GH_CONFIG_DIR='~/.config/gh/<userId>/'` +
`GH_NO_UPDATE_NOTIFIER='1'` + `GH_PROMPT_DISABLED='1'`;
`buildGlabEnv(userId)` -> `GLAB_CONFIG_DIR='~/.config/glab-cli/<userId>/'` +
`NO_COLOR='1'` + `CI='1'` -- GitLab isolation is via `GLAB_CONFIG_DIR`, not a
`GITLAB_TOKEN` env var. All calls `spawn(binary,args,{shell:false})` -- argv
array, never shell string concat, with metachar checks as belt-and-suspenders.
No streaming. Only `github.pr.create`/`gitlab.mr.create` get retry/breaker.

## `externalAutomations.*` (Hermes/OpenClaw cron automations, Part B only)

Registered `external-automations-handler.ts:84-88`.

| Method | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `externalAutomations.list` | `listJobs()` @819-838 | `{provider?:'hermes'\|'openclaw', default hermes}` | `{jobs, hermesAvailable, openclawAvailable, error}` | Lists cron jobs from `~/.hermes/cron/jobs.json` or `~/.openclaw/cron/jobs.json` | fixed paths under `homedir()` |
| `externalAutomations.runs` | `listRuns()` @622-655 | `{provider?, jobId(required), page?, pageSize?}` | `{total, runs}` | Paginated run history, merges output files + `~/.hermes/state.db` | `jobId` pattern-validated; parameterized SQL; log files `realpath`-checked, capped 5MB |
| `externalAutomations.create` | `createJob()` @901-919 | `{provider?('hermes' only), name?, prompt(required), schedule(required), workdir?}` | `{ok:true}` | `hermes cron create ...`; OpenClaw create unsupported | `execFileAsync` argv array, 30s timeout |
| `externalAutomations.update` | `updateJob()` @921-944 | `{provider?, jobId(required), name?, prompt, schedule, workdir?}` | `{ok:true}` | `hermes cron edit <jobId> ...` | same jobId pattern check |
| `externalAutomations.act` | `runAction()` @946-966 | `{provider?, action:'pause'\|'resume'\|'run'\|'delete'(required), jobId(required)}` | `{ok:true}` | Maps action to CLI verb (hermes: pause/resume/run/remove; openclaw: disable/enable/run/rm) | action restricted to 4-value union |

No streaming. Note: backend's `StepExecutors.executeNotification()` calls
`relay.call('notification.send', ...)`, a **different** namespace from
`externalAutomations.*` — see `notification.*` below.

## `shell.*` / `notification.*` -- workflow step executors (Part A only)

Both previously unimplemented -- see [`gaps-and-findings.md`](./gaps-and-findings.md) #1.

| Method | Registration | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|---|
| `shell.exec` | new case, `agent-rpc-dispatch.ts` | `handleShellExec`, `fs-agent-extensions.ts` | `script(required), env?, traceId?, timeoutMs?(clamped 1000-300000, default 300000)` | `{stdout, stderr, exitCode, truncated?}` | Runs `spawn('sh', ['-c', script], {env: {...process.env, ...env}})` for `StepExecutors.executeShell()`'s workflow `shell` steps | Same trust model as `shell.eval` (only reachable via authenticated relay, no allowlist on script content); output capped 4MB/stream (matches `agent.execNonInteractive`) |
| `notification.send` | new case, `agent-rpc-dispatch.ts` | `handleNotificationSend`, new file `notification-send-handler.ts` | `channel?(default 'default'), message(required), traceId?` | `{ok:true, delivered:boolean, note}` | Logs the notification and best-effort delivers an OS-level desktop notification (`notify-send` on Linux, `osascript` on macOS) if one is reachable on this host; always acknowledges even when no delivery target exists, so a workflow `notification` step never fails the run | No general slack/email delivery here by design -- that belongs on the backend, which owns user identity/integration credentials; delivery failures are swallowed, never thrown |

Distinct from `shell.eval` (git/fs catalog — internal-only `~` resolution,
no env/timeout/tracing params) and from `externalAutomations.*` (cron job
management, unrelated to one-shot notifications).

## `preflight.*`

Two unrelated implementations share the method name `preflight.check`.

| Method | Part | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|---|
| `preflight.detectAgents` | B | `preflight-handler.ts:71-115` | `commands:{id,cmd,requiredCommands?,unsupportedRuntimes?}[]` | `{agents:string[], platform}` | Detects installed agent CLIs via PATH lookup | never executes the detected command |
| `preflight.detectWindowsTerminalCapabilities` | B | `:118-143` | none | `{wslAvailable, wslDistros, pwshAvailable, pwshVersion?, gitBashAvailable, gitBashPath?, hostPlatform}` | Probes WSL/PowerShell/Git-Bash | `execFile` (no shell) |
| `preflight.detectGhosttyConfig` | B | `:193-204` | none | `{configPath, themeDir}` | Checks `~/.config/ghostty/{config,themes}` | pure `existsSync` |
| `preflight.check` | B | `:208-286` | none | `{platform, gh:{...}, glab:{...}, git:{...}}` | Full gh/glab/git install+auth+identity check | failures swallowed to `installed:false` |
| `preflight.check` | A (different contract, same name) | `agent-rpc-dispatch.ts:488` → `fs-agent-extensions.ts:262-304` | `{services:('github-cli'\|'ripgrep'\|'docker'\|'claude')[]}` | `Record<string,boolean>` | Binary-availability probe only, no auth/identity info | `spawn(binary,['--version'])`, 5s timeout |
| `preflight.setGitIdentity` | B | `:297-305` | `{name(required), email(required)}` | void | Stores git identity scoped to the RPC connection (`clientId`), never `git config --global` | in-memory map, cleared on client detach |

Confirmed backend caller talks to Part B: `backend/src/main/ipc/onboarding-ipc.ts`
-- `preflight.check` (:191), `preflight.setGitIdentity` (:215),
`preflight.detectGhosttyConfig` (:235), `preflight.detectWindowsTerminalCapabilities`
(:297), `preflight.detectAgents` (via `DevServerRelayBridge.detectAgents()`, :104,141).

## `ports.*` (port/host scanning, Part B only)

| Method | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `ports.detect` | `port-scan-handler.ts:20`; Linux: `scanLinuxListeningPorts()`; Windows: `windows-port-scan.ts`; else `[]` | none | `{ports:{port,host,pid?,processName?}[], platform}` | Scans local dev-server host for TCP listening sockets | see below |

Linux: reads `/proc/net/tcp{,6}` directly, cross-references inode→PID via
`/proc/<pid>/fd/*`; excludes port 22, relay's own pid/ppid, `sshd`-named
processes; capped `MAX_DETECTED_PORTS=50`. Windows: PowerShell
`Get-NetTCPConnection`/`Get-Process` via base64 `-EncodedCommand`, `netstat.exe`
fallback; only invokes `powershell.exe`/`pwsh.exe`/`netstat.exe`, never a
user-supplied binary; failure resolves to `[]`, never throws.

## `workspace.*` (client presence + cross-device session sync, Part B only)

| Method | Impl | Params | Returns | Description | Security |
|---|---|---|---|---|---|
| `workspace.get` | `:62`→`:71-113` | `{namespace?}` | `{namespace, revision, updatedAt, schemaVersion, session}` | Reads persisted session snapshot | file read only |
| `workspace.patch` | `:63`→`:127-160` | `{namespace?, baseRevision?, clientId?, patch:{kind:'replace-session', session}}` | `{ok:true,snapshot}` or stale-revision/unavailable variants | Optimistic-concurrency full-session replace, atomic write, broadcasts `workspace.changed` | file 0600/dir 0700; only `replace-session` supported |
| `workspace.presence` | `:64`→`:162-186` | `{namespace?, clientId?, clientName?}` | `{clients:{clientId,name,lastSeenAt}[]}` | Heartbeat/roster, prunes stale beyond `PRESENCE_TTL_MS=45_000` | namespace charset-sanitized |

Notification `workspace.changed` `{namespace, snapshot, sourceClientId?}` --
broadcast to every client on the relay daemon after a successful `patch`.

Note: `workspace-space-scan.ts` registers no RPC itself -- consumed by
`fs.workspaceSpaceScan` (git/fs catalog).

## Relay-wiring / connective methods (`relay.ts`, Part B)

| Method/constant | Params | Returns | Description | Security |
|---|---|---|---|---|
| `session.registerRoot` | `{rootPath}` | `{ok:true}` | Legacy no-op kept for version-skew compat | none |
| `session.resolveHome` | `{path}` | `{resolvedPath}` | Resolves `~` to absolute path | none |
| `orca.cli` | `{argv, cwd, env, stdin?}` | `{stdout,stderr,exitCode}` | **Reverse-RPC**: remote `orca` CLI shim calls this; relay forwards via `requestAnyClient` to the owning Orca app. Full trace in [`backend-rpc-catalog.md`](./backend-rpc-catalog.md) | timeout via `remoteCliRequestTimeoutMs()` |
| `relay.configureGraceTime` | `{graceTimeSeconds}` | `{graceTimeMs}` | Sets grace period to 0 before system sleep | none |
| `agent_hook.requestReplay` | none | `{replayed:number}` | Replays cached agent-status hook events after reconnect | capped `MAX_CACHED_PANES=256` |
| `agent_hook.installPlugins` | `{opencodePluginSource?, piExtensionSource?, ompExtensionSource?}` | `{installed:{opencode,pi,omp}}` | Ships plugin/extension source to write on next PTY spawn | 256KB cap per source |
| `relay.status` | none | `{pid, uptimeMs, detached, stdoutAlive, memory, ptys, socket, grace}` | Relay daemon health snapshot | none |

Handler wiring (`relay.ts:404`): `PtyHandler`(470), `FsHandler`(471),
`GitHandler`(472), `PreflightHandler`(474), `ExternalAutomationsHandler`(475),
`FsDirectoryBrowserHandler`(479), `GitCloneHandler`(482), `PortScanHandler`(485),
`AgentExecHandler`(488), `WorkspaceSessionHandler`(491).

`plugin-overlay.ts` and `relay-command-env.ts`/`relay-diagnostic-log.ts`
register no RPC -- pure filesystem/env-construction helpers consumed internally.

## `wslfs.*` -- Part C (WSL-guest micro-relay only)

`wsl-hook-fs-bridge.ts:60-145`: `wslfs.home`/`readFile`/`writeFile`/`stat`/
`rename`/`unlink`/`chmod`/`readdir`/`mkdir` -- home-scoped fs bridge for the
WSL-guest micro-relay, so the Windows host can install hook configs without
per-file `wsl.exe` spawns. Scoped under guest `$HOME`; only reachable from the
host-owned stdio channel, never agent-exposed to the backend directly.

## `tools/*` -- MCP tool discovery (Part A)

`tools/list` (`agent-rpc-dispatch.ts:270`) lists MCP `ToolDefinition[]`;
`tools/call` (`:283`) invokes a registered MCP tool by name.

## Formerly-confirmed gaps -- now fixed

`shell.exec`, `notification.send`, and `ai.provider.testConnection` were all
previously unimplemented agent-side (`StepExecutors.ts`/`AIProviderService.ts`
called them, but the agent had no handler for any of the three, confirmed by
a dedicated agent-side test asserting `MethodNotFound`). All three now have
real handlers -- see the `shell.*`/`notification.*` section above and the
`ai.provider.testConnection` row in the `ai.*` table. Full before/after in
[`gaps-and-findings.md`](./gaps-and-findings.md) #1.

## Distinct non-JSON-RPC connection: agent-hook loopback HTTP server

Not JSON-RPC -- see [`connection-modes.md`](./connection-modes.md) §9 and
[`backend-rpc-catalog.md`](./backend-rpc-catalog.md) §4 for the full trace of
`agent-hook-server.ts` → `agent.hook` notification.

## Streaming / notification protocols (exact message shapes)

All are JSON-RPC 2.0 notifications (no `id`).

**Part B PTY output** (`pty-handler.ts`) -- batched/throttled: `pty.data
{id,data}` (immediate if recent+small, else batched by an 8ms timer,
`PTY_OUTPUT_BATCH_INTERVAL_MS`, ≤16KB/call); output passes through an OSC
shell-ready-marker scanner before appending to a 100KB replay buffer.
`pty.exit {id, code}` (natural, or `code:-1` on forced-kill fallback).
`pty.replay {id, data}` (≤100KB) on `pty.attach` unless suppressed.

**Part A/daemon PTY output** (`pty-agent-bridge.ts`) -- unbatched, immediate:
`pty.data {id, data}`, `pty.exit {id, exitCode, signal}`, `pty.replay {id,
data}` (≤500 lines). Transport: NDJSON over a Unix socket between agent
process and detached daemon -- distinct from the WS binary framing.

**Field-name divergence**: `pty.exit`'s exit-code field is `code` on Part B vs
`exitCode` on Part A -- a consumer written against one shape will silently
misread the other. See [`gaps-and-findings.md`](./gaps-and-findings.md).

**AI-agent spawn output** (`agent-spawner.ts`, Part A only): `agent.output
{ptyId, data}` (base64), `agent.exited {ptyId, exitCode}`. `agent.spawn`'s
synchronous response is always `{type:'spawn.accepted'}` -- callers must
correlate `ptyId` from the first notification instead. **No confirmed backend
consumer found** for either -- see [`gaps-and-findings.md`](./gaps-and-findings.md).

**Workspace sync**: `workspace.changed {namespace, snapshot, sourceClientId?}`.

No `execStream`/chunked-shell-output shape exists for `github.*`/`gitlab.*`/
`externalAutomations.*` -- they all buffer fully before resolving.

## Sources

`agent/src/relay/agent-rpc-dispatch.ts`, `pty-handler.ts`, `pty-agent-bridge.ts`,
`pty-daemon-protocol.ts`, `pty-daemon-server.ts`, `pty-daemon-client.ts`,
`pty-shell-launch.ts`, `agent-spawner.ts`, `agent-exec-handler.ts`,
`ai-complete-handler.ts`, `agent-credential-store.ts`,
`notification-send-handler.ts`, `fs-agent-extensions.ts` (`handleShellExec`),
`external-automations-handler.ts`, `external-api-connector.ts`,
`preflight-handler.ts`, `port-scan-handler.ts`, `windows-port-scan.ts`,
`workspace-session-handler.ts`, `workspace-space-scan.ts`, `plugin-overlay.ts`,
`agent-hook-server.ts`, `relay-command-env.ts`, `relay-diagnostic-log.ts`,
`relay.ts`, `dispatcher.ts`, `wsl-hook-fs-bridge.ts`,
`wsl-agent-hook-relay.ts`, `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts`.
Backend cross-references: `backend/src/main/providers/dev-server-pty-provider.ts`,
`ssh-pty-provider.ts`, `backend/src/main/task/TaskAIPlanner.ts`,
`backend/src/main/workflow/StepExecutors.ts`,
`backend/src/main/project/ProfileAwareAgentSpawner.ts`,
`backend/src/main/ai-providers/AIProviderService.ts`,
`backend/src/main/ipc/onboarding-ipc.ts`.
