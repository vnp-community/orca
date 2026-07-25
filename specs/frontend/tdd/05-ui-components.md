# TDD-FE-05: UI Components & App Shell

**Document:** TDD-FE-05  
**Domain:** UI Components — App Shell, Major Screens, UI Library  
**Source files:** `src/renderer/src/components/`, `src/renderer/src/App.tsx`

---

## 1. App Shell (`App.tsx` ~127KB)

App.tsx là **layout root** của toàn bộ UI:

```
App.tsx
├─ <TooltipProvider>            (Radix tooltip context)
├─ <ConfirmationDialogProvider> (global confirm dialogs)
├─ <LinkRoutingPreferenceDialogProvider>
│
├─ Layout Frame
│   ├─ Titlebar (macOS/Windows/Linux chrome)
│   │   ├─ Traffic lights area (macOS)
│   │   ├─ <ActivityTitlebarControls>
│   │   ├─ <OrcaProfileSwitcher>
│   │   └─ Nav buttons (back/forward)
│   │
│   ├─ <Sidebar>  (left sidebar)
│   │   └─ WorkspaceBoard, repo list, SSH status
│   │
│   ├─ Main Content Area
│   │   ├─ <TaskPage>           (issues view)
│   │   ├─ <PullRequestPage>    (PR review)
│   │   ├─ <GitHubItemDialog>   (issue/PR detail)
│   │   ├─ <LinearItemDrawer>   (Linear issue)
│   │   ├─ Workspace area
│   │   │   ├─ <TabBar>
│   │   │   └─ Tab content:
│   │   │       ├─ <TerminalPane>       (terminal tab)
│   │   │       ├─ Editor tab           (file editor)
│   │   │       └─ <BrowserPane>        (headless browser)
│   │   └─ <FloatingTerminalToggleButton>
│   │
│   └─ <RightSidebar>
│       ├─ Git status / diff
│       ├─ Agent status
│       └─ Source control
│
└─ Global overlays
    ├─ <QuickOpen>              (Cmd+P file search)
    ├─ <WorktreeJumpPalette>    (Cmd+K worktree jump)
    ├─ <NewWorkspaceComposerModal>
    ├─ <UpdateCard>
    ├─ <StarNagCard>
    ├─ <ZoomOverlay>
    ├─ <CrashReportDialog>
    └─ <Toaster> (Sonner)
```

---

## 2. Lazy Loading Strategy

```typescript
// App.tsx sử dụng lazy() cho mọi heavy component:
const TaskPage = lazy(() => import('./components/TaskPage'))
const PullRequestPage = lazy(() => import('./components/PullRequestPage'))
const GitHubItemDialog = lazy(() => import('./components/GitHubItemDialog'))
const GitLabItemDialog = lazy(() => import('./components/GitLabItemDialog'))
const LinearItemDrawer = lazy(() => import('./components/LinearItemDrawer'))
const WorktreeJumpPalette = lazy(() => import('./components/WorktreeJumpPalette'))
const NewWorkspaceComposerCard = lazy(() => import('./components/NewWorkspaceComposerCard'))
const QuickOpen = lazy(() => import('./components/QuickOpen'))

// Wrap trong <Suspense>:
<Suspense fallback={null}>
  <TaskPage />
</Suspense>
```

---

## 3. Major Screens

### 3.1 TaskPage (~542KB — lớn nhất!)

```typescript
// src/renderer/src/components/TaskPage.tsx
// Hiển thị tasks/issues từ các providers:
// - GitHub Issues + PRs
// - Linear Issues
// - Jira Issues
// - Custom tasks

// Sub-components trong TaskPage:
// - task-page-jira-issue-list.tsx
// - task-page-localized-options.tsx
// - task-project-source-combobox.tsx
// - task-source-context-summary.ts
```

### 3.2 PullRequestPage (~259KB)

```typescript
// src/renderer/src/components/PullRequestPage.tsx
// PR review interface:
// - Diff viewer (file-by-file)
// - Inline comments (DiffComments)
// - PR checks status
// - Review submission
// - Merge controls (github-pr-merge-state.ts)
// - GitHub Actions check details
```

### 3.3 GitHubItemDialog (~285KB — lớn thứ 2!)

```typescript
// src/renderer/src/components/GitHubItemDialog.tsx
// Full-featured GitHub issue/PR viewer:
// - Issue detail + comments
// - PR diff + review
// - Label, assignee, milestone
// - Comment compose (markdown)
// - Linked worktrees
```

### 3.4 LinearItemDrawer (~59KB)

```typescript
// src/renderer/src/components/LinearItemDrawer.tsx
// Linear issue drawer (slide-in panel):
// - Issue detail (description, comments)
// - Status, priority, assignee, labels
// - Inline text editor (LinearIssueTextEditor)
// - Linked worktrees
```

### 3.5 NewWorkspaceComposerCard (~60KB)

