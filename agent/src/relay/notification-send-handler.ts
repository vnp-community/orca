// src/relay/notification-send-handler.ts
// Handles 'notification.send' RPC calls from workflow 'notification' steps.
// Called by: StepExecutors.executeNotification() via
//   relay.call('notification.send', { channel, message, traceId })
// Previously unimplemented (specs/agent/api/gaps-and-findings.md #1 —
// confirmed by the previously MethodNotFound-asserting test in
// agent-rpc-dispatch.test.ts).
//
// The dev-server agent runs headless (no desktop session in the common case),
// so there is no general slack/email delivery infra here — that belongs on
// the backend, which owns user identity and integration credentials. What
// this handler CAN do reliably: acknowledge the step so the workflow doesn't
// fail with MethodNotFound, log the notification for operator visibility,
// and best-effort deliver an OS-level desktop notification when the agent
// happens to run on a host that has one available (e.g. a Linux desktop dev
// box with notify-send, or macOS). Delivery failure is never fatal — a
// notification step should not fail a workflow run.

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'

const execFileAsync = promisify(execFile)
const notifyTracer = createTracer('agent:notification')

const OS_NOTIFY_TIMEOUT_MS = 3_000

async function tryOsNotify(title: string, message: string): Promise<boolean> {
  try {
    if (process.platform === 'linux') {
      await execFileAsync('notify-send', [title, message], { timeout: OS_NOTIFY_TIMEOUT_MS })
      return true
    }
    if (process.platform === 'darwin') {
      // osascript has no argv-injection risk here: values are passed as a
      // single AppleScript string built with JSON.stringify-escaped quotes,
      // not shell-interpolated (execFile, no shell).
      const script = `display notification ${JSON.stringify(message)} with title ${JSON.stringify(title)}`
      await execFileAsync('osascript', ['-e', script], { timeout: OS_NOTIFY_TIMEOUT_MS })
      return true
    }
    return false
  } catch {
    // Missing binary (headless dev server, the common case), no DISPLAY/no
    // notification daemon running, timeout, etc. — all non-fatal.
    return false
  }
}

export async function handleNotificationSend(
  id: string | number | null,
  params: Record<string, unknown>,
  log: AgentLogger
): Promise<object> {
  const channel = typeof params.channel === 'string' ? params.channel : 'default'
  const message = typeof params.message === 'string' ? params.message : ''
  const traceId = typeof params.traceId === 'string' ? params.traceId : undefined
  const span = notifyTracer.start({ method: 'notification.send', channel, traceId })

  if (!message) {
    span.fail('missing param: message', { method: 'notification.send' })
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: message' }
    }
  }

  log.info(`notification.send: channel=${channel} message=${message}`)

  const delivered = await tryOsNotify(`Orca (${channel})`, message)
  span.ok({ channel, delivered })

  return {
    jsonrpc: '2.0', id,
    result: {
      ok: true,
      delivered,
      note: delivered
        ? 'delivered via OS notification'
        : 'no OS notification target available on this host — logged only'
    }
  }
}
