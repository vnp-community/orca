/* eslint-disable unicorn/no-useless-spread -- Why: matches orca-runtime.ts's
own grandfathered disable — the waiter set is cloned before iterating
because resolving a waiter deletes it from the same set mid-iteration. */
// frontend/src/main/runtime/orca-runtime-terminal-message-waiter.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-061): terminal cross-agent-message
// long-poll waiter commands extracted from OrcaRuntimeService via the
// composition pattern. Field-span analysis (TASK-BIGFILE-054) confirmed
// `messageWaitersByHandle` and these 5 methods are fully self-contained —
// only `getLiveLeafForHandle`/`deliverPendingMessages` are real
// cross-domain dependencies.
import { MESSAGE_WAIT_DEFAULT_TIMEOUT_MS } from './orca-runtime-tail-buffer'
import type { RuntimeLeafRecord, TerminalHandleRecord } from './orca-runtime'

type MessageWaiter = {
  handle: string
  typeFilter: string[] | undefined
  resolve: (result: void) => void
  timeout: NodeJS.Timeout | null
  abortCleanup: (() => void) | null
}

export type RuntimeTerminalMessageWaiterCommandHost = {
  getLiveLeafForHandle(handle: string): { record: TerminalHandleRecord; leaf: RuntimeLeafRecord }
  deliverPendingMessages(leaf: RuntimeLeafRecord): void
}

export class RuntimeTerminalMessageWaiterCommands {
  private readonly messageWaitersByHandle = new Map<string, Set<MessageWaiter>>()

  constructor(private readonly host: RuntimeTerminalMessageWaiterCommandHost) {}

  deliverPendingMessagesForHandle(handle: string): void {
    try {
      const { leaf } = this.host.getLiveLeafForHandle(handle)
      if (leaf.lastAgentStatus === 'idle') {
        this.host.deliverPendingMessages(leaf)
      }
    } catch {
      // Unknown or stale handles cannot be pushed immediately; the persisted
      // message remains available via explicit check or future idle delivery.
    }
  }

  // Why: after a message is inserted for a recipient, any blocking
  // orchestration.check --wait calls watching that handle must be woken
  // so they can return the new message immediately instead of polling.
  notifyMessageArrived(handle: string, messageType?: string): void {
    const waiters = this.messageWaitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      // Why: a coordinator waiting for worker_done/escalation should not be
      // woken by worker heartbeat noise and mistake that empty read for idleness.
      if (messageType && waiter.typeFilter && !waiter.typeFilter.includes(messageType)) {
        continue
      }
      this.resolveMessageWaiter(waiter)
    }
  }

  waitForMessage(
    handle: string,
    options?: { typeFilter?: string[]; timeoutMs?: number; signal?: AbortSignal }
  ): Promise<void> {
    return new Promise((resolve) => {
      const timeoutMs = options?.timeoutMs ?? MESSAGE_WAIT_DEFAULT_TIMEOUT_MS

      const waiter: MessageWaiter = {
        handle,
        typeFilter: options?.typeFilter,
        resolve,
        timeout: null,
        abortCleanup: null
      }

      // Why: if the caller aborts (socket closed on the RPC side — see design
      // doc §3.1 counter-lifecycle), resolve immediately so the long-poll slot
      // is released instead of counting down the full timeoutMs with a dead
      // client on the other end.
      const signal = options?.signal
      const onAbort = (): void => {
        this.removeMessageWaiter(waiter)
        resolve()
      }
      if (signal) {
        if (signal.aborted) {
          resolve()
          return
        }
        waiter.abortCleanup = () => signal.removeEventListener('abort', onAbort)
        signal.addEventListener('abort', onAbort, { once: true })
      }

      waiter.timeout = setTimeout(() => {
        this.removeMessageWaiter(waiter)
        resolve()
      }, timeoutMs)

      let waiters = this.messageWaitersByHandle.get(handle)
      if (!waiters) {
        waiters = new Set()
        this.messageWaitersByHandle.set(handle, waiters)
      }
      waiters.add(waiter)
    })
  }

  private resolveMessageWaiter(waiter: MessageWaiter): void {
    this.removeMessageWaiter(waiter)
    waiter.resolve()
  }

  private removeMessageWaiter(waiter: MessageWaiter): void {
    if (waiter.timeout) {
      clearTimeout(waiter.timeout)
      waiter.timeout = null
    }
    if (waiter.abortCleanup) {
      waiter.abortCleanup()
      waiter.abortCleanup = null
    }
    const waiters = this.messageWaitersByHandle.get(waiter.handle)
    if (waiters) {
      waiters.delete(waiter)
      if (waiters.size === 0) {
        this.messageWaitersByHandle.delete(waiter.handle)
      }
    }
  }
}
