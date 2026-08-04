# Backend-Side Full-Flow Tracing — AI Execution Task Index

**Solutions Ref:** [../solutions/](../solutions/) ([00-index.md](../solutions/00-index.md) — đọc trước, chứa các phát hiện xuyên suốt)
**Total tasks:** 45 | **New files:** ~24 | **Modified files:** ~38 (nhiều file được modify bởi >1 task, xem "Quick Reference" bên dưới)

**⚠️ 2026-08-04 — destructive `git reset --hard HEAD` observed mid-run:** while executing TASK-BE-003.1→3, the working tree's edits to 5 already-tracked files (`terminal.ts`, `local-pty-provider.ts`, `ssh-pty-provider.ts`, `terminal-scrollback-snapshots.ts`, `ipc/session.ts`) were silently wiped back to HEAD — confirmed via `git reflog` showing repeated `reset: moving to HEAD` entries, not run by this agent. Untracked files (this whole `specs/` tree, new test files) survived since `git reset` doesn't touch untracked paths — only edits to files already committed to git are at risk. **If your task's target file is already tracked by git, re-`Read` and re-verify your edits (`git status`/`git diff`) are still present immediately before running verification/marking the task done** — a silent revert won't error, it just makes the file match HEAD again with no local trace. Root cause not identified (possibly a concurrent `git pull`/cleanup step run by another agent or harness process); flagging here so other concurrent agents don't lose work the same way.

---

## Execution Phases

| Phase | Domain | Tasks | Prerequisite |
|-------|--------|-------|---------------|
| **Phase 0** | Core API (`src/shared/trace/index.ts` — `Tracer.start(resume)`) | TASK-BE-000 | Không có — root của toàn bộ cây (agent + backend + frontend domain) |
| **Phase 1** | Worktree (CR-001), Agent Orchestration (CR-002), Terminal (CR-003) | TASK-BE-001.1→4, TASK-BE-002.1→4, TASK-BE-003.1→4 | Phase 0 |
| **Phase 2** | Code Review (CR-005), Agent WS (CR-013), Remote Integration (CR-014) | TASK-BE-005.1→4, TASK-BE-013.1→4, TASK-BE-014.1→4 | Phase 0 (độc lập với Phase 1) |
| **Phase 3** | Profile (CR-015), AI Providers (CR-016), Workflow Orchestration (CR-017), Task Graph (CR-018) | TASK-BE-015.1→5, TASK-BE-016.1→4, TASK-BE-017.1→5, TASK-BE-018.1→6 | Phase 0; CR-018 cần đọc CR-017 trước (xem `parentTraceId`/resume design dùng chung) |

**Lưu ý thứ tự trong Phase 3:** SOL-BE-TRACE-017 và SOL-BE-TRACE-018 dùng chung thiết kế `parentTraceId`/resume — `TASK-BE-017.1` (migration `0013_workflow_trace_correlation.ts`, cột `root_trace_id`) PHẢI chạy trước `TASK-BE-018.4` (mở rộng `AgentSpawnOptions.traceId`, tái dùng đúng cột/pattern persist của 017, không tạo migration mới cho Task Graph).

---

## ✅ Known Conflicts — Resolved 2026-08-02

Khi decompose, phát hiện **`ProfileAwareAgentSpawner.spawn()`** và **`TaskAgentExecutor.executeTask()`** được 2 CR độc lập (viết bởi nỗ lực trước, chưa cross-review lẫn nhau) đề xuất instrument theo 2 cách khác nhau trên cùng 1 method:

| File | CR-002 (Phase 1) đề xuất | CR-015/018 (Phase 3) đề xuất | Task liên quan |
|------|---------------------------|-------------------------------|-----------------|
| `ProfileAwareAgentSpawner.spawn()` | Bọc bằng `Tracers.agentOrchSpawn` (flow `agentOrch:spawn`) | Bọc bằng `Tracers.profileAgentSpawnFlow` (flow `profile:agentSpawnRoute`), sau đó CR-018 mở rộng để resume | `TASK-BE-002.2` vs `TASK-BE-015.4` + `TASK-BE-018.4` |
| `TaskAgentExecutor.executeTask()` | KHÔNG tạo span riêng — chỉ propagate `traceId` xuyên qua (SOL-002 §1.5, tránh double-instrument) | Tạo span mới `Tracers.taskGraphExecuteFlow` (flow `taskGraph:execute`) bao trọn hàm | `TASK-BE-002.3` vs `TASK-BE-018.5` |

