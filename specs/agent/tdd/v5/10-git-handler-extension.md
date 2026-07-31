# TDD-AG-10: Git Handler Extension (v5.0)

**Document:** TDD-AG-10 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Whitelisted git operations and streaming from Orca UI (Remote Git UI feature)
**Feature:** F39
**ADR:** ADR-012
**HLD Ref:** C3.12, C4.10
**Backend TDD:** TDD-20
**Frontend TDD:** TDD-FE-16

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. Motivation

Agent hiện có `git` tool (TDD-AG-06) cho phép bất kỳ git command từ MCP tool calls. v5.0 thêm:

1. **git.exec**: RPC method riêng (không qua tools/call) với **whitelist validation** — used by Orca UI cho Remote Git operations.
2. **git.execStream**: Streaming variant cho `push` và `pull` — sends multiple response frames.
3. **Separation**: `git` tool (CLI) vẫn giữ cho agent/LLM tool calls; `git.exec` là cho direct UI RPC.

---

## 2. git.exec — Whitelisted Git Handler

```javascript
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
]);

const SHELL_METACHARACTERS = /[&|;$`]/;

function validateGitArgs(args) {
  if (!args || args.length === 0) {
    throw new Error('GIT_NO_SUBCOMMAND');
  }
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new Error(`GIT_DISALLOWED_SUBCOMMAND: ${args[0]}`);
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new Error(`GIT_SHELL_METACHARACTER_IN_ARG: ${arg}`);
    }
  }
}

case 'git.exec':
  response = await handleGitExec(rpc);
  break;

async function handleGitExec(rpc) {
  const { cwd, args, timeout = 30000 } = rpc.params;

  try {
    validateGitArgs(args);
  } catch (err) {
    return { jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: err.message } };
  }

  const result = await runCommandCapture('git', args, {
    cwd: cwd || WORK_DIR,
    timeout: Math.min(timeout, 60000),  // max 60s for non-streaming
  });

  return {
    jsonrpc: '2.0', id: rpc.id,
    result: {
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exitCode,
    },
  };
}
```

---

## 3. git.execStream — Streaming Handler

```javascript
case 'git.execStream':
  await handleGitExecStream(ws, rpc);
  return;  // No single response — sends stream frames

async function handleGitExecStream(ws, rpc) {
  const { cwd, args } = rpc.params;

  try {
    validateGitArgs(args);
  } catch (err) {
    ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
      jsonrpc: '2.0', id: rpc.id,
      error: { code: -32602, message: err.message },
    })));
    return;
  }

  // Resolve git binary
  const gitBinary = TOOL_PATH.split(':').reduce((found, dir) => {
    if (found) return found;
    const candidate = path.join(dir, 'git');
    try { fs.accessSync(candidate, fs.constants.X_OK); return candidate; } catch { return null; }
  }, null) || 'git';

  const child = spawn(gitBinary, args, {
    cwd: cwd || WORK_DIR,
    env: TOOL_ENV,
    stdio: ['pipe', 'pipe', 'pipe'],
    shell: false,
  });

  // Stream stdout line by line
  child.stdout.on('data', chunk => {
    const lines = chunk.toString('utf8').split('\n').filter(l => l.length > 0);
    for (const line of lines) {
      if (ws.readyState !== WebSocket.OPEN) break;
      ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
        jsonrpc: '2.0', id: rpc.id,
        result: { type: 'stream.chunk', line },
      })));
    }
  });

  // Also stream stderr (git progress goes to stderr)
  child.stderr.on('data', chunk => {
    const lines = chunk.toString('utf8').split('\n').filter(l => l.length > 0);
    for (const line of lines) {
      if (ws.readyState !== WebSocket.OPEN) break;
      ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
        jsonrpc: '2.0', id: rpc.id,
        result: { type: 'stream.chunk', line, source: 'stderr' },
      })));
    }
  });

  // End frame
  child.on('close', (code) => {
    if (ws.readyState !== WebSocket.OPEN) return;
    ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
      jsonrpc: '2.0', id: rpc.id,
      result: { type: 'stream.end', exitCode: code ?? 0 },
    })));
  });

  child.on('error', (err) => {
    ws.send(encodeFrame(FRAME_TYPE.DATA, JSON.stringify({
      jsonrpc: '2.0', id: rpc.id,
      error: { code: -32603, message: err.message },
    })));
  });
}
```

---

## 4. Validation Rules

| Rule | Implementation |
|------|---------------|
| Whitelist subcommand | `ALLOWED_GIT_SUBCOMMANDS.has(args[0])` |
| No shell metacharacters | `/[&|;$`]/.test(arg)` check on each arg |
| Max timeout | 60s for exec, unlimited stream (process.kill on WS close) |
| `shell: false` | spawn(..., { shell: false }) |
| CWD validation | Defaults to WORK_DIR if not provided |

---

## 5. Error Codes

| Error | Code | When |
|-------|------|------|
| `GIT_NO_SUBCOMMAND` | -32602 | args empty |
| `GIT_DISALLOWED_SUBCOMMAND` | -32602 | args[0] not in whitelist |
| `GIT_SHELL_METACHARACTER_IN_ARG` | -32602 | shell char in any arg |
| Internal spawn error | -32603 | spawn() throws |

---

## 6. Stream Frame Types

| Frame | Meaning |
|-------|---------|
| `{ type: 'stream.chunk', line }` | One line of stdout |
| `{ type: 'stream.chunk', line, source: 'stderr' }` | One line of stderr (git progress) |
| `{ type: 'stream.end', exitCode: 0 }` | Command finished successfully |
| `{ type: 'stream.end', exitCode: N }` | Command finished with error |

---

## 7. Test Coverage

```
tests/unit/
├── git-handler.test.js
│   ├── validateGitArgs: allowed subcommand → no error
│   ├── validateGitArgs: disallowed → GIT_DISALLOWED_SUBCOMMAND
│   ├── validateGitArgs: empty args → GIT_NO_SUBCOMMAND
│   ├── validateGitArgs: metacharacter '|' → GIT_SHELL_METACHARACTER_IN_ARG
│   ├── validateGitArgs: metacharacter '&' → error
│   ├── validateGitArgs: metacharacter ';' → error
│   ├── validateGitArgs: metacharacter '$' → error
│   ├── git.exec: valid args → runCommandCapture called
│   ├── git.exec: invalid args → error frame (not crash)
│   └── git.exec: timeout capped at 60000ms
└── git-stream.test.js
    ├── git.execStream: sends stream.chunk frames for each stdout line
    ├── git.execStream: sends stream.chunk with source=stderr for stderr lines
    ├── git.execStream: sends stream.end with exitCode on close
    ├── git.execStream: invalid args → error frame immediately
    └── git.execStream: ws closed mid-stream → no more sends
```

**Target:** ≥ 20 tests; git-handler tested completely isolated from real git binary

---

## v2.1 Integration Note

**Source file:** `src/relay/git-handler.ts` — standalone module (không còn inline)

```typescript
// src/relay/git-handler.ts
import { spawn } from 'node:child_process'
import WebSocket from 'ws'
import type { WireState } from './agent-wire'
import type { AgentConfig } from './agent-config'
// validateGitArgs(), handleGitExec(), handleGitExecStream()
// Registered in agent-rpc-dispatch.ts route() switch
```

**Test file:** `src/relay/__tests__/git-handler.test.ts`
