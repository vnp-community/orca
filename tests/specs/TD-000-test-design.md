# TD-000 — Test Design: Orca Platform

**Tài liệu:** Test Design  
**Phiên bản:** 1.1  
**Ngày:** 2026-08-01  
**Tham chiếu:** TR-000-test-requirements.md, docs/flows/logic/, docs/logic/workflow-orchestration/, docs/logic/task-graph/, docs/logic/project-workspace/, F36/F37/F38/F39

---

## 1. Chiến lược kiểm thử

### 1.1 Pyramid kiểm thử

```
          ┌─────────────────────────┐
          │   E2E Tests (10%)       │  Actor workflow end-to-end
          │   (Actor Journeys)      │
          ├─────────────────────────┤
          │  Integration Tests (30%)│  Data flows, DB, WebSocket
          │  (Flow Verification)    │
          ├─────────────────────────┤
          │   Unit Tests (60%)      │  Business logic functions
          │   (BL Validation)       │
          └─────────────────────────┘
```

### 1.2 Kỹ thuật kiểm thử (Test Techniques)

| Kỹ thuật | Áp dụng cho | Mục đích |
|---------|-------------|----------|
| **Equivalence Partitioning** | Input validation | Giảm số test cases |
| **Boundary Value Analysis** | Số lượng (worktrees, fan-out), thời gian TTL | Test giá trị biên |
| **State Transition Testing** | Agent status, Session lifecycle, Worktree status | Kiểm tra chuyển trạng thái |
| **Decision Table Testing** | Auth conditions, Permission matrix, Provider priority | Logic phức tạp |
| **Error Guessing** | Security, Edge cases, Race conditions | Bổ sung từ kinh nghiệm |
| **Data Flow Testing** | End-to-end flows trong docs/flows/logic/ | Trace data path |

### 1.3 Loại kiểm thử

| Loại | Phạm vi | Công cụ gợi ý |
|------|---------|--------------|
| **Unit Tests** | Hàm BL đơn lẻ, validators, parsers | Jest / Vitest |
| **Integration Tests** | API endpoints, DB operations, WebSocket | Supertest + Jest |
| **Flow Tests** | End-to-end data flow theo docs/flows/ | Custom test harness |
| **Security Tests** | Auth, isolation, encryption | Jest + custom assertions |
| **Performance Tests** | Latency, throughput | k6 / custom timers |
| **E2E Tests** | Actor journeys | Playwright (desktop/web) |

---

## 2. Convention đặt tên Test Case

### 2.1 ID Format

```
TC-{DOMAIN}-{NUMBER}-{SCENARIO}
```

Ví dụ:
- `TC-AUTH-001` — Authentication, test case #001
- `TC-WT-002-FANOUT` — Worktree, fan-out scenario
- `TC-AG-001-HAPPY-PATH` — Agent start, happy path

### 2.2 Status values
- `PASS` — Test passed
- `FAIL` — Test failed
- `SKIP` — Test skipped (environment not available)
- `PENDING` — Not yet implemented
- `BLOCKED` — Blocked by dependency

### 2.3 Priority
- `P0` — Critical, must pass
- `P1` — Important, should pass
- `P2` — Nice to have

---

## 3. Template Test Case

```markdown
## TC-{ID}: {Tên ngắn gọn}

**BL Reference:** BL-{DOMAIN}-{NUMBER}  
**Flow Reference:** docs/flows/logic/{domain}.md  
**Priority:** P{0|1|2}  
**Type:** Unit | Integration | E2E | Security | Performance  
**Actor:** {Actor name}  

### Preconditions
- Điều kiện tiên quyết 1
- Điều kiện tiên quyết 2

### Test Data
| Field | Value |
|-------|-------|
| ... | ... |

### Steps
1. {Bước 1}
2. {Bước 2}

### Expected Results
- {Kết quả mong đợi 1}
- {Kết quả mong đợi 2}

### Assertions
- `assert X === Y`
- DB check: `SELECT ... WHERE ...`

### Error Scenarios
| Scenario | Input | Expected |
|----------|-------|---------|
| ... | ... | ... |
```

---

## 4. Thiết kế theo Domain

### 4.1 Authentication (BL-AUTH-*)

**Kỹ thuật:** Decision Table + State Transition  
**States:** Logged out → Logged in → Session expired → Deactivated

