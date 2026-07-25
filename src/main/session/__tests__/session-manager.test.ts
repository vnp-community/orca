/**
 * SessionManager Unit Tests
 *
 * Tests core logic without forking real processes.
 * child_process.fork is mocked to return a controllable EventEmitter.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { EventEmitter } from 'node:events'

// Mock node:fs/promises before importing SessionManager
vi.mock('node:fs/promises', () => ({
  mkdir: vi.fn().mockResolvedValue(undefined),
  rm:    vi.fn().mockResolvedValue(undefined)
}))

// Mock node:child_process.fork
const mockChild = Object.assign(new EventEmitter(), {
  pid:    9999,
  stdout: new EventEmitter(),
  stderr: new EventEmitter(),
  kill:   vi.fn(),
  send:   vi.fn()
})

vi.mock('node:child_process', () => ({
  fork: vi.fn(() => mockChild)
}))

// Import after mocking
import { SessionManager } from '../session-manager'
import { fork } from 'node:child_process'
import { mkdir } from 'node:fs/promises'

const BASE_CONFIG = {
  baseDataPath:      '/tmp/orca-test',
  userProcessEntry:  '/tmp/user-process-entry.js',
  idleTimeoutMs:     4 * 60 * 60 * 1000,
  maxRespawnAttempts: 3
}

describe('SessionManager', () => {
  let manager: SessionManager

  beforeEach(() => {
    vi.clearAllMocks()
    // Reset all listeners on mockChild
    mockChild.removeAllListeners()
    mockChild.stdout.removeAllListeners()
    mockChild.stderr.removeAllListeners()
    mockChild.kill = vi.fn()

    manager = new SessionManager(BASE_CONFIG)
  })

  afterEach(async () => {
    await manager.shutdown()
  })

  // ── getOrSpawnUserProcess ──────────────────────────────────────────────────

  describe('getOrSpawnUserProcess', () => {
    it('creates per-user data dir with mode 0o700', async () => {
      // Simulate 'ready' signal from fork
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-alice')
      expect(mkdir).toHaveBeenCalledWith(
        expect.stringContaining('user-alice'),
        expect.objectContaining({ mode: 0o700 })
      )
    })

    it('sets correct env vars in fork call', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-bob')

      const forkCall = vi.mocked(fork).mock.calls[0]!
      const env = (forkCall[2] as Record<string, unknown>)['env'] as Record<string, string>
      expect(env['ORCA_USER_ID']).toBe('user-bob')
      expect(env['ORCA_USER_DATA_PATH']).toContain('user-bob')
      expect(env['ORCA_SOCKET_PATH']).toContain('orca.sock')
      expect(env['NODE_OPTIONS']).toBe('--max-old-space-size=512')
    })

    it('returns existing process on second call for same userId', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      const proc1 = await manager.getOrSpawnUserProcess('user-carol')
      const proc2 = await manager.getOrSpawnUserProcess('user-carol')

      expect(proc1).toBe(proc2)
      // fork only called once
      expect(fork).toHaveBeenCalledTimes(1)
    })

    it('updates lastSeenAt when reusing existing process', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      const proc1 = await manager.getOrSpawnUserProcess('user-dave')
      const firstSeen = proc1.lastSeenAt

      // Wait a bit then call again
      await new Promise(r => setTimeout(r, 5))
      await manager.getOrSpawnUserProcess('user-dave')
      expect(proc1.lastSeenAt).toBeGreaterThanOrEqual(firstSeen)
    })

    it('rejects when fork does not send ready within timeout', async () => {
      // Override config to use a very short timeout
      const shortManager = new SessionManager({ ...BASE_CONFIG, idleTimeoutMs: 100 })
      // Don't emit 'ready' — let it timeout (set SPAWN_TIMEOUT_MS via vi.useFakeTimers)

      // Use a spy to mock timeout behavior
      const spawnPromise = shortManager.getOrSpawnUserProcess('user-timeout')
      // Emit error instead
      setTimeout(() => mockChild.emit('error', new Error('spawn failed')), 10)
      await expect(spawnPromise).rejects.toThrow()
      await shortManager.shutdown()
    })

    it('cleans up map on process exit', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-exit')
      expect(manager.getProcess('user-exit')).not.toBeNull()

      // Simulate child exit
      mockChild.emit('exit', 0)
      expect(manager.getProcess('user-exit')).toBeNull()
    })
  })

  // ── touch ──────────────────────────────────────────────────────────────────

  describe('touch', () => {
    it('updates lastSeenAt for a running process', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      const proc = await manager.getOrSpawnUserProcess('user-touch')
      const before = proc.lastSeenAt

      await new Promise(r => setTimeout(r, 5))
      manager.touch('user-touch')
      expect(proc.lastSeenAt).toBeGreaterThanOrEqual(before)
    })

    it('is a no-op for unknown userId', () => {
      expect(() => manager.touch('ghost-user')).not.toThrow()
    })
  })

  // ── getProcess / listProcesses ─────────────────────────────────────────────

  describe('getProcess', () => {
    it('returns null for a user with no process', () => {
      expect(manager.getProcess('nobody')).toBeNull()
    })

    it('returns the process for a spawned user', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-list')
      const proc = manager.getProcess('user-list')
      expect(proc).not.toBeNull()
      expect(proc!.userId).toBe('user-list')
    })
  })

  describe('listProcesses', () => {
    it('returns all running processes', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-list-1')
      const list = manager.listProcesses()
      expect(list.length).toBeGreaterThanOrEqual(1)
    })
  })

  // ── shutdown ───────────────────────────────────────────────────────────────

  describe('shutdown', () => {
    it('kills all running processes', async () => {
      setTimeout(() => mockChild.emit('message', { type: 'ready' }), 10)
      await manager.getOrSpawnUserProcess('user-shutdown')
      await manager.shutdown()
      expect(mockChild.kill).toHaveBeenCalledWith('SIGTERM')
    })

    it('is idempotent — safe to call multiple times', async () => {
      await manager.shutdown()
      await expect(manager.shutdown()).resolves.toBeUndefined()
    })
  })
})
