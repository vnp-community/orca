# Business Logic — Danh sách Nghiệp vụ Orca

Thư mục này chứa đặc tả chi tiết cho tất cả **nghiệp vụ (business logic)** của Orca, được tổ chức theo nhóm domain.  
**Cập nhật:** 2026-07-28 (thêm 26 nghiệp vụ mới cho Web Server Mode + Profile System + AI Providers + Workflows + Task Graph)

---

## Cấu trúc

```
logic/
├── worktree-management/    # Quản lý worktree git
├── agent-orchestration/    # Điều phối AI agent
├── terminal-management/    # Quản lý terminal PTY
├── remote-development/     # Phát triển từ xa qua SSH
├── code-review/            # Review và annotate code AI
├── project-integration/    # Tích hợp GitHub, Linear
├── mobile-companion/       # Ứng dụng mobile companion
├── automation/             # Tự động hóa workflow
├── design-browser/         # Design mode và embedded browser
├── cli-headless/           # CLI và headless mode
├── auth/                   # Xác thực & quản lý user (Web Server Mode)
├── fleet/                  # Fleet management & health monitoring
├── agent-ws/               # Agent WebSocket protocol
├── remote-integration/     # Remote Source Control Integrations
├── profile/                # User Profile Hierarchy & Project Binding
├── ai-providers/           # AI Provider Account Management [NEW]
├── workflow-orchestration/  # Multi-Server Workflow Orchestration [NEW]
├── task-graph/             # Task Graph Management System
└── project-workspace/      # Project Workspace, File Explorer, Remote Git [NEW]
```

---

## Danh sách Nghiệp vụ

### 1. Worktree Management — Quản lý Worktree

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-WT-01](./worktree-management/BL-WT-01-tao-worktree.md) | Tạo Worktree | P0 | Alex, Maya, Carlos |
| [BL-WT-02](./worktree-management/BL-WT-02-fan-out-worktree.md) | Fan-out Prompt tới Nhiều Worktree | P0 | Alex |
| [BL-WT-03](./worktree-management/BL-WT-03-xoa-worktree.md) | Xóa Worktree An Toàn | P0 | Alex, Maya, Carlos |
| [BL-WT-04](./worktree-management/BL-WT-04-so-sanh-worktree.md) | So sánh Kết quả Giữa Worktrees | P1 | Alex |
| [BL-WT-05](./worktree-management/BL-WT-05-merge-worktree.md) | Merge Worktree Thắng | P1 | Alex, Maya |

### 2. Agent Orchestration — Điều phối Agent

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-AG-01](./agent-orchestration/BL-AG-01-khoi-dong-agent.md) | Khởi động AI Agent | P0 | Alex, Maya, Carlos, Sam |
| [BL-AG-02](./agent-orchestration/BL-AG-02-dung-agent.md) | Dừng Agent | P0 | Alex, Maya, Carlos, Sam |
| [BL-AG-03](./agent-orchestration/BL-AG-03-resume-session.md) | Resume Agent Session | P1 | Alex, Maya, Carlos |
| [BL-AG-04](./agent-orchestration/BL-AG-04-switch-account.md) | Switch Account / Provider | P1 | Alex, Sam |
| [BL-AG-05](./agent-orchestration/BL-AG-05-monitor-status.md) | Monitor Trạng thái Agent Real-time | P0 | Tất cả |

### 3. Terminal Management — Quản lý Terminal

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-TM-01](./terminal-management/BL-TM-01-tao-pty-session.md) | Tạo PTY Session | P0 | Alex, Maya, Carlos |
| [BL-TM-02](./terminal-management/BL-TM-02-split-terminal.md) | Split Terminal | P0 | Alex, Carlos |
| [BL-TM-03](./terminal-management/BL-TM-03-scrollback-persistence.md) | Lưu và Khôi phục Scrollback | P1 | Alex, Carlos |
| [BL-TM-04](./terminal-management/BL-TM-04-shell-integration.md) | Shell Integration (OSC 133) | P1 | Alex, Maya |

