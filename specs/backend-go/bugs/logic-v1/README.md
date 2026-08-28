# Logic-v1 Bug Reports — `backend-go` vs. `docs/logic/` Business Logic Specs

This directory catalogs every gap found by auditing `backend-go` (the Go
microservices rewrite) against Orca's business-logic specification —
`docs/logic/` (19 domains, 74 numbered `BL-*` documents, each describing one
end-to-end user journey: actors, trigger, business rules `BR-*`, main flow,
alternate/error flows). Unlike `../missing-v1/` (which audits RPC
*wiring* against `specs/frontend/api/rpc-catalog.md`), this pass audits
*business logic* — whether a flow's actual rules, validations, edge cases,
and multi-step orchestration are implemented, not just whether some RPC
exists with a matching name. A channel can be "wired" per `missing-v1` and
still be a `logic-v1` gap here if it skips half the spec's business rules.

## Headline numbers

**74 business-logic flows audited. 6 fully implemented (8%). 48 partial
(65%). 20 not implemented at all (27%).** 68 bug reports were written,
one per non-fully-implemented flow.

| Status | Count | % |
|---|---:|---:|
| ✅ Fully implemented (no bug filed) | 6 | 8% |
| 🟡 Partial | 48 | 65% |
| ❌ Not implemented | 20 | 27% |

## Methodology

19 independent passes (one per `docs/logic/` domain folder, two small
domains paired up) each: (1) read every `BL-*.md` in the domain, (2)
cross-referenced its `BR-*` business rules and step-by-step flow against
the real current `backend-go` source — `.proto` contracts, `internal/usecase/`,
`internal/domain/`, `internal/adapter/grpc/server.go`,
`internal/adapter/httpgateway/*.go` (REST), `internal/adapter/wscompat/*.go`
(WebSocket channels reachable by frontend/agent), and
`internal/adapter/postgres/*.go` (persistence), (3) classified each flow as
**FULLY_IMPLEMENTED** (skip — every rule/step has a real, working
implementation), **PARTIAL** (happy path or some rules work, others
don't), or **NOT_IMPLEMENTED** (no meaningful backend-go implementation
exists), and (4) wrote one report per gap with real `file:line` citations
for both what exists and what's confirmed absent.

Each report also cross-checks `../missing-v1/*.md` and `../api-v1/*.md`
for an overlapping root cause and cites it with "See also" where relevant
— but still frames the finding around the business flow/user journey
("can a user actually do X end-to-end"), since that's what this audit is
for and RPC-wiring audits structurally can't see (e.g. a channel can be
100% wired and still skip every validation rule in the spec).

## Domain-by-domain results

### 1. Worktree Management (`worktree-management/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-WT-01 | Tạo Worktree | 🟡 Partial | [BUG-WT-01](./BUG-WT-01-tao-worktree-partial.md) |
| BL-WT-02 | Fan-out Prompt tới Nhiều Worktree | ❌ Missing | [BUG-WT-02](./BUG-WT-02-fan-out-not-implemented.md) |
| BL-WT-03 | Xóa Worktree An Toàn | 🟡 Partial | [BUG-WT-03](./BUG-WT-03-xoa-worktree-partial.md) |
| BL-WT-04 | So sánh Kết quả Giữa Worktrees | 🟡 Partial | [BUG-WT-04](./BUG-WT-04-so-sanh-worktree-partial.md) |
| BL-WT-05 | Merge Worktree Thắng | 🟡 Partial | [BUG-WT-05](./BUG-WT-05-merge-worktree-partial.md) |

### 2. Agent Orchestration (`agent-orchestration/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-AG-01 | Khởi động AI Agent | 🟡 Partial | [BUG-AG-01](./BUG-AG-01-khoi-dong-agent-partial.md) |
| BL-AG-02 | Dừng Agent | 🟡 Partial | [BUG-AG-02](./BUG-AG-02-dung-agent-partial.md) |
| BL-AG-03 | Resume Agent Session | ❌ Missing | [BUG-AG-03](./BUG-AG-03-resume-session-not-implemented.md) |
| BL-AG-04 | Switch Account / Provider | 🟡 Partial | [BUG-AG-04](./BUG-AG-04-switch-account-partial.md) |
| BL-AG-05 | Monitor Trạng thái Agent Real-time | 🟡 Partial | [BUG-AG-05](./BUG-AG-05-monitor-status-partial.md) |

