/**
 * AutomationEventBridge — Event-based automation triggers (TASK-AT-002)
 *
 * Subscribes to system events (git.push, pr.created, worktree.created) and
 * triggers relevant automation rules via AutomationService.
 *
 * Design:
 *   - Uses a simple EventEmitter bus (EventBus) to decouple from Electron IPC.
 *   - AutomationService.listAutomations() is used to find matching rules.
 *   - Non-throwing — errors per-rule are logged but don't block other rules.
 *
 * Usage:
 *   const bridge = new AutomationEventBridge(automationService, eventBus)
 *   bridge.start()   // subscribe to events
 *   bridge.stop()    // unsubscribe cleanly
 *
 * @module main/automations/AutomationEventBridge
 */

import { EventEmitter } from 'node:events'
import type { AutomationService } from './service'

// ── EventBus type ─────────────────────────────────────────────────────────────

export type EventBus = EventEmitter

// ── Event payloads ────────────────────────────────────────────────────────────

export interface GitPushEvent {
  projectId: string
  repoPath:  string
  branch:    string
  pushedBy?: string
}

export interface PullRequestCreatedEvent {
  projectId: string
  prId:      string
  branch:    string
  title?:    string
}

export interface WorktreeCreatedEvent {
  projectId:    string
  worktreePath: string
  branch:       string
}

// ── AutomationEventBridge ─────────────────────────────────────────────────────

export class AutomationEventBridge {
  private readonly handlers: Array<{ event: string; handler: (...args: unknown[]) => void }> = []
  private started = false

  constructor(
    private readonly automationService: AutomationService,
    private readonly eventBus: EventBus
  ) {}

  /** Subscribe to all trigger events. Idempotent. */
  start(): void {
    if (this.started) return
    this.started = true

    this.register('git.push',         (e) => void this.onGitPush(e as GitPushEvent))
    this.register('pr.created',       (e) => void this.onPRCreated(e as PullRequestCreatedEvent))
    this.register('worktree.created', (e) => void this.onWorktreeCreated(e as WorktreeCreatedEvent))

    console.log('[AutomationEventBridge] Started — listening for git.push, pr.created, worktree.created')
  }

  /** Unsubscribe all handlers. */
  stop(): void {
    for (const { event, handler } of this.handlers) {
      this.eventBus.off(event, handler)
    }
    this.handlers.length = 0
    this.started = false
  }

  // ── Private helpers ─────────────────────────────────────────────────────────

  private register(event: string, handler: (...args: unknown[]) => void): void {
    this.eventBus.on(event, handler)
    this.handlers.push({ event, handler })
  }

  // ── Event handlers ──────────────────────────────────────────────────────────

  private async onGitPush(event: GitPushEvent): Promise<void> {
    // FIX TASK-AT-002: Trigger automations for git.push events.
    // Find automations belonging to this project and dispatch them.
    await this.dispatchMatchingAutomations('git.push', event.projectId, {
      eventType: 'git.push',
      branch:    event.branch,
      repoPath:  event.repoPath,
      pushedBy:  event.pushedBy,
    })
  }

  private async onPRCreated(event: PullRequestCreatedEvent): Promise<void> {
    await this.dispatchMatchingAutomations('pr.created', event.projectId, {
      eventType: 'pr.created',
      prId:      event.prId,
      branch:    event.branch,
      title:     event.title,
    })
  }

  private async onWorktreeCreated(event: WorktreeCreatedEvent): Promise<void> {
    await this.dispatchMatchingAutomations('worktree.created', event.projectId, {
      eventType:    'worktree.created',
      worktreePath: event.worktreePath,
      branch:       event.branch,
    })
  }

  /**
   * Find automations for a project that match a trigger type and dispatch each.
   * Non-throwing — per-automation failures are logged individually.
   */
  private async dispatchMatchingAutomations(
    triggerType: string,
    projectId:   string,
    context:     Record<string, unknown>
  ): Promise<void> {
    try {
      // AutomationService.listAutomations() returns all automations from the store.
      // We filter by projectId and match trigger type.
      // Note: 'scheduled' + 'manual' are the existing trigger types;
      // event-based triggers are a new extension recognized by label prefix.
      const allAutomations = (this.automationService as unknown as {
        store: { listAutomations(): Array<{ id: string; name: string; projectId?: string; triggerType?: string; enabled?: boolean }> }
      }).store.listAutomations()

      const matching = allAutomations.filter((a) => {
        if (a.enabled === false) return false
        if (a.projectId && a.projectId !== projectId) return false
        // Match triggerType via explicit field or name prefix convention
        return a.triggerType === triggerType || a.name?.startsWith(`[${triggerType}]`)
      })

      for (const automation of matching) {
        try {
          await this.automationService.dispatchAutomation(automation.id, {
            trigger:   'manual',  // closest existing enum value for event-initiated
            projectId,
            context,
          })
          console.log(`[AutomationEventBridge] Dispatched automation '${automation.name}' for ${triggerType}`)
        } catch (err) {
          console.error(`[AutomationEventBridge] Failed to dispatch '${automation.name}' for ${triggerType}:`, err)
        }
      }
    } catch (err) {
      console.error(`[AutomationEventBridge] Error finding automations for ${triggerType}:`, err)
    }
  }
}
