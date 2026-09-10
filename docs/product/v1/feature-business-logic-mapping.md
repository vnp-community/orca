# Bảng Mapping: Feature ↔ Nghiệp vụ

**Tài liệu:** Feature to Business Logic Mapping  
**Ngày:** 2026-07-21 | **Cập nhật:** 2026-07-28  
**Tham chiếu:** [features/](./features/), [logic/](./logic/)

---

## 1. Ma trận tổng quan (Feature × Business Logic)

Ký hiệu:
- ● **Implements** — Feature là nền tảng kỹ thuật thực thi nghiệp vụ này
- ○ **Supports** — Feature hỗ trợ hoặc liên quan gián tiếp

| Nghiệp vụ | F01<br>Parallel<br>WT | F02<br>Terminal<br>Splits | F03<br>Mobile | F04<br>AI Agent | F05<br>Design<br>Mode | F06<br>GitHub<br>Linear | F07<br>SSH<br>WT | F08<br>Annotate<br>Diffs | F09<br>Orca<br>CLI | F11<br>Notif. | F12<br>File<br>Expl. | F14<br>Auto. | F15<br>Comp.<br>Use | **F22**<br>Web<br>Server | **F23**<br>Auth | **F24**<br>Sandbox | **F25**<br>Admin | **F26**<br>Multi<br>DB | **F27**<br>Fleet | **F28**<br>Onboard | **F29**<br>Agent<br>WS | **F30**<br>Remote<br>Int. |
|-----------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **Worktree Management** |
| BL-WT-01 Tạo Worktree | ● | | | | | | ○ | | ○ | | | | |
| BL-WT-02 Fan-out Worktrees | ● | ○ | | ● | | | | | ○ | ○ | | | |
| BL-WT-03 Xóa Worktree | ● | | | | | | | | ○ | | | ○ | |
| BL-WT-04 So sánh Worktrees | ● | | | | | | | ○ | | | | | |
| BL-WT-05 Merge Worktree | ● | | | | | ○ | | | | | | | |
| **Agent Orchestration** |
| BL-AG-01 Khởi động Agent | | | | ● | | | | | ○ | | | | |
| BL-AG-02 Dừng Agent | | | | ● | | | | | ○ | | | | |
| BL-AG-03 Resume Session | | | | ● | | | | | | | | | |
| BL-AG-04 Switch Account | | | ○ | ● | | | | | | ○ | | | |
| BL-AG-05 Monitor Status | | | ○ | ● | | | | | ○ | ● | | | |
| **Terminal Management** |
| BL-TM-01 Tạo PTY Session | ○ | ● | | | | | ○ | | | | | | |
| BL-TM-02 Split Terminal | | ● | | | | | | | | | | | |
| BL-TM-03 Scrollback Persistence | | ● | | | | | | | | | | | |
| BL-TM-04 Shell Integration | | ● | | ○ | | | | | | | | | |
| **Remote Development** |
| BL-SSH-01 Kết nối SSH | | | | | | | ● | | | | | | |
| BL-SSH-02 Deploy Relay | | | | | | | ● | | | | | | |
| BL-SSH-03 Auto-Reconnect | | | | | | | ● | | | | | | |
| BL-SSH-04 Port Forwarding | | | | | | | ● | | | | | | |
| **Code Review** |
| BL-CR-01 Xem Diff | | | | | | ○ | | ● | | | | | |
| BL-CR-02 Annotate Diff | | | | | | | | ● | | | | | |
| BL-CR-03 Gửi Feedback Agent | | | | | | | | ● | | | | | |
| BL-CR-04 Generate Commit | | | | | | ● | | | | | | | |
| BL-CR-05 Tạo Pull Request | | | | | | ● | | ○ | | | | | |
| **Project Integration** |
| BL-PI-01 Import Issues | | | | | | ● | | | | | | | |
| BL-PI-02 Worktree từ Task | ● | | | | | ● | | | | | | | |
| BL-PI-03 Update Issue Status | | | | | | ● | | | | | | | |
| BL-PI-04 Submit PR Review | | | | | | ● | | ● | | | | | |
| **Mobile Companion** |
| BL-MB-01 Pair Device | | | ● | | | | | | | | | | |
| BL-MB-02 Push Notification | | | ● | ○ | | | | | | ● | | | |
| BL-MB-03 Remote Dispatch | | | ● | | | | | | | | | | |
| BL-MB-04 Mobile Status | | | ● | | | | | | | | | | |
| **Automation** |
| BL-AT-01 Cấu hình Automation | | | | | | | | | ○ | | | ● | |
| BL-AT-02 Chạy theo Schedule | | | ○ | | | | | | ● | | | ● | |
| BL-AT-03 Event Trigger | | | | ○ | | ○ | | | ● | | | ● | |
| BL-AT-04 Cleanup Worktrees | ○ | | | | | | | | ● | | | ● | |
| **Design & Browser** |
| BL-DB-01 Capture UI Element | | | | | ● | | | | | | | | ○ | | | | | | | | | |
| BL-DB-02 Inject Context Agent | | | | | ● | | | | | | | | | | | | | | | | | |
| BL-DB-03 Viewport Testing | | | | | ● | | | | | | | | | | | | | | | | | |
| **CLI & Headless** |
| BL-CLI-01 Tạo Worktree CLI | ○ | | | | | | | | ● | | | | | | | | | | | | | |
| BL-CLI-02 Quản lý Agent CLI | | | | ○ | | | | | ● | | | | | | | | | | | | | |
| BL-CLI-03 Headless Mode | | | | | | | | | ● | | | | | ○ | | | | | | | | |
| **Authentication & User Mgmt** |
| BL-AUTH-01 Local Login | | | | | | | | | | | | | | ● | ● | ○ | | | | | | |
| BL-AUTH-02 Session Management | | | | | | | | | | | | | | ● | ● | | | | | | | |
| BL-AUTH-03 Per-User Sandbox | | | | | | | | | | | | | | ● | | ● | | | | | | |
| BL-AUTH-04 Admin User CRUD | | | | | | | | | | | | | | | | | ● | | | | | |
| BL-AUTH-05 Audit Log | | | | | | | | | | | | | | | | | ● | | | | | |
| **Fleet Management** |
| BL-FLEET-01 Fleet Inventory | | | | | | | | | | | | | | ○ | | | | | ● | ○ | | |
| BL-FLEET-02 Bulk Provisioning | | | | | | | | | ○ | | | | | | | | | | ● | ○ | | |
| BL-FLEET-03 Health Monitoring | | | | | | | | | | | | | | ○ | | | | | ● | | | |
| BL-FLEET-04 Onboarding Wizard | | | | | | | | | | | | | | | | | | | | ● | | |
| **Agent WebSocket** |
| BL-AWS-01 relay-websocket | | | | | | | ○ | | | | | | | ● | | | | | | | ● | |
| BL-AWS-02 direct-websocket | | | | | | | | | | | | | | ● | | | | | | | ● | |
| BL-AWS-03 Token Management | | | | | | | | | | | | | | | ○ | | ○ | | | | ● | |
| **Remote Integrations** |
| BL-INT-01 CLI Auth Proxy | | | | | | ○ | ○ | | | | | | | ○ | | | | | | | | ● |
| BL-INT-02 WebCredentialStore | | | | | | ○ | | | | | | | | ● | | | | | | | | ● |
| BL-INT-03 Preflight Merge | | | | | | ○ | ○ | | | | | | | ● | | | | | | ○ | | ● |