**Quyết định resolve (2026-08-02, theo CR-TRACE-000 §3.1 — cơ chế `resume`):** `ProfileAwareAgentSpawner.spawn()` là 1 method thật duy nhất được gọi từ 3 nơi (RPC trực tiếp, profile routing, task-graph execution). Nguyên tắc áp dụng: **flow đại diện trực tiếp nhất cho "spawn 1 AI agent" sở hữu span canonical, các flow liền kề correlate vào đó qua `resume` thay vì mở 1 span gốc cạnh tranh, khác tên, cho cùng 1 thao tác.**

- **`Tracers.agentOrchSpawn`** (`agentOrch:spawn`, SOL-BE-TRACE-002) là span CANONICAL duy nhất bọc `ProfileAwareAgentSpawner.spawn()` — tên khớp chính xác bảng quy ước CR-TRACE-000 §4 ("agentOrch: agentOrch:spawn"). `TASK-BE-002.2` giữ nguyên như đã viết, không đổi code — chỉ bổ sung ghi chú resume.
- **`Tracers.profileAgentSpawnFlow`** (`profile:agentSpawnRoute`, SOL-BE-TRACE-015) KHÔNG còn bọc `spawn()` độc lập. Nó chuyển xuống bọc phần chuẩn bị/routing theo profile domain (`assertAccess`) xảy ra TRƯỚC khi gọi `spawn()`, tại `project-rpc-handler.ts`'s `project.agentSpawn` handler — rồi forward span id của chính nó làm `traceId` để `agentOrch:spawn` **resume**. `TASK-BE-015.4` viết lại theo hướng này.
- **`TaskAgentExecutor.executeTask()`** ĐƯỢC sở hữu span riêng `taskGraph:execute` (khớp lập trường của SOL-018) — bao trọn grant-check + AI-planning + lời gọi `spawn()`. `span.id` của `taskGraph:execute` forward làm `traceId` khi gọi `agentSpawner.spawn()`, để `agentOrch:spawn` resume đúng id đó — KHÔNG qua `profile:agentSpawnRoute` (span đó chỉ tồn tại ở nhánh `project.agentSpawn` RPC trực tiếp). `TASK-BE-018.5` giữ nguyên phần lớn code, chỉ đổi tên tracer resume-target trong comment. `TASK-BE-018.4` (vốn định tự thêm `AgentSpawnOptions.traceId`) nay chỉ còn là task verify — field đó đã do `TASK-BE-002.2` sở hữu.

Cả 5 task liên quan (`TASK-BE-002.2`, `TASK-BE-002.3`, `TASK-BE-015.4`, `TASK-BE-018.4`, `TASK-BE-018.5`) và 3 solution doc gốc (`SOL-BE-TRACE-002`, `SOL-BE-TRACE-015`, `SOL-BE-TRACE-018`) đã được cập nhật để phản ánh quyết định này — không còn "⚠️ Xung đột chưa giải quyết" nào trong các file đó. **Thứ tự thực thi bắt buộc (mới, thay cho "không chạy song song" trước đây):** `TASK-BE-002.2` → `TASK-BE-002.3` → (`TASK-BE-015.4` và `TASK-BE-018.4`/`TASK-BE-018.5` có thể chạy song song với nhau, nhưng cả hai đều PHẢI chạy sau `TASK-BE-002.2`/`TASK-BE-002.3` vì chúng resume vào span mà 2 task đó tạo ra, và `TASK-BE-015.4` sửa tiếp cùng file `project-rpc-handler.ts` mà `TASK-BE-002.3` đã patch trước) — mỗi task file đã tự cập nhật mục "Prerequisite" ở đầu file cho khớp thứ tự này.

---

## Task List

