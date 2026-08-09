/**
 * Session Manager Types — User process sandbox
 *
 * @module main/session/session-types
 */

import type { ChildProcess } from 'node:child_process'

/** Represents a running user process (forked child) for a specific userId */
export type UserProcess = {
  userId:       string
  pid:          number
  socketPath:   string  // Unix domain socket path the user process listens on
  authToken:    string  // RPC auth token (from OrcaRuntimeRpcServer) for Unix socket
  startedAt:    number  // Unix ms
  lastSeenAt:   number  // Unix ms — updated on WS activity
  process:      ChildProcess
  respawnCount: number
}

/** Configuration for SessionManager */
export type SessionManagerConfig = {
  /** Base directory for all per-user data: /data/orca — users/<userId>/ will be created here */
  baseDataPath:        string
  /** Absolute path to the built user-process-entry.js (compiled entry point) */
  userProcessEntry:    string
  /** Milliseconds before an idle process is killed. Default: 4h */
  idleTimeoutMs?:      number
  /** Max times a crashed process will be respawned. Default: 3 */
  maxRespawnAttempts?: number
  /**
   * Master secret for WebCredentialStore (from ORCA_SERVER_SECRET env var).
   * When set, credential env vars are injected into each user child process
   * at spawn time so integration clients read from env without calling safeStorage.
   */
  serverSecret?:       string
  /**
   * Global DevServerManager to proxy dev server requests from user processes.
   */
  devServerManager: import('../dev-server/dev-server-manager').DevServerManager
}