---

## 2. Feature → Business Logic (Từng Feature)

### F01 — Parallel Worktrees

**Nghiệp vụ implement (●):** 5
**Nghiệp vụ support (○):** 4

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-WT-01 | Tạo Worktree | ● Implements |
| BL-WT-02 | Fan-out Worktrees | ● Implements |
| BL-WT-03 | Xóa Worktree | ● Implements |
| BL-WT-04 | So sánh Worktrees | ● Implements |
| BL-WT-05 | Merge Worktree | ● Implements |
| BL-PI-02 | Worktree từ Task | ● Implements |

**Kết luận:** F01 là tính năng nền tảng nhất, implement 6 nghiệp vụ core của Orca

---

### F02 — Terminal Splits

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-TM-01 | Tạo PTY Session | ● Implements |
| BL-TM-02 | Split Terminal | ● Implements |
| BL-TM-03 | Scrollback Persistence | ● Implements |
| BL-TM-04 | Shell Integration | ● Implements |

**Kết luận:** F02 implement hoàn toàn Terminal Management domain (4/4 nghiệp vụ)

---

### F03 — Mobile Companion App

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-MB-01 | Pair Device | ● Implements |
| BL-MB-02 | Push Notification | ● Implements |
| BL-MB-03 | Remote Dispatch | ● Implements |
| BL-MB-04 | Mobile Status | ● Implements |

