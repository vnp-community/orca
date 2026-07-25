# Developer Guide — Sử dụng Orca qua Browser, Desktop App và Mobile (v2)

**Đối tượng:** Developer mới onboarding hoặc cần reference  
**Điều kiện:** Orca Server đã được setup và chạy (xem 01, 02)  
**Điều kiện:** DevOps đã gửi Pairing URL cho bạn

---

## 1. Ba cách kết nối Orca Server

### So sánh nhanh

| | Browser (Web UI) | Orca Desktop App | Orca Mobile |
|--|---|---|---|
| **Cài đặt** | Không cần | Cài Orca App | Cài Orca Mobile |
| **Truy cập** | Paste URL vào browser | Paste Pairing URL | Scan QR |
| **Tính năng** | Đầy đủ | Đầy đủ + native terminal | Monitor + dispatch |
| **Platform** | Mọi OS | macOS/Win/Linux | iOS/Android |
| **Offline** | Không | Không | Không |
| **Tốt nhất cho** | Truy cập nhanh, mọi thiết bị | Làm việc chính hàng ngày | Monitor khi không ở máy |

---

## 2. Cách 1: Dùng Browser (Web UI)

> **Đây là cách đơn giản nhất. Không cần cài gì thêm.**

### Bước 1: Nhận Pairing URL từ DevOps

DevOps sẽ gửi cho bạn một trong hai:
- **Web Browser URL** (dài hơn, tự động kết nối):
  ```
  https://orca.vnpblc.internal/web-index.html?pair=orca%3A%2F%2Fpair%3Fcode%3D...
  ```
- **Pairing Code** (ngắn gọn, nhập thủ công):
  ```
  orca://pair?code=abc123xyz
  ```

### Bước 2: Mở Orca Web UI

**Option A: Dùng Web URL (tự động kết nối)**
1. Mở Chrome/Firefox/Safari
2. Paste URL vào address bar
3. Nếu dùng self-signed cert → click **"Advanced"** → **"Proceed anyway"**
4. Orca Web UI load, tự động kết nối ✅

**Option B: Mở UI rồi nhập code**
1. Mở `https://orca.vnpblc.internal`
2. Thấy màn hình **"Connect to Orca"**
3. Nhập pairing code vào field **"Pairing URL or code"**
4. Nhập tên server (tuỳ chọn): "VNP-BLC Dev"
5. Click **"Connect"**

### Bước 3: Sử dụng

Sau khi kết nối, giao diện Orca Web hiện ra đầy đủ:
- **Sidebar trái:** danh sách worktrees, projects
- **Terminal panel:** PTY chạy trực tiếp trên server
- **Editor:** file explorer và code editor
- **Diff viewer:** review AI-generated code

> **Lưu ý:** Pairing code được lưu trong `localStorage` của browser. Lần sau mở browser lại → tự động reconnect.

---

## 3. Cách 2: Dùng Orca Desktop App

> **Trải nghiệm tốt nhất với native terminal, keyboard shortcuts, và hiệu năng cao.**

### Bước 1: Cài Orca Desktop

