# TDD-FE-01: Kiến trúc tổng thể Frontend

**Document:** TDD-FE-01  
**Domain:** Frontend Architecture — Render Targets, Build, Tech Stack  
**Source files:** `src/renderer/`, `electron.vite.config.ts`, `package.json`

---

## 1. Tech Stack

| Layer | Technology |
|-------|-----------|
| UI Framework | React 18 (StrictMode) |
| State Management | **Zustand** (không Redux) |
| Build tool | **Vite** (via electron-vite) |
| Terminal | **xterm.js** (WebGL renderer) |
| Styling | **Tailwind CSS** + custom CSS vars |
| UI Components | **shadcn/ui** (Radix primitives) |
| i18n | **react-i18next** |
| Icons | **lucide-react** |
| Toasts | **Sonner** |
| HTTP/WS client | Custom (native `WebSocket` + `fetch`) |
| Type system | TypeScript strict |
| Test | **Vitest** (unit) + Playwright (e2e) |

---

## 2. Render Targets

Orca frontend có **hai render target** dùng cùng codebase:

```
src/renderer/
├── index.html          ← Electron Desktop renderer entry
├── web-index.html      ← Web browser entry (headless serve)
└── src/
    ├── main.tsx        ← Desktop entry (mount <App/>)
    └── web/
        └── main.tsx    ← Web entry (mount <App/> with web-preload-api)
```

### 2.1 Desktop Mode (Electron)

```typescript
// src/renderer/src/main.tsx
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <I18nProvider>
      <RecoverableRenderErrorBoundary ...>
        <App />   // ← same App.tsx as web
      </RecoverableRenderErrorBoundary>
    </I18nProvider>
  </StrictMode>
)

// window.api comes from Electron preload (src/preload/index.ts)
// window.api.filesystem.readFile(...)
// window.api.pty.create(...)
// etc.
```

### 2.2 Web Mode (Browser) — Sau restructure_v1

```typescript
// src/renderer/src/web/main.tsx
// Entry point tối giản — delegate sang bootstrapWebApp()
import { bootstrapWebApp } from './main-web-bootstrap'
bootstrapWebApp().catch(err => console.error('[Orca Web] Fatal:', err))
```

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx [MỚI]
// Testable bootstrap function — separated from side-effects
export async function bootstrapWebApp(options: BootstrapOptions = {}): Promise<void> {
  // 1. Find root element
  const rootEl = document.getElementById(options.rootElementId ?? 'root')

  // 2. Create lightweight RPC client (for connection tracking)
  const client = new WebSocketRpcClient(options.wsUrl)

  // 3. Connect with retries (maxRetries default: 3)
  let connected = false
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try { await client.connect(); connected = true; break }
    catch { if (attempt < maxRetries) await sleep(retryDelayMs) }
  }

  // 4a. Connection failed → show error UI
  if (!connected) { showErrorUi(rootEl); return }

  // 4b. Render with pairing flow + ConnectionStatusProvider
  ReactDOM.createRoot(rootEl).render(
    <StrictMode>
      <I18nProvider>
        <WebRootBoundary client={client} />   // includes WebRoot + ConnectionStatusProvider
      </I18nProvider>
    </StrictMode>
  )
}
```

```typescript
// WebRoot component — handles pairing vs App decision
function WebRoot({ client }: { client: IRpcClient }) {
  const [hasEnvironment, setHasEnvironment] = useState(...)

  if (!hasEnvironment) {
    return <WebConnect onConnected={() => setHasEnvironment(true)} />
  }

  installWebPreloadApi()   // inject window.api after pairing
  return (
    <ConnectionStatusProvider client={client}>
      <ConnectionStatusBanner />   // overlay khi disconnected
      <App />
    </ConnectionStatusProvider>
  )
}
```

### 2.3 `window.api` Abstraction

```typescript
// src/preload/index.ts (Desktop)
// src/renderer/src/web/web-preload-api.ts (Web)
// Cả hai expose cùng interface:

