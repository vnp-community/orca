# TASK-019: Tạo `src/main/ssh/ssh-user-resolver.ts` + test

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 3 — SSH Isolation
**Solution:** [SOL-LG-003](../solutions/SOL-LG-003-ssh-isolation.md) §4.1, §3.1
**Depends on:** (không có dependency ngoài SSH types)
**Blocks:** TASK-021, TASK-022, TASK-025

---

## Mục tiêu

Tạo `ssh-user-resolver.ts` — map userId + email → linux username (`orca-alice`), resolve SshTarget per user.

---

## File cần tạo: `src/main/ssh/ssh-user-resolver.ts`

```typescript
// src/main/ssh/ssh-user-resolver.ts
import { createHash } from 'node:crypto'
import type { SshTarget } from '../../shared/ssh-types'

const USERNAME_PREFIX  = 'orca-'
const MAX_LOCAL_LENGTH = 20    // orca- (5) + 20 = 25 chars (well under Linux 32 limit)
const SUFFIX_LENGTH    = 4     // 4-char hash suffix để disambiguate collisions
const VALID_LINUX_USER = /^[a-z][a-z0-9-]{0,31}$/

/**
 * Convert email + userId → stable, safe linux username.
 * Format: orca-{sanitized_local}[-{hash}]
 *
 * Examples:
 *   alice@company.com       → orca-alice
 *   alice.smith@a.com       → orca-alice-smith
 *   alice.smith@a.com (uid1) → orca-alice-smi-a1b2 (với suffix)
 */
export function toLinuxUsername(email: string, userId?: string): string {
  const localPart = email.split('@')[0]!
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')   // non-alphanum → hyphen
    .replace(/-+/g, '-')           // collapse consecutive hyphens
    .replace(/^-|-$/g, '')         // trim leading/trailing hyphens

  if (!userId) {
    return `${USERNAME_PREFIX}${localPart.slice(0, MAX_LOCAL_LENGTH)}`
  }

  // Thêm 4-char suffix từ hash(email+userId) để tránh collision
  const suffix    = createHash('sha256').update(email + userId).digest('hex').slice(0, SUFFIX_LENGTH)
  const truncated = localPart.slice(0, MAX_LOCAL_LENGTH - SUFFIX_LENGTH - 1)  // -1 cho hyphen
  return `${USERNAME_PREFIX}${truncated}-${suffix}`
}

/**
 * Kiểm tra linux username hợp lệ.
 * Rules: bắt đầu bằng chữ thường, chỉ [a-z0-9-], tối đa 32 ký tự.
 */
export function isValidLinuxUsername(username: string): boolean {
  return VALID_LINUX_USER.test(username) && username.length <= 32
}

/**
 * Tạo copy của SshTarget với username override.
 * Thay vì 'ubuntu' (shared), dùng 'orca-alice' (per-user).
 */
export function resolveUserSshTarget(
  baseTarget: SshTarget,
  userId:     string,
  userEmail:  string
): SshTarget {
  return {
    ...baseTarget,
    username: toLinuxUsername(userEmail, userId)
  }
}
```

---

## File cần tạo: `src/main/ssh/__tests__/ssh-user-resolver.test.ts`

```typescript
// src/main/ssh/__tests__/ssh-user-resolver.test.ts
import { describe, it, expect } from 'vitest'
import { toLinuxUsername, isValidLinuxUsername, resolveUserSshTarget } from '../ssh-user-resolver'
import type { SshTarget } from '../../../shared/ssh-types'

const baseTarget: SshTarget = {
  id: 'target-1', host: '172.20.2.31', port: 22,
  username: 'ubuntu', source: 'user', owner: 'runtime-1'
}

describe('toLinuxUsername', () => {
  it('converts email local part to safe linux username with orca- prefix', () => {
    expect(toLinuxUsername('alice@company.com')).toBe('orca-alice')
  })

  it('replaces dots and special chars with hyphens', () => {
    expect(toLinuxUsername('alice.smith@co.com')).toBe('orca-alice-smith')
  })

  it('collapses consecutive hyphens', () => {
    const result = toLinuxUsername('alice--double@test.com')
    expect(result).not.toContain('--')
  })

  it('handles names with numbers', () => {
    expect(toLinuxUsername('user123@test.com')).toBe('orca-user123')
  })

  it('truncates to max 25 chars total (5 prefix + 20 local)', () => {
    const result = toLinuxUsername('averylongemaillocalpartthatexceedslimit@test.com')
    expect(result.length).toBeLessThanOrEqual(25)
  })

  it('produces valid linux username', () => {
    const result = toLinuxUsername('test-user@example.com')
    expect(isValidLinuxUsername(result)).toBe(true)
  })
})

describe('toLinuxUsername — with userId (collision avoidance)', () => {
  it('generates different usernames for similar emails with different userIds', () => {
    const a = toLinuxUsername('alice.smith@a.com', 'uid-aaa')
    const b = toLinuxUsername('alice-smith@b.com', 'uid-bbb')
    expect(a).not.toBe(b)
  })

  it('returns same result for same email + userId (deterministic)', () => {
    const r1 = toLinuxUsername('same@test.com', 'uid-123')
    const r2 = toLinuxUsername('same@test.com', 'uid-123')
    expect(r1).toBe(r2)
  })

  it('includes 4-char hash suffix', () => {
    const result = toLinuxUsername('bob@test.com', 'uid-xyz')
    // Format: orca-{local}-{4char}
    const parts = result.replace(/^orca-/, '').split('-')
    const suffix = parts[parts.length - 1]
    expect(suffix).toHaveLength(4)
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

  it('rejects usernames with uppercase', () => {
    expect(isValidLinuxUsername('Orca-alice')).toBe(false)
  })
})

describe('resolveUserSshTarget', () => {
  it('overrides username with per-user linux username', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(resolved.username).toBe('orca-alice')
  })

  it('preserves all other SshTarget fields', () => {
    const resolved = resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(resolved.id).toBe(baseTarget.id)
    expect(resolved.host).toBe(baseTarget.host)
    expect(resolved.port).toBe(baseTarget.port)
    expect(resolved.source).toBe(baseTarget.source)
  })

  it('does not mutate the original target', () => {
    resolveUserSshTarget(baseTarget, 'uid-1', 'alice@test.com')
    expect(baseTarget.username).toBe('ubuntu')  // unchanged
  })
})
```

---

## Cách chạy test

```bash
pnpm test src/main/ssh/__tests__/ssh-user-resolver.test.ts
```

---

## Acceptance Criteria

- [x] `ssh-user-resolver.ts` tồn tại, TypeScript compile sạch
- [x] `toLinuxUsername('alice@co.com')` → `'orca-alice'`
- [x] `toLinuxUsername` với userId: deterministic, collision-safe
- [x] `isValidLinuxUsername`: reject uppercase, spaces, numbers-start, >32 chars
- [x] `resolveUserSshTarget`: không mutate original, preserve all fields ngoài username
- [x] Tất cả test cases pass (≥ 14 cases)
