# TASK-020: Tạo `src/main/ssh/dev-server-provisioner.ts` + test + script

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 3 — SSH Isolation
**Solution:** [SOL-LG-003](../solutions/SOL-LG-003-ssh-isolation.md) §4.2, §3.2, §4.3
**Depends on:** TASK-019
**Blocks:** TASK-021 (ssh-connection-store)

---

## Mục tiêu

Tạo `DevServerProvisioner` — idempotently tạo unix account trên dev server qua SSH. Tạo shell script deploy lên server.

---

## File 1: `src/main/ssh/dev-server-provisioner.ts`

```typescript
// src/main/ssh/dev-server-provisioner.ts
import { readFileSync } from 'node:fs'
import { toLinuxUsername } from './ssh-user-resolver'
import type { SshConnection } from './ssh-connection'

export class DevServerProvisioner {
  private readonly orcaPublicKey: string

  constructor(orcaPublicKeyPath: string) {
    // Đọc public key lúc construct — fail fast nếu file không có
    this.orcaPublicKey = readFileSync(orcaPublicKeyPath, 'utf-8').trim()
  }

  /**
   * Idempotent: tạo unix account nếu chưa có, authorize Orca Server SSH public key.
   * Cần conn có account với sudo NOPASSWD (thường ubuntu).
   * Trả về linux username đã provisioned.
   */
  async ensureUserAccount(
    conn: SshConnection,
    userId: string,
    userEmail: string
  ): Promise<string> {
    const linuxUser = toLinuxUsername(userEmail, userId)

    const exists = await this.checkUserExists(conn, linuxUser)
    if (!exists) {
      await this.createUser(conn, linuxUser)
    }
    await this.authorizeKey(conn, linuxUser, this.orcaPublicKey)

    return linuxUser
  }

  async checkUserExists(conn: SshConnection, linuxUser: string): Promise<boolean> {
    const result = await conn.exec(`id ${linuxUser} 2>&1`)
    return result.exitCode === 0
  }

  private async createUser(conn: SshConnection, linuxUser: string): Promise<void> {
    // Thêm vào group 'developers' nếu tồn tại (idempotent)
    const result = await conn.exec(
      `sudo useradd -m -s /bin/bash ${linuxUser} 2>&1 && id ${linuxUser}`
    )
    if (result.exitCode !== 0) {
      throw new Error(
        `Failed to create unix account '${linuxUser}'. stderr: ${result.stderr || result.stdout}`
      )
    }
    // Try add to developers group (ignore error if group doesn't exist)
    await conn.exec(
      `getent group developers &>/dev/null && sudo usermod -aG developers ${linuxUser} || true`
    )
  }

  private async authorizeKey(
    conn: SshConnection,
    linuxUser: string,
    publicKey: string
  ): Promise<void> {
    const sshDir      = `/home/${linuxUser}/.ssh`
    const authKeyPath = `${sshDir}/authorized_keys`

    // Idempotent: mkdir, set perms, append key only if not already present
    const script = [
      `sudo mkdir -p ${sshDir}`,
      `sudo chmod 700 ${sshDir}`,
      `sudo chown ${linuxUser}:${linuxUser} ${sshDir}`,
      `sudo grep -qF '${publicKey}' ${authKeyPath} 2>/dev/null || echo '${publicKey}' | sudo tee -a ${authKeyPath} > /dev/null`,
      `sudo chmod 600 ${authKeyPath}`,
      `sudo chown ${linuxUser}:${linuxUser} ${authKeyPath}`
    ].join(' && ')

    const result = await conn.exec(script)
    if (result.exitCode !== 0) {
      throw new Error(
        `Failed to authorize SSH key for ${linuxUser}: ${result.stderr || result.stdout}`
      )
    }
  }
}
```

---

## File 2: `src/main/ssh/__tests__/dev-server-provisioner.test.ts`