### 3. Terminal Management (`terminal-management/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-TM-01 | Tạo PTY Session | ✅ Full | — |
| BL-TM-02 | Split Terminal | ✅ Full | — |
| BL-TM-03 | Lưu và Khôi phục Scrollback | ❌ Missing | [BUG-TM-03](./BUG-TM-03-scrollback-persistence-not-implemented.md) |
| BL-TM-04 | Shell Integration (OSC 133) | ❌ Missing | [BUG-TM-04](./BUG-TM-04-shell-integration-osc133-not-implemented.md) |

### 4. Remote Development / SSH (`remote-development/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-SSH-01 | Kết nối SSH Host | 🟡 Partial | [BUG-SSH-01](./BUG-SSH-01-ket-noi-ssh-partial.md) |
| BL-SSH-02 | Deploy Orca Relay Binary | 🟡 Partial | [BUG-SSH-02](./BUG-SSH-02-deploy-relay-partial.md) |
| BL-SSH-03 | SSH Auto-Reconnect | 🟡 Partial | [BUG-SSH-03](./BUG-SSH-03-auto-reconnect-partial.md) |
| BL-SSH-04 | Auto Port Forwarding | 🟡 Partial | [BUG-SSH-04](./BUG-SSH-04-port-forwarding-partial.md) |

### 5. Code Review (`code-review/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-CR-01 | Xem Diff của Agent Changes | ✅ Full | — |
| BL-CR-02 | Annotate Dòng Code trong Diff | 🟡 Partial | [BUG-CR-02](./BUG-CR-02-annotate-diff-partial.md) |
| BL-CR-03 | Gửi Feedback về Agent | 🟡 Partial | [BUG-CR-03](./BUG-CR-03-gui-feedback-agent-partial.md) |
| BL-CR-04 | Tạo Commit Message bằng AI | 🟡 Partial | [BUG-CR-04](./BUG-CR-04-generate-commit-message-partial.md) |
| BL-CR-05 | Tạo Pull Request với AI | 🟡 Partial | [BUG-CR-05](./BUG-CR-05-tao-pull-request-partial.md) |

### 6. Project Integration (`project-integration/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-PI-01 | Import GitHub/GitLab Issues | 🟡 Partial | [BUG-PI-01](./BUG-PI-01-import-issues-partial.md) |
| BL-PI-02 | Tạo Worktree từ Issue/Task | ❌ Missing | [BUG-PI-02](./BUG-PI-02-worktree-from-issue-not-implemented.md) |
| BL-PI-03 | Cập nhật Trạng thái Issue | ❌ Missing | [BUG-PI-03](./BUG-PI-03-issue-status-sync-not-implemented.md) |
| BL-PI-04 | Submit PR Review lên GitHub | ❌ Missing | [BUG-PI-04](./BUG-PI-04-submit-pr-review-not-implemented.md) |

### 7. Mobile Companion (`mobile-companion/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-MB-01 | Pair Mobile Device | ❌ Missing | [BUG-MB-01](./BUG-MB-01-pair-device-not-implemented.md) |
| BL-MB-02 | Gửi Push Notification | 🟡 Partial | [BUG-MB-02](./BUG-MB-02-push-notification-partial.md) |
| BL-MB-03 | Remote Dispatch từ Mobile | ❌ Missing | [BUG-MB-03](./BUG-MB-03-remote-dispatch-not-implemented.md) |
| BL-MB-04 | Xem Agent Status từ Mobile | ❌ Missing | [BUG-MB-04](./BUG-MB-04-mobile-status-not-implemented.md) |