**Kết luận:** F03 implement hoàn toàn Mobile Companion domain (4/4 nghiệp vụ)

---

### F04 — AI Agent Support

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AG-01 | Khởi động Agent | ● Implements |
| BL-AG-02 | Dừng Agent | ● Implements |
| BL-AG-03 | Resume Session | ● Implements |
| BL-AG-04 | Switch Account | ● Implements |
| BL-AG-05 | Monitor Status | ● Implements |
| BL-WT-02 | Fan-out Worktrees | ● Implements |

**Kết luận:** F04 implement hoàn toàn Agent Orchestration domain (5/5 nghiệp vụ). Là tính năng infrastructure quan trọng nhất.

---

### F05 — Design Mode

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-DB-01 | Capture UI Element | ● Implements |
| BL-DB-02 | Inject Context Agent | ● Implements |
| BL-DB-03 | Viewport Testing | ● Implements |

**Kết luận:** F05 implement hoàn toàn Design & Browser domain (3/3 nghiệp vụ)

---

### F06 — GitHub & Linear Integration

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-CR-04 | Generate Commit Message | ● Implements |
| BL-CR-05 | Tạo Pull Request | ● Implements |
| BL-PI-01 | Import Issues | ● Implements |
| BL-PI-02 | Worktree từ Task | ● Co-implements |
| BL-PI-03 | Update Issue Status | ● Implements |
| BL-PI-04 | Submit PR Review | ● Co-implements |

**Kết luận:** F06 implement toàn bộ Project Integration domain (4/4 nghiệp vụ) và một phần Code Review

---

### F07 — SSH Worktrees

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-SSH-01 | Kết nối SSH | ● Implements |
| BL-SSH-02 | Deploy Relay | ● Implements |
| BL-SSH-03 | Auto-Reconnect | ● Implements |
| BL-SSH-04 | Port Forwarding | ● Implements |

**Kết luận:** F07 implement hoàn toàn Remote Development domain (4/4 nghiệp vụ)

---

### F08 — Annotate AI Diffs

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-CR-01 | Xem Diff | ● Implements |
| BL-CR-02 | Annotate Diff | ● Implements |
| BL-CR-03 | Gửi Feedback Agent | ● Implements |
| BL-PI-04 | Submit PR Review | ● Co-implements |

**Kết luận:** F08 implement core Code Review workflow (3/5 nghiệp vụ)

---

### F09 — Orca CLI

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-CLI-01 | Tạo Worktree CLI | ● Implements |
| BL-CLI-02 | Quản lý Agent CLI | ● Implements |
| BL-CLI-03 | Headless Mode | ● Implements |
| BL-AT-02 | Chạy theo Schedule | ● Co-implements |
| BL-AT-03 | Event Trigger | ● Co-implements |
| BL-AT-04 | Cleanup Worktrees | ● Co-implements |

**Kết luận:** F09 implement CLI & Headless domain (3/3) + hỗ trợ Automation

---

### F11 — Notifications

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AG-05 | Monitor Status | ● Co-implements |
| BL-MB-02 | Push Notification | ● Co-implements |
| BL-WT-02 | Fan-out completion | ○ Supports |