### 4. Remote Development — Phát triển Từ xa

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-SSH-01](./remote-development/BL-SSH-01-ket-noi-ssh.md) | Kết nối SSH Host | P1 | Carlos, DevOps |
| [BL-SSH-02](./remote-development/BL-SSH-02-deploy-relay.md) | Deploy Orca Relay Binary | P1 | Carlos, DevOps |
| [BL-SSH-03](./remote-development/BL-SSH-03-auto-reconnect.md) | SSH Auto-Reconnect | P1 | Carlos |
| [BL-SSH-04](./remote-development/BL-SSH-04-port-forwarding.md) | Auto Port Forwarding | P1 | Carlos, QA |

### 5. Code Review — Kiểm tra Code

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-CR-01](./code-review/BL-CR-01-xem-diff.md) | Xem Diff của Agent Changes | P1 | Alex, Maya |
| [BL-CR-02](./code-review/BL-CR-02-annotate-diff.md) | Annotate Dòng Code trong Diff | P1 | Maya, Alex |
| [BL-CR-03](./code-review/BL-CR-03-gui-feedback-agent.md) | Gửi Feedback về Agent | P1 | Maya, Alex |
| [BL-CR-04](./code-review/BL-CR-04-generate-commit-message.md) | Tạo Commit Message bằng AI | P1 | Alex, Maya |
| [BL-CR-05](./code-review/BL-CR-05-tao-pull-request.md) | Tạo Pull Request với AI | P1 | Alex, Maya |

### 6. Project Integration — Tích hợp Dự án

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-PI-01](./project-integration/BL-PI-01-import-issues.md) | Import GitHub/GitLab Issues | P1 | Maya, Alex |
| [BL-PI-02](./project-integration/BL-PI-02-tao-worktree-tu-task.md) | Tạo Worktree từ Issue/Task | P1 | Maya, Alex |
| [BL-PI-03](./project-integration/BL-PI-03-update-issue-status.md) | Cập nhật Trạng thái Issue | P2 | Maya |
| [BL-PI-04](./project-integration/BL-PI-04-submit-pr-review.md) | Submit PR Review lên GitHub | P1 | Maya |

### 7. Mobile Companion — Ứng dụng Mobile

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-MB-01](./mobile-companion/BL-MB-01-pair-device.md) | Pair Mobile Device | P0 | Sam |
| [BL-MB-02](./mobile-companion/BL-MB-02-push-notification.md) | Gửi Push Notification | P0 | Sam, Carlos |
| [BL-MB-03](./mobile-companion/BL-MB-03-remote-dispatch.md) | Remote Dispatch từ Mobile | P1 | Sam |
| [BL-MB-04](./mobile-companion/BL-MB-04-mobile-status.md) | Xem Agent Status từ Mobile | P1 | Sam, Carlos |

### 8. Automation — Tự động hóa

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-AT-01](./automation/BL-AT-01-cau-hinh-automation.md) | Cấu hình Automation | P2 | Sam, DevOps |
| [BL-AT-02](./automation/BL-AT-02-chay-automation.md) | Chạy Automation theo Schedule | P2 | Sam, DevOps |
| [BL-AT-03](./automation/BL-AT-03-event-trigger.md) | Event-based Automation Trigger | P2 | DevOps, Sam |
| [BL-AT-04](./automation/BL-AT-04-cleanup-worktrees.md) | Cleanup Worktrees Theo Policy | P2 | DevOps, Alex |

### 9. Design & Browser — Thiết kế & Trình duyệt

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-DB-01](./design-browser/BL-DB-01-capture-ui-element.md) | Capture UI Element | P1 | Alex, QA |
| [BL-DB-02](./design-browser/BL-DB-02-inject-context-agent.md) | Inject UI Context vào Agent | P1 | Alex, QA |
| [BL-DB-03](./design-browser/BL-DB-03-viewport-testing.md) | Viewport Testing | P1 | QA, Alex |

