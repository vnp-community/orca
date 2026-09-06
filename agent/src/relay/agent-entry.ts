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
//   stdio             — stdin/stdout wired directly to an SSH exec channel by
//                        infra-fleet-service (the Go backend); no WS dial or
//                        listen at all, SSH is the transport & trust boundary

import { loadAgentConfig } from './agent-config'
import { createAgentLogger } from './agent-logger'
import { discoverTools } from './agent-tool-registry'
import { connectDirect } from './agent-connection-direct'
import { listenRelay } from './agent-connection-relay'
import { connectStdio } from './agent-connection-stdio'

// TEMP DIAG BUG-FE-PTY-001: the agent's own outbound WS to Orca sends a raw
// TCP FIN (code 1005, no close frame) within ~6ms of processing 2 concurrent
// pty.attach failures — confirmed via tcpdump the agent's SIDE initiates it,
// but no explicit ws.close()/terminate() call site was found in
// agent-session.ts/agent-rpc-dispatch.ts/agent-wire.ts. If something is
// throwing and getting silently absorbed (or genuinely crashing the process
// hard enough that the normal SIGINT/SIGTERM shutdown path never logs),
// these two handlers are the last place that would ever see it.
process.on('uncaughtException', (err) => {
  process.stderr.write(`[DIAG BUG-FE-PTY-001] uncaughtException: ${err?.stack ?? err}\n`)
})
process.on('unhandledRejection', (reason) => {
  const detail = reason instanceof Error ? (reason.stack ?? reason.message) : String(reason)
  process.stderr.write(`[DIAG BUG-FE-PTY-001] unhandledRejection: ${detail}\n`)
})

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

  // Why this branch also runs before the normal path: infra-fleet-service
  // (the Go backend) SSH-deploys this same agent.js bundle to a remote host
  // and launches it as `node agent.js --stdio`, wiring stdin/stdout directly
  // to an SSH exec channel — no WebSocket dial/listen, no token; the SSH
  // connection itself is the transport and the trust boundary. See
  // agent-connection-stdio.ts.
  if (process.argv.includes('--stdio')) {
    const config = loadAgentConfig({ stdio: true })
    const log = createAgentLogger(config.logLevel)
    log.info('Orca Dev Agent v2.1.0 (stdio mode)')
    log.info(`DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`)
    log.info('Discovering tools...')
    const tools = await discoverTools(config)
    log.info(`Tools ready: ${tools.length} (${tools.map((t) => t.name).join(', ')})`)
    await connectStdio(config, tools, log)
    return
  }

  const config = loadAgentConfig()
  const log = createAgentLogger(config.logLevel)

  log.info('Orca Dev Agent v2.1.0')
  log.info(
    `Mode: ${config.mode}  |  DevServerId: ${config.devServerId}  |  WorkDir: ${config.workDir}`
  )

  log.info('Discovering tools...')
  const tools = await discoverTools(config)
  log.info(`Tools ready: ${tools.length} (${tools.map((t) => t.name).join(', ')})`)

  // Register clean shutdown handlers.
  // Why no PTY cleanup here anymore: terminal (pty.create) PTYs live in the
  // detached pty-daemon process, not this one — that's the entire point
  // (surviving exactly this kind of restart). Only fs.watch watchers, which
  // live in THIS process and have no reattach concept, get cleaned up here.
  const shutdown = (signal: string): void => {
    log.info(`Shutting down (${signal})`)
    // Why a hard-timeout fallback: a live incident on test-01 (2026-09-06)
    // showed this handler's log line firing on SIGTERM yet the process
    // still alive 20+ minutes later (confirmed via systemd's own "Unit
    // process ... remains running after unit stopped" and `ps`) — leaving
    // two agent.js processes racing for the same DEV_SERVER_ID after a
    // `systemctl restart`. Root cause not fully isolated (this handler and
    // agent-connection-direct.ts's own SIGTERM listener both look
    // individually sound), so this guarantees the process always dies
    // within a bounded time regardless of what else is holding the event
    // loop open — `systemctl restart` must always leave exactly one agent
    // process behind.
    const forceExitTimer = setTimeout(() => process.exit(0), 3_000)
    forceExitTimer.unref()
    import('./fs-agent-extensions')
      .then((m) => m.cleanupAgentWatches())
      .catch(() => {
        /* best effort — still exit */
      })
      .finally(() => process.exit(0))
  }
  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))

  await (config.mode === 'relay-websocket'
    ? listenRelay(config, tools, log)
    : connectDirect(config, tools, log))
}

main().catch((err: unknown) => {
  const msg = err instanceof Error ? err.message : String(err)
  console.error(`[agent:FATAL] ${msg}`)
  process.exit(1)
})
