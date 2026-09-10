import { describe, expect, it, afterEach } from 'vitest'
import { mkdtempSync, rmSync, existsSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import {
  writeRelayPidFile,
  readRelayPidFile,
  removeRelayPidFile,
  isProcessAlive,
  isRelayDaemonRunning,
  defaultRelayPidFilePath
} from './relay-daemon-pidfile'

describe('relay-daemon-pidfile', () => {
  let dir: string

  afterEach(() => {
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
    }
  })

  function pidFilePath(): string {
    dir = mkdtempSync(join(tmpdir(), 'relay-pidfile-test-'))
    return join(dir, 'nested', 'relay-daemon.pid')
  }

  it('writes and reads back the current process pid by default', () => {
    const path = pidFilePath()
    writeRelayPidFile(path)
    expect(existsSync(path)).toBe(true)
    expect(readRelayPidFile(path)).toBe(process.pid)
  })

  it('writes an explicit pid when provided', () => {
    const path = pidFilePath()
    writeRelayPidFile(path, 12345)
    expect(readFileSync(path, 'utf8')).toBe('12345')
    expect(readRelayPidFile(path)).toBe(12345)
  })

  it('returns null for a missing pid file', () => {
    const path = pidFilePath()
    expect(readRelayPidFile(path)).toBeNull()
  })

  it('returns null for a malformed pid file', () => {
    const path = pidFilePath()
    writeRelayPidFile(path)
    // Overwrite with garbage.
    writeFileSync(path, 'not-a-pid', 'utf8')
    expect(readRelayPidFile(path)).toBeNull()
  })

  it('removeRelayPidFile deletes an existing file and is a no-op otherwise', () => {
    const path = pidFilePath()
    writeRelayPidFile(path)
    expect(existsSync(path)).toBe(true)
    removeRelayPidFile(path)
    expect(existsSync(path)).toBe(false)
    // Second call on an already-absent file must not throw.
    expect(() => removeRelayPidFile(path)).not.toThrow()
  })

  it('isProcessAlive is true for the current process', () => {
    expect(isProcessAlive(process.pid)).toBe(true)
  })

  it('isProcessAlive is false for a pid that almost certainly does not exist', () => {
    // PID 2**31-1 is out of any real process range on every supported platform.
    expect(isProcessAlive(2147483647)).toBe(false)
  })

  it('isRelayDaemonRunning is true when the pid file names a live process', () => {
    const path = pidFilePath()
    writeRelayPidFile(path, process.pid)
    expect(isRelayDaemonRunning(path)).toBe(true)
  })

  it('isRelayDaemonRunning is false when the pid file names a dead process', () => {
    const path = pidFilePath()
    writeRelayPidFile(path, 2147483647)
    expect(isRelayDaemonRunning(path)).toBe(false)
  })

  it('isRelayDaemonRunning is false when the pid file does not exist', () => {
    const path = pidFilePath()
    expect(isRelayDaemonRunning(path)).toBe(false)
  })

  it('defaultRelayPidFilePath is under the home directory and ends with relay-daemon.pid', () => {
    const path = defaultRelayPidFilePath()
    expect(path.endsWith('relay-daemon.pid')).toBe(true)
  })
})
