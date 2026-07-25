# SOL-LG-003 — SSH Dev Server: Per-User Unix Account Isolation

**CR:** [CR-LOGIN-003](../../../../../docs/crs/v1/login/CR-LOGIN-003-ssh-isolation.md)
**TDD Refs:** TDD-05 (SSH Relay — deployAndLaunchRelay, SshRelaySession), TDD-13 (Dev Server Onboarding — DevServerManager)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Implemented (2026-07-24)
**Blocked by:** SOL-LG-002 (cần `userId` routing vào user process)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 SSH Relay hiện tại (TDD-05 §3)

```typescript
// src/main/ssh/ssh-relay-deploy.ts — hiện tại
// deployAndLaunchRelay(conn, ...) → relay chạy với user của SshTarget.username
// SshTarget.username = 'ubuntu' (shared cho mọi users)

// src/main/ssh/ssh-relay-session.ts
// SshRelaySession.establish(conn) → register SshPtyProvider + SshGitProvider
// Providers này dùng relay chạy dưới ubuntu@dev-machine → SHARED
```

### 1.2 Giải pháp: SSH Username Override per User

Không thay đổi `deployAndLaunchRelay()` hay `SshRelaySession`. Thêm 2 module nhỏ:
1. **`ssh-user-resolver.ts`**: Map `userId` → `linuxUsername` (`orca-alice`)
2. **`dev-server-provisioner.ts`**: SSH vào dev server và tạo unix account nếu chưa có

Khi user process cần connect SSH, resolve target với username override.

---

## 2. File Structure

```
src/main/ssh/
├── ssh-user-resolver.ts              ← [NEW] userId → linux username mapping
├── dev-server-provisioner.ts         ← [NEW] Tạo unix account trên dev server
└── __tests__/
    ├── ssh-user-resolver.test.ts
    └── dev-server-provisioner.test.ts

scripts/dev-server/
└── provision-user.sh                 ← [NEW] Shell script chạy trên dev server

src/main/ssh/ssh-connection-store.ts  ← [MODIFY] Thêm resolveSshTargetForUser()
```

---

## 3. Test Specifications

### 3.1 `ssh-user-resolver.test.ts`

```typescript
// src/main/ssh/__tests__/ssh-user-resolver.test.ts
import { describe, it, expect } from 'vitest'
import {
  toLinuxUsername,
  resolveUserSshTarget,
  isValidLinuxUsername
} from '../ssh-user-resolver'
import type { SshTarget } from '../../../shared/ssh-types'

const baseTarget: SshTarget = {
  id: 'target-1', host: '172.20.2.31', port: 22,
  username: 'ubuntu', source: 'user', owner: 'runtime-1'
}

describe('toLinuxUsername', () => {
  it('converts email local part to safe linux username', () => {
    expect(toLinuxUsername('alice@company.com')).toBe('orca-alice')
  })

  it('replaces dots and special chars with hyphens', () => {
    expect(toLinuxUsername('alice.smith@co.com')).toBe('orca-alice-smith')
  })

  it('truncates to 20 chars after prefix', () => {
    const long = toLinuxUsername('averylongusernamethatexceedslimit@test.com')
    const local = long.replace(/^orca-/, '')
    expect(local.length).toBeLessThanOrEqual(20)
  })

  it('handles names with numbers', () => {
    expect(toLinuxUsername('user123@test.com')).toBe('orca-user123')
  })

  it('produces valid linux username (no spaces, starts with letter)', () => {
    const result = toLinuxUsername('test-user@example.com')
    expect(isValidLinuxUsername(result)).toBe(true)
  })
})

describe('toLinuxUsername — uniqueness for similar emails', () => {
  it('generates different usernames for email/name collisions by appending userId suffix', () => {
    const a = toLinuxUsername('alice.smith@a.com', 'userId-aaa')
    const b = toLinuxUsername('alice-smith@b.com', 'userId-bbb')
    // When email local parts collide, userId hash disambiguates
    expect(a).not.toBe(b)
  })

  it('returns same result for same email + userId', () => {
    const r1 = toLinuxUsername('same@test.com', 'uid-1')
    const r2 = toLinuxUsername('same@test.com', 'uid-1')
    expect(r1).toBe(r2)
  })
})

describe('resolveUserSshTarget', () => {
  it('overrides username with per-user linux username', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(resolved.username).toBe('orca-alice')
    expect(resolved.host).toBe(baseTarget.host)  // other fields unchanged
    expect(resolved.port).toBe(baseTarget.port)
  })

  it('preserves all other SshTarget fields', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'bob@test.com')
    expect(resolved.id).toBe(baseTarget.id)
    expect(resolved.host).toBe(baseTarget.host)
    expect(resolved.identityFile).toBe(baseTarget.identityFile)
  })
})

describe('isValidLinuxUsername', () => {
  it('accepts valid usernames', () => {
    expect(isValidLinuxUsername('orca-alice')).toBe(true)
    expect(isValidLinuxUsername('orca-user123')).toBe(true)
  })

  it('rejects usernames starting with numbers', () => {
    expect(isValidLinuxUsername('1alice')).toBe(false)
  })

  it('rejects usernames over 32 chars', () => {
    expect(isValidLinuxUsername('a'.repeat(33))).toBe(false)
  })

  it('rejects usernames with spaces', () => {
    expect(isValidLinuxUsername('orca alice')).toBe(false)
  })
})
```

