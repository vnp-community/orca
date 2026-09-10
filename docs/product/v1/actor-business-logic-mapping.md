# Bảng Mapping: Actor ↔ Nghiệp vụ

**Tài liệu:** Actor to Business Logic Mapping  
**Ngày:** 2026-07-21 | **Cập nhật:** 2026-07-28  
**Tham chiếu:** [logic/](./logic/)

---

## 1. Ma trận tổng quan (Actor × Business Logic)

Ký hiệu:
- ● **Primary** — Actor là người khởi tạo nghiệp vụ
- ○ **Participant** — Actor tham gia hoặc hưởng lợi từ nghiệp vụ
- _(trống)_ — Không liên quan

| Nghiệp vụ | Alex<br>Senior Dev | Maya<br>Tech Lead | Carlos<br>Remote Dev | Sam<br>Mobile User | QA<br>Engineer | DevOps<br>Engineer | **Admin** | **Agent Dev** |
|-----------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **Worktree Management** |
| BL-WT-01 Tạo Worktree | ● | ● | ● | | | | | |
| BL-WT-02 Fan-out Worktrees | ● | | | | | | | |
| BL-WT-03 Xóa Worktree | ● | ● | ● | | | | | |
| BL-WT-04 So sánh Worktrees | ● | ○ | | | | | | |
| BL-WT-05 Merge Worktree | ● | ● | | | | | | |
| **Agent Orchestration** |
| BL-AG-01 Khởi động Agent | ● | ● | ● | ○ | | ○ | | |
| BL-AG-02 Dừng Agent | ● | ● | ● | ○ | | | | |
| BL-AG-03 Resume Session | ● | ● | ● | | | | | |
| BL-AG-04 Switch Account | ● | | | ● | | | | |
| BL-AG-05 Monitor Status | ● | ● | ● | ● | | ● | | |
| **Terminal Management** |
| BL-TM-01 Tạo PTY Session | ● | ○ | ● | | | | | |
| BL-TM-02 Split Terminal | ● | | ● | | | | | |
| BL-TM-03 Scrollback Persistence | ● | | ● | | | | | |
| BL-TM-04 Shell Integration | ● | ● | ○ | | | | | |
| **Remote Development** |
| BL-SSH-01 Kết nối SSH | | | ● | | | ● | | |
| BL-SSH-02 Deploy Relay | | | ● | | | ● | | |
| BL-SSH-03 Auto-Reconnect | | | ● | | | | | |
| BL-SSH-04 Port Forwarding | | | ● | | ● | | | |
| **Code Review** |
| BL-CR-01 Xem Diff | ● | ● | | | ○ | | | |
| BL-CR-02 Annotate Diff | ● | ● | | | | | | |
| BL-CR-03 Gửi Feedback Agent | ● | ● | | | | | | |
| BL-CR-04 Generate Commit | ● | ● | | | | | | |
| BL-CR-05 Tạo Pull Request | ● | ● | | | | | | |
| **Project Integration** |
| BL-PI-01 Import Issues | ● | ● | | | | | | |
| BL-PI-02 Worktree từ Task | ● | ● | | | | | | |
| BL-PI-03 Update Issue Status | | ● | | | | | | |
| BL-PI-04 Submit PR Review | | ● | | | | | | |
| **Mobile Companion** |
| BL-MB-01 Pair Device | | | | ● | | | | |
| BL-MB-02 Push Notification | | | ○ | ● | | | | |
| BL-MB-03 Remote Dispatch | | | | ● | | | | |
| BL-MB-04 Mobile Status | | | ○ | ● | | | | |
| **Automation** |
| BL-AT-01 Cấu hình Automation | | | | ● | | ● | | |
| BL-AT-02 Chạy theo Schedule | | | | ● | | ● | | |
| BL-AT-03 Event Trigger | | | | ○ | | ● | | |
| BL-AT-04 Cleanup Worktrees | ○ | | | | | ● | | |
| **Design & Browser** |
| BL-DB-01 Capture UI Element | ● | | | | ● | | | |
| BL-DB-02 Inject Context Agent | ● | | | | ● | | | |
| BL-DB-03 Viewport Testing | ○ | | | | ● | | | |
| **CLI & Headless** |
| BL-CLI-01 Tạo Worktree CLI | ○ | | | | | ● | | |
| BL-CLI-02 Quản lý Agent CLI | | | | | | ● | | |
| BL-CLI-03 Headless Mode | | | | | | ● | | |
| **Authentication & User Mgmt** |
| BL-AUTH-01 Local Login | ● | ● | ● | ● | | | ○ | |
| BL-AUTH-02 Session Management | ● | ● | ● | ● | | | ○ | |
| BL-AUTH-03 Per-User Sandbox | ● | ● | ● | ● | | | ○ | |
| BL-AUTH-04 Admin User CRUD | | | | | | | ● | |
| BL-AUTH-05 Audit Log | | | | | | | ● | ○ |
| **Fleet Management** |
| BL-FLEET-01 Fleet Inventory | | | ○ | | | ● | ● | |
| BL-FLEET-02 Bulk Provisioning | | | | | | ● | ● | |
| BL-FLEET-03 Health Monitoring | | | | | | ● | ● | |
| BL-FLEET-04 Onboarding Wizard | | | ● | | | | ○ | |
| **Agent WebSocket** |
| BL-AWS-01 relay-websocket | | | | | | | | ● |
| BL-AWS-02 direct-websocket | | | | | | | | ● |
| BL-AWS-03 Token Management | | | | | | | ○ | ● |
| **Remote Integrations** |
| BL-INT-01 CLI Auth Proxy | ○ | | ● | | | | | |
| BL-INT-02 WebCredentialStore | ○ | | ● | | | | | |
| BL-INT-03 Preflight Merge | ○ | | ● | | | | | |

