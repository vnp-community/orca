# SOL-DS-002 — Implement Relay RPC Methods Trong Agent

**Fixes:** [BUG-DS-002](../BUG-DS-002-rpc-method-mismatch.md)  
**TDD Ref:** TDD-13 §4 (IPC handlers), TDD-13 §5 (preflight.detectAgents response format)  
**File:** `deploy/dev/agent/agent.js`  
**Effort:** 2-4 giờ  
**Status:** ✅ DONE — 2026-07-27 (TASK-DS-002, 003, 004, 005)  
**Implemented in:** `deploy/dev/agent/agent.js` — helpers + 10 RPC method cases thêm vào `dispatchRpc()`

---

## Phân Tích

Theo TDD-13 §5, relay methods phải trả về đúng format mà Orca onboarding IPC handlers expect. Response format được định nghĩa rõ trong TDD-13:

```typescript
// preflight.detectAgents response (TDD-13 §5):
{ agents: string[], platform: NodeJS.Platform }

// preflight.check response (TDD-13 §4.2):
{
  platform: NodeJS.Platform,
  gh: { installed: boolean, authenticated: boolean, version?: string },
  git: { installed: boolean, version?: string, hasUserName: boolean, hasUserEmail: boolean }
}

// preflight.setGitIdentity: void response
// pty.spawn → returns ptyId string (nhưng phức tạp — Phase 2)
// fs.listDirectory → { entries: DirectoryEntry[], platform: NodeJS.Platform }
// git.clone → { path: string }
```

---

## Thay Đổi Cần Thực Hiện

### File: `deploy/dev/agent/agent.js`

**Thêm helper functions** (trước `dispatchRpc()`):

```javascript
// ── Shell exec helper ──────────────────────────────────────────────
const { execSync, execFile } = require('child_process');

function execSafe(cmd, options = {}) {
  try {
    return execSync(cmd, { encoding: 'utf8', timeout: 5000, ...options }).trim();
  } catch {
    return null;
  }
}

function whichTool(name) {
  return execSafe(`which ${name} 2>/dev/null`) ||
         execSafe(`command -v ${name} 2>/dev/null`);
}

function getToolVersion(cmd) {
  return execSafe(`${cmd} --version 2>/dev/null`)?.split('\n')[0] ?? null;
}
```

**Thêm handlers trong `dispatchRpc()`** — vào switch/case sau `agent.info`:

```javascript
// ── preflight.detectAgents ──────────────────────────────────────────
// TDD-13 §5: { agents: string[], platform: NodeJS.Platform }
case 'preflight.detectAgents': {
  const commands = msg.params?.commands ?? [];
  const detected = [];
  for (const cmd of commands) {
    // cmd có thể là string hoặc { name, checkCmd }
    const toolName = typeof cmd === 'string' ? cmd : cmd.name;
    const checkCmd = typeof cmd === 'object' && cmd.checkCmd ? cmd.checkCmd : `which ${toolName}`;
    const found = execSafe(checkCmd);
    if (found) detected.push(toolName);
  }
  // Fallback: nếu commands rỗng, detect các tools phổ biến
  if (commands.length === 0) {
    for (const tool of ['claude', 'gh', 'git', 'docker', 'node', 'npm']) {
      if (whichTool(tool)) detected.push(tool);
    }
  }
  response = {
    jsonrpc: '2.0', id: msg.id,
    result: { agents: detected, platform: process.platform },
  };
  break;
}

// ── preflight.check ─────────────────────────────────────────────────
// TDD-13 §4.2: gh, git status
case 'preflight.check': {
  const ghPath   = whichTool('gh');
  const gitPath  = whichTool('git');
  const ghVer    = ghPath ? getToolVersion('gh') : null;
  const gitVer   = gitPath ? getToolVersion('git') : null;

  // gh auth status: 0 = authenticated
  let ghAuthenticated = false;
  if (ghPath) {
    try { execSync('gh auth status 2>/dev/null', { timeout: 5000 }); ghAuthenticated = true; }
    catch { /* not authenticated */ }
  }

  const gitUserName  = execSafe('git config --global user.name');
  const gitUserEmail = execSafe('git config --global user.email');

  response = {
    jsonrpc: '2.0', id: msg.id,
    result: {
      platform: process.platform,
      gh: {
        installed:     !!ghPath,
        authenticated: ghAuthenticated,
        version:       ghVer ?? undefined,
      },
      git: {
        installed:    !!gitPath,
        version:      gitVer ?? undefined,
        hasUserName:  !!gitUserName,
        hasUserEmail: !!gitUserEmail,
      },
    },
  };
  break;
}

// ── preflight.setGitIdentity ────────────────────────────────────────
case 'preflight.setGitIdentity': {
  const { name, email } = msg.params ?? {};
  if (!name || !email) {
    response = { jsonrpc: '2.0', id: msg.id,
      error: { code: -32602, message: 'name and email are required' } };
    break;
  }
  execSafe(`git config --global user.name "${name.replace(/"/g, '\\"')}"`);
  execSafe(`git config --global user.email "${email.replace(/"/g, '\\"')}"`);
  response = { jsonrpc: '2.0', id: msg.id, result: null };
  break;
}