**Kết luận:** F11 là supporting feature cho Agent Monitoring và Mobile

---

### F14 — Automations

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AT-01 | Cấu hình Automation | ● Implements |
| BL-AT-02 | Chạy theo Schedule | ● Co-implements |
| BL-AT-03 | Event Trigger | ● Co-implements |
| BL-AT-04 | Cleanup Worktrees | ● Co-implements |

**Kết luận:** F14 implement Automation domain (4/4 nghiệp vụ)

---

### F15 — Computer Use

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-DB-01 | Capture UI Element | ○ Supports |

**Kết luận:** F15 là emerging feature, hỗ trợ advanced UI automation

### F22 — Web Server Mode

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AUTH-01 | Local Login | ● Implements |
| BL-AUTH-02 | Session Management | ● Implements |
| BL-AUTH-03 | Per-User Sandbox | ● Implements |
| BL-AUTH-04 | Admin User CRUD | ○ Supports |
| BL-INT-02 | WebCredentialStore | ● Implements |
| BL-INT-03 | Preflight Merge | ● Implements |
| BL-CLI-03 | Headless Mode | ○ Supports |

**Kết luận:** F22 là foundation cho tất cả server-mode features (F23, F24, F25, F26)

---

### F23 — Multi-User Authentication

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AUTH-01 | Local Login | ● Implements |
| BL-AUTH-02 | Session Management | ● Implements |

**Kết luận:** F23 implement Authentication domain (2/5 BL-AUTH-*)

---

### F24 — Per-User Process Sandbox

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AUTH-03 | Per-User Process Sandbox | ● Implements |

**Kết luận:** F24 implement isolation architecture

---

### F25 — Admin Panel

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AUTH-04 | Admin User CRUD | ● Implements |
| BL-AUTH-05 | Audit Log | ● Implements |

**Kết luận:** F25 implement Admin domain (2/5 BL-AUTH-*)

---

### F26 — Multi-Database Support

*F26 là infrastructure layer — không implement BL cụ thể mà enable tất cả BL có persistence.*

**Kết luận:** F26 là horizontal infrastructure feature

---

### F27 — Fleet Health Monitoring

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-FLEET-01 | Fleet Inventory Config | ● Implements |
| BL-FLEET-02 | Bulk Provisioning | ● Implements |
| BL-FLEET-03 | Fleet Health Monitoring | ● Implements |

**Kết luận:** F27 implement 3/4 Fleet Management domain

---

### F28 — Dev Server Onboarding

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-FLEET-04 | Dev Server Onboarding Wizard | ● Implements |
| BL-FLEET-01 | Fleet Inventory Config | ○ Supports |
| BL-FLEET-02 | Bulk Provisioning | ○ Supports |

**Kết luận:** F28 implement Onboarding Wizard (1/4 Fleet Management)

---

### F29 — Agent WebSocket Protocol

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-AWS-01 | relay-websocket Mode | ● Implements |
| BL-AWS-02 | direct-websocket Mode | ● Implements |
| BL-AWS-03 | Agent Token Management | ● Implements |

**Kết luận:** F29 implement hoàn toàn Agent WebSocket domain (3/3)

---

### F30 — Remote Source Control Integrations

| Mã | Nghiệp vụ | Vai trò |
|----|----------|--------|
| BL-INT-01 | CLI Auth Proxy (GitHub/GitLab) | ● Implements |
| BL-INT-02 | WebCredentialStore | ● Implements |
| BL-INT-03 | Preflight Status Merge | ● Implements |

**Kết luận:** F30 implement hoàn toàn Remote Integration domain (3/3)

---

## 3. Business Logic Domain → Feature Dependency