---

## 2. Nghiệp vụ theo từng Actor

### Alex — Senior Full-Stack Developer

**Tổng số nghiệp vụ:** 23 (Primary: 16, Participant: 7)

**P0 — Critical:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-WT-01 | Tạo Worktree |
| BL-WT-02 | Fan-out Worktrees |
| BL-WT-03 | Xóa Worktree |
| BL-AG-01 | Khởi động Agent |
| BL-AG-02 | Dừng Agent |
| BL-AG-05 | Monitor Trạng thái Agent |
| BL-TM-01 | Tạo PTY Session |
| BL-TM-02 | Split Terminal |

**P1 — Should Have:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-WT-04 | So sánh Worktrees |
| BL-WT-05 | Merge Worktree |
| BL-AG-03 | Resume Session |
| BL-AG-04 | Switch Account |
| BL-TM-03 | Scrollback Persistence |
| BL-TM-04 | Shell Integration |
| BL-CR-01 | Xem Diff |
| BL-CR-02 | Annotate Diff |
| BL-CR-03 | Gửi Feedback Agent |
| BL-CR-04 | Generate Commit |
| BL-CR-05 | Tạo Pull Request |
| BL-PI-01 | Import Issues |
| BL-PI-02 | Worktree từ Task |
| BL-DB-01 | Capture UI Element |
| BL-DB-02 | Inject Context Agent |

---

### Maya — Tech Lead

**Tổng số nghiệp vụ:** 18 (Primary: 14, Participant: 4)

**P0 — Critical:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-WT-01 | Tạo Worktree |
| BL-WT-03 | Xóa Worktree |
| BL-AG-01 | Khởi động Agent |
| BL-AG-02 | Dừng Agent |
| BL-AG-05 | Monitor Trạng thái |

**P1 — Should Have:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-WT-05 | Merge Worktree |
| BL-AG-03 | Resume Session |
| BL-TM-04 | Shell Integration |
| BL-CR-01 | Xem Diff |
| BL-CR-02 | Annotate Diff |
| BL-CR-03 | Gửi Feedback Agent |
| BL-CR-04 | Generate Commit |
| BL-CR-05 | Tạo Pull Request |
| BL-PI-01 | Import Issues |
| BL-PI-02 | Worktree từ Task |
| BL-PI-03 | Update Issue Status |
| BL-PI-04 | Submit PR Review |

---

### Carlos — Remote Developer

**Tổng số nghiệp vụ:** 14 (Primary: 11, Participant: 3)

**P0 — Critical:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-WT-01 | Tạo Worktree |
| BL-WT-03 | Xóa Worktree |
| BL-AG-01 | Khởi động Agent |
| BL-AG-02 | Dừng Agent |
| BL-AG-05 | Monitor Trạng thái |
| BL-TM-01 | Tạo PTY Session |
| BL-TM-02 | Split Terminal |