**Decision Table — Login:**
| Email exists | Password correct | Account active | Result |
|:---:|:---:|:---:|:---:|
| ✓ | ✓ | ✓ | 200 + Cookie |
| ✓ | ✗ | ✓ | 401 invalid_credentials |
| ✗ | — | — | 401 invalid_credentials |
| ✓ | ✓ | ✗ | 403 account_inactive |

**State Machine — Session:**
```
[No Session] --login.success--> [Active Session]
[Active Session] --8h elapsed--> [Expired]
[Active Session] --admin.kill--> [Terminated]
[Active Session] --logout--> [No Session]
```

### 4.2 Worktree Management (BL-WT-*)

**Kỹ thuật:** State Transition + Boundary Value  
**States:** none → creating → ready → comparing → merging → deleted  
**Boundaries:** N=1, N=5 (recommended), N=10 (max), N=11 (exceed)

**Fan-out boundary:**
| N | Expected | Notes |
|---|----------|-------|
| 1 | OK | Minimum |
| 5 | OK | Recommended max |
| 10 | OK | Hard limit |
| 0 | Error | Below minimum |
| 11 | Error | Exceeds maximum |

### 4.3 Agent Orchestration (BL-AG-*)

**Kỹ thuật:** State Transition + Error Guessing  
**States:** spawning → idle → running → waiting → completed → stopped → failed

**OSC 133 parsing table:**
| Sequence | Current State | New State |
|----------|--------------|-----------|
| `ESC]133;A ST` | any | running |
| `ESC]133;D;0 ST` | running | idle |
| `ESC]133;D;non0 ST` | running | failed |
| rate-limit pattern | any | rate-limited |

### 4.4 Mobile Companion (BL-MB-*)

**Kỹ thuật:** Sequence Testing  
**Sequence:** QR display → QR scan → WebSocket connect → ECDH key exchange → encryption established

**E2E Encryption assertions:**
- Message payload MUST NOT contain plaintext agent output
- TweetNaCl box sealed message verifiable by recipient private key

### 4.5 Profile Hierarchy (BL-PRF-*)

**Kỹ thuật:** Decision Table (inheritance merge)

**Merge priority matrix:**
| Field | Company | Dept | User | Result |
|-------|---------|------|------|--------|
| agentModel | "claude-4" | — | "gpt-4" | "gpt-4" (User wins) |
| agentModel | "claude-4" | "gemini" | — | "gemini" (Dept wins) |
| agentModel | "claude-4" | — | — | "claude-4" (Company only) |
| security.lockedModels | ["gpt-3.5"] | ignored | ignored | ["gpt-3.5"] (Company locked) |
| pathAdditions | ["/co/bin"] | ["/dept/bin"] | ["/user/bin"] | ["/co/bin","/dept/bin","/user/bin"] |

### 4.6 Task Graph (BL-TG-*)

**Kỹ thuật:** Graph Testing (cycle detection)  
**Cycle detection cases:**
- A → B → C: valid
- A → B → A: cycle, must reject
- A → B → C → A: cycle, must reject (transitive)

**Grant cascade (apply_tree):**
```
Root Task [grant: user@A = edit]
  ├── Child-1 [must inherit: user@A = edit]
  └── Child-2
        └── Grandchild [must inherit: user@A = edit]
```

### 4.7 AI Provider Resolution (BL-AIP-*)

**Kỹ thuật:** Decision Table (priority resolution)

| User scope | Project scope | Server scope | Result |
|:---:|:---:|:---:|:---:|
| ✓ | ✓ | ✓ | User wins |
| ✗ | ✓ | ✓ | Project wins |
| ✗ | ✗ | ✓ | Server default |
| ✗ | ✗ | ✗ | Error: no provider |

### 4.8 Workflow Orchestration (BL-WF-*)

**Kỹ thuật:** DAG Testing + State Transition + Equivalence Partitioning  

**DAG topological sort:**
```
Steps: A, B, C, D
Dependencies: D→C, C→B, B→A
Waves: [A], [B], [C], [D]
```

**Mixed dependency waves:**
```
A (no deps),  B (no deps),  C (deps: A),  D (deps: A, B)
Wave 1: [A, B]  — parallel
Wave 2: [C]     — after A done
Wave 3: [D]     — after A and B done
```

**Parallel execution:**
```
Steps: A, B, C (no deps)
Waves: [A, B, C] — all in one wave
```