### 8. Automation (`automation/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-AT-01 | Cấu hình Automation | 🟡 Partial | [BUG-AT-01](./BUG-AT-01-cau-hinh-automation-partial.md) |
| BL-AT-02 | Chạy Automation theo Schedule | 🟡 Partial | [BUG-AT-02](./BUG-AT-02-chay-automation-schedule-partial.md) |
| BL-AT-03 | Event-based Automation Trigger | ❌ Missing | [BUG-AT-03](./BUG-AT-03-event-trigger-not-implemented.md) |
| BL-AT-04 | Cleanup Worktrees Theo Policy | ❌ Missing | [BUG-AT-04](./BUG-AT-04-cleanup-worktrees-policy-not-implemented.md) |

### 9. Design & Browser (`design-browser/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-DB-01 | Capture UI Element | ✅ Full | — (desktop-local; no backend-go involvement expected) |
| BL-DB-02 | Inject UI Context vào Agent | ✅ Full | — |
| BL-DB-03 | Viewport Testing | ✅ Full | — (desktop-local; no backend-go involvement expected) |

### 10. CLI & Headless (`cli-headless/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-CLI-01 | Tạo Worktree qua CLI | 🟡 Partial | [BUG-CLI-01](./BUG-CLI-01-tao-worktree-cli-not-implemented.md) |
| BL-CLI-02 | Quản lý Agent qua CLI | 🟡 Partial | [BUG-CLI-02](./BUG-CLI-02-quan-ly-agent-cli-not-implemented.md) |
| BL-CLI-03 | Chạy Orca Headless Mode | 🟡 Partial | [BUG-CLI-03](./BUG-CLI-03-headless-mode-partial.md) |

### 11. Authentication & User Management (`auth/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-AUTH-01 | Local Login (email + password) | 🟡 Partial | [BUG-AUTH-01](./BUG-AUTH-01-local-login-partial.md) |
| BL-AUTH-02 | Session Management & Isolation | 🟡 Partial | [BUG-AUTH-02](./BUG-AUTH-02-session-lifecycle-partial.md) |
| BL-AUTH-03 | Per-User Process Sandbox | ❌ Missing | [BUG-AUTH-03](./BUG-AUTH-03-per-user-sandbox-not-implemented.md) |
| BL-AUTH-04 | Admin User CRUD & Session Kill | 🟡 Partial | [BUG-AUTH-04](./BUG-AUTH-04-admin-user-crud-partial.md) |
| BL-AUTH-05 | Audit Log Ghi nhận Action | 🟡 Partial | [BUG-AUTH-05](./BUG-AUTH-05-audit-log-partial.md) |

### 12. Fleet Management (`fleet/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-FLEET-01 | Fleet Inventory Config (YAML) | ❌ Missing | [BUG-FLEET-01](./BUG-FLEET-01-fleet-inventory-not-implemented.md) |
| BL-FLEET-02 | Bulk Server Provisioning | ❌ Missing | [BUG-FLEET-02](./BUG-FLEET-02-bulk-provisioning-not-implemented.md) |
| BL-FLEET-03 | Fleet Health Monitoring | 🟡 Partial | [BUG-FLEET-03](./BUG-FLEET-03-health-monitoring-partial.md) |
| BL-FLEET-04 | Dev Server Onboarding Wizard | 🟡 Partial | [BUG-FLEET-04](./BUG-FLEET-04-dev-server-onboarding-partial.md) |

### 13. Agent WebSocket (`agent-ws/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-AWS-01 | relay-websocket Mode | 🟡 Partial | [BUG-AWS-01](./BUG-AWS-01-relay-websocket-single-shared-token.md) |
| BL-AWS-02 | direct-websocket Mode | 🟡 Partial | [BUG-AWS-02](./BUG-AWS-02-direct-websocket-protocol-diverges-from-spec.md) |
| BL-AWS-03 | Agent Token Management | 🟡 Partial | [BUG-AWS-03](./BUG-AWS-03-token-management-not-persistent.md) |

