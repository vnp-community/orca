# TR-000 — Test Requirements: Orca Platform Business Logic

**Tài liệu:** Test Requirements  
**Phiên bản:** 1.1  
**Ngày:** 2026-08-01  
**Tham chiếu:** PRD v3.0, SRS v2.0, URD, actor-business-logic-mapping.md, feature-business-logic-mapping.md, F36/F37/F38/F39, BL-WF-01→03, BL-TG-01→04, BL-PW-01→04

---

## 1. Mục tiêu kiểm thử

### 1.1 Mục tiêu chính

| # | Mục tiêu | Mô tả |
|---|----------|-------|
| TR-OBJ-01 | Xác thực business logic | Toàn bộ 74 nghiệp vụ theo docs/logic/ phải được kiểm thử |
| TR-OBJ-02 | Xác thực data flows | 19 luồng dữ liệu theo docs/flows/logic/ phải pass end-to-end |
| TR-OBJ-03 | Xác thực error handling | Mọi điều kiện lỗi (error conditions) trong từng BL phải được test |
| TR-OBJ-04 | Xác thực security | Các yêu cầu bảo mật (auth, isolation, encryption) phải được kiểm tra |
| TR-OBJ-05 | Xác thực actor workflows | Các workflow end-to-end cho 8 actor types phải hoạt động đúng |

### 1.2 Phạm vi kiểm thử

**IN SCOPE:**
- Authentication & User Management (BL-AUTH-01 → 05)
- Worktree Management (BL-WT-01 → 05)
- Agent Orchestration (BL-AG-01 → 05)
- Terminal Management (BL-TM-01 → 04)
- Remote Development / SSH (BL-SSH-01 → 04)
- Code Review (BL-CR-01 → 05)
- Project Integration (BL-PI-01 → 04)
- Mobile Companion (BL-MB-01 → 04)
- Automation (BL-AT-01 → 04)
- Design & Browser (BL-DB-01 → 03)
- CLI & Headless (BL-CLI-01 → 03)
- Fleet Management (BL-FLEET-01 → 04)
- Agent WebSocket Protocol (BL-AWS-01 → 03)
- Remote Integrations (BL-INT-01 → 03)
- Profile & Project Hierarchy (BL-PRF-01 → 04)
- AI Provider Management (BL-AIP-01 → 03)
- Workflow Orchestration (BL-WF-01 → 03)
- Task Graph Management (BL-TG-01 → 04)
- Project Workspace (BL-PW-01 → 04)

**OUT OF SCOPE:**
- UI rendering pixel-perfect (visual regression — separate testing)
- Third-party AI provider internal logic (Claude, Codex, Gemini)
- Network infrastructure (firewall, DNS)
- Mobile OS-level push notification delivery SLA

---

## 2. Yêu cầu kiểm thử theo domain

### 2.1 Authentication & User Management (P0)

