// frontend/src/main/runtime/orca-runtime-connection-subscription-notify.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-058): mobile connection-scoped
// subscription-cleanup, desktop-notification fan-out, and fit-override
// listener commands extracted from OrcaRuntimeService via the composition
// pattern. Field-span + method-body dependency analysis (TASK-BIGFILE-054,
// corrected per its "Bài học phương pháp" note) confirmed these 9 methods
// are fully self-contained — only `getPushManager()` is a real
// cross-domain dependency.
import type { MobileNotificationEvent } from './orca-runtime-types'
import { addListenerToMap } from './orca-runtime'
import type { WebPushManager } from '../notifications/web-push-manager'

export type RuntimeConnectionSubscriptionNotifyCommandHost = {
  getPushManager(): WebPushManager | null
}

export class RuntimeConnectionSubscriptionNotifyCommands {
  // Why: mobile clients need to know when the desktop restores a terminal
  // from mobile-fit so they can update their UI. These listeners are
  // invoked from resizeForClient and onClientDisconnected/onPtyExit.
  private readonly fitOverrideListeners = new Map<
    string,
    Set<
      (event: {
        mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit'
        cols: number
        rows: number
      }) => void
    >
  >()
  private readonly subscriptionCleanups = new Map<string, () => void>()
  // Why: index of subscriptionIds by per-WebSocket connectionId so the
  // server can sweep all subscriptions for a closing socket without
  // touching subscriptions on other live sockets that share the same
  // deviceToken (multi-screen mobile).
  private readonly subscriptionsByConnection = new Map<string, Set<string>>()
  private readonly subscriptionConnectionByEntry = new Map<string, string>()
  // Why: mobile clients subscribe to desktop notifications via
  // notifications.subscribe. This set enables fan-out — each connected
  // mobile client gets its own listener, and dispatchMobileNotification
  // iterates them all. Listeners are cleaned up via subscriptionCleanups.
  private readonly notificationListeners = new Set<(event: MobileNotificationEvent) => void>()

  constructor(private readonly host: RuntimeConnectionSubscriptionNotifyCommandHost) {}

  subscribeToFitOverrideChanges(
    ptyId: string,
    listener: (event: {
      mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit'
      cols: number
      rows: number
    }) => void
  ): () => void {
    return addListenerToMap(this.fitOverrideListeners, ptyId, listener)
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-floor host wiring) — public, not private.
  notifyFitOverrideListeners(
    ptyId: string,
    mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit',
    cols: number,
    rows: number
  ): void {
    const listeners = this.fitOverrideListeners.get(ptyId)
    if (!listeners) {
      return
    }
    for (const listener of listeners) {
      listener({ mode, cols, rows })
    }
  }

  registerSubscriptionCleanup(
    subscriptionId: string,
    cleanup: () => void,
    connectionId?: string
  ): void {
    // Why: mobile clients reconnect frequently (phone lock, network switch).
    // The RPC client re-sends terminal.subscribe on reconnect, creating a new
    // handler before the old one is cleaned up. Without this, the old data
    // listener leaks in dataListeners and duplicates every PTY data event.
    const existing = this.subscriptionCleanups.get(subscriptionId)
    if (existing) {
      this.cleanupSubscription(subscriptionId)
    }
    this.subscriptionCleanups.set(subscriptionId, cleanup)
    if (connectionId) {
      let set = this.subscriptionsByConnection.get(connectionId)
      if (!set) {
        set = new Set()
        this.subscriptionsByConnection.set(connectionId, set)
      }
      set.add(subscriptionId)
      this.subscriptionConnectionByEntry.set(subscriptionId, connectionId)
    }
  }

  cleanupSubscription(subscriptionId: string): void {
    const cleanup = this.subscriptionCleanups.get(subscriptionId)
    if (cleanup) {
      this.subscriptionCleanups.delete(subscriptionId)
      const connectionId = this.subscriptionConnectionByEntry.get(subscriptionId)
      if (connectionId) {
        this.subscriptionConnectionByEntry.delete(subscriptionId)
        const set = this.subscriptionsByConnection.get(connectionId)
        if (set) {
          set.delete(subscriptionId)
          if (set.size === 0) {
            this.subscriptionsByConnection.delete(connectionId)
          }
        }
      }
      cleanup()
    }
  }

  cleanupSubscriptionsByPrefix(prefix: string): void {
    const ids = Array.from(this.subscriptionCleanups.keys()).filter((id) => id.startsWith(prefix))
    for (const id of ids) {
      this.cleanupSubscription(id)
    }
  }

  // Why: invoked from the WebSocket transport's on-close hook so streaming
  // listeners registered for this exact socket get torn down even when other
  // sockets sharing the same deviceToken are still alive (multi-screen
  // mobile). Without this sweep, listeners leak across every reconnect.
  cleanupSubscriptionsForConnection(connectionId: string): void {
    const set = this.subscriptionsByConnection.get(connectionId)
    if (!set) {
      return
    }
    // Why: snapshot the ids before iterating because cleanupSubscription
    // mutates both the set and the index map.
    const ids = Array.from(set)
    for (const id of ids) {
      this.cleanupSubscription(id)
    }
  }

  // Why: mobile clients subscribe via notifications.subscribe streaming RPC.
  // Each subscriber gets its own listener. Returns an unsubscribe function
  // that the subscription cleanup mechanism calls on disconnect.
  onNotificationDispatched(listener: (event: MobileNotificationEvent) => void): () => void {
    this.notificationListeners.add(listener)
    return () => {
      this.notificationListeners.delete(listener)
    }
  }

  getMobileNotificationListenerCount(): number {
    return this.notificationListeners.size
  }

  dispatchMobileNotification(event: MobileNotificationEvent): void {
    for (const listener of this.notificationListeners) {
      listener(event)
    }
    // TASK-036: fire web push for agent task completions.
    // Fire-and-forget — push errors must never surface to the caller.
    const pushManager = this.host.getPushManager()
    if (event.type === 'notification' && event.source === 'agent-task-complete' && pushManager) {
      pushManager
        .sendToAll({
          title: event.title,
          body: event.body,
          tag: event.worktreeId ? `worktree-${event.worktreeId}` : 'agent-task-complete',
          url: event.worktreeId ? `/worktree/${event.worktreeId}` : undefined
        })
        .catch((err: unknown) => {
          console.error('[WebPush] sendToAll failed:', err)
        })
    }
  }

  dismissMobileNotification(notificationId: string): void {
    this.dispatchMobileNotification({ type: 'dismiss', notificationId })
  }
}