| Task | Phase | SOL Ref | File | Description | Status |
|------|-------|---------|------|-------------|--------|
| [TASK-BE-000](./TASK-BE-000-tracer-resume-api.md) | 0 | CR-TRACE-000 §3.1 | `src/shared/trace/index.ts` | `Tracer.start(fields?, resume?)` — core API, precondition cho tất cả 30 solution | ⬜ Pending |
| [TASK-BE-001.1](./TASK-BE-001.1-worktree-tracers-and-schemas.md) | 1 | SOL-BE-TRACE-001 | `tracers.ts`, `worktree-schemas.ts`, `git-remote.ts` (schema) | Đăng ký 5 tracer `worktree:*` + field `traceId` vào schema | ✅ Done |
| [TASK-BE-001.2](./TASK-BE-001.2-worktree-rpc-handler-instrumentation.md) | 1 | SOL-BE-TRACE-001 | `runtime/rpc/methods/worktree.ts` | Instrument `worktree.create`/`worktree.rm` | ✅ Done |
| [TASK-BE-001.3](./TASK-BE-001.3-worktree-git-remote-relay-bridge.md) | 1 | SOL-BE-TRACE-001 | `git-remote.ts`, `dev-server-relay-bridge.ts` | Instrument `git.worktree.add/remove` + fix `callWithTimeout()` resume theo `params.traceId` | ✅ Done |
| [TASK-BE-001.4](./TASK-BE-001.4-worktree-tests.md) | 1 | SOL-BE-TRACE-001 | 4 test file | ≥20 Vitest case cho worktree tracing | ✅ Done |
| [TASK-BE-002.1](./TASK-BE-002.1-agent-orch-tracers.md) | 1 | SOL-BE-TRACE-002 | `tracers.ts` | Đăng ký 5 tracer `agentOrch:*` (chỉ `agentOrchSpawn` có call site thật) | ✅ Done |
| [TASK-BE-002.2](./TASK-BE-002.2-profile-aware-agent-spawner-instrumentation.md) | 1 | SOL-BE-TRACE-002 | `ProfileAwareAgentSpawner.ts` | Instrument `spawn()` — điểm hội tụ duy nhất, `agentOrch:spawn` là span **canonical** (✅ Known Conflicts resolved), gửi cả `traceId` phẳng + `_trace.id` lồng | ✅ Done |
| [TASK-BE-002.3](./TASK-BE-002.3-agent-spawn-callers-propagate-traceid.md) | 1 | SOL-BE-TRACE-002 | `project-rpc-handler.ts`, `TaskAgentExecutor.ts`, `task-rpc-handler.ts` | Propagate `traceId` qua 2 caller thật, không tạo span trùng (✅ Known Conflicts resolved) | ✅ Done |
| [TASK-BE-002.4](./TASK-BE-002.4-agent-orchestration-tests.md) | 1 | SOL-BE-TRACE-002 | 6 test file | ≥16 Vitest case cho agent orchestration tracing | ✅ Done |
| [TASK-BE-003.1](./TASK-BE-003.1-terminal-tracers-and-rpc-handlers.md) | 1 | SOL-BE-TRACE-003 | `tracers.ts`, `runtime/rpc/methods/terminal.ts` | Đăng ký 4 tracer `terminal:*` + instrument `.create`/`.split`/`.resizeForClient` | ✅ Done |
| [TASK-BE-003.2](./TASK-BE-003.2-pty-provider-instrumentation.md) | 1 | SOL-BE-TRACE-003 | `local-pty-provider.ts`, `ssh-pty-provider.ts` | Instrument `spawn()` trên cả 2 provider (cùng tên method, đã confirm) | ✅ Done |
| [TASK-BE-003.3](./TASK-BE-003.3-scrollback-save-restore-instrumentation.md) | 1 | SOL-BE-TRACE-003 | `terminal-scrollback-snapshots.ts`, `ipc/session.ts` | Instrument batch save `migrateWorkspaceSessionTerminalScrollbackSnapshots()` — KHÔNG phải per-terminal-destroy hook như CR gốc giả định | ✅ Done |
| [TASK-BE-003.4](./TASK-BE-003.4-terminal-management-tests.md) | 1 | SOL-BE-TRACE-003 | 7 test file | ≥18 Vitest case, kèm regression guard OSC133 không bị instrument | ✅ Done |
| [TASK-BE-005.1](./TASK-BE-005.1-code-review-tracers.md) | 2 | SOL-BE-TRACE-005 | `tracers.ts` | Đăng ký 5 tracer (annotate/feedback chỉ khai báo, chưa wire — BL-CR-02/03 chưa tồn tại); DRIFT: key names renamed to `codeReviewDiff`/`codeReviewAnnotate`/`codeReviewFeedback`/`codeReviewAiCommit`/`codeReviewCreatePr` (bare, no `Flow` suffix) — the `*Flow` names were already claimed by a concurrent frontend `ui:codeReview.*` task | ✅ Done |
| [TASK-BE-005.2](./TASK-BE-005.2-git-local-diff-commit-pr-tracing.md) | 2 | SOL-BE-TRACE-005 | `runtime/rpc/methods/git.ts` | Instrument `git.diff`/`generateCommitMessage`/`generatePullRequestFields` (local) | ✅ Done |
| [TASK-BE-005.3](./TASK-BE-005.3-git-remote-diff-commit-pr-tracing.md) | 2 | SOL-BE-TRACE-005 | `runtime/rpc/methods/git-remote.ts` | Instrument tương ứng bản remote, forward `traceId` vào `relay.call()`; DRIFT: `traceId` field already existed on `ProjectWorktreeParam` from TASK-BE-001.1, no schema change needed | ✅ Done |
| [TASK-BE-005.4](./TASK-BE-005.4-code-review-tracing-tests.md) | 2 | SOL-BE-TRACE-005 | 2 test file mới | ≥13 Vitest case + regression check annotate/feedback không có call site — delivered 19 (10+9) | ✅ Done |
| [TASK-BE-013.1](./TASK-BE-013.1-agent-ws-tracers.md) | 2 | SOL-BE-TRACE-013 | `tracers.ts` | Đăng ký 2 tracer `agentWsHandshakeFlow`, `agentWsTokenVerifyFlow` | ✅ Done |
| [TASK-BE-013.2](./TASK-BE-013.2-relay-bridge-handshake-tracing.md) | 2 | SOL-BE-TRACE-013 | `dev-server-relay-bridge.ts` | Instrument `connectRelayWebSocket()`/`attempt()`, mỗi reconnect = span độc lập | ✅ Done |
| [TASK-BE-013.3](./TASK-BE-013.3-ws-server-token-verify-and-token-routes.md) | 2 | SOL-BE-TRACE-013 | `agent-ws-server.ts`, `agent-token-routes.ts` | Instrument `handleConnection()`, **fix bug span mồ côi** (mở span tại socket-upgrade thay vì sau handshake resolve) | ✅ Done |
| [TASK-BE-013.4](./TASK-BE-013.4-agent-ws-tracing-tests.md) | 2 | SOL-BE-TRACE-013 | 3 test file mới | ≥12 Vitest case, kèm assertion trực tiếp cho fix bug span mồ côi — delivered 13 (6+5+2) | ✅ Done |
| [TASK-BE-014.1](./TASK-BE-014.1-remote-integration-tracers.md) | 2 | SOL-BE-TRACE-014 | `tracers.ts` | Đăng ký 3 tracer `remoteIntegration:*` (loại trừ `ghExec` — thuộc agent-side) | ✅ Done |
| [TASK-BE-014.2](./TASK-BE-014.2-git-provider-credential-decrypt-tracing.md) | 2 | SOL-BE-TRACE-014 | `GitProviderCredentialService.ts` | Instrument `getGitHubPAT()`/`getGitLabPAT()`, không log plaintext token | ✅ Done |
| [TASK-BE-014.3](./TASK-BE-014.3-credentials-and-preflight-tracing.md) | 2 | SOL-BE-TRACE-014 | `runtime/rpc/methods/credentials.ts`, `preflight.ts` | Instrument `credentials.set/revoke`, `preflight.check`; dùng đúng singleton `getWebCredentialStore()` (sửa giả định sai của CR gốc) | ✅ Done |
| [TASK-BE-014.4](./TASK-BE-014.4-remote-integration-tracing-tests.md) | 2 | SOL-BE-TRACE-014 | 3 test file mới | ≥18 Vitest case, tất cả kèm assertion không leak secret | ✅ Done (21 tests) |
| [TASK-BE-015.1](./TASK-BE-015.1-tracers-and-profile-rpc-handler.md) | 3 | SOL-BE-TRACE-015 | `tracers.ts`, `profile-rpc-handler.ts` | Đăng ký 4 tracer `profile:*` + instrument 4 write handler dưới `profile:updateLayer` | ✅ Done |
| [TASK-BE-015.2](./TASK-BE-015.2-profile-resolver.md) | 3 | SOL-BE-TRACE-015 | `ProfileResolver.ts` | Instrument `resolve()` — phân biệt cache hit/miss, KHÔNG step cho `merge()` nội bộ | ✅ Done |
| [TASK-BE-015.3](./TASK-BE-015.3-project-service-and-router.md) | 3 | SOL-BE-TRACE-015 | `ProjectService.ts`, `ProjectServerRouter.ts` | Instrument `create()` + `getRelayForProject()` dưới chung 1 tracer `profile:projectRoute` | ✅ Done |
| [TASK-BE-015.4](./TASK-BE-015.4-profile-aware-agent-spawner.md) | 3 | SOL-BE-TRACE-015 | `project-rpc-handler.ts` (✅ Known Conflicts resolved — không còn sửa `ProfileAwareAgentSpawner.ts`) | `profile:agentSpawnRoute` bọc `assertAccess` prep TRƯỚC `spawn()`, forward id để `agentOrch:spawn` resume; phụ thuộc TASK-BE-002.2 + TASK-BE-002.3 | ✅ Done |
| [TASK-BE-015.5](./TASK-BE-015.5-profile-tracing-tests.md) | 3 | SOL-BE-TRACE-015 | test files | ≥21 Vitest case cho profile tracing | ✅ Done |
| [TASK-BE-016.1](./TASK-BE-016.1-tracers-and-ai-provider-service.md) | 3 | SOL-BE-TRACE-016 | `tracers.ts`, `AIProviderService.ts`, `ai-provider-rpc-handler.ts` | Đăng ký 3 tracer + instrument `writeCredentialToDevServer()`, không log credential | ✅ Done |
| [TASK-BE-016.2](./TASK-BE-016.2-provider-resolver.md) | 3 | SOL-BE-TRACE-016 | `ProviderResolver.ts` | Instrument `resolve()` — thuật toán 2-pass scope-priority thật (sửa pseudo-code CR gốc) | ✅ Done |
| [TASK-BE-016.3](./TASK-BE-016.3-provider-health-checker.md) | 3 | SOL-BE-TRACE-016 | `ProviderHealthChecker.ts` | Instrument `runCheck()` — 3-way status classification thật + `onStatusChanged` callback | ✅ Done |
| [TASK-BE-016.4](./TASK-BE-016.4-ai-provider-tracing-tests.md) | 3 | SOL-BE-TRACE-016 | test files | ≥16 Vitest case, kèm test bảo mật chống leak credential | ✅ Done (19 tests) |
| [TASK-BE-017.1](./TASK-BE-017.1-migration-workflow-trace-correlation.md) | 3 | SOL-BE-TRACE-017 | `db/migrations/0013_workflow_trace_correlation.ts`, `migrations/index.ts` | Migration mới — cột `root_trace_id`, **prerequisite của TASK-BE-018.4** | ✅ Done |
| [TASK-BE-017.2](./TASK-BE-017.2-tracers-and-workflow-orchestrator.md) | 3 | SOL-BE-TRACE-017 | `tracers.ts`, `WorkflowOrchestrator.ts` | Core thiết kế `parentTraceId`: 4 tracer + `execute/resumeRunningExecutions/runExecution/executeStep` | ✅ Done |
| [TASK-BE-017.3](./TASK-BE-017.3-step-executors-trace-forwarding.md) | 3 | SOL-BE-TRACE-017 | `StepExecutors.ts` | Forward `traceId` vào `relay.call()` cho `executeAgent/executeShell/executeNotification` | ✅ Done |
| [TASK-BE-017.4](./TASK-BE-017.4-template-resolver-and-rpc-handler.md) | 3 | SOL-BE-TRACE-017 | `TemplateResolver.ts`, `workflow-rpc-handler.ts` | Instrument `TemplateResolver.create()` + wire `traceId` qua `workflow.execute` RPC | ✅ Done |
| [TASK-BE-017.5](./TASK-BE-017.5-workflow-tracing-tests.md) | 3 | SOL-BE-TRACE-017 | test files | ≥17 Vitest case, kèm test restart-survival cho `parentTraceId` | ✅ Done (35 tests) |
| [TASK-BE-018.1](./TASK-BE-018.1-tracers-and-task-service-add-edge.md) | 3 | SOL-BE-TRACE-018 | `tracers.ts`, `TaskService.ts`, `task-rpc-handler.ts` (schema) | Đăng ký 4 tracer + instrument `addEdge()`, sửa BFS→DFS cho `wouldCreateCycle()` | ✅ Done |
| [TASK-BE-018.2](./TASK-BE-018.2-task-ai-planner.md) | 3 | SOL-BE-TRACE-018 | `TaskAIPlanner.ts`, `task-rpc-handler.ts` (schema) | Instrument `decompose()`, tách rõ latency AI-call vs parse failure | ✅ Done |
| [TASK-BE-018.3](./TASK-BE-018.3-task-grant-service.md) | 3 | SOL-BE-TRACE-018 | `TaskGrantService.ts` | Instrument `resolvePermission()` — 1 step tổng kết, tránh noise hot path | ✅ Done |
| [TASK-BE-018.4](./TASK-BE-018.4-agent-spawn-options-trace-resume.md) | 3 | SOL-BE-TRACE-018 | (không sửa file — verify only, ✅ Known Conflicts resolved) | Xác nhận `AgentSpawnOptions.traceId` + resume logic (đã do TASK-BE-002.2 implement) đáp ứng nhu cầu Task Graph; DRIFT: field tạm thời biến mất khỏi `ProfileAwareAgentSpawner.ts` giữa phiên do hoạt động đồng thời không liên quan, xem Status trong task file | ✅ Done |
| [TASK-BE-018.5](./TASK-BE-018.5-task-agent-executor.md) | 3 | SOL-BE-TRACE-018 | `TaskAgentExecutor.ts`, `task-rpc-handler.ts` (schema) | `executeTask()` sở hữu span `taskGraph:execute`, resume vào `agentOrch:spawn` (✅ Known Conflicts resolved, không qua `profile:agentSpawnRoute`); phụ thuộc TASK-BE-002.2 | ✅ Done |
| [TASK-BE-018.6](./TASK-BE-018.6-task-graph-tracing-tests.md) | 3 | SOL-BE-TRACE-018 | test files | ≥16 Vitest case cho task graph tracing — delivered 16 (2+4+4+6), xem Status trong task file cho drift về `ProfileAwareAgentSpawner.test.ts` | ✅ Done |

