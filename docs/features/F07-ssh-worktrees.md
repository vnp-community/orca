# F07 — SSH Worktrees

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F07 |
| **Tên** | SSH Worktrees |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.6 |
| **Tham chiếu URD** | UR-013, UR-020, UR-021, UR-022 |
| **Tham chiếu SRS** | FR-4.1, FR-4.2, FR-4.3, FR-4.4 |
| **ADR References** | — |
| **HLD References** | C3.5 |

---

## Mô tả

Chạy AI agent trên máy chủ remote qua SSH với đầy đủ file editing, git operations, và terminal — tất cả từ giao diện desktop local. Bao gồm auto-reconnect, port forwarding tự động, và Orca relay deployment.

---

## Vấn đề cần giải quyết

Developers sử dụng máy chủ remote mạnh hơn laptop để chạy AI agent (cần GPU, RAM lớn). SSH session thông thường bị mất khi mạng gián đoạn, port forwarding phức tạp, và không có giao diện thống nhất để quản lý remote worktrees.

---

## Tính năng chi tiết

### SSH Connection

**Authentication:**
- SSH key (RSA, Ed25519, ECDSA)
- Password authentication
- SSH agent forwarding
- Keyboard-interactive

**Config:**
- Đọc `~/.ssh/config` đầy đủ
- Hỗ trợ `Include` directives
- Host patterns (wildcards)
- ProxyJump support
- Tilde expansion trong paths

**Channel Multiplexing:**
- Nhiều channel trên một connection
- Tối ưu số lượng TCP connections
- Reduces latency cho concurrent operations

### Orca Relay

- **Auto-deploy**: Orca relay binary được upload tự động lên remote qua SFTP
- **Version check**: Kiểm tra version mismatch, auto-upgrade nếu outdated
- **Hash verification**: Verify binary integrity sau upload
- **Cross-platform**: Relay binary cho Linux x64/arm64

Relay capabilities:
- Terminal I/O forwarding
- File system operations (read, write, delete)
- Git command execution
- Port scanning và forwarding

### Auto-Reconnect

- Phát hiện mất kết nối (socket close, keepalive timeout)
- Hiển thị trạng thái "Reconnecting" trong UI
- Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s max
- **Agent continuity**: Agent process tiếp tục chạy trên remote khi mất kết nối
- Buffer output từ remote trong thời gian reconnect
- Flush buffer khi reconnected

### Port Forwarding

**Auto-detection:**
- Scan ports mới mở trên remote (port scanner)
- Tự động forward về local khi phát hiện port mới
- Thông báo người dùng với local URL

**Localhost Proxy:**
- Label-based routing cho multiple worktrees
- Nhiều worktree có thể forward cùng port (ví dụ: 3000) mà không conflict
- Proxy request tới đúng worktree dựa trên header

**Forwarding types:**
- Local forward: `localhost:3000 → remote:3000`
- SSH2 SOCKS proxy

### Remote Agent Execution

- Agent chạy trên remote với full environment
- Cùng trust presets như local agent
- Remote-specific trust settings

---

## Luồng người dùng

```
[Thiết lập SSH connection]
1. Menu → Add SSH Host
2. Nhập hostname (hoặc pick từ ~/.ssh/config)
3. Orca test connection và deploy relay binary
4. SSH host xuất hiện trong workspace sidebar

[Tạo remote worktree]
5. Click "New Worktree" trên SSH host
6. Orca tạo worktree trên remote filesystem
7. Terminal open kết nối tới remote
8. Start Claude Code trên remote

[Auto-reconnect]
9. Mất WiFi → Orca hiển thị "Reconnecting..."
10. Claude Code tiếp tục chạy trên server
11. WiFi back → Orca reconnect trong vài giây
12. Output từ trong khi mất kết nối được flush về UI

[Port forwarding]
13. Claude Code khởi động dev server trên remote port 3000
14. Orca phát hiện → forward localhost:3001 → remote:3000
15. Thông báo: "Port 3001 forwarded → remote:3000"
16. Click link → browser mở localhost:3001
```

---

## Tiêu chí chấp nhận

- [ ] Kết nối SSH bằng key, password, và agent auth
- [ ] Orca relay được deploy tự động, không cần manual setup
- [ ] Auto-reconnect trong < 10 giây khi mất kết nối
- [ ] Agent tiếp tục chạy trên remote khi mất kết nối local
- [ ] Port forwarding tự động khi agent mở port mới
- [ ] Multiple worktrees có thể forward cùng port không conflict

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **SSH library** | `ssh2` v1.17.0 |
| **SSH connection** | `src/main/ssh/ssh-connection.ts` (~41K bytes) |
| **SSH relay** | `src/main/ssh/ssh-relay-session.ts` (~51K bytes) |
| **SSH relay deploy** | `src/main/ssh/ssh-relay-deploy.ts` (~43K bytes) |
| **SSH config parser** | `src/main/ssh/ssh-config-parser.ts` |
| **Port forwarding** | `src/main/ssh/ssh-port-forward.ts` |
| **Port scanner** | `src/main/ssh/ssh-port-scanner.ts` |
| **File transfer** | `src/main/ssh/system-ssh-file-transfer.ts` |
| **Channel multiplexer** | `src/main/ssh/ssh-channel-multiplexer.ts` |
| **Localhost proxy** | `src/main/localhost-worktree-label-proxy.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Reconnect time | < 10 giây |
| Reconnect success rate | > 95% |
| Relay deploy time | < 30 giây (first time) |
| Port forwarding latency | < 5ms overhead |
