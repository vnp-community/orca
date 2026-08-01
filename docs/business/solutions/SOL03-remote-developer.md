# SOL03 — Giải pháp cho Remote Developer (Carlos)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | SOL03 |
| **Actor** | Remote Developer — Carlos |
| **Tham chiếu Painpoints** | [PP03](../painpoints/PP03-remote-developer.md) |
| **Tính năng Orca liên quan** | F07, F02, F12 |

---

## Tổng quan giải pháp

Orca biến remote development thành trải nghiệm **"local-quality remote"** — SSH được abstracted hoàn toàn, auto-reconnect làm mất kết nối trở nên transparent, và port forwarding xảy ra tự động không cần config thủ công.

---

## Giải pháp cho từng Painpoint

### SOL03-01: Giải quyết PP03-01 — SSH Session Mất Kết Nối Liên Tục

**Giải pháp: Transparent Auto-Reconnect với Agent Continuity**

Orca tự động phát hiện mất kết nối và reconnect — quan trọng hơn, agent process vẫn tiếp tục chạy trên server trong khi đang reconnect.

**Cơ chế hoạt động:**
1. Orca phát hiện SSH drop (socket close, keepalive timeout)
2. Hiển thị "Reconnecting..." indicator trong UI — không crash, không hang
3. Exponential backoff reconnect: 1s → 2s → 4s → 8s → 16s → 30s (max)
4. **Agent continuity**: Claude Code/Codex tiếp tục chạy trên server
5. Output được buffer trong thời gian mất kết nối
6. Khi reconnected: flush buffer, resume terminal state

**Kết quả đo lường được:**
- Reconnect success rate: > 95% tự động, không cần can thiệp
- Reconnect time: < 10 giây (từ 2-5 phút manual)
- Agent progress mất khi drop: 0% (agent tiếp tục chạy)
- Thời gian gián đoạn effective: < 10 giây (từ 2-15 phút)

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md)

---

### SOL03-02: Giải quyết PP03-02 — Setup Môi Trường Remote Phức Tạp

**Giải pháp: Auto-Deploy Orca Relay Binary**

Orca tự động deploy relay binary lên remote server — Carlos không cần setup gì thủ công. Relay cung cấp tất cả capabilities cần thiết cho agent.

**Cơ chế hoạt động:**
1. Carlos thêm SSH host mới trong Orca (hostname + auth)
2. Orca kết nối SSH → kiểm tra relay version trên server
3. Nếu chưa có hoặc outdated → upload relay binary qua SFTP
4. Verify hash integrity → start relay process
5. Relay cung cấp: terminal, file access, git operations, port scanning

**Lần đầu setup:**
- Tổng thời gian: < 5 phút (từ 2-4 giờ manual)
- Không cần cài thêm bất kỳ dependency nào trên server
- Relay tự cài đặt và configure

**Lần tiếp theo:**
- Orca check relay version → nếu match, connect ngay
- Nếu Orca update, relay tự upgrade → không cần manual

**Kết quả đo lường được:**
- Setup time: từ 2-4 giờ → < 5 phút
- Server rebuild recovery: từ 2-4 giờ → < 5 phút
- Dependency conflicts: 0 (relay là self-contained binary)

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md)

---

### SOL03-03: Giải quyết PP03-03 — Port Forwarding Thủ Công Cồng Kềnh

**Giải pháp: Auto Port Detection và Transparent Forwarding**

Orca tự động phát hiện khi agent mở port trên remote và forward về local — Carlos không cần gõ SSH tunnel command.

**Cơ chế hoạt động:**
1. Relay scan ports trên remote server theo interval
2. Phát hiện port mới mở (ví dụ: 3000 từ dev server)
3. Tự động tạo local forward: `localhost:3001 → remote:3000`
4. Notification in-app: "Port 3001 forwarded → remote:3000 [Open in Browser]"
5. Carlos click link → browser mở localhost:3001

**Multiple worktree port management:**
- Worktree A: remote:3000 → local:3001
- Worktree B: remote:3000 → local:3002
- Orca tự resolve conflict, không cần Carlos manage

**Kết quả đo lường được:**
- Port forwarding setup: từ 15-30 phút/ngày → 0
- "Forgot to forward" incidents: từ thường xuyên → 0
- Multiple port conflict: Orca auto-resolve, Carlos không cần biết

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md)

---

### SOL03-04: Giải quyết PP03-04 — Không Có File Editing Trực Tiếp Remote

**Giải pháp: Monaco Editor với Remote File Access qua Relay**

Orca cung cấp file explorer và Monaco editor cho remote files — trải nghiệm giống local editing.

