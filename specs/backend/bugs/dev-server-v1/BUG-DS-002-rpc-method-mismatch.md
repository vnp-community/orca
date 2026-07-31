# BUG-DS-002 — Agent Không Implement Relay RPC Methods

**ID:** BUG-DS-002  
**Mức độ:** 🔴 Critical  
**Module:** agent.js RPC dispatch / Orca onboarding & remote workspace  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

Agent chỉ implement MCP tool interface (`tools/call`, `tools/list`). Orca server gọi nhiều relay methods khác qua `relay.call()` → `SshChannelMultiplexer.request()` → forward đến agent. Agent không biết xử lý các method này → trả về `MethodNotFound (-32601)` → server timeout sau 15-30s → UI đóng băng hoặc hiện error.

---

## Root Cause

**Orca IPC handlers** gọi relay methods mà agent KHÔNG implement:

```
onboarding-ipc.ts:
  relay.detectAgents(commands)      → "preflight.detectAgents"
  relay.call('preflight.check')     → check gh, git, disk
  relay.call('preflight.setGitIdentity')
  relay.call('preflight.detectGhosttyConfig')
  relay.call('preflight.detectWindowsTerminalCapabilities')
  relay.call('pty.spawn', {command: 'gh', args: ['auth', 'login'], ...})

repo-remote-ipc.ts:
  relay.call('fs.listDirectory', ...)
  relay.call('fs.stat', ...)
  relay.call('git.clone', ...)
  relay.call('fs.listWorkspaces', ...)
```

**agent.js dispatchRpc** chỉ xử lý:
```javascript
case 'ping':
case 'agent.ping':  // ✅
case 'tools/list':  // ✅
case 'tools/call':  // ✅
case 'agent.info':  // ✅
default:
  // Trả về MethodNotFound -32601 nếu msg.id có
  // Nếu server không nhận response → timeout
```

**Hậu quả theo feature**:

| Feature | Method | Hậu quả |
|---------|--------|---------|
| Onboarding: detect agents | `preflight.detectAgents` | Timeout 15s → onboarding fail |
| Onboarding: preflight check | `preflight.check` | Timeout 30s → UI error |
| Onboarding: gh auth | `pty.spawn` | Timeout → không mở terminal |
| Remote workspace: browse | `fs.listDirectory` | Timeout → không browse được |
| Remote workspace: clone | `git.clone` | Timeout → clone fail |
| Git identity setup | `preflight.setGitIdentity` | Timeout → settings không save |

---

## Chi Tiết Kỹ Thuật

`SshChannelMultiplexer.request()` (server side) có built-in timeout:
- `detectAgents` timeout: **15s** (callWithTimeout param)  
- `preflight.check` timeout: **30s**
- `git.clone` timeout: **30s**
- Mặc định: `30s`

Khi agent trả về `{error: {code: -32601}}` → server reject promise → IPC handler throw error → renderer nhận error state.

---

## Phạm Vi Impact

Tất cả tính năng phụ thuộc relay calls KHÔNG hoạt động với agent mới:
- ❌ Onboarding flow
- ❌ Remote workspace browser
- ❌ Git clone từ UI
- ❌ gh auth login
- ❌ Git identity setup
- ✅ Agent kết nối thành công (handshake)
- ✅ MCP tools (nếu dùng qua claude)

---

## Fix

Agent cần implement các relay methods tương đương. Ưu tiên theo impact:

### Priority 1: `preflight.detectAgents`

```javascript
case 'preflight.detectAgents': {
  const commands = msg.params?.commands ?? [];
  const agents = [];
  for (const cmd of commands) {
    try {
      execSync(`which ${cmd.name} 2>/dev/null`);
      agents.push(cmd.name);
    } catch {}
  }
  response = { jsonrpc: '2.0', id: msg.id,
    result: { agents, platform: process.platform } };
  break;
}
```

### Priority 2: `preflight.check`

```javascript
case 'preflight.check': {
  const gh = checkTool('gh');
  const git = checkTool('git');
  response = { jsonrpc: '2.0', id: msg.id,
    result: {
      platform: process.platform,
      gh: { installed: gh.ok, authenticated: false, version: gh.version },
      git: { installed: git.ok, version: git.version,
             hasUserName: !!execSafe('git config user.name'),
             hasUserEmail: !!execSafe('git config user.email') }
    }
  };
  break;
}
```

### Priority 3: `pty.spawn`, `fs.*`, `git.clone`

Cần implement đầy đủ hơn — tham khảo relay binary protocol (SSH relay) để biết exact response format.

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `deploy/dev/agent/agent.js` | Bug location — dispatchRpc switch |
| `src/main/ipc/onboarding-ipc.ts` | Gọi `preflight.*`, `pty.spawn` |
| `src/main/ipc/repo-remote-ipc.ts` | Gọi `fs.*`, `git.clone` |
| `src/main/dev-server/dev-server-relay-bridge.ts` | `call()`, `detectAgents()`, `callWithTimeout()` |
| `src/main/ssh/ssh-channel-multiplexer.ts` | `request()` — timeout logic |
