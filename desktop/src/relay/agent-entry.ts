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
  // Why this branch runs before anything else: pty-daemon-client.ts spawns
  // this SAME agent.js file with ORCA_PTY_DAEMON_SOCKET set to run as the
  // detached PTY daemon instead of a normal agent (see pty-daemon-server.ts
  // for why PTYs live there — surviving an agent process restart). The
  // daemon needs none of the normal agent's config/tool-discovery/WS setup.
  const daemonSocketPath = process.env['ORCA_PTY_DAEMON_SOCKET']
  if (daemonSocketPath) {
    const daemonLog = createAgentLogger(process.env['ORCA_LOG_LEVEL'] ?? 'info')
    const { runPtyDaemon } = await import('./pty-daemon-server')
    await runPtyDaemon(daemonSocketPath, daemonLog)
    return
  }

  const config = loadAgentConfig()
  const log = createAgentLogger(config.logLevel)

  log.info('Orca Dev Agent v2.1.0')
  log.info(`Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`)

  log.info('Discovering tools...')
  const tools = await discoverTools(config)
  log.info(`Tools ready: ${tools.length} (${tools.map(t => t.name).join(', ')})`)

  // Register clean shutdown handlers.
  // Why no PTY cleanup here anymore: terminal (pty.create) PTYs live in the
  // detached pty-daemon process, not this one — that's the entire point
  // (surviving exactly this kind of restart). Only fs.watch watchers, which
  // live in THIS process and have no reattach concept, get cleaned up here.
  const shutdown = (signal: string): void => {
    log.info(`Shutting down (${signal})`)
    import('./fs-agent-extensions')
      .then((m) => m.cleanupAgentWatches())
      .catch(() => { /* best effort — still exit */ })
      .finally(() => process.exit(0))
  }
  process.on('SIGINT',  () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))

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
