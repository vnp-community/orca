// ─── Pre-built Tracers ────────────────────────────────────────────────────────
// Named tracer instances for all built-in Orca flows.
// Import from this file instead of calling createTracer() directly to keep
// flow names consistent across the codebase.
//
// External projects should call createTracer() with their own flow names.

import { createTracer } from './index'

export const Tracers = {
  /** Browser → RPC → IPC → Relay → Agent: directory browse */
  browseDirFlow: createTracer('devServer:browseDir'),
  /** Browser → RPC → IPC → Relay → Agent: mkdir */
  mkdirFlow:     createTracer('devServer:mkdir'),
  /** Browser → RPC → IPC → Relay → Agent: rmdir */
  rmdirFlow:     createTracer('devServer:rmdir'),
  /** Agent WebSocket lifecycle (connect / disconnect) */
  agentWsFlow:   createTracer('agentWs:lifecycle'),
  /** IPC proxy call from user-process to main-process */
  ipcProxyFlow:  createTracer('ipc:devServerProxy'),
  /** DiffViewer: load diff content (staged or unstaged) */
  codeReviewDiffFlow:      createTracer('ui:codeReview.diff'),
  /** CodeReviewPanel: annotate a diff line */
  codeReviewAnnotateFlow:  createTracer('ui:codeReview.annotate'),
  /** CodeReviewPanel: send review feedback */
  codeReviewFeedbackFlow:  createTracer('ui:codeReview.sendFeedback'),
  /** CodeReviewPanel: generate AI commit message */
  codeReviewAiCommitFlow:  createTracer('ui:codeReview.aiCommitMessage'),
  /** CodeReviewPanel: create pull request */
  codeReviewCreatePrFlow:  createTracer('ui:codeReview.createPr'),

  // ─── CR-TRACE-003: Terminal Management (agent-side PTY only) ────────────────
  /** BL-TM-01/02 — create PTY (agent-PTY pty.create) */
  terminalCreate:   createTracer('terminal:create'),
  /** BL-TM-02 — resize PTY */
  terminalResize:   createTracer('terminal:resize'),
  /** BL-TM-03 — destroy/save-scrollback PTY */
  terminalDestroy:  createTracer('terminal:destroy'),
  /** Reconnect to a still-running PTY after a WebSocket reconnect (pty.attach) */
  terminalReattach: createTracer('terminal:reattach'),

  // ─── CR-TRACE-001: Worktree Management (agent/backend-side, shared flow names) ─
  /** BL-WT-01 — create worktree (RPC handler + Agent git.worktree.add) */
  worktreeCreate: createTracer('worktree:create'),
  /** BL-WT-02 — fan out worktree to multiple agents — reserved, chưa có RPC method thật */
  worktreeFanOut: createTracer('worktree:fanOut'),
  /** BL-WT-03 — delete worktree (RPC handler + Agent git.worktree.remove) */
  worktreeDelete: createTracer('worktree:delete'),
  /** BL-WT-04 — compare worktree branches — reserved, chưa có RPC method thật */
  worktreeCompare: createTracer('worktree:compare'),
  /** BL-WT-05 — merge worktree branch — reserved, chưa có RPC method thật */
  worktreeMerge: createTracer('worktree:merge'),

  // ─── CR-TRACE-002: Agent Orchestration ──────────────────────────────────────
  /** BL-AG-01 — spawn AI agent (agent.exec / agent.spawn) */
  agentOrchSpawn:      createTracer('agentOrch:spawn'),
  /** BL-AG-02 — stop agent (agent.kill / agent.sendInput Ctrl+C) */
  agentOrchStop:       createTracer('agentOrch:stop'),
  /** BL-AG-03 — resume agent session (agent.spawn với resumeId) */
  agentOrchResume:     createTracer('agentOrch:resume'),
  /** BL-AG-04 — switch account/provider (chưa có call site thật, đặt tên trước) */
  agentOrchSwitch:     createTracer('agentOrch:switch'),
  /** BL-AG-05 — polling loop rời rạc (KHÔNG dùng cho agent.output stream) */
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'),

  // --- ui:* — tracer khởi tạo từ browser/renderer (CR-TRACE-015/016/017/018) ---

  /** Browser-initiated: mount ProfileEditor → fetch resolved + user profile (SOL-FE-TRACE-015 BL-PRF-02) */
  uiProfileResolveFlow: createTracer('ui:profile.resolve'),
  /** Browser-initiated: click "Save Changes" trong ProfileEditor (SOL-FE-TRACE-015 BL-PRF-01) */
  uiProfileUpdateFlow: createTracer('ui:profile.update'),

  /** Browser-initiated: click "Save" trong ProviderForm khi có credential mới (SOL-FE-TRACE-016 BL-AIP-01) */
  uiAiProviderWriteCredFlow: createTracer('ui:aiProvider.writeCredential'),
  /** Browser-initiated: click "Test" trên 1 provider account (SOL-FE-TRACE-016 BL-AIP-03) */
  uiAiProviderTestConnFlow: createTracer('ui:aiProvider.testConnection'),

  /** Browser-initiated: click "Save" trong WorkflowBuilder (SOL-FE-TRACE-017 BL-WF-01) */
  uiWorkflowTemplateSaveFlow: createTracer('ui:workflow.templateSave'),
  /** Browser-initiated: click "Run" — root span của execution nhìn từ browser (SOL-FE-TRACE-017 BL-WF-02) */
  uiWorkflowExecuteFlow: createTracer('ui:workflow.execute'),
  /** Browser-initiated: click "Cancel" trên execution đang chạy (SOL-FE-TRACE-017) */
  uiWorkflowCancelFlow: createTracer('ui:workflow.cancel'),

  /** Browser-initiated: click "Decompose with AI" trong TaskAIDecompose (SOL-FE-TRACE-018 BL-TG-02) */
  uiTaskGraphAiPlanFlow: createTracer('ui:taskGraph.aiPlan'),
  /** Browser-initiated: click "Execute/Run with Agent" — dùng chung bởi TaskDetail + TaskPromptEditor (SOL-FE-TRACE-018 BL-TG-04) */
  uiTaskGraphExecuteFlow: createTracer('ui:taskGraph.execute'),

  /** Browser-initiated: "New Worktree" dialog submit (SOL-FE-TRACE-001 BL-WT-01) — distinct
   *  from agent/backend-side `worktreeCreate` (`worktree:create`) so TracePanel's `isBackend`
   *  heuristic doesn't mislabel a browser-originated span as a backend event. */
  uiWorktreeCreateFlow: createTracer('ui:worktree.create'),
  /** Browser-initiated: "Delete Worktree" confirm (SOL-FE-TRACE-001 BL-WT-03) — distinct from
   *  agent/backend-side `worktreeDelete` (`worktree:delete`), same reasoning as above. */
  uiWorktreeDeleteFlow: createTracer('ui:worktree.delete'),
  /** BL-WT-02 — fan out worktree to multiple agents — reserved, chưa có call site */
  uiWorktreeFanOutFlow: createTracer('ui:worktree.fanOut'),
  /** BL-WT-04 — compare worktree branches — reserved, chưa có call site */
  uiWorktreeCompareFlow: createTracer('ui:worktree.compare'),
  /** BL-WT-05 — merge worktree branch — reserved, chưa có call site */
  uiWorktreeMergeFlow: createTracer('ui:worktree.merge'),

  // ─── CR-TRACE-002: Agent Orchestration (renderer-initiated, ui: prefix) ─────
  // Why: distinct from agentOrchSpawn/Stop/Resume/Switch/StatusPoll above
  // (agent-domain-side, shared non-prefixed flow names) — same isBackend
  // mislabeling concern as the worktree ui:* entries above.
  /** BL-AG-01 — spawn AI agent (AgentPanel.tsx start, orphan component — not mounted) */
  uiAgentOrchSpawnFlow: createTracer('ui:agentOrch.spawn'),
  /** BL-AG-02 — stop agent (AgentPanel.tsx stop, orphan component — not mounted) */
  uiAgentOrchStopFlow: createTracer('ui:agentOrch.stop'),
  /** BL-AG-03 — resume agent session (AgentPanel.tsx resume, orphan component — not mounted) */
  uiAgentOrchResumeFlow: createTracer('ui:agentOrch.resume'),
  /** BL-AG-04 — switch account/provider — chưa có UI, đặt tên sẵn */
  uiAgentOrchSwitchFlow: createTracer('ui:agentOrch.switch'),
  /** BL-AG-05 — polling loop rời rạc — dự phòng, không dùng làm span riêng (xem TASK-FE-002.3, dùng chung span mở) */
  uiAgentOrchStatusPollFlow: createTracer('ui:agentOrch.statusPoll'),

  // ─── CR-TRACE-014: Remote Integration (Backend-side only) ─────────────────
  /** BL-INT-01 (phần Main): đọc + giải mã PAT cho gh/glab trước khi Dev Server
   *  dùng để build env cho CLI. KHÔNG bao gồm bước gh/glab auth status thật —
   *  đó là remoteIntegration:ghExec, chạy trên Dev Server (companion solution). */
  remoteIntegrationCredentialDecryptFlow: createTracer('remoteIntegration:credentialDecrypt'),
  /** BL-INT-02: store/revoke token qua credentials.set/credentials.revoke RPC */
  remoteIntegrationCredentialStoreFlow:   createTracer('remoteIntegration:credentialStore'),
  /** BL-INT-03: preflight check (local host hoặc relay-delegated) */
  remoteIntegrationPreflightFlow:         createTracer('remoteIntegration:preflight'),

  // ─── CR-TRACE-014: Remote Integration (renderer-initiated, ui: prefix) ─────
  // Why: TASK-FE-014.1/014.2 originally proposed bare `remoteIntegrationPreflightFlow`/
  // `remoteIntegrationCredentialStoreFlow` keys, but a concurrent backend task already
  // claimed those exact key names above (`remoteIntegration:preflight`/
  // `remoteIntegration:credentialStore`). Per the no-rename collision rule, the
  // renderer-initiated `ui:*` variants use the `ui` prefix — same pattern as
  // `uiWorktreeCreateFlow`/`uiAgentOrchSpawnFlow`/`uiTerminalCreateFlow` above.
  /** BL-INT-01 + BL-INT-03: click "Re-check" → refreshPreflightStatus({ force: true }) —
   *  single shared renderer entry point (usePreflightCardStatuses + auto triggers). */
  uiRemoteIntegrationPreflightFlow:       createTracer('ui:remoteIntegration.preflight'),
  /** BL-INT-02: CredentialInputForm.tsx handleSave/handleRevoke — orphan component,
   *  not mounted yet (TASK-FE-014.2). */
  uiRemoteIntegrationCredentialStoreFlow: createTracer('ui:remoteIntegration.credentialStore'),

  // ─── CR-TRACE-005: Code Review (Backend-side, `codeReview:` prefix per
  // CR-TRACE-000 §4) — NOTE naming drift: the `codeReview*Flow` keys above
  // (`codeReviewDiffFlow`/`codeReviewAnnotateFlow`/`codeReviewFeedbackFlow`/
  // `codeReviewAiCommitFlow`/`codeReviewCreatePrFlow`) were already claimed by
  // a concurrent frontend task for browser-initiated `ui:codeReview.*` flows.
  // Per the no-rename collision rule, backend entries below use bare
  // (no-`Flow`-suffix) keys — matching the sibling backend convention
  // (`worktreeCreate`, `agentOrchSpawn`, `terminalCreate`) — instead of the
  // key names originally proposed in TASK-BE-005.1/SOL-BE-TRACE-005. ─────────
  /** BL-CR-01: xem diff của agent changes (local + remote) */
  codeReviewDiff:      createTracer('codeReview:diff'),
  /** BL-CR-02: annotate dòng code — KHÔNG wire vào code cho tới khi
   *  annotation.create RPC method + AgentManager.injectAnnotations() tồn tại
   *  (BUG-AG-ORCH-005). Khai báo trước theo CR-TRACE-000 §4 naming convention. */
  codeReviewAnnotate:  createTracer('codeReview:annotate'),
  /** BL-CR-03: gửi feedback về agent — KHÔNG wire vào code cho tới khi
   *  review.sendFeedback RPC method tồn tại (BUG-AG-ORCH-001). */
  codeReviewFeedback:  createTracer('codeReview:sendFeedback'),
  /** BL-CR-04: tạo commit message bằng AI (local + remote) */
  codeReviewAiCommit:  createTracer('codeReview:aiCommitMessage'),
  /** BL-CR-05: tạo Pull Request với AI (local + remote) */
  codeReviewCreatePr:  createTracer('codeReview:createPr'),

  // ─── CR-TRACE-013: Agent WebSocket (handshake/auth phase) ─────────────────
  /** BL-AWS-01: Orca initiator handshake (relay-websocket mode) — TCP connect
   *  + agent.handshake round-trip, TRƯỚC khi agentWs:lifecycle bắt đầu. */
  agentWsHandshakeFlow:   createTracer('agentWs:handshake'),
  /** BL-AWS-02: Orca receiver handshake + token validation (direct-websocket
   *  mode) — từ lúc socket upgrade tới accept/reject, TRƯỚC agentWs:lifecycle. */
  agentWsTokenVerifyFlow: createTracer('agentWs:tokenVerify'),

  // ─── CR-TRACE-003: Terminal Management (renderer-initiated, ui: prefix) ────
  // Why: distinct from terminalCreate/terminalResize/terminalDestroy/
  // terminalReattach above (agent-side PTY-only, shared non-prefixed flow
  // names) — same isBackend mislabeling concern as the other ui:* entries.
  /** BL-TM-01 — create PTY session (createRemoteRuntimePtyTransport connect()) */
  uiTerminalCreateFlow: createTracer('ui:terminal.create'),
  /** BL-TM-02 — resize/claim viewport */
  uiTerminalResizeFlow: createTracer('ui:terminal.resize'),
  /** BL-TM-03 — destroy PTY / save scrollback */
  uiTerminalDestroyFlow: createTracer('ui:terminal.destroy'),
  /** BL-TM-03 restore — reconnect a still-running PTY — chưa có call site rõ ràng, đặt tên sẵn */
  uiTerminalReconnectFlow: createTracer('ui:terminal.reconnect'),

  // ─── CR-TRACE-015: Profile & Project (Backend-side) ────────────────────────
  /** BL-PRF-01: update company/dept/user profile + cache invalidate */
  profileUpdateLayerFlow:  createTracer('profile:updateLayer'),
  /** BL-PRF-02: 3-layer resolve (cache hit/miss + merge, không trace merge() nội bộ) */
  profileResolveFlow:      createTracer('profile:resolve'),
  /** BL-PRF-03: project create + dev-server relay routing (field `op` phân biệt) */
  profileProjectRouteFlow: createTracer('profile:projectRoute'),
  /** BL-PRF-04: profile-aware agent spawn orchestration (assertAccess prep TRƯỚC
   *  ProfileAwareAgentSpawner.spawn(), resume vào agentOrch:spawn — CR-TRACE-002) */
  profileAgentSpawnFlow:   createTracer('profile:agentSpawnRoute'),

  // ─── CR-TRACE-016: AI Provider Management (Backend-side) ───────────────────
  /** BL-AIP-01: write encrypted credential to dev server via relay */
  aiProviderWriteCredFlow: createTracer('aiProvider:writeCredential'),
  /** BL-AIP-02: priority + quota resolution cho agent/workflow spawn */
  aiProviderResolveFlow:   createTracer('aiProvider:resolve'),
  /** BL-AIP-03: background health check cron (15 phút/lần) */
  aiProviderHealthFlow:    createTracer('aiProvider:healthCheck'),

  // ─── CR-TRACE-018: Task Graph (Backend-side) ────────────────────────────────
  /** BL-TG-01: add dependency edge — cycle detection (DFS thật, không phải BFS) là phần đáng trace nhất */
  taskGraphEdgeFlow:    createTracer('taskGraph:addEdge'),
  /** BL-TG-02: AI decompose — tách rõ "AI call chậm" vs "parse JSON lỗi" */
  taskGraphAiPlanFlow:  createTracer('taskGraph:aiPlan'),
  /** BL-TG-03: multi-level ancestor grant resolution — chạy trên mọi permission check */
  taskGraphGrantFlow:   createTracer('taskGraph:grantResolve'),
  /** BL-TG-04: task prompt → agent execution, resume vào agentOrch:spawn (CR-TRACE-002) */
  taskGraphExecuteFlow: createTracer('taskGraph:execute'),

  // ─── CR-TRACE-017: Workflow Orchestration (Backend-side) ───────────────────
  /** BL-WF-01: template create/inherit */
  workflowTemplateCreateFlow: createTracer('workflow:templateCreate'),
  /** BL-WF-02: span CHA — 1 per execution, sống suốt vòng đời execution */
  workflowExecuteFlow:        createTracer('workflow:execute'),
  /** BL-WF-02: span CON — 1 per step, mang field parentTraceId để group theo execution */
  workflowStepFlow:           createTracer('workflow:stepExecute'),
  /** BL-WF-03: PLACEHOLDER — chưa có implementation, TemplateResolver.ts không có
   *  updateVisibility()/share-token/shared route nào trong code hiện tại. Khai báo tên
   *  tracer để sẵn sàng khi tính năng sharing tồn tại, KHÔNG viết call site nào cho nó. */
  workflowShareFlow:          createTracer('workflow:share'),

  // ─── OrcaProject Sharing (Backend-side, cross-user read) ───────────────────
  /** orcaProjects.linkSourceProject/unlinkSourceProject/getProjectData — audit
   *  trail for the cross-user read path (orcaProjectId, actingUserId, ownerUserId,
   *  projectId). See docs/guides/terminal-workspace-project-devserver-architecture.md
   *  "Điểm cần thiết kế cẩn thận nhất". */
  orcaProjectSharingFlow: createTracer('orcaProject:sharing'),
} as const