// ── preflight.detectGhosttyConfig ──────────────────────────────────
case 'preflight.detectGhosttyConfig': {
  // Check common Ghostty config locations per platform
  const { homedir } = require('os');
  const configPaths = {
    darwin: `${homedir()}/Library/Application Support/com.mitchellh.ghostty/config`,
    linux:  `${homedir()}/.config/ghostty/config`,
  };
  const configPath = configPaths[process.platform] ?? null;
  let foundConfig = null;
  let themeDir = null;
  if (configPath) {
    try { require('fs').accessSync(configPath); foundConfig = configPath; } catch {}
  }
  response = {
    jsonrpc: '2.0', id: msg.id,
    result: { configPath: foundConfig, themeDir },
  };
  break;
}

// ── preflight.detectWindowsTerminalCapabilities ────────────────────
case 'preflight.detectWindowsTerminalCapabilities': {
  // Chỉ meaningful trên Windows; Linux/Mac trả về defaults
  if (process.platform !== 'win32') {
    response = {
      jsonrpc: '2.0', id: msg.id,
      result: {
        wslAvailable: false, wslDistros: [],
        pwshAvailable: !!whichTool('pwsh'), pwshVersion: undefined,
        gitBashAvailable: false, gitBashPath: undefined,
      },
    };
    break;
  }
  // Windows detection
  const wslOut      = execSafe('wsl --list --quiet 2>/dev/null');
  const wslDistros  = wslOut ? wslOut.split('\n').filter(Boolean) : [];
  const pwshPath    = whichTool('pwsh');
  const pwshVer     = pwshPath ? execSafe('pwsh --version 2>/dev/null') : null;
  const gitBashPath = execSafe('where git 2>/dev/null')?.split('\n')[0];
  response = {
    jsonrpc: '2.0', id: msg.id,
    result: {
      wslAvailable:     wslDistros.length > 0,
      wslDistros,
      pwshAvailable:    !!pwshPath,
      pwshVersion:      pwshVer ?? undefined,
      gitBashAvailable: !!gitBashPath,
      gitBashPath:      gitBashPath ?? undefined,
    },
  };
  break;
}

// ── fs.listDirectory ────────────────────────────────────────────────
// repo-remote-ipc.ts expects: { entries: DirectoryEntry[], platform }
case 'fs.listDirectory': {
  const { path: dirPath = WORK_DIR } = msg.params ?? {};
  const fs = require('fs');
  try {
    const items = fs.readdirSync(dirPath, { withFileTypes: true });
    const entries = items.map(item => ({
      name:        item.name,
      path:        require('path').join(dirPath, item.name),
      isDirectory: item.isDirectory(),
      isFile:      item.isFile(),
      isSymlink:   item.isSymbolicLink(),
    }));
    response = {
      jsonrpc: '2.0', id: msg.id,
      result: { entries, platform: process.platform },
    };
  } catch (err) {
    response = {
      jsonrpc: '2.0', id: msg.id,
      error: { code: -32000, message: err.message },
    };
  }
  break;
}