```typescript
// src/renderer/src/components/NewWorkspaceComposerCard.tsx
// "Create new workspace/worktree" flow:
// - Select repo
// - Select branch (or create new)
// - Configure execution host
// - Run setup script
// - Start agent session (optional)
```

---

## 4. Terminal UI

### 4.1 TerminalPane (~127KB)

```typescript
// src/renderer/src/components/terminal-pane/TerminalPane.tsx
// ~1000+ lines, wraps entire terminal xterm.js + pane management

// Props:
type TerminalPaneProps = {
  tabId: string
  worktreeId: string
  environmentId: string | null    // null = local
  isActive: boolean
  initialPaneLayout?: TerminalPaneLayoutNode
}

// Internal:
// - PaneManager cho split/collapse
// - PtyConnection[] cho mỗi leaf pane
// - useTerminalPaneLifecycle() hook
// - useTerminalPaneGlobalEffects() hook
// - TerminalPaneOverlayLayer (SSH reconnect, etc.)
```

### 4.2 Tab Bar

```typescript
// src/renderer/src/components/tab-bar/
// - RecentTabSwitcher.tsx  (Ctrl+Tab switcher)
// - group-tab-order.ts     (visible tab ordering)

// Tab types:
type Tab = {
  id: string
  type: 'terminal' | 'editor' | 'browser' | 'pr' | 'task' | 'native-chat'
  worktreeId: string
  groupId: string
  title: string
  isPinned: boolean
}
```

### 4.3 Tab Group Layout

```typescript
// src/renderer/src/components/tab-group/
// Hỗ trợ: multiple tab groups side-by-side (column layout)

type TabGroupLayoutNode =
  | { type: 'leaf'; groupId: string }
  | { type: 'column'; children: TabGroupLayoutNode[]; sizes: number[] }
```

---

## 5. Sidebar

### 5.1 Left Sidebar

```typescript
// src/renderer/src/components/sidebar/
// Content:
// - Workspace status board (kanban-like)
// - Repo list với worktrees
// - SSH server status indicators
// - Quick actions

// Resizable:
// useSidebarResize() hook → drag handle → update leftSidebarWidth
```

### 5.2 Right Sidebar

```typescript
// src/renderer/src/components/right-sidebar/
// Content tabs:
// - Git status (staged/unstaged)
// - Source control (commit, push, PR)
// - Agent activity
// - Ports (detected open ports)

// useGitStatusPolling() — poll git status mỗi 5s khi tab active
```

---

## 6. UI Library (`components/ui/`)

shadcn/ui components (Radix UI + Tailwind):

```typescript
// src/renderer/src/components/ui/
// - button.tsx
// - input.tsx
// - dialog.tsx
// - dropdown-menu.tsx
// - context-menu.tsx
// - tooltip.tsx
// - select.tsx
// - checkbox.tsx
// - badge.tsx
// - scroll-area.tsx
// - separator.tsx
// - sonner.tsx    (Toaster)
// - popover.tsx
// - command.tsx   (command palette)
// - avatar.tsx
// - label.tsx
// - textarea.tsx
// - sheet.tsx     (slide-in panel)
// - tabs.tsx      (not terminal tabs, UI tabs)
// - collapsible.tsx
// - progress.tsx
// - slider.tsx
// - toggle.tsx
// - switch.tsx
// - alert.tsx
```

---

## 7. Dashboard Components

```typescript
// src/renderer/src/components/dashboard/
// RetainedAgentsSyncGate.tsx — gate rendering nếu agent sync chưa xong
// Prevents blank UI khi initial sync đang chạy
```

---

## 8. Onboarding

```typescript
// src/renderer/src/components/onboarding/
// Multi-step onboarding flow cho user mới:
// - SSH server setup
// - GitHub connection
// - Create first repo
// - Create first worktree
// - Start agent session

// Trigger: ONBOARDING_FLOW_VERSION mismatch → show onboarding
```

---

## 9. Settings Panel

```typescript
// src/renderer/src/components/settings/
// Full settings UI:
// - General (theme, language, font)
// - Terminal (shell, scrollback, custom themes)
// - AI providers (API keys per provider)
// - Integrations (GitHub, Linear, Jira, GitLab)
// - SSH & Remotes
// - Automations
// - Notifications
// - Keybindings
// - Advanced (telemetry, update channel, ...)

// useSettingsNavigationMetadata() — sidebar nav structure
```

---

## 10. BrowserPane

```typescript
// src/renderer/src/components/browser-pane/
// Headless Chromium via Orca browser RPC methods

// Features:
// - Navigate URL
// - Screenshot/PDF
// - Click, type (computer use)
// - Screencast (live preview)
// - Port-forwarded local servers (detect + open)
```

---

## 11. Quick Open (`QuickOpen.tsx`)