```typescript
// src/main/ssh/__tests__/dev-server-provisioner.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { DevServerProvisioner } from '../dev-server-provisioner'

// Mock readFileSync để không cần real key file
vi.mock('node:fs', () => ({
  readFileSync: vi.fn().mockReturnValue('ssh-ed25519 AAAAC3... orca@server')
}))

function makeConn(exitCode = 0, stdout = '') {
  return {
    exec: vi.fn().mockResolvedValue({ exitCode, stdout, stderr: '' })
  }
}

describe('DevServerProvisioner', () => {
  let provisioner: DevServerProvisioner

  beforeEach(() => {
    provisioner = new DevServerProvisioner('/home/orca/.ssh/id_ed25519.pub')
  })

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

  describe('ensureUserAccount', () => {
    it('skips useradd if user already exists', async () => {
      const conn = makeConn(0, 'uid=1001')
      await provisioner.ensureUserAccount(conn as any, 'uid-1', 'alice@test.com')
      // Only 1 call for checkUserExists + authorize (no useradd call)
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
  })
})
```

---

## File 3: `scripts/dev-server/provision-user.sh`

```bash
#!/bin/bash
# scripts/dev-server/provision-user.sh
# Deploy lên dev server: sudo cp provision-user.sh /usr/local/bin/orca-provision-user.sh
# Usage: sudo /usr/local/bin/orca-provision-user.sh <linux_user> <orca_public_key>

set -euo pipefail

LINUX_USER="${1:?Usage: $0 <linux_user> <orca_public_key>}"
ORCA_PUBKEY="${2:?Usage: $0 <linux_user> <orca_public_key>}"

# Validate username format (must start with orca-)
if [[ ! "${LINUX_USER}" =~ ^orca-[a-z][a-z0-9-]{0,20}$ ]]; then
  echo "ERROR: Invalid linux username: ${LINUX_USER}" >&2
  echo "       Must match: orca-[a-z][a-z0-9-]{0,20}" >&2
  exit 1
fi

# Create user (idempotent)
if ! id "${LINUX_USER}" &>/dev/null; then
  useradd -m -s /bin/bash "${LINUX_USER}"
  # Add to 'developers' group if exists
  getent group developers &>/dev/null && usermod -aG developers "${LINUX_USER}" || true
  echo "✅ Created user: ${LINUX_USER}"
else
  echo "ℹ️  User exists: ${LINUX_USER}"
fi

# Authorize Orca Server SSH key (idempotent)
SSH_DIR="/home/${LINUX_USER}/.ssh"
AUTH_KEYS="${SSH_DIR}/authorized_keys"

mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"
chown "${LINUX_USER}:${LINUX_USER}" "${SSH_DIR}"

if ! grep -qF "${ORCA_PUBKEY}" "${AUTH_KEYS}" 2>/dev/null; then
  echo "${ORCA_PUBKEY}" >> "${AUTH_KEYS}"
  echo "✅ Authorized Orca key for: ${LINUX_USER}"
else
  echo "ℹ️  Key already authorized for: ${LINUX_USER}"
fi

chmod 600 "${AUTH_KEYS}"
chown "${LINUX_USER}:${LINUX_USER}" "${AUTH_KEYS}"

echo "✅ Done: ${LINUX_USER}"
```

---

## Cách chạy test

```bash
pnpm test src/main/ssh/__tests__/dev-server-provisioner.test.ts
```

---

## Acceptance Criteria

- [x] `dev-server-provisioner.ts` tồn tại, TypeScript compile sạch
- [x] `checkUserExists()` → true/false theo exit code
- [x] `ensureUserAccount()` skip useradd nếu user đã tồn tại
- [x] `ensureUserAccount()` throw nếu useradd fail
- [x] `ensureUserAccount()` authorize SSH key luôn (idempotent)
- [x] `provision-user.sh` có shebang, `set -euo pipefail`, validate username format
- [x] Test: tất cả 6 test cases pass
