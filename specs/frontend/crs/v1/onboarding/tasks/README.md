# Frontend Tasks — Onboarding CR v1

Thư mục này chứa các **AI-executable tasks** được chia nhỏ từ các solution trong [`../solutions/`](../solutions/).

Mỗi file task mô tả **1 unit công việc nguyên tử** đủ nhỏ để AI thực thi trong 1 lần.

---

## Tổ chức theo Phase

### Phase 1 — Foundation (DevServer slice + UI)
| Task | Solution | Mô tả ngắn |
|------|----------|-----------|
| [TASK-FE-001](./TASK-FE-001-dev-server-types.md) | FE-SOL-A | Tạo `dev-server-types.ts` shared types |
| [TASK-FE-002](./TASK-FE-002-dev-server-slice.md) | FE-SOL-A | Tạo Zustand `dev-servers` slice + selectors |
| [TASK-FE-003](./TASK-FE-003-dev-server-sync-hook.md) | FE-SOL-A | Tạo `useDevServersSync` hook + IPC subscription |
| [TASK-FE-004](./TASK-FE-004-add-dev-server-hook.md) | FE-SOL-A | Tạo `useAddDevServer` hook |
| [TASK-FE-005](./TASK-FE-005-dev-server-status-badge.md) | FE-SOL-A | Tạo `DevServerStatusBadge` component |
| [TASK-FE-006](./TASK-FE-006-dev-server-step.md) | FE-SOL-A | Tạo `DevServerStep.tsx` wizard component |
| [TASK-FE-007](./TASK-FE-007-preload-bridge-devserver.md) | FE-SOL-A | Extend `window.api.devServer` preload bridge |
| [TASK-FE-008](./TASK-FE-008-onboarding-flow-devserver.md) | FE-SOL-A | Sửa `use-onboarding-flow.ts` thêm `dev_server` step |

### Phase 2 — Agent + Platform Wizard
| Task | Solution | Mô tả ngắn |
|------|----------|-----------|
| [TASK-FE-009](./TASK-FE-009-remote-agent-detection-hook.md) | FE-SOL-B | Tạo `useRemoteAgentDetection` hook với cache |
| [TASK-FE-010](./TASK-FE-010-platform-hooks.md) | FE-SOL-B | Tạo `useActiveDevServerPlatform` + visibility hooks |
| [TASK-FE-011](./TASK-FE-011-agent-catalog-platform.md) | FE-SOL-B | Sửa agent catalog: thêm platform filter |
| [TASK-FE-012](./TASK-FE-012-agent-step-remote.md) | FE-SOL-B | Sửa `AgentStep.tsx`: remote detection + multi-server |
| [TASK-FE-013](./TASK-FE-013-theme-step-remote.md) | FE-SOL-B | Sửa `ThemeStep.tsx`: Ghostty remote detect |
| [TASK-FE-014](./TASK-FE-014-preflight-slice.md) | FE-SOL-C | Sửa Preflight slice: thêm `remotePreflightByServer` |
| [TASK-FE-015](./TASK-FE-015-remote-preflight-hook.md) | FE-SOL-C | Tạo `useRemotePreflightStatus` hook |
| [TASK-FE-016](./TASK-FE-016-integrations-step-remote.md) | FE-SOL-C | Sửa `IntegrationsStep.tsx`: remote preflight + remote PTY |
| [TASK-FE-017](./TASK-FE-017-git-identity-card.md) | FE-SOL-C | Tạo `GitIdentityCard.tsx` component |
| [TASK-FE-018](./TASK-FE-018-remote-directory-browser.md) | FE-SOL-C | Tạo `useRemoteDirectoryBrowser` hook + `RemoteDirectoryBrowser` component |
| [TASK-FE-019](./TASK-FE-019-add-repo-step.md) | FE-SOL-C | Tạo `AddRepoStep.tsx`: browse/clone/scan modes |
| [TASK-FE-020](./TASK-FE-020-preload-bridge-repo-onboarding.md) | FE-SOL-C | Extend `window.api.repo.*` + `window.api.onboarding.*` |

### Phase 3 — Polish
| Task | Solution | Mô tả ngắn |
|------|----------|-----------|
| [TASK-FE-021](./TASK-FE-021-windows-caps-hook.md) | FE-SOL-D | Tạo `useRemoteWindowsTerminalCapabilities` hook |
| [TASK-FE-022](./TASK-FE-022-windows-terminal-step.md) | FE-SOL-D | Sửa `WindowsTerminalStep.tsx`: remote caps + per-server settings |
| [TASK-FE-023](./TASK-FE-023-browser-notification-hook.md) | FE-SOL-D | Tạo `useBrowserNotificationPermission` + `useWebPushSubscription` hooks |
| [TASK-FE-024](./TASK-FE-024-notification-step-web.md) | FE-SOL-D | Sửa `NotificationStep.tsx`: web mode UI |
| [TASK-FE-025](./TASK-FE-025-service-worker.md) | FE-SOL-D | Tạo `service-worker.js` + đăng ký trong bootstrap |
| [TASK-FE-026](./TASK-FE-026-checklist-slice.md) | FE-SOL-D | Sửa Onboarding slice: `perServer` checklist + actions |
| [TASK-FE-027](./TASK-FE-027-setup-guide-multiserver.md) | FE-SOL-D | Sửa `SetupGuideModal.tsx`: grouped per-server UI |
| [TASK-FE-028](./TASK-FE-028-checklist-triggers.md) | FE-SOL-D | Thêm checklist triggers vào IPC event handlers |

---

## Quy ước viết task

Mỗi task file bao gồm:
- **Context**: source files cần đọc trước
- **Goal**: kết quả cần đạt
- **Steps**: các bước thực hiện cụ thể
- **Acceptance criteria**: test để verify
- **Output files**: files tạo mới hoặc sửa đổi