```typescript
// Cmd+P — file quick open
// Dùng: ripgrep + fuzzy matching
// quick-open-file-list.ts — file list build + filter
// quick-open-search.ts    — search algo
// quick-open-install-rg-guidance.tsx — nếu rg chưa install
```

---

## 12. Worktree Jump Palette (`WorktreeJumpPalette.tsx`)

```typescript
// ~95K — Cmd+K shortcut
// Jump between worktrees/repos
// Show: agent status, branch, active terminals
// Actions: open terminal, open file, switch worktree, create new
```

---

## 13. Activity Components

```typescript
// src/renderer/src/components/activity/
// ActivityTitlebarControls.tsx — status icons in titlebar:
// - Agent running indicator
// - Unread agent count badge
// - Runtime connection status
// - Notification bell
```

---

## 14. Agent State Components

```typescript
// AgentStateDot.tsx — colored dot showing agent status
//   green = running, yellow = waiting, gray = idle, red = error

// AgentHibernationGate.tsx — renders children only when agents not hibernated

// CodexRestartChip.tsx (~9K)
// — Chip shown when Codex needs restart (context limit hit, error, etc.)
//   with auto-restart logic
```

---

## 15. Auth UI Components (v4.0 — CR-LOGIN-001, CR-LOGIN-002)

```typescript
// src/renderer/src/components/auth/

// UserAvatarMenu.tsx — Web-only (ORCA_PLATFORM === 'web' + authenticated)
// Avatar dropdown in Titlebar:
//   - Avatar image (avatarUrl) hoặc initials fallback (2 chữ đầu tên)
//   - Dropdown: tên đầy đủ, email, UserRoleBadge, Logout button
//   - Escape key đóng dropdown
//   - Click Logout → POST /auth/logout → clearAuth() → redirect /login

// UserRoleBadge.tsx — Role indicator
//   - 'admin'     → class 'badge-admin'    (đỏ/cam)
//   - 'lead'      → class 'badge-lead'     (tím)
//   - 'developer' → class 'badge-developer'(xanh)
```

---

## 16. Login Page Components (v4.0 — CR-LOGIN-001)

```typescript
// src/renderer/src/web/login/

// LoginPage.tsx — Container
//   Props: availableProviders: SsoProvider[], onLoginSuccess: (user: AuthUser) => void
//   Renders: LoginForm + (SsoButtons if providers) + PairCodeFallback

// LoginForm.tsx — Email/password form
//   Props: onSubmit, isLoading, error: string | null
//   Validates: email format client-side, server errors via error prop
//   aria: role="form", fields có label

// SsoButton.tsx — SSO provider link
//   href="/auth/sso/{provider}" (không POST, trực tiếp redirect)
//   Providers: 'github' | 'google' | 'keycloak'
//   role="link" với aria-label="Continue with {Provider}"

// PairCodeFallback.tsx — backward compat
//   Giữ lại PairCode input để user có thể dùng pairing URL thay login
```

---

## 17. Admin Panel SPA Components (v4.0 — CR-LOGIN-004)

```typescript
// src/renderer/src/components/admin/

// AdminApp.tsx — SPA root với React Router
//   Routes: / → AdminDashboard, /users → UsersPage,
//           /sessions → SessionsPage, /policies → PoliciesPage, /audit → AuditPage
//   Layout: AdminLayout với sidebar navigation + logout button

// AdminDashboard.tsx — stats overview
//   4 stat cards: Total Users, Active Sessions, SSH Connections, Devices
//   Poll /admin/api/stats mỗi 30s

// UsersPage.tsx + UserForm.tsx
//   Search/filter by role, deactivate user, create/edit user dialog

// SessionsPage.tsx
//   Active sessions list, kill session (DELETE /admin/api/sessions/:id)

// PoliciesPage.tsx + PolicyForm.tsx
//   Access policies CRUD

// AuditPage.tsx
//   Audit log với date range filter + action type filter

// admin-api-client.ts — fetch wrapper /admin/api/*
//   listUsers(), createUser(), updateUser(), deleteUser()
//   listSessions(), killSession()
//   listPolicies(), createPolicy(), updatePolicy(), deletePolicy()
//   getAuditLog(params)
//   getStats()
```

---

## 18. SSH Provisioning UI Components (v4.0 — CR-LOGIN-003)

```typescript
// src/renderer/src/components/ssh/

// SshProvisioningProgress.tsx
//   Props: step: string, progress: number (0–100)
//   Renders: progress bar (aria-role="progressbar") + step description text

// SshUserIndicator.tsx
//   Props: serverId, linuxUsername, provisioned, provisioningStatus
//   States:
//     idle     → chỉ hiển thị username
//     checking → "Checking..." text
//     provisioning → SshProvisioningProgress
//     done     → username + ✅ icon
//     error    → error message (role="alert")
//   Inject vào SshTargetRow — web mode only
```
