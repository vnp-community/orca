# Orca Dev Server Agent — Technical Design Document v4
## Index & Overview

**Version:** 4.0 (as-implemented)  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`  
**Baseline TDD:** [`../00-index.md`](../00-index.md)

---

## Tài liệu trong bộ TDD v4

| # | File | Nội dung | Status |
|---|------|---------|--------|
| 1 | [01-architecture.md](./01-architecture.md) | Process model, lifecycle, connection modes | ✅ |
| 2 | [02-wire-protocol.md](./02-wire-protocol.md) | Binary wire protocol — 13-byte header format | ✅ |
| 3 | [03-connection-modes.md](./03-connection-modes.md) | direct-ws vs relay-ws, token flow, reconnect | ✅ |
| 4 | [04-handshake-session.md](./04-handshake-session.md) | Handshake flow, keepalive, session management | ✅ |
| 5 | [05-tool-registry.md](./05-tool-registry.md) | Tool discovery, registration, dispatch | ✅ |
| 6 | [06-tool-handlers.md](./06-tool-handlers.md) | Claude Code, git, gh, shell, fs, docker tools | ✅ |
| 7 | [07-jsonrpc-dispatch.md](./07-jsonrpc-dispatch.md) | JSON-RPC 2.0 over binary frames, error handling | ✅ |
| 8 | [08-deployment.md](./08-deployment.md) | systemd service, start.sh, env vars, auto-reconnect | ✅ |

---

## Kiến trúc tổng thể

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ORCA DEV SERVER AGENT                            │
│                    deploy/dev/agent/agent.js (Node.js)             │
│                                                                     │
│  ┌───────────────┐   ┌─────────────────┐   ┌───────────────────┐   │
│  │  Connection   │   │  Wire Protocol  │   │  Tool Registry    │   │
│  │  Manager      │   │  (13-byte hdr)  │   │  discoverTools()  │   │
│  │               │   │                 │   │                   │   │
│  │  Mode 1:      │   │  encodeFrame()  │   │  claude_code ✓    │   │
│  │  direct-ws    │   │  decodeFrame()  │   │  gh ✓             │   │
│  │               │   │                 │   │  git ✓            │   │
│  │  Mode 2:      │   │  SEQ tracking   │   │  gitnexus ✓       │   │
│  │  relay-ws     │   │  ACK tracking   │   │  codegraph ✓      │   │
│  └───────┬───────┘   └────────┬────────┘   │  docker ✓         │   │
│          │                    │            │  shell ✓           │   │
│          └────────────────────┘            │  read_file ✓       │   │
│                       │                   │  list_dir ✓         │   │
│                       ▼                   └─────────────────────┘   │
│              ┌─────────────────┐                    │               │
│              │ Session Handler  │                    │               │
│              │ handleSession()  │◄───────────────────┘               │
│              │                  │                                    │
│              │  Handshake       │                                    │
│              │  Keepalive 10s   │                                    │
│              │  JSON-RPC route  │                                    │
│              └─────────────────┘                                    │
└─────────────────────────────────────────────────────────────────────┘
               │ WebSocket (binary frames)
               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    ORCA SERVER                                      │
│  AgentWebSocketServer (direct-ws) / RelayWebSocketClient (relay-ws) │
│  → SshChannelMultiplexer → FrameDecoder                            │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Key Constraints (Bất biến)

| Constraint | Lý do |
|-----------|-------|
| `shell: false` trong spawn() | Ngăn shell injection |
| Frame TYPE = `0x01` (Regular) | Server SILENTLY IGNORES non-Regular → handshake timeout |
| SEQ u32 tăng dần, ACK = highest SEQ nhận từ server | Ngăn SshChannelMultiplexer timeout (20s) |
| Keepalive interval ≤ 15s | TIMEOUT_MS=20s — cần gửi trước timeout |
| exit(2) on connection drop | Token one-time use → systemd restart → token mới |
| `AGENT_TOKEN` chỉ trong env var | Không log token |

---

## Environment Variables

| Variable | Required | Description |
|----------|---------|-------------|
| `ORCA_URL` | ✅ | WebSocket URL: `wss://<orca-host>/<path>` |
| `AGENT_TOKEN` | ✅ (direct-ws) | One-time token từ `POST /api/agent-token` |
| `RELAY_URL` | ✅ (relay-ws) | Relay WebSocket URL |
| `RELAY_TOKEN` | ✅ (relay-ws) | Relay authentication token |
| `WORK_DIR` | ❌ | Working directory for tool commands (default: ~) |
| `AGENT_NAME` | ❌ | Display name (default: hostname) |

---

## Deployment Model

```
Remote Dev Server (Linux/macOS/Windows)
│
├─ /etc/systemd/system/orca-agent.service
│   ExecStart=/opt/orca/start.sh
│   Restart=on-failure
│   RestartSec=5
│
└─ /opt/orca/start.sh
    # 1. GET /api/agent-token → lấy AGENT_TOKEN mới
    # 2. AGENT_TOKEN=<token> ORCA_URL=... node agent.js
    # 3. exit(2) → systemd restart → start.sh lại → fresh token
```
