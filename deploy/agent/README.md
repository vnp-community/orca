# Orca Dev Server Agent v2.1

Agent Node.js chạy trên **dev server** để kết nối với Orca server qua WebSocket.

> **Source code:** `agent/src/relay/agent-entry.ts` (TypeScript)  
> **Build output:** `agent/out/agent.js` (bundled, self-contained)  
> **Deploy:** `scp agent/out/agent.js ubuntu@<devserver>:~/orca-agent/agent.js`

---

## Modes

### Mode 1: direct-websocket (khuyến nghị)
Agent kết nối **VÀO** Orca. Không cần mở port trên dev server.

```
Dev Server                       Orca Server (b15.openledger.vn)
  agent.js ──── WebSocket ────► wss://<orca>/agent
               ─── agent.handshake {agentToken, platform, arch, ...} ──►
               ◄── { ok: true, orcaVersion, sessionId } ──
               ◄══ JSON-RPC (tools/call, preflight.*, fs.*, git.*) ══►
```

### Mode 2: relay-websocket
Orca kết nối **VÀO** agent. Cần mở port trên dev server.

```
Dev Server                       Orca Server (b15.openledger.vn)
  agent.js :6799  ◄── WebSocket ── Orca
  /orca-relay      ◄── Bearer token ──
                   ─── agent.handshake {orcaVersion} ──►
                   ◄── { ok:true, platform, arch, agentVersion, sessionId } ──
                   ◄══ JSON-RPC ══►
```

---

## Wire Protocol

13-byte binary header (phải khớp với `agent/src/main/ssh/relay-protocol.ts`):
```
[TYPE u8][SEQ u32 BE][ACK u32 BE][LENGTH u32 BE][PAYLOAD bytes]
```

| Type  | Value | Mô tả |
|-------|-------|-------|
| Regular   | 0x01 | JSON-RPC 2.0 payload (handshake, tools/call, RPC responses) |
| KeepAlive | 0x09 | Empty frame, gửi mỗi 5s để giữ connection alive |

---

## Handshake Protocol

### direct-websocket (agent → Orca)
Agent gửi request TRƯỚC:
```json
{
  "jsonrpc": "2.0", "id": 1, "method": "agent.handshake",
  "params": {
    "agentToken": "agt-dev-local-xxx",
    "agentVersion": "2.1.0",
    "platform": "linux",
    "arch": "x64",
    "nodeVersion": "v22.0.0",
    "capabilities": ["fs", "git", "preflight"]
  }
}
```
Orca reply thành công:
```json
{ "jsonrpc":"2.0", "id":1, "result": { "ok":true, "orcaVersion":"1.4.x", "sessionId":"sess-xxx" } }
```

### relay-websocket (Orca → agent)
Orca gửi request TRƯỚC:
```json
{ "jsonrpc":"2.0", "id":1, "method":"agent.handshake", "params": { "orcaVersion":"1.4.x" } }
```
Agent reply:
```json
{
  "jsonrpc":"2.0", "id":1, "result": {
    "ok":true, "platform":"linux", "arch":"x64",
    "nodeVersion":"v22.0.0", "agentVersion":"2.1.0", "sessionId":"sess-xxx"
  }
}
```

---

## Quick Start

### Auto (khuyến nghị)
```bash
# direct-websocket mode (từ máy dev):
bash deploy/agent/scripts/connect-agent.sh --deploy

# relay-websocket mode:
bash deploy/agent/scripts/connect-agent.sh --deploy --mode=relay-ws
```

### Manual
```bash
# 1. Build agent mới nhất
node agent/build.mjs  (hoặc: pnpm --filter orca-agent build)
# → agent/out/agent.js (self-contained, ~185KB)

# 2. Deploy lên dev server
scp agent/out/agent.js ubuntu@172.20.2.31:~/orca-agent/agent.js
ssh ubuntu@172.20.2.31 "cd ~/orca-agent && cp agent-runtime.env.example .env"
# Điền ORCA_URL, AGENT_TOKEN vào .env

# 3. Lấy agent token (direct-ws mode)
bash deploy/agent/scripts/connect-agent.sh  # prints token

# 4. Chạy agent
ssh ubuntu@172.20.2.31
ORCA_URL=wss://b15.openledger.vn/agent \
AGENT_TOKEN=agt-dev-local-XXXX \
node ~/orca-agent/agent.js
```

---

## Management

```bash
# Status
bash deploy/agent/scripts/connect-agent.sh --status

# Logs
bash deploy/agent/scripts/connect-agent.sh --logs

# Stop
bash deploy/agent/scripts/connect-agent.sh --stop
```

---

## Build & Update

```bash
# Rebuild sau khi sửa agent/src/relay/agent-*.ts
node agent/build.mjs  (hoặc: pnpm --filter orca-agent build)
# → agent/out/agent.js + agent/out/.agent-version

# Deploy lên dev server
scp agent/out/agent.js ubuntu@<devserver>:~/orca-agent/agent.js
ssh ubuntu@<devserver> "sudo systemctl restart orca-agent"
```

---

## Supported RPC Methods

| Method | Mô tả |
|--------|-------|
| `tools/list` | Trả về danh sách tools đã discover |
| `tools/call` | Gọi tool theo tên (claude_code, gh, git, shell, read_file, ...) |
| `agent.ping` | Health check |
| `agent.info` | Version, platform, workDir info |
| `preflight.check` | Kiểm tra gh/git installed + authenticated |
| `preflight.detectAgents` | Detect installed AI agents (claude, etc.) |
| `preflight.setGitIdentity` | Set git user.name + user.email |
| `fs.listDirectory` | List directory entries |
| `fs.stat` | Stat a path |
| `fs.listWorkspaces` | List git repos trong một directory |
| `git.clone` | Clone git repo (async) |
