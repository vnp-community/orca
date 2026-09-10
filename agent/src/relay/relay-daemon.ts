#!/usr/bin/env node
/**
 * relay-daemon.ts — minimal `--daemon` mode + HTTP `/health` listener + PID
 * file for the relay, closing (partially) the gap TASK-FLEET-02-08 tracked
 * as BLOCKED: BL-FLEET-02's "Start Relay" step
 * (`~/.local/bin/orca-relay --daemon`, PID file) and "Health Check" step
 * (`curl http://localhost:<relayPort>/health`) assumed a daemon binary with
 * an HTTP listener that did not exist anywhere in `agent/` — see this
 * task's spec doc for the full gap analysis.
 *
 * Scope of THIS pass: a genuinely working `--daemon` flag, PID file, and
 * `GET /health` route — the three primitives BL-FLEET-02 names — as a
 * small, self-contained, testable module (relay-daemon-pidfile.ts /
 * relay-daemon-health-server.ts). Deliberately NOT wired into relay.ts's
 * existing PTY/dispatcher runtime (that integration — deciding whether
 * `agent.js`'s Part A dispatcher or a revived `relay.js --detached` mode
 * owns this, then updating sshrelay.Provisioner's deploy step in
 * backend-go — is real follow-up `agent/`+backend-go engineering the
 * original spec doc scoped as a separate pass, not a size TASK-FLEET-02-08
 * could responsibly absorb). What this DOES give BL-FLEET-02's consumers
 * today: a real daemonized process that survives its parent, a PID file at
 * a documented path, and a real HTTP endpoint answering liveness — the spec
 * doc's "None in this pass — do not implement" default was overridden by
 * explicit project-owner direction to deliver this primitive for real
 * rather than only a design note.
 *
 * Process model mirrors pty-daemon-client.ts's spawnDaemon exactly (same
 * codebase, same problem shape: detach a child that outlives the parent):
 * the parent re-execs this same script with `--daemon` appended, spawned
 * `detached: true` + `.unref()`'d; the child (this file, now running with
 * `--daemon` in argv) writes the PID file, starts the health server, and
 * simply never exits on its own — SIGTERM/SIGINT clean up the PID file and
 * stop the health server before exiting.
 */
import { spawn } from 'node:child_process'
import { closeSync, mkdirSync, openSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'

import {
  defaultRelayPidFilePath,
  removeRelayPidFile,
  writeRelayPidFile
} from './relay-daemon-pidfile'
import { RelayDaemonHealthServer } from './relay-daemon-health-server'

export type RelayDaemonArgs = {
  /** True when argv carries `--daemon` — this process IS the long-running
   *  daemon child, not the parent that spawns one. */
  daemon: boolean
  /** HTTP health-listener port. 0 = OS-assigned ephemeral port. */
  port: number
  /** PID file path. Defaults to defaultRelayPidFilePath(). */
  pidFile: string
}

/** Parses the subset of relay CLI flags this module cares about —
 *  deliberately narrow (unlike relay.ts's full parseArgs) since this is a
 *  separate, additive entrypoint, not a replacement for relay.ts's own
 *  flag parsing. */
export function parseRelayDaemonArgs(argv: string[]): RelayDaemonArgs {
  let daemon = false
  let port = 0
  let pidFile = defaultRelayPidFilePath()
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === '--daemon') {
      daemon = true
    } else if (argv[i] === '--port' && argv[i + 1]) {
      const parsed = Number.parseInt(argv[i + 1]!, 10)
      if (!Number.isNaN(parsed) && parsed >= 0) {
        port = parsed
      }
      i++
    } else if (argv[i] === '--pid-file' && argv[i + 1]) {
      pidFile = argv[i + 1]!
      i++
    }
  }
  return { daemon, port, pidFile }
}

function defaultDaemonLogPath(): string {
  return join(homedir(), 'orca-agent', 'logs', 'relay-daemon.log')
}

/** Parent-side: spawns a detached daemon child running this same script
 *  with `--daemon` (plus `--port`/`--pid-file` passthrough), then returns
 *  immediately — mirrors pty-daemon-client.ts's spawnDaemon (same
 *  rationale: a log file, not 'ignore', since the detached+unref'd child's
 *  stdio would otherwise be silently discarded). Does not wait for the
 *  child to become ready — callers that need that should poll the PID file
 *  (isRelayDaemonRunning) or the health endpoint once the port is known. */
export function spawnRelayDaemon(args: { port?: number; pidFile?: string } = {}): void {
  const logPath = defaultDaemonLogPath()
  mkdirSync(dirname(logPath), { recursive: true })
  const logFd = openSync(logPath, 'a')
  const childArgv = ['--daemon']
  if (args.port !== undefined) {
    childArgv.push('--port', String(args.port))
  }
  if (args.pidFile !== undefined) {
    childArgv.push('--pid-file', args.pidFile)
  }
  const child = spawn(process.execPath, [process.argv[1]!, ...childArgv], {
    detached: true,
    stdio: ['ignore', logFd, logFd]
  })
  child.unref()
  closeSync(logFd) // the child holds its own fd table entry; safe to close ours
}

/** Child-side: runs the actual long-lived daemon body — write PID file,
 *  start the health server, wire signal-triggered cleanup. Resolves once
 *  the health server is listening; the process itself stays alive until a
 *  signal (or an explicit stop() call, used by tests) tears it down. */
export async function runRelayDaemon(
  args: RelayDaemonArgs
): Promise<{ port: number; stop: () => void }> {
  writeRelayPidFile(args.pidFile)
  const healthServer = new RelayDaemonHealthServer({ port: args.port })
  const port = await healthServer.start()

  const onSigterm = (): void => shutdown('SIGTERM')
  const onSigint = (): void => shutdown('SIGINT')

  // Why: remove the signal listeners on stop(), not just close the server —
  // otherwise every runRelayDaemon() call (each test run, in particular)
  // leaks a pair of process-level listeners that outlive this daemon
  // instance, eventually tripping Node's MaxListenersExceededWarning.
  const stop = (): void => {
    healthServer.stop()
    removeRelayPidFile(args.pidFile)
    process.off('SIGTERM', onSigterm)
    process.off('SIGINT', onSigint)
  }
  function shutdown(signal: string): void {
    process.stderr.write(`[relay-daemon] shutting down (${signal})\n`)
    stop()
    process.exit(0)
  }
  process.on('SIGTERM', onSigterm)
  process.on('SIGINT', onSigint)

  return { port, stop }
}

// Why: only run the CLI body when this file is the actual entrypoint (not
// when imported for its exports, e.g. by tests or spawnRelayDaemon's own
// parent process before it re-execs). Mirrors pty-daemon-server.ts's own
// module/entrypoint split.
const isDirectlyExecuted = typeof require !== 'undefined' && require.main === module

if (isDirectlyExecuted) {
  const args = parseRelayDaemonArgs(process.argv)
  if (args.daemon) {
    void runRelayDaemon(args).then(({ port }) => {
      process.stderr.write(
        `[relay-daemon] listening: http://127.0.0.1:${port}/health (pid=${process.pid})\n`
      )
    })
  } else {
    spawnRelayDaemon({ pidFile: args.pidFile })
    process.stderr.write(`[relay-daemon] daemon spawned, pid file: ${args.pidFile}\n`)
  }
}