**Cơ chế hoạt động:**
- File explorer hiển thị remote filesystem qua relay
- Click file → Monaco editor mở, đọc content qua relay
- Edit → autosave → relay ghi file lên remote
- Git status badges hiển thị đúng trạng thái remote files
- Latency được optimize: client-side optimistic updates

**Kết quả đo lường được:**
- Editing experience: gần bằng local (so với vim qua SSH hoặc SFTP lag)
- Không cần mở VSCode Remote SSH riêng biệt
- Workflow thống nhất: edit + terminal + agent trong cùng app

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md), [F12 File Explorer & Editor](../../features/F12-file-explorer-editor.md)

---

### SOL03-05: Giải quyết PP03-05 — Không Biết Agent Status Khi Mất Kết Nối

**Giải pháp: Agent Status Persistence + Mobile Notifications**

Relay trên server lưu agent status. Khi Carlos reconnect, trạng thái được sync ngay lập tức. Mobile companion cho phép check status mà không cần reconnect.

**Cơ chế hoạt động:**
- Relay buffer agent output và status khi Carlos offline
- Reconnect → flush buffer → Carlos thấy đầy đủ những gì đã xảy ra
- Mobile app: Carlos kiểm tra agent status từ điện thoại mà không cần mở laptop
- Push notification: "Claude Code trên dev-server-1 đã hoàn thành task"

**Kết quả đo lường được:**
- Số lần reconnect chỉ để check status: từ 5-10 lần/ngày → 0
- Carlos biết agent status bất cứ lúc nào qua mobile
- Thời gian để Carlos biết agent xong: từ 5-30 phút → < 30 giây

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md), [F03 Mobile Companion](../../features/F03-mobile-companion.md)

---

### SOL03-06: Giải quyết PP03-06 — Quản Lý Nhiều Server

**Giải pháp: Multi-Host Workspace với Unified Dashboard**

Orca cho phép thêm nhiều SSH hosts và quản lý tất cả từ một unified workspace sidebar.

**Cơ chế hoạt động:**
- Sidebar hiển thị tất cả SSH hosts đã cấu hình
- Mỗi host có badge status: Connected / Disconnected / Error
- Worktrees được group theo host
- Quick switch giữa projects trên các server khác nhau
- Đọc tự động từ `~/.ssh/config` — không cần nhập lại

**Kết quả đo lường được:**
- Thời gian tìm đúng server: từ 2-5 phút → < 10 giây
- Nhầm lẫn server: từ thường xuyên → 0 (visual indicator rõ ràng)
- Không cần nhớ hostnames: Orca đọc từ SSH config

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md)

---

### SOL03-07: Giải quyết PP03-07 — Git Operations Chậm

**Giải pháp: Git Operations Chạy Natively Trên Remote**

Git commands được thực thi trực tiếp trên remote server bởi relay — không có round-trip network overhead từ local tới remote.

**Cơ chế hoạt động:**
- `git fetch` chạy trực tiếp trên server (server ↔ GitHub, không qua laptop)
- Progress được stream về Orca real-time
- Carlos xem progress ngay trong Orca mà không cần shell vào server
- Large clone chạy parallel với Carlos làm việc khác

**Kết quả đo lường được:**
- Git operations speed: bằng với chạy trực tiếp trên server
- Visual progress: Carlos thấy được % completion
- Không block workflow: git chạy background

**Tính năng Orca:** [F07 SSH Worktrees](../../features/F07-ssh-worktrees.md)

---

## Tổng hợp ROI cho Carlos

| Painpoint | Trước Orca | Sau Orca | Tiết kiệm/ngày |
|-----------|-----------|---------|----------------|
| SSH drop + reconnect | 10-50 phút | < 1 phút | 9-49 phút |
| Setup môi trường | 2-4 giờ (1-2x/tháng) | < 5 phút | ~2 giờ/tháng |
| Port forwarding | 15-30 phút | 0 | 15-30 phút |
| File editing remote | 20-40 phút | < 5 phút | 15-35 phút |
| Check agent status | 10-20 phút | 0 | 10-20 phút |
| Quản lý nhiều server | 10-20 phút | 1-2 phút | 8-18 phút |
| **TỔNG** | **1-3 giờ/ngày** | **< 15 phút/ngày** | **45-165 phút/ngày** |

**Carlos tăng được 1-3 giờ productive time mỗi ngày** — tương đương 20-40% năng suất tăng thêm.

---

*Tham chiếu: PP03 Painpoints, PRD §3.6 (F07 SSH Worktrees)*
