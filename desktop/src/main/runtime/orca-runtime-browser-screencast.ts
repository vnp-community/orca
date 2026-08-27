// frontend/src/main/runtime/orca-runtime-browser-screencast.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-066): browser screencast streaming
// command extracted from OrcaRuntimeService via the composition pattern.
// Single method (previously the largest undivided method left in
// orca-runtime.ts) with a self-contained connection/page stream-registry
// pair — clean host deps once cross-checked (already-public forwarding
// fields from orca-runtime-browser.ts/TASK-BIGFILE-016,
// orca-runtime-mobile-floor.ts/TASK-BIGFILE-037, and
// orca-runtime-connection-subscription-notify.ts/TASK-BIGFILE-058).
import { BrowserError } from '../browser/cdp-bridge'
import type { BrowserScreencastResult } from '../../shared/runtime-types'
import type { RuntimeBrowserCommands } from './orca-runtime-browser'
import type { RuntimeMobileFloorCommands } from './orca-runtime-mobile-floor'

type ActiveBrowserScreencastStream = {
  cancel: (emitEnd?: boolean) => void
  done: Promise<void>
  connectionKey: string
}

export type RuntimeBrowserScreencastCommandHost = {
  browserScreencast: RuntimeBrowserCommands['browserScreencast']
  getBrowserDriver: RuntimeMobileFloorCommands['getBrowserDriver']
  setBrowserDriver: RuntimeMobileFloorCommands['setBrowserDriver']
  registerSubscriptionCleanup(
    subscriptionId: string,
    cleanup: () => void,
    connectionId?: string
  ): void
  cleanupSubscription(subscriptionId: string): void
}

export class RuntimeBrowserScreencastCommands {
  private readonly activeBrowserScreencastsByConnection = new Map<
    string,
    ActiveBrowserScreencastStream
  >()
  private readonly activeBrowserScreencastsByPage = new Map<string, ActiveBrowserScreencastStream>()

  constructor(private readonly host: RuntimeBrowserScreencastCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (mobile-floor host wiring, TASK-BIGFILE-037) — public, not private.
  cancelBrowserScreencastForPage(browserPageId: string): void {
    this.activeBrowserScreencastsByPage.get(browserPageId)?.cancel(true)
  }

  async browserScreencast(
    params: Parameters<RuntimeBrowserCommands['browserScreencast']>[0],
    options: {
      connectionId?: string
      sendBinary?: (bytes: Uint8Array<ArrayBufferLike>) => boolean | void
      signal?: AbortSignal
      emit: (result: BrowserScreencastResult) => void
    }
  ): Promise<void> {
    if (!options.sendBinary) {
      throw new BrowserError(
        'browser_error',
        'Browser screencast requires a binary streaming transport.'
      )
    }

    const connectionKey = options.connectionId ?? 'local'
    const requestedPageId = typeof params.page === 'string' ? params.page : null
    let existingPageStream = requestedPageId
      ? this.activeBrowserScreencastsByPage.get(requestedPageId)
      : undefined
    while (existingPageStream) {
      // Why: CDP only supports one screencast per browser page. A stale paired
      // web/mobile stream should not leave the next tab activation stuck on an
      // already-active error or old viewport dimensions.
      existingPageStream.cancel(existingPageStream.connectionKey !== connectionKey)
      await existingPageStream.done
      existingPageStream = requestedPageId
        ? this.activeBrowserScreencastsByPage.get(requestedPageId)
        : undefined
    }
    let existingStream = this.activeBrowserScreencastsByConnection.get(connectionKey)
    while (existingStream) {
      existingStream.cancel()
      await existingStream.done
      existingStream = this.activeBrowserScreencastsByConnection.get(connectionKey)
    }
    if (options.signal?.aborted) {
      throw new BrowserError('browser_error', 'Browser screencast was cancelled.')
    }

    let screencast: Awaited<ReturnType<RuntimeBrowserCommands['browserScreencast']>> | null = null
    let registeredSubscriptionId: string | null = null
    let activeBrowserPageId: string | null = null
    let ended = false
    let cancelledBeforeStart = false
    let readyEmitted = false
    let resolveActiveDone!: () => void
    const activeDone = new Promise<void>((resolve) => {
      resolveActiveDone = resolve
    })
    const end = (emitEnd: boolean): void => {
      if (ended) {
        return
      }
      ended = true
      screencast?.session.stop()
      if (emitEnd && screencast) {
        options.emit({ type: 'end', subscriptionId: screencast.subscriptionId })
      }
    }
    const cancel = (emitEnd = false): void => {
      if (!screencast) {
        cancelledBeforeStart = true
        return
      }
      end(emitEnd)
    }
    const abortScreencast = (): void => cancel()
    const sendBinaryAfterReady = (bytes: Uint8Array<ArrayBufferLike>): boolean | void => {
      if (!readyEmitted) {
        // Why: binary screencast frames are connection-scoped; clients learn the
        // owning subscription from `ready`, so CDP frames must remain unacked
        // until the stream's JSON ready event has been delivered.
        return false
      }
      return options.sendBinary?.(bytes)
    }

    // Why: a phone can rotate before the first stream reaches `ready`, so it
    // has no subscriptionId to unsubscribe. A same-socket replacement cancels
    // and waits here instead of racing the active connection/page gates.
    this.activeBrowserScreencastsByConnection.set(connectionKey, {
      cancel,
      done: activeDone,
      connectionKey
    })
    options.signal?.addEventListener('abort', abortScreencast, { once: true })
    try {
      screencast = await this.host.browserScreencast(params, {
        sendBinary: sendBinaryAfterReady,
        emit: options.emit
      })
      if (cancelledBeforeStart || options.signal?.aborted) {
        end(false)
        await screencast.session.done
        return
      }
      activeBrowserPageId = screencast.ready.browserPageId
      this.activeBrowserScreencastsByPage.set(activeBrowserPageId, {
        cancel,
        done: activeDone,
        connectionKey
      })
      this.host.setBrowserDriver(activeBrowserPageId, { kind: 'mobile', clientId: connectionKey })

      // Why: browser screencast frames are connection-scoped media. Registering
      // cleanup ties Page.stopScreencast to the exact remote socket so hidden
      // client panes and dropped connections do not leave Chromium streaming.
      this.host.registerSubscriptionCleanup(
        screencast.subscriptionId,
        () => end(true),
        options.connectionId
      )
      registeredSubscriptionId = screencast.subscriptionId
      options.emit(screencast.ready)
      readyEmitted = true
      await screencast.session.done
      end(true)
      this.host.cleanupSubscription(screencast.subscriptionId)
    } finally {
      options.signal?.removeEventListener('abort', abortScreencast)
      if (!ended) {
        end(false)
      }
      if (registeredSubscriptionId) {
        this.host.cleanupSubscription(registeredSubscriptionId)
      }
      const active = this.activeBrowserScreencastsByConnection.get(connectionKey)
      if (active?.done === activeDone) {
        this.activeBrowserScreencastsByConnection.delete(connectionKey)
      }
      if (activeBrowserPageId) {
        const activePageStream = this.activeBrowserScreencastsByPage.get(activeBrowserPageId)
        if (activePageStream?.done === activeDone) {
          this.activeBrowserScreencastsByPage.delete(activeBrowserPageId)
        }
        const driver = this.host.getBrowserDriver(activeBrowserPageId)
        if (driver.kind === 'mobile' && driver.clientId === connectionKey) {
          this.host.setBrowserDriver(activeBrowserPageId, { kind: 'idle' })
        }
      }
      resolveActiveDone()
    }
  }
}