### 10. CLI & Headless — Giao diện Lệnh

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-CLI-01](./cli-headless/BL-CLI-01-tao-worktree-cli.md) | Tạo Worktree qua CLI | P1 | DevOps, Alex |
| [BL-CLI-02](./cli-headless/BL-CLI-02-quan-ly-agent-cli.md) | Quản lý Agent qua CLI | P1 | DevOps |
| [BL-CLI-03](./cli-headless/BL-CLI-03-headless-mode.md) | Chạy Orca Headless Mode | P1 | DevOps |

### 11. Authentication & User Management — Xác thực và Quản lý User

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-AUTH-01](./auth/BL-AUTH-01-local-login.md) | Local Login (email + password) | P0 | Mọi user (Web Server) |
| [BL-AUTH-02](./auth/BL-AUTH-02-session-management.md) | Session Management & Isolation | P0 | Mọi user |
| [BL-AUTH-03](./auth/BL-AUTH-03-per-user-sandbox.md) | Per-User Process Sandbox | P0 | Mọi user |
| [BL-AUTH-04](./auth/BL-AUTH-04-admin-user-crud.md) | Admin User CRUD & Session Kill | P0 | Admin |
| [BL-AUTH-05](./auth/BL-AUTH-05-audit-log.md) | Audit Log Ghi nhận Action | P1 | Admin |

### 12. Fleet Management — Quản lý Fleet Dev Servers

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-FLEET-01](./fleet/BL-FLEET-01-fleet-inventory.md) | Fleet Inventory Config (YAML) | P1 | Admin, DevOps |
| [BL-FLEET-02](./fleet/BL-FLEET-02-bulk-provisioning.md) | Bulk Server Provisioning | P1 | Admin |
| [BL-FLEET-03](./fleet/BL-FLEET-03-health-monitoring.md) | Fleet Health Monitoring | P1 | Admin, DevOps |
| [BL-FLEET-04](./fleet/BL-FLEET-04-dev-server-onboarding.md) | Dev Server Onboarding Wizard | P1 | Carlos, Admin |

### 13. Agent WebSocket — Kết nối Agent qua WebSocket

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-AWS-01](./agent-ws/BL-AWS-01-relay-websocket.md) | relay-websocket Mode (Orca → Agent) | P1 | Agent Developer |
| [BL-AWS-02](./agent-ws/BL-AWS-02-direct-websocket.md) | direct-websocket Mode (Agent → Orca) | P1 | Agent Developer |
| [BL-AWS-03](./agent-ws/BL-AWS-03-token-management.md) | Agent Token Management | P1 | Agent Developer, Admin |

### 14. Remote Source Control Integrations — Tích hợp Remote

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-INT-01](./remote-integration/BL-INT-01-cli-auth-proxy.md) | CLI Auth Proxy (GitHub/GitLab qua SSH Relay) | P1 | Carlos, Alex |
| [BL-INT-02](./remote-integration/BL-INT-02-credential-store.md) | WebCredentialStore (API Token Management) | P1 | Carlos, Alex |
| [BL-INT-03](./remote-integration/BL-INT-03-preflight-merge.md) | Preflight Status Merge (Local + Remote) | P1 | Carlos, Alex |

### 15. Profile & Project Management — Profile Hệ thống và Project Binding [NEW]

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-PRF-01](./profile/BL-PRF-01-profile-crud.md) | Tạo và Cập nhật Profile (Company/Dept/User) | P0 | Admin, Lead, User |
| [BL-PRF-02](./profile/BL-PRF-02-profile-inheritance.md) | Profile Inheritance Resolution (3-layer merge) | P0 | System |
| [BL-PRF-03](./profile/BL-PRF-03-project-server-assignment.md) | Project-Dev Server Assignment | P0 | Admin, Lead |
| [BL-PRF-04](./profile/BL-PRF-04-profile-aware-agent-execution.md) | Profile-Aware Agent Execution Routing | P0 | Developer, Lead |