interface OrcaApi {
  filesystem: { readFile, writeFile, listDir, search, watch, ... }
  pty: { create, write, resize, kill, subscribe, ... }
  ssh: { listTargets, connect, disconnect, ... }
  worktrees: { detect, create, delete, ... }
  repos: { list, create, update, delete, ... }
  settings: { getGlobal, updateGlobal, ... }
  github: { listPRs, createPR, ... }
  runtimeEnvironments: { call, ... }
  // ...
}
```

---

## 3. Build Configuration

```typescript
// electron.vite.config.ts
export default defineConfig({
  renderer: {
    root: 'src/renderer',
    build: {
      outDir: 'out/renderer',
      rollupOptions: {
        input: {
          index: 'src/renderer/index.html',         // Desktop entry
          web: 'src/renderer/web-index.html',       // Web entry
        }
      }
    },
    plugins: [
      react(),
      tailwindcss(),
      // Path aliases:
      tsconfigPaths()   // @/ → src/renderer/src/
    ]
  },
  main: { /* Electron main process */ },
  preload: { /* Electron preload */ }
})
```

### Code splitting

```typescript
// Lazy loading tất cả heavy components:
const TaskPage = lazy(() => import('./components/TaskPage'))
const PullRequestPage = lazy(() => import('./components/PullRequestPage'))
const GitHubItemDialog = lazy(() => import('./components/GitHubItemDialog'))
const LinearItemDrawer = lazy(() => import('./components/LinearItemDrawer'))

// Custom lazy với retry:
// src/renderer/src/lib/lazy-with-retry.ts
export function lazyWithRetry<T>(factory: () => Promise<T>) {
  // Retry 3 lần với exponential backoff
  // Sentry breadcrumb khi import fails
}
```

---

## 4. App Initialization Flow

### 4.1 Desktop Mode (Electron)

```
Electron opens Orca
  ↓
src/renderer/src/main.tsx
  ├─ recordRendererCrashBreadcrumb('renderer_bootstrap_started')
  ├─ installRendererCrashDiagnostics()     ← Sentry setup
  ├─ applyDocumentTheme('system', ...)     ← CSS vars (light/dark)
  ├─ [DEV] init react-grab                ← component inspector
  └─ mount RendererRoot
       └─ RecoverableRenderErrorBoundary
            └─ App.tsx
                 ├─ useIpcEvents()          ← subscribe Electron IPC events
                 ├─ scheduleRuntimeGraphSync() ← sync store từ backend
                 └─ Main layout: Sidebar + Content + RightSidebar
```

### 4.2 Web Mode (Browser) — restructure_v1

```
Browser truy cập https://orca-server/
  ↓
src/renderer/src/web/main.tsx  (minimal — chỉ gọi bootstrapWebApp())
  ↓
src/renderer/src/web/main-web-bootstrap.tsx  [MỚI]
  ├─ new WebSocketRpcClient(wsUrl)  ← lightweight ping client
  ├─ client.connect() → retry tối đa 3 lần với 2s delay
  │
  ├─ [fail] showErrorUi()   ← "Cannot connect to Orca backend"
  │
  └─ [success] ReactDOM.createRoot().render()
        └─ StrictMode
             └─ I18nProvider
                  └─ RecoverableRenderErrorBoundary (web surface)
                       └─ WebRoot
                            ├─ [chưa paired] → <WebConnect />
                            └─ [đã paired] → installWebPreloadApi()
                                            ConnectionStatusProvider
                                              ├─ ConnectionStatusBanner  [MỚI]
                                              └─ <App />  (same App.tsx)
```

---

## 5. Path Aliases

```typescript
// tsconfig (+ vite resolve.alias)
{
  "@/": "src/renderer/src/",  // thường dùng nhất
  // Ví dụ:
  // import { useAppStore } from '@/store'
  // import { Button } from '@/components/ui/button'
  // import { translate } from '@/i18n/i18n'
}
```

---

## 6. Styling System

### Tailwind + CSS Variables

```css
/* src/renderer/src/assets/main.css */
:root {
  /* Color tokens */
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  --card: 0 0% 100%;
  --border: 214.3 31.8% 91.4%;
  --primary: 222.2 47.4% 11.2%;
  --destructive: 0 84.2% 60.2%;
  --muted: 210 40% 96.1%;
  --muted-foreground: 215.4 16.3% 46.9%;
  /* ... */
}

