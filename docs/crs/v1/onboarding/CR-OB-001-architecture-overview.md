# CR-OB-001 — Onboarding Architecture: Server Mode & Multi Dev-Server

| Field | Value |
|-------|-------|
| **CR ID** | CR-OB-001 |
| **Title** | Kiến trúc Onboarding mới: Orca chạy Web Server + Dev Servers đa platform |
| **Version** | v1 |
| **Status** | Implemented |
| **Priority** | Critical |
| **Author** | Architecture Team |
| **Date** | 2026-07-23 |

---

## 1. Bối cảnh & Động lực

### 1.1 Kiến trúc hiện tại

```
┌──────────────────────────────────────────────────┐
│              Electron App (local)                │
│  ┌─────────────┐   ┌──────────────┐             │
│  │ Main Process│   │  Renderer    │             │
│  │  (Node.js)  │◄─►│   (React)    │             │
│  └──────┬──────┘   └──────────────┘             │
│         │                                        │
│  Local PTY / CLI agents / git / gh               │
└──────────────────────────────────────────────────┘
```

- Orca là **Electron desktop app** chạy local
- Agent CLI (`claude`, `codex`...), `git`, `gh`, folder đều ở **cùng máy** với Orca
- Onboarding phát hiện agents trên **local PATH** của máy người dùng
- `preflightStatus` chạy trên **relay process** của local machine

### 1.2 Kiến trúc mới (Target)

```
┌─────────────────────────────────────────────────────────────┐
│                    ORCA WEB SERVER                          │
│  ┌──────────────┐   ┌─────────────────────────────────┐    │
│  │  HTTP Server │   │    OrcaRuntimeService (Node.js)  │    │
│  │  (web UI)    │   │    + OrcaRuntimeRpcServer (WS)   │    │
│  │  :6769       │   │    :6768                         │    │
│  └──────────────┘   └─────────────┬───────────────────┘    │
└─────────────────────────────────-─┼────────────────────────┘
                                    │ WebSocket RPC
            ┌───────────────────────┼───────────────────────┐
            │                       │                       │
    ┌───────▼──────┐    ┌──────────▼────────┐   ┌─────────▼────────┐
    │ Dev Server A │    │  Dev Server B     │   │  Dev Server C    │
    │  (macOS)     │    │  (Windows)        │   │  (Linux)         │
    │              │    │                   │   │                  │
    │ orca-relay   │    │  orca-relay       │   │  orca-relay      │
    │ orca-cli     │    │  orca-cli         │   │  orca-cli        │
    │ claude/codex │    │  codex/copilot    │   │  gemini/claude   │
    │ git / gh     │    │  git / gh         │   │  git / gh        │
    │ /home/user/  │    │  C:\Users\...     │   │  /home/user/     │
    └──────────────┘    └───────────────────┘   └──────────────────┘
```

### 1.3 Thay đổi cốt lõi

| Thành phần | Trước | Sau |
|-----------|-------|-----|
| Orca runtime | Electron local | Node.js Web Server |
| Agent detection | Local PATH | Remote relay (`preflight.detectAgents`) |
| GitHub CLI check | Local `gh` binary | Remote relay (`gh` trên dev server) |
| Folder/Repo | Local filesystem | Remote filesystem trên dev server |
| PTY / Terminal | Local PTY | Remote PTY qua relay WebSocket |
| Onboarding state | Local SQLite | Server SQLite (centralized) |
| Platform detection | `process.platform` | Dev server platform qua relay handshake |

---

## 2. Danh sách Change Requests

