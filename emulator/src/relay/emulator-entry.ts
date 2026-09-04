// src/relay/emulator-entry.ts
// Orca Mobile Emulator Agent — entry point.
//
// Two modes, selected by whether ORCA_BACKEND_URL is set:
//   - Set:     direct-websocket mode (emulator-connection-direct.ts) — dials
//              the real backend using orca-dev-agent-transport's wire codec
//              (TASK-EMU-001/006).
//   - Unset:   stdio debug mode — one JSON-RPC 2.0 request per stdin line,
//              one response per stdout line. Kept so device-*-handler.ts
//              logic stays runnable/testable end-to-end without a live
//              backend (see specs/emulator/tdd/v1/03-transport-reuse-analysis.md
//              and 04-deployment.md).
import { createInterface } from 'node:readline'
import { loadEmulatorConfig } from './emulator-config'
import { createEmulatorLogger } from './emulator-logger'
import { createEmulatorRpcDispatcher, type JsonRpcRequest } from './emulator-rpc-dispatch'
import { connectDirect } from './emulator-connection-direct'

export async function main(): Promise<void> {
  const config = loadEmulatorConfig()
  const log = createEmulatorLogger(config.logLevel)
  const dispatcher = createEmulatorRpcDispatcher(log)

  if (config.backendUrl) {
    log.info(`orca-emulator-agent starting in direct-websocket mode (${config.backendUrl})`)
    await connectDirect(config, log, dispatcher)
    return
  }

  log.info('ORCA_BACKEND_URL not set — starting in stdio debug mode (see specs/emulator/tdd/v1/04-deployment.md)')

  const rl = createInterface({ input: process.stdin, terminal: false })
  rl.on('line', (line: string) => {
    const trimmed = line.trim()
    if (!trimmed) return

    let rpc: JsonRpcRequest
    try {
      rpc = JSON.parse(trimmed) as JsonRpcRequest
    } catch {
      process.stdout.write(
        `${JSON.stringify({ jsonrpc: '2.0', id: null, error: { code: -32700, message: 'parse error' } })}\n`
      )
      return
    }

    dispatcher
      .dispatch(rpc)
      .then((response) => {
        process.stdout.write(`${JSON.stringify(response)}\n`)
      })
      .catch((error: unknown) => {
        log.error(`unexpected dispatch failure: ${error instanceof Error ? error.stack : String(error)}`)
      })
  })

  rl.on('close', () => {
    log.info('stdin closed — exiting')
  })
}

void main()
