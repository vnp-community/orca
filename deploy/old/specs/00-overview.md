# Orca Dev Server Deployment — Tổng quan (v2)

**Phạm vi:** Dev Environment  
**Phiên bản:** 2.0  
**Ngày:** 2026-07-22  
**Áp dụng cho:** VNP-BLC Internal Dev Teams  

---

## 1. Mô hình triển khai — Web-First

Orca hỗ trợ chạy **`orca serve`** (headless server) và tự phục vụ **Web UI** tích hợp sẵn. Developer kết nối trực tiếp bằng **trình duyệt** hoặc **Orca Desktop App** mà **không cần cài SSH riêng**.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Developer Access Methods                          │
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │  Browser     │    │ Orca Desktop │    │ Orca Mobile  │              │
│  │ (Web UI)     │    │ (App)        │    │ (iOS/Android)│              │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘              │
└─────────│───────────────────│───────────────────│────────────────────┘
          │                   │                   │
          │   HTTPS/WSS       │   WebSocket       │   WebSocket E2E
          │  (Pairing URL)    │  (Pairing Code)   │  (QR Pairing)
          └───────────────────┴───────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     ORCA SERVER (orca serve)                             │
│                                                                         │
│  HTTP/WebSocket Server  ←─── Port 6768 (default)                        │
│  Web UI Bundle          ←─── served tại http://orca-server:6768         │
│  Runtime Engine         ←─── manages worktrees, agents, PTY             │
│  Pairing System         ←─── generates pairing URLs for clients         │
│                                                                         │
│  Ubuntu Server, 16+ cores, 64GB RAM, fast SSD                          │
└───────────┬──────────────────────┬──────────────────────────────────────┘
            │                      │
     SSH / Internal          SSH / Internal
            │                      │
            ▼                      ▼
┌───────────────────┐   ┌──────────────────────┐
│  Dev Server A     │   │  Dev Server B         │
│  (Project Alpha)  │   │  (Project Beta)        │
│  - Git repos      │   │  - Git repos           │
│  - Services       │   │  - Services            │
└───────────────────┘   └──────────────────────┘
```

---

## 2. Các tài liệu trong specs này

| File | Nội dung |
|------|---------|
| 00-overview.md | Tổng quan mô hình (file này) |
| 01-orca-server-setup.md | Cài đặt Orca Server + orca serve |
| 02-web-access-setup.md | Cấu hình Web UI, HTTPS, Nginx reverse proxy |
| 03-dev-server-setup.md | Setup từng Dev Server cho từng project |
| 04-developer-guide.md | Hướng dẫn developer dùng Browser/App/Mobile |
| 05-flow-diagrams.md | Sơ đồ luồng chi tiết (mermaid) |
| 06-security.md | Bảo mật, pairing tokens, TLS, phân quyền |
| 07-troubleshooting.md | Xử lý sự cố thường gặp |

---

## 3. Ba cách Developer kết nối

| Cách | Client | Yêu cầu | Dùng cho |
|------|--------|---------|---------|
| **Web Browser** | Chrome/Firefox/Safari | URL + Pairing code | Truy cập nhanh, không cần cài app |
| **Orca Desktop App** | Orca.app (macOS/Win/Linux) | Pairing URL hoặc code | Trải nghiệm đầy đủ, có terminal, diff viewer |
| **Orca Mobile** | iOS/Android app | QR code scan | Monitor agent, gửi follow-up từ điện thoại |

---

## 4. Yêu cầu hệ thống

### 4.1 Orca Server (Central)

| Thành phần | Tối thiểu | Khuyến nghị |
|-----------|-----------|-------------|
| OS | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| CPU | 8 cores | 16–32 cores |
| RAM | 32 GB | 64–128 GB |
| Storage | 500 GB SSD | 1–2 TB NVMe |
| Network | 100 Mbps | 1 Gbps |
| Node.js | 22+ | 22 LTS |
| Git | 2.35+ | Latest |
| Port (HTTP) | 6768 (default) | Tuỳ chọn |
| Port (HTTPS) | 443 (qua Nginx) | Bắt buộc cho web |

### 4.2 Dev Server (mỗi project)

| Thành phần | Tối thiểu | Ghi chú |
|-----------|-----------|---------|
| OS | Ubuntu 22.04 | Debian 12 cũng OK |
| SSH | Port 22 (internal only) | Orca Server → Dev Server |
| Git | 2.35+ | Bắt buộc |
| Node.js | 22+ | Để chạy AI agents |

### 4.3 Developer Client

| Cách | Yêu cầu |
|------|---------|
| Web Browser | Chrome 120+, Firefox 120+, Safari 17+ |
| Orca Desktop | v1.4+, macOS/Windows/Linux |
| Orca Mobile | iOS 15+ / Android 8+ |

---

## 5. Tổng quan luồng hoạt động

```
DevOps khởi động Orca Server
        │
        ▼
orca serve --port 6768 --pairing-address https://orca.vnpblc.internal
        │
        ▼
Orca Server in ra:
  ✓ Web UI:     https://orca.vnpblc.internal/web-index.html?pair=...
  ✓ Pairing URL: orca://pair?code=xxxxx&endpoint=wss://...
        │
        ▼
DevOps gửi pairing URL cho Developer (qua Slack/email)
        │
        ├─── Browser: Mở URL → Web UI load → Nhập pairing code → Kết nối ✅
        │
        ├─── Orca Desktop: Paste pairing URL → Kết nối ✅
        │
        └─── Orca Mobile: Scan QR → Kết nối ✅
                │
                ▼
        Developer thấy giao diện Orca đầy đủ:
          ├── Tạo Worktree
          ├── Chạy AI Agent
          ├── Terminal (PTY trên server)
          └── Review Diff, Commit, Push
```

---

## 6. Phân vùng trách nhiệm

| Vai trò | Trách nhiệm |
|--------|-------------|
| **DevOps/Infra** | Cài Orca Server, cấu hình HTTPS/Nginx, quản lý pairing links |
| **Team Lead** | Phân phối pairing URL cho dev team, cấu hình project per dev server |
| **Developer** | Nhận pairing URL, mở browser hoặc Orca app, làm việc bình thường |
