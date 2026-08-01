# Luồng: Enable Orca CLI

**Ngày ghi:** 2026-07-25  
**Trạng thái:** RECHECK — cần xác nhận lại với code thực tế trước khi promote lên `flows/`

## Tổng quan

Orca hỗ trợ **2 chế độ chạy** — mỗi chế độ có luồng "Enable CLI" khác nhau:

| Chế độ | Giao diện | Cách gọi | Execution target |
|--------|-----------|----------|-----------------|
| **Electron app** | Desktop app | Electron IPC | Main process (local machine) |
| **Web/Server mode** | Browser → `b15.openledger.vn` | WebSocket RPC | Orca Server container (172.20.2.39) |

> **Quan trọng:** Trong chế độ **Web Browser**, "Enable Orca CLI" **không có trong UI** — CLI được cài tự động trên server khi khởi động.

---

## Môi trường Deploy (deploy/dev)

```
[Internet]
    ↓ HTTPS
[Gateway: 103.67.184.32 / 172.20.2.16]
  Nginx container (port 80/443) → upstream → 172.20.2.39:6768 (WS) / :6769 (HTTP)
    ↓ (internal network 172.20.x.x)
[Orca Server: 172.20.2.39]
  Docker container: orca-server
    - port 6768 → WebSocket RPC
    - port 6769 → HTTP (web SPA + /health/*)
    ↓ SSH
[Developer Machine: 172.20.2.31]
  Dev code chạy tại đây — Orca SSH vào để relay
```

Ref: [`deploy/dev/docker-compose.orca.yml`](../../deploy/dev/docker-compose.orca.yml)

---

## LUỒNG A: Electron App (Desktop)

### Trigger

User vào `Settings → Orca CLI → click toggle` → confirm dialog.

### Chi tiết bước

```
CliSection.tsx
  handleInstall()
  → window.api.cli.install()
        ↓
  preload/index.ts:1960
  cli.install: () => ipcRenderer.invoke('cli:install')
        ↓  [Electron IPC channel: 'cli:install']
  main/ipc/cli.ts:65
  ipcMain.handle('cli:install', async () => {
    await hydrateLocalShellPathForCli(true)   // hydrate PATH từ login shell
    return new CliInstaller().install()
  })
        ↓
  main/cli/cli-installer.ts → install()
    1. getStatus()           → kiểm tra trạng thái hiện tại
    2. resolveLauncherPath() → tìm launcher binary
       - Packaged: resources/bin/orca (bundled)
       - Dev mode: generate dev launcher script
    3. Theo platform:
       - macOS  → installSymlink()
                  symlink: /usr/local/bin/orca → launcher
                  nếu EACCES → privilegedRunner (osascript)
       - Linux  → installSymlink()
                  symlink: ~/.local/bin/orca-ide → launcher
       - Windows → installWindowsWrapper()
                   wrapper .cmd file + add to user PATH registry
    4. Return CliInstallStatus
        ↓
  Renderer: setStatus(next)
  toast.success("Registered `orca` in PATH.")
```

### Key files

| File | Vai trò |
|------|---------|
| [`src/renderer/src/components/settings/CliSection.tsx`](../../src/renderer/src/components/settings/CliSection.tsx) | UI — toggle Enable/Disable + state management |
| [`src/renderer/src/components/settings/CliRegistrationDialog.tsx`](../../src/renderer/src/components/settings/CliRegistrationDialog.tsx) | Confirmation dialog |
| [`src/preload/index.ts:1958`](../../src/preload/index.ts) | IPC bridge — `window.api.cli.*` |
| [`src/main/ipc/cli.ts`](../../src/main/ipc/cli.ts) | IPC handler — `ipcMain.handle('cli:install')` |
| [`src/main/cli/cli-installer.ts`](../../src/main/cli/cli-installer.ts) | Core logic — symlink/wrapper trên filesystem |
| [`src/shared/cli-install-types.ts`](../../src/shared/cli-install-types.ts) | Type definitions: `CliInstallStatus`, `CliInstallState` |

---

## LUỒNG B: Web Browser (b15.openledger.vn)

### Behavior

Khi user truy cập qua browser và mở `Settings → Orca CLI`:

```
CliSection.tsx
  → window.api.cli.getInstallStatus()
  ← { supported: false,
       state: 'unsupported',
       unsupportedReason: 'launch_mode_unavailable',
       detail: 'CLI registration is managed on the Orca server...' }

  → isBrowserManaged = true
  → Toggle button KHÔNG render (ẩn hoàn toàn)
  → Chỉ hiển thị thông tin trạng thái (không có action)
```

