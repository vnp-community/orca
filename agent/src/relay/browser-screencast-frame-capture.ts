// src/relay/browser-screencast-frame-capture.ts
// Frame encode/notify + the two ways a frame reaches the client — live
// Page.screencastFrame events (handleScreencastFrame) and the
// navigation-triggered Page.captureScreenshot fallback
// (emitFallbackSnapshot) — split out of browser-screencast-handler.ts
// (which owns session lifecycle/start/stop) to keep each file under this
// repo's max-lines budget without resorting to a disable comment.

import { Buffer } from 'node:buffer'
import type { AgentLogger } from './agent-logger'
import type { CdpClient } from './cdp-client'
import {
  BrowserScreencastOpcode,
  encodeBrowserScreencastFrame,
  type BrowserScreencastFormat,
  type BrowserScreencastFrameMetadata
} from '../shared/browser-screencast-protocol'
import { readBrowserScreencastImageSize } from '../shared/browser-screencast-image-size'

// Matches agent-rpc-dispatch.ts's makeNotifier return type exactly — every
// browser.screencast* event this file emits (frame) travels through this,
// not a synchronous RPC response.
export type NotifyFn = (method: string, params: Record<string, unknown>) => void

// ScreencastSession is the mutable state one live screencast needs across
// both this file (frame capture) and browser-screencast-handler.ts
// (start/stop lifecycle) — defined here since frame capture is what reads/
// mutates most of its fields (seq, lastFrameSentAt).
export type ScreencastSession = {
  cdp: CdpClient
  worktreeId: string
  format: BrowserScreencastFormat
  seq: number
  deviceMetricsOverridden: boolean
  lastFrameSentAt: number
  stopping: boolean
}

function finiteOrUndefined(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function readMetadata(raw: Record<string, unknown> | undefined): BrowserScreencastFrameMetadata {
  const m = raw ?? {}
  const pick = (k: string): number | undefined => finiteOrUndefined(m[k])
  return {
    offsetTop: pick('offsetTop'),
    pageScaleFactor: pick('pageScaleFactor'),
    deviceWidth: pick('deviceWidth'),
    deviceHeight: pick('deviceHeight'),
    imageWidth: pick('imageWidth'),
    imageHeight: pick('imageHeight'),
    scrollOffsetX: pick('scrollOffsetX'),
    scrollOffsetY: pick('scrollOffsetY'),
    timestamp: pick('timestamp')
  }
}

export function encodeAndNotifyFrame(
  session: ScreencastSession,
  image: Uint8Array,
  metadata: BrowserScreencastFrameMetadata,
  notify: NotifyFn
): void {
  session.lastFrameSentAt = Date.now()
  const encoded = encodeBrowserScreencastFrame({
    opcode: BrowserScreencastOpcode.Frame,
    seq: session.seq++,
    format: session.format,
    metadata,
    image
  })
  notify('browser.screencastFrame', {
    worktreeId: session.worktreeId,
    dataBase64: Buffer.from(encoded).toString('base64')
  })
}

// handleScreencastFrame processes one live Page.screencastFrame CDP event:
// ACKs it (always, even when dropped/throttled — CDP stalls otherwise),
// throttles by minFrameIntervalMs (dropping, not queuing — see this
// package's header comment on why), and otherwise decodes + notifies it.
export function handleScreencastFrame(
  session: ScreencastSession,
  cdp: CdpClient,
  base64Data: string,
  cdpSessionId: number,
  rawMetadata: Record<string, unknown> | undefined,
  opts: { viewportWidth?: number; viewportHeight?: number; minFrameIntervalMs: number },
  notify: NotifyFn,
  log: AgentLogger
): void {
  const ack = (): void => {
    void cdp.send('Page.screencastFrameAck', { sessionId: cdpSessionId }).catch(() => {})
  }

  if (session.stopping) {
    ack()
    return
  }

  const now = Date.now()
  if (
    opts.minFrameIntervalMs > 0 &&
    session.lastFrameSentAt !== 0 &&
    now - session.lastFrameSentAt < opts.minFrameIntervalMs
  ) {
    ack()
    return
  }

  try {
    const image = Buffer.from(base64Data, 'base64')
    const imageSize = readBrowserScreencastImageSize(image, session.format)
    const metadata = readMetadata(rawMetadata)
    if (imageSize) {
      metadata.imageWidth = imageSize.width
      metadata.imageHeight = imageSize.height
      if (metadata.deviceWidth === undefined) {
        metadata.deviceWidth = opts.viewportWidth ?? imageSize.width
      }
      if (metadata.deviceHeight === undefined) {
        metadata.deviceHeight = opts.viewportHeight ?? imageSize.height
      }
    }
    encodeAndNotifyFrame(session, image, metadata, notify)
  } catch (err) {
    log.debug(`browser.screencast: dropping unreadable frame: ${err instanceof Error ? err.message : String(err)}`)
  }
  ack()
}

// emitFallbackSnapshot mirrors the OLD backend's emitSnapshotFrame CDP-only
// path (Page.captureScreenshot) — dropping its Electron webContents.capturePage
// branch entirely, since there is no Electron surface here to capture from.
export async function emitFallbackSnapshot(
  session: ScreencastSession,
  cdp: CdpClient,
  opts: { format: BrowserScreencastFormat; quality: number; viewportWidth?: number; viewportHeight?: number },
  notify: NotifyFn,
  log: AgentLogger
): Promise<void> {
  if (session.stopping) {
    return
  }
  try {
    const result = (await cdp.send('Page.captureScreenshot', {
      format: opts.format,
      ...(opts.format === 'jpeg' ? { quality: opts.quality } : {}),
      ...(opts.viewportWidth && opts.viewportHeight
        ? {
            clip: { x: 0, y: 0, width: opts.viewportWidth, height: opts.viewportHeight, scale: 1 },
            captureBeyondViewport: false
          }
        : {})
    })) as { data?: string }
    if (session.stopping || typeof result.data !== 'string') {
      return
    }
    const image = Buffer.from(result.data, 'base64')
    const imageSize = readBrowserScreencastImageSize(image, opts.format)
    const metadata: BrowserScreencastFrameMetadata = {}
    if (opts.viewportWidth && opts.viewportHeight) {
      metadata.deviceWidth = opts.viewportWidth
      metadata.deviceHeight = opts.viewportHeight
    } else if (imageSize) {
      metadata.deviceWidth = imageSize.width
      metadata.deviceHeight = imageSize.height
    }
    if (imageSize) {
      metadata.imageWidth = imageSize.width
      metadata.imageHeight = imageSize.height
    }
    encodeAndNotifyFrame(session, image, metadata, notify)
  } catch (err) {
    // Best effort only: live Page.screencastFrame events still drive the
    // stream; a failed fallback capture is not fatal.
    log.debug(`browser.screencast: fallback snapshot capture failed: ${err instanceof Error ? err.message : String(err)}`)
  }
}
