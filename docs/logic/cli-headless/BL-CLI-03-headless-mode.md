# BL-CLI-03 — Chạy Orca ở Headless Mode

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-CLI-03 |
| **Tên** | Chạy Orca ở Headless Mode (Không GUI) |
| **Nhóm** | CLI & Headless |
| **Actors** | DevOps Engineer |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F09 Orca CLI |
| **SRS** | FR-9.2 |

---

## Mô tả nghiệp vụ

Chạy Orca daemon mà không cần GUI — phù hợp cho Linux headless server, Docker container, và CI/CD environment.

---

## Command Interface

```bash
# Khởi động headless daemon
orca serve --port 7777 --daemon

# Check daemon status
orca daemon status

# Stop daemon
orca daemon stop
```

---

## Luồng chính

```
1. `orca serve` được gọi
2. Electron main process khởi động (không có renderer)
3. Daemon mở Unix socket tại ~/.local/share/orca/daemon.sock
4. HTTP API khởi động tại localhost:7777 (nếu --port được chỉ định)
5. Daemon ready → write PID file
6. CLI commands hoạt động bình thường qua socket
```

---

## Headless Architecture

```
CI/CD Script
     │
     │ Unix socket / HTTP
     ▼
Orca Daemon (headless)
     │
     ├── Git operations
     ├── PTY / node-pty
     ├── Agent processes
     └── SQLite persistence
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-CLI-08 | Headless mode không được require display (X11, Wayland, Xvfb) |
| BR-CLI-09 | Daemon phải graceful shutdown khi nhận SIGTERM |
| BR-CLI-10 | PID file ở ~/.local/share/orca/daemon.pid để prevent duplicate |
| BR-CLI-11 | HTTP API phải require authentication token khi exposed |
| BR-CLI-12 | Docker-compatible: không có GUI dependencies trong headless mode |

---

## SLO

| Metric | Target |
|--------|--------|
| Daemon startup time | < 2 giây |
| Command response time | < 500ms |
| Memory (headless, idle) | < 150MB |
