# FE-SOL-02: CLI Auth Login UI — GitHub & GitLab (Web mode)

> **CRs:** CR-GH-002, CR-INT-001  
> **Backend SOL tương ứng:** SOL-03-Remote-PTY  
> **TDD:** TDD-FE-05 (UI Components), TDD-FE-04 (Terminal Subsystem)  
> **Status:** ✅ DONE & 🧪 AC Verified — Phase 1 + Phase 2 (xterm.js inline terminal) (2026-07-25)  
> **Tasks:** [FE-TASK-02](../tasks/FE-TASK-02-web-preload-github-gitlab.md), [FE-TASK-03](../tasks/FE-TASK-03-webmode-cli-auth-section.md), [FE-TASK-04](../tasks/FE-TASK-04-cli-cards-webmode-branch.md)

---

## Vấn đề

Khi GitHub/GitLab CLI chưa được authenticate trên Dev Server, UI hiện tại hiển thị:

```
"The GitHub CLI is installed but not authenticated. Run this command in a terminal:
  gh auth login"
```

Trong **Web mode**, người dùng không thể chạy lệnh này trực tiếp. Cần:
1. Phát hiện khi đang ở Web mode + Dev Server connected
2. Hiển thị nút "Login with GitHub" thay vì text command
3. Nút gọi `github.startAuthLogin(devServerId)` → mở PTY terminal trong UI

---

## Thiết kế giải pháp

### 1. Phát hiện Web mode trong Integration Cards

```typescript
// src/renderer/src/components/settings/cli-source-control-integration-cards.tsx

// Pattern phát hiện Web mode (theo codebase hiện tại):
const cliStatus = useAppStore(s => s.cliStatus)
const isWebMode = cliStatus?.unsupportedReason === 'launch_mode_unavailable'
// Hoặc: dùng activeDevServerId
const activeDevServerId = useAppStore(s => s.activeDevServerId)
const hasConnectedDevServer = activeDevServerId != null
```

### 2. Cập nhật `GitHubIntegrationCard` — Web mode "Login" action

```typescript
// src/renderer/src/components/settings/cli-source-control-integration-cards.tsx

// THAY ĐỔI: khi not-authenticated trong Web mode
{status === 'not-authenticated' && isWebMode && hasConnectedDevServer ? (
  // Web mode: hiện nút PTY login
  <WebModeGitHubAuthSection devServerId={activeDevServerId!} onRefresh={refresh} />
) : status === 'not-authenticated' ? (
  // Electron mode: hiển thị command text (giữ nguyên hành vi cũ)
  <>
    <p className="text-xs text-muted-foreground">
      The GitHub CLI is installed but not authenticated. Run this command in a terminal:
    </p>
    <div className={commandRowClass}>
      <Terminal className="size-3.5 shrink-0 text-muted-foreground" />
      gh auth login
    </div>
  </>
) : null}
```

### 3. Component `WebModeGitHubAuthSection`

```typescript
// src/renderer/src/components/settings/WebModeCliAuthSection.tsx [NEW]

type WebModeCliAuthSectionProps = {
  provider: 'github' | 'gitlab'
  devServerId: string
  onRefresh: () => void
}

export function WebModeCliAuthSection({
  provider,
  devServerId,
  onRefresh
}: WebModeCliAuthSectionProps): React.JSX.Element {
  const [isLoading, setIsLoading] = useState(false)
  const [ptyInfo, setPtyInfo] = useState<{ ptyId: string; devServerId: string } | null>(null)

  const handleStartLogin = async () => {
    setIsLoading(true)
    try {
      const result = provider === 'github'
        ? await window.api.github.startAuthLogin(devServerId)
        : await window.api.gitlab.startAuthLogin(devServerId)
      setPtyInfo(result)
      // Mở PTY terminal pane với ptyId
      // (tích hợp với useAppStore terminal system)
    } catch (err) {
      console.error(`[${provider} auth login]`, err)
    } finally {
      setIsLoading(false)
    }
  }

  if (ptyInfo) {
    // Render PTY terminal (xterm.js) inline hoặc trong pane
    return (
      <RemotePtyTerminal
        ptyId={ptyInfo.ptyId}
        devServerId={ptyInfo.devServerId}
        onClose={() => {
          setPtyInfo(null)
          onRefresh() // re-check preflight sau khi login xong
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-muted-foreground">
        {provider === 'github'
          ? 'Authenticate the GitHub CLI on your Dev Server.'
          : 'Authenticate the GitLab CLI on your Dev Server.'}
      </p>
      <Button
        variant="outline"
        size="sm"
        disabled={isLoading}
        onClick={handleStartLogin}
      >
        {isLoading ? (
          <Loader2 className="size-3.5 mr-1.5 animate-spin" />
        ) : (
          <Terminal className="size-3.5 mr-1.5" />
        )}
        {provider === 'github' ? 'Login with GitHub CLI' : 'Login with GitLab CLI'}
      </Button>
    </div>
  )
}
```