**Lý do:** Trong browser mode, `window.api.cli` là **stub cứng** defined trong:

```typescript
// src/renderer/src/web/web-preload-api.ts:2479-2502
function createCliApi() {
  const status = {
    supported: false,
    state: 'unsupported',
    unsupportedReason: 'launch_mode_unavailable',
    detail: 'CLI registration is managed on the Orca server, not in the web browser.'
  }
  return {
    getInstallStatus: () => Promise.resolve(status),
    install: () => Promise.resolve(status),  // no-op
    remove: () => Promise.resolve(status),
    // ...
  }
}
```

### CLI thực sự được install khi nào?

CLI **tự động được cài** mỗi khi **Orca Server container khởi động** (không cần user trigger):

```
docker compose up (172.20.2.39)
  → node out/server/index.js
        ↓
  server/index.ts → initializeOrcaServices()
        ↓
  main/index.ts (headless serve branch, line ~2120)
  // "orca CLI command is normally installed by renderer onboarding.
  //  Headless serve has no renderer."
  if (platform === 'darwin' || platform === 'linux') {
    new CliInstaller({
      privilegedRunner: async () => {
        throw new Error('must not request admin privileges')
        // serve không được popup osascript admin prompt
      }
    }).install()  // idempotent — chạy mỗi lần server boot
  }

  // Kết quả trên Linux container:
  ~/.local/bin/orca-ide → launcher binary (symlink)

  // Thêm bare `orca` dispatcher (Linux only):
  installLinuxBareOrcaDispatcher()
  → ~/.local/bin/orca → dispatcher script
  // (để `orca claude-teams` etc. resolve được)
```

**Lý do dùng `orca-ide` trên Linux:** GNOME Orca (screen reader) đã chiếm `/usr/bin/orca` trên hầu hết distro — Orca IDE dùng `orca-ide` để tránh conflict.

### Key files

| File | Vai trò |
|------|---------|
| [`src/renderer/src/web/web-preload-api.ts:2479`](../../src/renderer/src/web/web-preload-api.ts) | Web stub API — CLI luôn trả `unsupported` |
| [`src/server/index.ts`](../../src/server/index.ts) | Server entry point — gọi `initializeOrcaServices()` |
| [`src/main/index.ts:2120`](../../src/main/index.ts) | Auto-install CLI khi headless serve boot |
| [`src/main/cli/linux-bare-orca-dispatcher.ts`](../../src/main/cli/linux-bare-orca-dispatcher.ts) | Dispatcher script `orca` → `orca-ide` (Linux) |

---

## Kết quả sau khi CLI được cài

### Electron / macOS
```bash
/usr/local/bin/orca → /Applications/Orca.app/Contents/Resources/cli/bin/launcher
# hoặc arm64:
~/.local/bin/orca   → ...launcher
```

### Orca Server (Docker, Linux)
```bash
~/.local/bin/orca-ide → /opt/orca/app/out/cli/bin/launcher
~/.local/bin/orca     → dispatcher script
```

---

## Sơ đồ tổng thể

```
┌─────────────────────────────────────────────────────────────────┐
│ ELECTRON DESKTOP                                                │
│ CliSection.tsx → window.api.cli.install()                       │
│   → preload IPC bridge                                          │
│   → ipcMain.handle('cli:install')                               │
│   → CliInstaller.install()                                      │
│   → symlink/wrapper trên local filesystem                       │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ WEB BROWSER                                                     │
│ CliSection.tsx → window.api.cli.getInstallStatus()              │
│   → web-preload-api.ts stub                                     │
│   ← { unsupported, launch_mode_unavailable }                    │
│   → Toggle ẩn, không có action                                  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ORCA SERVER (172.20.2.39) — Auto on boot                        │
│ docker compose up → node out/server/index.js                    │
│   → initializeOrcaServices()                                    │
│   → CliInstaller.install() [no admin, idempotent]               │
│   → ~/.local/bin/orca-ide (symlink)                             │
│   → ~/.local/bin/orca (dispatcher)                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Điểm cần recheck

- [ ] Xác nhận `hydrateLocalShellPathForCli()` — behavior khi PATH shell không load được
- [ ] Xác nhận `privilegedRunner` (osascript) flow trên macOS packaged build
- [ ] Verify `installLinuxBareOrcaDispatcher` — dispatcher script content và PATH resolution
- [ ] Check: web mode có cách nào trigger CLI install qua admin API không? (future feature?)
- [ ] Confirm `ORCA_MULTI_USER=1` — CLI install behavior per-user hay shared?
