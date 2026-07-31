# TASK-008: ProfileResolver (Deep-Merge + Cache)

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §2.3  
**Prerequisite:** TASK-007 (ProfileService)  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Implement `ProfileResolver` — 3-layer merge engine với 60s TTL cache. Logic merge phức tạp nhất của TDD-14.

---

## File cần tạo: `src/main/profile/ProfileResolver.ts`

Implement đầy đủ theo [SOL-V5-001 §2.3](../solutions/SOL-V5-001-profile-hierarchy.md):

```typescript
import type { ProfileService } from './ProfileService'
import type { OrcaProfile, ResolvedProfile, ProfileMergeOptions, McpServerConfig } from './OrcaProfile'

const PROFILE_TTL_MS = 60_000

export class ProfileResolver {
  private cache = new Map<string, { resolved: ResolvedProfile; expiresAt: number }>()

  constructor(private readonly profileService: ProfileService) {}

  async resolve(userId: string): Promise<ResolvedProfile> { ... }
  invalidate(userId?: string): void { ... }
}
```

**Merge rules (MUST implement exactly):**

| Section | Merge strategy |
|---------|---------------|
| `security` | Company-locked: user/dept cannot override |
| `agent` | Scalar merge: user > dept > company per-field |
| `editor` | Scalar merge: user > dept > company per-field |
| `shell.defaultShell` | Single value: user > dept > company |
| `shell.pathAdditions` | Concatenate: company + dept + user |
| `shell.envVars` | Object merge: company + dept + user (user overrides) |
| `mcp.servers` | Dedup by `name`: company + dept + user, user wins on conflict |

**Cache:**
- Key: `userId`
- TTL: `60_000ms`
- `resolve()` checks cache first, fetches 3 layers in parallel on miss
- `invalidate(userId?)` — clears specific user or all

**Source tracking:**
- `_sources: Record<string, 'company' | 'dept' | 'user'>` — track which layer provided each field
- Example: `{ 'agent.preferredModel': 'user', 'security.maxSessionHours': 'company' }`

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] `ProfileResolver` class export
- [x] `resolve(userId)` trả về `ResolvedProfile` với `_sources`, `_resolvedAt`
- [x] Cache hit không gọi `ProfileService`
- [x] Cache miss gọi tất cả 3 layers parallel (`Promise.all`)
- [x] `security` luôn từ company (locked)
- [x] `pathAdditions` concat (không override)
- [x] `mcp.servers` dedup by name
- [x] `invalidate()` không args → clear toàn bộ cache
- [x] Không TypeScript errors
