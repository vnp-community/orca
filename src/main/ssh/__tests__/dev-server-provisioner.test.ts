/**
 * DevServerProvisioner Unit Tests
 *
 * Mocks readFileSync and SshConnection.exec — no real SSH needed.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock node:fs before importing provisioner
vi.mock('node:fs', () => ({
  readFileSync: vi.fn().mockReturnValue('ssh-ed25519 AAAAC3... orca@server')
}))

import { DevServerProvisioner } from '../dev-server-provisioner'

function makeConn(exitCode = 0, stdout = '') {
  return {
    exec: vi.fn().mockResolvedValue({ exitCode, stdout, stderr: '' })
  }
}

describe('DevServerProvisioner', () => {
  let provisioner: DevServerProvisioner

  beforeEach(() => {
    vi.clearAllMocks()
    provisioner = new DevServerProvisioner('/home/orca/.ssh/id_ed25519.pub')
  })

  // ── checkUserExists ────────────────────────────────────────────────────────

  describe('checkUserExists', () => {
    it('returns true when id command exits 0', async () => {
      const conn = makeConn(0, 'uid=1001(orca-alice)')
      expect(await provisioner.checkUserExists(conn as any, 'orca-alice')).toBe(true)
    })

    it('returns false when id command exits non-zero', async () => {
      const conn = makeConn(1, '')
      expect(await provisioner.checkUserExists(conn as any, 'orca-new')).toBe(false)
    })
  })

  // ── ensureUserAccount ──────────────────────────────────────────────────────

  describe('ensureUserAccount', () => {
    it('skips useradd if user already exists', async () => {
      const conn = makeConn(0, 'uid=1001')
      await provisioner.ensureUserAccount(conn as any, 'uid-1', 'alice@test.com')
      // All calls succeed with exit 0 → no useradd needed
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      expect(calls.some(c => c.includes('useradd'))).toBe(false)
    })

    it('calls useradd when user does not exist', async () => {
      const conn: any = {
        exec: vi.fn()
          .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: 'no such user' })  // id check
          .mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' })                   // useradd + authorize
      }
      await provisioner.ensureUserAccount(conn, 'uid-2', 'bob@test.com')
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      expect(calls.some(c => c.includes('useradd'))).toBe(true)
    })

    it('authorizes Orca Server public key', async () => {
      const conn: any = {
        exec: vi.fn()
          .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: '' })  // id check (not exists)
          .mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' })
      }
      await provisioner.ensureUserAccount(conn, 'uid-3', 'carol@test.com')
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      expect(calls.some(c => c.includes('authorized_keys'))).toBe(true)
    })

    it('throws if useradd fails', async () => {
      const conn: any = {
        exec: vi.fn()
          .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: 'not exists' })
          .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: 'permission denied' })
      }
      await expect(provisioner.ensureUserAccount(conn, 'uid-4', 'fail@test.com'))
        .rejects.toThrow('Failed to create unix account')
    })

    it('returns the linux username', async () => {
      const conn = makeConn(0)
      const result = await provisioner.ensureUserAccount(conn as any, 'uid-5', 'dave@test.com')
      expect(result).toMatch(/^orca-/)
    })

    it('includes public key content in authorize call', async () => {
      const conn = makeConn(0, 'uid=1001')  // user exists already → skip useradd
      await provisioner.ensureUserAccount(conn as any, 'uid-6', 'eve@test.com')
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      // The public key mock is 'ssh-ed25519 AAAAC3... orca@server'
      expect(calls.some(c => c.includes('ssh-ed25519'))).toBe(true)
    })
  })
})