| Domain | Nghiệp vụ | Feature chính | Feature phụ |
|--------|----------|--------------|------------|
| Worktree Management | BL-WT-01 đến WT-05 | **F01** | F09, F06 |
| Agent Orchestration | BL-AG-01 đến AG-05 | **F04** | F11, F09 |
| Terminal Management | BL-TM-01 đến TM-04 | **F02** | F07 |
| Remote Development | BL-SSH-01 đến SSH-04 | **F07** | F12 |
| Code Review | BL-CR-01 đến CR-05 | **F08**, **F06** | — |
| Project Integration | BL-PI-01 đến PI-04 | **F06** | F01 |
| Mobile Companion | BL-MB-01 đến MB-04 | **F03** | F11, F04 |
| Automation | BL-AT-01 đến AT-04 | **F14**, **F09** | F03 |
| Design & Browser | BL-DB-01 đến DB-03 | **F05** | F15 |
| CLI & Headless | BL-CLI-01 đến CLI-03 | **F09** | F04 |
| Auth & User Mgmt | BL-AUTH-01 đến AUTH-05 | **F22**, **F23**, **F24**, **F25** | — |
| Fleet Management | BL-FLEET-01 đến FLEET-04 | **F27**, **F28** | F22 |
| Agent WebSocket | BL-AWS-01 đến AWS-03 | **F29** | F22 |
| Remote Integrations | BL-INT-01 đến INT-03 | **F30** | F06, F07 |

---

## 4. Xếp hạng Feature theo số nghiệp vụ implement

| Hạng | Feature | Nghiệp vụ implement (●) | Domain bao phủ |
|------|---------|------------------------|---------------|
| 1 | **F04 AI Agent Support** | 6 | Agent Orchestration (100%) |
| 2 | **F01 Parallel Worktrees** | 6 | Worktree Management (100%) |
| 3 | **F06 GitHub & Linear** | 6 | Project Integration (100%) + Code Review |
| 4 | **F09 Orca CLI** | 6 | CLI & Headless (100%) + Automation |
| 5 | **F07 SSH Worktrees** | 4 | Remote Development (100%) |
| 6 | **F02 Terminal Splits** | 4 | Terminal Management (100%) |
| 7 | **F03 Mobile Companion** | 4 | Mobile Companion (100%) |
| 8 | **F08 Annotate AI Diffs** | 4 | Code Review (80%) |
| 9 | **F14 Automations** | 4 | Automation (100%) |
| 10 | **F29 Agent WebSocket** | 3 | Agent WebSocket (100%) |
| 11 | **F30 Remote Integrations** | 3 | Remote Integrations (100%) |
| 12 | **F22 Web Server Mode** | 7 | Auth + Multi foundation |
| 13 | **F23 Multi-User Auth** | 2 | Auth domain |
| 14 | **F25 Admin Panel** | 2 | Admin domain |
| 15 | **F27 Fleet Monitoring** | 3 | Fleet Management (75%) |
| 16 | **F05 Design Mode** | 3 | Design & Browser (100%) |
| 17 | **F11 Notifications** | 2 | Supporting |
| 18 | **F28 Dev Server Onboarding** | 1 | Fleet onboarding |
| 19 | **F24 Per-User Sandbox** | 1 | Auth isolation |
| 20 | **F15 Computer Use** | 0 (1 support) | Emerging |
| 21 | **F26 Multi-Database** | 0 | Infrastructure |

---

## 5. Nghiệp vụ không có Feature tương ứng

Tất cả 56 nghiệp vụ đều có feature implement.  
Một số nghiệp vụ có **coverage chưa đầy đủ** cần spec mở rộng:

| Nghiệp vụ | Feature hiện tại | Gap |
|----------|-----------------|-----|
| BL-WT-04 So sánh Worktrees | F01, F08 | Cần UI spec cho compare view |
| BL-AG-04 Switch Account | F04 | Cần UI spec cho account switcher |
| BL-MB-02 Push Notification (offline) | F03 | Cần spec cho offline buffering |
| BL-AT-03 Event Trigger | F14, F09 | Cần spec cho event bus |
| BL-AUTH-02 Session renewal | F22, F23 | TTL auto-renew vs explicit renew policy |

---

*Tham chiếu: [logic/README.md](./logic/README.md), [features/README.md](./features/README.md) — Cập nhật 2026-07-28 (56 nghiệp vụ, F22-F30 thêm mới)*