---

## Execution Rules for AI Agent

1. **Dependency order**: Phase 0 → 1/2 (song song được, cả 2 chỉ phụ thuộc Phase 0) → 3, không skip. Trong Phase 3, `TASK-BE-017.1` (migration) phải chạy trước `TASK-BE-018.4`. Riêng cụm agent-spawn (`TASK-BE-002.2`/`TASK-BE-002.3`/`TASK-BE-015.4`/`TASK-BE-018.4`/`TASK-BE-018.5`), xem rule 2.
2. **✅ Known Conflicts đã resolve — thứ tự bắt buộc cho cụm `ProfileAwareAgentSpawner.ts`/`TaskAgentExecutor.ts`:** `TASK-BE-002.2` → `TASK-BE-002.3` → sau đó `TASK-BE-015.4` và `TASK-BE-018.4`/`TASK-BE-018.5` có thể chạy song song với nhau (cả hai đều phụ thuộc `TASK-BE-002.2`/`TASK-BE-002.3` vì chúng resume vào span `agentOrch:spawn` mà 2 task đó tạo ra). Xem mục "Known Conflicts — Resolved" ở trên để biết chi tiết thiết kế.
3. **Test-first hoặc song song với implementation** — mỗi CR có 1 task test riêng (`.4`/`.5`/`.6` cuối cùng của CR đó); có thể viết test trước hoặc song song với code, miễn pass trước khi đóng task.
4. **Additive only** — không xoá/sửa business logic hiện tại ngoài các file được liệt kê trong từng task; chỉ thêm tracer call + (riêng CR-017) 1 migration mới `0013`.
5. Chạy `pnpm run typecheck:node` (hoặc `pnpm tsc --noEmit` theo từng task) sau mỗi task.
6. Chạy `pnpm test --run <file>` cho từng test file liên quan sau khi implement.
7. **Không log giá trị secret** (token, API key đã decrypt, credential) vào `TraceFields` — áp dụng nghiêm ngặt cho `TASK-BE-014.2`, `TASK-BE-014.3`, `TASK-BE-016.1` và bất kỳ task nào chạm `profileEnv`/`env` trong `ProfileAwareAgentSpawner.spawn()` (`TASK-BE-002.2`, `TASK-BE-015.4`).
8. **`traceId` dual-field tại `DevServerRelayBridge`** — khi `relay.call()` target là Agent WS (`agent.exec`), PHẢI gửi cả `traceId` (phẳng) VÀ `_trace: { id }` (lồng); các `relay.call()` khác chỉ cần `traceId` phẳng theo CR-TRACE-000 §3.3. Xem `TASK-BE-002.2`, `TASK-BE-001.3`.
9. **Nguyên tắc "khi nào `step()`"** (CR-TRACE-000 §5) — không tạo `step()` cho SELECT/INSERT/UPDATE đơn dòng hay biến đổi in-memory thuần tuý (`buildPrompt()`, `merge()`...); mọi task đã tự áp dụng nguyên tắc này, không thêm step ngoài phạm vi task.
10. Sau khi hoàn tất một CR, chạy `gitnexus_detect_changes()` (theo quy tắc CLAUDE.md của repo) để verify phạm vi thay đổi khớp danh sách file trong Quick Reference bên dưới trước khi commit.
11. **Bắt buộc `codegraph explore` trước khi đọc code bằng Read/grep** — trước khi tìm hiểu bất kỳ symbol/hàm/class nào nằm trong danh sách file của từng task, chạy `codegraph explore "<TênSymbol>"` (CLI) hoặc tool MCP `codegraph_explore` trước; KHÔNG dùng `cat`/`Read`/grep chung chung để "đọc hiểu" 1 hàm khi CodeGraph có thể trả lời trong 1 lần gọi (theo CLAUDE.md của repo — Token Optimization Rules).
12. **Bắt buộc `gitnexus_impact` trước khi sửa symbol đã tồn tại, và `gitnexus_detect_changes()` trước khi đóng task** — với mọi symbol thuộc case MODIFY (không áp dụng cho file/symbol hoàn toàn mới), chạy `gitnexus_impact({ target: "<TênSymbol>", direction: "upstream" })` và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) cho người dùng trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục. Sau khi sửa xong, chạy `gitnexus_detect_changes()` để xác nhận phạm vi thay đổi thực tế khớp kỳ vọng, trước khi đánh dấu task DONE. Xem `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` để biết chi tiết quy trình.

