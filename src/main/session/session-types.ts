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
}
