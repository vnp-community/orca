# Luồng Dữ liệu — Automation

**Domain:** Automation  
**Nghiệp vụ:** BL-AT-01 → BL-AT-04  
**Kiến trúc tham chiếu:** HLD v1 — Main Process / Daemon, F14 Automations, orca_automations table

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) | UI | Automation config form, schedule editor |
| CLI Tool | Actor alt | Trigger từ CI/CD pipeline |
| Main Process / Daemon | Business Logic | AutomationEngine, Scheduler, EventBus |
| Daemon Process | Runtime | Chạy headless, lắng nghe sự kiện |
| SQLite Database | Persistence | orca_automations, orca_automation_runs |
| Git Binary | External | git operations trong automation |
| Agent Process | External | AI agent chạy trong automation |

---

## BL-AT-01 — Cấu hình Automation

```
Người dùng (Sam/DevOps)
    │
    ▼
[Renderer] Settings → Automations → "New Automation"
    Input form:
    - name: "Nightly feature branch review"
    - trigger: schedule (cron: "0 22 * * 1-5") | event ("worktree:idle")
    - action: { type: "fan-out", n: 3, prompt: "Review and fix...", baseRef: "main" }
    - cleanup: { deleteAfter: "24h" }
    │ contextBridge.invoke('automation.create', { config })
    ▼
[Main Process — AutomationService.create()]
    ├─ Validate cron expression
    ├─ INSERT orca_automations { id, name, triggerType, cronExpr, actionConfig, enabled }  ← SQLite
    ├─ Register với Scheduler (nếu schedule-based)
    └─ emit: automation:created

CLI variant (DevOps):
    orca automation create --name "nightly" --cron "0 22 * * *" --config config.json
    → CLI → Unix Socket → Daemon → AutomationService.create()

Luồng:
User → Renderer → IPC → Main → SQLite (INSERT)
                              → Scheduler (register cron)
OR: CLI → Unix Socket → Daemon → AutomationService
```

---

## BL-AT-02 — Chạy Automation theo Schedule

```
[Scheduler] cron trigger fires (e.g., 22:00 weekdays)
    │
    ▼
[Main Process / Daemon — AutomationEngine.run()]
    ├─ SELECT automation FROM orca_automations WHERE id=?  ← SQLite
    ├─ INSERT orca_automation_runs { id, automationId, status: 'running', startedAt }
    ├─ Execute action:
    │   ├─ type: 'fan-out'  → BL-WT-02 (tạo N worktrees + start agents)
    │   ├─ type: 'single'   → BL-WT-01 + BL-AG-01 (1 worktree + 1 agent)
    │   └─ type: 'cleanup'  → BL-AT-04
    ├─ Monitor agents via OSC hooks
    ├─ Collect results khi tất cả agents done
    ├─ UPDATE orca_automation_runs { status: 'completed', output }  ← SQLite
    └─ emit: automation:completed { runId, summary }
    │
    ▼
[MobileNotificationService] push notification to Sam (nếu paired)

Luồng:
Cron tick → Daemon (AutomationEngine) → SQLite (load config + INSERT run)
          → [BL-WT-02 / BL-AT-04 execute]
          → SQLite (UPDATE run status)
          → Mobile push notification
```

---

## BL-AT-03 — Event-based Automation Trigger

```
[EventBus] sự kiện hệ thống:
    worktree:idle (agent đã xong task)
    pr:merged (GitHub webhook)
    git:push (file watch)
    │
    ▼
[Main Process — AutomationEventHandler]
    ├─ SELECT automations WHERE triggerType='event' AND triggerConfig.event=<eventType>
    │   ← SQLite
    ├─ Match điều kiện (filter, branch pattern, etc.)
    └─ FOR each matching automation:
        AutomationEngine.run(automationId, { eventContext })   ← BL-AT-02

GitHub Webhook variant:
    GitHub → POST /webhook/github → Main
    │ Verify signature (HMAC-SHA256)
    │ Parse event: push | pull_request
    → AutomationEventHandler.handle(event)

Luồng:
Internal event → EventBus → Main (query matching automations from SQLite)
                          → AutomationEngine.run() × N
OR:
GitHub webhook → POST /webhook → Main (verify + parse)
               → EventBus → AutomationEngine
```

---

## BL-AT-04 — Cleanup Worktrees Theo Policy

```
Trigger: schedule (e.g., mỗi ngày 00:00) hoặc manual
    │
    ▼
[Main Process / Daemon — WorktreeCleanupService.run()]
    ├─ SELECT worktrees WHERE createdAt < (now - policy.maxAge)  ← SQLite
    │   AND status IN ('idle', 'stopped')
    ├─ FOR each worktree:
    │   ├─ Safety check: git status (uncommitted changes?)  ← Git CLI
    │   ├─ IF safe: BL-WT-03 (delete worktree)
    │   └─ IF unsafe: log + skip + alert admin
    ├─ UPDATE orca_automation_runs { cleanedCount, skippedCount }  ← SQLite
    └─ emit: cleanup:completed { cleanedCount, skippedCount }
    │
    ▼
[Admin/DevOps] nhận report (qua log hoặc notification)

Luồng:
Schedule / Manual → Daemon → SQLite (query expired worktrees)
                           → Git CLI (safety check per worktree)
                           → [BL-WT-03 × N] (delete safe worktrees)
                           → SQLite (UPDATE run log)
```

---

## Sơ đồ tổng quan — Automation

```
┌─────────────┐          ┌──────────────────────────────────────┐
│  Renderer   │   IPC    │  Main Process / Daemon               │
│  Auto config│◄────────►│  AutomationService                   │
│  Run history│          │  AutomationEngine                    │
└─────────────┘          │  Scheduler (cron)                    │
                         │  EventBus                            │
┌─────────────┐          │  WorktreeCleanupService              │
│  CLI Tool   │─ socket ─►                                      │
└─────────────┘          └───────┬──────────────────────────────┘
                                 │
              ┌──────────────────┼────────────────────────┐
              │                  │                         │
    ┌─────────▼──┐    ┌──────────▼────┐    ┌─────────────▼──┐
    │  SQLite     │    │  Git CLI       │    │  Daemon PTY    │
    │  automations│    │  worktree ops  │    │  Agent spawn   │
    │  runs log   │    │  safety check  │    │  (BL-WT+AG)   │
    └────────────┘    └───────────────┘    └────────────────┘
              │
    ┌─────────▼──────────┐
    │  External           │
    │  APNs/FCM (notify)  │
    │  GitHub Webhooks    │
    └────────────────────┘
```
