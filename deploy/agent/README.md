# Orca Dev Server Agent v2.2

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
    "agentVersion": "2.2.0",
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
    "nodeVersion":"v22.0.0", "agentVersion":"2.2.0", "sessionId":"sess-xxx"
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

## Nâng cấp lên 2.2.0 (CR-REQ-033)

Khi triển khai phiên bản 2.2.0 hỗ trợ `agent.capabilities`, `agent.execPrompt` (với chế độ readonly), và `ai.complete`:

1. **Thứ tự triển khai (BẮT BUỘC):** Agent trước, backend sau.
   - Backend mới chỉ gọi các RPC method mới khi handshake chứa `features` tương ứng từ agent mới. Triển khai agent trước loại bỏ hoàn toàn cửa sổ lỗi tương thích ngược.
2. **Triển khai thủ công:**
   ```bash
   node agent/build.mjs
   scp agent/out/agent.js ubuntu@<devserver>:~/orca-agent/agent.js
   ssh ubuntu@<devserver> "sudo systemctl restart orca-agent"
   ```
3. **Lưu ý với relay-ssh:**
   - Cảnh báo: `sshrelay/provisioner.go:141` so sánh `version == p.cfg.OrcaVersion` (với `OrcaVersion` là biến `ORCA_VERSION` của backend) để quyết định bỏ qua việc đẩy bundle. Nếu `ORCA_VERSION` trùng với chuỗi `AGENT_VERSION` cũ trên máy xa, bundle mới sẽ không tự động được đẩy.
   - Người vận hành cần kiểm tra biến `ORCA_VERSION` của backend; nếu muốn buộc sshrelay đẩy lại bundle mới, hãy cấu hình `ORCA_VERSION` khớp phiên bản mới (`2.2.0`) hoặc xoá file bundle cũ trên máy xa.
4. **Cách xác nhận sau nâng cấp:**
   - Kết nối lại và gọi RPC `agent.capabilities`: kỳ vọng `agent.buildVersion == "2.2.0"`, `agent.protocolVersion == 2`.
   - Kiểm tra log handshake trên server: ghi nhận các tính năng `features: ["agent.capabilities", "agent.execPrompt", "ai.complete"]`.

---

## Supported RPC Methods

| Method | Mô tả |
|--------|-------|
| `agent.capabilities` | Trả về báo cáo khả năng agent (tools, envs, platform, protocolVersion) |
| `agent.execPrompt` | Thực thi AI prompt với tuỳ chọn readonly/print mode, timeout, env, workDir |
| `ai.complete` | Trực tiếp hoàn thành prompt thông qua Anthropic API relay |
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

---

## Kill Switch / Tắt Tính Năng Tại Máy (CR-073)

Quản trị viên có thể tắt các phân hệ Code Intelligence hoặc Quality Gate tại Dev Server khi cần giảm tải hoặc xử lý sự cố.

### Biến môi trường (`.env` trên Dev Server)

| Biến | Giá trị | Ý nghĩa |
|---|---|---|
| `ORCA_CODEINTEL_DISABLED` | `1` (hoặc `true`/`on`) | **Tắt toàn bộ Code Intelligence** (fail-closed: mọi giá trị lạ ngoài 0/false/off đều bị coi là tắt). |
| `ORCA_CODEINTEL_REINDEX` | `off` (hoặc `0`/`false`) | Tắt tính năng reindex (mặc định bật). |
| `ORCA_QUALITY_RUN` | `off` (hoặc `0`/`false`) | Tắt tính năng quality gate runner (mặc định bật). |

### Các bước áp dụng
1. Thêm hoặc cập nhật biến trong file `.env` tại thư mục cài agent (hoặc `~/.env`):
   ```bash
   echo "ORCA_CODEINTEL_DISABLED=1" >> ~/orca-agent/.env
   ```
2. Restart dịch vụ agent:
   ```bash
   sudo systemctl restart orca-agent
   ```
3. Xác minh trạng thái:
   - **Qua RPC `codeintel.status`**: Trả về `ok: true` nhưng kèm `warnings: ["codeintel_disabled"]`.
   - **Qua handshake `capabilities`**: Không còn xuất hiện `codeintel`, `codeintel:reindex`, hoặc `quality`.
   - **Lưu ý quan trọng**:
     - **KHÔNG** kiểm tra qua danh sách `tools[]` của `tools/list` vì `tools[]` dành cho tool discovery chung.
     - Các lệnh gọi trực tiếp tool CLI legacy `gitnexus` / `codegraph` qua `tools/call` vẫn chạy độc lập và không chịu ảnh hưởng bởi switch này.
     - Trong chế độ `--stdio` hoặc chạy qua SSH session, các biến môi trường được kế thừa trực tiếp từ session env.
   - *Ghi chú môi trường kiểm thử*: Đã kiểm chứng tự động qua bộ kiểm thử tự động của agent (`runtime-switches.test.ts`, `disabled-gate.test.ts`, `disabled-capabilities.test.ts`, `disabled-startup.test.ts`); chưa thử trên dev server vật lý trong môi trường local này.
