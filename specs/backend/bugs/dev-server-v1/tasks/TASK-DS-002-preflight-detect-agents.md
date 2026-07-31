# TASK-DS-002 — Implement `preflight.detectAgents` + `preflight.check`

**Solution:** [SOL-DS-002](../solutions/SOL-DS-002-agent-relay-rpc-methods.md)  
**Bug:** [BUG-DS-002](../BUG-DS-002-rpc-method-mismatch.md)  
**File:** `deploy/dev/agent/agent.js`  
**Estimated:** 45 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm 2 RPC method handlers vào `dispatchRpc()` trong agent.js:
1. `preflight.detectAgents` — phát hiện CLI tools (claude, gh, git, docker...)
2. `preflight.check` — kiểm tra gh auth status và git config

Đây là các methods được gọi đầu tiên trong onboarding flow.

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — hàm `dispatchRpc()`, xem cấu trúc switch/case hiện tại
- `src/main/ipc/onboarding-ipc.ts` dòng 98-116 — để hiểu format response mong đợi

**Format response (TDD-13 §5):**
```javascript
// preflight.detectAgents:
{ agents: string[], platform: NodeJS.Platform }
// Ví dụ: { agents: ['gh', 'git', 'claude'], platform: 'linux' }

// preflight.check:
{
  platform: NodeJS.Platform,
  gh: { installed: boolean, authenticated: boolean, version?: string },
  git: { installed: boolean, version?: string, hasUserName: boolean, hasUserEmail: boolean }
}
```

---

## Thay Đổi Cần Thực Hiện

### Bước 1: Thêm helper functions

**File:** `deploy/dev/agent/agent.js`

Tìm dòng `// ── RPC dispatch` hoặc `function dispatchRpc(`. Thêm ngay TRƯỚC hàm `dispatchRpc`:

```javascript
// ── Shell exec helpers cho relay RPC methods ───────────────────────
const { execSync: _execSync, execFile: _execFile } = require('child_process');

function execSafe(cmd, opts = {}) {
  try {
    return _execSync(cmd, { encoding: 'utf8', timeout: 5000, stdio: ['ignore','pipe','ignore'], ...opts }).trim();
  } catch { return null; }
}

function whichTool(name) {
  return execSafe(`which ${name}`) || execSafe(`command -v ${name}`);
}

function getToolVersion(cmd) {
  try { return _execSync(`${cmd} --version 2>/dev/null`, { encoding: 'utf8', timeout: 3000 }).split('\n')[0].trim(); }
  catch { return null; }
}
```

### Bước 2: Thêm case handlers trong `dispatchRpc()`

Tìm trong `dispatchRpc()` đoạn:
```javascript
    case 'agent.info':
      response = { ... };
      break;

    default:
```

Thêm 2 case MỚI giữa `case 'agent.info':` và `default:`:

```javascript
    case 'preflight.detectAgents': {
      // TDD-13 §5: { agents: string[], platform: NodeJS.Platform }
      const cmds = msg.params?.commands ?? [];
      const detected = [];
      // Nếu server truyền commands array, detect từng tool theo đó
      if (Array.isArray(cmds) && cmds.length > 0) {
        for (const cmd of cmds) {
          const toolName = typeof cmd === 'string' ? cmd : cmd.name;
          if (toolName && whichTool(toolName)) detected.push(toolName);
        }
      } else {
        // Fallback: detect các tools thường dùng
        for (const tool of ['claude', 'gh', 'git', 'docker', 'node', 'npm', 'python3', 'code']) {
          if (whichTool(tool)) detected.push(tool);
        }
      }
      response = {
        jsonrpc: '2.0', id: msg.id,
        result: { agents: detected, platform: process.platform },
      };
      break;
    }

    case 'preflight.check': {
      // TDD-13 §4.2: gh + git status
      const ghPath  = whichTool('gh');
      const gitPath = whichTool('git');

      let ghAuthenticated = false;
      if (ghPath) {
        try { _execSync('gh auth status', { timeout: 5000, stdio: 'ignore' }); ghAuthenticated = true; }
        catch { /* not authenticated */ }
      }

      response = {
        jsonrpc: '2.0', id: msg.id,
        result: {
          platform: process.platform,
          gh: {
            installed:     !!ghPath,
            authenticated: ghAuthenticated,
            version:       ghPath ? getToolVersion('gh') ?? undefined : undefined,
          },
          git: {
            installed:    !!gitPath,
            version:      gitPath ? getToolVersion('git') ?? undefined : undefined,
            hasUserName:  !!execSafe('git config --global user.name'),
            hasUserEmail: !!execSafe('git config --global user.email'),
          },
        },
      };
      break;
    }
```

---

## Verify

```bash
# Deploy agent mới:
bash deploy/dev/scripts/connect-agent.sh --deploy

# Restart agent:
ssh ubuntu@172.20.2.31 "sudo systemctl restart orca-agent-direct"

# Test qua Orca UI:
# Settings → Dev Servers → [server] → connect
# Onboarding → "Detect Agents" → phải trả về danh sách tools ✅ (không timeout)
# Onboarding → "Prerequisites Check" → phải hiện gh/git status ✅

# Hoặc test trực tiếp qua log:
bash deploy/dev/scripts/connect-agent.sh --logs
# Tìm dòng: "[rpc] preflight.detectAgents" và "[rpc] preflight.check"
```

---

## Definition of Done

- [x] Helper functions `execSafe`, `whichTool`, `getToolVersion` đã thêm trước `dispatchRpc`
- [x] `case 'preflight.detectAgents'` xử lý đúng, trả về `{agents, platform}`
- [x] `case 'preflight.check'` xử lý đúng, trả về gh+git status
- [x] Fallback detect các tools phổ biến khi `commands` rỗng
- [x] agent.js syntax check OK (`node --check`)