**Variable interpolation — Decision Table:**
| Expression | Source | Example |
|------------|--------|---------|
| `{{feature_description}}` | inputs | "Add OAuth login" |
| `{{outputs.backend.api_endpoint}}` | step output | "/api/v2/features" |
| `{{project.name}}` | project context | "vnp-blc-backend" |
| `{{user.email}}` | user context | "dev@company.com" |
| `{{now()}}` | system | "2026-08-01T10:00:00Z" |
| `{{unknown}}` | not found | passthrough as-is |

**Parallel step — allowPartialFailure matrix:**
| Sub-step A | Sub-step B | allowPartialFailure | Result |
|:---:|:---:|:---:|:---:|
| ✓ | ✓ | any | success |
| ✓ | ✗ | false | workflow stopped |
| ✓ | ✗ | true | warning, continue |
| ✗ | ✗ | false | workflow stopped |

**Server resolution — Decision Table:**
| spec | Resolution | Example |
|------|------------|--------|
| `project:<id>` | project.devServerId | dev-alpha |
| `server:<id>` | direct server ID | srv-prod-1 |
| `fleet:tag:<tag>` | random healthy server with tag | any backend server |
| (none) | workflow context default | ctx.defaultDevServerId |

**State machine — WorkflowExecution:**
```
[queued] --start--> [running] --all-done--> [completed]
                    [running] --step-fail --> [failed]
                    [running] --user-pause--> [paused]
                    [paused]  --resume-----> [running]
```

---

### 4.9 Task Graph (BL-TG-*) — Extended

**Kỹ thuật:** Graph Testing + State Transition + Decision Table  

**Task State Machine (extended):**
```
[backlog] --assign--> [todo]
[todo]    --start---> [in_progress]
[todo]    --dep-added(unresolved)--> [blocked]
[blocked] --all-deps-done--> [todo]    (auto-unblock)
[in_progress] --agent-complete--> [review]
[review]  --approve--> [done]
[done]    --reopen--> [in_progress]
any       --cancel--> [cancelled]
```

**Auto-unblock trigger:**
```
Task A status→'done'
  → SELECT from_task_id FROM orca_task_edges WHERE to_task_id=A.id AND edge_type='depends_on'
  → For each blocked task: check ALL its deps are done
  → If yes: UPDATE status → 'todo'
```

**Blocking deps check before run agent:**
```
task.runAgent called
  → check all depends_on tasks → status !== 'done'?
  → YES: return BLOCKED_BY_DEPS (403)
  → NO: proceed to spawn
```

**Time-limited grant expiry:**
| expiresAt | Current time | Grant valid? |
|:---------:|:------------:|:------------:|
| T+1h | T+0h | ✓ |
| T+1h | T+2h | ✗ (expired) |
| null | any | ✓ (no expiry) |

**Critical path — Decision Table:**
```
DAG: A(2h) → B(3h) → D(1h)
      A(2h) → C(5h) → D(1h)

Earliest End:
  A = 2,  B = 2+3 = 5,  C = 2+5 = 7
  D = max(5,7)+1 = 8

Critical path: A → C → D (total: 8h)
Non-critical:  A → B → D (total: 6h)
```

---

### 4.10 Project Workspace (BL-PW-*) — Extended

**Kỹ thuật:** State Transition + Decision Table  

**Workspace permission check:**
| User role | ProjectGrantService result | workspace.switch |
|:---------:|:-------------------------:|:----------------:|
| Project member | hasAccess=true | OK, context loaded |
| Non-member | hasAccess=false | 403 FORBIDDEN |
| Admin | bypass check | OK |

**Offline mode — Decision Table:**
| Server health | offlineMode | File reads | File writes |
|:---:|:---:|:---:|:---:|
| online | false | relay (live) | allowed |
| unreachable | true | cached data | disabled |
| degraded | false | relay (slow) | allowed |

**PR creation — Decision Table (F39 BL-PW-03):**
| GitHub CLI present | Token in CredStore | Method |
|:---:|:---:|:------:|
| ✓ | any | GitHub CLI (gh pr create) |
| ✗ | ✓ | GitHub API via token |
| ✗ | ✗ | Error: no auth method |

**Conflict detection — State after pull:**
```
git.pull result:
  FAST-FORWARD  → no conflict → refresh git status
  ALREADY UP TO DATE → no conflict
  CONFLICT      → offlineConflict mode:
    → relay.call('git.status') → unmerged files (U)
    → show conflict panel
    → Options: AI Resolve | Abort merge
```