**P1 — Remote-specific:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-AG-03 | Resume Session |
| BL-TM-03 | Scrollback Persistence |
| BL-SSH-01 | Kết nối SSH |
| BL-SSH-02 | Deploy Relay |
| BL-SSH-03 | Auto-Reconnect |
| BL-SSH-04 | Port Forwarding |
| BL-MB-02 | Push Notification (via mobile) |

---

### Sam — Mobile-First Power User

**Tổng số nghiệp vụ:** 10 (Primary: 7, Participant: 3)

**P0 — Critical (Mobile-only):**
| Mã | Nghiệp vụ |
|----|----------|
| BL-AG-01 | Khởi động Agent (từ desktop) |
| BL-AG-05 | Monitor Trạng thái |
| BL-MB-01 | Pair Device |
| BL-MB-02 | Push Notification |

**P1 — Mobile workflow:**
| Mã | Nghiệp vụ |
|----|----------|
| BL-AG-04 | Switch Account (remote) |
| BL-MB-03 | Remote Dispatch |
| BL-MB-04 | Mobile Status View |
| BL-AT-01 | Cấu hình Automation |
| BL-AT-02 | Chạy theo Schedule |

---

### QA Engineer

**Tổng số nghiệp vụ:** 6 (Primary: 5, Participant: 1)

| Mã | Nghiệp vụ | Priority |
|----|----------|---------|
| BL-SSH-04 | Port Forwarding (test env) | P1 |
| BL-CR-01 | Xem Diff | P1 |
| BL-DB-01 | Capture UI Element | P1 |
| BL-DB-02 | Inject Context Agent | P1 |
| BL-DB-03 | Viewport Testing | P1 |

---

### DevOps Engineer

**Tổng số nghiệp vụ:** 11 (Primary: 8, Participant: 3)

| Mã | Nghiệp vụ | Priority |
|----|----------|---------|
| BL-AG-01 | Khởi động Agent (automated) | P0 |
| BL-AG-05 | Monitor Trạng thái | P0 |
| BL-SSH-01 | Kết nối SSH | P1 |
| BL-SSH-02 | Deploy Relay | P1 |
| BL-AT-01 | Cấu hình Automation | P2 |
| BL-AT-02 | Chạy theo Schedule | P2 |
| BL-AT-03 | Event Trigger | P2 |
| BL-AT-04 | Cleanup Worktrees | P2 |
| BL-CLI-01 | Tạo Worktree CLI | P1 |
| BL-CLI-02 | Quản lý Agent CLI | P1 |
| BL-CLI-03 | Headless Mode | P1 |

---

## 3. Tổng hợp: Actor × Nghiệp vụ Coverage

| Actor | Primary | Participant | Total | % của 56 nghiệp vụ |
|-------|---------|-------------|-------|-------------------|
| Alex (Senior Dev) | 16 | 10 | 26 | **46%** |
| Maya (Tech Lead) | 14 | 4 | 18 | **32%** |
| Carlos (Remote Dev) | 11 | 6 | 17 | **30%** |
| Sam (Mobile User) | 7 | 3 | 10 | **18%** |
| QA Engineer | 5 | 1 | 6 | **11%** |
| DevOps Engineer | 8 | 6 | 14 | **25%** |
| **Admin** | **7** | **5** | **12** | **21%** |
| **Agent Developer** | **5** | **1** | **6** | **11%** |

---

## 4. Nghiệp vụ chung (dùng bởi nhiều actors)

| Nghiệp vụ | Số actors | Actors |
|-----------|----------|-------|
| BL-AG-05 Monitor Status | 6 | Tất cả |
| BL-AG-01 Khởi động Agent | 5 | Alex, Maya, Carlos, Sam, DevOps |
| BL-AG-02 Dừng Agent | 4 | Alex, Maya, Carlos, Sam |
| BL-WT-01 Tạo Worktree | 3 | Alex, Maya, Carlos |
| BL-WT-03 Xóa Worktree | 3 | Alex, Maya, Carlos |
| BL-CR-01 Xem Diff | 3 | Alex, Maya, QA |

---

*Tham chiếu: [logic/README.md](./logic/README.md), [URD.md](./URD.md) — Cập nhật 2026-07-28 (56 nghiệp vụ, 8 actors)*
