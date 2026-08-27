// src/relay/browser-screencast-handler.ts
// browser.screencast/browser.screencastStop — the remote headless-browser
// pane's live-view stream, the one browser.* capability TASK-036's original
// pass left unimplemented (see backend-go/services/api-gateway's
// channels_browser_screencast.go for the wscompat side and why the
// live-view gap was real but the "frontend architecture blocker" framing
// around it was not).
//
// Why this needs its OWN CDP connection (cdp-client.ts), not
// browser-handler.ts's short-lived-CLI-per-op model: a continuous frame
// stream needs a persistent CDP session with an event listener attached —
// `agent-browser` exposes no documented subcommand for that (its own
// `stream enable` wraps a persistent WS but doesn't expose the format/
// quality/maxWidth/maxHeight/everyNthFrame params this feature's contract
// requires; see the design note this PR's description carries). Instead:
// resolve the target Chromium's CDP WebSocket URL via `agent-browser get
// cdp-url`, then speak CDP's plain JSON-RPC-over-WS protocol directly via
// cdp-client.ts — the same approach the OLD Electron desktop bridge's
// CdpWsProxy/AgentBrowserBridge used (backend/src/main/browser/
// agent-browser-bridge.ts's `--cdp <port>` flag), just consuming the
// target end of that protocol instead of proxying it.
//
// Frame encode/decode/notify itself lives in
// browser-screencast-frame-capture.ts (split out to stay under this repo's
// max-lines budget) — ported from
// backend/src/main/browser/browser-screencast-stream.ts (Electron's
// `webContents.debugger` wrapper) with two deliberate simplifications for
// this headless, non-Electron context; see that file's header comment for
// both. Dialog/dialogClosed events (Page.javascriptDialogOpening/Closed)
// are also NOT forwarded — matching channels_browser_screencast.go's own
// documented scope cut. A JS alert()/confirm() during remote navigation
// won't surface in the pane this pass.

import { randomUUID } from 'node:crypto'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { requireWorktreeId, runBrowserCommand } from './browser-handler'
import { CdpClient } from './cdp-client'
import {
  emitFallbackSnapshot,
  handleScreencastFrame,
  type NotifyFn,
  type ScreencastSession
} from './browser-screencast-frame-capture'
import type { BrowserScreencastFormat } from '../shared/browser-screencast-protocol'

const NAVIGATION_CAPTURE_DELAY_MS = 250

type JsonRpcId = string | number | null

type JsonRpcSuccess = { readonly jsonrpc: '2.0'; readonly id: JsonRpcId; readonly result: unknown }
type JsonRpcError = {
  readonly jsonrpc: '2.0'
  readonly id: JsonRpcId
  readonly error: { code: number; message: string }
}
type JsonRpcResponse = JsonRpcSuccess | JsonRpcError

function makeSuccess(id: JsonRpcId, result: unknown): JsonRpcSuccess {
  return { jsonrpc: '2.0', id, result }
}

function makeFailure(id: JsonRpcId, message: string): JsonRpcError {
  return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message } }
}

function clamp(value: number | undefined, lo: number, hi: number, def: number): number {
  if (value === undefined) {
    return def
  }
  return Math.min(hi, Math.max(lo, value))
}

function clampOptional(value: number | undefined, lo: number, hi: number): number | undefined {
  if (value === undefined) {
    return undefined
  }
  return Math.min(hi, Math.max(lo, value))
}

