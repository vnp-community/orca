# TASK-DS-003 — Implement `preflight.setGitIdentity` + `preflight.detectGhosttyConfig` + `preflight.detectWindowsTerminalCapabilities`

**Solution:** [SOL-DS-002](../solutions/SOL-DS-002-agent-relay-rpc-methods.md)  
**Bug:** [BUG-DS-002](../BUG-DS-002-rpc-method-mismatch.md)  
**File:** `deploy/dev/agent/agent.js`  
**Phụ thuộc:** TASK-DS-002 phải hoàn thành trước (helpers `execSafe`, `whichTool` cần có sẵn)  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm 3 RPC method handlers vào `dispatchRpc()`:
1. `preflight.setGitIdentity` — set `user.name` và `user.email` trong git global config
2. `preflight.detectGhosttyConfig` — tìm file config Ghostty terminal
3. `preflight.detectWindowsTerminalCapabilities` — detect WSL, PowerShell, Git Bash (Windows only)

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — vị trí `case 'preflight.check':` vừa thêm ở TASK-DS-002
- `src/main/ipc/onboarding-ipc.ts` dòng 205-300 — response format

**Format response:**
```javascript
// preflight.setGitIdentity: null (void)

// preflight.detectGhosttyConfig:
{ configPath: string | null, themeDir: string | null }

// preflight.detectWindowsTerminalCapabilities:
{
  wslAvailable: boolean, wslDistros: string[],
  pwshAvailable: boolean, pwshVersion?: string,
  gitBashAvailable: boolean, gitBashPath?: string
}
```

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/agent/agent.js`

Thêm 3 case này ngay SAU `case 'preflight.check': { ... }`:

```javascript
    case 'preflight.setGitIdentity': {
      const { name, email } = msg.params ?? {};
      if (!name || !email) {
        response = {
          jsonrpc: '2.0', id: msg.id,
          error: { code: -32602, message: 'params.name and params.email are required' },
        };
        break;
      }
      // Escape double quotes trong name/email
      const safeName  = String(name).replace(/"/g, '\\"');
      const safeEmail = String(email).replace(/"/g, '\\"');
      execSafe(`git config --global user.name "${safeName}"`);
      execSafe(`git config --global user.email "${safeEmail}"`);
      response = { jsonrpc: '2.0', id: msg.id, result: null };
      break;
    }

    case 'preflight.detectGhosttyConfig': {
      const os   = require('os');
      const path = require('path');
      const fs   = require('fs');
      const home = os.homedir();
      // Ghostty config paths per platform
      const configCandidates = {
        darwin: path.join(home, 'Library', 'Application Support', 'com.mitchellh.ghostty', 'config'),
        linux:  path.join(home, '.config', 'ghostty', 'config'),
      };
      const configPath = configCandidates[process.platform] ?? null;
      let foundConfig = null;
      if (configPath) {
        try { fs.accessSync(configPath); foundConfig = configPath; } catch { /* not found */ }
      }
      // themeDir: sibling 'themes' directory
      const themeDir = foundConfig
        ? (() => {
            const d = path.join(path.dirname(foundConfig), 'themes');
            try { fs.accessSync(d); return d; } catch { return null; }
          })()
        : null;
      response = {
        jsonrpc: '2.0', id: msg.id,
        result: { configPath: foundConfig, themeDir },
      };
      break;
    }

    case 'preflight.detectWindowsTerminalCapabilities': {
      if (process.platform !== 'win32') {
        // Linux/macOS: trả về defaults (agent không chạy trên Windows ở đây)
        response = {
          jsonrpc: '2.0', id: msg.id,
          result: {
            wslAvailable:    false,
            wslDistros:      [],
            pwshAvailable:   !!whichTool('pwsh'),
            pwshVersion:     undefined,
            gitBashAvailable: false,
            gitBashPath:     undefined,
          },
        };
        break;
      }
      // Windows: detect WSL + PowerShell + Git Bash
      const wslOut     = execSafe('wsl --list --quiet 2>nul');
      const wslDistros = wslOut ? wslOut.split(/\r?\n/).filter(Boolean) : [];
      const pwshPath   = whichTool('pwsh');
      const pwshVer    = pwshPath ? getToolVersion('pwsh') : null;
      const gitWhere   = execSafe('where git 2>nul');
      const gitBashPath = gitWhere ? gitWhere.split(/\r?\n/)[0].trim() : null;
      response = {
        jsonrpc: '2.0', id: msg.id,
        result: {
          wslAvailable:    wslDistros.length > 0,
          wslDistros,
          pwshAvailable:   !!pwshPath,
          pwshVersion:     pwshVer ?? undefined,
          gitBashAvailable: !!gitBashPath,
          gitBashPath:     gitBashPath ?? undefined,
        },
      };
      break;
    }
```

---

## Verify

```bash
# Sau khi deploy agent mới:
# Trong Orca UI: Onboarding → "Set Git Identity" → nhập name + email → Save
# Expected: không còn timeout, git config được set

# Verify git config:
ssh ubuntu@172.20.2.31 "git config --global user.name && git config --global user.email"
```

---

## Definition of Done

- [x] `case 'preflight.setGitIdentity'` set git config đúng (sử dụng `git config --global`)
- [x] `case 'preflight.detectGhosttyConfig'` trả về config path hoặc null
- [x] `case 'preflight.detectWindowsTerminalCapabilities'` trả về defaults trên Linux
- [x] Input escaping cho name/email để tránh shell injection
- [x] agent.js syntax check OK (`node --check`)
