# SOLUTION: Automation Domain — Fix tất cả Bugs

**Domain:** automation  
**TDD Reference:** TDD-08 (Agent Orchestration), TDD-17 (Workflow Orchestration), TDD-07 (Runtime Service)  
**Files cần thay đổi:** `src/main/automation/AutomationService.ts`, `src/main/worktree/WorktreeCleanupService.ts` (NEW)  
**Tổng số bugs:** 5 (AT-001~003, BE-AT-001~002)

---

## Tổng quan phụ thuộc

```
BUG-AT-001 (automation service electron-dependent) — phải fix trước tất cả
    ├── BUG-AT-002 (event-based trigger not implemented)
    └── BUG-AT-003 (remote host scheduling disabled)

BUG-BE-AT-001 (event-based automation not implemented) — giống AT-002
BUG-BE-AT-002 (worktree cleanup service not implemented) — độc lập
```

**Thứ tự fix:** `AT-001 → BE-AT-002 → AT-002/BE-AT-001 → AT-003`

---

## BUG-AT-001 — Fix AutomationService phụ thuộc vào Electron

**Mức độ:** 🔴 HIGH  
**Root cause:** `AutomationService` import `electron` modules trực tiếp → không chạy được trong web server mode.

### Fix — Dùng Platform abstraction layer

Theo TDD v5 `00-index.md §Nguyên tắc 6`:
> "Platform abstraction: `src/platform/` — không có `import 'electron'` ngoài adapter"

```typescript
// src/main/automation/AutomationService.ts

// TRƯỚC (Electron-dependent):
import { app } from 'electron'
import { BrowserWindow } from 'electron'

// SAU — Dùng platform abstraction:
import { getPlatform } from '../../platform/context'

export class AutomationService {
  constructor(
    private readonly repository: IAutomationRepository,
    private readonly workflowOrchestrator: WorkflowOrchestrator,
    private readonly log: Logger,
  ) {
    // Không còn Electron import
  }

  async triggerAutomation(params: AutomationTriggerParams): Promise<void> {
    const platform = getPlatform()
    
    // Thay vì dùng Electron API trực tiếp, dùng platform abstraction:
    // platform.app.getPath() thay vì app.getPath()
    // platform.system.notify() thay vì new Notification()
    
    await this.executeAutomationWorkflow(params)
  }

  private async executeAutomationWorkflow(params: AutomationTriggerParams): Promise<void> {
    // Delegate đến WorkflowOrchestrator (platform-agnostic)
    await this.workflowOrchestrator.triggerFromEvent({
      eventType:  params.triggerType,
      payload:    params.payload,
      userId:     params.userId,
      projectId:  params.projectId,
    })
  }
}
```

---

## BUG-AT-002 & BUG-BE-AT-001 — Fix event-based trigger not implemented

**Mức độ:** 🟠 HIGH  
**Root cause:** Automation chỉ hỗ trợ manual trigger, không có event-driven triggers (git push, PR open, etc.).

### Fix — Implement EventBasedAutomationTrigger

```typescript
// src/main/automation/EventAutomationTrigger.ts (NEW)

export type AutomationEventType =
  | 'git.push'
  | 'git.pr.opened'
  | 'git.pr.merged'
  | 'schedule.cron'
  | 'workflow.completed'
  | 'agent.task.completed'

export interface AutomationRule {
  id:          string
  userId:      string
  projectId:   string
  eventType:   AutomationEventType
  conditions?: Record<string, unknown>  // e.g. { branch: 'main' }
  workflowId:  string
  enabled:     boolean
}

export class EventAutomationTrigger {
  constructor(
    private readonly repository: IAutomationRuleRepository,
    private readonly orchestrator: WorkflowOrchestrator,
    private readonly eventBus: EventBus,
    private readonly log: Logger,
  ) {
    // Subscribe to system events
    this.eventBus.on('git.push', (event) => this.handleEvent('git.push', event))
    this.eventBus.on('git.pr.opened', (event) => this.handleEvent('git.pr.opened', event))
    this.eventBus.on('workflow.completed', (event) => this.handleEvent('workflow.completed', event))
    this.eventBus.on('agent.task.completed', (event) => this.handleEvent('agent.task.completed', event))
  }

  private async handleEvent(eventType: AutomationEventType, payload: unknown): Promise<void> {
    const rules = await this.repository.findByEventType(eventType)
    
    for (const rule of rules) {
      if (!rule.enabled) continue
      if (!this.matchesConditions(rule.conditions, payload)) continue

      this.log.info(`[Automation] Triggering rule: ${rule.id} for event: ${eventType}`)
      
      await this.orchestrator.startExecution({
        workflowId: rule.workflowId,
        userId:     rule.userId,
        projectId:  rule.projectId,
        trigger:    { type: 'event', eventType, payload },
      }).catch(err => {
        this.log.error(`[Automation] Rule trigger failed: ${rule.id}`, err)
      })
    }
  }

  private matchesConditions(conditions?: Record<string, unknown>, payload?: unknown): boolean {
    if (!conditions || Object.keys(conditions).length === 0) return true
    if (!payload || typeof payload !== 'object') return false

    return Object.entries(conditions).every(([key, value]) =>
      (payload as Record<string, unknown>)[key] === value
    )
  }
}

// CronAutomationTrigger:
export class CronAutomationTrigger {
  private jobs = new Map<string, NodeJS.Timeout>()

  scheduleRule(rule: AutomationRule & { cronExpression: string }): void {
    // Parse cron và schedule next run
    const nextRunMs = getNextCronRunMs(rule.cronExpression)
    const timeout = setTimeout(async () => {
      await this.orchestrator.startExecution({ ... })
      // Reschedule
      this.scheduleRule(rule)
    }, nextRunMs)
    this.jobs.set(rule.id, timeout)
  }
}
```