### 3.2 `dev-server-provisioner.test.ts`

```typescript
// src/main/ssh/__tests__/dev-server-provisioner.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { DevServerProvisioner } from '../dev-server-provisioner'
import type { SshConnection } from '../ssh-connection'

describe('DevServerProvisioner', () => {
  let conn: SshConnection
  let provisioner: DevServerProvisioner

  beforeEach(() => {
    conn = {
      exec: vi.fn().mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 })
    } as any
    provisioner = new DevServerProvisioner('/home/orca/.ssh/id_ed25519.pub')
  })

  describe('ensureUserAccount', () => {
    it('skips provisioning if user already exists', async () => {
      // checkUserExists → exit code 0 = user exists
      vi.mocked(conn.exec).mockResolvedValue({ stdout: 'uid=1001(orca-alice)', stderr: '', exitCode: 0 })
      
      await provisioner.ensureUserAccount(conn, 'user-1', 'alice@test.com')
      
      // Should only call id command, not useradd
      expect(conn.exec).toHaveBeenCalledTimes(1)
      expect(conn.exec).toHaveBeenCalledWith(expect.stringContaining('id orca-alice'))
    })

    it('creates user if not exists (exit code 1)', async () => {
      // First call: id check → exit 1 (user not found)
      // Second call: useradd
      // Third call: authorize key
      vi.mocked(conn.exec)
        .mockResolvedValueOnce({ stdout: '', stderr: 'no such user', exitCode: 1 })
        .mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 })

      await provisioner.ensureUserAccount(conn, 'user-1', 'alice@test.com')
      
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      expect(calls.some(c => c.includes('useradd'))).toBe(true)
    })

    it('authorizes Orca Server public key for new user', async () => {
      vi.mocked(conn.exec)
        .mockResolvedValueOnce({ stdout: '', stderr: 'no such user', exitCode: 1 })
        .mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 })

      await provisioner.ensureUserAccount(conn, 'uid-1', 'bob@test.com')
      
      const calls = vi.mocked(conn.exec).mock.calls.map(c => c[0] as string)
      expect(calls.some(c => c.includes('authorized_keys'))).toBe(true)
    })

    it('throws if useradd fails', async () => {
      vi.mocked(conn.exec)
        .mockResolvedValueOnce({ stdout: '', stderr: 'no such user', exitCode: 1 })
        .mockResolvedValueOnce({ stdout: '', stderr: 'permission denied', exitCode: 1 })  // useradd fail

      await expect(provisioner.ensureUserAccount(conn, 'uid-2', 'fail@test.com'))
        .rejects.toThrow('Failed to create unix account')
    })
  })

  describe('checkUserExists', () => {
    it('returns true when id command exits 0', async () => {
      vi.mocked(conn.exec).mockResolvedValue({ stdout: 'uid=1001', stderr: '', exitCode: 0 })
      const exists = await provisioner.checkUserExists(conn, 'orca-alice')
      expect(exists).toBe(true)
    })

    it('returns false when id command exits non-zero', async () => {
      vi.mocked(conn.exec).mockResolvedValue({ stdout: '', stderr: 'no such user', exitCode: 1 })
      const exists = await provisioner.checkUserExists(conn, 'orca-new')
      expect(exists).toBe(false)
    })
  })
})
```

---

## 4. Implementation

### 4.1 `ssh-user-resolver.ts`