### 16. AI Provider Management — Quản lý AI Provider Accounts [NEW]

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-AIP-01](./ai-providers/BL-AIP-01-register-provider-account.md) | Đăng ký AI Provider Account trên Dev Server | P0 | Admin, Lead |
| [BL-AIP-02](./ai-providers/BL-AIP-02-provider-resolution.md) | Provider Account Resolution cho Agent/Workflow | P0 | System |
| [BL-AIP-03](./ai-providers/BL-AIP-03-provider-health-quota.md) | Provider Health Check & Quota Management | P1 | System, Admin |

### 17. Workflow Orchestration — Điều phối Workflow Đa Máy chủ [NEW]

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-WF-01](./workflow-orchestration/BL-WF-01-workflow-template.md) | Workflow Template Management (Create/Inherit/Share) | P1 | Admin, Lead, User |
| [BL-WF-02](./workflow-orchestration/BL-WF-02-workflow-execution.md) | Multi-Server Workflow Execution | P1 | Developer, Lead, System |
| [BL-WF-03](./workflow-orchestration/BL-WF-03-workflow-sharing.md) | Workflow Sharing & Library Discovery | P1 | Any User |

### 18. Task Graph Management — Quản lý Tác vụ theo Đồ thị [NEW]

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-TG-01](./task-graph/BL-TG-01-task-graph-crud.md) | Task Graph CRUD & Structural Management | P0 | Developer, Lead, Admin |
| [BL-TG-02](./task-graph/BL-TG-02-ai-task-planning.md) | AI-Assisted Task Planning & Decomposition | P0 | Developer, Lead |
| [BL-TG-03](./task-graph/BL-TG-03-task-access-control.md) | Task Access Control & Sharing | P0 | Owner, Lead, Admin |
| [BL-TG-04](./task-graph/BL-TG-04-task-agent-execution.md) | Task Prompt → Agent Execution | P0 | Developer, Lead |


### 19. Project Workspace — Workspace Tích hợp & Remote Git [NEW]

| Mã | Tên | Priority | Actor chính |
|----|-----|----------|------------|
| [BL-PW-01](./project-workspace/BL-PW-01-workspace-context.md) | Project Workspace Context | P0 | Developer, Lead |
| [BL-PW-02](./project-workspace/BL-PW-02-remote-file-explorer.md) | Remote File Explorer | P0 | Developer, Lead |
| [BL-PW-03](./project-workspace/BL-PW-03-remote-git-operations.md) | Remote Git UI Operations | P0 | Developer, Lead |
| [BL-PW-04](./project-workspace/BL-PW-04-workspace-integration.md) | Workspace Integration (Agent+Git+Tasks+Workflows) | P0 | Developer, Lead |

---

## Thống kê

| Nhóm | Số nghiệp vụ | P0 | P1 | P2 |
|------|------------|----|----|-----|
| Worktree Management | 5 | 3 | 2 | 0 |
| Agent Orchestration | 5 | 3 | 2 | 0 |
| Terminal Management | 4 | 2 | 2 | 0 |
| Remote Development | 4 | 0 | 4 | 0 |
| Code Review | 5 | 0 | 5 | 0 |
| Project Integration | 4 | 0 | 3 | 1 |
| Mobile Companion | 4 | 2 | 2 | 0 |
| Automation | 4 | 0 | 0 | 4 |
| Design & Browser | 3 | 0 | 3 | 0 |
| CLI & Headless | 3 | 0 | 3 | 0 |
| Auth & User Mgmt | 5 | 3 | 2 | 0 |
| Fleet Management | 4 | 0 | 4 | 0 |
| Agent WebSocket | 3 | 0 | 3 | 0 |
| Remote Integrations | 3 | 0 | 3 | 0 |
| **Profile & Project** | **4** | **4** | **0** | **0** |
| **AI Providers** | **3** | **2** | **1** | **0** |
| **Workflow Orchestration** | **3** | **0** | **3** | **0** |
| **Task Graph** | **4** | **4** | **0** | **0** |
| **Project Workspace** | **4** | **4** | **0** | **0** |
| **TỔNG** | **74** | **27** | **42** | **5** |