---

## BUG-AT-003 — Fix remote host scheduling disabled by default

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Automation scheduler bị disable khi `remoteHost` mode được detect.

### Fix — Enable automation cho cả remote và local modes

```typescript
// src/main/automation/AutomationService.ts

// TRƯỚC:
if (process.env.ORCA_REMOTE_HOST === '1') {
  this.log.warn('[Automation] Remote host mode — scheduling disabled')
  return  // BUG: skip automation
}

// SAU — Enable cho tất cả modes:
async initialize(): Promise<void> {
  // Không check remote host mode
  await this.loadAndScheduleRules()
  this.log.info('[Automation] Scheduler started')
}

private async loadAndScheduleRules(): Promise<void> {
  const rules = await this.repository.listEnabled()
  for (const rule of rules) {
    if (rule.eventType === 'schedule.cron' && rule.cronExpression) {
      this.cronTrigger.scheduleRule(rule as any)
    }
  }
}
```

---

## BUG-BE-AT-002 — Fix WorktreeCleanupService not implemented

**Mức độ:** 🟠 HIGH  
**Root cause:** Worktree cleanup (xóa worktrees cũ, merged branches) chưa được implement.

### Fix — Implement WorktreeCleanupService

```typescript
// src/main/worktree/WorktreeCleanupService.ts (NEW)

export interface WorktreeCleanupConfig {
  maxAgeMs:       number  // default: 7 ngày
  minDiskSpaceMb: number  // default: 500MB
  dryRun:         boolean // default: false
}

export class WorktreeCleanupService {
  constructor(
    private readonly worktreeRepository: IWorktreeRepository,
    private readonly devServerManager: DevServerManager,
    private readonly auditLogger: AuditLogger,
    private readonly config: WorktreeCleanupConfig,
    private readonly log: Logger,
  ) {}

  /**
   * Cleanup worktrees đủ điều kiện:
   * 1. Worktrees từ merged PRs
   * 2. Worktrees inactive > maxAgeMs
   * 3. Worktrees khi disk space < minDiskSpaceMb
   */
  async runCleanup(userId?: string): Promise<CleanupReport> {
    const worktrees = userId
      ? await this.worktreeRepository.listByUser(userId)
      : await this.worktreeRepository.listAll()

    const report: CleanupReport = { removed: [], errors: [], skipped: [] }

    for (const wt of worktrees) {
      const reason = await this.shouldCleanup(wt)
      if (!reason) {
        report.skipped.push(wt.id)
        continue
      }

      if (this.config.dryRun) {
        this.log.info(`[Cleanup] DryRun — would remove: ${wt.id} reason=${reason}`)
        continue
      }

      try {
        await this.removeWorktree(wt, reason)
        report.removed.push({ id: wt.id, reason })
      } catch (err) {
        report.errors.push({ id: wt.id, error: String(err) })
      }
    }

    return report
  }

  private async shouldCleanup(wt: Worktree): Promise<string | null> {
    // Check age
    const ageMs = Date.now() - wt.lastActiveAt
    if (ageMs > this.config.maxAgeMs) return `inactive_${Math.round(ageMs / 86400000)}d`

    // Check if branch merged
    if (wt.prStatus === 'merged') return 'pr_merged'

    // Check disk space
    const bridge = this.devServerManager.getBridge(wt.devServerId)
    if (bridge) {
      const diskInfo = await bridge.call('system.getDiskInfo', { path: wt.path }).catch(() => null)
      if (diskInfo && diskInfo.availableMb < this.config.minDiskSpaceMb) return 'low_disk_space'
    }

    return null
  }

  private async removeWorktree(wt: Worktree, reason: string): Promise<void> {
    const bridge = this.devServerManager.getBridge(wt.devServerId)
    if (bridge) {
      await bridge.call('git.worktree.remove', { path: wt.path, force: true })
    }

    await this.worktreeRepository.delete(wt.id)
    await this.auditLogger.logAction(wt.userId, '', 'worktree.cleanup', { worktreeId: wt.id, reason }, '')
    this.log.info(`[Cleanup] Removed worktree: ${wt.id} reason=${reason}`)
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/automation/AutomationService.ts` | Remove Electron imports, dùng platform abstraction | AT-001 |
| `src/main/automation/EventAutomationTrigger.ts` | NEW — event-driven trigger | AT-002, BE-AT-001 |
| `src/main/automation/CronAutomationTrigger.ts` | NEW — cron scheduler | AT-002, BE-AT-001 |
| `src/main/automation/AutomationService.ts` | Enable remote host mode | AT-003 |
| `src/main/worktree/WorktreeCleanupService.ts` | NEW — cleanup service | BE-AT-002 |
| `src/main/db/migrations/0009_automation_rules.ts` | NEW migration | BE-AT-001 |

---

## Verification Plan

```bash
pnpm tsc --noEmit -p config/tsconfig.node.json
pnpm vitest run src/main/automation/__tests__/
pnpm vitest run src/main/worktree/__tests__/cleanup.test.ts
```