---

## Quick Reference — Modified Files Only

**File hiện có bị sửa (MODIFY), theo task đóng góp:**

```
src/shared/trace/tracers.ts                                       — TASK-BE-001.1, 002.1, 003.1, 005.1, 013.1, 014.1, 015.1, 016.1, 017.2, 018.1
src/main/runtime/rpc/methods/worktree-schemas.ts                  — TASK-BE-001.1
src/main/runtime/rpc/methods/git-remote.ts                        — TASK-BE-001.1 (schema), 001.3, 005.3
src/main/runtime/rpc/methods/worktree.ts                          — TASK-BE-001.2
src/main/dev-server/dev-server-relay-bridge.ts                    — TASK-BE-001.3, 013.2
src/main/project/ProfileAwareAgentSpawner.ts                      — TASK-BE-002.2 (✅ span canonical agentOrch:spawn — resolved, xem trên; 015.4/018.4 KHÔNG còn sửa file này)
src/main/project/project-rpc-handler.ts                           — TASK-BE-002.3, 015.4 (✅ resolved — 015.4 nay có code thật, bọc profile:agentSpawnRoute quanh assertAccess, không còn note-only)
src/main/task/TaskAgentExecutor.ts                                — TASK-BE-002.3, 018.5 (✅ resolved, xem trên — taskGraph:execute là span bổ sung hợp lệ, không trùng agentOrch:spawn)
src/main/task/task-rpc-handler.ts                                 — TASK-BE-002.3, 018.1 (schema), 018.2 (schema), 018.5 (schema)
src/main/runtime/rpc/methods/terminal.ts                          — TASK-BE-003.1
src/main/providers/local-pty-provider.ts                          — TASK-BE-003.2
src/main/providers/ssh-pty-provider.ts                            — TASK-BE-003.2
src/main/terminal-scrollback-snapshots.ts                         — TASK-BE-003.3
src/main/ipc/session.ts                                           — TASK-BE-003.3
src/main/runtime/rpc/methods/git.ts                                — TASK-BE-005.2
src/main/dev-server/agent-ws-server.ts                            — TASK-BE-013.3
src/server/agent-token-routes.ts                                  — TASK-BE-013.3
src/main/project/GitProviderCredentialService.ts                  — TASK-BE-014.2
src/main/runtime/rpc/methods/credentials.ts                       — TASK-BE-014.3
src/main/runtime/rpc/methods/preflight.ts                         — TASK-BE-014.3
src/main/profile/profile-rpc-handler.ts                           — TASK-BE-015.1
src/main/profile/ProfileResolver.ts                                — TASK-BE-015.2
src/main/project/ProjectService.ts                                — TASK-BE-015.3
src/main/project/ProjectServerRouter.ts                           — TASK-BE-015.3
src/main/ai-providers/AIProviderService.ts                        — TASK-BE-016.1
src/main/ai-providers/ai-provider-rpc-handler.ts                  — TASK-BE-016.1
src/main/ai-providers/ProviderResolver.ts                         — TASK-BE-016.2
src/main/ai-providers/ProviderHealthChecker.ts                    — TASK-BE-016.3
src/main/db/migrations/index.ts                                   — TASK-BE-017.1
src/main/workflow/WorkflowOrchestrator.ts                         — TASK-BE-017.2
src/main/workflow/StepExecutors.ts                                — TASK-BE-017.3
src/main/workflow/TemplateResolver.ts                              — TASK-BE-017.4
src/main/workflow/workflow-rpc-handler.ts                          — TASK-BE-017.4
src/main/task/TaskService.ts                                      — TASK-BE-018.1
src/main/task/TaskAIPlanner.ts                                    — TASK-BE-018.2
src/main/task/TaskGrantService.ts                                 — TASK-BE-018.3
```