| CR ID | Tên | Priority |
|-------|-----|----------|
| [CR-OB-002](./CR-OB-002-dev-server-registration.md) | Dev Server Registration & Management | Critical |
| [CR-OB-003](./CR-OB-003-agent-detection-remote.md) | Remote Agent Detection per Dev Server | Critical |
| [CR-OB-004](./CR-OB-004-platform-aware-wizard.md) | Platform-aware Onboarding Wizard | High |
| [CR-OB-005](./CR-OB-005-remote-preflight.md) | Remote Preflight Checks (gh, git) | High |
| [CR-OB-006](./CR-OB-006-remote-folder-repo.md) | Remote Folder/Repo Adding | High |
| [CR-OB-007](./CR-OB-007-windows-terminal-remote.md) | Remote Windows Terminal Detection | Medium |
| [CR-OB-008](./CR-OB-008-notification-server.md) | Server-side Notification Management | Medium |
| [CR-OB-009](./CR-OB-009-multi-devserver-checklist.md) | Multi Dev-Server Setup Guide Checklist | Medium |

---

## 3. Nguyên tắc thiết kế

1. **Server-agnostic state** — Onboarding state (`OnboardingState`) lưu trên Orca server, không phụ thuộc vào dev server cụ thể
2. **Per-server platform detection** — Mỗi dev server báo cáo platform của mình qua relay handshake
3. **Lazy dev server connect** — Không cần dev server kết nối trước khi onboarding bắt đầu
4. **Graceful degradation** — Nếu dev server offline, wizard vẫn có thể tiếp tục các bước không cần remote
5. **Multi-server** — Người dùng có thể thêm nhiều dev servers với platform khác nhau

---

## 4. Mapping Onboarding Steps → Remote vs Local

| Bước | Hiện tại | Thay đổi |
|------|---------|---------|
| Step 1: Chọn Agent | Detect local PATH | **Detect agents trên active dev server** |
| Step 2: Theme | Local settings | Giữ nguyên (server-side settings) |
| Step 3: Integrations (gh) | Local `gh` binary | **Check `gh` trên active dev server** |
| Step 4: Windows Terminal | Local `process.platform` | **Detect từ dev server platform** |
| Step 5: Notifications | Local macOS permission | **Check permission trên Orca server host** |
| Add Repo | Local filesystem | **Remote filesystem trên dev server** |

---

## 5. Acceptance Criteria tổng quát

- [x] Orca server có thể onboard user mà không cần Electron
- [x] Wizard hiển thị đúng agents có trên dev server đang active
- [x] Step 4 (Windows Terminal) chỉ hiện khi active dev server là Windows
- [x] Repo path nhập theo filesystem convention của dev server (POSIX/Windows)
- [x] Nhiều dev servers có thể được thêm cùng lúc
- [x] Onboarding state không bị mất khi dev server reconnect

---

## 6. Implementation Summary

> **Implemented:** 2026-07-23  
> **Status:** ✅ All CRs (002–009) implemented

| Component | Files |
|-----------|-------|
| Shared types | `src/shared/dev-server-types.ts` |
| DevServer Manager | `src/main/dev-server/dev-server-manager.ts` |
| Onboarding IPC | `src/main/ipc/onboarding-ipc.ts` |
| Web Push Manager | `src/main/notifications/web-push-manager.ts` |
| DevServer UI Slice | `src/renderer/src/store/slices/dev-servers.ts` |
| DevServer Wizard Step | `src/renderer/src/components/onboarding/DevServerStep.tsx` |
| Remote Agent Detection | `src/renderer/src/hooks/useRemoteAgentDetection.ts` |
| Platform Hook | `src/renderer/src/hooks/useActiveDevServerPlatform.ts` |
| Remote Preflight | `src/renderer/src/hooks/useRemotePreflightStatus.ts` |
| Remote Directory Browser | `src/renderer/src/hooks/useRemoteDirectoryBrowser.ts` |
| Remote Windows Caps | `src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts` |
| Web Push Hooks | `src/renderer/src/hooks/useBrowserNotificationPermission.ts`, `useWebPushSubscription.ts` |
| Service Worker | `src/renderer/public/service-worker.js` |
| Multi-server Checklist | `src/renderer/src/store/slices/onboarding-checklist.ts` |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23**

Architecture overview CR — defines the onboarding architecture. All components implemented:
- `DevServerManager` — manages dev server registrations
- `DevServerOnboardingWizard` — step-by-step UI flow  
- Remote preflight check system
- Platform-aware wizard branching (Linux/Mac/Windows)