```typescript
// src/main/ssh/ssh-user-resolver.ts
import { createHash } from 'node:crypto'
import type { SshTarget } from '../../shared/ssh-types'

const USERNAME_PREFIX    = 'orca-'
const MAX_LOCAL_LENGTH   = 20    // orca- + 20 = 25 chars (under Linux 32 limit)
const SUFFIX_LENGTH      = 4     // 4-char hash suffix for collision avoidance
const VALID_USERNAME_RE  = /^[a-z][a-z0-9-]{0,31}$/

/**
 * Convert email + userId → stable linux username.
 * Format: orca-{sanitized_local}[-{hash}]
 * Example: alice@company.com → orca-alice
 *          alice.smith@a.com (userId=abc123) → orca-alice-smith-a1b2
 */
export function toLinuxUsername(email: string, userId?: string): string {
  const local = email.split('@')[0]!
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')  // non-alphanum → hyphen
    .replace(/-+/g, '-')          // collapse multiple hyphens
    .replace(/^-|-$/g, '')        // trim leading/trailing hyphens
    .slice(0, MAX_LOCAL_LENGTH)

  if (!userId) return `${USERNAME_PREFIX}${local}`

  // Add 4-char hash suffix to disambiguate collisions
  const suffix = createHash('sha256')
    .update(email + userId)
    .digest('hex')
    .slice(0, SUFFIX_LENGTH)

  const truncated = local.slice(0, MAX_LOCAL_LENGTH - SUFFIX_LENGTH - 1)  // -1 for hyphen
  return `${USERNAME_PREFIX}${truncated}-${suffix}`
}

export function isValidLinuxUsername(username: string): boolean {
  return VALID_USERNAME_RE.test(username) && username.length <= 32
}

/**
 * Override SshTarget.username with per-user linux username.
 * Relay will run as orca-{user} instead of shared 'ubuntu'.
 */
export function resolveUserSshTarget(
  baseTarget: SshTarget,
  userId: string,
  userEmail: string
): SshTarget {
  return {
    ...baseTarget,
    username: toLinuxUsername(userEmail, userId)
  }
}
```

### 4.2 `dev-server-provisioner.ts`

```typescript
// src/main/ssh/dev-server-provisioner.ts
import { readFileSync } from 'node:fs'
import { toLinuxUsername } from './ssh-user-resolver'
import type { SshConnection } from './ssh-connection'

export class DevServerProvisioner {
  private readonly orcaPublicKey: string

  constructor(orcaPublicKeyPath: string) {
    this.orcaPublicKey = readFileSync(orcaPublicKeyPath, 'utf-8').trim()
  }

  /**
   * Idempotent: tạo unix account nếu chưa có, authorize Orca Server key.
   * Cần SSH connection với account có sudo (thường là ubuntu với NOPASSWD).
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
    const result = await conn.exec(`id ${linuxUser}`)
    return result.exitCode === 0
  }

  private async createUser(conn: SshConnection, linuxUser: string): Promise<void> {
    // Tạo user với home dir, bash shell, group 'developers' (nếu tồn tại)
    const cmd = [
      `sudo useradd -m -s /bin/bash`,
      `$(getent group developers &>/dev/null && echo '-G developers') `,
      linuxUser
    ].join('')

    const result = await conn.exec(
      `sudo useradd -m -s /bin/bash ${linuxUser} 2>&1 || true && id ${linuxUser}`
    )
    if (result.exitCode !== 0) {
      throw new Error(`Failed to create unix account: ${linuxUser}. stderr: ${result.stderr}`)
    }
  }

  private async authorizeKey(
    conn: SshConnection,
    linuxUser: string,
    publicKey: string
  ): Promise<void> {
    const sshDir     = `/home/${linuxUser}/.ssh`
    const authKeysPath = `${sshDir}/authorized_keys`

    const script = [
      `sudo mkdir -p ${sshDir}`,
      `sudo chmod 700 ${sshDir}`,
      `sudo chown ${linuxUser}:${linuxUser} ${sshDir}`,
      // Idempotent: chỉ thêm nếu chưa có
      `sudo grep -qF '${publicKey}' ${authKeysPath} 2>/dev/null`,
      `  || echo '${publicKey}' | sudo tee -a ${authKeysPath} > /dev/null`,
      `sudo chmod 600 ${authKeysPath}`,
      `sudo chown ${linuxUser}:${linuxUser} ${authKeysPath}`,
    ].join(' && ')

    const result = await conn.exec(script)
    if (result.exitCode !== 0) {
      throw new Error(`Failed to authorize SSH key for ${linuxUser}: ${result.stderr}`)
    }
  }
}
```

### 4.3 `provision-user.sh` — Script độc lập trên dev server

