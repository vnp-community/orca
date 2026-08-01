# Luồng Dữ liệu — Remote Development

**Domain:** Remote Development (SSH)  
**Nghiệp vụ:** BL-SSH-01 → BL-SSH-04  
**Kiến trúc tham chiếu:** HLD v1 — Relay Binary, C3.3/C3.5, F07 SSH Worktrees

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) | UI | SSH host dialog, relay status, port forward panel |
| Main Process | Business Logic | SshManager, RelayManager, PortForwardManager |
| ssh2 Library | Transport | SSH connection management |
| SFTP | Transport | Upload relay binary |
| Orca Relay Binary | Remote | Node.js binary chạy trên remote host |
| Remote Host | Infrastructure | Linux/macOS server với git, shell |
| SQLite Database | Persistence | SSH host config, connection state |

---

## BL-SSH-01 — Kết nối SSH Host

```
Người dùng (Carlos/DevOps)
    │
    ▼
[Renderer] mở "Add SSH Host" dialog
    │ Input: { host, port, user, authMethod: 'key' | 'password', keyPath? }
    │ contextBridge.invoke('ssh.connect', { hostConfig })
    ▼
[Main Process — SshManager.connect()]
    ├─ Load SSH private key từ keyPath (hoặc prompt password)
    ├─ ssh2.connect({ host, port, username, privateKey })
    │   → TCP connection → SSH handshake → authentication
    ├─ Test connection: exec('echo orca-ok')
    ├─ INSERT orca_ssh_hosts { id, host, port, user, keyPath }   ← SQLite
    └─ emit: ssh:connected { hostId }
    │
    ▼
[Renderer] SSH host xuất hiện trong sidebar với status "connected"

Luồng:
User → Renderer → IPC → Main → ssh2.connect() → TCP/SSH → Remote Host
                              → SQLite (INSERT host config)
                              → Renderer (ssh:connected)
```

---

## BL-SSH-02 — Deploy Orca Relay Binary

```
Người dùng (Carlos/DevOps) — sau khi SSH connected
    │
    ▼
[Renderer] click "Deploy Relay" trên SSH host
    │ contextBridge.invoke('ssh.deployRelay', { hostId })
    ▼
[Main Process — RelayManager.deploy()]
    ├─ Đọc relay binary từ Orca app bundle
    │   Path: resources/relay/relay-linux-x64 (hoặc arm64)
    ├─ SFTP upload:
    │   sftp.put(localRelayPath, '~/.orca/bin/orca-relay')
    ├─ SSH exec: chmod +x ~/.orca/bin/orca-relay
    ├─ SSH exec: ~/.orca/bin/orca-relay --version (verify)
    ├─ SSH exec: ~/.orca/bin/orca-relay start --ws-port 6799
    │   → Relay WebSocket server listening on remote:6799
    ├─ Tạo SSH tunnel: localPort:6799 → remote:6799
    └─ UPDATE orca_ssh_hosts SET relayDeployed=1, relayPort=6799  ← SQLite
    │
    ▼
[Main Process] WebSocket connection đến relay:
    ws://localhost:<tunnelPort> → SSH tunnel → remote:6799
    │
    ▼
[Renderer] relay status: "connected" — remote workspace available

Luồng:
User → Renderer → IPC → Main → SFTP (upload binary)
                              → SSH exec (chmod + start relay)
                              → SSH tunnel (port forward)
                              → WebSocket (ws://localhost:tunnel) → Relay
                              → SQLite (UPDATE host status)
```

---

## BL-SSH-03 — SSH Auto-Reconnect

```
[System] SSH connection drop (network error / timeout)
    │ ssh2 error event: 'error' hoặc 'end'
    ▼
[Main Process — SshManager.handleDisconnect()]
    ├─ emit: ssh:disconnected { hostId, reason }
    ├─ Start reconnect loop:
    │   attempt 1: wait 2s → ssh2.connect()
    │   attempt 2: wait 4s → ssh2.connect()
    │   attempt 3: wait 8s → ssh2.connect()
    │   ...exponential backoff, max 60s
    ├─ Nếu reconnect OK:
    │   ├─ Re-establish SSH tunnel (port forwarding)
    │   ├─ Re-connect WebSocket → Relay
    │   ├─ Resume active PTY sessions
    │   └─ emit: ssh:reconnected { hostId }
    └─ Nếu fail 5 lần: emit: ssh:unreachable → alert user
    │
    ▼
[Renderer] status indicator: "Reconnecting..." → "Connected" hoặc "Unreachable"

Luồng:
Network drop → ssh2 error → Main (detect) → retry loop
                          → Renderer (status update)
                          → [reconnect OK] → tunnel restore → relay re-connect
```

---

## BL-SSH-04 — Auto Port Forwarding

```
[Relay Binary — Agent Running on Remote]
    │ Port scanner: detect ports mới bind (0.0.0.0 hoặc localhost)
    │ relay protocol: { type: 'port:opened', port: 3000, pid: 1234 }
    ▼
[Main Process — PortForwardManager.onNewPort()]
    ├─ Tạo SSH local forward: localPort → remote:3000
    │   ssh2.forwardOut(localhost, localPort, remote, 3000)
    ├─ INSERT port_forwards { localPort, remotePort, pid, hostId }  ← SQLite
    ├─ Generate local URL: http://localhost:<localPort>
    └─ emit: portForward:created { localUrl, remotePort }
    │
    ▼
[Renderer] hiển thị port forward card:
    "Port 3000 → http://localhost:8080" với [Open in Browser] button

USER manually:
    User → Renderer → IPC → Main → ssh2 port forward → Remote port

Luồng:
Remote agent starts → Relay scanner → relay protocol → Main
                                                      → SSH forward
                                                      → SQLite
                                                      → Renderer (port card)
```

---

## Sơ đồ tổng quan — Remote Development

```
┌─────────────┐   IPC   ┌──────────────────────────────────────┐
│  Renderer   │◄───────►│  Main Process                        │
│  SSH panel  │         │  SshManager (ssh2)                   │
│  Port fwds  │         │  RelayManager                        │
│  Status     │         │  PortForwardManager                  │
└─────────────┘         └──────────┬───────────────────────────┘
                                   │ SSH (ssh2 library)
                         ┌─────────▼─────────────────────────┐
                         │  SSH Tunnel (TCP port forward)     │
                         └─────────┬─────────────────────────┘
                                   │
                    ─── Internet / LAN ───
                                   │
                         ┌─────────▼─────────────────────────┐
                         │  Remote Host                       │
                         │  ┌──────────────────────────────┐ │
                         │  │  Orca Relay Binary            │ │
                         │  │  - PTY handler               │ │
                         │  │  - FS handler                │ │
                         │  │  - Git handler               │ │
                         │  │  - Port scanner              │ │
                         │  │  WS :6799                    │ │
                         │  └──────────────────────────────┘ │
                         │  Shell + Git + Agent process       │
                         └────────────────────────────────────┘
```