### 14. Remote Source Control Integrations (`remote-integration/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-INT-01 | CLI Auth Proxy (GitHub/GitLab qua SSH Relay) | ❌ Missing | [BUG-INT-01](./BUG-INT-01-cli-auth-proxy-not-implemented.md) |
| BL-INT-02 | WebCredentialStore | 🟡 Partial | [BUG-INT-02](./BUG-INT-02-credential-store-unreachable-and-different-architecture.md) |
| BL-INT-03 | Preflight Status Merge | ❌ Missing | [BUG-INT-03](./BUG-INT-03-preflight-merge-not-implemented.md) |

### 15. Profile & Project Management (`profile/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-PRF-01 | Profile CRUD (Company/Dept/User) | 🟡 Partial | [BUG-PRF-01](./BUG-PRF-01-profile-crud-validation-rbac-missing.md) |
| BL-PRF-02 | Profile Inheritance Resolution (3-layer merge) | 🟡 Partial | [BUG-PRF-02](./BUG-PRF-02-profile-inheritance-approvedmodels-servertags-missing.md) |
| BL-PRF-03 | Project-Dev Server Assignment | 🟡 Partial | [BUG-PRF-03](./BUG-PRF-03-project-devserver-assignment-partial.md) |
| BL-PRF-04 | Profile-Aware Agent Execution Routing | ❌ Missing | [BUG-PRF-04](./BUG-PRF-04-profile-aware-agent-execution-not-implemented.md) |

### 16. AI Provider Management (`ai-providers/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-AIP-01 | Đăng ký AI Provider Account trên Dev Server | 🟡 Partial | [BUG-AIP-01](./BUG-AIP-01-register-provider-account-partial.md) |
| BL-AIP-02 | Provider Account Resolution cho Agent/Workflow | 🟡 Partial | [BUG-AIP-02](./BUG-AIP-02-provider-resolution-partial.md) |
| BL-AIP-03 | Provider Health Check & Quota Management | 🟡 Partial | [BUG-AIP-03](./BUG-AIP-03-provider-health-quota-partial.md) |

### 17. Workflow Orchestration (`workflow-orchestration/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-WF-01 | Workflow Template Management | 🟡 Partial | [BUG-WF-01](./BUG-WF-01-workflow-template-sharing-fields-missing.md) |
| BL-WF-02 | Multi-Server Workflow Execution | 🟡 Partial | [BUG-WF-02](./BUG-WF-02-workflow-execution-partial.md) |
| BL-WF-03 | Workflow Sharing & Library Discovery | ❌ Missing | [BUG-WF-03](./BUG-WF-03-workflow-sharing-not-implemented.md) |

### 18. Task Graph Management (`task-graph/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-TG-01 | Task Graph CRUD & Structural Management | 🟡 Partial | [BUG-TG-01](./BUG-TG-01-task-graph-structural-management-partial.md) |
| BL-TG-02 | AI-Assisted Task Planning & Decomposition | 🟡 Partial | [BUG-TG-02](./BUG-TG-02-ai-task-planning-partial.md) |
| BL-TG-03 | Task Access Control & Sharing | 🟡 Partial | [BUG-TG-03](./BUG-TG-03-task-access-control-partial.md) |
| BL-TG-04 | Task Prompt → Agent Execution | 🟡 Partial | [BUG-TG-04](./BUG-TG-04-task-agent-execution-partial.md) |

### 19. Project Workspace (`project-workspace/`)
| ID | Title | Status | Report |
|----|-------|--------|--------|
| BL-PW-01 | Project Workspace Context | 🟡 Partial | [BUG-PW-01](./BUG-PW-01-workspace-active-executions-unwired.md) |
| BL-PW-02 | Remote File Explorer | 🟡 Partial | [BUG-PW-02](./BUG-PW-02-file-explorer-dir-entry-and-auto-refresh-gaps.md) |
| BL-PW-03 | Remote Git UI Operations | 🟡 Partial | [BUG-PW-03](./BUG-PW-03-remote-git-operations-merge-stash-branch-gaps.md) |
| BL-PW-04 | Workspace Integration (Agent+Git+Tasks+Workflows) | ❌ Missing | [BUG-PW-04](./BUG-PW-04-workspace-integration-not-implemented.md) |

