# TDD-FE-01: Kiến Trúc Tổng Thể Frontend v4

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/`, `vite.web-spa.config.ts`, `electron.vite.config.ts`

---

## 1. Render Targets

### 1.1 Desktop (Electron)

```
Entry:   src/renderer/index.html
Main:    src/renderer/src/main.tsx
  └── RecoverableErrorBoundary
      └── App.tsx (full Orca UI)

Platform:
  window.api = preload IPC bridge (src/preload/index.ts)
  Transport:  Electron IPC → Main Process
```

### 1.2 Web SPA (Browser)

```
Entry:   src/renderer/web-index.html
Main:    src/renderer/src/web/main.tsx
  └── bootstrapWebApp()  [src/renderer/src/web/main-web-bootstrap.tsx]
      ├─ checkAuthSession()  → GET /auth/me
      │   ├─ Authenticated → App.tsx (full Orca UI)
      │   └─ Not auth     → LoginPage (login form + SSO)
      ├─ ConnectionStatusProvider
      └─ Service Worker registration

Platform:
  window.api = web-preload-api.ts (WebSocket RPC bridge)
  Transport:  WebSocketRpcClient → ws://orca-server:6768
```

### 1.3 Admin SPA (Browser)

```
Entry:   src/renderer/admin-index.html
Main:    src/renderer/src/admin/admin-main.tsx
  └── AdminApp.tsx
      └── React Router
          ├─ /           → AdminDashboard
          ├─ /users      → UsersPage
          ├─ /sessions   → SessionsPage
          ├─ /policies   → PoliciesPage
          └─ /audit      → AuditPage
```

---

## 2. App.tsx (Core — không sửa)

```
App.tsx (~127KB) là entry point chung cho Desktop + Web.
Platform detection: if (window.__ORCA_PLATFORM__ === 'web') { /* web-only UI */ }

Main layout:
  ├─ Sidebar (repo list, worktree list)
  ├─ Tab bar (active workspaces)
  ├─ Main pane (Terminal/Editor/Browser/Task)
  └─ Right sidebar (agent status, source control, etc.)
```

---

## 3. Tech Stack

| Technology | Version | Usage |
|-----------|---------|-------|
| React | 18 | UI rendering |
| Zustand | 4 | State management |
| Vite | 5 | Build tool |
| TypeScript | 5 | Type safety |
| xterm.js | 5 | Terminal rendering (WebGL) |
| React Router | 6 | Admin SPA routing |
| vitest | 1 | Unit tests |
| @testing-library/react | 14 | React component tests |

---

## 4. Build Configuration

### vite.web-spa.config.ts

```typescript
export default defineConfig({
  build: {
    rollupOptions: {
      input: {
        main:  'src/renderer/web-index.html',
        admin: 'src/renderer/admin-index.html'   // NEW v4.0
      }
    }
  }
})
```

### electron.vite.config.ts

```typescript
// renderer entry thêm admin-index.html
{
  renderer: {
    build: {
      rollupOptions: {
        input: {
          index: 'src/renderer/index.html',
          admin: 'src/renderer/admin-index.html'  // NEW v4.0
        }
      }
    }
  }
}
```

---

## 5. Code Splitting

| Chunk | Lazy loaded? | Trigger |
|-------|-------------|---------|
| App.tsx core | No (eager) | Entry |
| Terminal (xterm.js) | Yes | First terminal open |
| GitHubItemDialog | Yes | First PR/issue open |
| LinearItemDrawer | Yes | First Linear issue open |
| AdminApp | Yes | /admin/ route |
| LoginPage | Yes | Auth check → not authenticated |
| FleetHealthDashboard | Yes | Fleet tab |

---

## 6. i18n

```typescript
// Mọi user-facing text dùng translate()
import { translate as t } from '../i18n'

// Hoặc useTranslation hook (React components)
const { t } = useTranslation()
```

Default locale: `en`. Supported: `en`, `ja`, `zh-CN`, `ko`.