.dark {
  --background: 222.2 84% 4.9%;
  /* dark mode overrides */
}
```

### Terminal theme variables (riêng biệt)

```typescript
// src/renderer/src/lib/terminal-theme.ts
// Terminal màu sắc không dùng Tailwind vars
// Thay vào đó: ANSI color palette (16 màu)
// Hỗ trợ: light/dark/custom themes
```

---

## 7. i18n System

```typescript
// src/renderer/src/i18n/I18nProvider.tsx
// react-i18next setup

// Sync translate (non-React):
import { translate } from '@/i18n/i18n'
const text = translate('key', 'English fallback')

// React hook:
import { useTranslation } from 'react-i18next'
const { t } = useTranslation()
const text = t('key', 'English fallback')

// Auto-generated keys format:
// 'auto.<component-path>.<hash>'
// Ví dụ: 'auto.web.WebConnect.3affe7de3a'
```

---

## 8. Error Boundaries

```typescript
// src/renderer/src/components/error-boundaries/RecoverableRenderErrorBoundary.tsx
// Wrap mọi major section trong App

// Tính năng:
// - Sentry error capture
// - "Retry" button để remount
// - boundaryId để track từng boundary
// - surface type để phân loại lỗi

<RecoverableRenderErrorBoundary
  boundaryId="terminal-pane"
  surface="terminal"
  title="Terminal hit a render error"
  description="Click retry to remount the terminal."
>
  <TerminalPane ... />
</RecoverableRenderErrorBoundary>
```

---

## 9. Crash Diagnostics

```typescript
// src/renderer/src/lib/crash-diagnostics.ts
installRendererCrashDiagnostics()
// - Uncaught error → Sentry + IPC → Main process log
// - Breadcrumbs cho từng bước init
// - GPU fallback detection khi WebGL fails

recordRendererCrashBreadcrumb('event-name', metadata)
```

---

## 10. Module Structure

```
src/
├── platform/
│   ├── rpc-client-interface.ts        [MỚI restructure_v1] IRpcClient interface
│   └── adapters/web/
│       ├── rpc-client.ts              [MỚI] WebSocketRpcClient
│       └── __tests__/
│           └── rpc-client.test.ts     [MỚI] 15 tests
│
└── renderer/src/
    ├── App.tsx              (~127KB) — app shell + routing + global effects
    ├── main.tsx             — Desktop entry point
    ├── assets/
    │   └── main.css         — global CSS + Tailwind + CSS vars
    ├── components/          — UI components (~50 subdirs, 200+ files)
    ├── hooks/               — Custom React hooks (~85 files)
    ├── i18n/                — i18n setup
    ├── lib/                 — Utilities, helpers
    ├── runtime/             — Runtime client layer (~73 files)
    ├── startup/             — Startup logic
    ├── store/               — Zustand store + slices (~161 files)
    └── web/                 — Web-only code
        ├── main.tsx                        — Web entry (gọi bootstrapWebApp)
        ├── main-web-bootstrap.tsx          [MỚI] bootstrapWebApp() function
        ├── ConnectionStatusProvider.tsx    [MỚI] React context + hooks
        ├── ConnectionStatusBanner.tsx      [MỚI] Disconnect overlay
        ├── web-preload-api.ts              (~135KB) window.api implementation
        ├── web-runtime-client.ts           (~27KB) E2EE WebSocket client
        ├── WebConnect.tsx                  Pairing UI
        └── __tests__/                      Test files