| OS | Cách cài |
|----|----------|
| macOS | `brew install --cask stablyai/orca/orca` hoặc [download DMG](https://github.com/stablyai/orca/releases/latest/download/orca-macos-arm64.dmg) |
| Windows | [Download .exe](https://github.com/stablyai/orca/releases/latest/download/orca-windows-setup.exe) |
| Linux | [Download AppImage](https://github.com/stablyai/orca/releases/latest/download/orca-linux.AppImage) |

### Bước 2: Kết nối với Orca Server

**Option A: Paste Pairing URL trực tiếp**

1. Mở Orca Desktop
2. Orca sẽ hiện màn hình **"Connect to Orca"** nếu chưa có server
3. Paste pairing URL vào field: `orca://pair?code=abc123...`
4. Click **"Connect"**

**Option B: Thêm từ Settings**

1. Mở Orca Desktop → **Settings** (⚙️ hoặc `Cmd+,`)
2. Chọn **"Runtime Environments"** hoặc **"Remote Connections"**
3. Click **"+ Add"**
4. Chọn **"Paste pairing URL"**
5. Paste URL → Click **"Connect"**

**Option C: Deep link (từ browser)**

1. Mở Web URL trong browser
2. Nếu Orca Desktop đã cài, browser hỏi: **"Open with Orca?"** → Click **"Open"**
3. Orca Desktop tự động kết nối

### Bước 3: Làm việc

Sau khi kết nối:
- Orca Desktop hiện đầy đủ interface
- Tất cả terminals, worktrees, và agents **chạy trên server**
- Máy local chỉ hiển thị UI → không tốn CPU/RAM local

---

## 4. Cách 3: Dùng Orca Mobile

> **Monitor agent từ điện thoại, gửi follow-up prompt khi đang di chuyển.**

### Bước 1: Cài Orca Mobile

- **iOS:** [App Store](https://apps.apple.com/us/app/orca-ide/id6766130217) hoặc [TestFlight](https://testflight.apple.com/join/YjeGMQBA)
- **Android:** [Download APK](https://github.com/stablyai/orca/releases/download/mobile-android-v0.0.27/app-release.apk)

### Bước 2: Lấy QR Code từ Orca Server

DevOps chạy lệnh tạo mobile QR code:

```bash
# Trên Orca Server
orca serve --pairing-address wss://orca.vnpblc.internal --mobile-pairing --json
# → Xuất ra QR code + mobile pairing URL
```

Hoặc từ Orca Desktop đang kết nối:
- Settings → **"Mobile Pairing"** → **"Generate QR"**

### Bước 3: Scan QR

1. Mở Orca Mobile
2. Tap **"Add Server"** hoặc scan QR
3. Scan QR code từ màn hình/Slack
4. Kết nối hoàn thành trong < 30 giây

### Bước 4: Theo dõi từ Mobile

- Nhận **push notification** khi agent hoàn thành task
- Xem **status** của tất cả agents đang chạy
- Gửi **follow-up prompt** từ điện thoại về agent

---

## 5. Làm việc hàng ngày trong Orca

### 5.1 Tạo Worktree mới (làm việc trên feature mới)

1. Trong sidebar → click **"+ New Worktree"**
2. Chọn project/repo (ví dụ: `/srv/projects/vnp-blc`)
3. Chọn base branch: `develop`
4. Đặt tên branch: `feature/user-auth`
5. Chọn AI agent: **Claude Code** (hoặc Gemini, Codex...)
6. Click **"Create"**

Orca sẽ:
```
1. git worktree add .wt/feature-user-auth -b feature/user-auth develop
2. Chạy setup script từ orca.yaml (npm install, v.v.)
3. Spawn AI agent trong PTY (trên server)
```

### 5.2 Nhập prompt cho AI Agent

1. Click vào worktree panel
2. Agent terminal hiện ra (đang ở trạng thái ready)
3. Gõ hoặc paste prompt:
   ```
   Implement JWT authentication for the /api/auth/login endpoint.
   Use the existing User model in src/models/user.go.
   Add unit tests.
   ```
4. Press Enter → Agent bắt đầu làm việc

### 5.3 Fan-out: 1 Prompt → Nhiều Agents

1. Click biểu tượng **"Fan Out"** (⑆)
2. Nhập prompt
3. Chọn số agents: 2–5
4. Chọn loại agent cho từng slot
5. Click **"Run"**

Orca tạo N worktrees song song, chạy N agents. Bạn so sánh kết quả và chọn cái tốt nhất.

### 5.4 Review diff và Annotate

1. Sau khi agent xong, click **"Diff"** tab
2. Xem thay đổi từng file
3. Click vào dòng cụ thể → popup comment box
4. Viết comment: `"Fix the error handling here, should return 400 not 500"`
5. Click **"Send to Agent"** → agent nhận comment và sửa

### 5.5 Commit và Push

1. Click **"Commit"** trong panel
2. Orca gợi ý commit message (AI-generated)
3. Chỉnh nếu cần → Click **"Commit"**
4. Click **"Push"** → push lên GitHub/GitLab

---

## 6. Terminal trong Orca

- Terminal chạy **trực tiếp trên server** (không phải máy local)
- Tách terminal: `Cmd+D` (ngang), `Cmd+Shift+D` (dọc)
- Terminal sessions tồn tại kể cả khi đóng browser/app
- Reconnect → thấy lại đúng trạng thái cũ

```bash
# Trong terminal của Orca (đang chạy trên server)
cd /srv/projects/vnp-blc
git status
go run ./cmd/server

# Hoặc
make test
docker compose up -d
```

---

## 7. Quản lý API Keys cho AI Agents

Mỗi developer cần set API key của riêng mình trong Orca:

**Option A: Qua Orca UI**
- Settings → Agents → Claude Code → **API Key** → nhập `sk-ant-...`
- Key được lưu secure trong session (không persist lên server)

**Option B: Trên server (persistent)**

Nếu cả team dùng chung key (shared key theo project):
```bash
# SSH vào Orca Server → set environment variable
sudo nano /etc/systemd/system/orca-server.service

# Thêm vào [Service]:
Environment=ANTHROPIC_API_KEY=sk-ant-...
Environment=OPENAI_API_KEY=sk-...

sudo systemctl daemon-reload && sudo systemctl restart orca-server
```

---

## 8. Port Forwarding từ Server về Local

Khi service chạy trên server cần truy cập từ máy local (ví dụ: web app trên port 3000):

**Orca Desktop:** Tự động detect và hỏi → Click **"Forward Port"**

**Orca Web:** Vào **Port Forwarding** panel → thêm rule:
```
Remote Port: 3000 → Local Port: 13000
```

Sau đó truy cập `http://localhost:13000` trên máy local.

---

## 9. Onboarding Checklist (Developer mới)

- [ ] Nhận Pairing URL từ DevOps qua Slack
- [ ] Test: mở Pairing URL trong browser → thấy Orca Web UI ✅
- [ ] (Tuỳ chọn) Cài Orca Desktop → paste pairing URL → kết nối ✅
- [ ] (Tuỳ chọn) Cài Orca Mobile → scan QR → kết nối ✅
- [ ] Tạo worktree đầu tiên → chọn project → agent chạy ✅
- [ ] Set API key cho AI agent trong Settings
- [ ] Thử terminal: gõ lệnh → output từ server ✅

---

## 10. FAQ

**Q: Pairing URL expire không?**  
A: Có, mỗi lần `orca serve` restart → token mới. DevOps sẽ gửi link mới khi cần. Browser nhớ connection trong localStorage, tự reconnect nếu token còn hiệu lực.

**Q: Dữ liệu có đi qua cloud Orca không?**  
A: Không. Kết nối trực tiếp từ browser/app đến Orca Server của công ty qua WebSocket. Không có trung gian.

**Q: Browser nào được hỗ trợ?**  
A: Chrome 120+, Firefox 120+, Safari 17+, Edge 120+. Cần WebSocket support.

**Q: Nếu server restart thì session có mất không?**  
A: Terminal sessions và agents **tiếp tục chạy** (daemon quản lý riêng). Sau khi reconnect, bạn thấy lại đúng trạng thái. Chỉ cần nhập lại pairing code nếu token expire.

**Q: Nhiều developer cùng làm 1 project có conflict không?**  
A: Không. Mỗi developer làm trên **worktree riêng** (branch riêng). Agents độc lập nhau hoàn toàn.

**Q: Agent chạy trên server hay máy local?**  
A: Agent (Claude Code, Gemini...) chạy **trên Orca Server**. Máy local/browser chỉ hiển thị output. Developer không tốn CPU/RAM.
