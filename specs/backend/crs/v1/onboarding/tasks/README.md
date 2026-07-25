# Tasks — Onboarding CR v1

Thư mục này chứa **AI-executable tasks** được phân tách từ các [Solutions](../solutions/).

Mỗi task được thiết kế để AI có thể thực thi độc lập với:
- Mục tiêu rõ ràng (1 file hoặc 1 thay đổi cụ thể)
- Input/Output xác định
- Acceptance criteria kiểm tra được

---

## Thứ tự thực thi

```
Phase 1 — Foundation:
  TASK-001  → dev-server-types.ts (schema)
  TASK-002  → types.ts: PersistedState + GlobalSettings
  TASK-003  → dev-server-store.ts
  TASK-004  → dev-server-manager.ts
  TASK-005  → dev-server-relay-bridge.ts (relay-ssh mode)
  TASK-006  → relay-handshake.ts: platform/arch/nodeVersion
  TASK-007  → dev-server-ipc.ts (IPC handlers)
  TASK-008  → server-bootstrap.ts: đăng ký DevServerManager + handlers
  TASK-009  → persistence.ts: migration devServers
  TASK-010  → tests: DevServerStore + DevServerManager

Phase 1 cont — Remote Agent Detection:
  TASK-011  → agent-detection-commands.ts
  TASK-012  → relay/preflight-handler.ts: platform trong detectAgents
  TASK-013  → dev-server-relay-bridge.ts: detectAgents + callWithTimeout
  TASK-014  → onboarding-ipc.ts: detectAgents + detectAgentsAllServers + cache
  TASK-015  → preload/bridge: window.api.onboarding
  TASK-016  → tests: onboarding-ipc detectAgents

Phase 2 — Remote Preflight & Wizard:
  TASK-017  → dev-server-ipc.ts: getPlatform + setActiveDevServer
  TASK-018  → relay/preflight-handler.ts: detectGhosttyConfig
  TASK-019  → dev-server-types.ts: RemotePreflightStatus type
  TASK-020  → relay/preflight-handler.ts: checkFullPreflight (gh + git)
  TASK-021  → relay/preflight-handler.ts: setGitIdentity
  TASK-022  → onboarding-ipc.ts: getPreflightStatus + setGitIdentity + openGhAuthTerminal
  TASK-023  → relay/fs-handler.ts: listDirectory + isGitRepo
  TASK-024  → relay/git-handler.ts: cloneRepo PTY
  TASK-025  → repo-ipc.ts: listRemoteDirectory, addRemote, cloneRemote, scanRemote
  TASK-026  → types.ts: Repo.devServerId field
  TASK-027  → tests: preflight + repo remote

Phase 3 — Windows, Push, Checklist:
  TASK-028  → relay/preflight-handler.ts: pwshVersion + gitBashPath
  TASK-029  → onboarding-ipc.ts: detectWindowsCapabilities + cache
  TASK-030  → types.ts: terminalWindowsConfigByServer
  TASK-031  → npm install web-push
  TASK-032  → web-push-manager.ts
  TASK-033  → types.ts: webPushSubscriptions + vapidKeys
  TASK-034  → push-api-routes.ts
  TASK-035  → server-bootstrap.ts + server/index.ts: tích hợp WebPushManager
  TASK-036  → orca-runtime.ts: trigger push khi agent task complete
  TASK-037  → types.ts: OnboardingChecklistState.perServer
  TASK-038  → persistence.ts: migration checklist v1→v2
  TASK-039  → onboarding-ipc.ts: markChecklistItem
  TASK-040  → feature-wall-setup-steps.ts: new steps + priority logic
  TASK-041  → tests: Phase 3 (Windows, Push, Checklist)
```

---

## Trạng thái

| Task | Mô tả | Phase | Status |
|------|-------|-------|--------|
| TASK-001 | Schema dev-server-types.ts | 1 | ✅ Done |
| TASK-002 | PersistedState + GlobalSettings | 1 | ✅ Done |
| TASK-003 | DevServerStore | 1 | ✅ Done |
| TASK-004 | DevServerManager | 1 | ✅ Done |
| TASK-005 | DevServerRelayBridge (relay-ssh) | 1 | ✅ Done |
| TASK-006 | Relay Handshake platform info | 1 | ✅ Done |
| TASK-007 | DevServer IPC handlers | 1 | ✅ Done |
| TASK-008 | server-bootstrap: register DevServer | 1 | ✅ Done |
| TASK-009 | Persistence migration devServers | 1 | ✅ Done |
| TASK-010 | Tests: Store + Manager | 1 | ✅ Done |
| TASK-011 | agent-detection-commands.ts | 1 | ✅ Done |
| TASK-012 | preflight-handler: platform in detectAgents | 1 | ✅ Done |
| TASK-013 | RelayBridge: detectAgents + timeout | 1 | ✅ Done |
| TASK-014 | onboarding-ipc: detectAgents + cache | 1 | ✅ Done |
| TASK-015 | Preload: window.api.onboarding | 1 | ✅ Done |
| TASK-016 | Tests: detectAgents IPC | 1 | ✅ Done |
| TASK-017 | DevServer IPC: getPlatform + setActive | 2 | ✅ Done |
| TASK-018 | Relay: detectGhosttyConfig | 2 | ✅ Done |
| TASK-019 | Type: RemotePreflightStatus | 2 | ✅ Done |
| TASK-020 | Relay: checkFullPreflight | 2 | ✅ Done |
| TASK-021 | Relay: setGitIdentity | 2 | ✅ Done |
| TASK-022 | onboarding-ipc: preflight handlers | 2 | ✅ Done |
| TASK-023 | Relay: fs.listDirectory | 2 | ✅ Done |
| TASK-024 | Relay: git.clone | 2 | ✅ Done |
| TASK-025 | repo-ipc: remote CRUD | 2 | ✅ Done |
| TASK-026 | Schema: Repo.devServerId | 2 | ✅ Done |
| TASK-027 | Tests: preflight + repo remote | 2 | ✅ Done |
| TASK-028 | Relay: pwshVersion + gitBashPath | 3 | ✅ Done |
| TASK-029 | onboarding-ipc: detectWindowsCapabilities | 3 | ✅ Done |
| TASK-030 | Types: terminalWindowsConfigByServer | 3 | ✅ Done |
| TASK-031 | npm install web-push | 3 | ✅ Done |
| TASK-032 | WebPushManager class | 3 | ✅ Done |
| TASK-033 | Schema: webPushSubscriptions + vapidKeys | 3 | ✅ Done |
| TASK-034 | push-api-routes.ts | 3 | ✅ Done |
| TASK-035 | server-bootstrap: WebPushManager integration | 3 | ✅ Done |
| TASK-036 | orca-runtime: push on agent complete | 3 | ✅ Done |
| TASK-037 | Schema: OnboardingChecklistState.perServer | 3 | ✅ Done |
| TASK-038 | Persistence: migration checklist v1→v2 | 3 | ✅ Done |
| TASK-039 | onboarding-ipc: markChecklistItem | 3 | ✅ Done |
| TASK-040 | feature-wall-setup-steps: new steps | 3 | ✅ Done |
| TASK-041 | Tests: Phase 3 | 3 | ✅ Done |
