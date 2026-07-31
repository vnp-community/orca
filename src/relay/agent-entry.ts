// src/relay/agent-entry.ts
// Orca Dev Agent v2.1 — main process entry point.
//
// Replaces: deploy/dev/agent/agent.js (CommonJS v1.0)
// Built by: pnpm run build:agent → out/relay/agent.js
// Deploy:   scp out/relay/agent.js user@devserver:/home/ubuntu/orca-agent/agent.js
//
// Modes:
//   direct-websocket  — Agent connects outbound to Orca Server (default)
//   relay-websocket   — Orca Server connects inbound to Agent (behind NAT/firewall)

import { loadAgentConfig } from './agent-config'
import { createAgentLogger } from './agent-logger'
import { discoverTools } from './agent-tool-registry'
import { connectDirect } from './agent-connection-direct'
import { listenRelay } from './agent-connection-relay'

async function main(): Promise<void> {
  const config = loadAgentConfig()
  const log = createAgentLogger(config.logLevel)

  log.info('Orca Dev Agent v2.1.0')
  log.info(`Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`)

  log.info('Discovering tools...')
  const tools = await discoverTools(config)
  log.info(`Tools ready: ${tools.length} (${tools.map(t => t.name).join(', ')})`)

  // Register clean shutdown handlers
  process.on('SIGINT',  () => { log.info('Shutting down (SIGINT)');  process.exit(0) })
  process.on('SIGTERM', () => { log.info('Shutting down (SIGTERM)'); process.exit(0) })

  if (config.mode === 'relay-websocket') {
    await listenRelay(config, tools, log)
  } else {
    await connectDirect(config, tools, log)
  }
}

main().catch((err: unknown) => {
  const msg = err instanceof Error ? err.message : String(err)
  console.error(`[agent:FATAL] ${msg}`)
  process.exit(1)
})