## Notable cross-cutting findings

- **`../missing-v1/` has gone stale on several fronts** — backend-go moved
  fast enough since that audit that a handful of its "not implemented"
  verdicts are no longer true. This pass found, and flagged inline in the
  affected reports:
  - **BUG-001** (`/admin/api/*` missing) — the full admin REST surface now
    exists (see BUG-AUTH-04).
  - **BUG-009** (`files.*` 18/18 missing) — all 18 methods are now wired
    for real (see BUG-PW-02).
  - **BUG-019/BUG-028** (`profile.*`/`team.*` mostly missing) — both are
    fully wired now, just in separate files (`channels_tenant_project.go`/
    `channels_team.go`) the original grep missed (see BUG-PRF-01).
  - **BUG-029** (`terminal.*` 10/10 missing) — fully wired now, backed by
    real `infra-fleet-service` PTY spawn/attach (see BL-TM-01/02, no bug
    filed).
  - **BUG-031/BUG-032** (`worktree.*`/`git.*` largely missing) — ~50 RPCs
    across both namespaces are now wired for real (see BUG-PW-03).

  This means `../missing-v1/`'s own headline numbers should not be taken
  as current without a refresh pass — treat it as a snapshot of an earlier
  point in backend-go's development, not the present state.

- **Two "orchestration glue" gaps show up repeatedly across unrelated
  domains** and are likely the single highest-leverage fix: (1) no
  cross-service event bus (agent-completed, PR-merged, task-closed,
  worktree-deleted events reach nothing) — surfaces as BUG-PI-03,
  BUG-AT-03, BUG-PW-04, and the "no notification" half of BUG-MB-02; (2)
  no profile-aware execution — agent/workflow spawn never consults the
  resolved profile for env, trust preset, or model routing (BUG-PRF-04,
  echoed in BUG-AIP-02's "wrong provider account" risk).

- **Two entire P0 features have zero backend-go code**: mobile pairing
  (BUG-MB-01, `BR` for QR/shared-secret crypto) and the per-user process
  sandbox (BUG-AUTH-03) — the latter is an explicit architecture change
  (stateless tenant-scoped microservices replace the old fork-per-user
  model), not an oversight; worth confirming with the team whether
  `docs/logic/auth/BL-AUTH-03` should be superseded rather than
  "implemented."

- **`SimpleExecutor`/`ComplexExecutor` being stubs (already flagged in
  missing-v1 BUG-034) blocks two independent specs**: task execution
  (BUG-TG-04) and part of multi-server workflow execution (BUG-WF-02).

- **AI-assisted "planning" flows are consistently thinner than the CRUD
  wrapper around them suggests** — BUG-TG-02 (task decomposition ignores
  dependencies/estimates/tech-stack), BUG-AIP-02 (provider resolution
  doesn't filter by requested provider/model), and BUG-CR-04 (commit
  message generation ignores repo style/branch-issue-ID) all share this
  shape: the AI call happens, but the context assembled for it is much
  poorer than the spec's business rules require.

## What this doesn't cover

- **UI/UX-only steps** — a `BL` doc's purely client-side rendering rules
  (e.g. how a diff is displayed) are out of scope; only backend-reachable
  rules were audited.
- **Byte-for-byte wire-shape correctness** for flows marked "wired" —
  matches `../missing-v1/`'s own caveat; a real RPC existing doesn't mean
  every field the spec names is present (each report calls out specific
  missing fields where found, but exhaustive schema diffing wasn't done).
- **Desktop-local features** (`design-browser/`'s embedded-browser
  capture/viewport-resize) were confirmed out of backend-go's scope by
  design, not silently skipped — see domain 9 above.
- Total counts above are a manual tally across 17 independently-researched
  passes and may be off by one or two — treat each domain's own table, and
  each report's own citations, as ground truth over this index's summary
  numbers.