function finiteOrUndefined(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

// ─── Session tracking ────────────────────────────────────────────────────
//
// One live screencast per worktree, keyed the same way browser-handler.ts
// keys its agent-browser sessions — a Map here rather than piggybacking on
// that file's session concept, since a screencast has its own CDP
// connection lifecycle independent of agent-browser's CLI daemon.
const activeSessions = new Map<string, ScreencastSession>()

function extractCdpWsUrl(data: unknown): string {
  if (typeof data === 'string' && data.length > 0) {
    return data
  }
  if (data && typeof data === 'object') {
    const obj = data as Record<string, unknown>
    for (const key of ['cdpUrl', 'url', 'wsUrl', 'webSocketDebuggerUrl']) {
      const v = obj[key]
      if (typeof v === 'string' && v.length > 0) {
        return v
      }
    }
  }
  throw new Error(`BROWSER_SCREENCAST_FAILED: agent-browser returned no CDP WebSocket URL: ${JSON.stringify(data)}`)
}

function browserPageIdFromCdpUrl(url: string): string {
  const match = /\/([^/]+)$/.exec(url)
  return match ? match[1] : randomUUID()
}

function stopScreencastSession(session: ScreencastSession, notify: NotifyFn): void {
  if (session.stopping) {
    return
  }
  session.stopping = true
  activeSessions.delete(session.worktreeId)
  void (async () => {
    await session.cdp.send('Page.stopScreencast').catch(() => {})
    if (session.deviceMetricsOverridden) {
      await session.cdp.send('Emulation.clearDeviceMetricsOverride').catch(() => {})
    }
    session.cdp.close()
  })()
  notify('browser.screencastEnded', { worktreeId: session.worktreeId })
}

// ─── browser.screencast (start) ─────────────────────────────────────────

export async function handleBrowserScreencastStart(
  id: JsonRpcId,
  params: Record<string, unknown>,
  log: AgentLogger,
  notify: NotifyFn
): Promise<JsonRpcResponse> {
  let worktreeId: string
  try {
    worktreeId = requireWorktreeId(params)
  } catch (err) {
    return makeFailure(id, err instanceof Error ? err.message : String(err))
  }

  // A repeated start replaces any previous live screencast for this
  // worktree, mirroring browser-handler.ts's one-session-per-worktree
  // model for agent-browser's own CLI session daemon.
  const existing = activeSessions.get(worktreeId)
  if (existing) {
    stopScreencastSession(existing, notify)
  }

  const format: BrowserScreencastFormat = params.format === 'png' ? 'png' : 'jpeg'
  const quality = clamp(finiteOrUndefined(params.quality), 10, 100, 70)
  const maxWidth = clamp(finiteOrUndefined(params.maxWidth), 320, 3840, 1440)
  const maxHeight = clamp(finiteOrUndefined(params.maxHeight), 240, 2160, 1200)
  const viewportWidth = clampOptional(finiteOrUndefined(params.viewportWidth), 320, 3840)
  const viewportHeight = clampOptional(finiteOrUndefined(params.viewportHeight), 240, 2160)
  const deviceScaleFactor = clampOptional(finiteOrUndefined(params.deviceScaleFactor), 1, 4)
  const mobile = params.mobile === true
  const everyNthFrame = clamp(finiteOrUndefined(params.everyNthFrame), 1, 10, 2)
  const minFrameIntervalMs = clamp(finiteOrUndefined(params.minFrameIntervalMs), 0, 1000, 0)

  try {
    const cdpUrlData = await runBrowserCommand(worktreeId, ['get', 'cdp-url'])
    const cdpUrl = extractCdpWsUrl(cdpUrlData)
    const cdp = await CdpClient.connect(cdpUrl)

    const session: ScreencastSession = {
      cdp,
      worktreeId,
      format,
      seq: 0,
      deviceMetricsOverridden: false,
      lastFrameSentAt: 0,
      stopping: false
    }
    activeSessions.set(worktreeId, session)

    cdp.onClose((reason) => {
      if (session.stopping) {
        return
      }
      session.stopping = true
      activeSessions.delete(worktreeId)
      notify('browser.screencastEnded', { worktreeId })
      log.debug(`browser.screencast: CDP connection for worktree ${worktreeId} closed: ${reason}`)
    })

    await cdp.send('Page.enable')

    if (viewportWidth && viewportHeight) {
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: viewportWidth,
        height: viewportHeight,
        deviceScaleFactor: deviceScaleFactor ?? 1,
        mobile
      })
      await cdp.send('Emulation.setVisibleSize', { width: viewportWidth, height: viewportHeight }).catch(() => {})
      session.deviceMetricsOverridden = true
    }

    let navigationCaptureTimer: ReturnType<typeof setTimeout> | null = null
    const scheduleNavigationCapture = (): void => {
      if (session.stopping) {
        return
      }
      if (navigationCaptureTimer) {
        clearTimeout(navigationCaptureTimer)
      }
      navigationCaptureTimer = setTimeout(() => {
        navigationCaptureTimer = null
        void emitFallbackSnapshot(session, cdp, { format, quality, viewportWidth, viewportHeight }, notify, log)
      }, NAVIGATION_CAPTURE_DELAY_MS)
    }

    // Why: static pages can finish navigation without ever emitting a live
    // Page.screencastFrame, leaving the pane on the previous page's image —
    // same rationale browser-screencast-stream.ts's own frameNavigated/
    // loadEventFired handling gives.
    cdp.on('Page.frameNavigated', (evt) => {
      const frame = evt.frame as Record<string, unknown> | undefined
      if (!frame || !('parentId' in frame)) {
        scheduleNavigationCapture()
      }
    })
    cdp.on('Page.loadEventFired', () => scheduleNavigationCapture())

    cdp.on('Page.screencastFrame', (evt) => {
      const data = evt.data
      const cdpSessionId = evt.sessionId
      if (typeof data !== 'string' || typeof cdpSessionId !== 'number') {
        return
      }
      handleScreencastFrame(
        session,
        cdp,
        data,
        cdpSessionId,
        evt.metadata as Record<string, unknown> | undefined,
        { viewportWidth, viewportHeight, minFrameIntervalMs },
        notify,
        log
      )
    })

    await cdp.send('Page.startScreencast', { format, quality, maxWidth, maxHeight, everyNthFrame })

    const subscriptionId = randomUUID()
    const browserPageId = browserPageIdFromCdpUrl(cdpUrl)
    notify('browser.screencastReady', { worktreeId, subscriptionId, browserPageId, format })

    return makeSuccess(id, { type: 'ack' })
  } catch (err) {
    activeSessions.delete(worktreeId)
    const message = err instanceof Error ? err.message : String(err)
    log.error(`browser.screencastStart failed for worktree ${worktreeId}: ${message}`)
    return makeFailure(id, message)
  }
}

// ─── browser.screencastStop ──────────────────────────────────────────────

export function handleBrowserScreencastStop(
  id: JsonRpcId,
  params: Record<string, unknown>,
  notify: NotifyFn
): JsonRpcResponse {
  let worktreeId: string
  try {
    worktreeId = requireWorktreeId(params)
  } catch (err) {
    return makeFailure(id, err instanceof Error ? err.message : String(err))
  }
  const session = activeSessions.get(worktreeId)
  if (session) {
    stopScreencastSession(session, notify)
  }
  return makeSuccess(id, { type: 'ack' })
}