```bash
#!/bin/bash
# scripts/dev-server/provision-user.sh
# Chạy trên dev server (không phải Orca Server)
# Usage: sudo provision-user.sh <linux_user> <orca_public_key>

set -euo pipefail

LINUX_USER="${1:?Usage: $0 <linux_user> <orca_public_key>}"
ORCA_PUBKEY="${2:?Usage: $0 <linux_user> <orca_public_key>}"

# Validate username format
if [[ ! "${LINUX_USER}" =~ ^orca-[a-z][a-z0-9-]{0,20}$ ]]; then
  echo "ERROR: Invalid linux username: ${LINUX_USER}" >&2
  exit 1
fi

# Create user (idempotent)
if ! id "${LINUX_USER}" &>/dev/null; then
  useradd -m -s /bin/bash "${LINUX_USER}"
  # Add to 'developers' group if exists
  getent group developers &>/dev/null && usermod -aG developers "${LINUX_USER}"
  echo "✅ Created user: ${LINUX_USER}"
else
  echo "ℹ️  User exists: ${LINUX_USER}"
fi

# Authorize Orca Server key (idempotent)
SSH_DIR="/home/${LINUX_USER}/.ssh"
AUTH_KEYS="${SSH_DIR}/authorized_keys"

mkdir -p "${SSH_DIR}"
chmod 700 "${SSH_DIR}"
chown "${LINUX_USER}:${LINUX_USER}" "${SSH_DIR}"

if ! grep -qF "${ORCA_PUBKEY}" "${AUTH_KEYS}" 2>/dev/null; then
  echo "${ORCA_PUBKEY}" >> "${AUTH_KEYS}"
fi

chmod 600 "${AUTH_KEYS}"
chown "${LINUX_USER}:${LINUX_USER}" "${AUTH_KEYS}"

echo "✅ Provisioned: ${LINUX_USER}"
```

### 4.4 Tích hợp vào SSH Connection Store

```typescript
// src/main/ssh/ssh-connection-store.ts — MODIFY
// Thêm method resolveSshTargetForUser()

import { resolveUserSshTarget } from './ssh-user-resolver'
import type { OrcaSession } from '../auth/auth-session-store'

export class SshConnectionStore {
  // ... existing methods ...

  /**
   * Resolve SshTarget với username override cho user cụ thể.
   * Dùng trong user process (ORCA_MULTI_USER=1) để mỗi user
   * SSH vào dev server với account riêng.
   */
  resolveSshTargetForUser(targetId: string, session: OrcaSession): SshTarget | undefined {
    const base = this.getTarget(targetId)
    if (!base) return undefined
    return resolveUserSshTarget(base, session.userId, session.userEmail)
  }
}
```

---

## 5. Sudoers Configuration cho Dev Server

```bash
# /etc/sudoers.d/orca-provisioner trên 172.20.2.31
# Chạy: sudo visudo -f /etc/sudoers.d/orca-provisioner

# Allow ubuntu user (Orca Server) to run only provision script
ubuntu ALL=(ALL) NOPASSWD: /usr/local/bin/orca-provision-user.sh

# KHÔNG dùng:  ubuntu ALL=(ALL) NOPASSWD: ALL
```

Deploy script vào dev server:
```bash
sudo cp scripts/dev-server/provision-user.sh /usr/local/bin/orca-provision-user.sh
sudo chmod 755 /usr/local/bin/orca-provision-user.sh
```

---

## 6. Khi nào trigger provisioning?

```typescript
// src/main/ssh/fleet-bootstrap-service.ts — MODIFY (hoặc dev-server-provisioner.ts)
// Gọi DevServerProvisioner.ensureUserAccount() khi:
// 1. User đầu tiên lần đầu connect SSH (lazy provisioning)
// 2. Admin trigger manual qua /admin/api/devservers/{id}/provision

// Trong SshRelaySession.establish():
// BEFORE deployAndLaunchRelay(), resolve username:
const resolvedTarget = sshStore.resolveSshTargetForUser(targetId, session)
// THEN deploy relay với resolvedTarget (username = orca-alice)
```

---

## 7. Relay Path per User

```
BEFORE (shared):
  ubuntu@172.20.2.31
    ~/.orca-relay/{version}/           ← shared relay dir
    ~/.orca-relay/{version}/orca.sock  ← shared socket

AFTER (per user):
  orca-alice@172.20.2.31
    ~/.orca-relay/{version}/           ← alice's relay dir (in /home/orca-alice/)
    ~/.orca-relay/{version}/orca.sock  ← alice's socket (isolated)

  orca-bob@172.20.2.31
    ~/.orca-relay/{version}/           ← bob's relay dir
    ~/.orca-relay/{version}/orca.sock  ← bob's socket
```

Relay binary deploy hoàn toàn isolated vì `home` khác nhau.

---

## 8. Acceptance Criteria

- [x] `ssh-user-resolver.test.ts` — tất cả tests pass
- [x] `dev-server-provisioner.test.ts` — idempotent check, create, authorize key
- [x] `toLinuxUsername('alice@co.com')` → `'orca-alice'`
- [x] `toLinuxUsername('alice.smith@a.com', uid1)` ≠ `toLinuxUsername('alice-smith@b.com', uid2)`
- [x] `provision-user.sh` chạy idempotent (safe to run twice)
- [x] SSH relay deploy: alice nhận `/home/orca-alice/.orca-relay/`, bob nhận `/home/orca-bob/.orca-relay/`
- [x] Audit log ghi: `ssh.connect` với `user_id` và `linux_user`
- [x] `ORCA_MULTI_USER=0`: không gọi user resolver (backward compat)
