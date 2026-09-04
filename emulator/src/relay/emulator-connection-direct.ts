// src/relay/emulator-connection-direct.ts
// Direct-websocket connection mode: Mobile Emulator Agent dials outbound to
// the Orca backend. Mirrors the shape of agent/src/relay/agent-connection-direct.ts
// (direct-websocket is the right mode for a personal machine behind NAT —
// see specs/emulator/tdd/v1/04-deployment.md) but drastically simplified:
// no AgentTokenManager/token renewal, no diagnostic instrumentation — just
// enough reconnect-with-backoff to survive a network blip.
import WebSocket from 'ws'
import type { EmulatorConfig } from './emulator-config'
import type { EmulatorLogger } from './emulator-logger'
import { createEmulatorSession } from './emulator-session'
import type { EmulatorRpcDispatcher } from './emulator-rpc-dispatch'

/** Reconnect backoff delays (ms): 1s → 2s → 5s → 15s → 30s (max). */
const RECONNECT_DELAYS_MS = [1_000, 2_000, 5_000, 15_000, 30_000]

type ConnectionOutcome = 'exit' | 'reconnect'

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function connectDirect(
  config: EmulatorConfig,
  log: EmulatorLogger,
  dispatcher: EmulatorRpcDispatcher
): Promise<never> {
  if (!config.backendUrl) {
    throw new Error('connectDirect requires config.backendUrl (ORCA_BACKEND_URL)')
  }
  const backendUrl = config.backendUrl

  let reconnectAttempt = 0

  const runConnection = (): Promise<ConnectionOutcome> =>
    new Promise((resolve) => {
      log.info(`Connecting to ${backendUrl} ...`)
      const ws = new WebSocket(backendUrl, {
        headers: { 'User-Agent': 'orca-emulator-agent/0.1.0' }
      })

      const session = createEmulatorSession(config, log, dispatcher)
      session.onHandshakeOk(() => {
        log.info('Connection established and authenticated.')
      })
      session.start(ws)

      ws.once('close', (code: number) => {
        session.stop()
        if (code === 1000) {
          log.info('Connection closed cleanly (code=1000). Shutting down.')
          resolve('exit')
          return
        }
        log.warn(`Connection dropped (code=${code}). Reconnecting...`)
        resolve('reconnect')
      })

      ws.once('error', (err: Error) => {
        log.warn(`WebSocket error: ${err.message}. Reconnecting...`)
        // 'close' fires after 'error' — let it resolve the promise.
      })
    })

  // eslint-disable-next-line no-constant-condition -- intentional reconnect loop, mirrors agent-connection-direct.ts
  while (true) {
    const result = await runConnection()

    if (result === 'exit') {
      return new Promise<never>(() => {
        setTimeout(() => process.exit(0), 100)
      })
    }

    const delay = RECONNECT_DELAYS_MS[Math.min(reconnectAttempt, RECONNECT_DELAYS_MS.length - 1)]!
    reconnectAttempt += 1
    log.info(`Reconnect in ${delay / 1000}s (attempt ${reconnectAttempt})...`)
    await sleep(delay)
  }
}
