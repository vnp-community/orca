/**
 * relay-daemon-pidfile.ts — PID-file lifecycle for a daemonized relay
 * process. Part of TASK-FLEET-02-08's minimal daemon/health primitive (see
 * relay-daemon.ts's doc comment for the full picture): BL-FLEET-02's
 * "Health Check" step assumes `~/.local/bin/orca-relay --daemon` leaves
 * behind a PID file a caller can stat/read independent of the process that
 * spawned it — this module is that primitive, factored out so it's testable
 * without actually spawning a child process.
 *
 * Deliberately NOT wired into relay.ts's existing --detached/--connect
 * Unix-socket reconnect flow — that mechanism already solves "does a relay
 * for this workspace exist" for relay.ts's own callers via socket-connect
 * probing (see relay.ts's runConnectMode). This PID file is a separate,
 * simpler liveness primitive for BL-FLEET-02's specific `curl .../health`
 * + PID-file contract, which needs to work without dialing a Unix socket.
 */
import { existsSync, mkdirSync, readFileSync, unlinkSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'

/** Default PID file path — mirrors pty-daemon-client.ts's
 *  `~/orca-agent/...` convention for this codebase's other detached daemon. */
export function defaultRelayPidFilePath(): string {
  return join(homedir(), 'orca-agent', 'relay-daemon.pid')
}

/** Writes pid (default: the current process) to pidFilePath, creating its
 *  parent directory if needed. */
export function writeRelayPidFile(pidFilePath: string, pid: number = process.pid): void {
  mkdirSync(dirname(pidFilePath), { recursive: true })
  writeFileSync(pidFilePath, String(pid), 'utf8')
}

/** Removes pidFilePath if present. Never throws — cleanup on shutdown must
 *  not itself fail the shutdown path. */
export function removeRelayPidFile(pidFilePath: string): void {
  try {
    unlinkSync(pidFilePath)
  } catch {
    // fine — already absent, or a permissions issue that shutdown can't fix anyway
  }
}

/** Reads and parses pidFilePath's PID. Returns null if the file is absent,
 *  empty, or does not contain a valid positive integer. */
export function readRelayPidFile(pidFilePath: string): number | null {
  if (!existsSync(pidFilePath)) {
    return null
  }
  let raw: string
  try {
    raw = readFileSync(pidFilePath, 'utf8').trim()
  } catch {
    return null
  }
  const pid = Number.parseInt(raw, 10)
  return Number.isInteger(pid) && pid > 0 ? pid : null
}

/** Reports whether pid refers to a currently-running process, via the
 *  standard `kill(pid, 0)` liveness probe (sends no signal — just tests
 *  existence/permission). Cross-platform: Node's process.kill implements
 *  the same signal-0 semantics on Windows too. */
export function isProcessAlive(pid: number): boolean {
  try {
    process.kill(pid, 0)
    return true
  } catch (err) {
    // EPERM means the process exists but is owned by another user — still
    // "alive" for our purposes (this daemon's PID files are always this
    // user's own processes in practice, but a stale/reused PID from a
    // different user's process should not be reported as alive; ESRCH is
    // the only case that means "actually not running").
    return (err as NodeJS.ErrnoException)?.code === 'EPERM'
  }
}

/** Reads pidFilePath and reports whether the PID it names is a live
 *  process — the read+liveness-check pair BL-FLEET-02's health check
 *  performs against the PID file independent of the HTTP /health probe. */
export function isRelayDaemonRunning(pidFilePath: string): boolean {
  const pid = readRelayPidFile(pidFilePath)
  return pid !== null && isProcessAlive(pid)
}