### 4. Thêm `github.startAuthLogin` và `gitlab.startAuthLogin` vào `window.api`

Backend đã implement RPC method (SOL-03). Cần expose trong web-preload-api:

```typescript
// src/renderer/src/web/web-preload-api.ts [VERIFY — đã thêm trong backend tasks]

// Đã có (từ TASK-05-06):
github: {
  startAuthLogin: (devServerId: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>('github.startAuthLogin', { devServerId }),
  revokeAuth: (devServerId: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>('github.revokeAuth', { devServerId }),
}
gitlab: {
  startAuthLogin: (devServerId: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>('gitlab.startAuthLogin', { devServerId }),
  revokeAuth: (devServerId: string) =>
    callRuntimeResult<{ ptyId: string; devServerId: string }>('gitlab.revokeAuth', { devServerId }),
}
```

### 5. `RemotePtyTerminal` Component — hiển thị PTY output

```typescript
// src/renderer/src/components/settings/RemotePtyTerminal.tsx [NEW]
// Wrapper xterm.js inline cho PTY stream từ relay

// Theo TDD-FE-04: PTY transport bridge qua WebSocket
// ptyId từ relay.call('pty.spawn') → subscribe vào PTY stream qua WS

type RemotePtyTerminalProps = {
  ptyId: string
  devServerId: string
  onClose: () => void
}
```

---

## Files cần thay đổi

### [MODIFY] `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx`
- Thêm logic phát hiện Web mode
- Phân nhánh UI: Web mode → `WebModeCliAuthSection` / Electron mode → command text

### [NEW] `src/renderer/src/components/settings/WebModeCliAuthSection.tsx`
- Component với state: idle → loading → pty-open
- Gọi `window.api.github.startAuthLogin(devServerId)` hoặc `window.api.gitlab.startAuthLogin(devServerId)`
- Mở `RemotePtyTerminal` sau khi nhận `ptyId`
- `onClose` → refresh preflight status

### [NEW] `src/renderer/src/components/settings/RemotePtyTerminal.tsx`
- xterm.js inline terminal (80x24 hoặc resize)
- Subscribe PTY stream qua WebSocket RPC
- `onClose` callback khi PTY exits

### [VERIFY] `src/renderer/src/web/web-preload-api.ts`
- Verify `github.startAuthLogin`, `github.revokeAuth`, `gitlab.startAuthLogin`, `gitlab.revokeAuth` đã được expose

---

## Acceptance Criteria

1. ✅ Trong Web mode + Dev Server connected + `not-authenticated` → hiển thị nút "Login with GitHub CLI" / "Login with GitLab CLI"
2. ✅ Click nút → spawn PTY terminal trên Dev Server → hiển thị `WebModeInlinePty` info panel (Phase 1)
3. ✅ Sau khi login xong → click "Done" → `onComplete()` → `refresh()` → card re-checks preflight
4. ✅ Trong Electron mode → behavior giữ nguyên (command text "gh auth login")
5. ✅ `window.api.github.startAuthLogin` và `window.api.gitlab.startAuthLogin` available (FE-TASK-02)

---

## Implementation Verified

| File | Status | Lines |
|------|--------|-------|
| `src/preload/api-types.ts` | ✅ `github` + `gitlab` types added | 3394–3415 |
| `src/renderer/src/web/web-preload-api.ts` | ✅ `github.*` + `gitlab.*` exposed | 747–776 |
| `src/renderer/src/components/settings/WebModeCliAuthSection.tsx` | ✅ Phase 1 + Phase 2 (352 lines) | — |
| `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx` | ✅ Web mode branch | 131–136, 276–281 |

**Phase 2 — xterm.js inline terminal (352 lines):**
- `WebModeInlinePty`: full xterm.js Terminal + FitAddon
- Subscribe to Dev Server relay stream via `getRemoteRuntimeTerminalMultiplexer(environmentId).subscribeTerminal(...)`
- `onData`/`onSnapshot` callbacks → `term.write(data)`
- `term.onData` → `stream.sendInput(data)` (keyboard input back to PTY)
- `ResizeObserver` → `fitAddon.fit()` + `stream.resize(cols, rows)`
- Status indicator: `connecting` → `connected` → `exited` / `error`
- `PTY_THEME`: Catppuccin Mocha dark palette (no store dependency)
- CSS loaded lazily via `import('@xterm/xterm/css/xterm.css')` (settings panel only)


---

## Tham chiếu Backend

- `github.startAuthLogin` → `relay.call('pty.spawn', { command: 'gh', args: ['auth', 'login'] })`
- `gitlab.startAuthLogin` → `relay.call('pty.spawn', { command: 'glab', args: ['auth', 'login'] })`
- PTY stream bridged qua WebSocket (`pty.data` events)
- Credential isolation: OS-level per Linux user (SOL-02)
