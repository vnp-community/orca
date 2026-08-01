# TASK-AT-002: Implement event-based automation triggers

**Priority:** 🟠 HIGH — Automation chỉ chạy manual, không có event triggers  
**Effort:** ~60 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AT-002, BUG-BE-AT-001  
**Solution ref:** [SOLUTION-automation.md](../solutions/SOLUTION-automation.md)

## Mục tiêu

Thêm event subscription cho `git.push`, `pr.created`, `worktree.created` events để trigger automation rules.

## File cần sửa / tạo

`src/main/automation/AutomationEventBridge.ts` (NEW hoặc sửa AutomationService)

## Pattern

```typescript
// AutomationEventBridge.ts
export class AutomationEventBridge {
  constructor(
    private readonly automationService: AutomationService,
    private readonly eventBus:          EventBus,
  ) {
    this.subscribe()
  }

  private subscribe(): void {
    this.eventBus.on('git.push',         (e) => this.onGitPush(e))
    this.eventBus.on('pr.created',       (e) => this.onPRCreated(e))
    this.eventBus.on('worktree.created', (e) => this.onWorktreeCreated(e))
  }

  private async onGitPush(event: GitPushEvent): Promise<void> {
    const rules = await this.automationService.getRulesForTrigger({
      type: 'git.push',
      projectId: event.projectId,
    })
    for (const rule of rules) {
      await this.automationService.executeRule(rule, event)
    }
  }
  // ... similar for other events
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: trigger git.push event → automation rule fires
```