// ── fs.stat ─────────────────────────────────────────────────────────
case 'fs.stat': {
  const { path: statPath } = msg.params ?? {};
  const fs = require('fs');
  try {
    const stat = fs.statSync(statPath);
    response = {
      jsonrpc: '2.0', id: msg.id,
      result: {
        exists:      true,
        isDirectory: stat.isDirectory(),
        isFile:      stat.isFile(),
        size:        stat.size,
        mtime:       stat.mtimeMs,
      },
    };
  } catch {
    response = { jsonrpc: '2.0', id: msg.id, result: { exists: false } };
  }
  break;
}

// ── fs.listWorkspaces ────────────────────────────────────────────────
// Liệt kê git repos trong WORK_DIR
case 'fs.listWorkspaces': {
  const { path: rootPath = WORK_DIR } = msg.params ?? {};
  const fs   = require('fs');
  const path = require('path');
  const entries = [];
  try {
    const items = fs.readdirSync(rootPath, { withFileTypes: true });
    for (const item of items) {
      if (!item.isDirectory()) continue;
      const gitDir = path.join(rootPath, item.name, '.git');
      try {
        fs.accessSync(gitDir);
        entries.push({ name: item.name, path: path.join(rootPath, item.name), isGitRepo: true });
      } catch {
        entries.push({ name: item.name, path: path.join(rootPath, item.name), isGitRepo: false });
      }
    }
  } catch (err) {
    response = { jsonrpc: '2.0', id: msg.id,
      error: { code: -32000, message: err.message } };
    break;
  }
  response = { jsonrpc: '2.0', id: msg.id,
    result: { entries, platform: process.platform } };
  break;
}

// ── git.clone ────────────────────────────────────────────────────────
case 'git.clone': {
  const { url, targetDir, branch } = msg.params ?? {};
  if (!url || !targetDir) {
    response = { jsonrpc: '2.0', id: msg.id,
      error: { code: -32602, message: 'url and targetDir are required' } };
    break;
  }
  const args = ['clone', url, targetDir];
  if (branch) args.push('--branch', branch);

  // Trả về ngay với promise — git clone có thể mất vài phút
  response = null; // defer
  const cloneCmd = `git ${args.map(a => JSON.stringify(a)).join(' ')}`;
  execFile('git', args, { timeout: 300_000, cwd: WORK_DIR }, (err, stdout, stderr) => {
    const result = err
      ? { jsonrpc: '2.0', id: msg.id,
          error: { code: -32000, message: stderr || err.message } }
      : { jsonrpc: '2.0', id: msg.id,
          result: { path: targetDir } };
    log.debug(`git.clone done: ${err ? 'FAIL' : 'OK'}`);
    ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify(result)));
  });
  break;
}
```

---

## Không Implement Trong Phase Này

- `pty.spawn` — cần PTY session management (phức tạp, cần pty library)
- Streaming responses — cần protocol extension

---

## Verification

```bash
# Sau khi deploy agent mới, test qua Orca UI:
# 1. Settings → Dev Servers → [server] → Reconnect
# 2. Onboarding → "Check Prerequisites" → phải hiện gh/git status ✅
# 3. Remote Workspace → Browse Directory → phải list files ✅
# 4. Onboarding → "Detect Agents" → phải list claude/gh/git ✅

# CLI test (curl trực tiếp):
# POST /api/... → kiểm tra không còn timeout 30s
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `deploy/dev/agent/agent.js` | Thêm case handlers vào `dispatchRpc()` |
| `src/main/ipc/onboarding-ipc.ts` | Gọi các methods này qua relay |
| `src/main/ipc/repo-remote-ipc.ts` | Gọi `fs.*`, `git.clone` |
| `specs/backend/tdd/13-dev-server-onboarding.md §5` | Response format spec |