**AI conflict resolution flow:**
```
User → [AI: Resolve conflict]
  → relay.call('git.diff', { conflictFile })
  → agent receives: both sides of conflict markers
  → agent rewrites file with resolved content
  → relay.call('git.add', { files: [conflictFile] })
  → User reviews → commit
```

---

## 5. Data Flow Test Design

Mỗi test trong `/flows/logic/` được test theo pattern:

```
1. Setup: Khởi tạo state ban đầu
2. Trigger: Gọi action bắt đầu flow
3. Trace: Xác minh từng step trong flow
4. Assert: Kiểm tra state cuối và side effects
5. Cleanup: Dọn dẹp test data
```

**Ví dụ — BL-AUTH-01 flow test:**
```
Setup:   INSERT user { email: 'test@test.com', passwordHash: bcrypt('pass'), active: true }
Trigger: POST /auth/local { email: 'test@test.com', password: 'pass' }
Trace:   → SQL: SELECT user WHERE email
         → bcrypt.compare(password, hash)
         → SQL: INSERT orca_sessions
         → SQL: INSERT orca_audit_log
Assert:  Response 200 + Set-Cookie header
         DB: session record exists
         DB: audit_log record 'login.success'
Cleanup: DELETE user, DELETE sessions
```

---

## 6. Security Test Design

### 6.1 Auth bypass attempts
- Direct /admin/api/* without cookie → 401
- Forged session cookie → 401
- Expired session → 401
- Valid user accessing admin → 403

### 6.2 Injection prevention
- SQL injection trong email field
- XSS trong profile envVars
- Path traversal trong worktree path

### 6.3 Data isolation
- User A cannot read User B's orca.db
- User A cannot access User B's worktrees
- User A cannot use User B's AI credentials

### 6.4 Encryption validation
- Mobile channel: ciphertext must differ each message (TweetNaCl nonce)
- Credential store: `.enc` file must not contain plaintext API keys

---

## 7. Performance Test Design

| Test | Method | Threshold |
|------|--------|-----------|
| Terminal latency | measure time(keystroke → render) | < 16ms |
| Login response | POST /auth/local timing | < 500ms |
| Profile resolve | cold cache timing | < 500ms |
| Profile resolve | warm cache timing | < 10ms |
| Worktree create | E2E timing | < 30s |
| Fan-out 5 worktrees | parallel create timing | < 60s |
| Mobile pairing | QR→connected timing | < 30s |
| Workflow start | init DAG + dispatch first step | < 1s |
| Step handoff | step N complete → step N+1 start | < 200ms |
| Template inherit resolve | deepMerge + inject/remove | < 50ms |
| Task graph render (100 nodes) | load + render | < 500ms |
| Task run agent | task.runAgent → PTY active | < 3s |
| Ripgrep file search | pattern across repo | < 2s |

---

## 8. E2E Actor Journey Design

### Journey 1 — Alex: Full Developer Workflow
```
Login → Create Project → Create Worktree → Start Agent → 
Monitor via Mobile → View Diff → Annotate → Merge Worktree → 
Generate PR → Push → Logout
```

### Journey 2 — Admin: User Management
```
Login as admin → Create user → Assign profile → 
Setup AI provider → Monitor fleet → Review audit log → 
Kill stale session → Deactivate user
```

### Journey 3 — Carlos: Remote Development
```
SSH connect → Deploy relay → Create worktree (remote) → 
Start agent → Port forward → View file explorer → 
Git commit via remote UI → Push
```

### Journey 4 — Maya: Tech Lead Review
```
Login → Import GitHub issues → Create task → AI decompose → 
Assign grant → Review worktree diff → Annotate → 
Submit PR review → Update issue status
```

### Journey 5 — Alex: Full Task → Agent → Git → PR in Workspace
```
Open Project Workspace → Tasks panel → click task → 
Run Agent from task → monitor agent output → 
Git panel auto-refresh → Stage All → AI commit message → 
Commit & Push → Create PR (AI description) → 
Task status → 'review'
```

### Journey 6 — Admin: Multi-Server Workflow Execution
```
Create workflow template (YAML) with 3 steps on 2 servers → 
Set visibility: company → 
Developer: Library → find template → Execute with inputs → 
Monitor real-time: Wave 1 [lint] → Wave 2 [backend, docs parallel] → Wave 3 [pr] → 
Verify state persistence (restart Orca mid-wave 2) → Resume from correct step
```

---

*Test Design Document — Orca v5.0 — Updated 2026-08-01 (v1.1)*