**TR-AUTH-01**: Kiểm tra login thành công với email/password hợp lệ khi `ORCA_MULTI_USER=1`  
**TR-AUTH-02**: Kiểm tra login thất bại — cùng error message cho "email không tồn tại" và "password sai" (không tiết lộ)  
**TR-AUTH-03**: Kiểm tra account bị deactivated trả về 403  
**TR-AUTH-04**: Kiểm tra rate limiting sau 10 lần thất bại/phút  
**TR-AUTH-05**: Kiểm tra session có TTL 8h và tự expire  
**TR-AUTH-06**: Kiểm tra per-user process sandbox — User A không thể access data của User B  
**TR-AUTH-07**: Kiểm tra Admin CRUD — tạo, cập nhật, deactivate user  
**TR-AUTH-08**: Kiểm tra requireAdmin guard — user thường bị 403 khi gọi /admin/api/*  
**TR-AUTH-09**: Kiểm tra audit log ghi nhận đầy đủ: login.success, login.fail, user.create, session.kill  
**TR-AUTH-10**: Kiểm tra cookie HttpOnly + SameSite  

### 2.2 Worktree Management (P0)

**TR-WT-01**: Kiểm tra tạo worktree với branch hợp lệ — tạo đúng git worktree trên Dev Server  
**TR-WT-02**: Kiểm tra tạo worktree với disk space không đủ — trả về error  
**TR-WT-03**: Kiểm tra fan-out N worktrees song song — tất cả N worktree được tạo  
**TR-WT-04**: Kiểm tra xóa worktree an toàn khi có uncommitted changes — yêu cầu xác nhận  
**TR-WT-05**: Kiểm tra xóa worktree khi agent đang chạy — kill agent trước  
**TR-WT-06**: Kiểm tra so sánh worktrees — diff giữa các branches được hiển thị  
**TR-WT-07**: Kiểm tra merge worktree — 3 strategies: merge, squash, rebase  
**TR-WT-08**: Kiểm tra merge conflict detection  

### 2.3 Agent Orchestration (P0)

**TR-AG-01**: Kiểm tra khởi động agent — spawn thành công trên Dev Server qua JSON-RPC  
**TR-AG-02**: Kiểm tra profile injection — env vars từ resolved profile được inject vào agent  
**TR-AG-03**: Kiểm tra AI provider resolution — đúng credentials được dùng  
**TR-AG-04**: Kiểm tra dừng agent gracefully (Ctrl+C) trong 10s  
**TR-AG-05**: Kiểm tra force kill agent sau timeout  
**TR-AG-06**: Kiểm tra resume session với đúng sessionId  
**TR-AG-07**: Kiểm tra switch account khi rate limited  
**TR-AG-08**: Kiểm tra OSC 133 parse — status changes được detect đúng  
**TR-AG-09**: Kiểm tra mobile notification khi agent complete  

### 2.4 Terminal Management (P0)

**TR-TM-01**: Kiểm tra tạo PTY session trên Dev Server  
**TR-TM-02**: Kiểm tra split terminal — horizontal và vertical  
**TR-TM-03**: Kiểm tra scrollback persistence sau restart  
**TR-TM-04**: Kiểm tra OSC 133 shell integration — command tracking  
**TR-TM-05**: Kiểm tra terminal typing latency < 16ms  

### 2.5 Remote Development / SSH (P1)

**TR-SSH-01**: Kiểm tra kết nối SSH với key authentication  
**TR-SSH-02**: Kiểm tra kết nối SSH với password  
**TR-SSH-03**: Kiểm tra deploy relay binary tự động lên remote  
**TR-SSH-04**: Kiểm tra auto-reconnect với exponential backoff  
**TR-SSH-05**: Kiểm tra port forwarding — local port ánh xạ tới remote service  
**TR-SSH-06**: Kiểm tra SSH config file parsing (Host patterns, includes)  

### 2.6 Code Review (P1)

**TR-CR-01**: Kiểm tra xem diff — unified diff view với syntax highlight  
**TR-CR-02**: Kiểm tra annotate diff — thêm comment trên một dòng cụ thể  
**TR-CR-03**: Kiểm tra gửi annotation feedback về agent — agent nhận và xử lý  
**TR-CR-04**: Kiểm tra AI generate commit message từ staged diff  
**TR-CR-05**: Kiểm tra tạo Pull Request với AI description  

### 2.7 Project Integration (P1)

**TR-PI-01**: Kiểm tra import GitHub issues — pagination, labels, assignees  
**TR-PI-02**: Kiểm tra import Linear tasks  
**TR-PI-03**: Kiểm tra tạo worktree từ GitHub issue — branch tên đúng convention  
**TR-PI-04**: Kiểm tra update issue status sau merge  
**TR-PI-05**: Kiểm tra submit PR review — approve, request changes, comment  

### 2.8 Mobile Companion (P0)

**TR-MB-01**: Kiểm tra pair device qua QR code — hoàn thành trong < 30s  
**TR-MB-02**: Kiểm tra E2E encryption của channel (TweetNaCl)  
**TR-MB-03**: Kiểm tra push notification delivery khi agent complete (< 5s)  
**TR-MB-04**: Kiểm tra remote dispatch — send instruction từ mobile  
**TR-MB-05**: Kiểm tra xem agent status từ mobile  
**TR-MB-06**: Kiểm tra unpair device  

### 2.9 Automation (P2)

**TR-AT-01**: Kiểm tra tạo automation với cron schedule  
**TR-AT-02**: Kiểm tra chạy automation theo schedule — đúng giờ  
**TR-AT-03**: Kiểm tra event trigger — git.push event kích hoạt automation  
**TR-AT-04**: Kiểm tra cleanup worktrees theo retention policy  

### 2.10 Design & Browser (P1)

**TR-DB-01**: Kiểm tra capture UI element — HTML + CSS + screenshot  
**TR-DB-02**: Kiểm tra inject captured context vào agent prompt  
**TR-DB-03**: Kiểm tra viewport preset switching  

### 2.11 CLI & Headless (P1)

**TR-CLI-01**: Kiểm tra `orca worktree create` — tạo worktree qua CLI  
**TR-CLI-02**: Kiểm tra `orca agent start/stop/status`  
**TR-CLI-03**: Kiểm tra `orca serve` — headless mode  
**TR-CLI-04**: Kiểm tra CLI exit codes — 0 thành công, non-zero thất bại  

### 2.12 Fleet Management (P1)

**TR-FLEET-01**: Kiểm tra fleet inventory YAML parsing  
**TR-FLEET-02**: Kiểm tra bulk provisioning — deploy relay lên nhiều servers  
**TR-FLEET-03**: Kiểm tra health monitoring — status healthy/degraded/unhealthy/unreachable  
**TR-FLEET-04**: Kiểm tra Prometheus metrics endpoint  
**TR-FLEET-05**: Kiểm tra webhook alert khi server status thay đổi  
**TR-FLEET-06**: Kiểm tra onboarding wizard — platform-aware (Linux/Mac)  

### 2.13 Agent WebSocket Protocol (P1)

**TR-AWS-01**: Kiểm tra relay-websocket mode — Orca kết nối tới agent WS server  
**TR-AWS-02**: Kiểm tra direct-websocket mode — agent kết nối vào Orca  
**TR-AWS-03**: Kiểm tra binary wire protocol header (13-byte)  
**TR-AWS-04**: Kiểm tra agentToken handshake và xác thực  
**TR-AWS-05**: Kiểm tra token management — generate, revoke, rotate  

### 2.14 Remote Integrations (P1)

**TR-INT-01**: Kiểm tra CLI auth proxy — GitHub/GitLab auth qua SSH relay  
**TR-INT-02**: Kiểm tra WebCredentialStore — AES-256-GCM encrypt/decrypt  
**TR-INT-03**: Kiểm tra per-user credential isolation  
**TR-INT-04**: Kiểm tra preflight merge — relay CLI checks + local API checks  

### 2.15 Profile & Project Hierarchy (P0)

**TR-PRF-01**: Kiểm tra tạo Company/Dept/User profile  
**TR-PRF-02**: Kiểm tra 3-layer inheritance — User > Dept > Company  
**TR-PRF-03**: Kiểm tra security fields bị lock ở Company level  
**TR-PRF-04**: Kiểm tra pathAdditions concatenation  
**TR-PRF-05**: Kiểm tra cache TTL 60s — auto-invalidate khi parent thay đổi  
**TR-PRF-06**: Kiểm tra project-dev server binding  
**TR-PRF-07**: Kiểm tra profile-aware agent execution — env inject đúng  

### 2.16 AI Provider Management (P0)

**TR-AIP-01**: Kiểm tra đăng ký AI provider account (Anthropic, OpenAI, v.v.)  
**TR-AIP-02**: Kiểm tra credentials AES-256-GCM trên Dev Server  
**TR-AIP-03**: Kiểm tra provider resolution priority: user > project > server-default  
**TR-AIP-04**: Kiểm tra health check cron mỗi 15 phút  
**TR-AIP-05**: Kiểm tra quota alert 80%  
**TR-AIP-06**: Kiểm tra key rotation với 30s grace period  

### 2.17 Workflow Orchestration (P1)

**TR-WF-01**: Kiểm tra tạo workflow template với YAML — step IDs unique, dependsOn valid, no cycles  
**TR-WF-02**: Kiểm tra template inheritance — overrides, inject_steps (after/before), remove_steps  
**TR-WF-03**: Kiểm tra DAG execution — topological sort + wave-based parallel  
**TR-WF-04**: Kiểm tra multi-server dispatch — steps trên đúng server (project:/server:/fleet:tag:)  
**TR-WF-05**: Kiểm tra workflow resumability sau restart — completed steps không re-run  
**TR-WF-06**: Kiểm tra workflow sharing — visibility: private/team/company/public  
**TR-WF-07**: Kiểm tra variable interpolation — `{{inputs.*}}`, `{{outputs.<stepId>.*}}`, `{{project.*}}`, `{{now()}}`  
**TR-WF-08**: Kiểm tra parallel step type — allSettled + allowPartialFailure option  
**TR-WF-09**: Kiểm tra condition step — branch logic dựa trên step output  
**TR-WF-10**: Kiểm tra fleet:tag: server resolution — load balance qua healthy servers với tag  
**TR-WF-11**: Kiểm tra real-time stream — step.output, step.completed, execution.completed events qua WebSocket/SSE  
**TR-WF-12**: Kiểm tra step fail → workflow dừng + persist state (on_fail: stop default)  

### 2.18 Task Graph Management (P0)

**TR-TG-01**: Kiểm tra tạo task với types: epic/story/task/subtask/bug/spike  
**TR-TG-02**: Kiểm tra parent-child relationship — parentId link + progress aggregation recursive  
**TR-TG-03**: Kiểm tra dependency edges (depends_on / blocks / relates_to)  
**TR-TG-04**: Kiểm tra cycle detection trước khi add dependency (BFS từ toId)  
**TR-TG-05**: Kiểm tra AI decompose — subtask suggestions với dependency graph  
**TR-TG-06**: Kiểm tra 5-level grant system — view/comment/edit/execute/manage  
**TR-TG-07**: Kiểm tra apply_tree — grant propagate xuống subtask tree (recursive BFS)  
**TR-TG-08**: Kiểm tra task agent execution — preamble inject + activity feed stream  
**TR-TG-09**: Kiểm tra task status transition — backlog→todo→in_progress→review→done + blocked  
**TR-TG-10**: Kiểm tra auto-unblock — khi dependency task done, blocked task → todo  
**TR-TG-11**: Kiểm tra blocking deps check trước khi run agent — trả lỗi BLOCKED_BY_DEPS  
**TR-TG-12**: Kiểm tra time-limited grant — grant với expiresAt, tự expire  
**TR-TG-13**: Kiểm tra critical path calculation từ DAG + estimates  
**TR-TG-14**: Kiểm tra AI generate agent prompt từ task context  
**TR-TG-15**: Kiểm tra delete task có children — cascade  

### 2.19 Project Workspace (P0)

**TR-PW-01**: Kiểm tra WorkspaceContext load đúng project + relay + profile  
**TR-PW-02**: Kiểm tra RelayConnectionPool — reuse connections, cleanup idle > 5min  
**TR-PW-03**: Kiểm tra offline mode — banner + cached file tree + disable writes  
**TR-PW-04**: Kiểm tra remote file explorer — lazy-load, git decorations  
**TR-PW-05**: Kiểm tra file viewer — read-only, syntax highlight, max 5MB  
**TR-PW-06**: Kiểm tra remote git UI — status, stage/unstage, commit, push, pull  
**TR-PW-07**: Kiểm tra conflict detection và AI resolution (agent reads conflict markers)  
**TR-PW-08**: Kiểm tra cross-panel event bus — agent.complete → refresh Git + Explorer + Tasks  
**TR-PW-09**: Kiểm tra project permission check — user cần 'view' access trước khi workspace.switch  
**TR-PW-10**: Kiểm tra file search (ripgrep) — fuzzy search trên dev server  
**TR-PW-11**: Kiểm tra discard changes — git restore với confirm dialog  
**TR-PW-12**: Kiểm tra stash push/pop  
**TR-PW-13**: Kiểm tra PR creation — GitHub CLI (Category A) + API token (Category B)  
**TR-PW-14**: Kiểm tra AI PR description generation từ diff + commits  
**TR-PW-15**: Kiểm tra worktree switcher per project — list + switch + create  
**TR-PW-16**: Kiểm tra git log — 50 commits với branch graph  
**TR-PW-17**: Kiểm tra workspace task integration — Task→workspace switch→worktree switch→agent run flow  

---

## 3. Tiêu chí chấp nhận (Acceptance Criteria)

### 3.1 Tiêu chí cứng (Must Pass)
- Tất cả P0 test cases phải PASS 100%
- Không có regression từ P0 features sau thay đổi
- Security tests (auth, isolation, encryption) phải PASS 100%
- Error handling tests phải PASS 100% (không expose internal errors)

### 3.2 Tiêu chí mềm (Should Pass)
- P1 test cases: ≥ 95% PASS
- P2 test cases: ≥ 80% PASS
- Performance tests: ≥ 90% pass rate

### 3.3 Tiêu chí hiệu năng
| Chỉ số | Ngưỡng chấp nhận |
|--------|-----------------|
| Terminal typing latency | < 16ms |
| Mobile pairing time | < 30s |
| Push notification delivery | < 5s |
| App cold start | < 3s |
| Profile resolve (cache miss) | < 500ms |
| Worktree create | < 30s |
| Agent spawn | < 10s |
| Workflow start (init DAG + dispatch) | < 1s |
| Step-to-step handoff latency | < 200ms |
| Template resolve (with inheritance) | < 50ms |
| Task graph load (100 tasks) | < 500ms |
| DAG cycle detection | < 10ms |
| AI decompose (avg task) | < 5s (LLM call) |
| Run Agent from task | < 3s to PTY active |
| Grant resolution (cached) | < 5ms |
| Git status refresh | < 1s |
| File search (ripgrep) | < 2s for typical repos |

---

## 4. Môi trường kiểm thử

### 4.1 Môi trường bắt buộc

| Môi trường | Cấu hình |
|------------|----------|
| Orca Web Server | Node.js 22+, ORCA_MULTI_USER=1, DB=SQLite |
| Dev Server (remote) | Linux x64, git ≥ 2.28, node-pty, relay binary, ripgrep, gh CLI |
| SSH | Password + key-based auth |
| Mobile | iOS 15+, Android 8+ với push notifications enabled |
| Database | SQLite (default), PostgreSQL (integration tests) |

### 4.2 Test Data

| Loại | Mô tả |
|------|-------|
| Users | admin@test.com, user@test.com, deactivated@test.com |
| Git Repos | test-repo với clean + dirty + conflict states |
| Worktrees | pre-created, mid-operation, stale states |
| AI Providers | Mock credentials cho unit tests, real cho integration |
| Workflow Templates | company-level template, team-level child, personal override |
| Tasks | epic → story × 2 → subtask × 3, với dependency edges |
| Projects | proj-backend (srv-1), proj-frontend (srv-2) |

---

## 5. Test Prioritization

| Tier | Priority | Nghiệp vụ | Ghi chú |
|------|----------|-----------|---------|
| Tier 1 | P0 Critical | AUTH, WT, AG, MB, PRF, AIP, TG, PW | Chặn release nếu fail |
| Tier 2 | P1 Important | TM, SSH, CR, PI, DB, CLI, FLEET, AWS, INT, WF | Cần pass cho production |
| Tier 3 | P2 Nice-to-have | AT | Có thể delay |

---

## 6. Rủi ro và giảm thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|--------|--------|------------|
| AI agent behavior không deterministic | Cao | Mock agent trong unit tests |
| SSH connectivity issues trong CI | Trung bình | Sử dụng local SSH server (sshd) |
| Database migration incompatibility | Thấp | Test với fresh + migrated DB |
| Mobile test setup phức tạp | Trung bình | Sử dụng mock WebSocket thay APNs/FCM |

---

*Test Requirements Document — Orca v5.0 — 2026-08-01*