```

---

## v5.0 — Frontend Architecture Extensions

### Tech Stack Additions (v5.0)

| Layer | Technology | Use |
|-------|-----------|-----|
| Workspace State | **WorkspaceContext** (React Context) | Per-project shared state + event bus |
| Relay Client | `rpc.call()` via WebSocket RPC | Remote relay operations (git, fs, AI) |
| Streaming | `rpc.callStream()` | push/pull progress, file read |
| Diff Viewer | **Monaco Editor** (read-only diff mode) | Side-by-side git diff |
| Graph Rendering | **@xyflow/react** (React Flow) | Task Graph DAG visualization |
| Markdown | **react-markdown** + **remark-gfm** | Task descriptions, AI output |

### v5.0 Render Targets (thêm Admin extensions)

```
src/renderer/
├── index.html              ← Electron Desktop
├── web-index.html          ← Web SPA
├── admin-index.html        ← Admin SPA
└── src/
    ├── context/
    │   └── WorkspaceContext.tsx    ← [NEW v5.0]
    ├── components/
    │   ├── workspace/              ← [NEW v5.0] per-project UI
    │   │   ├── WorkspaceLayout.tsx
    │   │   ├── ExplorerPanel.tsx
    │   │   ├── GitPanel/
    │   │   │   ├── GitPanel.tsx
    │   │   │   ├── DiffViewer.tsx
    │   │   │   ├── CommitForm.tsx
    │   │   │   ├── BranchManager.tsx
    │   │   │   └── PullRequestForm.tsx
    │   │   ├── AgentPanel.tsx
    │   │   └── TerminalPanel.tsx
    │   ├── profile/                ← [NEW v5.0] profile hierarchy
    │   │   ├── ProfileEditor.tsx
    │   │   └── ProfileSourceBadge.tsx
    │   ├── project/                ← [NEW v5.0] project management
    │   │   ├── ProjectSwitcher.tsx
    │   │   ├── ProjectSettings.tsx
    │   │   └── MemberManager.tsx
    │   ├── ai-provider/            ← [NEW v5.0] AI provider admin
    │   │   ├── ProviderList.tsx
    │   │   ├── ProviderForm.tsx
    │   │   ├── CredentialInput.tsx
    │   │   └── UsageChart.tsx
    │   ├── workflow/               ← [NEW v5.0] workflow builder
    │   │   ├── WorkflowBuilder.tsx
    │   │   ├── StepEditor.tsx
    │   │   ├── DAGPreview.tsx
    │   │   └── ExecutionMonitor.tsx
    │   └── task/                   ← [NEW v5.0] task graph
    │       ├── TaskGraph.tsx
    │       ├── TaskCard.tsx
    │       ├── TaskDetail.tsx
    │       ├── TaskAIDecompose.tsx
    │       └── TaskPromptEditor.tsx
    └── store/slices/
        ├── profile.ts              ← [NEW v5.0]
        ├── project.ts              ← [NEW v5.0]
        ├── ai-provider.ts          ← [NEW v5.0]
        ├── workflow.ts             ← [NEW v5.0]
        └── task.ts                 ← [NEW v5.0]
```

### v5.0 Application Boot Flow (Web SPA)

```
main-web-bootstrap.tsx
    │
    ├── checkAuthSession() → setAuth(user) or redirect /login
    │
    ├── loadProfileConfig() → rpc.call('profile.getResolved')
    │   → useProfileStore.setResolved(profile)
    │
    ├── loadProjects() → rpc.call('project.list')
    │   → useProjectStore.setProjects(projects)
    │
    └── mount <App />
            │
            └── <WorkspaceProvider>  ← Wraps all workspace panels
                    │
                    ├── <ProjectSwitcher> → WorkspaceContext.switchProject()
                    └── <WorkspaceLayout> → panels load when project selected
```

### v5.0 Zustand Slices mới

| Slice | State | Actions |
|-------|-------|---------|
| `profile` | resolvedProfile, companyProfile, deptProfile, userProfile | setResolved, updateUser, invalidate |
| `project` | projects[], activeProjectId | setProjects, setActive, addMember |
| `ai-provider` | accounts[], usage | setAccounts, updateStatus, recordUsage |
| `workflow` | templates[], executions[] | setTemplates, addExecution, updateStep |
| `task` | tasks{}, activeTaskId, expandedNodes | setTasks, updateTask, setActive |