**Core API (Phase 0):**

```
src/shared/trace/index.ts                                         — TASK-BE-000 [MODIFY]
```

**File mới (NEW):**

```
src/shared/trace/index.test.ts                                    — TASK-BE-000
src/main/db/migrations/0013_workflow_trace_correlation.ts          — TASK-BE-017.1
src/main/runtime/rpc/methods/__tests__/git-remote-tracing.test.ts  — TASK-BE-005.4
src/main/runtime/rpc/methods/__tests__/git-local-tracing.test.ts   — TASK-BE-005.4
src/main/dev-server/__tests__/dev-server-relay-bridge-tracing.test.ts — TASK-BE-013.4
src/main/dev-server/__tests__/agent-ws-server-tracing.test.ts      — TASK-BE-013.4
src/server/__tests__/agent-token-routes-tracing.test.ts            — TASK-BE-013.4
src/main/project/__tests__/GitProviderCredentialService-tracing.test.ts — TASK-BE-014.4
src/main/runtime/rpc/methods/__tests__/credentials-tracing.test.ts — TASK-BE-014.4
src/main/runtime/rpc/methods/__tests__/preflight-tracing.test.ts   — TASK-BE-014.4
src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts        — TASK-BE-002.4
(+ các test file khác đánh dấu "MODIFY hoặc NEW nếu chưa tồn tại" trong TASK-BE-001.4/003.4/015.5/016.4/017.5/018.6 — kiểm tra sự tồn tại trước khi chạy task đó)
```

Xem từng task file (`TASK-BE-0XX.N-*.md`) để biết danh sách test file đầy đủ theo CR — mục "Quick Reference" ở đây chỉ liệt kê các file production code + file test đã xác định chắc chắn là NEW.
